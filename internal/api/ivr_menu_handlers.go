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

// ivrMenuRequest is the JSON request body for creating/updating an IVR menu.
type ivrMenuRequest struct {
	Name         string          `json:"name"`
	GreetingFile string          `json:"greeting_file"`
	GreetingTTS  string          `json:"greeting_tts"`
	Timeout      *int            `json:"timeout"`
	MaxRetries   *int            `json:"max_retries"`
	DigitTimeout *int            `json:"digit_timeout"`
	Options      json.RawMessage `json:"options"`
}

// ivrMenuResponse is the JSON response for a single IVR menu.
type ivrMenuResponse struct {
	ID           int64           `json:"id"`
	Name         string          `json:"name"`
	GreetingFile string          `json:"greeting_file"`
	GreetingTTS  string          `json:"greeting_tts"`
	Timeout      int             `json:"timeout"`
	MaxRetries   int             `json:"max_retries"`
	DigitTimeout int             `json:"digit_timeout"`
	Options      json.RawMessage `json:"options"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
}

// toIVRMenuResponse converts a models.IVRMenu to the API response.
func toIVRMenuResponse(m *models.IVRMenu) ivrMenuResponse {
	resp := ivrMenuResponse{
		ID:           m.ID,
		Name:         m.Name,
		GreetingFile: m.GreetingFile,
		GreetingTTS:  m.GreetingTTS,
		Timeout:      m.Timeout,
		MaxRetries:   m.MaxRetries,
		DigitTimeout: m.DigitTimeout,
		CreatedAt:    m.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    m.UpdatedAt.Format(time.RFC3339),
	}

	if m.Options != "" {
		resp.Options = json.RawMessage(m.Options)
	} else {
		resp.Options = json.RawMessage("{}")
	}

	return resp
}

// handleListIVRMenus returns all IVR menus.
func (s *Server) handleListIVRMenus(w http.ResponseWriter, r *http.Request) {
	menus, err := s.ivrMenus.List(r.Context())
	if err != nil {
		slog.Error("list ivr menus: failed to query", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	items := make([]ivrMenuResponse, len(menus))
	for i := range menus {
		items[i] = toIVRMenuResponse(&menus[i])
	}

	writeJSON(w, http.StatusOK, items)
}

// handleCreateIVRMenu creates a new IVR menu.
func (s *Server) handleCreateIVRMenu(w http.ResponseWriter, r *http.Request) {
	var req ivrMenuRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if errMsg := validateIVRMenuRequest(req, true); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	m := &models.IVRMenu{
		Name:         req.Name,
		GreetingFile: req.GreetingFile,
		GreetingTTS:  req.GreetingTTS,
		Timeout:      10,
		MaxRetries:   3,
		DigitTimeout: 3,
		Options:      string(req.Options),
	}

	if req.Timeout != nil {
		m.Timeout = *req.Timeout
	}
	if req.MaxRetries != nil {
		m.MaxRetries = *req.MaxRetries
	}
	if req.DigitTimeout != nil {
		m.DigitTimeout = *req.DigitTimeout
	}

	if err := s.ivrMenus.Create(r.Context(), m); err != nil {
		slog.Error("create ivr menu: failed to insert", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	created, err := s.ivrMenus.GetByID(r.Context(), m.ID)
	if err != nil || created == nil {
		slog.Error("create ivr menu: failed to re-fetch", "error", err, "ivr_menu_id", m.ID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("ivr menu created", "ivr_menu_id", created.ID, "name", created.Name)

	writeJSON(w, http.StatusCreated, toIVRMenuResponse(created))
}

// handleGetIVRMenu returns a single IVR menu by ID.
func (s *Server) handleGetIVRMenu(w http.ResponseWriter, r *http.Request) {
	id, err := parseIVRMenuID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ivr menu id")
		return
	}

	m, err := s.ivrMenus.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("get ivr menu: failed to query", "error", err, "ivr_menu_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if m == nil {
		writeError(w, http.StatusNotFound, "ivr menu not found")
		return
	}

	writeJSON(w, http.StatusOK, toIVRMenuResponse(m))
}

// handleUpdateIVRMenu updates an existing IVR menu.
func (s *Server) handleUpdateIVRMenu(w http.ResponseWriter, r *http.Request) {
	id, err := parseIVRMenuID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ivr menu id")
		return
	}

	existing, err := s.ivrMenus.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("update ivr menu: failed to query", "error", err, "ivr_menu_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "ivr menu not found")
		return
	}

	var req ivrMenuRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if errMsg := validateIVRMenuRequest(req, false); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	existing.Name = req.Name
	existing.GreetingFile = req.GreetingFile
	existing.GreetingTTS = req.GreetingTTS
	if req.Timeout != nil {
		existing.Timeout = *req.Timeout
	}
	if req.MaxRetries != nil {
		existing.MaxRetries = *req.MaxRetries
	}
	if req.DigitTimeout != nil {
		existing.DigitTimeout = *req.DigitTimeout
	}
	if req.Options != nil {
		existing.Options = string(req.Options)
	}

	if err := s.ivrMenus.Update(r.Context(), existing); err != nil {
		slog.Error("update ivr menu: failed to update", "error", err, "ivr_menu_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	updated, err := s.ivrMenus.GetByID(r.Context(), id)
	if err != nil || updated == nil {
		slog.Error("update ivr menu: failed to re-fetch", "error", err, "ivr_menu_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("ivr menu updated", "ivr_menu_id", id, "name", updated.Name)

	writeJSON(w, http.StatusOK, toIVRMenuResponse(updated))
}

// handleDeleteIVRMenu removes an IVR menu by ID.
func (s *Server) handleDeleteIVRMenu(w http.ResponseWriter, r *http.Request) {
	id, err := parseIVRMenuID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ivr menu id")
		return
	}

	existing, err := s.ivrMenus.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("delete ivr menu: failed to query", "error", err, "ivr_menu_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "ivr menu not found")
		return
	}

	if err := s.ivrMenus.Delete(r.Context(), id); err != nil {
		slog.Error("delete ivr menu: failed to delete", "error", err, "ivr_menu_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("ivr menu deleted", "ivr_menu_id", id, "name", existing.Name)

	w.WriteHeader(http.StatusNoContent)
}

// parseIVRMenuID extracts and parses the IVR menu ID from the URL parameter.
func parseIVRMenuID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// validateIVRMenuRequest checks required fields for an IVR menu create/update.
func validateIVRMenuRequest(req ivrMenuRequest, isCreate bool) string {
	if req.Name == "" {
		return "name is required"
	}
	if isCreate && req.Options == nil {
		return "options is required"
	}
	if req.Options != nil {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(req.Options, &obj); err != nil {
			return "options must be a valid JSON object"
		}
	}
	if req.Timeout != nil && *req.Timeout < 1 {
		return "timeout must be a positive integer"
	}
	if req.MaxRetries != nil && *req.MaxRetries < 0 {
		return "max_retries must be a non-negative integer"
	}
	if req.DigitTimeout != nil && *req.DigitTimeout < 1 {
		return "digit_timeout must be a positive integer"
	}
	return ""
}
