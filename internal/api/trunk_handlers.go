package api

import (
	"context"
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
	RecordingMode  string   `json:"recording_mode"`
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
	RecordingMode  string   `json:"recording_mode"`
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
		RecordingMode:  t.RecordingMode,
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

// handleListTrunks returns trunks with pagination.
func (s *Server) handleListTrunks(w http.ResponseWriter, r *http.Request) {
	pg, errMsg := parsePagination(r)
	if errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	trunks, err := s.trunks.List(r.Context())
	if err != nil {
		slog.Error("list trunks: failed to query", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	all := make([]trunkResponse, len(trunks))
	for i := range trunks {
		all[i] = toTrunkResponse(&trunks[i])
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

	recordingMode := req.RecordingMode
	if recordingMode == "" {
		recordingMode = "off"
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
		RecordingMode:  recordingMode,
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

	// Start registration / health check if trunk is enabled.
	if created.Enabled && s.trunkLifecycle != nil {
		if err := s.trunkLifecycle.StartTrunk(r.Context(), *created); err != nil {
			slog.Error("create trunk: failed to start trunk registration",
				"error", err, "trunk_id", created.ID, "name", created.Name)
		} else {
			slog.Info("trunk created and registration started",
				"trunk_id", created.ID, "name", created.Name)
		}
	}

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

	// Capture previous enabled state to detect lifecycle changes.
	prevEnabled := existing.Enabled

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

	recordingMode := req.RecordingMode
	if recordingMode == "" {
		recordingMode = existing.RecordingMode
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
	existing.RecordingMode = recordingMode

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

	// Handle enable/disable lifecycle changes.
	if s.trunkLifecycle != nil {
		wasEnabled := prevEnabled
		nowEnabled := updated.Enabled

		if wasEnabled && !nowEnabled {
			// Trunk was disabled — stop registration / health check.
			s.trunkLifecycle.StopTrunk(id)
			slog.Info("trunk disabled, registration stopped", "trunk_id", id, "name", updated.Name)
		} else if !wasEnabled && nowEnabled {
			// Trunk was enabled — start registration / health check.
			if err := s.trunkLifecycle.StartTrunk(r.Context(), *updated); err != nil {
				slog.Error("update trunk: failed to start trunk after enable",
					"error", err, "trunk_id", id, "name", updated.Name)
			} else {
				slog.Info("trunk enabled, registration started", "trunk_id", id, "name", updated.Name)
			}
		} else if nowEnabled {
			// Trunk is still enabled but config may have changed — restart.
			s.trunkLifecycle.StopTrunk(id)
			if err := s.trunkLifecycle.StartTrunk(r.Context(), *updated); err != nil {
				slog.Error("update trunk: failed to restart trunk after config change",
					"error", err, "trunk_id", id, "name", updated.Name)
			} else {
				slog.Info("trunk config changed, registration restarted", "trunk_id", id, "name", updated.Name)
			}
		}
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

	// Stop registration / health check if running.
	if s.trunkLifecycle != nil {
		s.trunkLifecycle.StopTrunk(id)
	}

	slog.Info("trunk deleted", "trunk_id", id, "name", existing.Name)

	w.WriteHeader(http.StatusNoContent)
}

// parseTrunkID extracts and parses the trunk ID from the URL parameter.
func parseTrunkID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// trunkTestResponse is the JSON response for a trunk connectivity test.
type trunkTestResponse struct {
	Success  bool   `json:"success"`
	Method   string `json:"method"`
	Message  string `json:"message"`
	Duration string `json:"duration"`
}

// handleTestTrunk performs a one-shot connectivity test on a trunk.
// For register-type trunks, it attempts a SIP REGISTER to verify credentials
// and registrar reachability. For IP-auth trunks, it sends an OPTIONS ping.
func (s *Server) handleTestTrunk(w http.ResponseWriter, r *http.Request) {
	id, err := parseTrunkID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid trunk id")
		return
	}

	trunk, err := s.trunks.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("test trunk: failed to query", "error", err, "trunk_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if trunk == nil {
		writeError(w, http.StatusNotFound, "trunk not found")
		return
	}

	if s.trunkTester == nil {
		writeError(w, http.StatusServiceUnavailable, "sip stack not available")
		return
	}

	// For register trunks, decrypt the password before testing.
	if trunk.Type == "register" && trunk.Password != "" && s.encryptor != nil {
		decrypted, err := s.encryptor.Decrypt(trunk.Password)
		if err != nil {
			slog.Error("test trunk: failed to decrypt password", "error", err, "trunk_id", id)
			writeError(w, http.StatusInternalServerError, "failed to decrypt trunk credentials")
			return
		}
		trunk.Password = decrypted
	}

	// Use a bounded timeout for the test to prevent long hangs.
	testCtx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	start := time.Now()

	var testErr error
	method := "OPTIONS"
	if trunk.Type == "register" {
		method = "REGISTER"
		testErr = s.trunkTester.TestRegister(testCtx, *trunk)
	} else {
		testErr = s.trunkTester.SendOptions(testCtx, *trunk)
	}
	duration := time.Since(start)

	resp := trunkTestResponse{
		Method:   method,
		Duration: duration.Round(time.Millisecond).String(),
	}

	if testErr != nil {
		resp.Success = false
		resp.Message = testErr.Error()
		slog.Info("trunk test failed",
			"trunk_id", id,
			"name", trunk.Name,
			"method", method,
			"error", testErr,
			"duration", duration.String(),
		)
	} else {
		resp.Success = true
		resp.Message = method + " successful"
		slog.Info("trunk test succeeded",
			"trunk_id", id,
			"name", trunk.Name,
			"method", method,
			"duration", duration.String(),
		)
	}

	writeJSON(w, http.StatusOK, resp)
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
	if req.RecordingMode != "" && req.RecordingMode != "off" && req.RecordingMode != "always" && req.RecordingMode != "on_demand" {
		return "recording_mode must be \"off\", \"always\", or \"on_demand\""
	}
	return ""
}
