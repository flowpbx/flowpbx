package sip

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	"github.com/flowpbx/flowpbx/internal/database/models"
)

// ForkResult describes the outcome of a forked INVITE attempt.
type ForkResult struct {
	// Answered is true if at least one fork received a 200 OK.
	Answered bool

	// AnsweringContact is the registration that answered (sent 200 OK).
	AnsweringContact *models.Registration

	// AnswerResponse is the 200 OK response from the answering device.
	AnswerResponse *sip.Response

	// AnsweringTx is the client transaction for the answered fork,
	// which the caller must ACK.
	AnsweringTx sip.ClientTransaction

	// AnsweringLeg contains the fork leg details for the answering device,
	// including the original INVITE request needed to build the ACK.
	AnsweringLeg *forkLeg

	// AllBusy is true if every fork responded with 486 Busy Here.
	AllBusy bool

	// Error is set if the fork failed for a non-SIP reason (e.g. transport error).
	Error error
}

// forkLeg represents a single outbound INVITE leg to one registered contact.
type forkLeg struct {
	contact models.Registration
	tx      sip.ClientTransaction
	req     *sip.Request
}

// forkLegResponse pairs a response (or error) with the fork leg it came from.
type forkLegResponse struct {
	leg *forkLeg
	res *sip.Response
	err error
}

// Forker manages parallel INVITE forking to multiple registered contacts.
// It sends INVITE to all contacts simultaneously (ring-all strategy) and
// relays provisional responses (180/183) back to the caller's server
// transaction. The first 200 OK wins; all other forks are cancelled.
type Forker struct {
	ua     *sipgo.UserAgent
	client *sipgo.Client
	logger *slog.Logger
}

// NewForker creates a new INVITE forker.
func NewForker(ua *sipgo.UserAgent, logger *slog.Logger) (*Forker, error) {
	client, err := sipgo.NewClient(ua,
		sipgo.WithClientLogger(logger.With("subsystem", "forker")),
	)
	if err != nil {
		return nil, fmt.Errorf("creating sip client for forker: %w", err)
	}

	return &Forker{
		ua:     ua,
		client: client,
		logger: logger.With("subsystem", "forker"),
	}, nil
}

// Close releases the forker's SIP client resources.
func (f *Forker) Close() {
	f.client.Close()
}

// Client returns the underlying SIP client for sending out-of-transaction
// requests (e.g. ACK for 2xx responses). Per RFC 3261 §13.2.2.4, the ACK
// for a 2xx is sent by the UAC core directly, not via a client transaction.
func (f *Forker) Client() *sipgo.Client {
	return f.client
}

// Fork sends INVITE requests to all contacts in parallel (ring-all strategy).
// It relays 180 Ringing / 183 Session Progress back to the caller via the
// inbound server transaction (callerTx). The first 200 OK from any fork wins;
// all other forks are immediately cancelled via CANCEL.
//
// If sdpBody is non-nil, it overrides the SDP body from the incoming INVITE
// for all forked legs. This is used by the media proxy to rewrite SDP so that
// the callee's RTP is directed to the proxy rather than the caller directly.
//
// The caller is responsible for:
//   - Sending ACK for the winning fork's 200 OK
//   - Setting up media bridging
//   - Managing BYE teardown
//
// The ctx should be cancelled to abort all forks (e.g. on caller CANCEL or timeout).
func (f *Forker) Fork(
	ctx context.Context,
	incomingReq *sip.Request,
	callerTx sip.ServerTransaction,
	contacts []models.Registration,
	callerExt *models.Extension,
	callID string,
	sdpBody []byte,
) *ForkResult {
	if len(contacts) == 0 {
		return &ForkResult{Error: fmt.Errorf("no contacts to fork to")}
	}

	// Create a cancellable context for all fork legs. When one answers,
	// we cancel the rest.
	forkCtx, forkCancel := context.WithCancel(ctx)
	defer forkCancel()

	// Launch all fork legs.
	legs := make([]*forkLeg, 0, len(contacts))
	for i := range contacts {
		leg, err := f.createLeg(forkCtx, incomingReq, &contacts[i], callerExt, callID, sdpBody)
		if err != nil {
			f.logger.Error("failed to create fork leg",
				"call_id", callID,
				"contact", contacts[i].ContactURI,
				"error", err,
			)
			continue
		}
		legs = append(legs, leg)
	}

	if len(legs) == 0 {
		return &ForkResult{Error: fmt.Errorf("failed to create any fork legs")}
	}

	f.logger.Info("forked invite to contacts",
		"call_id", callID,
		"legs", len(legs),
	)

	// Collect responses from all legs. First 200 OK wins.
	responseCh := make(chan forkLegResponse, len(legs)*4) // buffer for multiple provisional + final
	var wg sync.WaitGroup

	for _, leg := range legs {
		wg.Add(1)
		go func(l *forkLeg) {
			defer wg.Done()
			f.collectResponses(forkCtx, l, responseCh)
		}(leg)
	}

	// Close response channel when all collectors finish.
	go func() {
		wg.Wait()
		close(responseCh)
	}()

	// Track state across all forks.
	ringingRelayed := false
	busyCount := 0
	failedCount := 0
	totalLegs := len(legs)
	var winningLeg *forkLeg
	var winningResponse *sip.Response

	for lr := range responseCh {
		if lr.err != nil {
			f.logger.Debug("fork leg error",
				"call_id", callID,
				"contact", lr.leg.contact.ContactURI,
				"error", lr.err,
			)
			failedCount++
			if busyCount+failedCount >= totalLegs {
				break
			}
			continue
		}

		res := lr.res
		f.logger.Debug("fork leg response",
			"call_id", callID,
			"contact", lr.leg.contact.ContactURI,
			"status", res.StatusCode,
			"reason", res.Reason,
		)

		switch {
		case res.StatusCode == 100:
			// 100 Trying — absorb (we already sent our own 100 Trying).
			continue

		case res.StatusCode == 180 || res.StatusCode == 183:
			// Relay provisional responses back to the caller.
			//
			// 180 Ringing: relayed once so the caller's phone generates local ringback.
			// 183 Session Progress: may carry an SDP body for early media (remote
			// ringback tones or call progress announcements). The SDP body must be
			// copied so the caller can set up the media path before the call is answered.
			//
			// Only the first provisional is relayed to avoid confusing the caller UA
			// with multiple provisional SDP offers.
			if !ringingRelayed {
				ringingRelayed = true

				// Include the SDP body for 183 early media; 180 typically has no body.
				var body []byte
				if res.StatusCode == 183 && len(res.Body()) > 0 {
					body = res.Body()
				}

				ringing := sip.NewResponseFromRequest(incomingReq, res.StatusCode, res.Reason, body)

				// Copy Content-Type when relaying an SDP body.
				if body != nil {
					if ct := res.ContentType(); ct != nil {
						ringing.AppendHeader(sip.NewHeader("Content-Type", ct.Value()))
					}
				}

				if err := callerTx.Respond(ringing); err != nil {
					f.logger.Error("failed to relay ringing to caller",
						"call_id", callID,
						"error", err,
					)
				} else {
					f.logger.Info("relayed ringing to caller",
						"call_id", callID,
						"status", res.StatusCode,
						"has_sdp", body != nil,
					)
				}
			}

		case res.StatusCode >= 200 && res.StatusCode < 300:
			// 200 OK — first answering device wins.
			winningLeg = lr.leg
			winningResponse = res
			f.logger.Info("fork answered",
				"call_id", callID,
				"contact", lr.leg.contact.ContactURI,
				"status", res.StatusCode,
			)
			// Cancel all other forks.
			forkCancel()
			goto answered

		case res.StatusCode == 486:
			// Busy — track it.
			busyCount++
			f.logger.Debug("fork leg busy",
				"call_id", callID,
				"contact", lr.leg.contact.ContactURI,
			)
			if busyCount+failedCount >= totalLegs {
				break
			}

		case res.StatusCode == 487:
			// Request Terminated — expected after CANCEL.
			failedCount++
			if busyCount+failedCount >= totalLegs {
				break
			}

		case res.StatusCode >= 400:
			// Other failure — count it.
			failedCount++
			f.logger.Debug("fork leg failed",
				"call_id", callID,
				"contact", lr.leg.contact.ContactURI,
				"status", res.StatusCode,
				"reason", res.Reason,
			)
			if busyCount+failedCount >= totalLegs {
				break
			}
		}
	}

	// No fork answered — cancel remaining and return result.
	forkCancel()
	f.cancelLegs(legs, nil)
	f.terminateLegs(legs, nil)

	if busyCount == totalLegs {
		return &ForkResult{AllBusy: true}
	}
	return &ForkResult{Answered: false}

answered:
	// Cancel and terminate all non-winning legs.
	f.cancelLegs(legs, winningLeg)
	f.terminateLegs(legs, winningLeg)

	return &ForkResult{
		Answered:         true,
		AnsweringContact: &winningLeg.contact,
		AnswerResponse:   winningResponse,
		AnsweringTx:      winningLeg.tx,
		AnsweringLeg:     winningLeg,
	}
}

// createLeg builds and sends a forked INVITE to one registered contact.
// If sdpBody is non-nil, it is used instead of the incoming INVITE's SDP.
func (f *Forker) createLeg(
	ctx context.Context,
	incomingReq *sip.Request,
	contact *models.Registration,
	callerExt *models.Extension,
	callID string,
	sdpBody []byte,
) (*forkLeg, error) {
	// Parse the contact URI as the Request-URI for the outbound INVITE.
	var recipient sip.Uri
	if err := sip.ParseUri(contact.ContactURI, &recipient); err != nil {
		return nil, fmt.Errorf("parsing contact uri %q: %w", contact.ContactURI, err)
	}

	// For NAT traversal, use the source IP:port from the registration
	// rather than the Contact URI host, since the phone may be behind NAT.
	if contact.SourceIP != "" && contact.SourcePort > 0 {
		recipient.Host = contact.SourceIP
		recipient.Port = contact.SourcePort
	}

	req := sip.NewRequest(sip.INVITE, recipient)
	req.SetTransport(transportForContact(contact))

	// Set the SDP body. If an sdpBody override is provided (rewritten by the
	// media proxy), use it; otherwise copy from the incoming INVITE.
	body := sdpBody
	if body == nil {
		body = incomingReq.Body()
	}
	if len(body) > 0 {
		req.SetBody(body)
		req.AppendHeader(sip.NewHeader("Content-Type", "application/sdp"))
	}

	// Set caller ID in the From header (the SIP client will populate From
	// via ClientRequestBuild, but we set display name for caller ID).
	// The From header is built by sipgo's ClientRequestBuild from the UA,
	// so we add caller information as custom headers that phones understand.
	if callerExt != nil {
		req.AppendHeader(sip.NewHeader("X-Caller-Name", callerExt.Name))
		req.AppendHeader(sip.NewHeader("X-Caller-Ext", callerExt.Extension))
	}

	// Preserve the original Call-ID so both legs share the same call identifier
	// for logging and CDR correlation. Note: in a full B2BUA implementation
	// each leg would have its own Call-ID; for now we share it.
	if cid := incomingReq.CallID(); cid != nil {
		req.AppendHeader(sip.NewHeader("Call-ID", cid.Value()))
	}

	tx, err := f.client.TransactionRequest(ctx, req, sipgo.ClientRequestBuild)
	if err != nil {
		return nil, fmt.Errorf("sending invite to %s: %w", contact.ContactURI, err)
	}

	return &forkLeg{
		contact: *contact,
		tx:      tx,
		req:     req,
	}, nil
}

// collectResponses reads responses from a fork leg's client transaction
// and sends them to the shared response channel.
func (f *Forker) collectResponses(ctx context.Context, leg *forkLeg, ch chan<- forkLegResponse) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-leg.tx.Done():
			if err := leg.tx.Err(); err != nil {
				ch <- forkLegResponse{leg: leg, err: err}
			}
			return
		case res, ok := <-leg.tx.Responses():
			if !ok {
				return
			}
			ch <- forkLegResponse{leg: leg, res: res}
			// Final response — stop collecting.
			if res.StatusCode >= 200 {
				return
			}
		}
	}
}

// cancelLegs sends CANCEL to all fork legs except the winner.
func (f *Forker) cancelLegs(legs []*forkLeg, winner *forkLeg) {
	for _, leg := range legs {
		if leg == winner {
			continue
		}
		// Build CANCEL from the original INVITE request.
		cancelReq := sip.NewRequest(sip.CANCEL, leg.req.Recipient)
		cancelReq.SetTransport(leg.req.Transport())

		// CANCEL must have the same Call-ID, From, and To as the INVITE.
		if cid := leg.req.CallID(); cid != nil {
			cancelReq.AppendHeader(sip.NewHeader("Call-ID", cid.Value()))
		}

		cancelCtx := context.Background()
		cancelTx, err := f.client.TransactionRequest(cancelCtx, cancelReq, sipgo.ClientRequestBuild)
		if err != nil {
			f.logger.Debug("failed to send cancel for fork leg",
				"contact", leg.contact.ContactURI,
				"error", err,
			)
			continue
		}
		cancelTx.Terminate()
	}
}

// terminateLegs terminates all fork leg transactions except the winner.
func (f *Forker) terminateLegs(legs []*forkLeg, winner *forkLeg) {
	for _, leg := range legs {
		if leg == winner {
			continue
		}
		leg.tx.Terminate()
	}
}

// transportForContact returns the SIP transport to use for a registration.
func transportForContact(contact *models.Registration) string {
	switch contact.Transport {
	case "tcp":
		return "TCP"
	case "tls":
		return "TLS"
	case "wss":
		return "WSS"
	default:
		return "UDP"
	}
}
