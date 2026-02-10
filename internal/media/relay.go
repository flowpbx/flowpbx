package media

import (
	"errors"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"
)

const (
	// RTP payload types for supported codecs.
	PayloadPCMU = 0   // G.711 u-law
	PayloadPCMA = 8   // G.711 a-law
	PayloadOpus = 111 // Opus (dynamic, commonly 111)

	// maxRTPPacket is the maximum UDP packet size we handle.
	// Standard Ethernet MTU minus IP/UDP headers gives ~1472 bytes,
	// but we allow larger for jumbo frames or aggregation.
	maxRTPPacket = 1500

	// minRTPHeader is the minimum RTP header size (12 bytes).
	minRTPHeader = 12
)

// rtpPayloadType extracts the payload type from an RTP packet.
// Returns -1 if the packet is too small to be valid RTP.
func rtpPayloadType(pkt []byte) int {
	if len(pkt) < minRTPHeader {
		return -1
	}
	// Payload type is bits 1-7 of the second byte (mask off marker bit).
	return int(pkt[1] & 0x7F)
}

// Relay manages bidirectional RTP forwarding between two legs of a session.
// It reads packets from each leg's RTP socket and forwards them to the
// other leg's remote endpoint, filtering by allowed payload types.
type Relay struct {
	session *Session
	logger  *slog.Logger

	// allowedPT is the set of payload types to relay.
	allowedPT map[int]struct{}

	// callerRemote is the remote RTP address for the caller leg.
	callerRemote *net.UDPAddr
	// calleeRemote is the remote RTP address for the callee leg.
	calleeRemote *net.UDPAddr

	wg sync.WaitGroup
}

// NewRelay creates a relay for the given session with the specified allowed
// payload types. callerRemote and calleeRemote are the far-end RTP addresses
// learned from SDP negotiation.
func NewRelay(session *Session, callerRemote, calleeRemote *net.UDPAddr, allowedPayloadTypes []int, logger *slog.Logger) *Relay {
	pt := make(map[int]struct{}, len(allowedPayloadTypes))
	for _, p := range allowedPayloadTypes {
		pt[p] = struct{}{}
	}
	return &Relay{
		session:      session,
		logger:       logger.With("subsystem", "rtp-relay", "session_id", session.ID),
		allowedPT:    pt,
		callerRemote: callerRemote,
		calleeRemote: calleeRemote,
	}
}

// Start begins bidirectional RTP relay between the two legs.
// Caller→Callee: reads from CallerLeg.RTPConn, writes to CalleeLeg.RTPConn → calleeRemote.
// Callee→Caller: reads from CalleeLeg.RTPConn, writes to CallerLeg.RTPConn → callerRemote.
// This method is non-blocking; relay runs in background goroutines.
func (r *Relay) Start() {
	r.session.SetState(SessionStateActive)

	r.wg.Add(2)
	go r.forward("caller→callee", r.session.CallerLeg.RTPConn, r.session.CalleeLeg.RTPConn, r.calleeRemote)
	go r.forward("callee→caller", r.session.CalleeLeg.RTPConn, r.session.CallerLeg.RTPConn, r.callerRemote)

	r.logger.Info("rtp relay started",
		"caller_local_port", r.session.CallerLeg.Ports.RTP,
		"callee_local_port", r.session.CalleeLeg.Ports.RTP,
		"caller_remote", r.callerRemote.String(),
		"callee_remote", r.calleeRemote.String(),
	)
}

// Stop signals the relay goroutines to stop and waits for them to finish.
func (r *Relay) Stop() {
	r.session.Stop()
	r.wg.Wait()
	r.logger.Info("rtp relay stopped", "session_id", r.session.ID)
}

// readTimeout is the read deadline for UDP sockets in the relay loop.
// This allows goroutines to periodically check the stopped flag.
const readTimeout = 100 * time.Millisecond

// forward reads RTP packets from src and writes them to dst toward the
// given remote address. Only packets with allowed payload types are forwarded.
func (r *Relay) forward(direction string, src, dst *net.UDPConn, remote *net.UDPAddr) {
	defer r.wg.Done()

	buf := make([]byte, maxRTPPacket)
	for {
		if r.session.IsStopped() {
			return
		}

		src.SetReadDeadline(time.Now().Add(readTimeout))
		n, _, err := src.ReadFromUDP(buf)
		if err != nil {
			if r.session.IsStopped() {
				return
			}
			// Timeout is expected; loop to re-check stopped flag.
			if errors.Is(err, os.ErrDeadlineExceeded) {
				continue
			}
			r.logger.Debug("rtp read error",
				"direction", direction,
				"error", err,
			)
			continue
		}

		pkt := buf[:n]

		pt := rtpPayloadType(pkt)
		if pt < 0 {
			// Too small to be valid RTP; drop.
			continue
		}

		if _, ok := r.allowedPT[pt]; !ok {
			// Payload type not in allowed set; drop.
			continue
		}

		_, err = dst.WriteToUDP(pkt, remote)
		if err != nil {
			if r.session.IsStopped() {
				return
			}
			r.logger.Debug("rtp write error",
				"direction", direction,
				"error", err,
			)
		}
	}
}

// StartPCMARelay creates and starts a relay for G.711 a-law (PCMA, payload type 8)
// passthrough between the two legs of the session.
func StartPCMARelay(session *Session, callerRemote, calleeRemote *net.UDPAddr, logger *slog.Logger) *Relay {
	relay := NewRelay(session, callerRemote, calleeRemote, []int{PayloadPCMA}, logger)
	relay.Start()
	return relay
}

// StartPCMURelay creates and starts a relay for G.711 u-law (PCMU, payload type 0)
// passthrough between the two legs of the session.
func StartPCMURelay(session *Session, callerRemote, calleeRemote *net.UDPAddr, logger *slog.Logger) *Relay {
	relay := NewRelay(session, callerRemote, calleeRemote, []int{PayloadPCMU}, logger)
	relay.Start()
	return relay
}

// StartOpusRelay creates and starts a relay for Opus (payload type 111)
// passthrough between the two legs of the session.
func StartOpusRelay(session *Session, callerRemote, calleeRemote *net.UDPAddr, logger *slog.Logger) *Relay {
	relay := NewRelay(session, callerRemote, calleeRemote, []int{PayloadOpus}, logger)
	relay.Start()
	return relay
}
