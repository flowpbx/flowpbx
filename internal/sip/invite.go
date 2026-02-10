package sip

import (
	"context"
	"log/slog"
	"net"

	"github.com/emiago/sipgo/sip"
	"github.com/flowpbx/flowpbx/internal/database"
	"github.com/flowpbx/flowpbx/internal/database/models"
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
	logger         *slog.Logger
}

// NewInviteHandler creates a new INVITE request handler.
func NewInviteHandler(
	extensions database.ExtensionRepository,
	registrations database.RegistrationRepository,
	inboundNumbers database.InboundNumberRepository,
	trunkRegistrar *TrunkRegistrar,
	auth *Authenticator,
	logger *slog.Logger,
) *InviteHandler {
	router := NewCallRouter(extensions, registrations, logger)
	return &InviteHandler{
		extensions:     extensions,
		registrations:  registrations,
		inboundNumbers: inboundNumbers,
		trunkRegistrar: trunkRegistrar,
		auth:           auth,
		router:         router,
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

	// Send 100 Trying immediately.
	trying := sip.NewResponseFromRequest(req, 100, "Trying", nil)
	if err := tx.Respond(trying); err != nil {
		h.logger.Error("failed to send 100 trying",
			"call_id", callID,
			"error", err,
		)
		return
	}

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

	h.logger.Info("internal call routed, ready to fork",
		"call_id", callID,
		"target", route.TargetExtension.Extension,
		"contacts", len(route.Contacts),
	)

	// TODO: Fork INVITE to all contacts in route.Contacts (multi-device ringing),
	// handle 180/183 relay, 200 OK from first answering device, cancel other forks.
	// This will be implemented in subsequent sprint tasks.
	h.respondError(req, tx, 501, "Not Implemented")
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

func (h *InviteHandler) respondError(req *sip.Request, tx sip.ServerTransaction, code int, reason string) {
	res := sip.NewResponseFromRequest(req, code, reason, nil)
	if err := tx.Respond(res); err != nil {
		h.logger.Error("failed to send error response",
			"code", code,
			"error", err,
		)
	}
}
