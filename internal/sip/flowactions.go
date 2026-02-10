package sip

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/emiago/sipgo/sip"
	"github.com/flowpbx/flowpbx/internal/database"
	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/flowpbx/flowpbx/internal/flow"
	"github.com/flowpbx/flowpbx/internal/media"
)

// FlowSIPActions implements flow.SIPActions, bridging the flow engine's node
// handlers to the SIP stack's call routing and forking infrastructure.
type FlowSIPActions struct {
	extensions    database.ExtensionRepository
	registrations database.RegistrationRepository
	forker        *Forker
	dialogMgr     *DialogManager
	pendingMgr    *PendingCallManager
	sessionMgr    *media.SessionManager
	cdrs          database.CDRRepository
	proxyIP       string
	logger        *slog.Logger
}

// NewFlowSIPActions creates a new SIP actions adapter for the flow engine.
func NewFlowSIPActions(
	extensions database.ExtensionRepository,
	registrations database.RegistrationRepository,
	forker *Forker,
	dialogMgr *DialogManager,
	pendingMgr *PendingCallManager,
	sessionMgr *media.SessionManager,
	cdrs database.CDRRepository,
	proxyIP string,
	logger *slog.Logger,
) *FlowSIPActions {
	return &FlowSIPActions{
		extensions:    extensions,
		registrations: registrations,
		forker:        forker,
		dialogMgr:     dialogMgr,
		pendingMgr:    pendingMgr,
		sessionMgr:    sessionMgr,
		cdrs:          cdrs,
		proxyIP:       proxyIP,
		logger:        logger.With("subsystem", "flow_sip_actions"),
	}
}

// RingExtension rings all registered devices for the given extension.
// It performs the same forking and media bridging as handleInternalCall/handleInboundCall,
// but returns the result to the flow engine instead of directly terminating.
func (a *FlowSIPActions) RingExtension(ctx context.Context, callCtx *flow.CallContext, ext *models.Extension, ringTimeout int) (*flow.RingResult, error) {
	if callCtx.Request == nil || callCtx.Transaction == nil {
		return nil, fmt.Errorf("call context has no sip request or transaction")
	}

	// Check DND.
	if ext.DND {
		return &flow.RingResult{DND: true}, nil
	}

	// Look up active registrations for the extension.
	regs, err := a.registrations.GetByExtensionID(ctx, ext.ID)
	if err != nil {
		return nil, fmt.Errorf("looking up registrations for extension %s: %w", ext.Extension, err)
	}

	// Filter expired registrations.
	now := time.Now()
	active := make([]models.Registration, 0, len(regs))
	for _, reg := range regs {
		if reg.Expires.After(now) {
			active = append(active, reg)
		}
	}

	if len(active) == 0 {
		return &flow.RingResult{NoRegistrations: true}, nil
	}

	// Check if all devices are already busy.
	if a.dialogMgr != nil {
		activeCalls := a.dialogMgr.ActiveCallCountForExtension(ext.ID)
		if activeCalls > 0 && activeCalls >= len(active) {
			return &flow.RingResult{AllBusy: true}, nil
		}
	}

	req := callCtx.Request
	tx := callCtx.Transaction
	callID := callCtx.CallID

	a.logger.Info("ringing extension via flow",
		"call_id", callID,
		"extension", ext.Extension,
		"contacts", len(active),
		"ring_timeout", ringTimeout,
	)

	// Allocate media bridge for RTP proxying.
	var bridge *MediaBridge
	var calleeSDP []byte
	if len(req.Body()) > 0 && a.sessionMgr != nil {
		var err error
		bridge, calleeSDP, err = AllocateMediaBridge(a.sessionMgr, req.Body(), callID, a.proxyIP, a.logger)
		if err != nil {
			a.logger.Error("failed to allocate media bridge for flow ring",
				"call_id", callID,
				"error", err,
			)
			return nil, fmt.Errorf("allocating media bridge: %w", err)
		}
	}

	// Set ring timeout.
	ringDuration := time.Duration(ringTimeout) * time.Second
	forkCtx, cancelFork := context.WithTimeout(ctx, ringDuration)

	// Register as pending so the CANCEL handler can find it.
	a.pendingMgr.Add(&PendingCall{
		CallID:     callID,
		CallerTx:   tx,
		CallerReq:  req,
		CancelFork: cancelFork,
		Bridge:     bridge,
	})

	// Fork INVITE to all registered contacts.
	result := a.forker.Fork(forkCtx, req, tx, active, nil, callID, calleeSDP)

	// Remove from pending calls.
	pc := a.pendingMgr.Remove(callID)
	cancelFork()

	// If the pending call was already cancelled by the CANCEL handler,
	// the fork result doesn't matter.
	if pc == nil {
		a.logger.Info("flow ring completed but call was already cancelled",
			"call_id", callID,
		)
		if result.Answered && result.AnsweringTx != nil {
			result.AnsweringTx.Terminate()
		}
		return &flow.RingResult{}, fmt.Errorf("call cancelled during ringing")
	}

	if result.Error != nil {
		if bridge != nil {
			bridge.Release()
		}
		return nil, fmt.Errorf("forking to extension %s: %w", ext.Extension, result.Error)
	}

	if result.AllBusy {
		if bridge != nil {
			bridge.Release()
		}
		return &flow.RingResult{AllBusy: true}, nil
	}

	if !result.Answered {
		if bridge != nil {
			bridge.Release()
		}
		return &flow.RingResult{Answered: false}, nil
	}

	// A device answered — complete media bridging and relay the 200 OK.
	a.logger.Info("extension answered via flow",
		"call_id", callID,
		"extension", ext.Extension,
		"contact", result.AnsweringContact.ContactURI,
	)

	// Send ACK to the answering callee device.
	ackReq := buildACKFor2xx(result.AnsweringLeg.req, result.AnswerResponse)
	if err := a.forker.Client().WriteRequest(ackReq); err != nil {
		a.logger.Error("failed to send ack to callee via flow",
			"call_id", callID,
			"contact", result.AnsweringContact.ContactURI,
			"error", err,
		)
		result.AnsweringTx.Terminate()
		if bridge != nil {
			bridge.Release()
		}
		return nil, fmt.Errorf("sending ack to callee: %w", err)
	}

	// Complete media bridge with callee's SDP.
	var mediaSession *media.MediaSession
	okBody := result.AnswerResponse.Body()
	if bridge != nil && len(result.AnswerResponse.Body()) > 0 {
		rewrittenForCaller, err := bridge.CompleteMediaBridge(result.AnswerResponse.Body())
		if err != nil {
			a.logger.Error("failed to complete media bridge via flow",
				"call_id", callID,
				"error", err,
			)
		} else {
			okBody = rewrittenForCaller
			mediaSession = bridge.Session()
		}
	}

	// Forward the 200 OK to the caller (trunk or originating extension).
	okResponse := sip.NewResponseFromRequest(req, 200, "OK", okBody)
	if len(okBody) > 0 {
		okResponse.AppendHeader(sip.NewHeader("Content-Type", "application/sdp"))
	}

	if err := tx.Respond(okResponse); err != nil {
		a.logger.Error("failed to relay 200 ok to caller via flow",
			"call_id", callID,
			"error", err,
		)
		result.AnsweringTx.Terminate()
		if mediaSession != nil {
			mediaSession.Release()
		}
		return nil, fmt.Errorf("relaying 200 ok: %w", err)
	}

	// Track the active call as a dialog.
	dialog := &Dialog{
		CallID:       callID,
		Direction:    CallTypeInbound,
		TrunkID:      callCtx.TrunkID,
		CallerIDName: callCtx.CallerIDName,
		CallerIDNum:  callCtx.CallerIDNum,
		CalledNum:    ext.Extension,
		StartTime:    callCtx.StartTime,
		CallerTx:     tx,
		CallerReq:    req,
		CalleeTx:     result.AnsweringTx,
		CalleeReq:    result.AnsweringLeg.req,
		CalleeRes:    result.AnswerResponse,
		Media:        mediaSession,
		Caller:       CallLeg{},
		Callee: CallLeg{
			Extension:    ext,
			Registration: result.AnsweringContact,
			ContactURI:   result.AnsweringContact.ContactURI,
		},
	}

	// Extract dialog tags.
	if from := req.From(); from != nil {
		if tag, ok := from.Params.Get("tag"); ok {
			dialog.Caller.FromTag = tag
		}
	}
	if to := result.AnswerResponse.To(); to != nil {
		if tag, ok := to.Params.Get("tag"); ok {
			dialog.Callee.ToTag = tag
		}
	}

	// Extract callee remote target from Contact header in 200 OK.
	if contact := result.AnswerResponse.Contact(); contact != nil {
		uri := contact.Address.Clone()
		dialog.Callee.RemoteTarget = uri
	}

	a.dialogMgr.CreateDialog(dialog)

	// Update CDR with answer time.
	a.updateCDROnAnswer(callID)

	a.logger.Info("flow call dialog established",
		"call_id", callID,
		"extension", ext.Extension,
		"active_calls", a.dialogMgr.ActiveCallCount(),
		"media_bridged", mediaSession != nil,
	)

	return &flow.RingResult{Answered: true}, nil
}

// RingGroup rings multiple extensions simultaneously (ring_all strategy).
// It gathers all active registrations across the provided extensions, checks
// DND and busy status, then forks INVITE to all available contacts at once.
func (a *FlowSIPActions) RingGroup(ctx context.Context, callCtx *flow.CallContext, extensions []*models.Extension, ringTimeout int) (*flow.RingResult, error) {
	if callCtx.Request == nil || callCtx.Transaction == nil {
		return nil, fmt.Errorf("call context has no sip request or transaction")
	}

	if len(extensions) == 0 {
		return &flow.RingResult{NoRegistrations: true}, nil
	}

	callID := callCtx.CallID
	now := time.Now()

	// Gather active registrations from all non-DND, non-busy extensions.
	var allContacts []models.Registration
	dndCount := 0
	busyCount := 0

	for _, ext := range extensions {
		if ext.DND {
			dndCount++
			a.logger.Debug("ring group member has dnd enabled",
				"call_id", callID,
				"extension", ext.Extension,
			)
			continue
		}

		regs, err := a.registrations.GetByExtensionID(ctx, ext.ID)
		if err != nil {
			a.logger.Error("failed to get registrations for ring group member",
				"call_id", callID,
				"extension", ext.Extension,
				"error", err,
			)
			continue
		}

		// Filter expired registrations.
		active := make([]models.Registration, 0, len(regs))
		for _, reg := range regs {
			if reg.Expires.After(now) {
				active = append(active, reg)
			}
		}

		if len(active) == 0 {
			continue
		}

		// Check if all devices for this extension are busy.
		if a.dialogMgr != nil {
			activeCalls := a.dialogMgr.ActiveCallCountForExtension(ext.ID)
			if activeCalls > 0 && activeCalls >= len(active) {
				busyCount++
				a.logger.Debug("ring group member is busy",
					"call_id", callID,
					"extension", ext.Extension,
				)
				continue
			}
		}

		allContacts = append(allContacts, active...)
	}

	if len(allContacts) == 0 {
		if busyCount > 0 {
			return &flow.RingResult{AllBusy: true}, nil
		}
		return &flow.RingResult{NoRegistrations: true}, nil
	}

	req := callCtx.Request
	tx := callCtx.Transaction

	a.logger.Info("ringing group via flow",
		"call_id", callID,
		"members", len(extensions),
		"contacts", len(allContacts),
		"ring_timeout", ringTimeout,
	)

	// Allocate media bridge for RTP proxying.
	var bridge *MediaBridge
	var calleeSDP []byte
	if len(req.Body()) > 0 && a.sessionMgr != nil {
		var err error
		bridge, calleeSDP, err = AllocateMediaBridge(a.sessionMgr, req.Body(), callID, a.proxyIP, a.logger)
		if err != nil {
			a.logger.Error("failed to allocate media bridge for ring group",
				"call_id", callID,
				"error", err,
			)
			return nil, fmt.Errorf("allocating media bridge: %w", err)
		}
	}

	// Set ring timeout.
	ringDuration := time.Duration(ringTimeout) * time.Second
	forkCtx, cancelFork := context.WithTimeout(ctx, ringDuration)

	// Register as pending so the CANCEL handler can find it.
	a.pendingMgr.Add(&PendingCall{
		CallID:     callID,
		CallerTx:   tx,
		CallerReq:  req,
		CancelFork: cancelFork,
		Bridge:     bridge,
	})

	// Fork INVITE to all registered contacts across all member extensions.
	result := a.forker.Fork(forkCtx, req, tx, allContacts, nil, callID, calleeSDP)

	// Remove from pending calls.
	pc := a.pendingMgr.Remove(callID)
	cancelFork()

	if pc == nil {
		a.logger.Info("ring group completed but call was already cancelled",
			"call_id", callID,
		)
		if result.Answered && result.AnsweringTx != nil {
			result.AnsweringTx.Terminate()
		}
		return &flow.RingResult{}, fmt.Errorf("call cancelled during ringing")
	}

	if result.Error != nil {
		if bridge != nil {
			bridge.Release()
		}
		return nil, fmt.Errorf("forking to ring group: %w", result.Error)
	}

	if result.AllBusy {
		if bridge != nil {
			bridge.Release()
		}
		return &flow.RingResult{AllBusy: true}, nil
	}

	if !result.Answered {
		if bridge != nil {
			bridge.Release()
		}
		return &flow.RingResult{Answered: false}, nil
	}

	// A device answered — complete media bridging and relay the 200 OK.
	a.logger.Info("ring group member answered via flow",
		"call_id", callID,
		"contact", result.AnsweringContact.ContactURI,
	)

	// Send ACK to the answering callee device.
	ackReq := buildACKFor2xx(result.AnsweringLeg.req, result.AnswerResponse)
	if err := a.forker.Client().WriteRequest(ackReq); err != nil {
		a.logger.Error("failed to send ack to callee via ring group",
			"call_id", callID,
			"contact", result.AnsweringContact.ContactURI,
			"error", err,
		)
		result.AnsweringTx.Terminate()
		if bridge != nil {
			bridge.Release()
		}
		return nil, fmt.Errorf("sending ack to callee: %w", err)
	}

	// Complete media bridge with callee's SDP.
	var mediaSession *media.MediaSession
	okBody := result.AnswerResponse.Body()
	if bridge != nil && len(result.AnswerResponse.Body()) > 0 {
		rewrittenForCaller, err := bridge.CompleteMediaBridge(result.AnswerResponse.Body())
		if err != nil {
			a.logger.Error("failed to complete media bridge via ring group",
				"call_id", callID,
				"error", err,
			)
		} else {
			okBody = rewrittenForCaller
			mediaSession = bridge.Session()
		}
	}

	// Forward the 200 OK to the caller.
	okResponse := sip.NewResponseFromRequest(req, 200, "OK", okBody)
	if len(okBody) > 0 {
		okResponse.AppendHeader(sip.NewHeader("Content-Type", "application/sdp"))
	}

	if err := tx.Respond(okResponse); err != nil {
		a.logger.Error("failed to relay 200 ok to caller via ring group",
			"call_id", callID,
			"error", err,
		)
		result.AnsweringTx.Terminate()
		if mediaSession != nil {
			mediaSession.Release()
		}
		return nil, fmt.Errorf("relaying 200 ok: %w", err)
	}

	// Determine which extension answered based on registration.
	var answeredExt *models.Extension
	if result.AnsweringContact != nil && result.AnsweringContact.ExtensionID != nil {
		for _, ext := range extensions {
			if ext.ID == *result.AnsweringContact.ExtensionID {
				answeredExt = ext
				break
			}
		}
	}

	calledNum := ""
	if answeredExt != nil {
		calledNum = answeredExt.Extension
	}

	// Track the active call as a dialog.
	dialog := &Dialog{
		CallID:       callID,
		Direction:    CallTypeInbound,
		TrunkID:      callCtx.TrunkID,
		CallerIDName: callCtx.CallerIDName,
		CallerIDNum:  callCtx.CallerIDNum,
		CalledNum:    calledNum,
		StartTime:    callCtx.StartTime,
		CallerTx:     tx,
		CallerReq:    req,
		CalleeTx:     result.AnsweringTx,
		CalleeReq:    result.AnsweringLeg.req,
		CalleeRes:    result.AnswerResponse,
		Media:        mediaSession,
		Caller:       CallLeg{},
		Callee: CallLeg{
			Extension:    answeredExt,
			Registration: result.AnsweringContact,
			ContactURI:   result.AnsweringContact.ContactURI,
		},
	}

	// Extract dialog tags.
	if from := req.From(); from != nil {
		if tag, ok := from.Params.Get("tag"); ok {
			dialog.Caller.FromTag = tag
		}
	}
	if to := result.AnswerResponse.To(); to != nil {
		if tag, ok := to.Params.Get("tag"); ok {
			dialog.Callee.ToTag = tag
		}
	}

	// Extract callee remote target from Contact header in 200 OK.
	if contact := result.AnswerResponse.Contact(); contact != nil {
		uri := contact.Address.Clone()
		dialog.Callee.RemoteTarget = uri
	}

	a.dialogMgr.CreateDialog(dialog)

	// Update CDR with answer time.
	a.updateCDROnAnswer(callID)

	a.logger.Info("ring group call dialog established",
		"call_id", callID,
		"answered_by", calledNum,
		"active_calls", a.dialogMgr.ActiveCallCount(),
		"media_bridged", mediaSession != nil,
	)

	return &flow.RingResult{Answered: true}, nil
}

// PlayAndCollect plays an audio prompt and collects DTMF digits from the caller.
// This is used by the IVR menu handler for interactive voice menus.
//
// TODO(sprint-14): Full media implementation — play audio file via RTP, listen
// for RFC 2833 DTMF events. Current implementation collects DTMF from the
// call context buffer with timing logic.
func (a *FlowSIPActions) PlayAndCollect(ctx context.Context, callCtx *flow.CallContext, prompt string, isTTS bool, timeout int, digitTimeout int, maxDigits int) (*flow.CollectResult, error) {
	callID := callCtx.CallID
	a.logger.Info("play and collect starting",
		"call_id", callID,
		"prompt", prompt,
		"is_tts", isTTS,
		"timeout", timeout,
		"digit_timeout", digitTimeout,
		"max_digits", maxDigits,
	)

	// Clear any previously collected DTMF before starting collection.
	callCtx.ClearDTMF()

	if maxDigits <= 0 {
		maxDigits = 1
	}

	overallTimeout := time.Duration(timeout) * time.Second
	interDigitTimeout := time.Duration(digitTimeout) * time.Second

	// Wait for first digit with overall timeout.
	timer := time.NewTimer(overallTimeout)
	defer timer.Stop()

	collected := ""
	for {
		select {
		case <-ctx.Done():
			return &flow.CollectResult{Digits: collected, TimedOut: true}, nil
		case <-timer.C:
			if collected == "" {
				return &flow.CollectResult{TimedOut: true}, nil
			}
			return &flow.CollectResult{Digits: collected}, nil
		default:
			digits := callCtx.GetDTMF()
			if len(digits) > len(collected) {
				collected = digits
				// Check for terminator (#).
				if len(collected) > 0 && collected[len(collected)-1] == '#' {
					collected = collected[:len(collected)-1]
					return &flow.CollectResult{Digits: collected}, nil
				}
				if len(collected) >= maxDigits {
					return &flow.CollectResult{Digits: collected}, nil
				}
				// Reset timer for inter-digit timeout.
				timer.Reset(interDigitTimeout)
			}
			// Small sleep to avoid busy-waiting.
			time.Sleep(50 * time.Millisecond)
		}
	}
}

// updateCDROnAnswer updates the CDR with the answer time.
func (a *FlowSIPActions) updateCDROnAnswer(callID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cdr, err := a.cdrs.GetByCallID(ctx, callID)
	if err != nil {
		a.logger.Error("failed to fetch cdr for answer update",
			"call_id", callID,
			"error", err,
		)
		return
	}
	if cdr == nil {
		return
	}

	now := time.Now()
	cdr.AnswerTime = &now

	if err := a.cdrs.Update(ctx, cdr); err != nil {
		a.logger.Error("failed to update cdr on answer",
			"call_id", callID,
			"error", err,
		)
	}
}

// Ensure FlowSIPActions satisfies the flow.SIPActions interface.
var _ flow.SIPActions = (*FlowSIPActions)(nil)
