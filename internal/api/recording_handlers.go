package api

import (
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/flowpbx/flowpbx/internal/database"
	"github.com/go-chi/chi/v5"
)

// recordingResponse is the JSON response for a single recording entry.
// Recordings are CDRs that have a non-empty recording_file.
type recordingResponse struct {
	ID           int64  `json:"id"`
	CallID       string `json:"call_id"`
	StartTime    string `json:"start_time"`
	Duration     *int   `json:"duration"`
	CallerIDName string `json:"caller_id_name"`
	CallerIDNum  string `json:"caller_id_num"`
	Callee       string `json:"callee"`
	Direction    string `json:"direction"`
	Disposition  string `json:"disposition"`
	Filename     string `json:"filename"`
	FileSize     *int64 `json:"file_size"`
}

// handleListRecordings returns CDRs that have recordings, with pagination and search.
// Query params: limit, offset, search, start_date, end_date.
func (s *Server) handleListRecordings(w http.ResponseWriter, r *http.Request) {
	pg, errMsg := parsePagination(r)
	if errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	q := r.URL.Query()

	filter := database.CDRListFilter{
		Limit:     pg.Limit,
		Offset:    pg.Offset,
		Search:    q.Get("search"),
		StartDate: q.Get("start_date"),
		EndDate:   q.Get("end_date"),
	}

	cdrs, total, err := s.cdrs.ListWithRecordings(r.Context(), filter)
	if err != nil {
		slog.Error("list recordings: failed to query", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	items := make([]recordingResponse, len(cdrs))
	for i := range cdrs {
		c := &cdrs[i]
		resp := recordingResponse{
			ID:           c.ID,
			CallID:       c.CallID,
			StartTime:    c.StartTime.Format(time.RFC3339),
			Duration:     c.Duration,
			CallerIDName: c.CallerIDName,
			CallerIDNum:  c.CallerIDNum,
			Callee:       c.Callee,
			Direction:    c.Direction,
			Disposition:  c.Disposition,
			Filename:     filepath.Base(c.RecordingFile),
		}

		// Try to get file size from disk.
		fullPath := s.recordingFilePath(c.RecordingFile)
		if info, err := os.Stat(fullPath); err == nil {
			size := info.Size()
			resp.FileSize = &size
		}

		items[i] = resp
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Items:  items,
		Total:  total,
		Limit:  pg.Limit,
		Offset: pg.Offset,
	})
}

// handleDownloadRecording streams the recording file for a CDR.
func (s *Server) handleDownloadRecording(w http.ResponseWriter, r *http.Request) {
	id, err := parseRecordingID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid recording id")
		return
	}

	cdr, err := s.cdrs.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("download recording: failed to query cdr", "error", err, "cdr_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if cdr == nil || cdr.RecordingFile == "" {
		writeError(w, http.StatusNotFound, "recording not found")
		return
	}

	fullPath := s.recordingFilePath(cdr.RecordingFile)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "recording file not found on disk")
		return
	}

	filename := filepath.Base(cdr.RecordingFile)
	contentType := mime.TypeByExtension(filepath.Ext(filename))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	w.Header().Set("Content-Type", contentType)
	http.ServeFile(w, r, fullPath)
}

// handleDeleteRecording removes the recording file for a CDR and clears the recording_file field.
func (s *Server) handleDeleteRecording(w http.ResponseWriter, r *http.Request) {
	id, err := parseRecordingID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid recording id")
		return
	}

	cdr, err := s.cdrs.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("delete recording: failed to query cdr", "error", err, "cdr_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if cdr == nil || cdr.RecordingFile == "" {
		writeError(w, http.StatusNotFound, "recording not found")
		return
	}

	// Remove the file from disk (ignore if already missing).
	fullPath := s.recordingFilePath(cdr.RecordingFile)
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		slog.Error("delete recording: failed to remove file", "error", err, "path", fullPath)
		writeError(w, http.StatusInternalServerError, "failed to delete recording file")
		return
	}

	// Clear the recording_file field on the CDR.
	cdr.RecordingFile = ""
	if err := s.cdrs.Update(r.Context(), cdr); err != nil {
		slog.Error("delete recording: failed to update cdr", "error", err, "cdr_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	slog.Info("recording deleted", "cdr_id", id, "file", fullPath)

	w.WriteHeader(http.StatusNoContent)
}

// parseRecordingID extracts and parses the recording (CDR) ID from the URL parameter.
func parseRecordingID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// recordingFilePath resolves a recording file path. If the path is relative,
// it is resolved under the data directory's recordings subdirectory.
func (s *Server) recordingFilePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(s.cfg.DataDir, "recordings", path)
}
