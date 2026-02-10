package flow

import (
	"context"

	"github.com/flowpbx/flowpbx/internal/database/models"
)

// RingResult describes the outcome of ringing an extension or set of extensions.
type RingResult struct {
	// Answered is true if a device answered the call.
	Answered bool

	// AllBusy is true if every target device was busy.
	AllBusy bool

	// DND is true if the target extension had Do Not Disturb enabled.
	DND bool

	// NoRegistrations is true if the target had no active registrations.
	NoRegistrations bool

	// Error is set if ringing failed for a non-SIP reason.
	Error error
}

// SIPActions abstracts the SIP operations that flow node handlers need to
// perform. This interface is defined in the flow package to avoid circular
// dependencies between flow and sip packages. The sip package implements it.
type SIPActions interface {
	// RingExtension rings all registered devices for the given extension.
	// It forks INVITE to all active registrations, waits up to ringTimeout
	// seconds for an answer, and manages media bridging and dialog creation.
	// The callCtx provides the inbound call's SIP request and transaction.
	RingExtension(ctx context.Context, callCtx *CallContext, ext *models.Extension, ringTimeout int) (*RingResult, error)

	// RingGroup rings multiple extensions simultaneously (ring_all strategy).
	// It gathers all active registrations from the provided extensions,
	// forks INVITE to all of them, and returns the result. The first device
	// to answer wins; all other forks are cancelled.
	RingGroup(ctx context.Context, callCtx *CallContext, extensions []*models.Extension, ringTimeout int) (*RingResult, error)
}
