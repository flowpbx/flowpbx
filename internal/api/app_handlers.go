package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/flowpbx/flowpbx/internal/database/models"
)

// appExtensionKey is the context key for the authenticated app extension ID.
// Set by the app JWT middleware (Sprint 19).
type appContextKey string

const appExtensionIDKey appContextKey = "app_extension_id"

// AppExtensionIDFromContext retrieves the authenticated extension ID from the
// request context. Returns 0 if not set (no app auth middleware present).
func AppExtensionIDFromContext(ctx context.Context) int64 {
	id, _ := ctx.Value(appExtensionIDKey).(int64)
	return id
}

// WithAppExtensionID returns a context with the app extension ID set.
// Used by the app JWT auth middleware.
func WithAppExtensionID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, appExtensionIDKey, id)
}

// appMeUpdateRequest is the JSON request body for PUT /api/v1/app/me.
// Only fields that the mobile app extension user is allowed to toggle.
type appMeUpdateRequest struct {
	FollowMeEnabled *bool `json:"follow_me_enabled"`
	DND             *bool `json:"dnd"`
}

// appMeResponse is the JSON response for GET/PUT /api/v1/app/me.
type appMeResponse struct {
	ID               int64           `json:"id"`
	Extension        string          `json:"extension"`
	Name             string          `json:"name"`
	DND              bool            `json:"dnd"`
	FollowMeEnabled  bool            `json:"follow_me_enabled"`
	FollowMeNumbers  json.RawMessage `json:"follow_me_numbers"`
	FollowMeStrategy string          `json:"follow_me_strategy"`
	FollowMeConfirm  bool            `json:"follow_me_confirm"`
	UpdatedAt        string          `json:"updated_at"`
}

// handleAppUpdateMe handles PUT /api/v1/app/me — allows the authenticated
// extension user to toggle follow_me_enabled and DND.
func (s *Server) handleAppUpdateMe(w http.ResponseWriter, r *http.Request) {
	extID := AppExtensionIDFromContext(r.Context())
	if extID == 0 {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req appMeUpdateRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	// At least one field must be provided.
	if req.FollowMeEnabled == nil && req.DND == nil {
		writeError(w, http.StatusBadRequest, "at least one field must be provided")
		return
	}

	ext, err := s.extensions.GetByID(r.Context(), extID)
	if err != nil {
		slog.Error("app update me: failed to query extension", "error", err, "extension_id", extID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if ext == nil {
		writeError(w, http.StatusNotFound, "extension not found")
		return
	}

	if req.FollowMeEnabled != nil {
		ext.FollowMeEnabled = *req.FollowMeEnabled
	}
	if req.DND != nil {
		ext.DND = *req.DND
	}

	if err := s.extensions.Update(r.Context(), ext); err != nil {
		slog.Error("app update me: failed to update extension", "error", err, "extension_id", extID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Re-fetch to get updated timestamp.
	updated, err := s.extensions.GetByID(r.Context(), extID)
	if err != nil || updated == nil {
		slog.Error("app update me: failed to re-fetch extension", "error", err, "extension_id", extID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("app extension updated", "extension_id", extID, "follow_me_enabled", updated.FollowMeEnabled, "dnd", updated.DND)

	writeJSON(w, http.StatusOK, toAppMeResponse(updated))
}

// toAppMeResponse converts a models.Extension to the app me API response.
func toAppMeResponse(e *models.Extension) appMeResponse {
	strategy := e.FollowMeStrategy
	if strategy == "" {
		strategy = "sequential"
	}

	var numbers json.RawMessage
	if e.FollowMeNumbers != "" {
		numbers = json.RawMessage(e.FollowMeNumbers)
	} else {
		numbers = json.RawMessage("[]")
	}

	return appMeResponse{
		ID:               e.ID,
		Extension:        e.Extension,
		Name:             e.Name,
		DND:              e.DND,
		FollowMeEnabled:  e.FollowMeEnabled,
		FollowMeNumbers:  numbers,
		FollowMeStrategy: strategy,
		FollowMeConfirm:  e.FollowMeConfirm,
		UpdatedAt:        e.UpdatedAt.Format(time.RFC3339),
	}
}
