package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/crypto/acme/autocert"

	"github.com/flowpbx/flowpbx/internal/api"
	"github.com/flowpbx/flowpbx/internal/api/middleware"
	"github.com/flowpbx/flowpbx/internal/config"
	"github.com/flowpbx/flowpbx/internal/database"
	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/flowpbx/flowpbx/internal/email"
	"github.com/flowpbx/flowpbx/internal/media"
	"github.com/flowpbx/flowpbx/internal/prompts"
	"github.com/flowpbx/flowpbx/internal/recording"
	sipserver "github.com/flowpbx/flowpbx/internal/sip"
	"github.com/flowpbx/flowpbx/internal/voicemail"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Configure structured logging (text or json format, configurable level).
	logger := slog.New(cfg.SlogHandler(os.Stdout))
	slog.SetDefault(logger)

	slog.Info("starting flowpbx",
		"http_port", cfg.HTTPPort,
		"sip_port", cfg.SIPPort,
		"data_dir", cfg.DataDir,
		"tls", cfg.TLSEnabled(),
	)

	// Open database and run migrations.
	db, err := database.Open(cfg.DataDir)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Extract embedded system prompts to data directory on first boot.
	if err := prompts.ExtractToDataDir(cfg.DataDir); err != nil {
		slog.Error("failed to extract system prompts", "error", err)
		os.Exit(1)
	}

	// Initialize encryptor for sensitive database fields (trunk passwords).
	var enc *database.Encryptor
	if keyBytes, err := cfg.EncryptionKeyBytes(); err != nil {
		slog.Error("failed to decode encryption key", "error", err)
		os.Exit(1)
	} else if keyBytes != nil {
		enc, err = database.NewEncryptor(keyBytes)
		if err != nil {
			slog.Error("failed to create encryptor", "error", err)
			os.Exit(1)
		}
		slog.Info("field encryption enabled")
	} else {
		slog.Warn("no encryption key configured, trunk passwords will be stored in plaintext")
	}

	// Application context for background goroutines.
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	// Load system configuration from database.
	sysConfig, err := database.NewSystemConfigRepository(context.Background(), db)
	if err != nil {
		slog.Error("failed to load system config", "error", err)
		os.Exit(1)
	}

	// Create email sender for voicemail notifications.
	emailSend := email.NewSender(slog.Default())

	// Initialize SIP server.
	sipSrv, err := sipserver.NewServer(cfg, db, enc, sysConfig, emailSend)
	if err != nil {
		slog.Error("failed to create sip server", "error", err)
		os.Exit(1)
	}
	if err := sipSrv.Start(appCtx); err != nil {
		slog.Error("failed to start sip server", "error", err)
		os.Exit(1)
	}

	// Session store for admin auth.
	sessions := middleware.NewSessionStore()
	middleware.StartCleanupTicker(appCtx, sessions, 15*time.Minute)

	// Voicemail retention cleanup: delete messages older than per-box retention_days.
	voicemail.StartCleanupTicker(appCtx, db, 1*time.Hour)

	// Recording retention cleanup: delete recordings older than recording_max_days setting.
	recording.StartCleanupTicker(appCtx, db, sysConfig, 1*time.Hour)

	// Create adapter for trunk status so the API can query SIP trunk state.
	trunkStatus := &trunkStatusAdapter{registrar: sipSrv.TrunkRegistrar()}

	// Load all enabled trunks and begin registration / health checks.
	loadTrunks(appCtx, db, sipSrv.TrunkRegistrar(), enc)

	// Create adapter for trunk testing so the API can trigger one-shot SIP tests.
	trunkTester := &trunkTesterAdapter{registrar: sipSrv.TrunkRegistrar()}

	// Create adapter for trunk lifecycle so the API can start/stop registration
	// when trunks are created, updated (enabled/disabled), or deleted.
	trunkLifecycle := &trunkLifecycleAdapter{registrar: sipSrv.TrunkRegistrar(), enc: enc}

	// Create adapter for active calls so the API can query SIP call state.
	activeCalls := &activeCallsAdapter{
		dialogMgr:  sipSrv.DialogManager(),
		pendingMgr: sipSrv.PendingCallManager(),
	}

	// Config reloader for hot-reload without restart.
	reloader := &configReloader{
		db:        db,
		registrar: sipSrv.TrunkRegistrar(),
		enc:       enc,
	}

	// Create adapter for conference management so the API can mute/unmute participants.
	conferenceProv := &conferenceProviderAdapter{mgr: sipSrv.ConferenceManager()}

	// Create adapter for SIP message tracing verbosity control via API.
	sipLogVerbosity := &sipLogVerbosityAdapter{tracer: sipSrv.MessageTracer()}

	// HTTP server using the api package.
	handler := api.NewServer(db, cfg, sessions, sysConfig, trunkStatus, trunkTester, trunkLifecycle, activeCalls, conferenceProv, enc, reloader, sipLogVerbosity)

	srv := &http.Server{
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Optional HTTP→HTTPS redirect server (started when TLS is enabled).
	var redirectSrv *http.Server

	errCh := make(chan error, 1)

	switch {
	case cfg.ACMEDomain != "":
		// Automatic TLS via Let's Encrypt (ACME).
		cacheDir := filepath.Join(cfg.DataDir, "acme-certs")
		m := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(cfg.ACMEDomain),
			Cache:      autocert.DirCache(cacheDir),
			Email:      cfg.ACMEEmail,
		}
		srv.Addr = ":443"
		srv.TLSConfig = m.TLSConfig()

		// The ACME manager needs to handle HTTP-01 challenges on port 80.
		// Non-challenge requests are redirected to HTTPS.
		redirectSrv = &http.Server{
			Addr:         ":80",
			Handler:      m.HTTPHandler(middleware.HTTPSRedirectHandler()),
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
			IdleTimeout:  30 * time.Second,
		}

		go func() {
			slog.Info("https server listening (acme)", "addr", srv.Addr, "domain", cfg.ACMEDomain)
			if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				errCh <- err
			}
		}()
		go func() {
			slog.Info("http redirect server listening", "addr", redirectSrv.Addr)
			if err := redirectSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("http redirect server error", "error", err)
			}
		}()

	case cfg.TLSCert != "":
		// Manual TLS certificate.
		srv.Addr = fmt.Sprintf(":%d", cfg.HTTPPort)
		srv.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		// Start HTTP→HTTPS redirect on port 80 unless the main port is 80.
		if cfg.HTTPPort != 80 {
			redirectSrv = &http.Server{
				Addr:         ":80",
				Handler:      middleware.HTTPSRedirectHandler(),
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 5 * time.Second,
				IdleTimeout:  30 * time.Second,
			}
			go func() {
				slog.Info("http redirect server listening", "addr", redirectSrv.Addr)
				if err := redirectSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					slog.Error("http redirect server error", "error", err)
				}
			}()
		}

		go func() {
			slog.Info("https server listening", "addr", srv.Addr)
			if err := srv.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey); err != nil && err != http.ErrServerClosed {
				errCh <- err
			}
		}()

	default:
		// Plain HTTP (no TLS configured).
		srv.Addr = fmt.Sprintf(":%d", cfg.HTTPPort)
		go func() {
			slog.Info("http server listening", "addr", srv.Addr)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				errCh <- err
			}
		}()
	}

	// Wait for interrupt or server error.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		slog.Info("received shutdown signal", "signal", sig.String())
	case err := <-errCh:
		slog.Error("http server error", "error", err)
	}

	// Graceful shutdown with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	slog.Info("shutting down servers")
	sipSrv.Stop()

	if redirectSrv != nil {
		if err := redirectSrv.Shutdown(ctx); err != nil {
			slog.Error("http redirect server shutdown error", "error", err)
		}
	}

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("http server shutdown error", "error", err)
		os.Exit(1)
	}

	slog.Info("flowpbx stopped")
}

// loadTrunks queries the database for all enabled trunks and starts their
// registration or health check loops. Register-type trunks have their
// passwords decrypted before being handed to the SIP trunk registrar.
func loadTrunks(ctx context.Context, db *database.DB, registrar *sipserver.TrunkRegistrar, enc *database.Encryptor) {
	trunks := database.NewTrunkRepository(db)
	enabled, err := trunks.ListEnabled(ctx)
	if err != nil {
		slog.Error("failed to load enabled trunks", "error", err)
		return
	}

	if len(enabled) == 0 {
		slog.Info("no enabled trunks to load")
		return
	}

	slog.Info("loading enabled trunks", "count", len(enabled))

	for _, trunk := range enabled {
		switch trunk.Type {
		case "register":
			// Decrypt password before starting registration.
			if trunk.Password != "" && enc != nil {
				decrypted, err := enc.Decrypt(trunk.Password)
				if err != nil {
					slog.Error("failed to decrypt trunk password, skipping",
						"trunk", trunk.Name,
						"trunk_id", trunk.ID,
						"error", err,
					)
					continue
				}
				trunk.Password = decrypted
			}
			if err := registrar.StartTrunk(ctx, trunk); err != nil {
				slog.Error("failed to start trunk registration",
					"trunk", trunk.Name,
					"trunk_id", trunk.ID,
					"error", err,
				)
			}
		case "ip":
			if err := registrar.StartHealthCheck(ctx, trunk); err != nil {
				slog.Error("failed to start trunk health check",
					"trunk", trunk.Name,
					"trunk_id", trunk.ID,
					"error", err,
				)
			}
		default:
			slog.Warn("unknown trunk type, skipping",
				"trunk", trunk.Name,
				"trunk_id", trunk.ID,
				"type", trunk.Type,
			)
		}
	}
}

// trunkStatusAdapter bridges the SIP trunk registrar with the API's
// TrunkStatusProvider interface, converting between SIP and API types.
type trunkStatusAdapter struct {
	registrar *sipserver.TrunkRegistrar
}

func (a *trunkStatusAdapter) GetTrunkStatus(trunkID int64) (api.TrunkStatusEntry, bool) {
	st, ok := a.registrar.GetStatus(trunkID)
	if !ok {
		return api.TrunkStatusEntry{}, false
	}
	return api.TrunkStatusEntry{
		TrunkID:        st.TrunkID,
		Name:           st.Name,
		Type:           st.Type,
		Status:         string(st.Status),
		LastError:      st.LastError,
		RetryAttempt:   st.RetryAttempt,
		FailedAt:       st.FailedAt,
		RegisteredAt:   st.RegisteredAt,
		ExpiresAt:      st.ExpiresAt,
		LastOptionsAt:  st.LastOptionsAt,
		OptionsHealthy: st.OptionsHealthy,
	}, true
}

func (a *trunkStatusAdapter) GetAllTrunkStatuses() []api.TrunkStatusEntry {
	states := a.registrar.GetAllStatuses()
	entries := make([]api.TrunkStatusEntry, len(states))
	for i, st := range states {
		entries[i] = api.TrunkStatusEntry{
			TrunkID:        st.TrunkID,
			Name:           st.Name,
			Type:           st.Type,
			Status:         string(st.Status),
			LastError:      st.LastError,
			RetryAttempt:   st.RetryAttempt,
			FailedAt:       st.FailedAt,
			RegisteredAt:   st.RegisteredAt,
			ExpiresAt:      st.ExpiresAt,
			LastOptionsAt:  st.LastOptionsAt,
			OptionsHealthy: st.OptionsHealthy,
		}
	}
	return entries
}

// trunkTesterAdapter bridges the SIP trunk registrar with the API's
// TrunkTester interface for one-shot connectivity tests.
type trunkTesterAdapter struct {
	registrar *sipserver.TrunkRegistrar
}

func (a *trunkTesterAdapter) TestRegister(ctx context.Context, trunk models.Trunk) error {
	return a.registrar.TestRegister(ctx, trunk)
}

func (a *trunkTesterAdapter) SendOptions(ctx context.Context, trunk models.Trunk) error {
	return a.registrar.SendOptions(ctx, trunk)
}

// trunkLifecycleAdapter bridges the SIP trunk registrar with the API's
// TrunkLifecycleManager interface for starting/stopping trunk registration
// on config changes.
type trunkLifecycleAdapter struct {
	registrar *sipserver.TrunkRegistrar
	enc       *database.Encryptor
}

func (a *trunkLifecycleAdapter) StartTrunk(ctx context.Context, trunk models.Trunk) error {
	switch trunk.Type {
	case "register":
		// Decrypt password before starting registration.
		if trunk.Password != "" && a.enc != nil {
			decrypted, err := a.enc.Decrypt(trunk.Password)
			if err != nil {
				return fmt.Errorf("decrypting trunk password: %w", err)
			}
			trunk.Password = decrypted
		}
		return a.registrar.StartTrunk(ctx, trunk)
	case "ip":
		return a.registrar.StartHealthCheck(ctx, trunk)
	default:
		return fmt.Errorf("unknown trunk type %q", trunk.Type)
	}
}

func (a *trunkLifecycleAdapter) StopTrunk(trunkID int64) {
	a.registrar.StopTrunk(trunkID)
}

// activeCallsAdapter bridges the SIP dialog and pending call managers with
// the API's ActiveCallsProvider interface, combining ringing and answered
// calls into a unified view.
type activeCallsAdapter struct {
	dialogMgr  *sipserver.DialogManager
	pendingMgr *sipserver.PendingCallManager
}

func (a *activeCallsAdapter) GetActiveCalls() []api.ActiveCallEntry {
	now := time.Now()
	var entries []api.ActiveCallEntry

	// Add answered (in-dialog) calls.
	for _, d := range a.dialogMgr.ActiveCalls() {
		entry := api.ActiveCallEntry{
			CallID:       d.CallID,
			State:        string(d.State),
			Direction:    string(d.Direction),
			CallerIDName: d.CallerIDName,
			CallerIDNum:  d.CallerIDNum,
			CalledNum:    d.CalledNum,
			StartTime:    d.StartTime,
			AnswerTime:   d.AnswerTime,
			DurationSec:  int(now.Sub(d.StartTime).Seconds()),
		}
		entries = append(entries, entry)
	}

	// Add pending (ringing) calls.
	for _, pc := range a.pendingMgr.PendingCalls() {
		entry := api.ActiveCallEntry{
			CallID: pc.CallID,
			State:  "ringing",
		}
		if pc.CallerReq != nil {
			if from := pc.CallerReq.From(); from != nil {
				entry.CallerIDName = from.DisplayName
				entry.CallerIDNum = from.Address.User
			}
			entry.CalledNum = pc.CallerReq.Recipient.User
			entry.StartTime = now // approximate; pending calls don't track start time
			entry.DurationSec = 0
		}
		entries = append(entries, entry)
	}

	return entries
}

func (a *activeCallsAdapter) GetActiveCallCount() int {
	return a.dialogMgr.ActiveCallCount() + a.pendingMgr.PendingCallCount()
}

// configReloader implements api.ConfigReloader. It stops all trunk
// registrations and health checks, then reloads enabled trunks from
// the database.
type configReloader struct {
	db        *database.DB
	registrar *sipserver.TrunkRegistrar
	enc       *database.Encryptor
}

func (cr *configReloader) Reload(ctx context.Context) error {
	// Stop all running trunks.
	stopped := cr.registrar.StopAllTrunks()
	slog.Info("reload: stopped trunks", "count", len(stopped))

	// Reload enabled trunks from the database.
	loadTrunks(ctx, cr.db, cr.registrar, cr.enc)

	slog.Info("reload: trunks reloaded")
	return nil
}

// conferenceProviderAdapter bridges the media.ConferenceManager with the
// API's ConferenceProvider interface for runtime conference control.
type conferenceProviderAdapter struct {
	mgr *media.ConferenceManager
}

func (a *conferenceProviderAdapter) MuteParticipant(bridgeID int64, participantID string, muted bool) error {
	return a.mgr.MuteParticipant(bridgeID, participantID, muted)
}

func (a *conferenceProviderAdapter) KickParticipant(bridgeID int64, participantID string) error {
	return a.mgr.Kick(bridgeID, participantID)
}

func (a *conferenceProviderAdapter) Participants(bridgeID int64) ([]api.ConferenceParticipantEntry, error) {
	participants, err := a.mgr.Participants(bridgeID)
	if err != nil {
		return nil, err
	}
	entries := make([]api.ConferenceParticipantEntry, len(participants))
	for i, p := range participants {
		entries[i] = api.ConferenceParticipantEntry{
			ID:           p.ID,
			CallerIDName: p.CallerIDName,
			CallerIDNum:  p.CallerIDNum,
			JoinedAt:     p.JoinedAt,
			Muted:        p.Muted,
		}
	}
	return entries, nil
}

// sipLogVerbosityAdapter bridges the SIP message tracer with the API's
// SIPLogVerbositySetter interface for runtime verbosity control.
type sipLogVerbosityAdapter struct {
	tracer *sipserver.MessageTracer
}

func (a *sipLogVerbosityAdapter) SetSIPLogVerbosity(level string) {
	a.tracer.SetVerbosity(sipserver.ParseSIPLogVerbosity(level))
}
