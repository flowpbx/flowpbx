package sip

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	"github.com/flowpbx/flowpbx/internal/database"
	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/flowpbx/flowpbx/internal/media"
	"github.com/icholy/digest"
)

// OutboundRouter selects a trunk for outbound calls and builds the INVITE
// to send to the trunk provider.
type OutboundRouter struct {
	trunks    database.TrunkRepository
	encryptor *database.Encryptor
	logger    *slog.Logger
}

// NewOutboundRouter creates a new outbound call router.
func NewOutboundRouter(
	trunks database.TrunkRepository,
	encryptor *database.Encryptor,
	logger *slog.Logger,
) *OutboundRouter {
	return &OutboundRouter{
		trunks:    trunks,
		encryptor: encryptor,
		logger:    logger.With("subsystem", "outbound-router"),
	}
}

// ErrNoTrunksAvailable is returned when no enabled trunks exist.
var ErrNoTrunksAvailable = fmt.Errorf("no trunks available for outbound routing")

// SelectTrunk returns the first enabled trunk ordered by priority.
// Trunk selection with failover (skip failed/disabled, try next) is
// implemented in a subsequent sprint task.
func (r *OutboundRouter) SelectTrunk(ctx context.Context) (*models.Trunk, error) {
	trunks, err := r.trunks.ListEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing enabled trunks: %w", err)
	}

	if len(trunks) == 0 {
		return nil, ErrNoTrunksAvailable
	}

	// ListEnabled returns trunks ordered by priority, name — take the first.
	trunk := trunks[0]

	// Decrypt the trunk password if encryption is configured.
	if trunk.Password != "" && r.encryptor != nil {
		decrypted, err := r.encryptor.Decrypt(trunk.Password)
		if err != nil {
			return nil, fmt.Errorf("decrypting trunk password: %w", err)
		}
		trunk.Password = decrypted
	}

	return &trunk, nil
}

// handleOutboundCall routes a call from a local extension to an external number
// via a SIP trunk. The PBX acts as a B2BUA: the caller's INVITE is terminated
// here and a new INVITE is sent to the trunk.
func (h *InviteHandler) handleOutboundCall(req *sip.Request, tx sip.ServerTransaction, ic *InviteContext, callID string) {
	ctx := context.Background()

	if h.outboundRouter == nil {
		h.logger.Error("outbound router not configured", "call_id", callID)
		h.respondError(req, tx, 500, "Internal Server Error")
		return
	}

	// Select the outbound trunk.
	trunk, err := h.outboundRouter.SelectTrunk(ctx)
	if err != nil {
		if err == ErrNoTrunksAvailable {
			h.logger.Warn("outbound call failed: no trunks available",
				"call_id", callID,
				"dialed", ic.RequestURI,
			)
			h.respondError(req, tx, 503, "Service Unavailable")
			return
		}
		h.logger.Error("outbound call failed: trunk selection error",
			"call_id", callID,
			"error", err,
		)
		h.respondError(req, tx, 500, "Internal Server Error")
		return
	}

	h.logger.Info("outbound call routing via trunk",
		"call_id", callID,
		"trunk", trunk.Name,
		"trunk_id", trunk.ID,
		"dialed", ic.RequestURI,
	)

	// Phase 1: Allocate media bridge to proxy RTP between caller and trunk.
	var bridge *MediaBridge
	var trunkSDP []byte
	if len(req.Body()) > 0 && h.sessionMgr != nil {
		bridge, trunkSDP, err = AllocateMediaBridge(h.sessionMgr, req.Body(), callID, h.proxyIP, h.logger)
		if err != nil {
			h.logger.Error("failed to allocate media bridge for outbound call",
				"call_id", callID,
				"error", err,
			)
			h.respondError(req, tx, 500, "Internal Server Error")
			return
		}
	}

	// Register as pending call so CANCEL handler can abort.
	outboundCtx, cancelOutbound := context.WithTimeout(ctx, 60*time.Second)
	h.pendingMgr.Add(&PendingCall{
		CallID:     callID,
		CallerTx:   tx,
		CallerReq:  req,
		CancelFork: cancelOutbound,
		Bridge:     bridge,
	})

	// Send INVITE to the trunk (runs synchronously until final response).
	result := h.sendOutboundInvite(outboundCtx, req, tx, ic, trunk, callID, trunkSDP)

	// Remove from pending calls.
	pc := h.pendingMgr.Remove(callID)
	cancelOutbound()

	// If pending call was already cancelled by the CANCEL handler, clean up.
	if pc == nil {
		h.logger.Info("outbound call completed but was already cancelled",
			"call_id", callID,
		)
		if result != nil && result.answered && result.tx != nil {
			// Send BYE to trunk if it answered during cancellation.
			result.tx.Terminate()
		}
		if bridge != nil {
			bridge.Release()
		}
		return
	}

	if result.err != nil {
		h.logger.Error("outbound invite to trunk failed",
			"call_id", callID,
			"trunk", trunk.Name,
			"error", result.err,
		)
		if bridge != nil {
			bridge.Release()
		}
		h.respondError(req, tx, 502, "Bad Gateway")
		return
	}

	if !result.answered {
		h.logger.Info("outbound call not answered by trunk",
			"call_id", callID,
			"trunk", trunk.Name,
			"status", result.statusCode,
			"reason", result.reason,
		)
		if bridge != nil {
			bridge.Release()
		}
		// Map trunk response codes to caller-facing responses.
		code, reason := mapTrunkFailure(result.statusCode, result.reason)
		h.respondError(req, tx, code, reason)
		return
	}

	h.logger.Info("outbound call answered by trunk",
		"call_id", callID,
		"trunk", trunk.Name,
	)

	// Send ACK to the trunk for its 200 OK.
	ackReq := buildACKFor2xx(result.req, result.res)
	if err := h.forker.Client().WriteRequest(ackReq); err != nil {
		h.logger.Error("failed to send ack to trunk",
			"call_id", callID,
			"trunk", trunk.Name,
			"error", err,
		)
		result.tx.Terminate()
		if bridge != nil {
			bridge.Release()
		}
		h.respondError(req, tx, 500, "Internal Server Error")
		return
	}

	// Phase 2: Complete media bridge with trunk's SDP.
	var mediaSession *media.MediaSession
	okBody := result.res.Body()
	if bridge != nil && len(result.res.Body()) > 0 {
		rewrittenForCaller, err := bridge.CompleteMediaBridge(result.res.Body())
		if err != nil {
			h.logger.Error("failed to complete media bridge for outbound call",
				"call_id", callID,
				"error", err,
			)
			// Fall back to direct media (SDP pass-through).
		} else {
			okBody = rewrittenForCaller
			mediaSession = bridge.Session()
		}
	}

	// Forward the 200 OK to the caller (local extension).
	okResponse := sip.NewResponseFromRequest(req, 200, "OK", okBody)
	if len(okBody) > 0 {
		okResponse.AppendHeader(sip.NewHeader("Content-Type", "application/sdp"))
	}

	if err := tx.Respond(okResponse); err != nil {
		h.logger.Error("failed to relay 200 ok to caller for outbound call",
			"call_id", callID,
			"error", err,
		)
		result.tx.Terminate()
		if mediaSession != nil {
			mediaSession.Release()
		}
		return
	}

	// Track the active call as a dialog.
	dialog := &Dialog{
		CallID:       callID,
		Direction:    ic.CallType,
		CallerIDName: ic.CallerIDName,
		CallerIDNum:  ic.CallerIDNum,
		CalledNum:    ic.RequestURI,
		StartTime:    time.Now(),
		CallerTx:     tx,
		CallerReq:    req,
		CalleeTx:     result.tx,
		CalleeReq:    result.req,
		CalleeRes:    result.res,
		Media:        mediaSession,
		Caller: CallLeg{
			Extension: ic.CallerExtension,
		},
		Callee: CallLeg{
			// Trunk leg has no extension; ContactURI is the trunk address.
			ContactURI: fmt.Sprintf("sip:%s:%d", trunk.Host, trunk.Port),
		},
	}

	// Extract dialog tags.
	if from := req.From(); from != nil {
		if tag, ok := from.Params.Get("tag"); ok {
			dialog.Caller.FromTag = tag
		}
	}
	if to := result.res.To(); to != nil {
		if tag, ok := to.Params.Get("tag"); ok {
			dialog.Callee.ToTag = tag
		}
	}

	// Extract trunk remote target from Contact header in 200 OK.
	if contact := result.res.Contact(); contact != nil {
		uri := contact.Address.Clone()
		dialog.Callee.RemoteTarget = uri
	}

	h.dialogMgr.CreateDialog(dialog)

	h.logger.Info("outbound call dialog established",
		"call_id", callID,
		"caller", ic.CallerIDNum,
		"callee", ic.RequestURI,
		"trunk", trunk.Name,
		"trunk_id", trunk.ID,
		"active_calls", h.dialogMgr.ActiveCallCount(),
		"media_bridged", mediaSession != nil,
	)
}

// outboundResult holds the outcome of sending an INVITE to a trunk.
type outboundResult struct {
	answered   bool
	statusCode int
	reason     string
	res        *sip.Response
	req        *sip.Request
	tx         sip.ClientTransaction
	err        error
}

// sendOutboundInvite builds and sends an INVITE to the trunk for an outbound call.
// It handles digest authentication challenges (401/407) and relays provisional
// responses back to the caller.
func (h *InviteHandler) sendOutboundInvite(
	ctx context.Context,
	callerReq *sip.Request,
	callerTx sip.ServerTransaction,
	ic *InviteContext,
	trunk *models.Trunk,
	callID string,
	sdpBody []byte,
) *outboundResult {
	// Build the INVITE Request-URI: sip:<dialed_number>@<trunk_host>:<trunk_port>
	dialedNumber := ic.RequestURI
	recipientStr := fmt.Sprintf("sip:%s@%s:%d", dialedNumber, trunk.Host, trunk.Port)
	var recipient sip.Uri
	if err := sip.ParseUri(recipientStr, &recipient); err != nil {
		return &outboundResult{err: fmt.Errorf("parsing trunk uri: %w", err)}
	}

	req := sip.NewRequest(sip.INVITE, recipient)
	req.SetTransport(strings.ToUpper(trunk.Transport))

	// Set the SDP body (rewritten by media proxy, or original from caller).
	body := sdpBody
	if body == nil {
		body = callerReq.Body()
	}
	if len(body) > 0 {
		req.SetBody(body)
		req.AppendHeader(sip.NewHeader("Content-Type", "application/sdp"))
	}

	// Preserve the Call-ID for CDR correlation.
	req.AppendHeader(sip.NewHeader("Call-ID", callID))

	h.logger.Debug("sending outbound invite to trunk",
		"call_id", callID,
		"trunk", trunk.Name,
		"recipient", recipientStr,
	)

	// Send the initial INVITE.
	inviteTx, err := h.forker.Client().TransactionRequest(ctx, req, sipgo.ClientRequestBuild)
	if err != nil {
		return &outboundResult{err: fmt.Errorf("sending invite to trunk: %w", err)}
	}

	// Collect responses from the trunk.
	ringingRelayed := false
	for {
		var res *sip.Response
		select {
		case <-ctx.Done():
			inviteTx.Terminate()
			return &outboundResult{err: ctx.Err()}
		case <-inviteTx.Done():
			inviteTx.Terminate()
			if txErr := inviteTx.Err(); txErr != nil {
				return &outboundResult{err: fmt.Errorf("trunk transaction error: %w", txErr)}
			}
			return &outboundResult{err: fmt.Errorf("trunk transaction ended without final response")}
		case res = <-inviteTx.Responses():
		}

		h.logger.Debug("outbound trunk response",
			"call_id", callID,
			"status", res.StatusCode,
			"reason", res.Reason,
		)

		switch {
		case res.StatusCode == 100:
			// 100 Trying — absorb.
			continue

		case res.StatusCode == 180 || res.StatusCode == 183:
			// Relay provisional responses to the caller.
			if !ringingRelayed {
				ringingRelayed = true
				var provBody []byte
				if res.StatusCode == 183 && len(res.Body()) > 0 {
					provBody = res.Body()
				}
				ringing := sip.NewResponseFromRequest(callerReq, res.StatusCode, res.Reason, provBody)
				if provBody != nil {
					if ct := res.ContentType(); ct != nil {
						ringing.AppendHeader(sip.NewHeader("Content-Type", ct.Value()))
					}
				}
				if err := callerTx.Respond(ringing); err != nil {
					h.logger.Error("failed to relay ringing to caller",
						"call_id", callID,
						"error", err,
					)
				}
			}

		case res.StatusCode == 401 || res.StatusCode == 407:
			// Digest auth challenge from the trunk.
			inviteTx.Terminate()
			authResult := h.handleTrunkAuth(ctx, callerReq, callerTx, req, res, trunk, callID, body)
			return authResult

		case res.StatusCode >= 200 && res.StatusCode < 300:
			// 200 OK — call answered.
			return &outboundResult{
				answered: true,
				res:      res,
				req:      req,
				tx:       inviteTx,
			}

		case res.StatusCode >= 300:
			// Final failure response.
			inviteTx.Terminate()
			return &outboundResult{
				answered:   false,
				statusCode: res.StatusCode,
				reason:     res.Reason,
			}
		}
	}
}

// handleTrunkAuth handles a 401/407 digest authentication challenge from a trunk.
// It computes the digest response and re-sends the INVITE with authorization.
func (h *InviteHandler) handleTrunkAuth(
	ctx context.Context,
	callerReq *sip.Request,
	callerTx sip.ServerTransaction,
	origReq *sip.Request,
	challengeRes *sip.Response,
	trunk *models.Trunk,
	callID string,
	sdpBody []byte,
) *outboundResult {
	authHeader := "WWW-Authenticate"
	authzHeader := "Authorization"
	if challengeRes.StatusCode == 407 {
		authHeader = "Proxy-Authenticate"
		authzHeader = "Proxy-Authorization"
	}

	wwwAuth := challengeRes.GetHeader(authHeader)
	if wwwAuth == nil {
		return &outboundResult{
			err: fmt.Errorf("trunk sent %d but no %s header", challengeRes.StatusCode, authHeader),
		}
	}

	chal, err := digest.ParseChallenge(wwwAuth.Value())
	if err != nil {
		return &outboundResult{err: fmt.Errorf("parsing trunk auth challenge: %w", err)}
	}

	// Use auth_username if configured, otherwise username.
	authUser := trunk.Username
	if trunk.AuthUsername != "" {
		authUser = trunk.AuthUsername
	}

	recipientStr := fmt.Sprintf("sip:%s@%s:%d", origReq.Recipient.User, trunk.Host, trunk.Port)

	cred, err := digest.Digest(chal, digest.Options{
		Method:   origReq.Method.String(),
		URI:      recipientStr,
		Username: authUser,
		Password: trunk.Password,
	})
	if err != nil {
		return &outboundResult{err: fmt.Errorf("computing trunk digest: %w", err)}
	}

	h.logger.Debug("re-sending outbound invite with auth",
		"call_id", callID,
		"trunk", trunk.Name,
	)

	// Clone the original request and add the authorization header.
	authReq := origReq.Clone()
	authReq.RemoveHeader("Via")
	authReq.AppendHeader(sip.NewHeader(authzHeader, cred.String()))

	// Re-send with auth.
	authTx, err := h.forker.Client().TransactionRequest(ctx, authReq,
		sipgo.ClientRequestIncreaseCSEQ,
		sipgo.ClientRequestAddVia,
	)
	if err != nil {
		return &outboundResult{err: fmt.Errorf("sending authenticated invite to trunk: %w", err)}
	}

	// Collect responses from the authenticated INVITE.
	ringingRelayed := false
	for {
		var res *sip.Response
		select {
		case <-ctx.Done():
			authTx.Terminate()
			return &outboundResult{err: ctx.Err()}
		case <-authTx.Done():
			authTx.Terminate()
			if txErr := authTx.Err(); txErr != nil {
				return &outboundResult{err: fmt.Errorf("trunk auth transaction error: %w", txErr)}
			}
			return &outboundResult{err: fmt.Errorf("trunk auth transaction ended without final response")}
		case res = <-authTx.Responses():
		}

		h.logger.Debug("outbound trunk auth response",
			"call_id", callID,
			"status", res.StatusCode,
			"reason", res.Reason,
		)

		switch {
		case res.StatusCode == 100:
			continue

		case res.StatusCode == 180 || res.StatusCode == 183:
			if !ringingRelayed {
				ringingRelayed = true
				var provBody []byte
				if res.StatusCode == 183 && len(res.Body()) > 0 {
					provBody = res.Body()
				}
				ringing := sip.NewResponseFromRequest(callerReq, res.StatusCode, res.Reason, provBody)
				if provBody != nil {
					if ct := res.ContentType(); ct != nil {
						ringing.AppendHeader(sip.NewHeader("Content-Type", ct.Value()))
					}
				}
				if err := callerTx.Respond(ringing); err != nil {
					h.logger.Error("failed to relay ringing to caller",
						"call_id", callID,
						"error", err,
					)
				}
			}

		case res.StatusCode >= 200 && res.StatusCode < 300:
			return &outboundResult{
				answered: true,
				res:      res,
				req:      authReq,
				tx:       authTx,
			}

		case res.StatusCode >= 300:
			authTx.Terminate()
			return &outboundResult{
				answered:   false,
				statusCode: res.StatusCode,
				reason:     res.Reason,
			}
		}
	}
}

// mapTrunkFailure maps a SIP failure status code from a trunk to an
// appropriate response for the calling extension.
func mapTrunkFailure(statusCode int, reason string) (int, string) {
	switch {
	case statusCode == 403:
		return 403, "Forbidden"
	case statusCode == 404:
		return 404, "Not Found"
	case statusCode == 480:
		return 480, "Temporarily Unavailable"
	case statusCode == 486 || statusCode == 600:
		return 486, "Busy Here"
	case statusCode == 487:
		return 487, "Request Terminated"
	case statusCode == 488:
		return 488, "Not Acceptable Here"
	case statusCode == 503:
		return 503, "Service Unavailable"
	case statusCode >= 400 && statusCode < 500:
		return 503, "Service Unavailable"
	case statusCode >= 500:
		return 502, "Bad Gateway"
	default:
		return 503, "Service Unavailable"
	}
}
