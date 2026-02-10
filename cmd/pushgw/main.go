package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/flowpbx/flowpbx/internal/pushgw"
	"github.com/flowpbx/flowpbx/internal/pushgw/pgstore"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	httpPort := flag.Int("http-port", 8081, "HTTP server listen port")
	dbDSN := flag.String("db-dsn", "", "PostgreSQL connection string (e.g. postgres://user:pass@host/pushgw)")
	logLevel := flag.String("log-level", "info", "log level (debug, info, warn, error)")
	flag.Parse()

	// Configure structured logging.
	var level slog.Level
	switch *logLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	slog.Info("starting pushgw", "http_port", *httpPort)

	// Open PostgreSQL store if DSN is provided; otherwise handlers
	// that require the store will return 503.
	var store *pgstore.Store
	if *dbDSN != "" {
		var err error
		store, err = pgstore.New(*dbDSN)
		if err != nil {
			slog.Error("failed to open postgresql store", "error", err)
			os.Exit(1)
		}
		defer store.Close()
	} else {
		slog.Warn("no --db-dsn provided, license and push logging endpoints will be unavailable")
	}

	// Create the push gateway server.
	// The FCM/APNs sender will be wired up in a subsequent sprint task.
	var licenseStore pushgw.LicenseStore
	var pushLog pushgw.PushLogger
	if store != nil {
		licenseStore = store
		pushLog = store
	}
	gwServer := pushgw.NewServer(licenseStore, nil, pushLog)

	// HTTP router with global middleware.
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// Health check.
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	// Mount push gateway routes.
	r.Mount("/", gwServer)

	// HTTP server with graceful shutdown.
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", *httpPort),
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

	slog.Info("pushgw stopped")
}
