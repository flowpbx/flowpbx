package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/flowpbx/flowpbx/internal/api"
	"github.com/flowpbx/flowpbx/internal/api/middleware"
	"github.com/flowpbx/flowpbx/internal/config"
	"github.com/flowpbx/flowpbx/internal/database"
	sipserver "github.com/flowpbx/flowpbx/internal/sip"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Configure structured logging.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.SlogLevel()}))
	slog.SetDefault(logger)

	slog.Info("starting flowpbx",
		"http_port", cfg.HTTPPort,
		"sip_port", cfg.SIPPort,
		"data_dir", cfg.DataDir,
	)

	// Open database and run migrations.
	db, err := database.Open(cfg.DataDir)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Application context for background goroutines.
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	// Initialize SIP server.
	sipSrv, err := sipserver.NewServer(cfg, db)
	if err != nil {
		slog.Error("failed to create sip server", "error", err)
		os.Exit(1)
	}
	if err := sipSrv.Start(appCtx); err != nil {
		slog.Error("failed to start sip server", "error", err)
		os.Exit(1)
	}

	// Load system configuration from database.
	sysConfig, err := database.NewSystemConfigRepository(context.Background(), db)
	if err != nil {
		slog.Error("failed to load system config", "error", err)
		os.Exit(1)
	}

	// Session store for admin auth.
	sessions := middleware.NewSessionStore()
	middleware.StartCleanupTicker(appCtx, sessions, 15*time.Minute)

	// Create adapter for trunk status so the API can query SIP trunk state.
	trunkStatus := &trunkStatusAdapter{registrar: sipSrv.TrunkRegistrar()}

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

	// HTTP server using the api package.
	handler := api.NewServer(db, cfg, sessions, sysConfig, trunkStatus, enc)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine.
	errCh := make(chan error, 1)
	go func() {
		slog.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

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

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("http server shutdown error", "error", err)
		os.Exit(1)
	}

	slog.Info("flowpbx stopped")
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
