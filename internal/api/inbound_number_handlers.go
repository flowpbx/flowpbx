package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/go-chi/chi/v5"
)

// inboundNumberRequest is the JSON request body for creating/updating an inbound number.
type inboundNumberRequest struct {
	Number        string `json:"number"`
	Name          string `json:"name"`
	TrunkID       *int64 `json:"trunk_id"`
	FlowID        *int64 `json:"flow_id"`
	FlowEntryNode string `json:"flow_entry_node"`
	Enabled       *bool  `json:"enabled"`
}

// inboundNumberResponse is the JSON response for a single inbound number.
type inboundNumberResponse struct {
	ID            int64  `json:"id"`
	Number        string `json:"number"`
	Name          string `json:"name"`
	TrunkID       *int64 `json:"trunk_id"`
	FlowID        *int64 `json:"flow_id"`
	FlowEntryNode string `json:"flow_entry_node"`
	Enabled       bool   `json:"enabled"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// toInboundNumberResponse converts a models.InboundNumber to the API response.
func toInboundNumberResponse(n *models.InboundNumber) inboundNumberResponse {
	return inboundNumberResponse{
		ID:            n.ID,
		Number:        n.Number,
		Name:          n.Name,
		TrunkID:       n.TrunkID,
		FlowID:        n.FlowID,
		FlowEntryNode: n.FlowEntryNode,
		Enabled:       n.Enabled,
		CreatedAt:     n.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     n.UpdatedAt.Format(time.RFC3339),
	}
}

// handleListInboundNumbers returns all inbound numbers.
func (s *Server) handleListInboundNumbers(w http.ResponseWriter, r *http.Request) {
	numbers, err := s.inboundNumbers.List(r.Context())
	if err != nil {
		slog.Error("list inbound numbers: failed to query", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	items := make([]inboundNumberResponse, len(numbers))
	for i := range numbers {
		items[i] = toInboundNumberResponse(&numbers[i])
	}

	writeJSON(w, http.StatusOK, items)
}

// handleCreateInboundNumber creates a new inbound number.
func (s *Server) handleCreateInboundNumber(w http.ResponseWriter, r *http.Request) {
	var req inboundNumberRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if errMsg := validateInboundNumberRequest(req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	num := &models.InboundNumber{
		Number:        req.Number,
		Name:          req.Name,
		TrunkID:       req.TrunkID,
		FlowID:        req.FlowID,
		FlowEntryNode: req.FlowEntryNode,
		Enabled:       enabled,
	}

	if err := s.inboundNumbers.Create(r.Context(), num); err != nil {
		slog.Error("create inbound number: failed to insert", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Re-fetch to get timestamps populated by the database.
	created, err := s.inboundNumbers.GetByID(r.Context(), num.ID)
	if err != nil || created == nil {
		slog.Error("create inbound number: failed to re-fetch", "error", err, "number_id", num.ID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("inbound number created", "number_id", created.ID, "number", created.Number)

	writeJSON(w, http.StatusCreated, toInboundNumberResponse(created))
}

// handleGetInboundNumber returns a single inbound number by ID.
func (s *Server) handleGetInboundNumber(w http.ResponseWriter, r *http.Request) {
	id, err := parseInboundNumberID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid number id")
		return
	}

	num, err := s.inboundNumbers.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("get inbound number: failed to query", "error", err, "number_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if num == nil {
		writeError(w, http.StatusNotFound, "inbound number not found")
		return
	}

	writeJSON(w, http.StatusOK, toInboundNumberResponse(num))
}

// handleUpdateInboundNumber updates an existing inbound number.
func (s *Server) handleUpdateInboundNumber(w http.ResponseWriter, r *http.Request) {
	id, err := parseInboundNumberID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid number id")
		return
	}

	existing, err := s.inboundNumbers.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("update inbound number: failed to query", "error", err, "number_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "inbound number not found")
		return
	}

	var req inboundNumberRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if errMsg := validateInboundNumberRequest(req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	existing.Number = req.Number
	existing.Name = req.Name
	existing.TrunkID = req.TrunkID
	existing.FlowID = req.FlowID
	existing.FlowEntryNode = req.FlowEntryNode
	existing.Enabled = enabled

	if err := s.inboundNumbers.Update(r.Context(), existing); err != nil {
		slog.Error("update inbound number: failed to update", "error", err, "number_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Re-fetch to get updated timestamp.
	updated, err := s.inboundNumbers.GetByID(r.Context(), id)
	if err != nil || updated == nil {
		slog.Error("update inbound number: failed to re-fetch", "error", err, "number_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("inbound number updated", "number_id", id, "number", updated.Number)

	writeJSON(w, http.StatusOK, toInboundNumberResponse(updated))
}

// handleDeleteInboundNumber removes an inbound number by ID.
func (s *Server) handleDeleteInboundNumber(w http.ResponseWriter, r *http.Request) {
	id, err := parseInboundNumberID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid number id")
		return
	}

	existing, err := s.inboundNumbers.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("delete inbound number: failed to query", "error", err, "number_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "inbound number not found")
		return
	}

	if err := s.inboundNumbers.Delete(r.Context(), id); err != nil {
		slog.Error("delete inbound number: failed to delete", "error", err, "number_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("inbound number deleted", "number_id", id, "number", existing.Number)

	w.WriteHeader(http.StatusNoContent)
}

// parseInboundNumberID extracts and parses the inbound number ID from the URL parameter.
func parseInboundNumberID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// validateInboundNumberRequest checks required fields for an inbound number create/update.
func validateInboundNumberRequest(req inboundNumberRequest) string {
	if req.Number == "" {
		return "number is required"
	}
	return ""
}
