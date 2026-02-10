package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/flowpbx/flowpbx/internal/flow"
	"github.com/go-chi/chi/v5"
)

// flowRequest is the JSON request body for creating/updating a call flow.
type flowRequest struct {
	Name     string `json:"name"`
	FlowData string `json:"flow_data"`
}

// flowResponse is the JSON response for a single call flow.
type flowResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	FlowData    string `json:"flow_data"`
	Version     int    `json:"version"`
	Published   bool   `json:"published"`
	PublishedAt string `json:"published_at,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// toFlowResponse converts a models.CallFlow to the API response.
func toFlowResponse(f *models.CallFlow) flowResponse {
	resp := flowResponse{
		ID:        f.ID,
		Name:      f.Name,
		FlowData:  f.FlowData,
		Version:   f.Version,
		Published: f.Published,
		CreatedAt: f.CreatedAt.Format(time.RFC3339),
		UpdatedAt: f.UpdatedAt.Format(time.RFC3339),
	}
	if f.PublishedAt != nil {
		resp.PublishedAt = f.PublishedAt.Format(time.RFC3339)
	}
	return resp
}

// handleListFlows returns all call flows.
func (s *Server) handleListFlows(w http.ResponseWriter, r *http.Request) {
	flows, err := s.callFlows.List(r.Context())
	if err != nil {
		slog.Error("list flows: failed to query", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	items := make([]flowResponse, len(flows))
	for i := range flows {
		items[i] = toFlowResponse(&flows[i])
	}

	writeJSON(w, http.StatusOK, items)
}

// handleCreateFlow creates a new call flow.
func (s *Server) handleCreateFlow(w http.ResponseWriter, r *http.Request) {
	var req flowRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if errMsg := validateFlowRequest(req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	// Validate that flow_data is valid JSON if provided.
	if req.FlowData != "" {
		if _, err := flow.ParseFlowGraph(req.FlowData); err != nil {
			writeError(w, http.StatusBadRequest, "invalid flow_data: "+err.Error())
			return
		}
	}

	f := &models.CallFlow{
		Name:     req.Name,
		FlowData: req.FlowData,
		Version:  1,
	}

	if f.FlowData == "" {
		f.FlowData = `{"nodes":[],"edges":[]}`
	}

	if err := s.callFlows.Create(r.Context(), f); err != nil {
		slog.Error("create flow: failed to insert", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	created, err := s.callFlows.GetByID(r.Context(), f.ID)
	if err != nil || created == nil {
		slog.Error("create flow: failed to re-fetch", "error", err, "flow_id", f.ID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("call flow created", "flow_id", created.ID, "name", created.Name)

	writeJSON(w, http.StatusCreated, toFlowResponse(created))
}

// handleGetFlow returns a single call flow by ID.
func (s *Server) handleGetFlow(w http.ResponseWriter, r *http.Request) {
	id, err := parseFlowID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid flow id")
		return
	}

	f, err := s.callFlows.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("get flow: failed to query", "error", err, "flow_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if f == nil {
		writeError(w, http.StatusNotFound, "flow not found")
		return
	}

	writeJSON(w, http.StatusOK, toFlowResponse(f))
}

// handleUpdateFlow updates an existing call flow.
func (s *Server) handleUpdateFlow(w http.ResponseWriter, r *http.Request) {
	id, err := parseFlowID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid flow id")
		return
	}

	existing, err := s.callFlows.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("update flow: failed to query", "error", err, "flow_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "flow not found")
		return
	}

	var req flowRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if errMsg := validateFlowRequest(req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	// Validate that flow_data is valid JSON if provided.
	if req.FlowData != "" {
		if _, err := flow.ParseFlowGraph(req.FlowData); err != nil {
			writeError(w, http.StatusBadRequest, "invalid flow_data: "+err.Error())
			return
		}
	}

	existing.Name = req.Name
	if req.FlowData != "" {
		existing.FlowData = req.FlowData
	}

	if err := s.callFlows.Update(r.Context(), existing); err != nil {
		slog.Error("update flow: failed to update", "error", err, "flow_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	updated, err := s.callFlows.GetByID(r.Context(), id)
	if err != nil || updated == nil {
		slog.Error("update flow: failed to re-fetch", "error", err, "flow_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("call flow updated", "flow_id", id, "name", updated.Name)

	writeJSON(w, http.StatusOK, toFlowResponse(updated))
}

// handleDeleteFlow removes a call flow by ID.
func (s *Server) handleDeleteFlow(w http.ResponseWriter, r *http.Request) {
	id, err := parseFlowID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid flow id")
		return
	}

	existing, err := s.callFlows.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("delete flow: failed to query", "error", err, "flow_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "flow not found")
		return
	}

	if err := s.callFlows.Delete(r.Context(), id); err != nil {
		slog.Error("delete flow: failed to delete", "error", err, "flow_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("call flow deleted", "flow_id", id, "name", existing.Name)

	w.WriteHeader(http.StatusNoContent)
}

// handlePublishFlow publishes a call flow, making it available for live call routing.
func (s *Server) handlePublishFlow(w http.ResponseWriter, r *http.Request) {
	id, err := parseFlowID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid flow id")
		return
	}

	existing, err := s.callFlows.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("publish flow: failed to query", "error", err, "flow_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "flow not found")
		return
	}

	if err := s.callFlows.Publish(r.Context(), id); err != nil {
		slog.Error("publish flow: failed to publish", "error", err, "flow_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	published, err := s.callFlows.GetByID(r.Context(), id)
	if err != nil || published == nil {
		slog.Error("publish flow: failed to re-fetch", "error", err, "flow_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("call flow published", "flow_id", id, "name", published.Name, "version", published.Version)

	writeJSON(w, http.StatusOK, toFlowResponse(published))
}

// handleValidateFlow validates a call flow graph and returns any errors or warnings.
func (s *Server) handleValidateFlow(w http.ResponseWriter, r *http.Request) {
	id, err := parseFlowID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid flow id")
		return
	}

	existing, err := s.callFlows.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("validate flow: failed to query", "error", err, "flow_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "flow not found")
		return
	}

	graph, err := flow.ParseFlowGraph(existing.FlowData)
	if err != nil {
		writeJSON(w, http.StatusOK, &flow.ValidationResult{
			Valid: false,
			Issues: []flow.ValidationIssue{
				{
					Severity: flow.SeverityError,
					Message:  "invalid flow data: " + err.Error(),
				},
			},
		})
		return
	}

	// Determine the entry node from any inbound number referencing this flow.
	// For validation purposes, use empty string if no specific entry is set.
	entryNode := ""

	result := s.flowValidator.Validate(r.Context(), graph, entryNode)

	writeJSON(w, http.StatusOK, result)
}

// parseFlowID extracts and parses the call flow ID from the URL parameter.
func parseFlowID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// validateFlowRequest checks required fields for a call flow create/update.
func validateFlowRequest(req flowRequest) string {
	if req.Name == "" {
		return "name is required"
	}
	return ""
}
