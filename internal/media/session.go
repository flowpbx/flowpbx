package media

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// SessionState represents the lifecycle state of an RTP session.
type SessionState int

const (
	SessionStateNew     SessionState = iota // allocated, not yet relaying
	SessionStateActive                      // actively relaying RTP
	SessionStateStopped                     // stopped, awaiting release
)

func (s SessionState) String() string {
	switch s {
	case SessionStateNew:
		return "new"
	case SessionStateActive:
		return "active"
	case SessionStateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// SessionStats holds RTP packet counters and byte totals for a session.
// All values are snapshots captured atomically at the time of the call.
type SessionStats struct {
	// PacketsCallerToCallee is the number of RTP packets forwarded from caller to callee.
	PacketsCallerToCallee uint64
	// PacketsCalleeToCaller is the number of RTP packets forwarded from callee to caller.
	PacketsCalleeToCaller uint64
	// BytesCallerToCallee is the total bytes forwarded from caller to callee.
	BytesCallerToCallee uint64
	// BytesCalleeToCaller is the total bytes forwarded from callee to caller.
	BytesCalleeToCaller uint64
	// PacketsDropped is the total number of packets dropped (invalid or filtered).
	PacketsDropped uint64
}

// TotalPackets returns the total number of packets forwarded in both directions.
func (s SessionStats) TotalPackets() uint64 {
	return s.PacketsCallerToCallee + s.PacketsCalleeToCaller
}

// TotalBytes returns the total bytes forwarded in both directions.
func (s SessionStats) TotalBytes() uint64 {
	return s.BytesCallerToCallee + s.BytesCalleeToCaller
}

// Session represents an RTP media session for a single call.
// Each call has two legs (caller and callee), and each leg gets a dedicated
// RTP+RTCP socket pair. The session manages the lifecycle of both pairs.
type Session struct {
	ID        string
	CallID    string
	CallerLeg *SocketPair
	CalleeLeg *SocketPair
	CreatedAt time.Time

	mu    sync.RWMutex
	state SessionState

	// stopped is used to signal relay goroutines to stop.
	stopped atomic.Bool

	// lastActivity stores the unix nanosecond timestamp of the last
	// forwarded RTP packet. Used by the reaper to detect orphaned sessions.
	lastActivity atomic.Int64

	// RTP packet counters — updated atomically by the relay goroutines.
	packetsCallerToCallee atomic.Uint64
	packetsCalleeToCaller atomic.Uint64
	bytesCallerToCallee   atomic.Uint64
	bytesCalleeToCaller   atomic.Uint64
	packetsDropped        atomic.Uint64
}

// State returns the current session state.
func (s *Session) State() SessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// SetState transitions the session to a new state.
func (s *Session) SetState(state SessionState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
}

// Stop signals the session to cease relaying media.
func (s *Session) Stop() {
	s.stopped.Store(true)
	s.SetState(SessionStateStopped)
}

// IsStopped returns true if the session has been signaled to stop.
func (s *Session) IsStopped() bool {
	return s.stopped.Load()
}

// TouchActivity updates the last activity timestamp to now.
// Called by the relay on each successfully forwarded RTP packet.
func (s *Session) TouchActivity() {
	s.lastActivity.Store(time.Now().UnixNano())
}

// LastActivity returns the time of the last forwarded RTP packet,
// or the session creation time if no packets have been relayed.
func (s *Session) LastActivity() time.Time {
	ns := s.lastActivity.Load()
	if ns == 0 {
		return s.CreatedAt
	}
	return time.Unix(0, ns)
}

// RecordPacket records a successfully forwarded packet for the given direction.
// direction should be "caller→callee" or "callee→caller".
func (s *Session) RecordPacket(direction string, size int) {
	switch direction {
	case "caller→callee":
		s.packetsCallerToCallee.Add(1)
		s.bytesCallerToCallee.Add(uint64(size))
	case "callee→caller":
		s.packetsCalleeToCaller.Add(1)
		s.bytesCalleeToCaller.Add(uint64(size))
	}
}

// RecordDrop records a dropped packet.
func (s *Session) RecordDrop() {
	s.packetsDropped.Add(1)
}

// Stats returns a snapshot of the session's RTP packet counters.
func (s *Session) Stats() SessionStats {
	return SessionStats{
		PacketsCallerToCallee: s.packetsCallerToCallee.Load(),
		PacketsCalleeToCaller: s.packetsCalleeToCaller.Load(),
		BytesCallerToCallee:   s.bytesCallerToCallee.Load(),
		BytesCalleeToCaller:   s.bytesCalleeToCaller.Load(),
		PacketsDropped:        s.packetsDropped.Load(),
	}
}

const (
	// DefaultSessionTimeout is how long a session can be idle (no RTP
	// packets forwarded) before the reaper considers it orphaned.
	DefaultSessionTimeout = 60 * time.Second

	// defaultReapInterval is how often the reaper scans for orphaned sessions.
	defaultReapInterval = 30 * time.Second
)

// SessionManager handles allocation and tracking of RTP media sessions.
// It uses the underlying Proxy to allocate port pairs and maintains a
// registry of active sessions. A background reaper goroutine can be
// started to automatically clean up orphaned sessions that have had
// no RTP activity within the configured timeout.
type SessionManager struct {
	proxy  *Proxy
	logger *slog.Logger

	mu       sync.RWMutex
	sessions map[string]*Session // keyed by session ID

	sessionTimeout time.Duration
	cancelReaper   context.CancelFunc
	reaperDone     chan struct{}
}

// NewSessionManager creates a session manager backed by the given proxy.
func NewSessionManager(proxy *Proxy, logger *slog.Logger) *SessionManager {
	return &SessionManager{
		proxy:          proxy,
		logger:         logger.With("subsystem", "media-sessions"),
		sessions:       make(map[string]*Session),
		sessionTimeout: DefaultSessionTimeout,
	}
}

// SetSessionTimeout configures the idle timeout for orphaned session
// detection. Must be called before StartReaper.
func (m *SessionManager) SetSessionTimeout(d time.Duration) {
	m.sessionTimeout = d
}

// Allocate creates a new RTP session for a call by allocating two port pairs:
// one for the caller leg and one for the callee leg. The session is registered
// and returned in the New state.
func (m *SessionManager) Allocate(sessionID, callID string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[sessionID]; exists {
		return nil, fmt.Errorf("session %q already exists", sessionID)
	}

	callerPair, err := m.proxy.Allocate()
	if err != nil {
		return nil, fmt.Errorf("allocating caller leg: %w", err)
	}

	calleePair, err := m.proxy.Allocate()
	if err != nil {
		// Release the caller pair since we failed to get the callee pair.
		m.proxy.Release(callerPair)
		return nil, fmt.Errorf("allocating callee leg: %w", err)
	}

	session := &Session{
		ID:        sessionID,
		CallID:    callID,
		CallerLeg: callerPair,
		CalleeLeg: calleePair,
		CreatedAt: time.Now(),
		state:     SessionStateNew,
	}

	m.sessions[sessionID] = session

	m.logger.Info("rtp session allocated",
		"session_id", sessionID,
		"call_id", callID,
		"caller_rtp_port", callerPair.Ports.RTP,
		"callee_rtp_port", calleePair.Ports.RTP,
	)

	return session, nil
}

// Release stops and releases all resources for a session, returning its port
// pairs to the pool.
func (m *SessionManager) Release(sessionID string) {
	m.mu.Lock()
	session, exists := m.sessions[sessionID]
	if !exists {
		m.mu.Unlock()
		return
	}
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	session.Stop()
	m.proxy.Release(session.CallerLeg)
	m.proxy.Release(session.CalleeLeg)

	m.logger.Info("rtp session released",
		"session_id", sessionID,
		"call_id", session.CallID,
	)
}

// Get returns a session by ID, or nil if not found.
func (m *SessionManager) Get(sessionID string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[sessionID]
}

// Count returns the number of active sessions.
func (m *SessionManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// ReleaseAll stops and releases all sessions. Used during shutdown.
func (m *SessionManager) ReleaseAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	for _, id := range ids {
		m.Release(id)
	}

	m.logger.Info("all rtp sessions released", "count", len(ids))
}

// StartReaper launches a background goroutine that periodically scans for
// sessions with no RTP activity within the configured timeout and releases
// them. Call StopReaper to shut down the goroutine gracefully.
func (m *SessionManager) StartReaper() {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelReaper = cancel
	m.reaperDone = make(chan struct{})

	go m.reapLoop(ctx)

	m.logger.Info("session reaper started",
		"timeout", m.sessionTimeout.String(),
		"interval", defaultReapInterval.String(),
	)
}

// StopReaper signals the reaper goroutine to stop and waits for it to finish.
func (m *SessionManager) StopReaper() {
	if m.cancelReaper == nil {
		return
	}
	m.cancelReaper()
	<-m.reaperDone
	m.logger.Info("session reaper stopped")
}

// reapLoop runs the periodic orphan scan until the context is cancelled.
func (m *SessionManager) reapLoop(ctx context.Context) {
	defer close(m.reaperDone)

	ticker := time.NewTicker(defaultReapInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.reapOrphaned()
		}
	}
}

// reapOrphaned scans all sessions and releases any that have been idle
// longer than the configured session timeout.
func (m *SessionManager) reapOrphaned() {
	now := time.Now()

	m.mu.RLock()
	var orphaned []string
	for id, session := range m.sessions {
		idle := now.Sub(session.LastActivity())
		if idle > m.sessionTimeout {
			orphaned = append(orphaned, id)
		}
	}
	m.mu.RUnlock()

	for _, id := range orphaned {
		m.logger.Warn("reaping orphaned rtp session",
			"session_id", id,
			"timeout", m.sessionTimeout.String(),
		)
		m.Release(id)
	}

	if len(orphaned) > 0 {
		m.logger.Info("reaper cycle complete",
			"reaped", len(orphaned),
			"remaining", m.Count(),
		)
	}
}
