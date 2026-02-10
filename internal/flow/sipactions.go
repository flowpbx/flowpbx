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

// CollectResult describes the outcome of a play-and-collect DTMF operation.
type CollectResult struct {
	// Digits contains the DTMF digits collected before a terminator or timeout.
	Digits string

	// TimedOut is true if the collect operation timed out before receiving input.
	TimedOut bool
}

// RecordResult describes the outcome of a voicemail recording operation.
type RecordResult struct {
	// FilePath is the path to the recorded WAV file.
	FilePath string

	// DurationSecs is the duration of the recording in seconds.
	DurationSecs int
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

	// PlayAndCollect plays an audio prompt (by file path or TTS text) and
	// collects DTMF digits. It waits up to timeout seconds for the first
	// digit and up to digitTimeout seconds between subsequent digits.
	// maxDigits limits the number of digits collected (0 = single digit).
	// The terminator character (e.g. "#") ends collection early if pressed.
	PlayAndCollect(ctx context.Context, callCtx *CallContext, prompt string, isTTS bool, timeout int, digitTimeout int, maxDigits int) (*CollectResult, error)

	// RecordMessage plays a greeting prompt and then records the caller's
	// message to a WAV file. Recording stops when the caller hangs up,
	// presses '#', or the maxDuration (seconds) is reached. The filePath
	// specifies the destination file for the recording.
	RecordMessage(ctx context.Context, callCtx *CallContext, greeting string, maxDuration int, filePath string) (*RecordResult, error)

	// SendMWI sends a SIP NOTIFY message to the specified extension to
	// update the Message Waiting Indicator (voicemail lamp). newMessages
	// and oldMessages indicate the counts for the mailbox summary.
	SendMWI(ctx context.Context, ext *models.Extension, newMessages int, oldMessages int) error

	// HangupCall terminates the call with the given SIP cause code and
	// reason phrase. For answered calls this sends BYE; for unanswered
	// calls this sends the appropriate error response.
	HangupCall(ctx context.Context, callCtx *CallContext, cause int, reason string) error

	// BlindTransfer performs a blind (unattended) transfer of the call
	// to the specified destination URI or extension number. The call is
	// handed off and the flow terminates.
	BlindTransfer(ctx context.Context, callCtx *CallContext, destination string) error

	// JoinConference joins the caller into the specified conference bridge.
	// This call blocks until the caller leaves the conference (hang up,
	// kicked, or context cancellation). PIN verification is handled
	// internally if the bridge requires one.
	JoinConference(ctx context.Context, callCtx *CallContext, bridge *models.ConferenceBridge) error
}
