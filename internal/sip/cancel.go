package sip

import (
	"log/slog"
	"sync"

	"github.com/emiago/sipgo/sip"
)

// PendingCall represents a call that is ringing but not yet answered.
// It holds the cancel function to abort all fork legs and the caller's
// server transaction so we can send 487 Request Terminated.
type PendingCall struct {
	// CallID is the SIP Call-ID for this pending call.
	CallID string

	// CallerTx is the original INVITE server transaction from the caller.
	CallerTx sip.ServerTransaction

	// CallerReq is the original INVITE request from the caller.
	CallerReq *sip.Request

	// CancelFork cancels the fork context, causing all outbound INVITE
	// legs to be cancelled.
	CancelFork func()

	// Bridge holds the allocated media bridge (may be nil). Released
	// if the call is cancelled before answer.
	Bridge *MediaBridge
}

// PendingCallManager tracks calls that are in the ringing/forking state
// (between INVITE receipt and answer or failure). This allows the CANCEL
// handler to find and abort pending calls.
type PendingCallManager struct {
	mu      sync.RWMutex
	pending map[string]*PendingCall // keyed by Call-ID
	logger  *slog.Logger
}

// NewPendingCallManager creates a new pending call tracker.
func NewPendingCallManager(logger *slog.Logger) *PendingCallManager {
	return &PendingCallManager{
		pending: make(map[string]*PendingCall),
		logger:  logger.With("subsystem", "pending-calls"),
	}
}

// Add registers a pending call. Called when forking begins.
func (pm *PendingCallManager) Add(pc *PendingCall) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.pending[pc.CallID] = pc
	pm.logger.Debug("pending call added",
		"call_id", pc.CallID,
	)
}

// Remove removes a pending call. Called when the call is answered or all
// forks fail. Returns the pending call, or nil if not found.
func (pm *PendingCallManager) Remove(callID string) *PendingCall {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pc, ok := pm.pending[callID]
	if !ok {
		return nil
	}
	delete(pm.pending, callID)
	pm.logger.Debug("pending call removed",
		"call_id", callID,
	)
	return pc
}

// Get retrieves a pending call by Call-ID without removing it.
func (pm *PendingCallManager) Get(callID string) *PendingCall {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.pending[callID]
}

// PendingCalls returns a snapshot of all currently pending (ringing) calls.
// The returned slice is a copy safe for iteration without holding the lock.
func (pm *PendingCallManager) PendingCalls() []*PendingCall {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	calls := make([]*PendingCall, 0, len(pm.pending))
	for _, pc := range pm.pending {
		calls = append(calls, pc)
	}
	return calls
}

// PendingCallCount returns the number of currently pending calls.
func (pm *PendingCallManager) PendingCallCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return len(pm.pending)
}

// Cancel cancels a pending call: aborts all fork legs and sends
// 487 Request Terminated to the caller's INVITE transaction.
// Returns true if the call was found and cancelled.
func (pm *PendingCallManager) Cancel(callID string, logger *slog.Logger) bool {
	pc := pm.Remove(callID)
	if pc == nil {
		return false
	}

	// Cancel the fork context — this causes the Forker.Fork() goroutines
	// to stop and all outbound INVITE legs to be cancelled.
	pc.CancelFork()

	// Release any pre-allocated media bridge.
	if pc.Bridge != nil {
		pc.Bridge.Release()
		logger.Debug("media bridge released on cancel",
			"call_id", callID,
		)
	}

	// Send 487 Request Terminated to the caller's original INVITE transaction.
	terminatedRes := sip.NewResponseFromRequest(pc.CallerReq, 487, "Request Terminated", nil)
	if err := pc.CallerTx.Respond(terminatedRes); err != nil {
		logger.Error("failed to send 487 to caller on cancel",
			"call_id", callID,
			"error", err,
		)
	} else {
		logger.Info("sent 487 request terminated to caller",
			"call_id", callID,
		)
	}

	return true
}
