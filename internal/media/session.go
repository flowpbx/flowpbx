package media

import (
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

// SessionManager handles allocation and tracking of RTP media sessions.
// It uses the underlying Proxy to allocate port pairs and maintains a
// registry of active sessions.
type SessionManager struct {
	proxy  *Proxy
	logger *slog.Logger

	mu       sync.RWMutex
	sessions map[string]*Session // keyed by session ID
}

// NewSessionManager creates a session manager backed by the given proxy.
func NewSessionManager(proxy *Proxy, logger *slog.Logger) *SessionManager {
	return &SessionManager{
		proxy:    proxy,
		logger:   logger.With("subsystem", "media-sessions"),
		sessions: make(map[string]*Session),
	}
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
