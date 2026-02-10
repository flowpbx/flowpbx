package sip

import (
	"context"
	"log/slog"
	"net"
	"time"

	"github.com/emiago/sipgo/sip"
	"github.com/flowpbx/flowpbx/internal/database"
	"github.com/flowpbx/flowpbx/internal/database/models"
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
	trunkRegistrar *TrunkRegistrar
	auth           *Authenticator
	router         *CallRouter
	forker         *Forker
	dialogMgr      *DialogManager
	pendingMgr     *PendingCallManager
	sessionMgr     *media.SessionManager
	proxyIP        string
	logger         *slog.Logger
}

// NewInviteHandler creates a new INVITE request handler.
func NewInviteHandler(
	extensions database.ExtensionRepository,
	registrations database.RegistrationRepository,
	inboundNumbers database.InboundNumberRepository,
	trunkRegistrar *TrunkRegistrar,
	auth *Authenticator,
	forker *Forker,
	dialogMgr *DialogManager,
	pendingMgr *PendingCallManager,
	sessionMgr *media.SessionManager,
	proxyIP string,
	logger *slog.Logger,
) *InviteHandler {
	router := NewCallRouter(extensions, registrations, dialogMgr, logger)
	return &InviteHandler{
		extensions:     extensions,
		registrations:  registrations,
		inboundNumbers: inboundNumbers,
		trunkRegistrar: trunkRegistrar,
		auth:           auth,
		router:         router,
		forker:         forker,
		dialogMgr:      dialogMgr,
		pendingMgr:     pendingMgr,
		sessionMgr:     sessionMgr,
		proxyIP:        proxyIP,
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

	// Dispatch to call routing based on call type.
	switch ic.CallType {
	case CallTypeInternal:
		h.handleInternalCall(req, tx, ic, callID)
	case CallTypeInbound:
		// TODO: look up flow from InboundNumber, walk graph.
		h.logger.Warn("inbound call routing not yet implemented",
			"call_id", callID,
		)
		h.respondError(req, tx, 501, "Not Implemented")
	case CallTypeOutbound:
		// TODO: match outbound route, send INVITE to trunk.
		h.logger.Warn("outbound call routing not yet implemented",
			"call_id", callID,
		)
		h.respondError(req, tx, 501, "Not Implemented")
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
			h.respondError(req, tx, 486, "Busy Here")
			return
		case ErrAllBusy:
			h.logger.Info("internal call rejected: all devices busy",
				"call_id", callID,
				"target", ic.TargetExtension.Extension,
			)
			h.respondError(req, tx, 486, "Busy Here")
			return
		case ErrNoRegistrations:
			h.logger.Info("internal call failed: no registrations",
				"call_id", callID,
				"target", ic.TargetExtension.Extension,
			)
			h.respondError(req, tx, 480, "Temporarily Unavailable")
			return
		case ErrExtensionNotFound:
			h.logger.Info("internal call failed: extension not found",
				"call_id", callID,
				"request_uri", ic.RequestURI,
			)
			h.respondError(req, tx, 404, "Not Found")
			return
		default:
			h.logger.Error("internal call routing error",
				"call_id", callID,
				"error", err,
			)
			h.respondError(req, tx, 500, "Internal Server Error")
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
			h.respondError(req, tx, 500, "Internal Server Error")
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
		h.respondError(req, tx, 500, "Internal Server Error")
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
		h.respondError(req, tx, 486, "Busy Here")
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
		// 480 Temporarily Unavailable — no device picked up within ring timeout.
		h.respondError(req, tx, 480, "Temporarily Unavailable")
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
		h.respondError(req, tx, 500, "Internal Server Error")
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

	h.dialogMgr.CreateDialog(dialog)

	h.logger.Info("call dialog established",
		"call_id", callID,
		"caller", ic.CallerIDNum,
		"callee", ic.RequestURI,
		"active_calls", h.dialogMgr.ActiveCallCount(),
		"media_bridged", mediaSession != nil,
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

func (h *InviteHandler) respondError(req *sip.Request, tx sip.ServerTransaction, code int, reason string) {
	res := sip.NewResponseFromRequest(req, code, reason, nil)
	if err := tx.Respond(res); err != nil {
		h.logger.Error("failed to send error response",
			"code", code,
			"error", err,
		)
	}
}
