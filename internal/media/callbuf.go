package media

import (
	"log/slog"
	"sync"
)

// callBuffer holds the DTMF digit channel for a single active call.
type callBuffer struct {
	// digits receives DTMF digits from any source (RFC 2833, SIP INFO).
	// Buffered to avoid blocking senders.
	digits chan string
}

// CallDTMFManager provides per-call DTMF buffer management. It maintains
// a registry of active calls and their associated digit channels, allowing
// multiple DTMF sources (RFC 2833 collector, SIP INFO handler) to feed
// digits into a single per-call channel that the DigitBuffer reads from.
//
// Typical lifecycle for IVR digit collection:
//
//  1. Flow engine calls Acquire(callID) to create a buffered digit channel.
//  2. DTMFCollector goroutine and/or SIP INFO handler inject digits via
//     Inject(callID, digit).
//  3. A DigitBuffer reads from the channel returned by Acquire.
//  4. When collection is complete, the flow engine calls Release(callID).
//
// All methods are safe for concurrent use.
type CallDTMFManager struct {
	mu      sync.RWMutex
	buffers map[string]*callBuffer
	logger  *slog.Logger
}

// callBufferSize is the capacity of the per-call digit channel. Generous
// enough to never block the sender under normal conditions (a human can
// only press digits so fast).
const callBufferSize = 32

// NewCallDTMFManager creates a new per-call DTMF buffer manager.
func NewCallDTMFManager(logger *slog.Logger) *CallDTMFManager {
	return &CallDTMFManager{
		buffers: make(map[string]*callBuffer),
		logger:  logger.With("subsystem", "call-dtmf"),
	}
}

// Acquire creates a new digit channel for the given call and returns it.
// The returned channel can be passed to NewDigitBuffer as the source.
// If a buffer already exists for the call, the existing channel is returned.
func (m *CallDTMFManager) Acquire(callID string) <-chan string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if buf, ok := m.buffers[callID]; ok {
		m.logger.Debug("reusing existing dtmf buffer",
			"call_id", callID,
		)
		return buf.digits
	}

	buf := &callBuffer{
		digits: make(chan string, callBufferSize),
	}
	m.buffers[callID] = buf

	m.logger.Debug("dtmf buffer acquired",
		"call_id", callID,
	)
	return buf.digits
}

// Release removes the digit channel for the given call. Any digits
// remaining in the channel are discarded. After Release, Inject calls
// for this callID are silently dropped. It is safe to call Release
// multiple times for the same callID.
func (m *CallDTMFManager) Release(callID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.buffers[callID]; !ok {
		return
	}
	delete(m.buffers, callID)

	m.logger.Debug("dtmf buffer released",
		"call_id", callID,
	)
}

// Inject sends a DTMF digit to the buffer for the given call. This is
// called by both the SIP INFO handler and the RFC 2833 DTMFCollector
// bridge. If no buffer exists for the call (collection not active), the
// digit is silently dropped. If the channel is full (unlikely), the digit
// is also dropped to avoid blocking the caller.
func (m *CallDTMFManager) Inject(callID string, digit string) {
	m.mu.RLock()
	buf, ok := m.buffers[callID]
	m.mu.RUnlock()

	if !ok {
		return
	}

	select {
	case buf.digits <- digit:
		m.logger.Debug("dtmf digit injected",
			"call_id", callID,
			"digit", digit,
		)
	default:
		m.logger.Warn("dtmf buffer full, digit dropped",
			"call_id", callID,
			"digit", digit,
		)
	}
}

// Has returns true if a DTMF buffer is currently active for the given call.
func (m *CallDTMFManager) Has(callID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.buffers[callID]
	return ok
}

// ActiveCount returns the number of calls with active DTMF buffers.
func (m *CallDTMFManager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.buffers)
}

// Drain removes all active buffers. Used during graceful shutdown.
func (m *CallDTMFManager) Drain() {
	m.mu.Lock()
	count := len(m.buffers)
	m.buffers = make(map[string]*callBuffer)
	m.mu.Unlock()

	if count > 0 {
		m.logger.Info("drained all dtmf buffers",
			"count", count,
		)
	}
}
