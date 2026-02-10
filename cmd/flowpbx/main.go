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

	"github.com/flowpbx/flowpbx/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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

	// Placeholder: SIP stack initialization.
	slog.Info("sip stack initialization placeholder", "sip_port", cfg.SIPPort)

	// HTTP router.
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	// HTTP server with graceful shutdown.
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      r,
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

	slog.Info("shutting down http server")
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("http server shutdown error", "error", err)
		os.Exit(1)
	}

	slog.Info("flowpbx stopped")
}
