package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/go-chi/chi/v5"
)

// timeSwitchRequest is the JSON request body for creating/updating a time switch.
type timeSwitchRequest struct {
	Name        string          `json:"name"`
	Timezone    string          `json:"timezone"`
	Rules       json.RawMessage `json:"rules"`
	DefaultDest string          `json:"default_dest"`
}

// timeSwitchResponse is the JSON response for a single time switch.
type timeSwitchResponse struct {
	ID          int64           `json:"id"`
	Name        string          `json:"name"`
	Timezone    string          `json:"timezone"`
	Rules       json.RawMessage `json:"rules"`
	DefaultDest string          `json:"default_dest"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

// toTimeSwitchResponse converts a models.TimeSwitch to the API response.
func toTimeSwitchResponse(ts *models.TimeSwitch) timeSwitchResponse {
	resp := timeSwitchResponse{
		ID:          ts.ID,
		Name:        ts.Name,
		Timezone:    ts.Timezone,
		DefaultDest: ts.DefaultDest,
		CreatedAt:   ts.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   ts.UpdatedAt.Format(time.RFC3339),
	}

	if ts.Rules != "" {
		resp.Rules = json.RawMessage(ts.Rules)
	} else {
		resp.Rules = json.RawMessage("[]")
	}

	return resp
}

// handleListTimeSwitches returns all time switches.
func (s *Server) handleListTimeSwitches(w http.ResponseWriter, r *http.Request) {
	switches, err := s.timeSwitches.List(r.Context())
	if err != nil {
		slog.Error("list time switches: failed to query", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	items := make([]timeSwitchResponse, len(switches))
	for i := range switches {
		items[i] = toTimeSwitchResponse(&switches[i])
	}

	writeJSON(w, http.StatusOK, items)
}

// handleCreateTimeSwitch creates a new time switch.
func (s *Server) handleCreateTimeSwitch(w http.ResponseWriter, r *http.Request) {
	var req timeSwitchRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if errMsg := validateTimeSwitchRequest(req, true); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	ts := &models.TimeSwitch{
		Name:        req.Name,
		Timezone:    "Australia/Sydney",
		Rules:       string(req.Rules),
		DefaultDest: req.DefaultDest,
	}

	if req.Timezone != "" {
		ts.Timezone = req.Timezone
	}

	if err := s.timeSwitches.Create(r.Context(), ts); err != nil {
		slog.Error("create time switch: failed to insert", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	created, err := s.timeSwitches.GetByID(r.Context(), ts.ID)
	if err != nil || created == nil {
		slog.Error("create time switch: failed to re-fetch", "error", err, "time_switch_id", ts.ID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("time switch created", "time_switch_id", created.ID, "name", created.Name)

	writeJSON(w, http.StatusCreated, toTimeSwitchResponse(created))
}

// handleGetTimeSwitch returns a single time switch by ID.
func (s *Server) handleGetTimeSwitch(w http.ResponseWriter, r *http.Request) {
	id, err := parseTimeSwitchID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid time switch id")
		return
	}

	ts, err := s.timeSwitches.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("get time switch: failed to query", "error", err, "time_switch_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if ts == nil {
		writeError(w, http.StatusNotFound, "time switch not found")
		return
	}

	writeJSON(w, http.StatusOK, toTimeSwitchResponse(ts))
}

// handleUpdateTimeSwitch updates an existing time switch.
func (s *Server) handleUpdateTimeSwitch(w http.ResponseWriter, r *http.Request) {
	id, err := parseTimeSwitchID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid time switch id")
		return
	}

	existing, err := s.timeSwitches.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("update time switch: failed to query", "error", err, "time_switch_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "time switch not found")
		return
	}

	var req timeSwitchRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if errMsg := validateTimeSwitchRequest(req, false); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	existing.Name = req.Name
	if req.Timezone != "" {
		existing.Timezone = req.Timezone
	}
	if req.Rules != nil {
		existing.Rules = string(req.Rules)
	}
	existing.DefaultDest = req.DefaultDest

	if err := s.timeSwitches.Update(r.Context(), existing); err != nil {
		slog.Error("update time switch: failed to update", "error", err, "time_switch_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	updated, err := s.timeSwitches.GetByID(r.Context(), id)
	if err != nil || updated == nil {
		slog.Error("update time switch: failed to re-fetch", "error", err, "time_switch_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("time switch updated", "time_switch_id", id, "name", updated.Name)

	writeJSON(w, http.StatusOK, toTimeSwitchResponse(updated))
}

// handleDeleteTimeSwitch removes a time switch by ID.
func (s *Server) handleDeleteTimeSwitch(w http.ResponseWriter, r *http.Request) {
	id, err := parseTimeSwitchID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid time switch id")
		return
	}

	existing, err := s.timeSwitches.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("delete time switch: failed to query", "error", err, "time_switch_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "time switch not found")
		return
	}

	if err := s.timeSwitches.Delete(r.Context(), id); err != nil {
		slog.Error("delete time switch: failed to delete", "error", err, "time_switch_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("time switch deleted", "time_switch_id", id, "name", existing.Name)

	w.WriteHeader(http.StatusNoContent)
}

// parseTimeSwitchID extracts and parses the time switch ID from the URL parameter.
func parseTimeSwitchID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// validateTimeSwitchRequest checks required fields for a time switch create/update.
func validateTimeSwitchRequest(req timeSwitchRequest, isCreate bool) string {
	if req.Name == "" {
		return "name is required"
	}
	if isCreate && req.Rules == nil {
		return "rules is required"
	}
	if req.Rules != nil {
		var arr []json.RawMessage
		if err := json.Unmarshal(req.Rules, &arr); err != nil {
			return "rules must be a valid JSON array"
		}
	}
	return ""
}
