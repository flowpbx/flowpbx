package api

import (
	"log/slog"
	"net/http"

	"github.com/flowpbx/flowpbx/internal/config"
	"github.com/flowpbx/flowpbx/internal/database"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server holds HTTP handler dependencies and the chi router.
type Server struct {
	router *chi.Mux
	db     *database.DB
	cfg    *config.Config
}

// NewServer creates the HTTP handler with all routes mounted.
func NewServer(db *database.DB, cfg *config.Config) *Server {
	s := &Server{
		router: chi.NewRouter(),
		db:     db,
		cfg:    cfg,
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
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// API routes under /api/v1.
	r.Route("/api/v1", func(r chi.Router) {
		// Unauthenticated routes.
		r.Get("/health", s.handleHealth)
		r.Post("/setup", s.handleSetup)

		// Auth routes (login is unauthenticated, logout/me require auth).
		r.Post("/auth/login", s.handleLogin)
		r.Post("/auth/logout", s.handleLogout)
		r.Get("/auth/me", s.handleMe)

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

// handleHealth returns basic health status. Unauthenticated.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleSetup is a placeholder for the first-boot setup wizard endpoint.
func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "setup not implemented")
}

// handleLogin is a placeholder for the login endpoint.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "login not implemented")
}

// handleLogout is a placeholder for the logout endpoint.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "logout not implemented")
}

// handleMe is a placeholder for the current-user endpoint.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
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
