package api

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/flowpbx/flowpbx/internal/media"
	"github.com/flowpbx/flowpbx/internal/prompts"
	"github.com/go-chi/chi/v5"
)

// voicemailBoxRequest is the JSON request body for creating/updating a voicemail box.
type voicemailBoxRequest struct {
	Name               string `json:"name"`
	MailboxNumber      string `json:"mailbox_number"`
	PIN                string `json:"pin"`
	GreetingType       string `json:"greeting_type"`
	EmailNotify        *bool  `json:"email_notify"`
	EmailAddress       string `json:"email_address"`
	EmailAttachAudio   *bool  `json:"email_attach_audio"`
	MaxMessageDuration *int   `json:"max_message_duration"`
	MaxMessages        *int   `json:"max_messages"`
	RetentionDays      *int   `json:"retention_days"`
	NotifyExtensionID  *int64 `json:"notify_extension_id"`
}

// voicemailBoxResponse is the JSON response for a single voicemail box.
// PIN is never returned.
type voicemailBoxResponse struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	MailboxNumber      string `json:"mailbox_number"`
	GreetingFile       string `json:"greeting_file"`
	GreetingType       string `json:"greeting_type"`
	EmailNotify        bool   `json:"email_notify"`
	EmailAddress       string `json:"email_address"`
	EmailAttachAudio   bool   `json:"email_attach_audio"`
	MaxMessageDuration int    `json:"max_message_duration"`
	MaxMessages        int    `json:"max_messages"`
	RetentionDays      int    `json:"retention_days"`
	NotifyExtensionID  *int64 `json:"notify_extension_id"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

// toVoicemailBoxResponse converts a models.VoicemailBox to the API response.
func toVoicemailBoxResponse(b *models.VoicemailBox) voicemailBoxResponse {
	return voicemailBoxResponse{
		ID:                 b.ID,
		Name:               b.Name,
		MailboxNumber:      b.MailboxNumber,
		GreetingFile:       b.GreetingFile,
		GreetingType:       b.GreetingType,
		EmailNotify:        b.EmailNotify,
		EmailAddress:       b.EmailAddress,
		EmailAttachAudio:   b.EmailAttachAudio,
		MaxMessageDuration: b.MaxMessageDuration,
		MaxMessages:        b.MaxMessages,
		RetentionDays:      b.RetentionDays,
		NotifyExtensionID:  b.NotifyExtensionID,
		CreatedAt:          b.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          b.UpdatedAt.Format(time.RFC3339),
	}
}

// handleListVoicemailBoxes returns voicemail boxes with pagination.
func (s *Server) handleListVoicemailBoxes(w http.ResponseWriter, r *http.Request) {
	pg, errMsg := parsePagination(r)
	if errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	boxes, err := s.voicemailBoxes.List(r.Context())
	if err != nil {
		slog.Error("list voicemail boxes: failed to query", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	all := make([]voicemailBoxResponse, len(boxes))
	for i := range boxes {
		all[i] = toVoicemailBoxResponse(&boxes[i])
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

// handleCreateVoicemailBox creates a new voicemail box.
func (s *Server) handleCreateVoicemailBox(w http.ResponseWriter, r *http.Request) {
	var req voicemailBoxRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if errMsg := validateVoicemailBoxRequest(req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	box := &models.VoicemailBox{
		Name:               req.Name,
		MailboxNumber:      req.MailboxNumber,
		GreetingType:       "default",
		EmailNotify:        false,
		EmailAddress:       req.EmailAddress,
		EmailAttachAudio:   true,
		MaxMessageDuration: 120,
		MaxMessages:        50,
		RetentionDays:      90,
		NotifyExtensionID:  req.NotifyExtensionID,
	}

	// Hash PIN if provided.
	if req.PIN != "" {
		box.PIN = req.PIN // PIN hashing handled by the repository or a future sprint
	}

	// Apply optional fields.
	if req.GreetingType != "" {
		box.GreetingType = req.GreetingType
	}
	if req.EmailNotify != nil {
		box.EmailNotify = *req.EmailNotify
	}
	if req.EmailAttachAudio != nil {
		box.EmailAttachAudio = *req.EmailAttachAudio
	}
	if req.MaxMessageDuration != nil {
		box.MaxMessageDuration = *req.MaxMessageDuration
	}
	if req.MaxMessages != nil {
		box.MaxMessages = *req.MaxMessages
	}
	if req.RetentionDays != nil {
		box.RetentionDays = *req.RetentionDays
	}

	if err := s.voicemailBoxes.Create(r.Context(), box); err != nil {
		slog.Error("create voicemail box: failed to insert", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Re-fetch to get timestamps populated by the database.
	created, err := s.voicemailBoxes.GetByID(r.Context(), box.ID)
	if err != nil || created == nil {
		slog.Error("create voicemail box: failed to re-fetch", "error", err, "box_id", box.ID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("voicemail box created", "box_id", created.ID, "name", created.Name)

	writeJSON(w, http.StatusCreated, toVoicemailBoxResponse(created))
}

// handleGetVoicemailBox returns a single voicemail box by ID.
func (s *Server) handleGetVoicemailBox(w http.ResponseWriter, r *http.Request) {
	id, err := parseVoicemailBoxID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid voicemail box id")
		return
	}

	box, err := s.voicemailBoxes.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("get voicemail box: failed to query", "error", err, "box_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if box == nil {
		writeError(w, http.StatusNotFound, "voicemail box not found")
		return
	}

	writeJSON(w, http.StatusOK, toVoicemailBoxResponse(box))
}

// handleUpdateVoicemailBox updates an existing voicemail box.
func (s *Server) handleUpdateVoicemailBox(w http.ResponseWriter, r *http.Request) {
	id, err := parseVoicemailBoxID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid voicemail box id")
		return
	}

	existing, err := s.voicemailBoxes.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("update voicemail box: failed to query", "error", err, "box_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "voicemail box not found")
		return
	}

	var req voicemailBoxRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if errMsg := validateVoicemailBoxRequest(req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	// Update fields from request.
	existing.Name = req.Name
	existing.MailboxNumber = req.MailboxNumber
	existing.EmailAddress = req.EmailAddress
	existing.NotifyExtensionID = req.NotifyExtensionID

	// Only update PIN if a new one is provided.
	if req.PIN != "" {
		existing.PIN = req.PIN
	}

	if req.GreetingType != "" {
		existing.GreetingType = req.GreetingType
	}
	if req.EmailNotify != nil {
		existing.EmailNotify = *req.EmailNotify
	}
	if req.EmailAttachAudio != nil {
		existing.EmailAttachAudio = *req.EmailAttachAudio
	}
	if req.MaxMessageDuration != nil {
		existing.MaxMessageDuration = *req.MaxMessageDuration
	}
	if req.MaxMessages != nil {
		existing.MaxMessages = *req.MaxMessages
	}
	if req.RetentionDays != nil {
		existing.RetentionDays = *req.RetentionDays
	}

	if err := s.voicemailBoxes.Update(r.Context(), existing); err != nil {
		slog.Error("update voicemail box: failed to update", "error", err, "box_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Re-fetch to get updated timestamp.
	updated, err := s.voicemailBoxes.GetByID(r.Context(), id)
	if err != nil || updated == nil {
		slog.Error("update voicemail box: failed to re-fetch", "error", err, "box_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("voicemail box updated", "box_id", id, "name", updated.Name)

	writeJSON(w, http.StatusOK, toVoicemailBoxResponse(updated))
}

// handleDeleteVoicemailBox removes a voicemail box by ID.
func (s *Server) handleDeleteVoicemailBox(w http.ResponseWriter, r *http.Request) {
	id, err := parseVoicemailBoxID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid voicemail box id")
		return
	}

	existing, err := s.voicemailBoxes.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("delete voicemail box: failed to query", "error", err, "box_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "voicemail box not found")
		return
	}

	if err := s.voicemailBoxes.Delete(r.Context(), id); err != nil {
		slog.Error("delete voicemail box: failed to delete", "error", err, "box_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("voicemail box deleted", "box_id", id, "name", existing.Name)

	w.WriteHeader(http.StatusNoContent)
}

// maxGreetingUploadSize is the upper limit for greeting file uploads (10 MB).
const maxGreetingUploadSize = 10 << 20

// handleUploadGreeting handles custom greeting WAV upload for a voicemail box.
func (s *Server) handleUploadGreeting(w http.ResponseWriter, r *http.Request) {
	id, err := parseVoicemailBoxID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid voicemail box id")
		return
	}

	box, err := s.voicemailBoxes.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("upload greeting: failed to query box", "error", err, "box_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if box == nil {
		writeError(w, http.StatusNotFound, "voicemail box not found")
		return
	}

	// Limit request body size.
	r.Body = http.MaxBytesReader(w, r.Body, maxGreetingUploadSize)

	if err := r.ParseMultipartForm(maxGreetingUploadSize); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file field is required")
		return
	}
	defer file.Close()

	// Validate file extension.
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".wav" {
		writeError(w, http.StatusBadRequest, "unsupported audio format; only .wav files are accepted for greetings")
		return
	}

	// Read file data for validation and storage.
	data, err := io.ReadAll(file)
	if err != nil {
		slog.Error("upload greeting: failed to read file", "error", err, "box_id", id)
		writeError(w, http.StatusInternalServerError, "failed to read uploaded file")
		return
	}

	// Validate WAV format: must be G.711 (alaw/ulaw), 8kHz, mono, 8-bit.
	if err := media.ValidateWAVData(data); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid wav file: %s", err))
		return
	}

	// Write greeting to standard path: $DATA_DIR/greetings/box_{id}.wav
	greetingPath := prompts.GreetingPath(s.cfg.DataDir, id)

	// Ensure greetings directory exists.
	if err := os.MkdirAll(filepath.Dir(greetingPath), 0750); err != nil {
		slog.Error("upload greeting: failed to create directory", "error", err, "box_id", id)
		writeError(w, http.StatusInternalServerError, "failed to save greeting")
		return
	}

	if err := os.WriteFile(greetingPath, data, 0640); err != nil {
		slog.Error("upload greeting: failed to write file", "error", err, "box_id", id, "path", greetingPath)
		writeError(w, http.StatusInternalServerError, "failed to save greeting")
		return
	}

	// Update box to use the custom greeting.
	box.GreetingFile = greetingPath
	box.GreetingType = "custom"

	if err := s.voicemailBoxes.Update(r.Context(), box); err != nil {
		// Clean up file on database failure.
		os.Remove(greetingPath)
		slog.Error("upload greeting: failed to update box", "error", err, "box_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Re-fetch to get updated timestamp.
	updated, err := s.voicemailBoxes.GetByID(r.Context(), id)
	if err != nil || updated == nil {
		slog.Error("upload greeting: failed to re-fetch box", "error", err, "box_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("voicemail greeting uploaded", "box_id", id, "name", updated.Name)

	writeJSON(w, http.StatusOK, toVoicemailBoxResponse(updated))
}

// parseVoicemailBoxID extracts and parses the voicemail box ID from the URL parameter.
func parseVoicemailBoxID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// validateVoicemailBoxRequest checks required fields for a voicemail box create/update.
func validateVoicemailBoxRequest(req voicemailBoxRequest) string {
	if msg := validateRequiredStringLen("name", req.Name, maxNameLen); msg != "" {
		return msg
	}
	if msg := validateNoControlChars("name", req.Name); msg != "" {
		return msg
	}
	if msg := validateRequiredStringLen("mailbox_number", req.MailboxNumber, maxShortStringLen); msg != "" {
		return msg
	}
	if msg := validateExtensionNumber("mailbox_number", req.MailboxNumber); msg != "" {
		return msg
	}
	if msg := validatePIN("pin", req.PIN); msg != "" {
		return msg
	}
	if req.GreetingType != "" && req.GreetingType != "default" && req.GreetingType != "custom" && req.GreetingType != "name_only" {
		return "greeting_type must be \"default\", \"custom\", or \"name_only\""
	}
	if msg := validateIntRange("max_message_duration", req.MaxMessageDuration, 1, 3600); msg != "" {
		return msg
	}
	if msg := validateIntRange("max_messages", req.MaxMessages, 1, 10000); msg != "" {
		return msg
	}
	if msg := validateIntRange("retention_days", req.RetentionDays, 0, 3650); msg != "" {
		return msg
	}
	if msg := validateEmail("email_address", req.EmailAddress); msg != "" {
		return msg
	}
	if req.EmailNotify != nil && *req.EmailNotify && req.EmailAddress == "" {
		return "email_address is required when email_notify is enabled"
	}
	return ""
}
