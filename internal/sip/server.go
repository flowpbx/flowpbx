package sip

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"sync"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	"github.com/flowpbx/flowpbx/internal/config"
	"github.com/flowpbx/flowpbx/internal/database"
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
	sessionMgr     *media.SessionManager
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	logger         *slog.Logger
}

// NewServer creates a SIP server with all handlers registered.
func NewServer(cfg *config.Config, db *database.DB) (*Server, error) {
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
	inviteHandler := NewInviteHandler(extensions, registrations, inboundNumbers, trunkRegistrar, auth, forker, dialogMgr, sessionMgr, proxyIP, logger)

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
		sessionMgr:     sessionMgr,
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
// Full BYE handling (tearing down both legs, releasing media, updating CDR)
// will be implemented in a subsequent sprint task.
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

	// Release media resources before terminating the dialog.
	if d.Media != nil {
		d.Media.Release()
		s.logger.Debug("media session released on bye",
			"call_id", callID,
		)
	}

	// Acknowledge the BYE with 200 OK.
	res := sip.NewResponseFromRequest(req, 200, "OK", nil)
	if err := tx.Respond(res); err != nil {
		s.logger.Error("failed to respond to bye", "error", err)
	}

	// Terminate the dialog and record hangup cause.
	s.dialogMgr.TerminateDialog(callID, "normal_clearing")

	// TODO: Send BYE to the other leg, create CDR.
}

// handleCANCEL processes incoming CANCEL requests when the caller hangs up
// before the call is answered. Full CANCEL handling (cancelling all forks)
// will be implemented in a subsequent sprint task.
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

	// TODO: Cancel all fork legs, send 487 to original INVITE transaction.
}

// DialogManager returns the call dialog tracker for querying active calls.
func (s *Server) DialogManager() *DialogManager {
	return s.dialogMgr
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

	// TODO: Route the DTMF event to the call's flow engine for IVR processing.
	// This will be connected in Phase 1C when the IVR DTMF collection is implemented.

	res := sip.NewResponseFromRequest(req, 200, "OK", nil)
	if err := tx.Respond(res); err != nil {
		s.logger.Error("failed to respond to info", "error", err)
	}
}
