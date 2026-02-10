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

	inviteHandler := NewInviteHandler(extensions, registrations, inboundNumbers, trunkRegistrar, auth, forker, logger)

	s := &Server{
		cfg:            cfg,
		ua:             ua,
		srv:            srv,
		registrar:      registrar,
		trunkRegistrar: trunkRegistrar,
		inviteHandler:  inviteHandler,
		forker:         forker,
		auth:           auth,
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

	return nil
}

// Stop gracefully shuts down all SIP listeners and waits for goroutines.
func (s *Server) Stop() {
	s.logger.Info("stopping sip server")
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
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
// they have no response. For now we log receipt; dialog state tracking
// (matching ACK to active calls) will be implemented in dialog.go.
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

	// TODO: Match the ACK to the active call dialog and confirm the
	// caller leg of the call. This will be connected when dialog.go
	// implements call state tracking.
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
