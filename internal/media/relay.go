package media

import (
	"errors"
	"log/slog"
	"net"
	"os"
	"sync"
	"sync/atomic"
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

// atomicAddr provides thread-safe storage for a UDP address.
// Used for symmetric RTP where the remote address is learned from the
// first incoming packet rather than relying solely on the SDP-signaled address.
type atomicAddr struct {
	v atomic.Pointer[net.UDPAddr]
}

func newAtomicAddr(addr *net.UDPAddr) *atomicAddr {
	a := &atomicAddr{}
	a.v.Store(addr)
	return a
}

func (a *atomicAddr) load() *net.UDPAddr {
	return a.v.Load()
}

// update atomically replaces the stored address and returns true if it changed.
func (a *atomicAddr) update(addr *net.UDPAddr) bool {
	old := a.v.Load()
	if old.IP.Equal(addr.IP) && old.Port == addr.Port {
		return false
	}
	a.v.Store(addr)
	return true
}

// Relay manages bidirectional RTP forwarding between two legs of a session.
// It reads packets from each leg's RTP socket and forwards them to the
// other leg's remote endpoint, filtering by allowed payload types.
//
// Symmetric RTP: The relay learns the actual remote address from the first
// valid RTP packet received on each leg. This handles NAT traversal because
// the real source address (post-NAT) may differ from the SDP-signaled address.
type Relay struct {
	session *Session
	logger  *slog.Logger

	// allowedPT is the set of payload types to relay.
	allowedPT map[int]struct{}

	// callerRemote is the learned remote RTP address for the caller leg.
	// Initialized from SDP and updated on first packet (symmetric RTP).
	callerRemote *atomicAddr
	// calleeRemote is the learned remote RTP address for the callee leg.
	// Initialized from SDP and updated on first packet (symmetric RTP).
	calleeRemote *atomicAddr

	// recorder captures both directions of RTP audio to a WAV file.
	// Set via SetRecorder before Start, or nil to disable recording.
	recorder *Recorder

	wg sync.WaitGroup
}

// NewRelay creates a relay for the given session with the specified allowed
// payload types. callerRemote and calleeRemote are the far-end RTP addresses
// learned from SDP negotiation. These addresses serve as initial targets and
// are updated via symmetric RTP when the first packet arrives from each endpoint.
func NewRelay(session *Session, callerRemote, calleeRemote *net.UDPAddr, allowedPayloadTypes []int, logger *slog.Logger) *Relay {
	pt := make(map[int]struct{}, len(allowedPayloadTypes))
	for _, p := range allowedPayloadTypes {
		pt[p] = struct{}{}
	}
	return &Relay{
		session:      session,
		logger:       logger.With("subsystem", "rtp-relay", "session_id", session.ID),
		allowedPT:    pt,
		callerRemote: newAtomicAddr(callerRemote),
		calleeRemote: newAtomicAddr(calleeRemote),
	}
}

// SetRecorder attaches a call recorder to this relay. Both directions of
// RTP audio will be fed to the recorder. Must be called before Start.
func (r *Relay) SetRecorder(rec *Recorder) {
	r.recorder = rec
}

// Start begins bidirectional RTP relay between the two legs.
// Caller→Callee: reads from CallerLeg.RTPConn, writes to CalleeLeg.RTPConn → calleeRemote.
// Callee→Caller: reads from CalleeLeg.RTPConn, writes to CallerLeg.RTPConn → callerRemote.
// Symmetric RTP: each direction learns the actual remote address from the first
// valid RTP packet, handling NAT traversal transparently.
// This method is non-blocking; relay runs in background goroutines.
func (r *Relay) Start() {
	r.session.SetState(SessionStateActive)

	r.wg.Add(2)
	go r.forward("caller→callee", r.session.CallerLeg.RTPConn, r.session.CalleeLeg.RTPConn, r.calleeRemote, r.callerRemote)
	go r.forward("callee→caller", r.session.CalleeLeg.RTPConn, r.session.CallerLeg.RTPConn, r.callerRemote, r.calleeRemote)

	r.logger.Info("rtp relay started",
		"caller_local_port", r.session.CallerLeg.Ports.RTP,
		"callee_local_port", r.session.CalleeLeg.Ports.RTP,
		"caller_remote", r.callerRemote.load().String(),
		"callee_remote", r.calleeRemote.load().String(),
	)
}

// Stop signals the relay goroutines to stop and waits for them to finish.
func (r *Relay) Stop() {
	r.session.Stop()
	r.wg.Wait()
	stats := r.session.Stats()
	r.logger.Info("rtp relay stopped",
		"session_id", r.session.ID,
		"packets_caller_to_callee", stats.PacketsCallerToCallee,
		"packets_callee_to_caller", stats.PacketsCalleeToCaller,
		"bytes_caller_to_callee", stats.BytesCallerToCallee,
		"bytes_callee_to_caller", stats.BytesCalleeToCaller,
		"packets_dropped", stats.PacketsDropped,
	)
}

// CallerAddr returns the current remote address for the caller leg.
// After symmetric RTP learning, this may differ from the SDP-signaled address.
func (r *Relay) CallerAddr() *net.UDPAddr {
	return r.callerRemote.load()
}

// CalleeAddr returns the current remote address for the callee leg.
// After symmetric RTP learning, this may differ from the SDP-signaled address.
func (r *Relay) CalleeAddr() *net.UDPAddr {
	return r.calleeRemote.load()
}

// readTimeout is the read deadline for UDP sockets in the relay loop.
// This allows goroutines to periodically check the stopped flag.
const readTimeout = 100 * time.Millisecond

// forward reads RTP packets from src and writes them to dst toward the
// given remote address. Only packets with allowed payload types are forwarded.
//
// Symmetric RTP: writeRemote is the destination for outgoing packets (the far end
// of the opposite leg). learnRemote is updated with the actual source address of
// the first valid RTP packet received on this leg. This allows the opposite
// direction's forward goroutine to send replies back to the real (post-NAT) address.
func (r *Relay) forward(direction string, src, dst *net.UDPConn, writeRemote, learnRemote *atomicAddr) {
	defer r.wg.Done()

	buf := make([]byte, maxRTPPacket)
	learned := false
	for {
		if r.session.IsStopped() {
			return
		}

		src.SetReadDeadline(time.Now().Add(readTimeout))
		n, srcAddr, err := src.ReadFromUDP(buf)
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
			r.session.RecordDrop()
			continue
		}

		if _, ok := r.allowedPT[pt]; !ok {
			// Payload type not in allowed set; drop.
			r.session.RecordDrop()
			continue
		}

		// Symmetric RTP: learn the actual remote address from the first
		// valid RTP packet. This handles NAT where the real source differs
		// from the SDP-signaled address.
		if !learned {
			if learnRemote.update(srcAddr) {
				r.logger.Info("symmetric rtp: learned remote address",
					"direction", direction,
					"address", srcAddr.String(),
				)
			}
			learned = true
		}

		// Feed RTP payload to recorder if active. The RTP payload starts
		// after the fixed 12-byte header (plus CSRC and extension if present,
		// but G.711 typically has none). We use the simple 12-byte offset.
		if r.recorder != nil && n > minRTPHeader {
			r.recorder.Feed(pkt[minRTPHeader:n], pt)
		}

		_, err = dst.WriteToUDP(pkt, writeRemote.load())
		if err != nil {
			if r.session.IsStopped() {
				return
			}
			r.logger.Debug("rtp write error",
				"direction", direction,
				"error", err,
			)
			continue
		}

		r.session.TouchActivity()
		r.session.RecordPacket(direction, n)
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
