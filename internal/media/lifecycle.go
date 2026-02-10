package media

import (
	"fmt"
	"log/slog"
	"net"
	"sync"
)

// MediaSession manages the full lifecycle of an RTP media session for a call:
// create (allocate ports) → start relay → stop → release ports.
//
// It ties together the SessionManager (port allocation), Session (state tracking),
// and Relay (RTP forwarding) into a single cohesive unit that can be controlled
// by the call handler.
type MediaSession struct {
	manager *SessionManager
	session *Session
	logger  *slog.Logger

	mu    sync.Mutex
	relay *Relay
}

// CreateMediaSession allocates a new media session with two port pairs
// (caller and callee legs) from the session manager. The session starts
// in the New state, ready for relay to begin.
func CreateMediaSession(manager *SessionManager, sessionID, callID string, logger *slog.Logger) (*MediaSession, error) {
	session, err := manager.Allocate(sessionID, callID)
	if err != nil {
		return nil, fmt.Errorf("creating media session: %w", err)
	}

	return &MediaSession{
		manager: manager,
		session: session,
		logger:  logger.With("subsystem", "media-lifecycle", "session_id", sessionID, "call_id", callID),
	}, nil
}

// Session returns the underlying RTP session.
func (ms *MediaSession) Session() *Session {
	return ms.session
}

// StartRelay begins bidirectional RTP relay between the caller and callee legs.
// The allowedPayloadTypes controls which RTP payload types are forwarded.
// callerRemote and calleeRemote are the far-end addresses from SDP negotiation.
//
// Returns an error if the session is not in the New state or if a relay is
// already running.
func (ms *MediaSession) StartRelay(callerRemote, calleeRemote *net.UDPAddr, allowedPayloadTypes []int) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.relay != nil {
		return fmt.Errorf("relay already started for session %q", ms.session.ID)
	}

	state := ms.session.State()
	if state != SessionStateNew {
		return fmt.Errorf("cannot start relay: session %q in state %s, expected new", ms.session.ID, state)
	}

	ms.relay = NewRelay(ms.session, callerRemote, calleeRemote, allowedPayloadTypes, ms.logger)
	ms.relay.Start()

	ms.logger.Info("media session relay started",
		"caller_remote", callerRemote.String(),
		"callee_remote", calleeRemote.String(),
		"payload_types", allowedPayloadTypes,
	)

	return nil
}

// SetRecorder attaches a call recorder to the relay. Both directions of
// RTP audio will be fed to the recorder. Must be called after StartRelay.
// Returns an error if no relay is running.
func (ms *MediaSession) SetRecorder(rec *Recorder) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.relay == nil {
		return fmt.Errorf("cannot set recorder: no relay running for session %q", ms.session.ID)
	}
	ms.relay.SetRecorder(rec)
	return nil
}

// Stop gracefully stops the relay (if running) and transitions the session
// to the Stopped state. The session remains allocated; call Release to
// return ports to the pool.
func (ms *MediaSession) Stop() {
	ms.mu.Lock()
	relay := ms.relay
	ms.mu.Unlock()

	if relay != nil {
		relay.Stop()
	} else {
		// No relay running; just mark the session as stopped.
		ms.session.Stop()
	}

	ms.logger.Info("media session stopped")
}

// Release stops the relay (if running) and releases all port pairs back to
// the pool. After Release, the MediaSession must not be used.
func (ms *MediaSession) Release() {
	ms.Stop()
	ms.manager.Release(ms.session.ID)
	ms.logger.Info("media session released")
}

// State returns the current lifecycle state of the session.
func (ms *MediaSession) State() SessionState {
	return ms.session.State()
}

// CallerAddr returns the current remote address for the caller leg.
// After symmetric RTP learning, this may differ from the SDP-signaled address.
// Returns nil if no relay has been started.
func (ms *MediaSession) CallerAddr() *net.UDPAddr {
	ms.mu.Lock()
	relay := ms.relay
	ms.mu.Unlock()
	if relay == nil {
		return nil
	}
	return relay.CallerAddr()
}

// CalleeAddr returns the current remote address for the callee leg.
// After symmetric RTP learning, this may differ from the SDP-signaled address.
// Returns nil if no relay has been started.
func (ms *MediaSession) CalleeAddr() *net.UDPAddr {
	ms.mu.Lock()
	relay := ms.relay
	ms.mu.Unlock()
	if relay == nil {
		return nil
	}
	return relay.CalleeAddr()
}

// CallerRTPPort returns the local RTP port allocated for the caller leg.
func (ms *MediaSession) CallerRTPPort() int {
	return ms.session.CallerLeg.Ports.RTP
}

// CalleeRTPPort returns the local RTP port allocated for the callee leg.
func (ms *MediaSession) CalleeRTPPort() int {
	return ms.session.CalleeLeg.Ports.RTP
}

// Stats returns a snapshot of the session's RTP packet counters.
func (ms *MediaSession) Stats() SessionStats {
	return ms.session.Stats()
}
