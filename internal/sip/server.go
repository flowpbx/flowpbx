package sip

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	"github.com/flowpbx/flowpbx/internal/config"
	"github.com/flowpbx/flowpbx/internal/database"
	"github.com/flowpbx/flowpbx/internal/email"
	"github.com/flowpbx/flowpbx/internal/flow"
	"github.com/flowpbx/flowpbx/internal/flow/nodes"
	"github.com/flowpbx/flowpbx/internal/media"
)

// Server wraps the sipgo SIP stack with FlowPBX-specific handlers.
type Server struct {
	cfg            *config.Config
	ua             *sipgo.UserAgent
	srv            *sipgo.Server
	registrar      *Registrar
	trunkRegistrar *TrunkRegistrar
	inviteHandler  *InviteHandler
	forker         *Forker
	auth           *Authenticator
	dialogMgr      *DialogManager
	pendingMgr     *PendingCallManager
	sessionMgr     *media.SessionManager
	dtmfMgr        *media.CallDTMFManager
	cdrs           database.CDRRepository
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	logger         *slog.Logger
}

// NewServer creates a SIP server with all handlers registered.
func NewServer(cfg *config.Config, db *database.DB, enc *database.Encryptor, sysConfig database.SystemConfigRepository, emailSend *email.Sender) (*Server, error) {
	logger := slog.Default().With("component", "sip")

	ua, err := sipgo.NewUA(
		sipgo.WithUserAgent("FlowPBX"),
		sipgo.WithUserAgentHostname(cfg.SIPHost()),
	)
	if err != nil {
		return nil, fmt.Errorf("creating sip user agent: %w", err)
	}

	srv, err := sipgo.NewServer(ua,
		sipgo.WithServerLogger(logger),
	)
	if err != nil {
		ua.Close()
		return nil, fmt.Errorf("creating sip server: %w", err)
	}

	extensions := database.NewExtensionRepository(db)
	registrations := database.NewRegistrationRepository(db)
	inboundNumbers := database.NewInboundNumberRepository(db)
	trunks := database.NewTrunkRepository(db)

	auth := NewAuthenticator(extensions, logger)
	registrar := NewRegistrar(extensions, registrations, auth, logger)
	trunkRegistrar := NewTrunkRegistrar(ua, logger)

	forker, err := NewForker(ua, logger)
	if err != nil {
		srv.Close()
		ua.Close()
		return nil, fmt.Errorf("creating invite forker: %w", err)
	}

	// Create RTP media proxy and session manager.
	rtpProxy, err := media.NewProxy(cfg.RTPPortMin, cfg.RTPPortMax, logger)
	if err != nil {
		forker.Close()
		srv.Close()
		ua.Close()
		return nil, fmt.Errorf("creating rtp media proxy: %w", err)
	}

	sessionMgr := media.NewSessionManager(rtpProxy, logger)
	proxyIP := cfg.MediaIP()
	logger.Info("media proxy configured",
		"proxy_ip", proxyIP,
		"rtp_port_min", cfg.RTPPortMin,
		"rtp_port_max", cfg.RTPPortMax,
	)

	dialogMgr := NewDialogManager(logger)
	pendingMgr := NewPendingCallManager(logger)
	dtmfMgr := media.NewCallDTMFManager(logger)
	cdrs := database.NewCDRRepository(db)
	callFlows := database.NewCallFlowRepository(db)
	outboundRouter := NewOutboundRouter(trunks, trunkRegistrar, enc, logger)

	// Create conference manager for active conference room lifecycle.
	conferenceMgr := media.NewConferenceManager(rtpProxy, logger)

	// Create the flow engine for inbound call routing via visual flow graphs.
	voicemailMessages := database.NewVoicemailMessageRepository(db)
	flowEngine := flow.NewEngine(callFlows, cdrs, nil, logger)
	flowSIPActions := NewFlowSIPActions(extensions, registrations, forker, dialogMgr, pendingMgr, sessionMgr, dtmfMgr, conferenceMgr, cdrs, proxyIP, logger)
	nodes.RegisterAll(flowEngine, flowSIPActions, extensions, voicemailMessages, sysConfig, enc, emailSend, cfg.DataDir, logger)

	inviteHandler := NewInviteHandler(extensions, registrations, inboundNumbers, trunks, trunkRegistrar, auth, outboundRouter, forker, dialogMgr, pendingMgr, sessionMgr, cdrs, flowEngine, proxyIP, logger)

	s := &Server{
		cfg:            cfg,
		ua:             ua,
		srv:            srv,
		registrar:      registrar,
		trunkRegistrar: trunkRegistrar,
		inviteHandler:  inviteHandler,
		forker:         forker,
		auth:           auth,
		dialogMgr:      dialogMgr,
		pendingMgr:     pendingMgr,
		sessionMgr:     sessionMgr,
		dtmfMgr:        dtmfMgr,
		cdrs:           cdrs,
		logger:         logger,
	}

	s.registerHandlers()
	return s, nil
}

// registerHandlers attaches SIP method handlers to the server.
func (s *Server) registerHandlers() {
	s.srv.OnInvite(s.inviteHandler.HandleInvite)
	s.srv.OnRegister(s.registrar.HandleRegister)
	s.srv.OnAck(s.handleACK)
	s.srv.OnBye(s.handleBYE)
	s.srv.OnCancel(s.handleCANCEL)
	s.srv.OnOptions(s.handleOptions)
	s.srv.OnInfo(s.handleInfo)
}

// Start begins listening on configured transports. It blocks until the
// context is cancelled or a fatal listener error occurs.
func (s *Server) Start(ctx context.Context) error {
	ctx, s.cancel = context.WithCancel(ctx)

	udpAddr := fmt.Sprintf("0.0.0.0:%d", s.cfg.SIPPort)
	tcpAddr := fmt.Sprintf("0.0.0.0:%d", s.cfg.SIPPort)

	// Start UDP listener.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.logger.Info("sip udp listener starting", "addr", udpAddr)
		if err := s.srv.ListenAndServe(ctx, "udp", udpAddr); err != nil {
			s.logger.Error("sip udp listener stopped", "error", err)
		}
	}()

	// Start TCP listener.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.logger.Info("sip tcp listener starting", "addr", tcpAddr)
		if err := s.srv.ListenAndServe(ctx, "tcp", tcpAddr); err != nil {
			s.logger.Error("sip tcp listener stopped", "error", err)
		}
	}()

	// Start TLS listener if cert and key are configured.
	if s.cfg.TLSCert != "" && s.cfg.TLSKey != "" {
		tlsAddr := fmt.Sprintf("0.0.0.0:%d", s.cfg.SIPTLSPort)
		cert, err := tls.LoadX509KeyPair(s.cfg.TLSCert, s.cfg.TLSKey)
		if err != nil {
			s.cancel()
			return fmt.Errorf("loading tls certificate: %w", err)
		}

		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.logger.Info("sip tls listener starting", "addr", tlsAddr)
			if err := s.srv.ListenAndServeTLS(ctx, "tls", tlsAddr, tlsCfg); err != nil {
				s.logger.Error("sip tls listener stopped", "error", err)
			}
		}()
	}

	// WSS listener is reserved for Phase 2 (WebRTC). Log the reservation.
	s.logger.Info("sip wss listener reserved for phase 2",
		"port", 8089,
		"enabled", false,
	)

	// Start registration expiry cleanup.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.registrar.RunExpiryCleanup(ctx)
	}()

	// Start the RTP session reaper for orphaned media sessions.
	s.sessionMgr.StartReaper()

	return nil
}

// Stop gracefully shuts down all SIP listeners and waits for goroutines.
func (s *Server) Stop() {
	s.logger.Info("stopping sip server")
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	// Drain per-call DTMF buffers.
	if s.dtmfMgr != nil {
		s.dtmfMgr.Drain()
	}
	// Stop the session reaper and release all active media sessions.
	if s.sessionMgr != nil {
		s.sessionMgr.StopReaper()
		s.sessionMgr.ReleaseAll()
	}
	if s.forker != nil {
		s.forker.Close()
	}
	s.srv.Close()
	s.ua.Close()
	s.logger.Info("sip server stopped")
}

// TrunkRegistrar returns the trunk registration manager for querying status
// and managing trunk registrations.
func (s *Server) TrunkRegistrar() *TrunkRegistrar {
	return s.trunkRegistrar
}

// handleACK processes incoming ACK requests. Per RFC 3261 §13.2.2.4, when
// the PBX (as B2BUA) sends a 200 OK to the caller, the caller responds
// with an ACK to confirm the dialog. ACK requests are not transactional —
// they have no response.
func (s *Server) handleACK(req *sip.Request, tx sip.ServerTransaction) {
	callID := ""
	if cid := req.CallID(); cid != nil {
		callID = cid.Value()
	}

	s.logger.Debug("sip ack received",
		"call_id", callID,
		"from", req.From().Address.User,
		"source", req.Source(),
	)

	// Verify the ACK matches an active dialog.
	if d := s.dialogMgr.GetDialog(callID); d != nil {
		s.logger.Debug("ack matched active dialog",
			"call_id", callID,
			"caller", d.CallerIDNum,
			"callee", d.CalledNum,
		)
	} else {
		s.logger.Debug("ack for unknown dialog (may be pre-dialog or stale)",
			"call_id", callID,
		)
	}
}

// handleBYE processes incoming BYE requests to terminate an active call.
// It identifies which leg sent the BYE, tears down the other leg, releases
// media resources, and creates a CDR record.
func (s *Server) handleBYE(req *sip.Request, tx sip.ServerTransaction) {
	callID := ""
	if cid := req.CallID(); cid != nil {
		callID = cid.Value()
	}

	s.logger.Info("sip bye received",
		"call_id", callID,
		"from", req.From().Address.User,
		"source", req.Source(),
	)

	// Look up the active dialog for this call.
	d := s.dialogMgr.GetDialog(callID)
	if d == nil {
		s.logger.Warn("bye for unknown dialog",
			"call_id", callID,
		)
		res := sip.NewResponseFromRequest(req, 481, "Call/Transaction Does Not Exist", nil)
		if err := tx.Respond(res); err != nil {
			s.logger.Error("failed to respond to bye", "error", err)
		}
		return
	}

	// Acknowledge the BYE with 200 OK.
	res := sip.NewResponseFromRequest(req, 200, "OK", nil)
	if err := tx.Respond(res); err != nil {
		s.logger.Error("failed to respond to bye", "error", err)
	}

	// Determine which leg sent the BYE and send BYE to the other leg.
	fromTag := ""
	if from := req.From(); from != nil {
		if tag, ok := from.Params.Get("tag"); ok {
			fromTag = tag
		}
	}

	hangupCause := "normal_clearing"
	callerHangup := fromTag == d.Caller.FromTag || fromTag == ""

	if callerHangup {
		s.logger.Debug("bye from caller, sending bye to callee",
			"call_id", callID,
		)
		s.sendBYEToCallee(d)
		hangupCause = "caller_bye"
	} else {
		s.logger.Debug("bye from callee, sending bye to caller",
			"call_id", callID,
		)
		s.sendBYEToCaller(d)
		hangupCause = "callee_bye"
	}

	// Release media resources.
	if d.Media != nil {
		d.Media.Release()
		s.logger.Debug("media session released on bye",
			"call_id", callID,
		)
	}

	// Terminate the dialog.
	terminated := s.dialogMgr.TerminateDialog(callID, hangupCause)
	if terminated == nil {
		return
	}

	// Create CDR record.
	s.finalizeCDR(terminated)
}

// sendBYEToCallee sends a BYE request to the callee (answering device).
// The BYE is constructed as an in-dialog request using the dialog parameters
// from the original INVITE and 200 OK exchange.
func (s *Server) sendBYEToCallee(d *Dialog) {
	if d.CalleeReq == nil {
		s.logger.Warn("cannot send bye to callee: no callee request stored",
			"call_id", d.CallID,
		)
		return
	}

	byeReq := s.buildInDialogBYE(
		d.CalleeReq,
		d.CalleeRes,
		d.Callee.RemoteTarget,
	)

	if err := s.forker.Client().WriteRequest(byeReq); err != nil {
		s.logger.Error("failed to send bye to callee",
			"call_id", d.CallID,
			"error", err,
		)
	} else {
		s.logger.Debug("bye sent to callee",
			"call_id", d.CallID,
		)
	}
}

// sendBYEToCaller sends a BYE request to the caller (originating device).
// The BYE is constructed as an in-dialog request using the dialog parameters
// from the original INVITE.
func (s *Server) sendBYEToCaller(d *Dialog) {
	if d.CallerReq == nil {
		s.logger.Warn("cannot send bye to caller: no caller request stored",
			"call_id", d.CallID,
		)
		return
	}

	// For the caller leg, we build a BYE as a UAS sending to the UAC.
	// The roles are reversed: the From/To are swapped relative to the original INVITE.
	byeReq := s.buildReverseDialogBYE(d.CallerReq)

	if err := s.forker.Client().WriteRequest(byeReq); err != nil {
		s.logger.Error("failed to send bye to caller",
			"call_id", d.CallID,
			"error", err,
		)
	} else {
		s.logger.Debug("bye sent to caller",
			"call_id", d.CallID,
		)
	}
}

// buildInDialogBYE creates a BYE request within an established dialog on the
// outbound (callee) leg. The Request-URI is the Contact from the callee's 200 OK
// (remoteTarget), and dialog headers match the original INVITE/response exchange.
func (s *Server) buildInDialogBYE(
	inviteReq *sip.Request,
	inviteResp *sip.Response,
	remoteTarget *sip.Uri,
) *sip.Request {
	// Request-URI: Contact from the callee's 200 OK, or original INVITE recipient.
	recipient := &inviteReq.Recipient
	if remoteTarget != nil {
		recipient = remoteTarget
	}

	bye := sip.NewRequest(sip.BYE, *recipient.Clone())
	bye.SipVersion = inviteReq.SipVersion

	// From: same as the original INVITE (our side of the dialog).
	if h := inviteReq.From(); h != nil {
		bye.AppendHeader(sip.HeaderClone(h))
	}

	// To: from the response (includes remote tag).
	if inviteResp != nil {
		if h := inviteResp.To(); h != nil {
			bye.AppendHeader(sip.HeaderClone(h))
		}
	} else if h := inviteReq.To(); h != nil {
		bye.AppendHeader(sip.HeaderClone(h))
	}

	// Call-ID: same as the dialog.
	if h := inviteReq.CallID(); h != nil {
		bye.AppendHeader(sip.HeaderClone(h))
	}

	// CSeq: new sequence number, method BYE.
	cseq := &sip.CSeqHeader{
		SeqNo:      2,
		MethodName: sip.BYE,
	}
	bye.AppendHeader(cseq)

	maxFwd := sip.MaxForwardsHeader(70)
	bye.AppendHeader(&maxFwd)

	bye.SetTransport(inviteReq.Transport())
	bye.SetSource(inviteReq.Source())

	return bye
}

// buildReverseDialogBYE creates a BYE request to the caller (originating side).
// Since the PBX is the UAS for the caller's INVITE, the From/To headers are
// swapped: our To becomes From, and the caller's From becomes To.
func (s *Server) buildReverseDialogBYE(callerReq *sip.Request) *sip.Request {
	// Request-URI: the Contact from the caller's INVITE (where to send BYE).
	recipient := &callerReq.Recipient
	if contact := callerReq.Contact(); contact != nil {
		recipient = &contact.Address
	}

	bye := sip.NewRequest(sip.BYE, *recipient.Clone())
	bye.SipVersion = callerReq.SipVersion

	// From/To swapped: we are now the initiator of BYE.
	// From = original To (PBX side), To = original From (caller side).
	if h := callerReq.To(); h != nil {
		fromHeader := h.AsFrom()
		bye.AppendHeader(&fromHeader)
	}
	if h := callerReq.From(); h != nil {
		toHeader := h.AsTo()
		bye.AppendHeader(&toHeader)
	}

	// Call-ID: same as the dialog.
	if h := callerReq.CallID(); h != nil {
		bye.AppendHeader(sip.HeaderClone(h))
	}

	// CSeq: new sequence number for this direction.
	cseq := &sip.CSeqHeader{
		SeqNo:      1,
		MethodName: sip.BYE,
	}
	bye.AppendHeader(cseq)

	maxFwd := sip.MaxForwardsHeader(70)
	bye.AppendHeader(&maxFwd)

	bye.SetTransport(callerReq.Transport())
	bye.SetSource(callerReq.Source())

	return bye
}

// finalizeCDR updates the CDR that was created at call start with hangup
// information from the terminated dialog.
func (s *Server) finalizeCDR(d *Dialog) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cdr, err := s.cdrs.GetByCallID(ctx, d.CallID)
	if err != nil {
		s.logger.Error("failed to fetch cdr for finalization",
			"call_id", d.CallID,
			"error", err,
		)
		return
	}
	if cdr == nil {
		s.logger.Warn("no cdr found to finalize",
			"call_id", d.CallID,
		)
		return
	}

	durationSec := int(d.Duration().Seconds())
	billableSec := int(d.BillableDuration().Seconds())

	cdr.AnswerTime = d.AnswerTime
	cdr.EndTime = d.EndTime
	cdr.Duration = &durationSec
	cdr.BillableDur = &billableSec
	cdr.Disposition = d.Disposition()
	cdr.HangupCause = d.HangupCause

	if err := s.cdrs.Update(ctx, cdr); err != nil {
		s.logger.Error("failed to finalize cdr",
			"call_id", d.CallID,
			"error", err,
		)
		return
	}

	s.logger.Info("cdr finalized",
		"call_id", d.CallID,
		"cdr_id", cdr.ID,
		"direction", cdr.Direction,
		"disposition", cdr.Disposition,
		"duration", durationSec,
		"billable", billableSec,
	)
}

// handleCANCEL processes incoming CANCEL requests when the caller hangs up
// before the call is answered. Per RFC 3261 §9.2, the server responds 200 OK
// to the CANCEL, cancels all forked INVITE legs, and sends 487 Request
// Terminated on the original INVITE server transaction.
func (s *Server) handleCANCEL(req *sip.Request, tx sip.ServerTransaction) {
	callID := ""
	if cid := req.CallID(); cid != nil {
		callID = cid.Value()
	}

	s.logger.Info("sip cancel received",
		"call_id", callID,
		"from", req.From().Address.User,
		"source", req.Source(),
	)

	// Respond 200 OK to the CANCEL itself (per RFC 3261 §9.2).
	res := sip.NewResponseFromRequest(req, 200, "OK", nil)
	if err := tx.Respond(res); err != nil {
		s.logger.Error("failed to respond to cancel", "error", err)
	}

	// Cancel the pending call: abort all fork legs, release media, send 487.
	if s.pendingMgr.Cancel(callID, s.logger) {
		s.logger.Info("pending call cancelled",
			"call_id", callID,
		)

		// Finalize the CDR for the cancelled call.
		s.finalizeCancelledCDR(callID)
		return
	}

	// If no pending call found, check if it's an answered call (the caller
	// sent CANCEL after the callee answered but before ACK was processed).
	// In that case, treat it like a BYE.
	if d := s.dialogMgr.GetDialog(callID); d != nil {
		s.logger.Info("cancel for answered call, treating as bye",
			"call_id", callID,
		)
		s.sendBYEToCallee(d)
		if d.Media != nil {
			d.Media.Release()
		}
		terminated := s.dialogMgr.TerminateDialog(callID, "caller_cancel")
		if terminated != nil {
			s.finalizeCDR(terminated)
		}
		return
	}

	s.logger.Warn("cancel for unknown call",
		"call_id", callID,
	)
}

// finalizeCancelledCDR updates the CDR for a call that was cancelled
// by the caller before being answered.
func (s *Server) finalizeCancelledCDR(callID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cdr, err := s.cdrs.GetByCallID(ctx, callID)
	if err != nil {
		s.logger.Error("failed to fetch cdr for cancelled call",
			"call_id", callID,
			"error", err,
		)
		return
	}
	if cdr == nil {
		s.logger.Warn("no cdr found for cancelled call",
			"call_id", callID,
		)
		return
	}

	now := time.Now()
	durationSec := int(now.Sub(cdr.StartTime).Seconds())
	billableSec := 0
	cdr.EndTime = &now
	cdr.Duration = &durationSec
	cdr.BillableDur = &billableSec
	cdr.Disposition = "cancelled"
	cdr.HangupCause = "caller_cancel"

	if err := s.cdrs.Update(ctx, cdr); err != nil {
		s.logger.Error("failed to finalize cdr for cancelled call",
			"call_id", callID,
			"error", err,
		)
		return
	}

	s.logger.Info("cdr finalized for cancelled call",
		"call_id", callID,
		"cdr_id", cdr.ID,
		"disposition", cdr.Disposition,
	)
}

// DialogManager returns the call dialog tracker for querying active calls.
func (s *Server) DialogManager() *DialogManager {
	return s.dialogMgr
}

// PendingCallManager returns the pending call tracker for querying ringing calls.
func (s *Server) PendingCallManager() *PendingCallManager {
	return s.pendingMgr
}

// CallDTMFManager returns the per-call DTMF buffer manager for injecting
// and collecting DTMF digits during IVR operations.
func (s *Server) CallDTMFManager() *media.CallDTMFManager {
	return s.dtmfMgr
}

// handleOptions responds to SIP OPTIONS requests (keepalive pings from
// trunks and phones).
func (s *Server) handleOptions(req *sip.Request, tx sip.ServerTransaction) {
	s.logger.Debug("sip options received",
		"from", req.From().Address.User,
		"source", req.Source(),
	)

	res := sip.NewResponseFromRequest(req, 200, "OK", nil)
	res.AppendHeader(sip.NewHeader("Accept", "application/sdp"))
	res.AppendHeader(sip.NewHeader("Allow", "INVITE, ACK, CANCEL, BYE, REGISTER, OPTIONS, INFO"))

	if err := tx.Respond(res); err != nil {
		s.logger.Error("failed to respond to options", "error", err)
	}
}

// handleInfo processes SIP INFO requests. Currently detects DTMF digits
// sent via SIP INFO as a fallback for endpoints that do not support
// RFC 2833 telephone-event.
func (s *Server) handleInfo(req *sip.Request, tx sip.ServerTransaction) {
	callID := ""
	if cid := req.CallID(); cid != nil {
		callID = cid.Value()
	}

	ct := req.ContentType()
	if ct == nil {
		s.logger.Debug("sip info without content-type, ignoring",
			"call_id", callID,
			"source", req.Source(),
		)
		res := sip.NewResponseFromRequest(req, 200, "OK", nil)
		if err := tx.Respond(res); err != nil {
			s.logger.Error("failed to respond to info", "error", err)
		}
		return
	}

	dtmfInfo, err := media.ParseSIPInfoDTMF(ct.Value(), req.Body())
	if err != nil {
		// Not a DTMF INFO — respond 200 OK but don't process further.
		s.logger.Debug("sip info with unsupported content type",
			"content_type", ct.Value(),
			"call_id", callID,
			"source", req.Source(),
		)
		res := sip.NewResponseFromRequest(req, 200, "OK", nil)
		if err := tx.Respond(res); err != nil {
			s.logger.Error("failed to respond to info", "error", err)
		}
		return
	}

	s.logger.Info("sip info dtmf received",
		"signal", dtmfInfo.Signal,
		"duration", dtmfInfo.Duration,
		"call_id", callID,
		"source", req.Source(),
	)

	// Route the DTMF digit to the call's per-call buffer for IVR collection.
	if s.dtmfMgr != nil {
		s.dtmfMgr.Inject(callID, dtmfInfo.Signal)
	}

	res := sip.NewResponseFromRequest(req, 200, "OK", nil)
	if err := tx.Respond(res); err != nil {
		s.logger.Error("failed to respond to info", "error", err)
	}
}
