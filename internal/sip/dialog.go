package sip

import (
	"log/slog"
	"sync"
	"time"

	"github.com/emiago/sipgo/sip"
	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/flowpbx/flowpbx/internal/media"
)

// CallState represents the lifecycle state of a call.
type CallState string

const (
	CallStateRinging    CallState = "ringing"
	CallStateAnswered   CallState = "answered"
	CallStateTerminated CallState = "terminated"
)

// CallLeg represents one side of a call (caller or callee).
type CallLeg struct {
	// Extension is the local extension for this leg (nil for trunk legs).
	Extension *models.Extension

	// Registration is the contact that answered (callee side).
	Registration *models.Registration

	// FromTag identifies the dialog participant (From header tag).
	FromTag string

	// ToTag identifies the dialog participant (To header tag).
	ToTag string

	// ContactURI is the Contact header URI for this leg.
	ContactURI string

	// RemoteTarget is the SIP URI to send in-dialog requests (BYE) to.
	RemoteTarget *sip.Uri
}

// Dialog represents an active call session between two parties.
// It tracks the SIP dialog state and timing information needed for
// CDR generation and call teardown.
type Dialog struct {
	// CallID is the SIP Call-ID header value shared by both legs.
	CallID string

	// State is the current lifecycle state of the call.
	State CallState

	// Direction is the call type (internal, inbound, outbound).
	Direction CallType

	// TrunkID is the trunk used for this call (inbound or outbound).
	// Zero for internal calls.
	TrunkID int64

	// Caller is the originating leg of the call.
	Caller CallLeg

	// Callee is the terminating leg of the call.
	Callee CallLeg

	// CallerIDName is the display name of the caller.
	CallerIDName string

	// CallerIDNum is the extension or phone number of the caller.
	CallerIDNum string

	// CalledNum is the dialed number/extension.
	CalledNum string

	// CallerTx is the inbound server transaction (caller → PBX).
	CallerTx sip.ServerTransaction

	// CallerReq is the original INVITE from the caller, needed for
	// building in-dialog requests (e.g. BYE).
	CallerReq *sip.Request

	// CalleeTx is the outbound client transaction (PBX → callee).
	CalleeTx sip.ClientTransaction

	// CalleeReq is the forked INVITE sent to the callee, needed for
	// building in-dialog requests (e.g. BYE).
	CalleeReq *sip.Request

	// CalleeRes is the 200 OK response from the callee, containing
	// dialog parameters (To tag, Contact) needed for BYE.
	CalleeRes *sip.Response

	// StartTime is when the INVITE was received.
	StartTime time.Time

	// AnswerTime is when the call was answered (200 OK received).
	AnswerTime *time.Time

	// EndTime is when the call was terminated (BYE sent/received).
	EndTime *time.Time

	// HangupCause describes why the call ended.
	HangupCause string

	// Media is the RTP media session for this call, managing the relay
	// between caller and callee legs. Released on call teardown.
	Media *media.MediaSession
}

// Duration returns the total call duration from start to end.
// Returns zero if the call has not ended.
func (d *Dialog) Duration() time.Duration {
	if d.EndTime == nil {
		return 0
	}
	return d.EndTime.Sub(d.StartTime)
}

// BillableDuration returns the duration from answer to end.
// Returns zero if the call was never answered or has not ended.
func (d *Dialog) BillableDuration() time.Duration {
	if d.AnswerTime == nil || d.EndTime == nil {
		return 0
	}
	return d.EndTime.Sub(*d.AnswerTime)
}

// Disposition returns the CDR disposition string based on call state.
func (d *Dialog) Disposition() string {
	switch {
	case d.State == CallStateTerminated && d.AnswerTime != nil:
		return "answered"
	case d.HangupCause == "caller_cancel":
		return "cancelled"
	case d.HangupCause == "no_answer":
		return "no_answer"
	case d.HangupCause == "busy":
		return "busy"
	default:
		return "failed"
	}
}

// DialogManager tracks all active call dialogs in memory.
// It provides thread-safe access for concurrent SIP request processing.
type DialogManager struct {
	mu      sync.RWMutex
	dialogs map[string]*Dialog // keyed by Call-ID
	logger  *slog.Logger
}

// NewDialogManager creates a new in-memory dialog tracker.
func NewDialogManager(logger *slog.Logger) *DialogManager {
	return &DialogManager{
		dialogs: make(map[string]*Dialog),
		logger:  logger.With("subsystem", "dialog"),
	}
}

// CreateDialog registers a new call dialog when an INVITE is answered.
// The dialog is stored with state CallStateAnswered.
func (dm *DialogManager) CreateDialog(d *Dialog) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	now := time.Now()
	d.AnswerTime = &now
	d.State = CallStateAnswered

	dm.dialogs[d.CallID] = d
	dm.logger.Info("dialog created",
		"call_id", d.CallID,
		"direction", d.Direction,
		"caller", d.CallerIDNum,
		"callee", d.CalledNum,
	)
}

// GetDialog retrieves an active dialog by Call-ID.
// Returns nil if no dialog exists for the given Call-ID.
func (dm *DialogManager) GetDialog(callID string) *Dialog {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.dialogs[callID]
}

// TerminateDialog marks a dialog as terminated and removes it from the
// active map. Returns the terminated dialog for CDR generation, or nil
// if no dialog was found.
func (dm *DialogManager) TerminateDialog(callID string, hangupCause string) *Dialog {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	d, ok := dm.dialogs[callID]
	if !ok {
		return nil
	}

	now := time.Now()
	d.EndTime = &now
	d.State = CallStateTerminated
	d.HangupCause = hangupCause

	delete(dm.dialogs, callID)
	dm.logger.Info("dialog terminated",
		"call_id", d.CallID,
		"direction", d.Direction,
		"hangup_cause", hangupCause,
		"duration_ms", d.Duration().Milliseconds(),
		"billable_ms", d.BillableDuration().Milliseconds(),
	)

	return d
}

// ActiveCalls returns a snapshot of all currently active dialogs.
// The returned slice is a copy safe for iteration without holding the lock.
func (dm *DialogManager) ActiveCalls() []*Dialog {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	calls := make([]*Dialog, 0, len(dm.dialogs))
	for _, d := range dm.dialogs {
		calls = append(calls, d)
	}
	return calls
}

// ActiveCallCount returns the number of currently active calls.
func (dm *DialogManager) ActiveCallCount() int {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return len(dm.dialogs)
}

// HasDialog returns true if a dialog exists for the given Call-ID.
func (dm *DialogManager) HasDialog(callID string) bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	_, ok := dm.dialogs[callID]
	return ok
}

// ActiveCallCountForTrunk returns the number of active calls using the given
// trunk ID. This is used to enforce the trunk's max_channels limit: if the
// count equals or exceeds max_channels, the trunk should not accept new calls.
func (dm *DialogManager) ActiveCallCountForTrunk(trunkID int64) int {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	count := 0
	for _, d := range dm.dialogs {
		if d.TrunkID == trunkID {
			count++
		}
	}
	return count
}

// ActiveCallCountForExtension returns the number of active calls involving
// the given extension ID (as either caller or callee). This is used for
// busy detection: if the count equals or exceeds the number of registered
// devices, the extension is considered busy.
func (dm *DialogManager) ActiveCallCountForExtension(extensionID int64) int {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	count := 0
	for _, d := range dm.dialogs {
		if d.Caller.Extension != nil && d.Caller.Extension.ID == extensionID {
			count++
		}
		if d.Callee.Extension != nil && d.Callee.Extension.ID == extensionID {
			count++
		}
	}
	return count
}
