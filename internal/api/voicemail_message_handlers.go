package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/go-chi/chi/v5"
)

// voicemailMessageResponse is the JSON response for a single voicemail message.
type voicemailMessageResponse struct {
	ID            int64   `json:"id"`
	MailboxID     int64   `json:"mailbox_id"`
	CallerIDName  string  `json:"caller_id_name"`
	CallerIDNum   string  `json:"caller_id_num"`
	Timestamp     string  `json:"timestamp"`
	Duration      int     `json:"duration"`
	Read          bool    `json:"read"`
	ReadAt        *string `json:"read_at"`
	Transcription string  `json:"transcription,omitempty"`
	CreatedAt     string  `json:"created_at"`
}

// handleListVoicemailMessages returns all messages for a voicemail box.
func (s *Server) handleListVoicemailMessages(w http.ResponseWriter, r *http.Request) {
	boxID, err := parseVoicemailBoxID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid voicemail box id")
		return
	}

	// Verify the box exists.
	box, err := s.voicemailBoxes.GetByID(r.Context(), boxID)
	if err != nil {
		slog.Error("list voicemail messages: failed to query box", "error", err, "box_id", boxID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if box == nil {
		writeError(w, http.StatusNotFound, "voicemail box not found")
		return
	}

	msgs, err := s.voicemailMessages.ListByMailbox(r.Context(), boxID)
	if err != nil {
		slog.Error("list voicemail messages: failed to query", "error", err, "box_id", boxID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	items := make([]voicemailMessageResponse, len(msgs))
	for i := range msgs {
		items[i] = toVoicemailMessageResponse(&msgs[i])
	}

	writeJSON(w, http.StatusOK, items)
}

// handleDeleteVoicemailMessage deletes a voicemail message and its audio file.
func (s *Server) handleDeleteVoicemailMessage(w http.ResponseWriter, r *http.Request) {
	boxID, err := parseVoicemailBoxID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid voicemail box id")
		return
	}

	msgID, err := parseVoicemailMessageID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid message id")
		return
	}

	msg, err := s.voicemailMessages.GetByID(r.Context(), msgID)
	if err != nil {
		slog.Error("delete voicemail message: failed to query", "error", err, "msg_id", msgID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if msg == nil || msg.MailboxID != boxID {
		writeError(w, http.StatusNotFound, "voicemail message not found")
		return
	}

	if err := s.voicemailMessages.Delete(r.Context(), msgID); err != nil {
		slog.Error("delete voicemail message: failed to delete", "error", err, "msg_id", msgID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Remove the audio file from disk.
	if msg.FilePath != "" {
		if err := os.Remove(msg.FilePath); err != nil && !os.IsNotExist(err) {
			slog.Warn("delete voicemail message: failed to remove audio file", "error", err, "path", msg.FilePath)
		}
	}

	slog.Info("voicemail message deleted", "msg_id", msgID, "box_id", boxID)

	w.WriteHeader(http.StatusNoContent)
}

// handleMarkVoicemailMessageRead marks a voicemail message as read.
func (s *Server) handleMarkVoicemailMessageRead(w http.ResponseWriter, r *http.Request) {
	boxID, err := parseVoicemailBoxID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid voicemail box id")
		return
	}

	msgID, err := parseVoicemailMessageID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid message id")
		return
	}

	msg, err := s.voicemailMessages.GetByID(r.Context(), msgID)
	if err != nil {
		slog.Error("mark voicemail read: failed to query", "error", err, "msg_id", msgID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if msg == nil || msg.MailboxID != boxID {
		writeError(w, http.StatusNotFound, "voicemail message not found")
		return
	}

	if err := s.voicemailMessages.MarkRead(r.Context(), msgID); err != nil {
		slog.Error("mark voicemail read: failed to update", "error", err, "msg_id", msgID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Re-fetch to return updated state.
	updated, err := s.voicemailMessages.GetByID(r.Context(), msgID)
	if err != nil || updated == nil {
		slog.Error("mark voicemail read: failed to re-fetch", "error", err, "msg_id", msgID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("voicemail message marked read", "msg_id", msgID, "box_id", boxID)

	writeJSON(w, http.StatusOK, toVoicemailMessageResponse(updated))
}

// handleGetVoicemailMessageAudio streams the WAV audio file for a voicemail message.
func (s *Server) handleGetVoicemailMessageAudio(w http.ResponseWriter, r *http.Request) {
	boxID, err := parseVoicemailBoxID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid voicemail box id")
		return
	}

	msgID, err := parseVoicemailMessageID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid message id")
		return
	}

	msg, err := s.voicemailMessages.GetByID(r.Context(), msgID)
	if err != nil {
		slog.Error("get voicemail audio: failed to query", "error", err, "msg_id", msgID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if msg == nil || msg.MailboxID != boxID {
		writeError(w, http.StatusNotFound, "voicemail message not found")
		return
	}

	f, err := os.Open(msg.FilePath)
	if err != nil {
		slog.Error("get voicemail audio: failed to open file", "error", err, "msg_id", msgID, "path", msg.FilePath)
		writeError(w, http.StatusInternalServerError, "audio file not found")
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", fmt.Sprintf("voicemail_%d.wav", msgID)))

	http.ServeContent(w, r, fmt.Sprintf("voicemail_%d.wav", msgID), msg.CreatedAt, f)
}

// parseVoicemailMessageID extracts and parses the message ID from the URL parameter.
func parseVoicemailMessageID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "msgID"), 10, 64)
}

// toVoicemailMessageResponse converts a models.VoicemailMessage to the API response.
func toVoicemailMessageResponse(m *models.VoicemailMessage) voicemailMessageResponse {
	resp := voicemailMessageResponse{
		ID:            m.ID,
		MailboxID:     m.MailboxID,
		CallerIDName:  m.CallerIDName,
		CallerIDNum:   m.CallerIDNum,
		Timestamp:     m.Timestamp.Format(time.RFC3339),
		Duration:      m.Duration,
		Read:          m.Read,
		Transcription: m.Transcription,
		CreatedAt:     m.CreatedAt.Format(time.RFC3339),
	}
	if m.ReadAt != nil {
		t := m.ReadAt.Format(time.RFC3339)
		resp.ReadAt = &t
	}
	return resp
}
