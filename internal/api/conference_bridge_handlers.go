package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/flowpbx/flowpbx/internal/database"
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
	HasPIN        bool   `json:"has_pin"`
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
		HasPIN:        b.PIN != "",
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

	// Hash the conference PIN if provided.
	var pinHash string
	if req.PIN != "" {
		var err error
		pinHash, err = database.HashPassword(req.PIN)
		if err != nil {
			slog.Error("create conference bridge: failed to hash pin", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	bridge := &models.ConferenceBridge{
		Name:          req.Name,
		Extension:     req.Extension,
		PIN:           pinHash,
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

	// Hash the conference PIN if provided, or clear it if empty.
	if req.PIN != "" {
		pinHash, err := database.HashPassword(req.PIN)
		if err != nil {
			slog.Error("update conference bridge: failed to hash pin", "error", err, "conference_bridge_id", id)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		existing.PIN = pinHash
	} else {
		existing.PIN = ""
	}

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

// conferenceParticipantResponse is the JSON response for an active conference participant.
type conferenceParticipantResponse struct {
	ID           string `json:"id"`
	CallerIDName string `json:"caller_id_name"`
	CallerIDNum  string `json:"caller_id_num"`
	JoinedAt     string `json:"joined_at"`
	Muted        bool   `json:"muted"`
}

// handleListConferenceParticipants returns the active participants for a conference room.
func (s *Server) handleListConferenceParticipants(w http.ResponseWriter, r *http.Request) {
	bridgeID, err := parseConferenceBridgeID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid conference bridge id")
		return
	}

	// Verify the conference bridge exists in the database.
	bridge, err := s.conferenceBridges.GetByID(r.Context(), bridgeID)
	if err != nil {
		slog.Error("list conference participants: failed to query bridge", "error", err, "conference_bridge_id", bridgeID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if bridge == nil {
		writeError(w, http.StatusNotFound, "conference bridge not found")
		return
	}

	if s.conferenceProv == nil {
		writeError(w, http.StatusServiceUnavailable, "conference manager not available")
		return
	}

	participants, err := s.conferenceProv.Participants(bridgeID)
	if err != nil {
		slog.Error("list conference participants: failed", "error", err, "conference_bridge_id", bridgeID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Return empty array rather than null when no participants.
	items := make([]conferenceParticipantResponse, 0, len(participants))
	for _, p := range participants {
		items = append(items, conferenceParticipantResponse{
			ID:           p.ID,
			CallerIDName: p.CallerIDName,
			CallerIDNum:  p.CallerIDNum,
			JoinedAt:     p.JoinedAt.Format(time.RFC3339),
			Muted:        p.Muted,
		})
	}

	writeJSON(w, http.StatusOK, items)
}

// handleMuteConferenceParticipant sets or clears the mute state for a
// participant in an active conference room.
func (s *Server) handleMuteConferenceParticipant(w http.ResponseWriter, r *http.Request) {
	bridgeID, err := parseConferenceBridgeID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid conference bridge id")
		return
	}

	participantID := chi.URLParam(r, "participantID")
	if participantID == "" {
		writeError(w, http.StatusBadRequest, "participant id is required")
		return
	}

	var req struct {
		Muted bool `json:"muted"`
	}
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if s.conferenceProv == nil {
		writeError(w, http.StatusServiceUnavailable, "conference manager not available")
		return
	}

	if err := s.conferenceProv.MuteParticipant(bridgeID, participantID, req.Muted); err != nil {
		slog.Error("mute conference participant: failed",
			"error", err,
			"conference_bridge_id", bridgeID,
			"participant_id", participantID,
			"muted", req.Muted,
		)
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	slog.Info("conference participant mute state changed",
		"conference_bridge_id", bridgeID,
		"participant_id", participantID,
		"muted", req.Muted,
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"participant_id": participantID,
		"muted":          req.Muted,
	})
}

// handleKickConferenceParticipant removes a participant from an active conference room.
func (s *Server) handleKickConferenceParticipant(w http.ResponseWriter, r *http.Request) {
	bridgeID, err := parseConferenceBridgeID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid conference bridge id")
		return
	}

	participantID := chi.URLParam(r, "participantID")
	if participantID == "" {
		writeError(w, http.StatusBadRequest, "participant id is required")
		return
	}

	if s.conferenceProv == nil {
		writeError(w, http.StatusServiceUnavailable, "conference manager not available")
		return
	}

	if err := s.conferenceProv.KickParticipant(bridgeID, participantID); err != nil {
		slog.Error("kick conference participant: failed",
			"error", err,
			"conference_bridge_id", bridgeID,
			"participant_id", participantID,
		)
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	slog.Info("conference participant kicked",
		"conference_bridge_id", bridgeID,
		"participant_id", participantID,
	)

	w.WriteHeader(http.StatusNoContent)
}

// validateConferenceBridgeRequest checks required fields for a conference bridge create/update.
func validateConferenceBridgeRequest(req conferenceBridgeRequest, isCreate bool) string {
	if msg := validateRequiredStringLen("name", req.Name, maxNameLen); msg != "" {
		return msg
	}
	if msg := validateNoControlChars("name", req.Name); msg != "" {
		return msg
	}
	if isCreate && req.Extension == "" {
		return "extension is required"
	}
	if req.Extension != "" {
		if msg := validateExtensionNumber("extension", req.Extension); msg != "" {
			return msg
		}
	}
	if msg := validatePIN("pin", req.PIN); msg != "" {
		return msg
	}
	if msg := validateIntRange("max_members", req.MaxMembers, 2, 200); msg != "" {
		return msg
	}
	return ""
}
