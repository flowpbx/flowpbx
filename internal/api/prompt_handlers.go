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
	"github.com/go-chi/chi/v5"
)

// maxPromptUploadSize is the upper limit for audio file uploads (10 MB).
const maxPromptUploadSize = 10 << 20

// promptResponse is the JSON response for a single audio prompt.
type promptResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Filename  string `json:"filename"`
	Format    string `json:"format"`
	FileSize  int64  `json:"file_size"`
	CreatedAt string `json:"created_at"`
}

// toPromptResponse converts a models.AudioPrompt to the API response.
func toPromptResponse(p *models.AudioPrompt) promptResponse {
	return promptResponse{
		ID:        p.ID,
		Name:      p.Name,
		Filename:  p.Filename,
		Format:    p.Format,
		FileSize:  p.FileSize,
		CreatedAt: p.CreatedAt.Format(time.RFC3339),
	}
}

// handleListPrompts returns all custom audio prompts.
func (s *Server) handleListPrompts(w http.ResponseWriter, r *http.Request) {
	prompts, err := s.audioPrompts.List(r.Context())
	if err != nil {
		slog.Error("list prompts: failed to query", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	items := make([]promptResponse, len(prompts))
	for i := range prompts {
		items[i] = toPromptResponse(&prompts[i])
	}

	writeJSON(w, http.StatusOK, items)
}

// handleUploadPrompt handles audio file upload via multipart form data.
func (s *Server) handleUploadPrompt(w http.ResponseWriter, r *http.Request) {
	// Limit request body size.
	r.Body = http.MaxBytesReader(w, r.Body, maxPromptUploadSize)

	if err := r.ParseMultipartForm(maxPromptUploadSize); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file field is required")
		return
	}
	defer file.Close()

	name := r.FormValue("name")
	if name == "" {
		// Default name to filename without extension.
		name = strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	}

	// Validate file format by extension.
	ext := strings.ToLower(filepath.Ext(header.Filename))
	var format string
	switch ext {
	case ".wav":
		format = "wav"
	case ".alaw", ".al":
		format = "alaw"
	case ".ulaw", ".ul":
		format = "ulaw"
	default:
		writeError(w, http.StatusBadRequest, "unsupported audio format; accepted: .wav, .alaw, .ulaw")
		return
	}

	// Validate WAV content type hint if provided.
	ct := header.Header.Get("Content-Type")
	if format == "wav" && ct != "" && ct != "application/octet-stream" && ct != "audio/wav" && ct != "audio/wave" && ct != "audio/x-wav" {
		writeError(w, http.StatusBadRequest, "invalid content type for WAV file")
		return
	}

	// Read the file data for validation and storage.
	data, err := io.ReadAll(file)
	if err != nil {
		slog.Error("upload prompt: failed to read file", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to read uploaded file")
		return
	}

	// Basic WAV header validation.
	if format == "wav" {
		if errMsg := validateWAVHeader(data); errMsg != "" {
			writeError(w, http.StatusBadRequest, errMsg)
			return
		}
	}

	// Ensure custom prompts directory exists.
	promptDir := filepath.Join(s.cfg.DataDir, "prompts", "custom")
	if err := os.MkdirAll(promptDir, 0750); err != nil {
		slog.Error("upload prompt: failed to create prompts directory", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Generate a unique filename to avoid collisions.
	storedFilename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), sanitizeFilename(header.Filename))
	filePath := filepath.Join(promptDir, storedFilename)

	if err := os.WriteFile(filePath, data, 0640); err != nil {
		slog.Error("upload prompt: failed to write file", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save file")
		return
	}

	prompt := &models.AudioPrompt{
		Name:     name,
		Filename: header.Filename,
		Format:   format,
		FileSize: int64(len(data)),
		FilePath: storedFilename,
	}

	if err := s.audioPrompts.Create(r.Context(), prompt); err != nil {
		// Clean up the file on database failure.
		os.Remove(filePath)
		slog.Error("upload prompt: failed to insert", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Re-fetch to get the created_at timestamp from the database.
	created, err := s.audioPrompts.GetByID(r.Context(), prompt.ID)
	if err != nil || created == nil {
		slog.Error("upload prompt: failed to re-fetch", "error", err, "prompt_id", prompt.ID)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("audio prompt uploaded", "prompt_id", created.ID, "name", created.Name, "format", created.Format)

	writeJSON(w, http.StatusCreated, toPromptResponse(created))
}

// handleGetPromptAudio streams the audio file for a prompt.
func (s *Server) handleGetPromptAudio(w http.ResponseWriter, r *http.Request) {
	id, err := parsePromptID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid prompt id")
		return
	}

	prompt, err := s.audioPrompts.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("get prompt audio: failed to query", "error", err, "prompt_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if prompt == nil {
		writeError(w, http.StatusNotFound, "prompt not found")
		return
	}

	filePath := filepath.Join(s.cfg.DataDir, "prompts", "custom", prompt.FilePath)

	f, err := os.Open(filePath)
	if err != nil {
		slog.Error("get prompt audio: failed to open file", "error", err, "prompt_id", id, "path", filePath)
		writeError(w, http.StatusInternalServerError, "audio file not found")
		return
	}
	defer f.Close()

	// Set appropriate content type based on format.
	switch prompt.Format {
	case "wav":
		w.Header().Set("Content-Type", "audio/wav")
	case "alaw":
		w.Header().Set("Content-Type", "audio/basic")
	case "ulaw":
		w.Header().Set("Content-Type", "audio/basic")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", prompt.Filename))

	http.ServeContent(w, r, prompt.Filename, prompt.CreatedAt, f)
}

// handleDeletePrompt removes a prompt and its audio file.
func (s *Server) handleDeletePrompt(w http.ResponseWriter, r *http.Request) {
	id, err := parsePromptID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid prompt id")
		return
	}

	existing, err := s.audioPrompts.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("delete prompt: failed to query", "error", err, "prompt_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "prompt not found")
		return
	}

	if err := s.audioPrompts.Delete(r.Context(), id); err != nil {
		slog.Error("delete prompt: failed to delete", "error", err, "prompt_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Remove the audio file from disk.
	filePath := filepath.Join(s.cfg.DataDir, "prompts", "custom", existing.FilePath)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		slog.Warn("delete prompt: failed to remove audio file", "error", err, "path", filePath)
	}

	slog.Info("audio prompt deleted", "prompt_id", id, "name", existing.Name)

	w.WriteHeader(http.StatusNoContent)
}

// parsePromptID extracts and parses the prompt ID from the URL parameter.
func parsePromptID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// validateWAVHeader checks that the data starts with a valid RIFF/WAVE header.
func validateWAVHeader(data []byte) string {
	if len(data) < 12 {
		return "file too small to be a valid WAV"
	}
	if string(data[0:4]) != "RIFF" {
		return "invalid WAV file: missing RIFF header"
	}
	if string(data[8:12]) != "WAVE" {
		return "invalid WAV file: missing WAVE format identifier"
	}
	return ""
}

// sanitizeFilename removes path separators and non-printable characters
// from a filename to prevent path traversal.
func sanitizeFilename(name string) string {
	// Use only the base name.
	name = filepath.Base(name)
	// Replace any remaining path separators.
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	if name == "" || name == "." || name == ".." {
		return "upload"
	}
	return name
}
