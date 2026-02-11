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

// ringGroupRequest is the JSON request body for creating/updating a ring group.
type ringGroupRequest struct {
	Name         string          `json:"name"`
	Strategy     string          `json:"strategy"`
	RingTimeout  *int            `json:"ring_timeout"`
	Members      json.RawMessage `json:"members"`
	CallerIDMode string          `json:"caller_id_mode"`
}

// ringGroupResponse is the JSON response for a single ring group.
type ringGroupResponse struct {
	ID           int64           `json:"id"`
	Name         string          `json:"name"`
	Strategy     string          `json:"strategy"`
	RingTimeout  int             `json:"ring_timeout"`
	Members      json.RawMessage `json:"members"`
	CallerIDMode string          `json:"caller_id_mode"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
}

// toRingGroupResponse converts a models.RingGroup to the API response.
func toRingGroupResponse(rg *models.RingGroup) ringGroupResponse {
	resp := ringGroupResponse{
		ID:           rg.ID,
		Name:         rg.Name,
		Strategy:     rg.Strategy,
		RingTimeout:  rg.RingTimeout,
		CallerIDMode: rg.CallerIDMode,
		CreatedAt:    rg.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    rg.UpdatedAt.Format(time.RFC3339),
	}

	if rg.Members != "" {
		resp.Members = json.RawMessage(rg.Members)
	} else {
		resp.Members = json.RawMessage("[]")
	}

	return resp
}

// handleListRingGroups returns all ring groups.
func (s *Server) handleListRingGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := s.ringGroups.List(r.Context())
	if err != nil {
		slog.Error("list ring groups: failed to query", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	items := make([]ringGroupResponse, len(groups))
	for i := range groups {
		items[i] = toRingGroupResponse(&groups[i])
	}

	writeJSON(w, http.StatusOK, items)
}

// handleCreateRingGroup creates a new ring group.
func (s *Server) handleCreateRingGroup(w http.ResponseWriter, r *http.Request) {
	var req ringGroupRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if errMsg := validateRingGroupRequest(req, true); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	rg := &models.RingGroup{
		Name:         req.Name,
		Strategy:     "ring_all",
		RingTimeout:  30,
		Members:      string(req.Members),
		CallerIDMode: "pass",
	}

	if req.Strategy != "" {
		rg.Strategy = req.Strategy
	}
	if req.RingTimeout != nil {
		rg.RingTimeout = *req.RingTimeout
	}
	if req.CallerIDMode != "" {
		rg.CallerIDMode = req.CallerIDMode
	}

	if err := s.ringGroups.Create(r.Context(), rg); err != nil {
		slog.Error("create ring group: failed to insert", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	created, err := s.ringGroups.GetByID(r.Context(), rg.ID)
	if err != nil || created == nil {
		slog.Error("create ring group: failed to re-fetch", "error", err, "ring_group_id", rg.ID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("ring group created", "ring_group_id", created.ID, "name", created.Name)

	writeJSON(w, http.StatusCreated, toRingGroupResponse(created))
}

// handleGetRingGroup returns a single ring group by ID.
func (s *Server) handleGetRingGroup(w http.ResponseWriter, r *http.Request) {
	id, err := parseRingGroupID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ring group id")
		return
	}

	rg, err := s.ringGroups.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("get ring group: failed to query", "error", err, "ring_group_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if rg == nil {
		writeError(w, http.StatusNotFound, "ring group not found")
		return
	}

	writeJSON(w, http.StatusOK, toRingGroupResponse(rg))
}

// handleUpdateRingGroup updates an existing ring group.
func (s *Server) handleUpdateRingGroup(w http.ResponseWriter, r *http.Request) {
	id, err := parseRingGroupID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ring group id")
		return
	}

	existing, err := s.ringGroups.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("update ring group: failed to query", "error", err, "ring_group_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "ring group not found")
		return
	}

	var req ringGroupRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if errMsg := validateRingGroupRequest(req, false); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	existing.Name = req.Name
	if req.Strategy != "" {
		existing.Strategy = req.Strategy
	}
	if req.RingTimeout != nil {
		existing.RingTimeout = *req.RingTimeout
	}
	if req.Members != nil {
		existing.Members = string(req.Members)
	}
	if req.CallerIDMode != "" {
		existing.CallerIDMode = req.CallerIDMode
	}

	if err := s.ringGroups.Update(r.Context(), existing); err != nil {
		slog.Error("update ring group: failed to update", "error", err, "ring_group_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	updated, err := s.ringGroups.GetByID(r.Context(), id)
	if err != nil || updated == nil {
		slog.Error("update ring group: failed to re-fetch", "error", err, "ring_group_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("ring group updated", "ring_group_id", id, "name", updated.Name)

	writeJSON(w, http.StatusOK, toRingGroupResponse(updated))
}

// handleDeleteRingGroup removes a ring group by ID.
func (s *Server) handleDeleteRingGroup(w http.ResponseWriter, r *http.Request) {
	id, err := parseRingGroupID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ring group id")
		return
	}

	existing, err := s.ringGroups.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("delete ring group: failed to query", "error", err, "ring_group_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "ring group not found")
		return
	}

	if err := s.ringGroups.Delete(r.Context(), id); err != nil {
		slog.Error("delete ring group: failed to delete", "error", err, "ring_group_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("ring group deleted", "ring_group_id", id, "name", existing.Name)

	w.WriteHeader(http.StatusNoContent)
}

// parseRingGroupID extracts and parses the ring group ID from the URL parameter.
func parseRingGroupID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// validateRingGroupRequest checks required fields for a ring group create/update.
func validateRingGroupRequest(req ringGroupRequest, isCreate bool) string {
	if msg := validateRequiredStringLen("name", req.Name, maxNameLen); msg != "" {
		return msg
	}
	if msg := validateNoControlChars("name", req.Name); msg != "" {
		return msg
	}
	if req.Strategy != "" {
		switch req.Strategy {
		case "ring_all", "round_robin", "random", "longest_idle":
			// valid
		default:
			return "strategy must be \"ring_all\", \"round_robin\", \"random\", or \"longest_idle\""
		}
	}
	if req.CallerIDMode != "" {
		switch req.CallerIDMode {
		case "pass", "prepend":
			// valid
		default:
			return "caller_id_mode must be \"pass\" or \"prepend\""
		}
	}
	if msg := validateIntRange("ring_timeout", req.RingTimeout, 1, 600); msg != "" {
		return msg
	}
	if isCreate && req.Members == nil {
		return "members is required"
	}
	if req.Members != nil {
		var arr []json.RawMessage
		if err := json.Unmarshal(req.Members, &arr); err != nil {
			return "members must be a valid JSON array"
		}
		if len(arr) > 100 {
			return "members must contain at most 100 entries"
		}
	}
	return ""
}
