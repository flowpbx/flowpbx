package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/go-chi/chi/v5"
)

// extensionRequest is the JSON request body for creating/updating an extension.
type extensionRequest struct {
	Extension        string          `json:"extension"`
	Name             string          `json:"name"`
	Email            string          `json:"email"`
	SIPUsername      string          `json:"sip_username"`
	SIPPassword      string          `json:"sip_password"`
	RingTimeout      *int            `json:"ring_timeout"`
	DND              *bool           `json:"dnd"`
	FollowMeEnabled  *bool           `json:"follow_me_enabled"`
	FollowMeNumbers  json.RawMessage `json:"follow_me_numbers"`
	FollowMeStrategy string          `json:"follow_me_strategy"`
	FollowMeConfirm  *bool           `json:"follow_me_confirm"`
	RecordingMode    string          `json:"recording_mode"`
	MaxRegistrations *int            `json:"max_registrations"`
}

// extensionResponse is the JSON response for a single extension.
// SIP password is never returned.
type extensionResponse struct {
	ID               int64           `json:"id"`
	Extension        string          `json:"extension"`
	Name             string          `json:"name"`
	Email            string          `json:"email"`
	SIPUsername      string          `json:"sip_username"`
	RingTimeout      int             `json:"ring_timeout"`
	DND              bool            `json:"dnd"`
	FollowMeEnabled  bool            `json:"follow_me_enabled"`
	FollowMeNumbers  json.RawMessage `json:"follow_me_numbers"`
	FollowMeStrategy string          `json:"follow_me_strategy"`
	FollowMeConfirm  bool            `json:"follow_me_confirm"`
	RecordingMode    string          `json:"recording_mode"`
	MaxRegistrations int             `json:"max_registrations"`
	CreatedAt        string          `json:"created_at"`
	UpdatedAt        string          `json:"updated_at"`
}

// toExtensionResponse converts a models.Extension to the API response.
func toExtensionResponse(e *models.Extension) extensionResponse {
	strategy := e.FollowMeStrategy
	if strategy == "" {
		strategy = "sequential"
	}
	resp := extensionResponse{
		ID:               e.ID,
		Extension:        e.Extension,
		Name:             e.Name,
		Email:            e.Email,
		SIPUsername:      e.SIPUsername,
		RingTimeout:      e.RingTimeout,
		DND:              e.DND,
		FollowMeEnabled:  e.FollowMeEnabled,
		FollowMeStrategy: strategy,
		FollowMeConfirm:  e.FollowMeConfirm,
		RecordingMode:    e.RecordingMode,
		MaxRegistrations: e.MaxRegistrations,
		CreatedAt:        e.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        e.UpdatedAt.Format(time.RFC3339),
	}

	// Decode follow_me_numbers JSON, default to empty array.
	if e.FollowMeNumbers != "" {
		resp.FollowMeNumbers = json.RawMessage(e.FollowMeNumbers)
	} else {
		resp.FollowMeNumbers = json.RawMessage("[]")
	}

	return resp
}

// handleListExtensions returns extensions with pagination.
func (s *Server) handleListExtensions(w http.ResponseWriter, r *http.Request) {
	pg, errMsg := parsePagination(r)
	if errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	exts, err := s.extensions.List(r.Context())
	if err != nil {
		slog.Error("list extensions: failed to query", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	all := make([]extensionResponse, len(exts))
	for i := range exts {
		all[i] = toExtensionResponse(&exts[i])
	}

	total := len(all)
	start := pg.Offset
	if start > total {
		start = total
	}
	end := start + pg.Limit
	if end > total {
		end = total
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Items:  all[start:end],
		Total:  total,
		Limit:  pg.Limit,
		Offset: pg.Offset,
	})
}

// handleCreateExtension creates a new extension.
func (s *Server) handleCreateExtension(w http.ResponseWriter, r *http.Request) {
	var req extensionRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if errMsg := validateExtensionRequest(req, true); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	// Encrypt SIP password at rest if encryptor is available.
	sipPassword := req.SIPPassword
	if sipPassword != "" && s.encryptor != nil {
		encrypted, err := s.encryptor.Encrypt(sipPassword)
		if err != nil {
			slog.Error("create extension: failed to encrypt sip password", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		sipPassword = encrypted
	}

	ext := &models.Extension{
		Extension:        req.Extension,
		Name:             req.Name,
		Email:            req.Email,
		SIPUsername:      req.SIPUsername,
		SIPPassword:      sipPassword,
		RingTimeout:      30,
		DND:              false,
		FollowMeEnabled:  false,
		FollowMeNumbers:  "",
		FollowMeStrategy: "sequential",
		RecordingMode:    "off",
		MaxRegistrations: 5,
	}

	// Apply optional fields.
	if req.RingTimeout != nil {
		ext.RingTimeout = *req.RingTimeout
	}
	if req.DND != nil {
		ext.DND = *req.DND
	}
	if req.FollowMeEnabled != nil {
		ext.FollowMeEnabled = *req.FollowMeEnabled
	}
	if req.FollowMeNumbers != nil {
		ext.FollowMeNumbers = string(req.FollowMeNumbers)
	}
	if req.FollowMeStrategy != "" {
		ext.FollowMeStrategy = req.FollowMeStrategy
	}
	if req.FollowMeConfirm != nil {
		ext.FollowMeConfirm = *req.FollowMeConfirm
	}
	if req.RecordingMode != "" {
		ext.RecordingMode = req.RecordingMode
	}
	if req.MaxRegistrations != nil {
		ext.MaxRegistrations = *req.MaxRegistrations
	}

	if err := s.extensions.Create(r.Context(), ext); err != nil {
		slog.Error("create extension: failed to insert", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Re-fetch to get timestamps populated by the database.
	created, err := s.extensions.GetByID(r.Context(), ext.ID)
	if err != nil || created == nil {
		slog.Error("create extension: failed to re-fetch", "error", err, "extension_id", ext.ID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("extension created", "extension_id", created.ID, "extension", created.Extension)

	writeJSON(w, http.StatusCreated, toExtensionResponse(created))
}

// handleGetExtension returns a single extension by ID.
func (s *Server) handleGetExtension(w http.ResponseWriter, r *http.Request) {
	id, err := parseExtensionID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid extension id")
		return
	}

	ext, err := s.extensions.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("get extension: failed to query", "error", err, "extension_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if ext == nil {
		writeError(w, http.StatusNotFound, "extension not found")
		return
	}

	writeJSON(w, http.StatusOK, toExtensionResponse(ext))
}

// handleUpdateExtension updates an existing extension.
func (s *Server) handleUpdateExtension(w http.ResponseWriter, r *http.Request) {
	id, err := parseExtensionID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid extension id")
		return
	}

	existing, err := s.extensions.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("update extension: failed to query", "error", err, "extension_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "extension not found")
		return
	}

	var req extensionRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if errMsg := validateExtensionRequest(req, false); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	// Update fields from request.
	existing.Extension = req.Extension
	existing.Name = req.Name
	existing.Email = req.Email
	existing.SIPUsername = req.SIPUsername

	// Only update SIP password if a new one is provided.
	if req.SIPPassword != "" {
		sipPassword := req.SIPPassword
		if s.encryptor != nil {
			encrypted, err := s.encryptor.Encrypt(sipPassword)
			if err != nil {
				slog.Error("update extension: failed to encrypt sip password", "error", err)
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			sipPassword = encrypted
		}
		existing.SIPPassword = sipPassword
	}

	if req.RingTimeout != nil {
		existing.RingTimeout = *req.RingTimeout
	}
	if req.DND != nil {
		existing.DND = *req.DND
	}
	if req.FollowMeEnabled != nil {
		existing.FollowMeEnabled = *req.FollowMeEnabled
	}
	if req.FollowMeNumbers != nil {
		existing.FollowMeNumbers = string(req.FollowMeNumbers)
	}
	if req.FollowMeStrategy != "" {
		existing.FollowMeStrategy = req.FollowMeStrategy
	}
	if req.FollowMeConfirm != nil {
		existing.FollowMeConfirm = *req.FollowMeConfirm
	}
	if req.RecordingMode != "" {
		existing.RecordingMode = req.RecordingMode
	}
	if req.MaxRegistrations != nil {
		existing.MaxRegistrations = *req.MaxRegistrations
	}

	if err := s.extensions.Update(r.Context(), existing); err != nil {
		slog.Error("update extension: failed to update", "error", err, "extension_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Re-fetch to get updated timestamp.
	updated, err := s.extensions.GetByID(r.Context(), id)
	if err != nil || updated == nil {
		slog.Error("update extension: failed to re-fetch", "error", err, "extension_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("extension updated", "extension_id", id, "extension", updated.Extension)

	writeJSON(w, http.StatusOK, toExtensionResponse(updated))
}

// handleDeleteExtension removes an extension by ID.
func (s *Server) handleDeleteExtension(w http.ResponseWriter, r *http.Request) {
	id, err := parseExtensionID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid extension id")
		return
	}

	existing, err := s.extensions.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("delete extension: failed to query", "error", err, "extension_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "extension not found")
		return
	}

	if err := s.extensions.Delete(r.Context(), id); err != nil {
		slog.Error("delete extension: failed to delete", "error", err, "extension_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("extension deleted", "extension_id", id, "extension", existing.Extension)

	w.WriteHeader(http.StatusNoContent)
}

// registrationResponse is the JSON response for a single active registration.
type registrationResponse struct {
	ID           int64  `json:"id"`
	ExtensionID  int64  `json:"extension_id"`
	ContactURI   string `json:"contact_uri"`
	Transport    string `json:"transport"`
	UserAgent    string `json:"user_agent"`
	SourceIP     string `json:"source_ip"`
	SourcePort   int    `json:"source_port"`
	Expires      string `json:"expires"`
	RegisteredAt string `json:"registered_at"`
}

// handleListExtensionRegistrations returns active SIP registrations for an extension.
func (s *Server) handleListExtensionRegistrations(w http.ResponseWriter, r *http.Request) {
	id, err := parseExtensionID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid extension id")
		return
	}

	// Verify the extension exists.
	ext, err := s.extensions.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("list registrations: failed to query extension", "error", err, "extension_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if ext == nil {
		writeError(w, http.StatusNotFound, "extension not found")
		return
	}

	regs, err := s.registrations.GetByExtensionID(r.Context(), id)
	if err != nil {
		slog.Error("list registrations: failed to query", "error", err, "extension_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	items := make([]registrationResponse, len(regs))
	for i, reg := range regs {
		extID := int64(0)
		if reg.ExtensionID != nil {
			extID = *reg.ExtensionID
		}
		items[i] = registrationResponse{
			ID:           reg.ID,
			ExtensionID:  extID,
			ContactURI:   reg.ContactURI,
			Transport:    reg.Transport,
			UserAgent:    reg.UserAgent,
			SourceIP:     reg.SourceIP,
			SourcePort:   reg.SourcePort,
			Expires:      reg.Expires.Format(time.RFC3339),
			RegisteredAt: reg.RegisteredAt.Format(time.RFC3339),
		}
	}

	writeJSON(w, http.StatusOK, items)
}

// parseExtensionID extracts and parses the extension ID from the URL parameter.
func parseExtensionID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// validateExtensionRequest checks required fields for an extension create/update.
// isCreate controls whether sip_password is required (mandatory on create, optional on update).
func validateExtensionRequest(req extensionRequest, isCreate bool) string {
	if req.Extension == "" {
		return "extension is required"
	}
	if req.Name == "" {
		return "name is required"
	}
	if req.SIPUsername == "" {
		return "sip_username is required"
	}
	if isCreate && req.SIPPassword == "" {
		return "sip_password is required"
	}
	if req.RecordingMode != "" && req.RecordingMode != "off" && req.RecordingMode != "always" && req.RecordingMode != "on_demand" {
		return "recording_mode must be \"off\", \"always\", or \"on_demand\""
	}
	if req.RingTimeout != nil && *req.RingTimeout < 1 {
		return "ring_timeout must be a positive integer"
	}
	if req.MaxRegistrations != nil && *req.MaxRegistrations < 1 {
		return "max_registrations must be a positive integer"
	}
	if req.FollowMeNumbers != nil {
		// Validate that follow_me_numbers is a valid JSON array of follow-me entries
		// with required fields and sensible per-destination timeout/delay values.
		var entries []struct {
			Number  string `json:"number"`
			Delay   *int   `json:"delay"`
			Timeout *int   `json:"timeout"`
		}
		if err := json.Unmarshal(req.FollowMeNumbers, &entries); err != nil {
			return "follow_me_numbers must be a valid JSON array"
		}
		for i, entry := range entries {
			if entry.Number == "" {
				return fmt.Sprintf("follow_me_numbers[%d].number is required", i)
			}
			if entry.Delay != nil && *entry.Delay < 0 {
				return fmt.Sprintf("follow_me_numbers[%d].delay must be non-negative", i)
			}
			if entry.Timeout != nil && *entry.Timeout < 1 {
				return fmt.Sprintf("follow_me_numbers[%d].timeout must be a positive integer", i)
			}
		}
	}
	if req.FollowMeStrategy != "" && req.FollowMeStrategy != "sequential" && req.FollowMeStrategy != "simultaneous" {
		return "follow_me_strategy must be \"sequential\" or \"simultaneous\""
	}
	return ""
}
