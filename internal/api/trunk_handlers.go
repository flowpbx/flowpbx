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

// trunkRequest is the JSON request body for creating/updating a trunk.
type trunkRequest struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	Enabled        *bool    `json:"enabled"`
	Host           string   `json:"host"`
	Port           int      `json:"port"`
	Transport      string   `json:"transport"`
	Username       string   `json:"username"`
	Password       string   `json:"password"`
	AuthUsername   string   `json:"auth_username"`
	RegisterExpiry int      `json:"register_expiry"`
	RemoteHosts    []string `json:"remote_hosts"`
	LocalHost      string   `json:"local_host"`
	Codecs         []string `json:"codecs"`
	MaxChannels    int      `json:"max_channels"`
	CallerIDName   string   `json:"caller_id_name"`
	CallerIDNum    string   `json:"caller_id_num"`
	PrefixStrip    int      `json:"prefix_strip"`
	PrefixAdd      string   `json:"prefix_add"`
	Priority       int      `json:"priority"`
}

// trunkResponse is the JSON response for a single trunk. Password is never returned.
type trunkResponse struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	Enabled        bool     `json:"enabled"`
	Host           string   `json:"host"`
	Port           int      `json:"port"`
	Transport      string   `json:"transport"`
	Username       string   `json:"username"`
	AuthUsername   string   `json:"auth_username"`
	RegisterExpiry int      `json:"register_expiry"`
	RemoteHosts    []string `json:"remote_hosts"`
	LocalHost      string   `json:"local_host"`
	Codecs         []string `json:"codecs"`
	MaxChannels    int      `json:"max_channels"`
	CallerIDName   string   `json:"caller_id_name"`
	CallerIDNum    string   `json:"caller_id_num"`
	PrefixStrip    int      `json:"prefix_strip"`
	PrefixAdd      string   `json:"prefix_add"`
	Priority       int      `json:"priority"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}

// trunkDetailResponse extends trunkResponse with live registration status.
type trunkDetailResponse struct {
	trunkResponse
	Status         string  `json:"status"`
	LastError      string  `json:"last_error"`
	RetryAttempt   int     `json:"retry_attempt"`
	OptionsHealthy bool    `json:"options_healthy"`
	RegisteredAt   *string `json:"registered_at"`
	ExpiresAt      *string `json:"expires_at"`
	FailedAt       *string `json:"failed_at"`
	LastOptionsAt  *string `json:"last_options_at"`
}

// toTrunkResponse converts a models.Trunk to the API response, decoding JSON
// array fields and omitting the password.
func toTrunkResponse(t *models.Trunk) trunkResponse {
	resp := trunkResponse{
		ID:             t.ID,
		Name:           t.Name,
		Type:           t.Type,
		Enabled:        t.Enabled,
		Host:           t.Host,
		Port:           t.Port,
		Transport:      t.Transport,
		Username:       t.Username,
		AuthUsername:   t.AuthUsername,
		RegisterExpiry: t.RegisterExpiry,
		LocalHost:      t.LocalHost,
		MaxChannels:    t.MaxChannels,
		CallerIDName:   t.CallerIDName,
		CallerIDNum:    t.CallerIDNum,
		PrefixStrip:    t.PrefixStrip,
		PrefixAdd:      t.PrefixAdd,
		Priority:       t.Priority,
		CreatedAt:      t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      t.UpdatedAt.Format(time.RFC3339),
	}

	// Decode JSON array fields, default to empty array.
	resp.RemoteHosts = decodeJSONStringArray(t.RemoteHosts)
	resp.Codecs = decodeJSONStringArray(t.Codecs)

	return resp
}

// decodeJSONStringArray parses a JSON-encoded string array, returning an empty
// slice on failure or empty input.
func decodeJSONStringArray(s string) []string {
	if s == "" {
		return []string{}
	}
	var arr []string
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		return []string{}
	}
	return arr
}

// encodeJSONStringArray marshals a string slice to JSON for storage.
func encodeJSONStringArray(arr []string) string {
	if arr == nil {
		return "[]"
	}
	b, err := json.Marshal(arr)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// handleListTrunks returns all trunks with pagination.
func (s *Server) handleListTrunks(w http.ResponseWriter, r *http.Request) {
	trunks, err := s.trunks.List(r.Context())
	if err != nil {
		slog.Error("list trunks: failed to query", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	items := make([]trunkResponse, len(trunks))
	for i := range trunks {
		items[i] = toTrunkResponse(&trunks[i])
	}

	writeJSON(w, http.StatusOK, items)
}

// handleCreateTrunk creates a new trunk.
func (s *Server) handleCreateTrunk(w http.ResponseWriter, r *http.Request) {
	var req trunkRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if errMsg := validateTrunkRequest(req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	// Encrypt password if encryptor is available and password is provided.
	password := req.Password
	if password != "" && s.encryptor != nil {
		encrypted, err := s.encryptor.Encrypt(password)
		if err != nil {
			slog.Error("create trunk: failed to encrypt password", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		password = encrypted
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	trunk := &models.Trunk{
		Name:           req.Name,
		Type:           req.Type,
		Enabled:        enabled,
		Host:           req.Host,
		Port:           req.Port,
		Transport:      req.Transport,
		Username:       req.Username,
		Password:       password,
		AuthUsername:   req.AuthUsername,
		RegisterExpiry: req.RegisterExpiry,
		RemoteHosts:    encodeJSONStringArray(req.RemoteHosts),
		LocalHost:      req.LocalHost,
		Codecs:         encodeJSONStringArray(req.Codecs),
		MaxChannels:    req.MaxChannels,
		CallerIDName:   req.CallerIDName,
		CallerIDNum:    req.CallerIDNum,
		PrefixStrip:    req.PrefixStrip,
		PrefixAdd:      req.PrefixAdd,
		Priority:       req.Priority,
	}

	// Apply defaults.
	if trunk.Port == 0 {
		trunk.Port = 5060
	}
	if trunk.Transport == "" {
		trunk.Transport = "udp"
	}
	if trunk.RegisterExpiry == 0 {
		trunk.RegisterExpiry = 300
	}
	if trunk.Priority == 0 {
		trunk.Priority = 10
	}

	if err := s.trunks.Create(r.Context(), trunk); err != nil {
		slog.Error("create trunk: failed to insert", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Re-fetch to get timestamps populated by the database.
	created, err := s.trunks.GetByID(r.Context(), trunk.ID)
	if err != nil || created == nil {
		slog.Error("create trunk: failed to re-fetch", "error", err, "trunk_id", trunk.ID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("trunk created", "trunk_id", created.ID, "name", created.Name, "type", created.Type)

	writeJSON(w, http.StatusCreated, toTrunkResponse(created))
}

// handleGetTrunk returns a single trunk by ID including current registration status.
func (s *Server) handleGetTrunk(w http.ResponseWriter, r *http.Request) {
	id, err := parseTrunkID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid trunk id")
		return
	}

	trunk, err := s.trunks.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("get trunk: failed to query", "error", err, "trunk_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if trunk == nil {
		writeError(w, http.StatusNotFound, "trunk not found")
		return
	}

	detail := trunkDetailResponse{
		trunkResponse: toTrunkResponse(trunk),
		Status:        "unknown",
	}

	if s.trunkStatus != nil {
		if st, ok := s.trunkStatus.GetTrunkStatus(id); ok {
			detail.Status = st.Status
			detail.LastError = st.LastError
			detail.RetryAttempt = st.RetryAttempt
			detail.OptionsHealthy = st.OptionsHealthy
			if st.RegisteredAt != nil {
				v := st.RegisteredAt.Format(time.RFC3339)
				detail.RegisteredAt = &v
			}
			if st.ExpiresAt != nil {
				v := st.ExpiresAt.Format(time.RFC3339)
				detail.ExpiresAt = &v
			}
			if st.FailedAt != nil {
				v := st.FailedAt.Format(time.RFC3339)
				detail.FailedAt = &v
			}
			if st.LastOptionsAt != nil {
				v := st.LastOptionsAt.Format(time.RFC3339)
				detail.LastOptionsAt = &v
			}
		}
	}

	writeJSON(w, http.StatusOK, detail)
}

// handleUpdateTrunk updates an existing trunk.
func (s *Server) handleUpdateTrunk(w http.ResponseWriter, r *http.Request) {
	id, err := parseTrunkID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid trunk id")
		return
	}

	existing, err := s.trunks.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("update trunk: failed to query", "error", err, "trunk_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "trunk not found")
		return
	}

	var req trunkRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if errMsg := validateTrunkRequest(req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	// Handle password: if provided, encrypt it; if empty, keep existing.
	password := existing.Password
	if req.Password != "" {
		if s.encryptor != nil {
			encrypted, err := s.encryptor.Encrypt(req.Password)
			if err != nil {
				slog.Error("update trunk: failed to encrypt password", "error", err)
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			password = encrypted
		} else {
			password = req.Password
		}
	}

	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	port := req.Port
	if port == 0 {
		port = existing.Port
	}
	transport := req.Transport
	if transport == "" {
		transport = existing.Transport
	}
	registerExpiry := req.RegisterExpiry
	if registerExpiry == 0 {
		registerExpiry = existing.RegisterExpiry
	}
	priority := req.Priority
	if priority == 0 {
		priority = existing.Priority
	}

	existing.Name = req.Name
	existing.Type = req.Type
	existing.Enabled = enabled
	existing.Host = req.Host
	existing.Port = port
	existing.Transport = transport
	existing.Username = req.Username
	existing.Password = password
	existing.AuthUsername = req.AuthUsername
	existing.RegisterExpiry = registerExpiry
	existing.RemoteHosts = encodeJSONStringArray(req.RemoteHosts)
	existing.LocalHost = req.LocalHost
	existing.Codecs = encodeJSONStringArray(req.Codecs)
	existing.MaxChannels = req.MaxChannels
	existing.CallerIDName = req.CallerIDName
	existing.CallerIDNum = req.CallerIDNum
	existing.PrefixStrip = req.PrefixStrip
	existing.PrefixAdd = req.PrefixAdd
	existing.Priority = priority

	if err := s.trunks.Update(r.Context(), existing); err != nil {
		slog.Error("update trunk: failed to update", "error", err, "trunk_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Re-fetch to get updated timestamp.
	updated, err := s.trunks.GetByID(r.Context(), id)
	if err != nil || updated == nil {
		slog.Error("update trunk: failed to re-fetch", "error", err, "trunk_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("trunk updated", "trunk_id", id, "name", updated.Name)

	writeJSON(w, http.StatusOK, toTrunkResponse(updated))
}

// handleDeleteTrunk removes a trunk by ID.
func (s *Server) handleDeleteTrunk(w http.ResponseWriter, r *http.Request) {
	id, err := parseTrunkID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid trunk id")
		return
	}

	existing, err := s.trunks.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("delete trunk: failed to query", "error", err, "trunk_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "trunk not found")
		return
	}

	if err := s.trunks.Delete(r.Context(), id); err != nil {
		slog.Error("delete trunk: failed to delete", "error", err, "trunk_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("trunk deleted", "trunk_id", id, "name", existing.Name)

	w.WriteHeader(http.StatusNoContent)
}

// parseTrunkID extracts and parses the trunk ID from the URL parameter.
func parseTrunkID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// validateTrunkRequest checks required fields for a trunk create/update.
func validateTrunkRequest(req trunkRequest) string {
	if req.Name == "" {
		return "name is required"
	}
	if req.Type != "register" && req.Type != "ip" {
		return "type must be \"register\" or \"ip\""
	}
	if req.Type == "register" {
		if req.Host == "" {
			return "host is required for register trunks"
		}
		if req.Username == "" {
			return "username is required for register trunks"
		}
	}
	if req.Type == "ip" {
		if len(req.RemoteHosts) == 0 {
			return "remote_hosts is required for ip trunks"
		}
	}
	if req.Transport != "" && req.Transport != "udp" && req.Transport != "tcp" && req.Transport != "tls" {
		return "transport must be \"udp\", \"tcp\", or \"tls\""
	}
	if req.Port < 0 || req.Port > 65535 {
		return "port must be between 0 and 65535"
	}
	if req.Priority < 0 {
		return "priority must be non-negative"
	}
	return ""
}
