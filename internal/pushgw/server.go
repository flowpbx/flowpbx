package pushgw

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// LicenseStore abstracts database operations for license management.
// Implemented by the PostgreSQL store in a subsequent sprint task.
type LicenseStore interface {
	// ValidateLicense checks a license key and returns the license if valid.
	ValidateLicense(key string) (*License, error)

	// ActivateLicense registers a new installation for a license key.
	// Returns the created installation with its instance_id.
	ActivateLicense(key string, hostname string, version string) (*Installation, error)

	// GetLicenseStatus returns license details and installation count.
	GetLicenseStatus(key string) (*LicenseStatus, error)
}

// PushSender delivers push notifications via FCM or APNs.
// Implemented by FCM and APNs providers in subsequent sprint tasks.
type PushSender interface {
	// Send delivers a push notification to the specified token.
	// platform is "fcm" or "apns".
	Send(platform, token string, payload PushPayload) error
}

// PushLogger records push delivery attempts for audit and debugging.
type PushLogger interface {
	// Log records the result of a push delivery attempt.
	Log(entry PushLogEntry) error
}

// Server holds the push gateway HTTP handler dependencies.
type Server struct {
	router      *chi.Mux
	store       LicenseStore
	sender      PushSender
	pushLog     PushLogger
	rateLimiter *RateLimiter
}

// NewServer creates a push gateway HTTP server with all routes mounted.
// If rateLimiter is non-nil, rate limiting is applied to the push endpoint.
func NewServer(store LicenseStore, sender PushSender, pushLog PushLogger, rateLimiter *RateLimiter) *Server {
	s := &Server{
		router:      chi.NewRouter(),
		store:       store,
		sender:      sender,
		pushLog:     pushLog,
		rateLimiter: rateLimiter,
	}

	s.routes()
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// Router returns the underlying chi.Mux so the caller can add middleware.
func (s *Server) Router() *chi.Mux {
	return s.router
}

// routes mounts all push gateway API routes under /v1.
func (s *Server) routes() {
	r := s.router

	r.Route("/v1", func(r chi.Router) {
		// Apply rate limiting to the push endpoint if configured.
		if s.rateLimiter != nil {
			r.With(s.rateLimiter.Middleware).Post("/push", s.handlePush)
		} else {
			r.Post("/push", s.handlePush)
		}
		r.Post("/license/validate", s.handleLicenseValidate)
		r.Post("/license/activate", s.handleLicenseActivate)
		r.Get("/license/status", s.handleLicenseStatus)
	})
}

// handlePush handles POST /v1/push — validate license, send push notification.
func (s *Server) handlePush(w http.ResponseWriter, r *http.Request) {
	if s.store == nil || s.sender == nil {
		writeError(w, http.StatusServiceUnavailable, "push service not configured")
		return
	}

	var req PushRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if req.LicenseKey == "" {
		writeError(w, http.StatusBadRequest, "license_key is required")
		return
	}
	if req.PushToken == "" {
		writeError(w, http.StatusBadRequest, "push_token is required")
		return
	}
	if req.PushPlatform != "fcm" && req.PushPlatform != "apns" {
		writeError(w, http.StatusBadRequest, "push_platform must be fcm or apns")
		return
	}
	if req.CallID == "" {
		writeError(w, http.StatusBadRequest, "call_id is required")
		return
	}

	// Validate the license key.
	license, err := s.store.ValidateLicense(req.LicenseKey)
	if err != nil {
		slog.Error("push: license validation failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if license == nil {
		writeError(w, http.StatusForbidden, "invalid or expired license key")
		return
	}

	// Build the push payload and send.
	payload := PushPayload{
		CallID:   req.CallID,
		CallerID: req.CallerID,
		Type:     "incoming_call",
	}

	sendErr := s.sender.Send(req.PushPlatform, req.PushToken, payload)

	// Log the push attempt.
	if s.pushLog != nil {
		logEntry := PushLogEntry{
			LicenseKey: req.LicenseKey,
			Platform:   req.PushPlatform,
			CallID:     req.CallID,
			Success:    sendErr == nil,
			Timestamp:  time.Now(),
		}
		if sendErr != nil {
			logEntry.Error = sendErr.Error()
		}
		if logErr := s.pushLog.Log(logEntry); logErr != nil {
			slog.Error("push: failed to write push log", "error", logErr)
		}
	}

	if sendErr != nil {
		slog.Error("push: delivery failed", "error", sendErr, "platform", req.PushPlatform, "call_id", req.CallID)
		writeError(w, http.StatusBadGateway, "push delivery failed")
		return
	}

	slog.Info("push: notification sent", "platform", req.PushPlatform, "call_id", req.CallID, "license_key_prefix", truncateKey(req.LicenseKey))

	writeJSON(w, http.StatusOK, PushResponse{
		Delivered: true,
		CallID:    req.CallID,
	})
}

// handleLicenseValidate handles POST /v1/license/validate.
func (s *Server) handleLicenseValidate(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "license service not configured")
		return
	}

	var req LicenseValidateRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if req.LicenseKey == "" {
		writeError(w, http.StatusBadRequest, "license_key is required")
		return
	}

	license, err := s.store.ValidateLicense(req.LicenseKey)
	if err != nil {
		slog.Error("license validate: store error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if license == nil {
		writeError(w, http.StatusNotFound, "license key not found or expired")
		return
	}

	writeJSON(w, http.StatusOK, LicenseValidateResponse{
		Valid:         true,
		Tier:          license.Tier,
		MaxExtensions: license.MaxExtensions,
		ExpiresAt:     license.ExpiresAt,
	})
}

// handleLicenseActivate handles POST /v1/license/activate.
func (s *Server) handleLicenseActivate(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "license service not configured")
		return
	}

	var req LicenseActivateRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if req.LicenseKey == "" {
		writeError(w, http.StatusBadRequest, "license_key is required")
		return
	}
	if req.Hostname == "" {
		writeError(w, http.StatusBadRequest, "hostname is required")
		return
	}

	inst, err := s.store.ActivateLicense(req.LicenseKey, req.Hostname, req.Version)
	if err != nil {
		slog.Error("license activate: store error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if inst == nil {
		writeError(w, http.StatusForbidden, "license key not found or expired")
		return
	}

	slog.Info("license activated", "instance_id", inst.InstanceID, "license_key_prefix", truncateKey(req.LicenseKey))

	writeJSON(w, http.StatusOK, LicenseActivateResponse{
		InstanceID:  inst.InstanceID,
		ActivatedAt: inst.ActivatedAt,
	})
}

// handleLicenseStatus handles GET /v1/license/status.
func (s *Server) handleLicenseStatus(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "license service not configured")
		return
	}

	key := r.Header.Get("X-License-Key")
	if key == "" {
		key = r.URL.Query().Get("license_key")
	}
	if key == "" {
		writeError(w, http.StatusBadRequest, "license key is required (X-License-Key header or license_key query param)")
		return
	}

	status, err := s.store.GetLicenseStatus(key)
	if err != nil {
		slog.Error("license status: store error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if status == nil {
		writeError(w, http.StatusNotFound, "license key not found")
		return
	}

	writeJSON(w, http.StatusOK, status)
}

// truncateKey returns the first 8 characters of a license key for safe logging.
func truncateKey(key string) string {
	if len(key) <= 8 {
		return key
	}
	return key[:8] + "..."
}

// envelope is the standard response wrapper for the push gateway API.
type envelope struct {
	Data  any    `json:"data"`
	Error string `json:"error,omitempty"`
}

// writeJSON writes a JSON response with the given status code and data payload.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(envelope{Data: data}); err != nil {
		slog.Error("failed to encode json response", "error", err)
	}
}

// writeError writes a JSON error response with the given status code and message.
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(envelope{Error: msg}); err != nil {
		slog.Error("failed to encode json error response", "error", err)
	}
}

// maxRequestBodySize is the upper limit for JSON request bodies (1 MB).
const maxRequestBodySize = 1 << 20

// readJSON decodes a JSON request body into dst with size limiting.
// Returns a user-friendly error string on failure, or "" on success.
func readJSON(r *http.Request, dst any) string {
	r.Body = http.MaxBytesReader(nil, r.Body, maxRequestBodySize)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return "invalid request body"
	}

	if dec.More() {
		return "request body must contain a single json object"
	}

	return ""
}
