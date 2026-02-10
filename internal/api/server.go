package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/flowpbx/flowpbx/internal/api/middleware"
	"github.com/flowpbx/flowpbx/internal/config"
	"github.com/flowpbx/flowpbx/internal/database"
	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// Server holds HTTP handler dependencies and the chi router.
type Server struct {
	router       *chi.Mux
	db           *database.DB
	cfg          *config.Config
	sessions     *middleware.SessionStore
	adminUsers   database.AdminUserRepository
	systemConfig database.SystemConfigRepository
}

// NewServer creates the HTTP handler with all routes mounted.
func NewServer(db *database.DB, cfg *config.Config, sessions *middleware.SessionStore, sysConfig database.SystemConfigRepository) *Server {
	s := &Server{
		router:       chi.NewRouter(),
		db:           db,
		cfg:          cfg,
		sessions:     sessions,
		adminUsers:   database.NewAdminUserRepository(db),
		systemConfig: sysConfig,
	}

	s.routes()
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// routes configures all middleware and mounts all route groups.
func (s *Server) routes() {
	r := s.router

	// Global middleware stack.
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(middleware.CORS(middleware.ParseCORSOrigins(s.cfg.CORSOrigins)))
	r.Use(middleware.StructuredLogger)
	r.Use(middleware.Recoverer)

	// API routes under /api/v1.
	r.Route("/api/v1", func(r chi.Router) {
		// Unauthenticated routes.
		r.Get("/health", s.handleHealth)
		r.Post("/setup", s.handleSetup)

		// Auth routes (login is unauthenticated, logout/me require auth).
		r.Post("/auth/login", s.handleLogin)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(s.sessions, s.cfg.TLSCert != ""))
			r.Post("/auth/logout", s.handleLogout)
			r.Get("/auth/me", s.handleMe)
		})

		// Protected admin routes — auth middleware will be added in a
		// subsequent sprint task when session middleware is implemented.
		r.Route("/extensions", func(r chi.Router) {
			r.Get("/", s.handleNotImplemented)
			r.Post("/", s.handleNotImplemented)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.handleNotImplemented)
				r.Put("/", s.handleNotImplemented)
				r.Delete("/", s.handleNotImplemented)
				r.Get("/registrations", s.handleNotImplemented)
			})
		})

		r.Route("/trunks", func(r chi.Router) {
			r.Get("/", s.handleNotImplemented)
			r.Post("/", s.handleNotImplemented)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.handleNotImplemented)
				r.Put("/", s.handleNotImplemented)
				r.Delete("/", s.handleNotImplemented)
				r.Post("/test", s.handleNotImplemented)
			})
		})

		r.Route("/numbers", func(r chi.Router) {
			r.Get("/", s.handleNotImplemented)
			r.Post("/", s.handleNotImplemented)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.handleNotImplemented)
				r.Put("/", s.handleNotImplemented)
				r.Delete("/", s.handleNotImplemented)
			})
		})

		r.Route("/voicemail-boxes", func(r chi.Router) {
			r.Get("/", s.handleNotImplemented)
			r.Post("/", s.handleNotImplemented)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.handleNotImplemented)
				r.Put("/", s.handleNotImplemented)
				r.Delete("/", s.handleNotImplemented)
				r.Get("/messages", s.handleNotImplemented)
				r.Post("/greeting", s.handleNotImplemented)
				r.Route("/messages/{msgID}", func(r chi.Router) {
					r.Delete("/", s.handleNotImplemented)
					r.Put("/read", s.handleNotImplemented)
					r.Get("/audio", s.handleNotImplemented)
				})
			})
		})

		r.Route("/ring-groups", func(r chi.Router) {
			r.Get("/", s.handleNotImplemented)
			r.Post("/", s.handleNotImplemented)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.handleNotImplemented)
				r.Put("/", s.handleNotImplemented)
				r.Delete("/", s.handleNotImplemented)
			})
		})

		r.Route("/ivr-menus", func(r chi.Router) {
			r.Get("/", s.handleNotImplemented)
			r.Post("/", s.handleNotImplemented)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.handleNotImplemented)
				r.Put("/", s.handleNotImplemented)
				r.Delete("/", s.handleNotImplemented)
			})
		})

		r.Route("/time-switches", func(r chi.Router) {
			r.Get("/", s.handleNotImplemented)
			r.Post("/", s.handleNotImplemented)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.handleNotImplemented)
				r.Put("/", s.handleNotImplemented)
				r.Delete("/", s.handleNotImplemented)
			})
		})

		r.Route("/conferences", func(r chi.Router) {
			r.Get("/", s.handleNotImplemented)
			r.Post("/", s.handleNotImplemented)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.handleNotImplemented)
				r.Put("/", s.handleNotImplemented)
				r.Delete("/", s.handleNotImplemented)
			})
		})

		r.Route("/flows", func(r chi.Router) {
			r.Get("/", s.handleNotImplemented)
			r.Post("/", s.handleNotImplemented)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.handleNotImplemented)
				r.Put("/", s.handleNotImplemented)
				r.Delete("/", s.handleNotImplemented)
				r.Post("/publish", s.handleNotImplemented)
				r.Post("/validate", s.handleNotImplemented)
			})
		})

		r.Route("/cdrs", func(r chi.Router) {
			r.Get("/", s.handleNotImplemented)
			r.Get("/export", s.handleNotImplemented)
			r.Get("/{id}", s.handleNotImplemented)
		})

		r.Route("/recordings", func(r chi.Router) {
			r.Get("/", s.handleNotImplemented)
			r.Get("/{id}/download", s.handleNotImplemented)
			r.Delete("/{id}", s.handleNotImplemented)
		})

		r.Route("/prompts", func(r chi.Router) {
			r.Get("/", s.handleNotImplemented)
			r.Post("/", s.handleNotImplemented)
			r.Get("/{id}/audio", s.handleNotImplemented)
			r.Delete("/{id}", s.handleNotImplemented)
		})

		r.Get("/settings", s.handleNotImplemented)
		r.Put("/settings", s.handleNotImplemented)

		r.Route("/system", func(r chi.Router) {
			r.Get("/status", s.handleNotImplemented)
			r.Post("/reload", s.handleNotImplemented)
		})

		r.Get("/dashboard/stats", s.handleNotImplemented)

		r.Route("/calls", func(r chi.Router) {
			r.Get("/active", s.handleNotImplemented)
			r.Post("/{id}/hangup", s.handleNotImplemented)
			r.Post("/{id}/transfer", s.handleNotImplemented)
		})

		// Mobile app endpoints.
		r.Route("/app", func(r chi.Router) {
			r.Post("/auth", s.handleNotImplemented)
			r.Get("/me", s.handleNotImplemented)
			r.Put("/me", s.handleNotImplemented)
			r.Get("/voicemail", s.handleNotImplemented)
			r.Put("/voicemail/{id}/read", s.handleNotImplemented)
			r.Get("/voicemail/{id}/audio", s.handleNotImplemented)
			r.Get("/history", s.handleNotImplemented)
			r.Post("/push-token", s.handleNotImplemented)
		})
	})

	// SPA fallback — serve embedded React UI for non-API routes.
	// This will be wired to //go:embed static file serving in a later task.
	// For now, return a placeholder so the route structure is established.
	r.NotFound(s.handleSPAFallback)

	slog.Info("api routes mounted")
}

// handleHealth returns basic health status including first-boot detection.
// Unauthenticated so the SPA can determine whether to show setup wizard or login.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	needsSetup, err := s.isFirstBoot(r.Context())
	if err != nil {
		slog.Error("health: failed to check first-boot status", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "ok",
		"needs_setup": needsSetup,
	})
}

// isFirstBoot returns true when the admin_users table is empty, indicating
// the system has not been configured yet and the setup wizard should run.
func (s *Server) isFirstBoot(ctx context.Context) (bool, error) {
	count, err := s.adminUsers.Count(ctx)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

// handleSetup completes the first-boot setup wizard by creating the initial
// admin account and saving system configuration (hostname, SIP ports).
// Only allowed when the system is in first-boot state (no admin users exist).
func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	needsSetup, err := s.isFirstBoot(r.Context())
	if err != nil {
		slog.Error("setup: failed to check first-boot status", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !needsSetup {
		writeError(w, http.StatusForbidden, "setup already completed")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Hostname string `json:"hostname"`
		SIPPort  int    `json:"sip_port"`
		SIPTLS   int    `json:"sip_tls_port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields.
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	// Hash the admin password with Argon2id.
	hash, err := database.HashPassword(req.Password)
	if err != nil {
		slog.Error("setup: failed to hash password", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Create the admin user.
	user := &models.AdminUser{
		Username:     req.Username,
		PasswordHash: hash,
	}
	if err := s.adminUsers.Create(r.Context(), user); err != nil {
		slog.Error("setup: failed to create admin user", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create admin account")
		return
	}

	// Store system configuration values (only if provided).
	ctx := r.Context()
	if req.Hostname != "" {
		if err := s.systemConfig.Set(ctx, "hostname", req.Hostname); err != nil {
			slog.Error("setup: failed to save hostname", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to save configuration")
			return
		}
	}
	if req.SIPPort > 0 {
		if err := s.systemConfig.Set(ctx, "sip_port", strconv.Itoa(req.SIPPort)); err != nil {
			slog.Error("setup: failed to save sip port", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to save configuration")
			return
		}
	}
	if req.SIPTLS > 0 {
		if err := s.systemConfig.Set(ctx, "sip_tls_port", strconv.Itoa(req.SIPTLS)); err != nil {
			slog.Error("setup: failed to save sip tls port", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to save configuration")
			return
		}
	}

	slog.Info("setup: initial configuration completed", "username", req.Username, "user_id", user.ID)

	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":  user.ID,
		"username": user.Username,
	})
}

// handleLogin validates admin credentials and creates a session.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	user, err := s.adminUsers.GetByUsername(r.Context(), req.Username)
	if err != nil {
		slog.Error("login: failed to query user", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if user == nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	match, err := database.CheckPassword(req.Password, user.PasswordHash)
	if err != nil {
		slog.Error("login: failed to verify password", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !match {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	sess, err := s.sessions.Create(user.ID, user.Username)
	if err != nil {
		slog.Error("login: failed to create session", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	middleware.SetSessionCookie(w, sess, s.cfg.TLSCert != "")

	slog.Info("admin login", "username", user.Username, "user_id", user.ID)

	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":  user.ID,
		"username": user.Username,
	})
}

// handleLogout destroys the current session and clears cookies.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	sessionID := middleware.SessionIDFromContext(r.Context())
	if sessionID != "" {
		s.sessions.Delete(sessionID)
	}

	middleware.ClearSessionCookie(w, s.cfg.TLSCert != "")

	writeJSON(w, http.StatusOK, nil)
}

// handleMe returns the currently authenticated admin user.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user := middleware.AdminUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":  user.ID,
		"username": user.Username,
	})
}

// handleNotImplemented returns 501 for endpoints not yet wired up.
func (s *Server) handleNotImplemented(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

// handleSPAFallback serves the embedded React SPA for non-API routes.
// Will be replaced with //go:embed static file serving in a later task.
func (s *Server) handleSPAFallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("<!doctype html><html><body><p>FlowPBX UI not built yet. Run <code>make ui-build</code>.</p></body></html>"))
}
