package api

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flowpbx/flowpbx/internal/api/middleware"
	"github.com/flowpbx/flowpbx/internal/config"
	"github.com/flowpbx/flowpbx/internal/database"
	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/flowpbx/flowpbx/internal/flow"
	"github.com/flowpbx/flowpbx/internal/web"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// TrunkStatusEntry represents the runtime status of a single trunk.
type TrunkStatusEntry struct {
	TrunkID        int64
	Name           string
	Type           string
	Status         string
	LastError      string
	RetryAttempt   int
	FailedAt       *time.Time
	RegisteredAt   *time.Time
	ExpiresAt      *time.Time
	LastOptionsAt  *time.Time
	OptionsHealthy bool
}

// TrunkStatusProvider exposes trunk registration status. Implemented by
// the SIP trunk registrar.
type TrunkStatusProvider interface {
	GetTrunkStatus(trunkID int64) (TrunkStatusEntry, bool)
	GetAllTrunkStatuses() []TrunkStatusEntry
}

// TrunkTester performs one-shot connectivity tests against trunks.
// For register-type trunks it attempts a SIP REGISTER; for IP-auth trunks
// it sends an OPTIONS ping.
type TrunkTester interface {
	TestRegister(ctx context.Context, trunk models.Trunk) error
	SendOptions(ctx context.Context, trunk models.Trunk) error
}

// TrunkLifecycleManager handles starting and stopping trunk registration
// or health checks in response to configuration changes (enable/disable,
// create, delete).
type TrunkLifecycleManager interface {
	StartTrunk(ctx context.Context, trunk models.Trunk) error
	StopTrunk(trunkID int64)
}

// ActiveCallEntry represents a single active call for the API response.
type ActiveCallEntry struct {
	CallID       string     `json:"call_id"`
	State        string     `json:"state"`
	Direction    string     `json:"direction"`
	CallerIDName string     `json:"caller_id_name"`
	CallerIDNum  string     `json:"caller_id_num"`
	CalledNum    string     `json:"called_num"`
	StartTime    time.Time  `json:"start_time"`
	AnswerTime   *time.Time `json:"answer_time,omitempty"`
	DurationSec  int        `json:"duration_sec"`
}

// ActiveCallsProvider exposes active call state. Implemented by
// an adapter that queries the SIP dialog and pending call managers.
type ActiveCallsProvider interface {
	GetActiveCalls() []ActiveCallEntry
	GetActiveCallCount() int
}

// Server holds HTTP handler dependencies and the chi router.
type Server struct {
	router         *chi.Mux
	db             *database.DB
	cfg            *config.Config
	sessions       *middleware.SessionStore
	adminUsers     database.AdminUserRepository
	systemConfig   database.SystemConfigRepository
	extensions     database.ExtensionRepository
	trunks         database.TrunkRepository
	inboundNumbers database.InboundNumberRepository
	registrations  database.RegistrationRepository
	cdrs           database.CDRRepository
	callFlows      database.CallFlowRepository
	flowValidator  *flow.Validator
	trunkStatus    TrunkStatusProvider
	trunkTester    TrunkTester
	trunkLifecycle TrunkLifecycleManager
	activeCalls    ActiveCallsProvider
	audioPrompts   database.AudioPromptRepository
	encryptor      *database.Encryptor
}

// NewServer creates the HTTP handler with all routes mounted.
func NewServer(db *database.DB, cfg *config.Config, sessions *middleware.SessionStore, sysConfig database.SystemConfigRepository, trunkStatus TrunkStatusProvider, trunkTester TrunkTester, trunkLifecycle TrunkLifecycleManager, activeCalls ActiveCallsProvider, enc *database.Encryptor) *Server {
	s := &Server{
		router:         chi.NewRouter(),
		db:             db,
		cfg:            cfg,
		sessions:       sessions,
		adminUsers:     database.NewAdminUserRepository(db),
		systemConfig:   sysConfig,
		extensions:     database.NewExtensionRepository(db),
		trunks:         database.NewTrunkRepository(db),
		inboundNumbers: database.NewInboundNumberRepository(db),
		registrations:  database.NewRegistrationRepository(db),
		cdrs:           database.NewCDRRepository(db),
		callFlows:      database.NewCallFlowRepository(db),
		audioPrompts:   database.NewAudioPromptRepository(db),
		flowValidator:  flow.NewValidator(nil),
		trunkStatus:    trunkStatus,
		trunkTester:    trunkTester,
		trunkLifecycle: trunkLifecycle,
		activeCalls:    activeCalls,
		encryptor:      enc,
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
			r.Get("/", s.handleListExtensions)
			r.Post("/", s.handleCreateExtension)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.handleGetExtension)
				r.Put("/", s.handleUpdateExtension)
				r.Delete("/", s.handleDeleteExtension)
				r.Get("/registrations", s.handleListExtensionRegistrations)
			})
		})

		r.Route("/trunks", func(r chi.Router) {
			r.Get("/", s.handleListTrunks)
			r.Post("/", s.handleCreateTrunk)
			r.Get("/status", s.handleListTrunkStatus)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.handleGetTrunk)
				r.Put("/", s.handleUpdateTrunk)
				r.Delete("/", s.handleDeleteTrunk)
				r.Post("/test", s.handleTestTrunk)
			})
		})

		r.Route("/numbers", func(r chi.Router) {
			r.Get("/", s.handleListInboundNumbers)
			r.Post("/", s.handleCreateInboundNumber)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.handleGetInboundNumber)
				r.Put("/", s.handleUpdateInboundNumber)
				r.Delete("/", s.handleDeleteInboundNumber)
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
			r.Get("/", s.handleListFlows)
			r.Post("/", s.handleCreateFlow)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.handleGetFlow)
				r.Put("/", s.handleUpdateFlow)
				r.Delete("/", s.handleDeleteFlow)
				r.Post("/publish", s.handlePublishFlow)
				r.Post("/validate", s.handleValidateFlow)
			})
		})

		r.Route("/cdrs", func(r chi.Router) {
			r.Get("/", s.handleListCDRs)
			r.Get("/export", s.handleExportCDRs)
			r.Get("/{id}", s.handleGetCDR)
		})

		r.Route("/recordings", func(r chi.Router) {
			r.Get("/", s.handleNotImplemented)
			r.Get("/{id}/download", s.handleNotImplemented)
			r.Delete("/{id}", s.handleNotImplemented)
		})

		r.Route("/prompts", func(r chi.Router) {
			r.Get("/", s.handleListPrompts)
			r.Post("/", s.handleUploadPrompt)
			r.Get("/{id}/audio", s.handleGetPromptAudio)
			r.Delete("/{id}", s.handleDeletePrompt)
		})

		r.Get("/settings", s.handleNotImplemented)
		r.Put("/settings", s.handleNotImplemented)

		r.Route("/system", func(r chi.Router) {
			r.Get("/status", s.handleNotImplemented)
			r.Post("/reload", s.handleNotImplemented)
		})

		r.Get("/dashboard/stats", s.handleDashboardStats)

		r.Route("/calls", func(r chi.Router) {
			r.Get("/active", s.handleListActiveCalls)
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

	// Serve embedded React SPA for non-API routes.
	s.mountSPA(r)

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
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
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
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
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

// handleListTrunkStatus returns the registration status of all trunks.
func (s *Server) handleListTrunkStatus(w http.ResponseWriter, r *http.Request) {
	if s.trunkStatus == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	statuses := s.trunkStatus.GetAllTrunkStatuses()

	items := make([]map[string]any, len(statuses))
	for i, st := range statuses {
		item := map[string]any{
			"trunk_id":        st.TrunkID,
			"name":            st.Name,
			"type":            st.Type,
			"status":          st.Status,
			"last_error":      st.LastError,
			"retry_attempt":   st.RetryAttempt,
			"options_healthy": st.OptionsHealthy,
		}
		if st.FailedAt != nil {
			item["failed_at"] = st.FailedAt.Format(time.RFC3339)
		}
		if st.RegisteredAt != nil {
			item["registered_at"] = st.RegisteredAt.Format(time.RFC3339)
		}
		if st.ExpiresAt != nil {
			item["expires_at"] = st.ExpiresAt.Format(time.RFC3339)
		}
		if st.LastOptionsAt != nil {
			item["last_options_at"] = st.LastOptionsAt.Format(time.RFC3339)
		}
		items[i] = item
	}

	writeJSON(w, http.StatusOK, items)
}

// handleListActiveCalls returns all currently active calls (both ringing
// and answered) from the in-memory call state.
func (s *Server) handleListActiveCalls(w http.ResponseWriter, r *http.Request) {
	if s.activeCalls == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	calls := s.activeCalls.GetActiveCalls()

	items := make([]map[string]any, len(calls))
	for i, c := range calls {
		item := map[string]any{
			"call_id":        c.CallID,
			"state":          c.State,
			"direction":      c.Direction,
			"caller_id_name": c.CallerIDName,
			"caller_id_num":  c.CallerIDNum,
			"called_num":     c.CalledNum,
			"start_time":     c.StartTime.Format(time.RFC3339),
			"duration_sec":   c.DurationSec,
		}
		if c.AnswerTime != nil {
			item["answer_time"] = c.AnswerTime.Format(time.RFC3339)
		}
		items[i] = item
	}

	writeJSON(w, http.StatusOK, items)
}

// handleNotImplemented returns 501 for endpoints not yet wired up.
func (s *Server) handleNotImplemented(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

// mountSPA configures static file serving from the embedded web.DistFS with
// SPA fallback: requests for files that exist in the bundle are served directly;
// all other non-API paths receive index.html so client-side routing works.
func (s *Server) mountSPA(r *chi.Mux) {
	// web.DistFS embeds files under "dist/", so sub the root to strip the prefix.
	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		slog.Error("failed to create sub filesystem for embedded SPA", "error", err)
		return
	}

	staticHandler := http.FileServer(http.FS(distFS))

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the exact file from the embedded bundle.
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Check whether the requested file exists in the embedded FS.
		if f, err := distFS.Open(path); err == nil {
			f.Close()
			staticHandler.ServeHTTP(w, r)
			return
		}

		// File not found — serve index.html for SPA client-side routing.
		r.URL.Path = "/"
		staticHandler.ServeHTTP(w, r)
	})
}
