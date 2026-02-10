package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/go-chi/chi/v5"
)

// conferenceBridgeRequest is the JSON request body for creating/updating a conference bridge.
type conferenceBridgeRequest struct {
	Name          string `json:"name"`
	Extension     string `json:"extension"`
	PIN           string `json:"pin"`
	MaxMembers    *int   `json:"max_members"`
	Record        *bool  `json:"record"`
	MuteOnJoin    *bool  `json:"mute_on_join"`
	AnnounceJoins *bool  `json:"announce_joins"`
}

// conferenceBridgeResponse is the JSON response for a single conference bridge.
type conferenceBridgeResponse struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Extension     string `json:"extension"`
	PIN           string `json:"pin"`
	MaxMembers    int    `json:"max_members"`
	Record        bool   `json:"record"`
	MuteOnJoin    bool   `json:"mute_on_join"`
	AnnounceJoins bool   `json:"announce_joins"`
	CreatedAt     string `json:"created_at"`
}

// toConferenceBridgeResponse converts a models.ConferenceBridge to the API response.
func toConferenceBridgeResponse(b *models.ConferenceBridge) conferenceBridgeResponse {
	return conferenceBridgeResponse{
		ID:            b.ID,
		Name:          b.Name,
		Extension:     b.Extension,
		PIN:           b.PIN,
		MaxMembers:    b.MaxMembers,
		Record:        b.Record,
		MuteOnJoin:    b.MuteOnJoin,
		AnnounceJoins: b.AnnounceJoins,
		CreatedAt:     b.CreatedAt.Format(time.RFC3339),
	}
}

// handleListConferenceBridges returns all conference bridges.
func (s *Server) handleListConferenceBridges(w http.ResponseWriter, r *http.Request) {
	bridges, err := s.conferenceBridges.List(r.Context())
	if err != nil {
		slog.Error("list conference bridges: failed to query", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	items := make([]conferenceBridgeResponse, len(bridges))
	for i := range bridges {
		items[i] = toConferenceBridgeResponse(&bridges[i])
	}

	writeJSON(w, http.StatusOK, items)
}

// handleCreateConferenceBridge creates a new conference bridge.
func (s *Server) handleCreateConferenceBridge(w http.ResponseWriter, r *http.Request) {
	var req conferenceBridgeRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if errMsg := validateConferenceBridgeRequest(req, true); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	bridge := &models.ConferenceBridge{
		Name:          req.Name,
		Extension:     req.Extension,
		PIN:           req.PIN,
		MaxMembers:    10,
		Record:        false,
		MuteOnJoin:    false,
		AnnounceJoins: false,
	}

	if req.MaxMembers != nil {
		bridge.MaxMembers = *req.MaxMembers
	}
	if req.Record != nil {
		bridge.Record = *req.Record
	}
	if req.MuteOnJoin != nil {
		bridge.MuteOnJoin = *req.MuteOnJoin
	}
	if req.AnnounceJoins != nil {
		bridge.AnnounceJoins = *req.AnnounceJoins
	}

	if err := s.conferenceBridges.Create(r.Context(), bridge); err != nil {
		slog.Error("create conference bridge: failed to insert", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	created, err := s.conferenceBridges.GetByID(r.Context(), bridge.ID)
	if err != nil || created == nil {
		slog.Error("create conference bridge: failed to re-fetch", "error", err, "conference_bridge_id", bridge.ID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("conference bridge created", "conference_bridge_id", created.ID, "name", created.Name)

	writeJSON(w, http.StatusCreated, toConferenceBridgeResponse(created))
}

// handleGetConferenceBridge returns a single conference bridge by ID.
func (s *Server) handleGetConferenceBridge(w http.ResponseWriter, r *http.Request) {
	id, err := parseConferenceBridgeID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid conference bridge id")
		return
	}

	bridge, err := s.conferenceBridges.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("get conference bridge: failed to query", "error", err, "conference_bridge_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if bridge == nil {
		writeError(w, http.StatusNotFound, "conference bridge not found")
		return
	}

	writeJSON(w, http.StatusOK, toConferenceBridgeResponse(bridge))
}

// handleUpdateConferenceBridge updates an existing conference bridge.
func (s *Server) handleUpdateConferenceBridge(w http.ResponseWriter, r *http.Request) {
	id, err := parseConferenceBridgeID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid conference bridge id")
		return
	}

	existing, err := s.conferenceBridges.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("update conference bridge: failed to query", "error", err, "conference_bridge_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "conference bridge not found")
		return
	}

	var req conferenceBridgeRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if errMsg := validateConferenceBridgeRequest(req, false); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	existing.Name = req.Name
	if req.Extension != "" {
		existing.Extension = req.Extension
	}
	existing.PIN = req.PIN
	if req.MaxMembers != nil {
		existing.MaxMembers = *req.MaxMembers
	}
	if req.Record != nil {
		existing.Record = *req.Record
	}
	if req.MuteOnJoin != nil {
		existing.MuteOnJoin = *req.MuteOnJoin
	}
	if req.AnnounceJoins != nil {
		existing.AnnounceJoins = *req.AnnounceJoins
	}

	if err := s.conferenceBridges.Update(r.Context(), existing); err != nil {
		slog.Error("update conference bridge: failed to update", "error", err, "conference_bridge_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	updated, err := s.conferenceBridges.GetByID(r.Context(), id)
	if err != nil || updated == nil {
		slog.Error("update conference bridge: failed to re-fetch", "error", err, "conference_bridge_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("conference bridge updated", "conference_bridge_id", id, "name", updated.Name)

	writeJSON(w, http.StatusOK, toConferenceBridgeResponse(updated))
}

// handleDeleteConferenceBridge removes a conference bridge by ID.
func (s *Server) handleDeleteConferenceBridge(w http.ResponseWriter, r *http.Request) {
	id, err := parseConferenceBridgeID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid conference bridge id")
		return
	}

	existing, err := s.conferenceBridges.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("delete conference bridge: failed to query", "error", err, "conference_bridge_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "conference bridge not found")
		return
	}

	if err := s.conferenceBridges.Delete(r.Context(), id); err != nil {
		slog.Error("delete conference bridge: failed to delete", "error", err, "conference_bridge_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("conference bridge deleted", "conference_bridge_id", id, "name", existing.Name)

	w.WriteHeader(http.StatusNoContent)
}

// parseConferenceBridgeID extracts and parses the conference bridge ID from the URL parameter.
func parseConferenceBridgeID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// validateConferenceBridgeRequest checks required fields for a conference bridge create/update.
func validateConferenceBridgeRequest(req conferenceBridgeRequest, isCreate bool) string {
	if req.Name == "" {
		return "name is required"
	}
	if req.MaxMembers != nil && *req.MaxMembers < 2 {
		return "max_members must be at least 2"
	}
	return ""
}
