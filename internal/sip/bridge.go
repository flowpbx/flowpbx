package sip

import (
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/flowpbx/flowpbx/internal/media"
)

// MediaBridge manages the two-phase media bridging setup for a call.
// Phase 1 (pre-fork): allocate RTP session, rewrite caller SDP for the callee.
// Phase 2 (post-answer): rewrite callee SDP for the caller, start RTP relay.
type MediaBridge struct {
	session  *media.MediaSession
	proxyIP  string
	callID   string
	logger   *slog.Logger
	codecPT  int
	callerSD *media.SessionDescription
}

// AllocateMediaBridge performs phase 1 of media bridging: parses the caller's
// SDP, allocates an RTP session with two port pairs, and rewrites the caller's
// SDP so the callee's RTP is directed to the proxy's callee-leg socket.
//
// Returns the MediaBridge (for phase 2) and the rewritten SDP body that
// should be sent in the forked INVITE to the callee.
func AllocateMediaBridge(
	sessionMgr *media.SessionManager,
	callerSDPBody []byte,
	callID string,
	proxyIP string,
	logger *slog.Logger,
) (*MediaBridge, []byte, error) {
	// Parse caller's SDP.
	callerSD, err := media.ParseSDP(callerSDPBody)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing caller sdp: %w", err)
	}

	callerAudio := callerSD.AudioMedia()
	if callerAudio == nil {
		return nil, nil, fmt.Errorf("caller sdp has no audio media")
	}

	// Allocate an RTP session with two port pairs (caller + callee legs).
	ms, err := media.CreateMediaSession(sessionMgr, callID, callID, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("allocating media session: %w", err)
	}

	// Rewrite caller's SDP: replace IP/port with proxy's callee-leg address.
	// This SDP will be sent in the forked INVITE so the callee sends its
	// RTP to the proxy's callee-leg socket.
	rewrittenForCallee, err := media.RewriteSDPBytes(callerSDPBody, proxyIP, ms.CalleeRTPPort())
	if err != nil {
		ms.Release()
		return nil, nil, fmt.Errorf("rewriting sdp for callee: %w", err)
	}

	logger.Info("media bridge allocated",
		"call_id", callID,
		"proxy_ip", proxyIP,
		"caller_leg_port", ms.CallerRTPPort(),
		"callee_leg_port", ms.CalleeRTPPort(),
	)

	return &MediaBridge{
		session:  ms,
		proxyIP:  proxyIP,
		callID:   callID,
		logger:   logger,
		callerSD: callerSD,
	}, rewrittenForCallee, nil
}

// CompleteMediaBridge performs phase 2 of media bridging: parses the callee's
// 200 OK SDP, negotiates a common audio codec, rewrites the callee's SDP so
// the caller's RTP is directed to the proxy's caller-leg socket, and starts
// the bidirectional RTP relay.
//
// Returns the rewritten SDP body to send in the 200 OK to the caller.
// On error, the media session is released.
func (mb *MediaBridge) CompleteMediaBridge(calleeSDPBody []byte) ([]byte, error) {
	// Parse callee's SDP.
	calleeSDP, err := media.ParseSDP(calleeSDPBody)
	if err != nil {
		mb.Release()
		return nil, fmt.Errorf("parsing callee sdp: %w", err)
	}

	// Negotiate a common audio codec.
	codecPT, codecName, err := negotiateAudioCodec(mb.callerSD, calleeSDP)
	if err != nil {
		mb.Release()
		return nil, fmt.Errorf("codec negotiation failed: %w", err)
	}
	mb.codecPT = codecPT

	mb.logger.Info("codec negotiated",
		"call_id", mb.callID,
		"codec", codecName,
		"payload_type", codecPT,
	)

	// Rewrite callee's SDP: replace IP/port with proxy's caller-leg address.
	// This SDP will be sent in the 200 OK to the caller so it sends RTP
	// to the proxy's caller-leg socket.
	rewrittenForCaller, err := media.RewriteSDPBytes(calleeSDPBody, mb.proxyIP, mb.session.CallerRTPPort())
	if err != nil {
		mb.Release()
		return nil, fmt.Errorf("rewriting sdp for caller: %w", err)
	}

	// Extract far-end RTP addresses from the original (pre-rewrite) SDPs.
	callerRemote, err := extractRTPAddr(mb.callerSD)
	if err != nil {
		mb.Release()
		return nil, fmt.Errorf("extracting caller rtp address: %w", err)
	}

	calleeRemote, err := extractRTPAddr(calleeSDP)
	if err != nil {
		mb.Release()
		return nil, fmt.Errorf("extracting callee rtp address: %w", err)
	}

	// Allowed payload types: the negotiated audio codec + DTMF.
	allowedPTs := []int{codecPT, media.PayloadTelephoneEvent}

	// Start the bidirectional RTP relay.
	if err := mb.session.StartRelay(callerRemote, calleeRemote, allowedPTs); err != nil {
		mb.Release()
		return nil, fmt.Errorf("starting rtp relay: %w", err)
	}

	mb.logger.Info("media bridge active",
		"call_id", mb.callID,
		"caller_remote", callerRemote.String(),
		"callee_remote", calleeRemote.String(),
		"codec", codecName,
	)

	return rewrittenForCaller, nil
}

// Session returns the underlying media session for attaching to the dialog.
func (mb *MediaBridge) Session() *media.MediaSession {
	return mb.session
}

// Release stops and releases the media session. Called on error paths
// or when the call fails before media bridging completes.
func (mb *MediaBridge) Release() {
	mb.session.Release()
}

// negotiateAudioCodec finds the first common audio codec between the caller
// and callee SDPs. It iterates the caller's format list in preference order
// and returns the callee's payload type for the matching codec name.
func negotiateAudioCodec(callerSDP, calleeSDP *media.SessionDescription) (int, string, error) {
	callerAudio := callerSDP.AudioMedia()
	if callerAudio == nil {
		return 0, "", fmt.Errorf("caller sdp has no audio media")
	}

	calleeAudio := calleeSDP.AudioMedia()
	if calleeAudio == nil {
		return 0, "", fmt.Errorf("callee sdp has no audio media")
	}

	// Walk caller's format list in preference order.
	for _, pt := range callerAudio.Formats {
		callerCodec := callerAudio.CodecByPayloadType(pt)
		if callerCodec == nil {
			// Static payload types (0=PCMU, 8=PCMA) may lack rtpmap.
			name := staticPTName(pt)
			if name == "" {
				continue
			}
			if calleeCodec := calleeAudio.CodecByName(name); calleeCodec != nil {
				return calleeCodec.PayloadType, name, nil
			}
			// Callee might also use static PT without rtpmap.
			for _, cpt := range calleeAudio.Formats {
				if cpt == pt {
					return pt, name, nil
				}
			}
			continue
		}

		// Skip telephone-event — not an audio codec.
		if strings.EqualFold(callerCodec.Name, "telephone-event") {
			continue
		}

		if calleeCodec := calleeAudio.CodecByName(callerCodec.Name); calleeCodec != nil {
			return calleeCodec.PayloadType, callerCodec.Name, nil
		}
	}

	return 0, "", fmt.Errorf("no common audio codec between caller and callee")
}

// staticPTName returns the codec name for well-known static RTP payload types.
func staticPTName(pt int) string {
	switch pt {
	case media.PayloadPCMU:
		return "PCMU"
	case media.PayloadPCMA:
		return "PCMA"
	default:
		return ""
	}
}

// extractRTPAddr extracts the RTP endpoint address (IP:port) from an SDP's
// first audio media description.
func extractRTPAddr(sd *media.SessionDescription) (*net.UDPAddr, error) {
	audio := sd.AudioMedia()
	if audio == nil {
		return nil, fmt.Errorf("no audio media in sdp")
	}

	ip := sd.ConnectionAddress(audio)
	if ip == "" {
		return nil, fmt.Errorf("no connection address in sdp")
	}

	return &net.UDPAddr{
		IP:   net.ParseIP(ip),
		Port: audio.Port,
	}, nil
}
