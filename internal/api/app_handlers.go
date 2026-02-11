package api

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/flowpbx/flowpbx/internal/api/middleware"
	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/go-chi/chi/v5"
)

// appAuthRequest is the JSON request body for POST /api/v1/app/auth.
type appAuthRequest struct {
	Extension   string `json:"extension"`
	SIPPassword string `json:"sip_password"`
}

// appAuthResponse is the JSON response for POST /api/v1/app/auth.
type appAuthResponse struct {
	Token     string        `json:"token"`
	ExpiresAt string        `json:"expires_at"`
	Extension appMeResponse `json:"extension"`
	SIP       appSIPConfig  `json:"sip"`
}

// appSIPConfig holds the SIP connection configuration returned to the mobile app.
type appSIPConfig struct {
	Domain    string `json:"domain"`
	Port      int    `json:"port"`
	TLSPort   int    `json:"tls_port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Transport string `json:"transport"`
}

// appMeUpdateRequest is the JSON request body for PUT /api/v1/app/me.
type appMeUpdateRequest struct {
	FollowMeEnabled *bool `json:"follow_me_enabled"`
	DND             *bool `json:"dnd"`
}

// appMeResponse is the JSON response for GET/PUT /api/v1/app/me.
type appMeResponse struct {
	ID               int64           `json:"id"`
	Extension        string          `json:"extension"`
	Name             string          `json:"name"`
	Email            string          `json:"email"`
	DND              bool            `json:"dnd"`
	FollowMeEnabled  bool            `json:"follow_me_enabled"`
	FollowMeNumbers  json.RawMessage `json:"follow_me_numbers"`
	FollowMeStrategy string          `json:"follow_me_strategy"`
	FollowMeConfirm  bool            `json:"follow_me_confirm"`
	UpdatedAt        string          `json:"updated_at"`
}

// appPushTokenRequest is the JSON request body for POST /api/v1/app/push-token.
type appPushTokenRequest struct {
	Token      string `json:"token"`
	Platform   string `json:"platform"`
	DeviceID   string `json:"device_id"`
	AppVersion string `json:"app_version"`
}

// handleAppAuth handles POST /api/v1/app/auth — extension login via SIP
// credentials. Returns a JWT token and SIP configuration.
func (s *Server) handleAppAuth(w http.ResponseWriter, r *http.Request) {
	if len(s.jwtSecret) == 0 {
		writeError(w, http.StatusServiceUnavailable, "app authentication not configured")
		return
	}

	var req appAuthRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if req.Extension == "" || req.SIPPassword == "" {
		writeError(w, http.StatusBadRequest, "extension and sip_password are required")
		return
	}
	if msg := validateStringLen("extension", req.Extension, maxShortStringLen); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if msg := validateStringLen("sip_password", req.SIPPassword, maxPasswordLen); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	ext, err := s.extensions.GetByExtension(r.Context(), req.Extension)
	if err != nil {
		slog.Error("app auth: failed to query extension", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if ext == nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Decrypt stored SIP password and compare.
	storedPassword := ext.SIPPassword
	if s.encryptor != nil && storedPassword != "" {
		decrypted, err := s.encryptor.Decrypt(storedPassword)
		if err != nil {
			slog.Error("app auth: failed to decrypt sip password", "error", err, "extension_id", ext.ID)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		storedPassword = decrypted
	}

	if subtle.ConstantTimeCompare([]byte(storedPassword), []byte(req.SIPPassword)) != 1 {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Generate JWT token.
	token, expiresAt, err := middleware.GenerateAppToken(s.jwtSecret, ext.ID, ext.Extension)
	if err != nil {
		slog.Error("app auth: failed to generate token", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Build SIP config for the app. Use the SIP password (plaintext) so the
	// app can register with the PBX.
	sipPassword := req.SIPPassword
	domain := s.cfg.SIPHost()
	if hostname, err := s.systemConfig.Get(r.Context(), "hostname"); err == nil && hostname != "" {
		domain = hostname
	}

	slog.Info("app extension authenticated", "extension_id", ext.ID, "extension", ext.Extension)

	writeJSON(w, http.StatusOK, appAuthResponse{
		Token:     token,
		ExpiresAt: expiresAt.Format(time.RFC3339),
		Extension: toAppMeResponse(ext),
		SIP: appSIPConfig{
			Domain:    domain,
			Port:      s.cfg.SIPPort,
			TLSPort:   s.cfg.SIPTLSPort,
			Username:  ext.SIPUsername,
			Password:  sipPassword,
			Transport: "tls",
		},
	})
}

// handleAppGetMe handles GET /api/v1/app/me — returns the authenticated
// extension's profile.
func (s *Server) handleAppGetMe(w http.ResponseWriter, r *http.Request) {
	extID := middleware.AppExtensionIDFromContext(r.Context())
	if extID == 0 {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	ext, err := s.extensions.GetByID(r.Context(), extID)
	if err != nil {
		slog.Error("app get me: failed to query extension", "error", err, "extension_id", extID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if ext == nil {
		writeError(w, http.StatusNotFound, "extension not found")
		return
	}

	writeJSON(w, http.StatusOK, toAppMeResponse(ext))
}

// handleAppUpdateMe handles PUT /api/v1/app/me — allows the authenticated
// extension user to toggle follow_me_enabled and DND.
func (s *Server) handleAppUpdateMe(w http.ResponseWriter, r *http.Request) {
	extID := middleware.AppExtensionIDFromContext(r.Context())
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

// handleAppListVoicemail handles GET /api/v1/app/voicemail — returns voicemail
// messages from all voicemail boxes linked to the authenticated extension via
// notify_extension_id.
func (s *Server) handleAppListVoicemail(w http.ResponseWriter, r *http.Request) {
	extID := middleware.AppExtensionIDFromContext(r.Context())
	if extID == 0 {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Find all voicemail boxes linked to this extension.
	boxes, err := s.voicemailBoxes.ListByNotifyExtensionID(r.Context(), extID)
	if err != nil {
		slog.Error("app list voicemail: failed to query boxes", "error", err, "extension_id", extID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Collect all messages from all linked boxes.
	var allMessages []voicemailMessageResponse
	for _, box := range boxes {
		msgs, err := s.voicemailMessages.ListByMailbox(r.Context(), box.ID)
		if err != nil {
			slog.Error("app list voicemail: failed to query messages", "error", err, "box_id", box.ID)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		for i := range msgs {
			allMessages = append(allMessages, toVoicemailMessageResponse(&msgs[i]))
		}
	}

	if allMessages == nil {
		allMessages = []voicemailMessageResponse{}
	}

	writeJSON(w, http.StatusOK, allMessages)
}

// handleAppMarkVoicemailRead handles PUT /api/v1/app/voicemail/:id/read —
// marks a voicemail message as read, but only if it belongs to a box linked
// to the authenticated extension.
func (s *Server) handleAppMarkVoicemailRead(w http.ResponseWriter, r *http.Request) {
	extID := middleware.AppExtensionIDFromContext(r.Context())
	if extID == 0 {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	msgID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid voicemail message id")
		return
	}

	msg, err := s.voicemailMessages.GetByID(r.Context(), msgID)
	if err != nil {
		slog.Error("app mark voicemail read: failed to query message", "error", err, "msg_id", msgID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if msg == nil {
		writeError(w, http.StatusNotFound, "voicemail message not found")
		return
	}

	// Verify ownership: the message's mailbox must be linked to this extension.
	box, err := s.voicemailBoxes.GetByID(r.Context(), msg.MailboxID)
	if err != nil {
		slog.Error("app mark voicemail read: failed to query box", "error", err, "box_id", msg.MailboxID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if box == nil || box.NotifyExtensionID == nil || *box.NotifyExtensionID != extID {
		writeError(w, http.StatusNotFound, "voicemail message not found")
		return
	}

	if err := s.voicemailMessages.MarkRead(r.Context(), msgID); err != nil {
		slog.Error("app mark voicemail read: failed to update", "error", err, "msg_id", msgID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	updated, err := s.voicemailMessages.GetByID(r.Context(), msgID)
	if err != nil || updated == nil {
		slog.Error("app mark voicemail read: failed to re-fetch", "error", err, "msg_id", msgID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("app voicemail marked read", "msg_id", msgID, "extension_id", extID)

	writeJSON(w, http.StatusOK, toVoicemailMessageResponse(updated))
}

// handleAppGetVoicemailAudio handles GET /api/v1/app/voicemail/:id/audio —
// streams the WAV audio file for a voicemail message owned by the extension.
func (s *Server) handleAppGetVoicemailAudio(w http.ResponseWriter, r *http.Request) {
	extID := middleware.AppExtensionIDFromContext(r.Context())
	if extID == 0 {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	msgID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid voicemail message id")
		return
	}

	msg, err := s.voicemailMessages.GetByID(r.Context(), msgID)
	if err != nil {
		slog.Error("app get voicemail audio: failed to query message", "error", err, "msg_id", msgID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if msg == nil {
		writeError(w, http.StatusNotFound, "voicemail message not found")
		return
	}

	// Verify ownership.
	box, err := s.voicemailBoxes.GetByID(r.Context(), msg.MailboxID)
	if err != nil {
		slog.Error("app get voicemail audio: failed to query box", "error", err, "box_id", msg.MailboxID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if box == nil || box.NotifyExtensionID == nil || *box.NotifyExtensionID != extID {
		writeError(w, http.StatusNotFound, "voicemail message not found")
		return
	}

	f, err := os.Open(msg.FilePath)
	if err != nil {
		slog.Error("app get voicemail audio: failed to open file", "error", err, "msg_id", msgID, "path", msg.FilePath)
		writeError(w, http.StatusInternalServerError, "audio file not found")
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", fmt.Sprintf("voicemail_%d.wav", msgID)))

	http.ServeContent(w, r, fmt.Sprintf("voicemail_%d.wav", msgID), msg.CreatedAt, f)
}

// handleAppHistory handles GET /api/v1/app/history — returns call history
// for the authenticated extension with pagination.
func (s *Server) handleAppHistory(w http.ResponseWriter, r *http.Request) {
	extID := middleware.AppExtensionIDFromContext(r.Context())
	if extID == 0 {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	pg, errMsg := parsePagination(r)
	if errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	// Look up the extension number to match against CDR fields.
	ext, err := s.extensions.GetByID(r.Context(), extID)
	if err != nil {
		slog.Error("app history: failed to query extension", "error", err, "extension_id", extID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if ext == nil {
		writeError(w, http.StatusNotFound, "extension not found")
		return
	}

	cdrs, total, err := s.cdrs.ListByExtension(r.Context(), ext.Extension, pg.Limit, pg.Offset)
	if err != nil {
		slog.Error("app history: failed to query cdrs", "error", err, "extension", ext.Extension)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	items := make([]cdrResponse, len(cdrs))
	for i := range cdrs {
		items[i] = toCDRResponse(&cdrs[i])
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Items:  items,
		Total:  total,
		Limit:  pg.Limit,
		Offset: pg.Offset,
	})
}

// handleAppPushToken handles POST /api/v1/app/push-token — registers or
// updates a push notification token for the authenticated extension.
func (s *Server) handleAppPushToken(w http.ResponseWriter, r *http.Request) {
	extID := middleware.AppExtensionIDFromContext(r.Context())
	if extID == 0 {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req appPushTokenRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if msg := validateRequiredStringLen("token", req.Token, maxPasswordLen); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if req.Platform != "fcm" && req.Platform != "apns" {
		writeError(w, http.StatusBadRequest, "platform must be \"fcm\" or \"apns\"")
		return
	}
	if msg := validateRequiredStringLen("device_id", req.DeviceID, maxNameLen); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if msg := validateStringLen("app_version", req.AppVersion, maxShortStringLen); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	token := &models.PushToken{
		ExtensionID: extID,
		Token:       req.Token,
		Platform:    req.Platform,
		DeviceID:    req.DeviceID,
		AppVersion:  req.AppVersion,
	}

	if err := s.pushTokens.Upsert(r.Context(), token); err != nil {
		slog.Error("app push token: failed to upsert", "error", err, "extension_id", extID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("app push token registered", "extension_id", extID, "platform", req.Platform, "device_id", req.DeviceID)

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

// appDirectoryEntry is a lightweight extension entry for the mobile app directory.
type appDirectoryEntry struct {
	ID        int64  `json:"id"`
	Extension string `json:"extension"`
	Name      string `json:"name"`
	Online    bool   `json:"online"`
}

// handleAppDirectory handles GET /api/v1/app/directory — returns a list of all
// extensions for the PBX contact directory in the mobile app.
func (s *Server) handleAppDirectory(w http.ResponseWriter, r *http.Request) {
	extID := middleware.AppExtensionIDFromContext(r.Context())
	if extID == 0 {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	exts, err := s.extensions.List(r.Context())
	if err != nil {
		slog.Error("app directory: failed to list extensions", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	registeredIDs, err := s.registrations.RegisteredExtensionIDs(r.Context())
	if err != nil {
		slog.Error("app directory: failed to query registrations", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	entries := make([]appDirectoryEntry, len(exts))
	for i := range exts {
		entries[i] = appDirectoryEntry{
			ID:        exts[i].ID,
			Extension: exts[i].Extension,
			Name:      exts[i].Name,
			Online:    registeredIDs[exts[i].ID],
		}
	}

	writeJSON(w, http.StatusOK, entries)
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
		Email:            e.Email,
		DND:              e.DND,
		FollowMeEnabled:  e.FollowMeEnabled,
		FollowMeNumbers:  numbers,
		FollowMeStrategy: strategy,
		FollowMeConfirm:  e.FollowMeConfirm,
		UpdatedAt:        e.UpdatedAt.Format(time.RFC3339),
	}
}
