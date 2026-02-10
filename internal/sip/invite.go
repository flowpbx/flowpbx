package sip

import (
	"context"
	"log/slog"
	"net"
	"time"

	"github.com/emiago/sipgo/sip"
	"github.com/flowpbx/flowpbx/internal/database"
	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/flowpbx/flowpbx/internal/flow"
	"github.com/flowpbx/flowpbx/internal/media"
)

// CallType identifies the direction/nature of a call.
type CallType string

const (
	// CallTypeInternal is an extension-to-extension call within the PBX.
	CallTypeInternal CallType = "internal"
	// CallTypeInbound is a call arriving from an external trunk to a DID/extension.
	CallTypeInbound CallType = "inbound"
	// CallTypeOutbound is a call from a local extension to an external number via trunk.
	CallTypeOutbound CallType = "outbound"
)

// InviteContext holds the classified information about an incoming INVITE.
type InviteContext struct {
	CallType CallType

	// CallerExtension is set when the caller is a local extension (internal/outbound).
	CallerExtension *models.Extension

	// TrunkID is set when the call arrives from a trunk (inbound).
	TrunkID int64

	// TargetExtension is set when the call target is a local extension (internal/inbound).
	TargetExtension *models.Extension

	// InboundNumber is set when an inbound call matches a DID.
	InboundNumber *models.InboundNumber

	// RequestURI is the user part of the Request-URI (the dialed number/extension).
	RequestURI string

	// CallerID is the display info from the From header.
	CallerIDName string
	CallerIDNum  string
}

// InviteHandler processes incoming SIP INVITE requests, classifying them by
// call type and dispatching to the appropriate handler.
type InviteHandler struct {
	extensions     database.ExtensionRepository
	registrations  database.RegistrationRepository
	inboundNumbers database.InboundNumberRepository
	trunks         database.TrunkRepository
	trunkRegistrar *TrunkRegistrar
	auth           *Authenticator
	router         *CallRouter
	outboundRouter *OutboundRouter
	forker         *Forker
	dialogMgr      *DialogManager
	pendingMgr     *PendingCallManager
	sessionMgr     *media.SessionManager
	cdrs           database.CDRRepository
	systemConfig   database.SystemConfigRepository
	flowEngine     *flow.Engine
	flowActions    *FlowSIPActions
	proxyIP        string
	dataDir        string
	logger         *slog.Logger
}

// NewInviteHandler creates a new INVITE request handler.
func NewInviteHandler(
	extensions database.ExtensionRepository,
	registrations database.RegistrationRepository,
	inboundNumbers database.InboundNumberRepository,
	trunks database.TrunkRepository,
	trunkRegistrar *TrunkRegistrar,
	auth *Authenticator,
	outboundRouter *OutboundRouter,
	forker *Forker,
	dialogMgr *DialogManager,
	pendingMgr *PendingCallManager,
	sessionMgr *media.SessionManager,
	cdrs database.CDRRepository,
	sysConfig database.SystemConfigRepository,
	flowEngine *flow.Engine,
	flowActions *FlowSIPActions,
	proxyIP string,
	dataDir string,
	logger *slog.Logger,
) *InviteHandler {
	router := NewCallRouter(extensions, registrations, dialogMgr, logger)
	return &InviteHandler{
		extensions:     extensions,
		registrations:  registrations,
		inboundNumbers: inboundNumbers,
		trunks:         trunks,
		trunkRegistrar: trunkRegistrar,
		auth:           auth,
		router:         router,
		outboundRouter: outboundRouter,
		forker:         forker,
		dialogMgr:      dialogMgr,
		pendingMgr:     pendingMgr,
		sessionMgr:     sessionMgr,
		cdrs:           cdrs,
		systemConfig:   sysConfig,
		flowEngine:     flowEngine,
		flowActions:    flowActions,
		proxyIP:        proxyIP,
		dataDir:        dataDir,
		logger:         logger.With("subsystem", "invite"),
	}
}

// HandleInvite is the entry point for all INVITE requests.
func (h *InviteHandler) HandleInvite(req *sip.Request, tx sip.ServerTransaction) {
	callID := ""
	if cid := req.CallID(); cid != nil {
		callID = cid.Value()
	}

	h.logger.Info("invite received",
		"call_id", callID,
		"from", req.From().Address.User,
		"to", req.To().Address.User,
		"source", req.Source(),
	)

	// Send 100 Trying immediately to stop UAC retransmissions (RFC 3261 §8.2.6.1).
	trying := sip.NewResponseFromRequest(req, 100, "Trying", nil)
	if err := tx.Respond(trying); err != nil {
		h.logger.Error("failed to send 100 trying",
			"call_id", callID,
			"error", err,
		)
		return
	}

	// Classify the call type.
	ic, err := h.classifyCall(req, tx)
	if err != nil {
		h.logger.Error("failed to classify invite",
			"call_id", callID,
			"error", err,
		)
		h.respondError(req, tx, 500, "Internal Server Error")
		return
	}
	if ic == nil {
		// classifyCall already sent the SIP response (e.g. 401, 403, 404).
		return
	}

	h.logger.Info("invite classified",
		"call_id", callID,
		"call_type", ic.CallType,
		"request_uri", ic.RequestURI,
		"caller_name", ic.CallerIDName,
		"caller_num", ic.CallerIDNum,
		"trunk_id", ic.TrunkID,
	)

	// Create CDR at call start with initial fields.
	h.createInitialCDR(ic, callID)

	// Dispatch to call routing based on call type.
	switch ic.CallType {
	case CallTypeInternal:
		h.handleInternalCall(req, tx, ic, callID)
	case CallTypeInbound:
		h.handleInboundCall(req, tx, ic, callID)
	case CallTypeOutbound:
		h.handleOutboundCall(req, tx, ic, callID)
	default:
		h.logger.Error("unknown call type",
			"call_id", callID,
			"call_type", ic.CallType,
		)
		h.respondError(req, tx, 500, "Internal Server Error")
	}
}

// handleInternalCall routes an extension-to-extension call by looking up
// the target extension's active registrations via the CallRouter.
func (h *InviteHandler) handleInternalCall(req *sip.Request, tx sip.ServerTransaction, ic *InviteContext, callID string) {
	ctx := context.Background()

	route, err := h.router.RouteInternalCall(ctx, ic)
	if err != nil {
		switch err {
		case ErrDND:
			h.logger.Info("internal call rejected: dnd enabled",
				"call_id", callID,
				"target", ic.TargetExtension.Extension,
			)
			h.respondErrorWithCDR(req, tx, 486, "Busy Here", callID)
			return
		case ErrAllBusy:
			h.logger.Info("internal call rejected: all devices busy",
				"call_id", callID,
				"target", ic.TargetExtension.Extension,
			)
			h.respondErrorWithCDR(req, tx, 486, "Busy Here", callID)
			return
		case ErrNoRegistrations:
			h.logger.Info("internal call failed: no registrations",
				"call_id", callID,
				"target", ic.TargetExtension.Extension,
			)
			// Try follow-me if enabled (no registered devices at all).
			if h.tryFollowMe(ctx, req, tx, ic.TargetExtension, callID, ic.CallerIDName, ic.CallerIDNum) {
				return
			}
			h.respondErrorWithCDR(req, tx, 480, "Temporarily Unavailable", callID)
			return
		case ErrExtensionNotFound:
			h.logger.Info("internal call failed: extension not found",
				"call_id", callID,
				"request_uri", ic.RequestURI,
			)
			h.respondErrorWithCDR(req, tx, 404, "Not Found", callID)
			return
		default:
			h.logger.Error("internal call routing error",
				"call_id", callID,
				"error", err,
			)
			h.respondErrorWithCDR(req, tx, 500, "Internal Server Error", callID)
			return
		}
	}

	h.logger.Info("internal call routed, forking to contacts",
		"call_id", callID,
		"target", route.TargetExtension.Extension,
		"contacts", len(route.Contacts),
	)

	// Phase 1 of media bridging: allocate RTP session and rewrite the caller's
	// SDP so forked INVITEs direct the callee's RTP to the proxy.
	var bridge *MediaBridge
	var calleeSDP []byte
	if len(req.Body()) > 0 && h.sessionMgr != nil {
		var err error
		bridge, calleeSDP, err = AllocateMediaBridge(h.sessionMgr, req.Body(), callID, h.proxyIP, h.logger)
		if err != nil {
			h.logger.Error("failed to allocate media bridge",
				"call_id", callID,
				"error", err,
			)
			h.respondErrorWithCDR(req, tx, 500, "Internal Server Error", callID)
			return
		}
	}

	// Determine ring timeout from the target extension's settings.
	// Default to 30 seconds if unset (0 or negative).
	ringTimeout := time.Duration(route.TargetExtension.RingTimeout) * time.Second
	if ringTimeout <= 0 {
		ringTimeout = 30 * time.Second
	}

	// Create a context with ring timeout for forking. The CANCEL handler can
	// also abort all fork legs by calling cancelFork().
	forkCtx, cancelFork := context.WithTimeout(ctx, ringTimeout)

	h.logger.Debug("forking with ring timeout",
		"call_id", callID,
		"ring_timeout_s", int(ringTimeout.Seconds()),
	)

	// Register this call as pending so the CANCEL handler can find it.
	h.pendingMgr.Add(&PendingCall{
		CallID:     callID,
		CallerTx:   tx,
		CallerReq:  req,
		CancelFork: cancelFork,
		Bridge:     bridge,
	})

	// Fork INVITE to all registered contacts (multi-device ringing).
	// Pass the rewritten SDP so callees send RTP to the proxy.
	result := h.forker.Fork(forkCtx, req, tx, route.Contacts, ic.CallerExtension, callID, calleeSDP)

	// Remove from pending calls now that forking is complete. If the CANCEL
	// handler already removed it (race), pc will be nil and that's fine —
	// it means the call was already cancelled and cleaned up.
	pc := h.pendingMgr.Remove(callID)
	cancelFork() // always clean up the context

	// If the pending call was already cancelled by the CANCEL handler,
	// the fork result doesn't matter — the caller already got 487.
	if pc == nil {
		h.logger.Info("fork completed but call was already cancelled",
			"call_id", callID,
		)
		// If a device answered despite the cancel, terminate that leg.
		if result.Answered && result.AnsweringTx != nil {
			result.AnsweringTx.Terminate()
		}
		return
	}

	if result.Error != nil {
		h.logger.Error("fork failed",
			"call_id", callID,
			"error", result.Error,
		)
		if bridge != nil {
			bridge.Release()
		}
		h.respondErrorWithCDR(req, tx, 500, "Internal Server Error", callID)
		return
	}

	if result.AllBusy {
		h.logger.Info("all devices busy",
			"call_id", callID,
			"target", route.TargetExtension.Extension,
		)
		if bridge != nil {
			bridge.Release()
		}
		h.respondErrorWithCDR(req, tx, 486, "Busy Here", callID)
		return
	}

	if !result.Answered {
		h.logger.Info("no device answered (ring timeout)",
			"call_id", callID,
			"target", route.TargetExtension.Extension,
			"ring_timeout_s", int(ringTimeout.Seconds()),
		)
		if bridge != nil {
			bridge.Release()
		}

		// Check if follow-me is enabled and try external numbers.
		if h.tryFollowMe(ctx, req, tx, route.TargetExtension, callID, ic.CallerIDName, ic.CallerIDNum) {
			return
		}

		// 480 Temporarily Unavailable — no device picked up within ring timeout.
		h.respondErrorWithCDR(req, tx, 480, "Temporarily Unavailable", callID)
		return
	}

	// A device answered — relay the 200 OK to the caller.
	h.logger.Info("call answered, relaying 200 ok",
		"call_id", callID,
		"target", route.TargetExtension.Extension,
		"contact", result.AnsweringContact.ContactURI,
	)

	// Send ACK to the answering callee device. Per RFC 3261 §13.2.2.4,
	// the ACK for a 2xx response is generated by the UAC core (not the
	// transaction layer) and sent directly via the transport.
	ackReq := buildACKFor2xx(result.AnsweringLeg.req, result.AnswerResponse)
	if err := h.forker.Client().WriteRequest(ackReq); err != nil {
		h.logger.Error("failed to send ack to callee",
			"call_id", callID,
			"contact", result.AnsweringContact.ContactURI,
			"error", err,
		)
		result.AnsweringTx.Terminate()
		if bridge != nil {
			bridge.Release()
		}
		h.respondErrorWithCDR(req, tx, 500, "Internal Server Error", callID)
		return
	}

	h.logger.Debug("ack sent to callee",
		"call_id", callID,
		"contact", result.AnsweringContact.ContactURI,
	)

	// Phase 2 of media bridging: negotiate codec, rewrite callee's SDP for the
	// caller, and start the RTP relay.
	var mediaSession *media.MediaSession
	okBody := result.AnswerResponse.Body()
	if bridge != nil && len(result.AnswerResponse.Body()) > 0 {
		rewrittenForCaller, err := bridge.CompleteMediaBridge(result.AnswerResponse.Body())
		if err != nil {
			h.logger.Error("failed to complete media bridge",
				"call_id", callID,
				"error", err,
			)
			// Fall back to direct media (SDP pass-through) — bridge already released.
		} else {
			okBody = rewrittenForCaller
			mediaSession = bridge.Session()
		}
	}

	// Forward the 200 OK from the callee back to the caller.
	// Use the (potentially rewritten) SDP body.
	okResponse := sip.NewResponseFromRequest(req, 200, "OK", okBody)
	if len(okBody) > 0 {
		okResponse.AppendHeader(sip.NewHeader("Content-Type", "application/sdp"))
	}

	if err := tx.Respond(okResponse); err != nil {
		h.logger.Error("failed to relay 200 ok to caller",
			"call_id", callID,
			"error", err,
		)
		// Clean up the answering leg since the caller won't get the 200 OK.
		result.AnsweringTx.Terminate()
		if mediaSession != nil {
			mediaSession.Release()
		}
		return
	}

	// Track the active call as a dialog for BYE/CANCEL handling and CDR.
	dialog := &Dialog{
		CallID:       callID,
		Direction:    ic.CallType,
		CallerIDName: ic.CallerIDName,
		CallerIDNum:  ic.CallerIDNum,
		CalledNum:    ic.RequestURI,
		StartTime:    time.Now(),
		CallerTx:     tx,
		CallerReq:    req,
		CalleeTx:     result.AnsweringTx,
		CalleeReq:    result.AnsweringLeg.req,
		CalleeRes:    result.AnswerResponse,
		Media:        mediaSession,
		Caller: CallLeg{
			Extension: ic.CallerExtension,
		},
		Callee: CallLeg{
			Extension:    ic.TargetExtension,
			Registration: result.AnsweringContact,
			ContactURI:   result.AnsweringContact.ContactURI,
		},
	}

	// Extract dialog tags from the SIP headers.
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

	// Start call recording based on global policy and per-extension recording_mode.
	if shouldRecord(h.globalRecordingPolicy(), ic.CallerExtension, ic.TargetExtension, nil) && mediaSession != nil {
		dialog.Recorder = h.startCallRecording(callID, mediaSession)
	}

	h.dialogMgr.CreateDialog(dialog)
	h.updateCDROnAnswer(callID, 0)

	h.logger.Info("call dialog established",
		"call_id", callID,
		"caller", ic.CallerIDNum,
		"callee", ic.RequestURI,
		"active_calls", h.dialogMgr.ActiveCallCount(),
		"media_bridged", mediaSession != nil,
		"recording", dialog.Recorder != nil,
	)
}

// handleInboundCall routes a call arriving from an external trunk to a local
// extension. This handles two sub-cases:
//
//  1. The Request-URI matched a local extension directly (ic.TargetExtension set).
//  2. The Request-URI matched an inbound_numbers DID entry (ic.InboundNumber set).
//     For now, until the flow engine is implemented, this returns 501.
//
// Caller ID is passed through from the trunk (From header values set during
// classifyInboundCall).
func (h *InviteHandler) handleInboundCall(req *sip.Request, tx sip.ServerTransaction, ic *InviteContext, callID string) {
	ctx := context.Background()

	// Look up the inbound trunk for channel limits and recording config.
	var inboundTrunk *models.Trunk
	if ic.TrunkID > 0 && h.trunks != nil {
		trunk, err := h.trunks.GetByID(ctx, ic.TrunkID)
		if err != nil {
			h.logger.Error("failed to look up inbound trunk",
				"call_id", callID,
				"trunk_id", ic.TrunkID,
				"error", err,
			)
			// Continue — don't block the call on a DB read error.
		} else {
			inboundTrunk = trunk
		}
	}

	// Enforce max_channels on the inbound trunk.
	if inboundTrunk != nil && inboundTrunk.MaxChannels > 0 {
		active := h.dialogMgr.ActiveCallCountForTrunk(ic.TrunkID)
		if active >= inboundTrunk.MaxChannels {
			h.logger.Warn("inbound call rejected: trunk at max channels",
				"call_id", callID,
				"trunk_id", ic.TrunkID,
				"active_channels", active,
				"max_channels", inboundTrunk.MaxChannels,
			)
			h.respondErrorWithCDR(req, tx, 503, "Service Unavailable", callID)
			return
		}
	}

	// If the inbound number matched a DID but we have no target extension,
	// that means the DID is mapped to a flow. Spawn the flow engine.
	if ic.TargetExtension == nil && ic.InboundNumber != nil {
		if ic.InboundNumber.FlowID == nil || h.flowEngine == nil {
			h.logger.Warn("inbound call matched did but no flow configured",
				"call_id", callID,
				"did", ic.InboundNumber.Number,
				"flow_id", ic.InboundNumber.FlowID,
			)
			h.respondErrorWithCDR(req, tx, 501, "Not Implemented", callID)
			return
		}

		h.logger.Info("inbound call entering flow engine",
			"call_id", callID,
			"did", ic.InboundNumber.Number,
			"flow_id", *ic.InboundNumber.FlowID,
			"entry_node", ic.InboundNumber.FlowEntryNode,
		)

		callCtx := flow.NewCallContext(
			callID,
			ic.CallerIDName,
			ic.CallerIDNum,
			ic.RequestURI,
			ic.InboundNumber,
			ic.TrunkID,
			req,
			tx,
		)

		// Spawn the flow walker goroutine so we don't block the SIP handler.
		go func() {
			if err := h.flowEngine.ExecuteFlow(callCtx, *ic.InboundNumber.FlowID, ic.InboundNumber.FlowEntryNode); err != nil {
				h.logger.Error("flow execution failed",
					"call_id", callID,
					"flow_id", *ic.InboundNumber.FlowID,
					"error", err,
				)
			}
		}()
		return
	}

	// No target extension and no InboundNumber — nowhere to route.
	if ic.TargetExtension == nil {
		h.logger.Info("inbound call has no matching destination",
			"call_id", callID,
			"request_uri", ic.RequestURI,
			"trunk_id", ic.TrunkID,
		)
		h.respondErrorWithCDR(req, tx, 404, "Not Found", callID)
		return
	}

	// Route to the target extension using the same logic as internal calls.
	route, err := h.router.RouteInternalCall(ctx, ic)
	if err != nil {
		switch err {
		case ErrDND:
			h.logger.Info("inbound call rejected: dnd enabled",
				"call_id", callID,
				"target", ic.TargetExtension.Extension,
			)
			h.respondErrorWithCDR(req, tx, 486, "Busy Here", callID)
			return
		case ErrAllBusy:
			h.logger.Info("inbound call rejected: all devices busy",
				"call_id", callID,
				"target", ic.TargetExtension.Extension,
			)
			h.respondErrorWithCDR(req, tx, 486, "Busy Here", callID)
			return
		case ErrNoRegistrations:
			h.logger.Info("inbound call failed: no registrations",
				"call_id", callID,
				"target", ic.TargetExtension.Extension,
			)
			// Try follow-me if enabled (no registered devices at all).
			if h.tryFollowMe(ctx, req, tx, ic.TargetExtension, callID, ic.CallerIDName, ic.CallerIDNum) {
				return
			}
			h.respondErrorWithCDR(req, tx, 480, "Temporarily Unavailable", callID)
			return
		case ErrExtensionNotFound:
			h.logger.Info("inbound call failed: extension not found",
				"call_id", callID,
				"request_uri", ic.RequestURI,
			)
			h.respondErrorWithCDR(req, tx, 404, "Not Found", callID)
			return
		default:
			h.logger.Error("inbound call routing error",
				"call_id", callID,
				"error", err,
			)
			h.respondErrorWithCDR(req, tx, 500, "Internal Server Error", callID)
			return
		}
	}

	h.logger.Info("inbound call routed, forking to contacts",
		"call_id", callID,
		"target", route.TargetExtension.Extension,
		"contacts", len(route.Contacts),
		"trunk_id", ic.TrunkID,
	)

	// Allocate media bridge for NAT traversal.
	var bridge *MediaBridge
	var calleeSDP []byte
	if len(req.Body()) > 0 && h.sessionMgr != nil {
		var err error
		bridge, calleeSDP, err = AllocateMediaBridge(h.sessionMgr, req.Body(), callID, h.proxyIP, h.logger)
		if err != nil {
			h.logger.Error("failed to allocate media bridge",
				"call_id", callID,
				"error", err,
			)
			h.respondErrorWithCDR(req, tx, 500, "Internal Server Error", callID)
			return
		}
	}

	// Determine ring timeout from the target extension's settings.
	ringTimeout := time.Duration(route.TargetExtension.RingTimeout) * time.Second
	if ringTimeout <= 0 {
		ringTimeout = 30 * time.Second
	}

	forkCtx, cancelFork := context.WithTimeout(ctx, ringTimeout)

	h.logger.Debug("forking inbound call with ring timeout",
		"call_id", callID,
		"ring_timeout_s", int(ringTimeout.Seconds()),
	)

	// Register as pending so CANCEL handler can find it.
	h.pendingMgr.Add(&PendingCall{
		CallID:     callID,
		CallerTx:   tx,
		CallerReq:  req,
		CancelFork: cancelFork,
		Bridge:     bridge,
	})

	// Fork INVITE to all registered contacts.
	result := h.forker.Fork(forkCtx, req, tx, route.Contacts, nil, callID, calleeSDP)

	pc := h.pendingMgr.Remove(callID)
	cancelFork()

	if pc == nil {
		h.logger.Info("inbound fork completed but call was already cancelled",
			"call_id", callID,
		)
		if result.Answered && result.AnsweringTx != nil {
			result.AnsweringTx.Terminate()
		}
		return
	}

	if result.Error != nil {
		h.logger.Error("inbound fork failed",
			"call_id", callID,
			"error", result.Error,
		)
		if bridge != nil {
			bridge.Release()
		}
		h.respondErrorWithCDR(req, tx, 500, "Internal Server Error", callID)
		return
	}

	if result.AllBusy {
		h.logger.Info("inbound call all devices busy",
			"call_id", callID,
			"target", route.TargetExtension.Extension,
		)
		if bridge != nil {
			bridge.Release()
		}
		h.respondErrorWithCDR(req, tx, 486, "Busy Here", callID)
		return
	}

	if !result.Answered {
		h.logger.Info("inbound call no device answered (ring timeout)",
			"call_id", callID,
			"target", route.TargetExtension.Extension,
			"ring_timeout_s", int(ringTimeout.Seconds()),
		)
		if bridge != nil {
			bridge.Release()
		}

		// Check if follow-me is enabled and try external numbers.
		if h.tryFollowMe(ctx, req, tx, route.TargetExtension, callID, ic.CallerIDName, ic.CallerIDNum) {
			return
		}

		h.respondErrorWithCDR(req, tx, 480, "Temporarily Unavailable", callID)
		return
	}

	// A device answered — relay the 200 OK to the trunk.
	h.logger.Info("inbound call answered, relaying 200 ok",
		"call_id", callID,
		"target", route.TargetExtension.Extension,
		"contact", result.AnsweringContact.ContactURI,
	)

	// Send ACK to the answering callee device.
	ackReq := buildACKFor2xx(result.AnsweringLeg.req, result.AnswerResponse)
	if err := h.forker.Client().WriteRequest(ackReq); err != nil {
		h.logger.Error("failed to send ack to callee",
			"call_id", callID,
			"contact", result.AnsweringContact.ContactURI,
			"error", err,
		)
		result.AnsweringTx.Terminate()
		if bridge != nil {
			bridge.Release()
		}
		h.respondErrorWithCDR(req, tx, 500, "Internal Server Error", callID)
		return
	}

	// Complete media bridge with callee's SDP.
	var mediaSession *media.MediaSession
	okBody := result.AnswerResponse.Body()
	if bridge != nil && len(result.AnswerResponse.Body()) > 0 {
		rewrittenForCaller, err := bridge.CompleteMediaBridge(result.AnswerResponse.Body())
		if err != nil {
			h.logger.Error("failed to complete media bridge",
				"call_id", callID,
				"error", err,
			)
		} else {
			okBody = rewrittenForCaller
			mediaSession = bridge.Session()
		}
	}

	// Forward the 200 OK back to the trunk.
	okResponse := sip.NewResponseFromRequest(req, 200, "OK", okBody)
	if len(okBody) > 0 {
		okResponse.AppendHeader(sip.NewHeader("Content-Type", "application/sdp"))
	}

	if err := tx.Respond(okResponse); err != nil {
		h.logger.Error("failed to relay 200 ok to trunk",
			"call_id", callID,
			"error", err,
		)
		result.AnsweringTx.Terminate()
		if mediaSession != nil {
			mediaSession.Release()
		}
		return
	}

	// Track the active call as a dialog.
	dialog := &Dialog{
		CallID:       callID,
		Direction:    ic.CallType,
		TrunkID:      ic.TrunkID,
		CallerIDName: ic.CallerIDName,
		CallerIDNum:  ic.CallerIDNum,
		CalledNum:    ic.RequestURI,
		StartTime:    time.Now(),
		CallerTx:     tx,
		CallerReq:    req,
		CalleeTx:     result.AnsweringTx,
		CalleeReq:    result.AnsweringLeg.req,
		CalleeRes:    result.AnswerResponse,
		Media:        mediaSession,
		Caller:       CallLeg{},
		Callee: CallLeg{
			Extension:    ic.TargetExtension,
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

	// Start call recording based on global policy and per-extension/trunk recording_mode.
	if shouldRecord(h.globalRecordingPolicy(), nil, ic.TargetExtension, inboundTrunk) && mediaSession != nil {
		dialog.Recorder = h.startCallRecording(callID, mediaSession)
	}

	h.dialogMgr.CreateDialog(dialog)
	h.updateCDROnAnswer(callID, ic.TrunkID)

	h.logger.Info("inbound call dialog established",
		"call_id", callID,
		"caller", ic.CallerIDNum,
		"callee", ic.RequestURI,
		"trunk_id", ic.TrunkID,
		"active_calls", h.dialogMgr.ActiveCallCount(),
		"media_bridged", mediaSession != nil,
		"recording", dialog.Recorder != nil,
	)
}

// classifyCall determines whether the INVITE is internal, inbound, or outbound.
// Returns nil InviteContext (without error) if classifyCall already sent a SIP
// response (auth challenge, rejection, etc.).
func (h *InviteHandler) classifyCall(req *sip.Request, tx sip.ServerTransaction) (*InviteContext, error) {
	ctx := context.Background()

	sourceIP := sourceHost(req)
	requestUser := req.Recipient.User

	fromUser := ""
	fromName := ""
	if from := req.From(); from != nil {
		fromUser = from.Address.User
		fromName = from.DisplayName
	}

	// Step 1: Check if the INVITE is from a known trunk (IP-auth match).
	trunkID, trunkName, isTrunk := h.trunkRegistrar.IPMatcher().MatchIPTrunk(sourceIP)
	if isTrunk {
		h.logger.Debug("invite from ip-auth trunk",
			"trunk_id", trunkID,
			"trunk", trunkName,
			"source", sourceIP,
		)

		return h.classifyInboundCall(ctx, req, trunkID, requestUser, fromName, fromUser)
	}

	// Step 2: Not from a trunk — try to authenticate as a local extension.
	ext := h.auth.Authenticate(req, tx)
	if ext == nil {
		// Auth sent 401 challenge or 403 rejection — no InviteContext to return.
		return nil, nil
	}

	// Caller is a local extension. Determine if target is internal or external.
	ic := &InviteContext{
		CallerExtension: ext,
		RequestURI:      requestUser,
		CallerIDName:    ext.Name,
		CallerIDNum:     ext.Extension,
	}

	// Step 3: Check if the target matches a local extension.
	targetExt, err := h.extensions.GetByExtension(ctx, requestUser)
	if err != nil {
		return nil, err
	}

	if targetExt != nil {
		// Internal call: extension-to-extension.
		ic.CallType = CallTypeInternal
		ic.TargetExtension = targetExt
		return ic, nil
	}

	// Step 4: Target is not a local extension — outbound call.
	ic.CallType = CallTypeOutbound
	return ic, nil
}

// classifyInboundCall processes an INVITE that arrived from a trunk.
// It checks the Request-URI against inbound_numbers and local extensions.
func (h *InviteHandler) classifyInboundCall(
	ctx context.Context,
	req *sip.Request,
	trunkID int64,
	requestUser string,
	callerName string,
	callerNum string,
) (*InviteContext, error) {
	ic := &InviteContext{
		CallType:     CallTypeInbound,
		TrunkID:      trunkID,
		RequestURI:   requestUser,
		CallerIDName: callerName,
		CallerIDNum:  callerNum,
	}

	// Check if the dialed number matches an inbound_numbers entry.
	inNum, err := h.inboundNumbers.GetByNumber(ctx, requestUser)
	if err != nil {
		return nil, err
	}
	if inNum != nil && inNum.Enabled {
		ic.InboundNumber = inNum
		return ic, nil
	}

	// Fall back: check if the dialed number is a local extension directly.
	targetExt, err := h.extensions.GetByExtension(ctx, requestUser)
	if err != nil {
		return nil, err
	}
	if targetExt != nil {
		ic.TargetExtension = targetExt
		return ic, nil
	}

	// No matching DID or extension — the call has nowhere to go.
	h.logger.Warn("inbound invite no matching destination",
		"trunk_id", trunkID,
		"request_uri", requestUser,
		"source", req.Source(),
	)
	return ic, nil
}

// sourceHost extracts the IP address (without port) from the request's source.
func sourceHost(req *sip.Request) string {
	source := req.Source()
	host, _, err := net.SplitHostPort(source)
	if err != nil {
		return source
	}
	return host
}

// buildACKFor2xx creates an ACK request for a 2xx response to an INVITE.
// Per RFC 3261 §13.2.2.4, the ACK for a 2xx is generated by the UAC core
// (not the transaction layer). The Request-URI is taken from the Contact
// header in the response if present, otherwise from the original INVITE.
func buildACKFor2xx(inviteReq *sip.Request, inviteResp *sip.Response) *sip.Request {
	recipient := &inviteReq.Recipient
	if contact := inviteResp.Contact(); contact != nil {
		recipient = &contact.Address
	}

	ack := sip.NewRequest(sip.ACK, *recipient.Clone())
	ack.SipVersion = inviteReq.SipVersion

	// Copy Route headers from the original INVITE if present.
	if len(inviteReq.GetHeaders("Route")) > 0 {
		sip.CopyHeaders("Route", inviteReq, ack)
	}

	// From: same as original INVITE.
	if h := inviteReq.From(); h != nil {
		ack.AppendHeader(sip.HeaderClone(h))
	}

	// To: from the response (includes the remote tag).
	if h := inviteResp.To(); h != nil {
		ack.AppendHeader(sip.HeaderClone(h))
	}

	// Call-ID: same as original INVITE.
	if h := inviteReq.CallID(); h != nil {
		ack.AppendHeader(sip.HeaderClone(h))
	}

	// CSeq: same sequence number, method changed to ACK.
	if h := inviteReq.CSeq(); h != nil {
		ack.AppendHeader(sip.HeaderClone(h))
	}
	if cseq := ack.CSeq(); cseq != nil {
		cseq.MethodName = sip.ACK
	}

	maxFwd := sip.MaxForwardsHeader(70)
	ack.AppendHeader(&maxFwd)

	// Contact from original INVITE for target refresh.
	if h := inviteReq.Contact(); h != nil {
		ack.AppendHeader(sip.HeaderClone(h))
	}

	ack.SetTransport(inviteReq.Transport())
	ack.SetSource(inviteReq.Source())

	return ack
}

// updateCDROnAnswer updates the CDR with the answer time when a call is answered.
func (h *InviteHandler) updateCDROnAnswer(callID string, trunkID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cdr, err := h.cdrs.GetByCallID(ctx, callID)
	if err != nil {
		h.logger.Error("failed to fetch cdr for answer update",
			"call_id", callID,
			"error", err,
		)
		return
	}
	if cdr == nil {
		h.logger.Warn("no cdr found to update on answer",
			"call_id", callID,
		)
		return
	}

	now := time.Now()
	cdr.AnswerTime = &now

	// Set trunk_id if known at answer time (e.g. outbound calls select trunk during routing).
	if trunkID > 0 && cdr.TrunkID == nil {
		cdr.TrunkID = &trunkID
	}

	if err := h.cdrs.Update(ctx, cdr); err != nil {
		h.logger.Error("failed to update cdr on answer",
			"call_id", callID,
			"error", err,
		)
		return
	}

	h.logger.Debug("cdr updated on answer",
		"call_id", callID,
		"cdr_id", cdr.ID,
	)
}

// createInitialCDR inserts a CDR row at call start with initial fields.
// The CDR will be updated on answer and hangup.
func (h *InviteHandler) createInitialCDR(ic *InviteContext, callID string) {
	cdr := &models.CDR{
		CallID:       callID,
		StartTime:    time.Now(),
		CallerIDName: ic.CallerIDName,
		CallerIDNum:  ic.CallerIDNum,
		Callee:       ic.RequestURI,
		Direction:    string(ic.CallType),
		Disposition:  "in_progress",
	}

	if ic.TrunkID > 0 {
		cdr.TrunkID = &ic.TrunkID
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.cdrs.Create(ctx, cdr); err != nil {
		h.logger.Error("failed to create initial cdr",
			"call_id", callID,
			"error", err,
		)
		return
	}

	h.logger.Debug("initial cdr created",
		"call_id", callID,
		"cdr_id", cdr.ID,
		"direction", cdr.Direction,
	)
}

// globalRecordingPolicy returns the system-wide recording policy from config.
// Returns an empty string if no policy is set (equivalent to "on_demand").
func (h *InviteHandler) globalRecordingPolicy() string {
	val, _ := h.systemConfig.Get(context.Background(), "recording_policy")
	return val
}

// shouldRecord determines whether a call should be recorded based on the
// global recording policy and the recording_mode of the caller extension,
// callee extension, and/or trunk. The global policy acts as a system-wide
// override: "always" forces all calls to be recorded, "off" disables
// recording entirely. When the policy is empty or "on_demand", the
// per-extension and per-trunk recording_mode settings are respected.
func shouldRecord(globalPolicy string, caller, callee *models.Extension, trunk *models.Trunk) bool {
	if globalPolicy == "always" {
		return true
	}
	if globalPolicy == "off" {
		return false
	}
	// Global policy is empty or "on_demand" — defer to per-entity settings.
	if caller != nil && caller.RecordingMode == "always" {
		return true
	}
	if callee != nil && callee.RecordingMode == "always" {
		return true
	}
	if trunk != nil && trunk.RecordingMode == "always" {
		return true
	}
	return false
}

// startCallRecording creates a Recorder for the call and attaches it to the
// media session's relay. Returns the recorder on success, or nil if recording
// could not be started (errors are logged but not fatal to the call).
func (h *InviteHandler) startCallRecording(callID string, mediaSession *media.MediaSession) *media.Recorder {
	if h.dataDir == "" || mediaSession == nil {
		return nil
	}

	filePath := media.RecordingPath(h.dataDir, callID, time.Now())
	rec, err := media.NewRecorder(filePath, h.logger)
	if err != nil {
		h.logger.Error("failed to start call recording",
			"call_id", callID,
			"file", filePath,
			"error", err,
		)
		return nil
	}

	if err := mediaSession.SetRecorder(rec); err != nil {
		h.logger.Error("failed to attach recorder to media session",
			"call_id", callID,
			"error", err,
		)
		rec.Stop()
		return nil
	}

	h.logger.Info("call recording started",
		"call_id", callID,
		"file", filePath,
	)

	return rec
}

// MapSIPToDisposition maps a SIP response status code to a CDR-friendly
// disposition label and hangup cause string.
func MapSIPToDisposition(statusCode int) (disposition string, hangupCause string) {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return "answered", "normal_clearing"
	case statusCode == 486 || statusCode == 600:
		return "busy", "busy"
	case statusCode == 480 || statusCode == 408:
		return "no_answer", "no_answer"
	case statusCode == 487:
		return "cancelled", "caller_cancel"
	case statusCode == 404:
		return "failed", "not_found"
	case statusCode == 403:
		return "failed", "forbidden"
	case statusCode == 488:
		return "failed", "not_acceptable"
	case statusCode == 501:
		return "failed", "not_implemented"
	case statusCode == 503:
		return "failed", "service_unavailable"
	case statusCode == 603:
		return "failed", "declined"
	case statusCode >= 400 && statusCode < 500:
		return "failed", "client_error"
	case statusCode >= 500:
		return "failed", "server_error"
	default:
		return "failed", "unknown"
	}
}

// finalizeCDRFailed updates a CDR when a call fails before being answered
// (e.g. rejected, not found, busy). Uses the SIP response code to determine
// the disposition and hangup cause.
func (h *InviteHandler) finalizeCDRFailed(callID string, sipCode int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cdr, err := h.cdrs.GetByCallID(ctx, callID)
	if err != nil {
		h.logger.Error("failed to fetch cdr for failure update",
			"call_id", callID,
			"error", err,
		)
		return
	}
	if cdr == nil {
		return
	}

	now := time.Now()
	durationSec := int(now.Sub(cdr.StartTime).Seconds())
	billableSec := 0
	disposition, hangupCause := MapSIPToDisposition(sipCode)

	cdr.EndTime = &now
	cdr.Duration = &durationSec
	cdr.BillableDur = &billableSec
	cdr.Disposition = disposition
	cdr.HangupCause = hangupCause

	if err := h.cdrs.Update(ctx, cdr); err != nil {
		h.logger.Error("failed to finalize cdr on failure",
			"call_id", callID,
			"error", err,
		)
	}
}

// respondErrorWithCDR sends a SIP error response and finalizes the CDR with
// the failure disposition based on the SIP status code.
func (h *InviteHandler) respondErrorWithCDR(req *sip.Request, tx sip.ServerTransaction, code int, reason string, callID string) {
	res := sip.NewResponseFromRequest(req, code, reason, nil)
	if err := tx.Respond(res); err != nil {
		h.logger.Error("failed to send error response",
			"code", code,
			"error", err,
		)
	}
	if callID != "" {
		h.finalizeCDRFailed(callID, code)
	}
}

func (h *InviteHandler) respondError(req *sip.Request, tx sip.ServerTransaction, code int, reason string) {
	res := sip.NewResponseFromRequest(req, code, reason, nil)
	if err := tx.Respond(res); err != nil {
		h.logger.Error("failed to send error response",
			"code", code,
			"error", err,
		)
	}
}

// tryFollowMe checks if the target extension has follow-me enabled and
// attempts to ring external numbers sequentially via outbound trunk.
// Returns true if follow-me was attempted (regardless of whether a number
// answered), meaning the caller should not send a failure response.
// Returns false if follow-me is not enabled/configured.
func (h *InviteHandler) tryFollowMe(
	ctx context.Context,
	req *sip.Request,
	tx sip.ServerTransaction,
	ext *models.Extension,
	callID string,
	callerIDName string,
	callerIDNum string,
) bool {
	if ext == nil || !ext.FollowMeEnabled || h.flowActions == nil {
		return false
	}

	numbers := models.ParseFollowMeNumbers(ext.FollowMeNumbers)
	if len(numbers) == 0 {
		return false
	}

	h.logger.Info("attempting follow-me for extension",
		"call_id", callID,
		"extension", ext.Extension,
		"follow_me_numbers", len(numbers),
	)

	// Build a flow CallContext for the follow-me ring.
	callCtx := flow.NewCallContext(
		callID,
		callerIDName,
		callerIDNum,
		ext.Extension,
		nil,
		0,
		req,
		tx,
	)

	result, err := h.flowActions.RingFollowMe(ctx, callCtx, numbers, callerIDName, callerIDNum)
	if err != nil {
		h.logger.Error("follow-me failed",
			"call_id", callID,
			"extension", ext.Extension,
			"error", err,
		)
		// Follow-me was attempted but failed — still return true because
		// the pending call state may have been altered.
		h.respondErrorWithCDR(req, tx, 480, "Temporarily Unavailable", callID)
		return true
	}

	if result.Answered {
		h.logger.Info("follow-me answered",
			"call_id", callID,
			"extension", ext.Extension,
		)
		return true
	}

	// Follow-me was attempted but no number answered.
	h.logger.Info("follow-me no answer on any external number",
		"call_id", callID,
		"extension", ext.Extension,
	)
	return false
}
