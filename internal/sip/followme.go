package sip

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/flowpbx/flowpbx/internal/flow"
	"github.com/flowpbx/flowpbx/internal/media"
	"github.com/icholy/digest"
)

// RingFollowMe rings external follow-me numbers sequentially via an outbound
// trunk. Each number is tried in order with its configured delay and timeout.
// If any number answers, the call is bridged and a dialog is created.
func (a *FlowSIPActions) RingFollowMe(ctx context.Context, callCtx *flow.CallContext, numbers []models.FollowMeNumber, callerIDName string, callerIDNum string) (*flow.RingResult, error) {
	if callCtx.Request == nil || callCtx.Transaction == nil {
		return nil, fmt.Errorf("call context has no sip request or transaction")
	}

	if len(numbers) == 0 {
		return &flow.RingResult{Answered: false}, nil
	}

	if a.outboundRouter == nil {
		a.logger.Warn("follow-me cannot ring external numbers: no outbound router configured",
			"call_id", callCtx.CallID,
		)
		return &flow.RingResult{Answered: false}, nil
	}

	callID := callCtx.CallID

	a.logger.Info("follow-me sequential ring starting",
		"call_id", callID,
		"numbers", len(numbers),
	)

	// Select candidate trunks for outbound dialling.
	trunks, err := a.outboundRouter.SelectTrunks(ctx)
	if err != nil {
		a.logger.Warn("follow-me no trunks available",
			"call_id", callID,
			"error", err,
		)
		return &flow.RingResult{Answered: false}, nil
	}

	// Try each follow-me number sequentially.
	for i, fmNum := range numbers {
		if ctx.Err() != nil {
			return &flow.RingResult{Answered: false}, nil
		}

		a.logger.Info("follow-me trying external number",
			"call_id", callID,
			"number", fmNum.Number,
			"delay", fmNum.Delay,
			"timeout", fmNum.Timeout,
			"attempt", i+1,
			"total", len(numbers),
		)

		// Wait for the configured delay before ringing this number.
		if fmNum.Delay > 0 {
			select {
			case <-ctx.Done():
				return &flow.RingResult{Answered: false}, nil
			case <-time.After(time.Duration(fmNum.Delay) * time.Second):
			}
		}

		// Determine ring timeout for this number.
		ringTimeout := fmNum.Timeout
		if ringTimeout <= 0 {
			ringTimeout = 30
		}

		// Try to ring this external number via available trunks.
		result, err := a.ringExternalNumber(ctx, callCtx, trunks, fmNum.Number, ringTimeout, callerIDName, callerIDNum)
		if err != nil {
			a.logger.Warn("follow-me external number failed",
				"call_id", callID,
				"number", fmNum.Number,
				"error", err,
			)
			continue
		}

		if result.Answered {
			a.logger.Info("follow-me external number answered",
				"call_id", callID,
				"number", fmNum.Number,
			)
			return result, nil
		}

		a.logger.Info("follow-me external number not answered",
			"call_id", callID,
			"number", fmNum.Number,
		)
	}

	a.logger.Info("follow-me all external numbers exhausted",
		"call_id", callID,
	)
	return &flow.RingResult{Answered: false}, nil
}

// ringExternalNumber attempts to ring a single external number via the provided
// trunks with failover. If the number answers, it completes the media bridge,
// sends 200 OK to the caller, and creates a dialog.
func (a *FlowSIPActions) ringExternalNumber(
	ctx context.Context,
	callCtx *flow.CallContext,
	trunks []models.Trunk,
	number string,
	ringTimeout int,
	callerIDName string,
	callerIDNum string,
) (*flow.RingResult, error) {
	callID := callCtx.CallID
	req := callCtx.Request
	tx := callCtx.Transaction

	// Allocate media bridge for RTP proxying.
	var bridge *MediaBridge
	var trunkSDP []byte
	if len(req.Body()) > 0 && a.sessionMgr != nil {
		var err error
		bridge, trunkSDP, err = AllocateMediaBridge(a.sessionMgr, req.Body(), callID, a.proxyIP, a.logger)
		if err != nil {
			return nil, fmt.Errorf("allocating media bridge: %w", err)
		}
	}

	// Create a context with the ring timeout.
	ringCtx, cancelRing := context.WithTimeout(ctx, time.Duration(ringTimeout)*time.Second)

	// Register as pending so the CANCEL handler can abort.
	a.pendingMgr.Add(&PendingCall{
		CallID:     callID,
		CallerTx:   tx,
		CallerReq:  req,
		CancelFork: cancelRing,
		Bridge:     bridge,
	})

	// Try each trunk in priority order.
	var outResult *outboundResult
	var selectedTrunk *models.Trunk
	for i := range trunks {
		trunk := &trunks[i]

		// Enforce max_channels.
		if trunk.MaxChannels > 0 {
			active := a.dialogMgr.ActiveCallCountForTrunk(trunk.ID)
			if active >= trunk.MaxChannels {
				continue
			}
		}

		outResult = a.sendFollowMeInvite(ringCtx, req, tx, trunk, number, callID, trunkSDP, callerIDName, callerIDNum)

		if ringCtx.Err() != nil {
			break
		}

		if outResult.answered {
			selectedTrunk = trunk
			break
		}

		// Callee-level failures: don't retry on another trunk.
		if outResult.err == nil && isCalleeFailure(outResult.statusCode) {
			break
		}

		// Trunk-level failure: try next trunk.
		a.logger.Debug("follow-me trunk failed, trying next",
			"call_id", callID,
			"trunk", trunk.Name,
			"number", number,
		)
	}

	// Remove from pending calls.
	pc := a.pendingMgr.Remove(callID)
	cancelRing()

	// If pending call was cancelled by the CANCEL handler, clean up.
	if pc == nil {
		if outResult != nil && outResult.answered && outResult.tx != nil {
			outResult.tx.Terminate()
		}
		if bridge != nil {
			bridge.Release()
		}
		return &flow.RingResult{}, fmt.Errorf("call cancelled during follow-me ringing")
	}

	// No trunk answered.
	if outResult == nil || outResult.err != nil || !outResult.answered {
		if bridge != nil {
			bridge.Release()
		}
		return &flow.RingResult{Answered: false}, nil
	}

	// External number answered — complete media bridging.
	a.logger.Info("follow-me completing media bridge",
		"call_id", callID,
		"number", number,
		"trunk", selectedTrunk.Name,
	)

	// Send ACK to the trunk for its 200 OK.
	ackReq := buildACKFor2xx(outResult.req, outResult.res)
	if err := a.forker.Client().WriteRequest(ackReq); err != nil {
		a.logger.Error("failed to send ack to trunk for follow-me",
			"call_id", callID,
			"error", err,
		)
		outResult.tx.Terminate()
		if bridge != nil {
			bridge.Release()
		}
		return nil, fmt.Errorf("sending ack to trunk: %w", err)
	}

	// Complete media bridge with trunk's SDP.
	var mediaSession *media.MediaSession
	okBody := outResult.res.Body()
	if bridge != nil && len(outResult.res.Body()) > 0 {
		rewrittenForCaller, err := bridge.CompleteMediaBridge(outResult.res.Body())
		if err != nil {
			a.logger.Error("failed to complete media bridge for follow-me",
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
		a.logger.Error("failed to relay 200 ok to caller for follow-me",
			"call_id", callID,
			"error", err,
		)
		outResult.tx.Terminate()
		if mediaSession != nil {
			mediaSession.Release()
		}
		return nil, fmt.Errorf("relaying 200 ok: %w", err)
	}

	// Track the active call as a dialog.
	dialog := &Dialog{
		CallID:       callID,
		Direction:    CallTypeInbound,
		TrunkID:      selectedTrunk.ID,
		CallerIDName: callCtx.CallerIDName,
		CallerIDNum:  callCtx.CallerIDNum,
		CalledNum:    number,
		StartTime:    callCtx.StartTime,
		CallerTx:     tx,
		CallerReq:    req,
		CalleeTx:     outResult.tx,
		CalleeReq:    outResult.req,
		CalleeRes:    outResult.res,
		Media:        mediaSession,
		Caller:       CallLeg{},
		Callee: CallLeg{
			ContactURI: fmt.Sprintf("sip:%s:%d", selectedTrunk.Host, selectedTrunk.Port),
		},
	}

	// Extract dialog tags.
	if from := req.From(); from != nil {
		if tag, ok := from.Params.Get("tag"); ok {
			dialog.Caller.FromTag = tag
		}
	}
	if to := outResult.res.To(); to != nil {
		if tag, ok := to.Params.Get("tag"); ok {
			dialog.Callee.ToTag = tag
		}
	}
	if contact := outResult.res.Contact(); contact != nil {
		uri := contact.Address.Clone()
		dialog.Callee.RemoteTarget = uri
	}

	a.dialogMgr.CreateDialog(dialog)
	a.updateCDROnAnswer(callID)

	a.logger.Info("follow-me dialog established",
		"call_id", callID,
		"number", number,
		"trunk", selectedTrunk.Name,
		"trunk_id", selectedTrunk.ID,
		"media_bridged", mediaSession != nil,
	)

	return &flow.RingResult{Answered: true}, nil
}

// sendFollowMeInvite builds and sends an INVITE to a trunk for a follow-me
// external number. This is similar to sendOutboundInvite but adapted for the
// follow-me context (FlowSIPActions rather than InviteHandler).
func (a *FlowSIPActions) sendFollowMeInvite(
	ctx context.Context,
	callerReq *sip.Request,
	callerTx sip.ServerTransaction,
	trunk *models.Trunk,
	number string,
	callID string,
	sdpBody []byte,
	callerIDName string,
	callerIDNum string,
) *outboundResult {
	// Apply trunk prefix manipulation rules.
	dialedNumber := applyPrefixRules(number, trunk.PrefixStrip, trunk.PrefixAdd)

	// Build the INVITE Request-URI.
	recipientStr := fmt.Sprintf("sip:%s@%s:%d", dialedNumber, trunk.Host, trunk.Port)
	var recipient sip.Uri
	if err := sip.ParseUri(recipientStr, &recipient); err != nil {
		return &outboundResult{err: fmt.Errorf("parsing trunk uri: %w", err)}
	}

	req := sip.NewRequest(sip.INVITE, recipient)
	req.SetTransport(strings.ToUpper(trunk.Transport))

	// Set the SDP body.
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

	// Build caller ID: use the passed-in values unless trunk overrides.
	cidName := callerIDName
	cidNum := callerIDNum
	if trunk.CallerIDName != "" {
		cidName = trunk.CallerIDName
	}
	if trunk.CallerIDNum != "" {
		cidNum = trunk.CallerIDNum
	}

	from := &sip.FromHeader{
		DisplayName: cidName,
		Address: sip.Uri{
			Scheme: "sip",
			User:   cidNum,
			Host:   a.proxyIP,
		},
	}
	from.Params.Add("tag", sip.GenerateTagN(16))
	req.AppendHeader(from)

	a.logger.Debug("sending follow-me invite to trunk",
		"call_id", callID,
		"trunk", trunk.Name,
		"number", number,
		"recipient", recipientStr,
	)

	// Send the initial INVITE.
	inviteTx, err := a.forker.Client().TransactionRequest(ctx, req, sipgo.ClientRequestBuild)
	if err != nil {
		return &outboundResult{err: fmt.Errorf("sending invite to trunk: %w", err)}
	}

	// Collect responses.
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

		switch {
		case res.StatusCode == 100:
			continue

		case res.StatusCode == 180 || res.StatusCode == 183:
			// Relay first provisional response to the caller.
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
					a.logger.Error("failed to relay ringing to caller for follow-me",
						"call_id", callID,
						"error", err,
					)
				}
			}

		case res.StatusCode == 401 || res.StatusCode == 407:
			// Digest auth challenge from the trunk.
			inviteTx.Terminate()
			return a.handleFollowMeTrunkAuth(ctx, callerReq, callerTx, req, res, trunk, callID, body)

		case res.StatusCode >= 200 && res.StatusCode < 300:
			return &outboundResult{
				answered: true,
				res:      res,
				req:      req,
				tx:       inviteTx,
			}

		case res.StatusCode >= 300:
			inviteTx.Terminate()
			return &outboundResult{
				answered:   false,
				statusCode: res.StatusCode,
				reason:     res.Reason,
			}
		}
	}
}

// handleFollowMeTrunkAuth handles a 401/407 digest authentication challenge
// from a trunk during follow-me dialling.
func (a *FlowSIPActions) handleFollowMeTrunkAuth(
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

	authReq := origReq.Clone()
	authReq.RemoveHeader("Via")
	authReq.AppendHeader(sip.NewHeader(authzHeader, cred.String()))

	authTx, err := a.forker.Client().TransactionRequest(ctx, authReq,
		sipgo.ClientRequestIncreaseCSEQ,
		sipgo.ClientRequestAddVia,
	)
	if err != nil {
		return &outboundResult{err: fmt.Errorf("sending authenticated invite to trunk: %w", err)}
	}

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
					a.logger.Error("failed to relay ringing to caller for follow-me auth",
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

// followMeLegResult carries the outcome of one simultaneous follow-me leg.
type followMeLegResult struct {
	number string
	result *outboundResult
	trunk  *models.Trunk
	bridge *MediaBridge
	err    error
}

// RingFollowMeSimultaneous rings all external follow-me numbers simultaneously
// via outbound trunks. All numbers are dialled at once; the first to answer
// wins and all other legs are cancelled. Each leg gets its own media bridge
// allocation; only the winning leg's bridge is completed.
func (a *FlowSIPActions) RingFollowMeSimultaneous(ctx context.Context, callCtx *flow.CallContext, numbers []models.FollowMeNumber, callerIDName string, callerIDNum string) (*flow.RingResult, error) {
	if callCtx.Request == nil || callCtx.Transaction == nil {
		return nil, fmt.Errorf("call context has no sip request or transaction")
	}

	if len(numbers) == 0 {
		return &flow.RingResult{Answered: false}, nil
	}

	if a.outboundRouter == nil {
		a.logger.Warn("follow-me cannot ring external numbers: no outbound router configured",
			"call_id", callCtx.CallID,
		)
		return &flow.RingResult{Answered: false}, nil
	}

	callID := callCtx.CallID
	req := callCtx.Request
	tx := callCtx.Transaction

	a.logger.Info("follow-me simultaneous ring starting",
		"call_id", callID,
		"numbers", len(numbers),
	)

	// Select candidate trunks for outbound dialling.
	trunks, err := a.outboundRouter.SelectTrunks(ctx)
	if err != nil {
		a.logger.Warn("follow-me no trunks available",
			"call_id", callID,
			"error", err,
		)
		return &flow.RingResult{Answered: false}, nil
	}

	// Determine the maximum ring timeout across all numbers.
	maxTimeout := 30
	for _, n := range numbers {
		if n.Timeout > maxTimeout {
			maxTimeout = n.Timeout
		}
	}

	// Create a cancellable context for all legs. When one answers, cancel the rest.
	ringCtx, cancelRing := context.WithTimeout(ctx, time.Duration(maxTimeout)*time.Second)

	// Register as pending so the CANCEL handler can abort all legs.
	a.pendingMgr.Add(&PendingCall{
		CallID:     callID,
		CallerTx:   tx,
		CallerReq:  req,
		CancelFork: cancelRing,
	})

	// Launch all legs in parallel.
	resultCh := make(chan followMeLegResult, len(numbers))
	var wg sync.WaitGroup

	for _, fmNum := range numbers {
		wg.Add(1)
		go func(num models.FollowMeNumber) {
			defer wg.Done()
			a.ringFollowMeSimultaneousLeg(ringCtx, callCtx, trunks, num, callerIDName, callerIDNum, resultCh)
		}(fmNum)
	}

	// Close result channel when all legs finish.
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Collect results. First answer wins.
	var winner *followMeLegResult
	var losers []followMeLegResult

	for lr := range resultCh {
		if lr.err != nil {
			a.logger.Warn("follow-me simultaneous leg failed",
				"call_id", callID,
				"number", lr.number,
				"error", lr.err,
			)
			if lr.bridge != nil {
				lr.bridge.Release()
			}
			continue
		}

		if lr.result != nil && lr.result.answered && winner == nil {
			winner = &lr
			a.logger.Info("follow-me simultaneous leg answered",
				"call_id", callID,
				"number", lr.number,
			)
			// Cancel all other legs.
			cancelRing()
		} else {
			losers = append(losers, lr)
		}
	}

	// Clean up losing legs.
	for _, loser := range losers {
		if loser.result != nil && loser.result.answered && loser.result.tx != nil {
			loser.result.tx.Terminate()
		}
		if loser.bridge != nil {
			loser.bridge.Release()
		}
	}

	// Remove from pending calls.
	pc := a.pendingMgr.Remove(callID)
	cancelRing()

	// If pending call was cancelled by the CANCEL handler, clean up.
	if pc == nil {
		if winner != nil && winner.result.tx != nil {
			winner.result.tx.Terminate()
		}
		if winner != nil && winner.bridge != nil {
			winner.bridge.Release()
		}
		return &flow.RingResult{}, fmt.Errorf("call cancelled during follow-me ringing")
	}

	// No number answered.
	if winner == nil {
		a.logger.Info("follow-me simultaneous ring all numbers exhausted",
			"call_id", callID,
		)
		return &flow.RingResult{Answered: false}, nil
	}

	// Winner answered — complete media bridging.
	selectedTrunk := winner.trunk
	outResult := winner.result
	bridge := winner.bridge

	a.logger.Info("follow-me simultaneous completing media bridge",
		"call_id", callID,
		"number", winner.number,
		"trunk", selectedTrunk.Name,
	)

	// Send ACK to the trunk for its 200 OK.
	ackReq := buildACKFor2xx(outResult.req, outResult.res)
	if err := a.forker.Client().WriteRequest(ackReq); err != nil {
		a.logger.Error("failed to send ack to trunk for follow-me simultaneous",
			"call_id", callID,
			"error", err,
		)
		outResult.tx.Terminate()
		if bridge != nil {
			bridge.Release()
		}
		return nil, fmt.Errorf("sending ack to trunk: %w", err)
	}

	// Complete media bridge with trunk's SDP.
	var mediaSession *media.MediaSession
	okBody := outResult.res.Body()
	if bridge != nil && len(outResult.res.Body()) > 0 {
		rewrittenForCaller, err := bridge.CompleteMediaBridge(outResult.res.Body())
		if err != nil {
			a.logger.Error("failed to complete media bridge for follow-me simultaneous",
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
		a.logger.Error("failed to relay 200 ok to caller for follow-me simultaneous",
			"call_id", callID,
			"error", err,
		)
		outResult.tx.Terminate()
		if mediaSession != nil {
			mediaSession.Release()
		}
		return nil, fmt.Errorf("relaying 200 ok: %w", err)
	}

	// Track the active call as a dialog.
	dialog := &Dialog{
		CallID:       callID,
		Direction:    CallTypeInbound,
		TrunkID:      selectedTrunk.ID,
		CallerIDName: callCtx.CallerIDName,
		CallerIDNum:  callCtx.CallerIDNum,
		CalledNum:    winner.number,
		StartTime:    callCtx.StartTime,
		CallerTx:     tx,
		CallerReq:    req,
		CalleeTx:     outResult.tx,
		CalleeReq:    outResult.req,
		CalleeRes:    outResult.res,
		Media:        mediaSession,
		Caller:       CallLeg{},
		Callee: CallLeg{
			ContactURI: fmt.Sprintf("sip:%s:%d", selectedTrunk.Host, selectedTrunk.Port),
		},
	}

	// Extract dialog tags.
	if from := req.From(); from != nil {
		if tag, ok := from.Params.Get("tag"); ok {
			dialog.Caller.FromTag = tag
		}
	}
	if to := outResult.res.To(); to != nil {
		if tag, ok := to.Params.Get("tag"); ok {
			dialog.Callee.ToTag = tag
		}
	}
	if contact := outResult.res.Contact(); contact != nil {
		uri := contact.Address.Clone()
		dialog.Callee.RemoteTarget = uri
	}

	a.dialogMgr.CreateDialog(dialog)
	a.updateCDROnAnswer(callID)

	a.logger.Info("follow-me simultaneous dialog established",
		"call_id", callID,
		"number", winner.number,
		"trunk", selectedTrunk.Name,
		"trunk_id", selectedTrunk.ID,
		"media_bridged", mediaSession != nil,
	)

	return &flow.RingResult{Answered: true}, nil
}

// ringFollowMeSimultaneousLeg attempts to ring a single external number for
// a simultaneous follow-me ring. Each leg allocates its own media bridge and
// tries trunks in priority order. The result is sent to resultCh.
func (a *FlowSIPActions) ringFollowMeSimultaneousLeg(
	ctx context.Context,
	callCtx *flow.CallContext,
	trunks []models.Trunk,
	fmNum models.FollowMeNumber,
	callerIDName string,
	callerIDNum string,
	resultCh chan<- followMeLegResult,
) {
	callID := callCtx.CallID
	req := callCtx.Request

	// Apply per-number ring timeout. Default to 30s if unset.
	ringTimeout := fmNum.Timeout
	if ringTimeout <= 0 {
		ringTimeout = 30
	}

	legCtx, legCancel := context.WithTimeout(ctx, time.Duration(ringTimeout)*time.Second)
	defer legCancel()

	a.logger.Info("follow-me simultaneous leg starting",
		"call_id", callID,
		"number", fmNum.Number,
		"timeout", ringTimeout,
	)

	// Allocate a media bridge for this leg.
	var bridge *MediaBridge
	var trunkSDP []byte
	if len(req.Body()) > 0 && a.sessionMgr != nil {
		var err error
		bridge, trunkSDP, err = AllocateMediaBridge(a.sessionMgr, req.Body(), callID+"_fm_"+fmNum.Number, a.proxyIP, a.logger)
		if err != nil {
			resultCh <- followMeLegResult{number: fmNum.Number, err: fmt.Errorf("allocating media bridge: %w", err)}
			return
		}
	}

	// Try each trunk in priority order.
	var outResult *outboundResult
	var selectedTrunk *models.Trunk
	for i := range trunks {
		if legCtx.Err() != nil {
			break
		}

		trunk := &trunks[i]

		// Enforce max_channels.
		if trunk.MaxChannels > 0 {
			active := a.dialogMgr.ActiveCallCountForTrunk(trunk.ID)
			if active >= trunk.MaxChannels {
				continue
			}
		}

		outResult = a.sendFollowMeInvite(legCtx, req, callCtx.Transaction, trunk, fmNum.Number, callID, trunkSDP, callerIDName, callerIDNum)

		if legCtx.Err() != nil {
			break
		}

		if outResult.answered {
			selectedTrunk = trunk
			break
		}

		// Callee-level failures: don't retry on another trunk.
		if outResult.err == nil && isCalleeFailure(outResult.statusCode) {
			break
		}
	}

	if outResult == nil || outResult.err != nil || !outResult.answered {
		resultCh <- followMeLegResult{
			number: fmNum.Number,
			bridge: bridge,
			err:    fmt.Errorf("no trunk answered for %s", fmNum.Number),
		}
		return
	}

	resultCh <- followMeLegResult{
		number: fmNum.Number,
		result: outResult,
		trunk:  selectedTrunk,
		bridge: bridge,
	}
}
