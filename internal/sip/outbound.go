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
	trunks         database.TrunkRepository
	trunkRegistrar *TrunkRegistrar
	encryptor      *database.Encryptor
	logger         *slog.Logger
}

// NewOutboundRouter creates a new outbound call router.
func NewOutboundRouter(
	trunks database.TrunkRepository,
	trunkRegistrar *TrunkRegistrar,
	encryptor *database.Encryptor,
	logger *slog.Logger,
) *OutboundRouter {
	return &OutboundRouter{
		trunks:         trunks,
		trunkRegistrar: trunkRegistrar,
		encryptor:      encryptor,
		logger:         logger.With("subsystem", "outbound-router"),
	}
}

// ErrNoTrunksAvailable is returned when no enabled trunks exist.
var ErrNoTrunksAvailable = fmt.Errorf("no trunks available for outbound routing")

// SelectTrunks returns enabled trunks ordered by priority, skipping any whose
// runtime status is failed or disabled. Each trunk's password is decrypted
// before being returned. The caller should try trunks in order, falling back
// to the next if one fails.
func (r *OutboundRouter) SelectTrunks(ctx context.Context) ([]models.Trunk, error) {
	trunks, err := r.trunks.ListEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing enabled trunks: %w", err)
	}

	if len(trunks) == 0 {
		return nil, ErrNoTrunksAvailable
	}

	// Filter out trunks whose runtime status indicates they are not usable.
	var candidates []models.Trunk
	for _, trunk := range trunks {
		if r.trunkRegistrar != nil {
			state, ok := r.trunkRegistrar.GetStatus(trunk.ID)
			if ok && (state.Status == TrunkStatusFailed || state.Status == TrunkStatusDisabled) {
				r.logger.Debug("skipping trunk with unhealthy status",
					"trunk", trunk.Name,
					"trunk_id", trunk.ID,
					"status", state.Status,
				)
				continue
			}
		}

		// Decrypt the trunk password if encryption is configured.
		if trunk.Password != "" && r.encryptor != nil {
			decrypted, err := r.encryptor.Decrypt(trunk.Password)
			if err != nil {
				r.logger.Warn("skipping trunk: failed to decrypt password",
					"trunk", trunk.Name,
					"trunk_id", trunk.ID,
					"error", err,
				)
				continue
			}
			trunk.Password = decrypted
		}

		candidates = append(candidates, trunk)
	}

	if len(candidates) == 0 {
		return nil, ErrNoTrunksAvailable
	}

	return candidates, nil
}

// handleOutboundCall routes a call from a local extension to an external number
// via a SIP trunk. The PBX acts as a B2BUA: the caller's INVITE is terminated
// here and a new INVITE is sent to the trunk.
//
// Trunk failover: trunks are tried in priority order. If a trunk fails (network
// error, 5xx, or 4xx indicating a trunk-level issue), the next trunk is tried.
// Failures that indicate a callee-level issue (404, 486, 480, 487, 488, 600)
// are returned to the caller immediately without trying the next trunk.
func (h *InviteHandler) handleOutboundCall(req *sip.Request, tx sip.ServerTransaction, ic *InviteContext, callID string) {
	ctx := context.Background()

	if h.outboundRouter == nil {
		h.logger.Error("outbound router not configured", "call_id", callID)
		h.respondError(req, tx, 500, "Internal Server Error")
		return
	}

	// Select candidate trunks ordered by priority, filtered by health.
	trunks, err := h.outboundRouter.SelectTrunks(ctx)
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

	// Try each trunk in priority order until one succeeds or a callee-level
	// failure is returned that should not be retried on another trunk.
	var result *outboundResult
	var selectedTrunk *models.Trunk
	for i := range trunks {
		trunk := &trunks[i]

		h.logger.Info("outbound call routing via trunk",
			"call_id", callID,
			"trunk", trunk.Name,
			"trunk_id", trunk.ID,
			"dialed", ic.RequestURI,
			"attempt", i+1,
			"candidates", len(trunks),
		)

		result = h.sendOutboundInvite(outboundCtx, req, tx, ic, trunk, callID, trunkSDP)

		// Check if context was cancelled (e.g. CANCEL from caller).
		if outboundCtx.Err() != nil {
			break
		}

		if result.answered {
			selectedTrunk = trunk
			break
		}

		// Determine whether to try the next trunk or return the failure.
		// Callee-level failures should not be retried on another trunk.
		if result.err == nil && isCalleeFailure(result.statusCode) {
			h.logger.Info("outbound call callee failure, not retrying",
				"call_id", callID,
				"trunk", trunk.Name,
				"status", result.statusCode,
				"reason", result.reason,
			)
			selectedTrunk = trunk
			break
		}

		// Trunk-level failure — log and try next.
		if result.err != nil {
			h.logger.Warn("outbound trunk failed, trying next",
				"call_id", callID,
				"trunk", trunk.Name,
				"error", result.err,
				"attempt", i+1,
			)
		} else {
			h.logger.Warn("outbound trunk rejected call, trying next",
				"call_id", callID,
				"trunk", trunk.Name,
				"status", result.statusCode,
				"reason", result.reason,
				"attempt", i+1,
			)
		}
		selectedTrunk = trunk
	}

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
		trunkName := ""
		if selectedTrunk != nil {
			trunkName = selectedTrunk.Name
		}
		h.logger.Error("outbound invite failed on all trunks",
			"call_id", callID,
			"trunk", trunkName,
			"error", result.err,
			"trunks_tried", len(trunks),
		)
		if bridge != nil {
			bridge.Release()
		}
		h.respondError(req, tx, 502, "Bad Gateway")
		return
	}

	if !result.answered {
		trunkName := ""
		if selectedTrunk != nil {
			trunkName = selectedTrunk.Name
		}
		h.logger.Info("outbound call not answered",
			"call_id", callID,
			"trunk", trunkName,
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
		"trunk", selectedTrunk.Name,
	)

	// Send ACK to the trunk for its 200 OK.
	ackReq := buildACKFor2xx(result.req, result.res)
	if err := h.forker.Client().WriteRequest(ackReq); err != nil {
		h.logger.Error("failed to send ack to trunk",
			"call_id", callID,
			"trunk", selectedTrunk.Name,
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
			ContactURI: fmt.Sprintf("sip:%s:%d", selectedTrunk.Host, selectedTrunk.Port),
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
		"trunk", selectedTrunk.Name,
		"trunk_id", selectedTrunk.ID,
		"active_calls", h.dialogMgr.ActiveCallCount(),
		"media_bridged", mediaSession != nil,
	)
}

// isCalleeFailure returns true if the SIP status code indicates a failure
// specific to the callee (the dialed number) rather than the trunk itself.
// These failures should not be retried on another trunk.
func isCalleeFailure(statusCode int) bool {
	switch statusCode {
	case 404, 480, 486, 487, 488, 600, 603:
		return true
	default:
		return false
	}
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

// callerIDSource describes where the outbound caller ID values were taken from.
type callerIDSource string

const (
	callerIDFromTrunk     callerIDSource = "trunk"
	callerIDFromExtension callerIDSource = "extension"
)

// buildOutboundCallerID determines the caller ID name and number to use on an
// outbound INVITE. The rules are:
//  1. If the trunk has CallerIDName and/or CallerIDNum configured, those values
//     take priority (the trunk provider may require a specific caller ID).
//  2. Otherwise, fall back to the calling extension's Name and Extension number.
//
// Each field (name and number) is resolved independently, so a trunk could
// override just the number while letting the extension's name pass through.
func buildOutboundCallerID(ext *models.Extension, trunk *models.Trunk) (name, number string, source callerIDSource) {
	// Start with extension values as defaults.
	if ext != nil {
		name = ext.Name
		number = ext.Extension
	}
	source = callerIDFromExtension

	// Trunk caller ID overrides extension values when set.
	if trunk.CallerIDName != "" {
		name = trunk.CallerIDName
		source = callerIDFromTrunk
	}
	if trunk.CallerIDNum != "" {
		number = trunk.CallerIDNum
		source = callerIDFromTrunk
	}

	return name, number, source
}

// applyPrefixRules transforms a dialed number according to trunk prefix rules.
// It strips the configured number of leading digits, then prepends the configured
// prefix. For example, with PrefixStrip=1 and PrefixAdd="0044", dialing
// "07700900000" becomes "00447700900000".
func applyPrefixRules(number string, strip int, add string) string {
	if strip > 0 && strip < len(number) {
		number = number[strip:]
	} else if strip >= len(number) {
		number = ""
	}
	if add != "" {
		number = add + number
	}
	return number
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
	// Apply trunk prefix manipulation rules to the dialed number.
	dialedNumber := applyPrefixRules(ic.RequestURI, trunk.PrefixStrip, trunk.PrefixAdd)

	if dialedNumber != ic.RequestURI {
		h.logger.Debug("applied prefix rules to dialed number",
			"call_id", callID,
			"trunk", trunk.Name,
			"original", ic.RequestURI,
			"transformed", dialedNumber,
			"prefix_strip", trunk.PrefixStrip,
			"prefix_add", trunk.PrefixAdd,
		)
	}

	// Build the INVITE Request-URI: sip:<dialed_number>@<trunk_host>:<trunk_port>
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

	// Apply caller ID rules: trunk CID overrides extension CID.
	cidName, cidNum, cidSource := buildOutboundCallerID(ic.CallerExtension, trunk)
	from := &sip.FromHeader{
		DisplayName: cidName,
		Address: sip.Uri{
			Scheme: "sip",
			User:   cidNum,
			Host:   h.proxyIP,
		},
	}
	from.Params.Add("tag", sip.GenerateTagN(16))
	req.AppendHeader(from)

	h.logger.Debug("sending outbound invite to trunk",
		"call_id", callID,
		"trunk", trunk.Name,
		"recipient", recipientStr,
		"caller_id_name", cidName,
		"caller_id_num", cidNum,
		"caller_id_source", cidSource,
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
