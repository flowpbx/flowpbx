package sip

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/emiago/sipgo"
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
	dtmfMgr       *media.CallDTMFManager
	conferenceMgr *media.ConferenceManager
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
	dtmfMgr *media.CallDTMFManager,
	conferenceMgr *media.ConferenceManager,
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
		dtmfMgr:       dtmfMgr,
		conferenceMgr: conferenceMgr,
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
// It acquires a per-call DTMF buffer from the CallDTMFManager, configures a
// DigitBuffer with the requested timing, and blocks until collection completes.
// DTMF digits arrive from both SIP INFO (injected by handleInfo) and RFC 2833
// (injected by the DTMFCollector bridge).
//
// TODO(sprint-14): Full media implementation — play audio file via RTP while
// collecting. Current implementation collects DTMF without playing audio.
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

	// Acquire a per-call digit channel from the DTMF manager.
	// This channel receives digits from both SIP INFO and RFC 2833 sources.
	digitCh := a.dtmfMgr.Acquire(callID)
	defer a.dtmfMgr.Release(callID)

	// Configure the digit buffer with the requested timing parameters.
	buf := media.NewDigitBuffer(digitCh, a.logger)
	buf.SetFirstDigitTimeout(time.Duration(timeout) * time.Second)
	buf.SetInterDigitTimeout(time.Duration(digitTimeout) * time.Second)
	buf.SetMaxDigits(maxDigits)
	buf.SetTerminator("#")

	// Block until collection completes (max digits, terminator, timeout, or cancel).
	result := buf.Collect(ctx)

	a.logger.Info("play and collect completed",
		"call_id", callID,
		"digits", result.Digits,
		"timed_out", result.TimedOut,
		"terminated", result.Terminated,
	)

	return &flow.CollectResult{
		Digits:   result.Digits,
		TimedOut: result.TimedOut,
	}, nil
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

// RecordMessage plays a greeting prompt and records the caller's message.
//
// TODO(sprint-14): Full media implementation — play greeting via RTP, then
// record incoming RTP to WAV file. Current implementation creates an empty
// recording file and returns immediately.
func (a *FlowSIPActions) RecordMessage(ctx context.Context, callCtx *flow.CallContext, greeting string, maxDuration int, filePath string) (*flow.RecordResult, error) {
	callID := callCtx.CallID
	a.logger.Info("record message starting",
		"call_id", callID,
		"greeting", greeting,
		"max_duration", maxDuration,
		"file_path", filePath,
	)

	// Stub: create an empty file as a placeholder for the recording.
	// The full RTP recording implementation will be added in the media sprint.
	f, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("creating recording file: %w", err)
	}
	f.Close()

	a.logger.Info("record message completed (stub)",
		"call_id", callID,
		"file_path", filePath,
	)

	return &flow.RecordResult{
		FilePath:     filePath,
		DurationSecs: 0,
	}, nil
}

// SendMWI sends a SIP NOTIFY to all registered devices for the specified
// extension to update the Message Waiting Indicator (voicemail lamp). The
// NOTIFY carries an Event: message-summary header and an RFC 3842 body
// with the voice-message counts.
func (a *FlowSIPActions) SendMWI(ctx context.Context, ext *models.Extension, newMessages int, oldMessages int) error {
	a.logger.Info("sending MWI notification",
		"extension", ext.Extension,
		"new_messages", newMessages,
		"old_messages", oldMessages,
	)

	// Look up active registrations for the extension.
	regs, err := a.registrations.GetByExtensionID(ctx, ext.ID)
	if err != nil {
		return fmt.Errorf("looking up registrations for MWI: %w", err)
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
		a.logger.Debug("no active registrations for MWI, skipping",
			"extension", ext.Extension,
		)
		return nil
	}

	// Build the RFC 3842 message-summary body.
	waiting := "no"
	if newMessages > 0 {
		waiting = "yes"
	}
	body := fmt.Sprintf("Messages-Waiting: %s\r\nMessage-Account: sip:%s@%s\r\nVoice-Message: %d/%d (%d new, %d old)\r\n",
		waiting,
		ext.Extension,
		a.proxyIP,
		newMessages, oldMessages,
		newMessages, oldMessages,
	)

	// Send NOTIFY to each active registration.
	var lastErr error
	for i := range active {
		if err := a.sendMWINotify(&active[i], ext, body); err != nil {
			a.logger.Error("failed to send MWI NOTIFY",
				"extension", ext.Extension,
				"contact", active[i].ContactURI,
				"error", err,
			)
			lastErr = err
			continue
		}
		a.logger.Debug("MWI NOTIFY sent",
			"extension", ext.Extension,
			"contact", active[i].ContactURI,
		)
	}

	return lastErr
}

// sendMWINotify builds and sends a single SIP NOTIFY request with
// message-summary event to one registered contact.
func (a *FlowSIPActions) sendMWINotify(reg *models.Registration, ext *models.Extension, body string) error {
	// Parse the contact URI as the Request-URI.
	var recipient sip.Uri
	if err := sip.ParseUri(reg.ContactURI, &recipient); err != nil {
		return fmt.Errorf("parsing contact uri %q: %w", reg.ContactURI, err)
	}

	// NAT traversal: use the source IP:port from the registration.
	if reg.SourceIP != "" && reg.SourcePort > 0 {
		recipient.Host = reg.SourceIP
		recipient.Port = reg.SourcePort
	}

	req := sip.NewRequest(sip.NOTIFY, recipient)
	req.SetTransport(transportForContact(reg))

	// Event header per RFC 3842.
	req.AppendHeader(sip.NewHeader("Event", "message-summary"))

	// Subscription-State: we send unsolicited NOTIFY (no SUBSCRIBE dialog).
	req.AppendHeader(sip.NewHeader("Subscription-State", "active"))

	// Content-Type for the message-summary body.
	req.AppendHeader(sip.NewHeader("Content-Type", "application/simple-message-summary"))

	// Max-Forwards.
	maxFwd := sip.MaxForwardsHeader(70)
	req.AppendHeader(&maxFwd)

	// Set the body.
	req.SetBody([]byte(body))

	// Send via a client transaction so we get a proper response.
	tx, err := a.forker.Client().TransactionRequest(context.Background(), req, sipgo.ClientRequestBuild)
	if err != nil {
		return fmt.Errorf("sending MWI NOTIFY to %s: %w", reg.ContactURI, err)
	}

	// Wait briefly for the response but don't block the caller indefinitely.
	select {
	case res := <-tx.Responses():
		if res.StatusCode >= 300 {
			a.logger.Warn("MWI NOTIFY rejected",
				"contact", reg.ContactURI,
				"status", res.StatusCode,
				"reason", res.Reason,
			)
		}
	case <-tx.Done():
		if err := tx.Err(); err != nil {
			return fmt.Errorf("MWI NOTIFY transaction failed for %s: %w", reg.ContactURI, err)
		}
	case <-time.After(5 * time.Second):
		tx.Terminate()
		a.logger.Warn("MWI NOTIFY timed out",
			"contact", reg.ContactURI,
		)
	}

	return nil
}

// HangupCall terminates the call with the given SIP cause code and reason.
// For answered calls (active dialog), it sends BYE. For unanswered calls,
// it sends an error response on the server transaction.
//
// TODO(sprint-14): Full implementation — look up active dialog, send BYE to
// both legs, release media. Current implementation sends the response code
// on the original server transaction.
func (a *FlowSIPActions) HangupCall(ctx context.Context, callCtx *flow.CallContext, cause int, reason string) error {
	callID := callCtx.CallID
	a.logger.Info("hanging up call",
		"call_id", callID,
		"cause", cause,
		"reason", reason,
	)

	// Check for active dialog first.
	if a.dialogMgr != nil {
		dialog := a.dialogMgr.GetDialog(callID)
		if dialog != nil {
			a.dialogMgr.TerminateDialog(callID, reason)
			a.logger.Info("terminated active dialog for hangup",
				"call_id", callID,
			)
			return nil
		}
	}

	// No active dialog — respond on the server transaction if possible.
	if callCtx.Request != nil && callCtx.Transaction != nil {
		res := sip.NewResponseFromRequest(callCtx.Request, cause, reason, nil)
		if err := callCtx.Transaction.Respond(res); err != nil {
			return fmt.Errorf("sending hangup response: %w", err)
		}
	}

	return nil
}

// BlindTransfer performs a blind (unattended) transfer by sending a SIP REFER
// to the caller's user agent, instructing it to send a new INVITE to the
// specified destination.
//
// TODO(sprint-14): Full REFER implementation — send SIP REFER with Refer-To
// header, handle NOTIFY subscription for transfer status. Current
// implementation logs the transfer and terminates the existing dialog.
func (a *FlowSIPActions) BlindTransfer(ctx context.Context, callCtx *flow.CallContext, destination string) error {
	callID := callCtx.CallID
	a.logger.Info("blind transfer (stub)",
		"call_id", callID,
		"destination", destination,
	)

	// Stub: log the transfer. The full SIP REFER implementation will be
	// added in the SIP signaling sprint.
	return nil
}

// conferencePINMaxAttempts is the maximum number of PIN entry attempts
// before rejecting a caller from a PIN-protected conference.
const conferencePINMaxAttempts = 3

// conferencePINMaxDigits is the maximum number of digits accepted for a
// conference PIN. PINs are terminated by "#" so callers can enter shorter
// PINs without waiting for the inter-digit timeout.
const conferencePINMaxDigits = 10

// conferencePINFirstDigitTimeout is the time to wait for the first PIN
// digit before treating it as a timeout (seconds).
const conferencePINFirstDigitTimeout = 10

// conferencePINInterDigitTimeout is the time to wait between consecutive
// PIN digits (seconds).
const conferencePINInterDigitTimeout = 5

// verifyConferencePIN prompts the caller for the conference PIN and
// validates it against the stored hash. Returns nil if the PIN is correct,
// or an error if the caller fails all attempts or the context is cancelled.
func (a *FlowSIPActions) verifyConferencePIN(ctx context.Context, callCtx *flow.CallContext, pinHash string) error {
	callID := callCtx.CallID

	a.logger.Info("conference pin required, collecting digits",
		"call_id", callID,
	)

	for attempt := 1; attempt <= conferencePINMaxAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		a.logger.Debug("conference pin attempt",
			"call_id", callID,
			"attempt", attempt,
			"max_attempts", conferencePINMaxAttempts,
		)

		// Clear any stale DTMF from previous nodes or attempts.
		callCtx.ClearDTMF()

		// Collect PIN digits using the standard PlayAndCollect mechanism.
		// The prompt is a placeholder — audio playback is not yet
		// implemented (TODO sprint-14), but DTMF collection via SIP INFO
		// and RFC 2833 works.
		result, err := a.PlayAndCollect(ctx, callCtx, "conf-pin-entry", false,
			conferencePINFirstDigitTimeout, conferencePINInterDigitTimeout, conferencePINMaxDigits)
		if err != nil {
			return fmt.Errorf("collecting conference pin: %w", err)
		}

		if result.TimedOut && result.Digits == "" {
			a.logger.Debug("conference pin timeout, no digits received",
				"call_id", callID,
				"attempt", attempt,
			)
			if attempt >= conferencePINMaxAttempts {
				a.logger.Info("conference pin max attempts exhausted (timeout)",
					"call_id", callID,
				)
				return fmt.Errorf("conference pin: max attempts exhausted")
			}
			continue
		}

		if result.Digits == "" {
			continue
		}

		// Verify the entered PIN against the stored Argon2id hash.
		match, err := database.CheckPassword(result.Digits, pinHash)
		if err != nil {
			a.logger.Error("conference pin verification error",
				"call_id", callID,
				"error", err,
			)
			return fmt.Errorf("verifying conference pin: %w", err)
		}

		if match {
			a.logger.Info("conference pin accepted",
				"call_id", callID,
				"attempt", attempt,
			)
			return nil
		}

		a.logger.Debug("conference pin rejected",
			"call_id", callID,
			"attempt", attempt,
		)

		if attempt >= conferencePINMaxAttempts {
			a.logger.Info("conference pin max attempts exhausted (invalid)",
				"call_id", callID,
			)
			return fmt.Errorf("conference pin: max attempts exhausted")
		}
	}

	return fmt.Errorf("conference pin: max attempts exhausted")
}

// JoinConference joins the caller into the specified conference bridge.
// This blocks until the caller leaves the conference (hang up or context
// cancellation). The caller's RTP stream is connected to the conference
// mixer so they can hear and be heard by all other participants.
// If the bridge has a PIN configured, the caller is prompted to enter it
// before being admitted. Up to 3 attempts are allowed.
func (a *FlowSIPActions) JoinConference(ctx context.Context, callCtx *flow.CallContext, bridge *models.ConferenceBridge) error {
	callID := callCtx.CallID

	if callCtx.Request == nil || callCtx.Transaction == nil {
		return fmt.Errorf("call context has no sip request or transaction")
	}

	if a.conferenceMgr == nil {
		return fmt.Errorf("conference manager not available")
	}

	a.logger.Info("joining conference",
		"call_id", callID,
		"conference", bridge.Name,
		"conference_id", bridge.ID,
		"max_members", bridge.MaxMembers,
		"has_pin", bridge.PIN != "",
	)

	// If the bridge has a PIN, verify it before allowing entry.
	if bridge.PIN != "" {
		if err := a.verifyConferencePIN(ctx, callCtx, bridge.PIN); err != nil {
			return fmt.Errorf("conference pin verification failed: %w", err)
		}
	}

	// Parse the caller's SDP to extract their RTP address and codec.
	sdpBody := callCtx.Request.Body()
	if len(sdpBody) == 0 {
		return fmt.Errorf("caller has no SDP body")
	}

	callerSD, err := media.ParseSDP(sdpBody)
	if err != nil {
		return fmt.Errorf("parsing caller sdp: %w", err)
	}

	callerAudio := callerSD.AudioMedia()
	if callerAudio == nil {
		return fmt.Errorf("caller sdp has no audio media")
	}

	// Determine the caller's RTP address from SDP.
	callerIP := callerSD.ConnectionAddress(callerAudio)
	if callerIP == "" {
		return fmt.Errorf("no connection address in caller sdp")
	}

	callerRemote := &net.UDPAddr{
		IP:   net.ParseIP(callerIP),
		Port: callerAudio.Port,
	}

	// Negotiate a G.711 codec (PCMU or PCMA) from the caller's SDP.
	// The mixer only supports G.711 codecs.
	payloadType := -1
	for _, pt := range callerAudio.Formats {
		if pt == media.PayloadPCMU || pt == media.PayloadPCMA {
			payloadType = pt
			break
		}
	}
	if payloadType < 0 {
		return fmt.Errorf("no supported G.711 codec in caller sdp (need PCMU or PCMA)")
	}

	// Add participant to the conference room via ConferenceManager.
	joinResult, err := a.conferenceMgr.Join(ctx, bridge.ID, bridge.Name, bridge.MaxMembers, bridge.AnnounceJoins, bridge.Record, callID, callerRemote, payloadType)
	if err != nil {
		return fmt.Errorf("joining conference room: %w", err)
	}

	// Rewrite the caller's SDP so RTP flows to the mixer's allocated port.
	rewrittenSDP, err := media.RewriteSDPBytes(sdpBody, a.proxyIP, joinResult.Port)
	if err != nil {
		a.conferenceMgr.Leave(bridge.ID, callID)
		return fmt.Errorf("rewriting sdp for conference: %w", err)
	}

	// Apply mute-on-join if configured.
	if bridge.MuteOnJoin {
		a.conferenceMgr.MuteParticipant(bridge.ID, callID, true)
	}

	// Send 200 OK to the caller with the rewritten SDP pointing to the mixer.
	okResponse := sip.NewResponseFromRequest(callCtx.Request, 200, "OK", rewrittenSDP)
	okResponse.AppendHeader(sip.NewHeader("Content-Type", "application/sdp"))

	if err := callCtx.Transaction.Respond(okResponse); err != nil {
		a.conferenceMgr.Leave(bridge.ID, callID)
		return fmt.Errorf("sending 200 ok for conference: %w", err)
	}

	a.logger.Info("participant connected to conference",
		"call_id", callID,
		"conference", bridge.Name,
		"conference_id", bridge.ID,
		"mixer_port", joinResult.Port,
		"payload_type", payloadType,
		"muted", bridge.MuteOnJoin,
	)

	// Block until the caller hangs up (context cancelled) or we are shut down.
	<-ctx.Done()

	// Remove participant from the conference on exit.
	if err := a.conferenceMgr.Leave(bridge.ID, callID); err != nil {
		a.logger.Warn("error leaving conference",
			"call_id", callID,
			"conference", bridge.Name,
			"error", err,
		)
	}

	a.logger.Info("participant left conference",
		"call_id", callID,
		"conference", bridge.Name,
		"conference_id", bridge.ID,
	)

	return nil
}

// Ensure FlowSIPActions satisfies the flow.SIPActions interface.
var _ flow.SIPActions = (*FlowSIPActions)(nil)
