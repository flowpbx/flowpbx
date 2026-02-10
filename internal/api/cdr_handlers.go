package api

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/flowpbx/flowpbx/internal/database"
	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/go-chi/chi/v5"
)

// cdrResponse is the JSON response for a single CDR.
type cdrResponse struct {
	ID            int64   `json:"id"`
	CallID        string  `json:"call_id"`
	StartTime     string  `json:"start_time"`
	AnswerTime    *string `json:"answer_time"`
	EndTime       *string `json:"end_time"`
	Duration      *int    `json:"duration"`
	BillableDur   *int    `json:"billable_dur"`
	CallerIDName  string  `json:"caller_id_name"`
	CallerIDNum   string  `json:"caller_id_num"`
	Callee        string  `json:"callee"`
	TrunkID       *int64  `json:"trunk_id"`
	Direction     string  `json:"direction"`
	Disposition   string  `json:"disposition"`
	RecordingFile string  `json:"recording_file,omitempty"`
	FlowPath      string  `json:"flow_path,omitempty"`
	HangupCause   string  `json:"hangup_cause"`
}

// toCDRResponse converts a models.CDR to the API response.
func toCDRResponse(c *models.CDR) cdrResponse {
	resp := cdrResponse{
		ID:            c.ID,
		CallID:        c.CallID,
		StartTime:     c.StartTime.Format(time.RFC3339),
		Duration:      c.Duration,
		BillableDur:   c.BillableDur,
		CallerIDName:  c.CallerIDName,
		CallerIDNum:   c.CallerIDNum,
		Callee:        c.Callee,
		TrunkID:       c.TrunkID,
		Direction:     c.Direction,
		Disposition:   c.Disposition,
		RecordingFile: c.RecordingFile,
		FlowPath:      c.FlowPath,
		HangupCause:   c.HangupCause,
	}
	if c.AnswerTime != nil {
		s := c.AnswerTime.Format(time.RFC3339)
		resp.AnswerTime = &s
	}
	if c.EndTime != nil {
		s := c.EndTime.Format(time.RFC3339)
		resp.EndTime = &s
	}
	return resp
}

// handleListCDRs returns CDRs with pagination and optional filters.
// Query params: limit, offset, search, direction, start_date, end_date.
func (s *Server) handleListCDRs(w http.ResponseWriter, r *http.Request) {
	pg, errMsg := parsePagination(r)
	if errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	q := r.URL.Query()
	direction := q.Get("direction")
	if direction != "" && direction != "inbound" && direction != "outbound" && direction != "internal" {
		writeError(w, http.StatusBadRequest, "direction must be \"inbound\", \"outbound\", or \"internal\"")
		return
	}

	filter := database.CDRListFilter{
		Limit:     pg.Limit,
		Offset:    pg.Offset,
		Search:    q.Get("search"),
		Direction: direction,
		StartDate: q.Get("start_date"),
		EndDate:   q.Get("end_date"),
	}

	cdrs, total, err := s.cdrs.List(r.Context(), filter)
	if err != nil {
		slog.Error("list cdrs: failed to query", "error", err)
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

// handleGetCDR returns a single CDR by ID.
func (s *Server) handleGetCDR(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid cdr id")
		return
	}

	cdr, err := s.cdrs.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("get cdr: failed to query", "error", err, "cdr_id", id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if cdr == nil {
		writeError(w, http.StatusNotFound, "cdr not found")
		return
	}

	writeJSON(w, http.StatusOK, toCDRResponse(cdr))
}

// handleExportCDRs exports CDRs as CSV with the same filters as list.
func (s *Server) handleExportCDRs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	direction := q.Get("direction")
	if direction != "" && direction != "inbound" && direction != "outbound" && direction != "internal" {
		writeError(w, http.StatusBadRequest, "direction must be \"inbound\", \"outbound\", or \"internal\"")
		return
	}

	// Use a large limit for export (all matching records, capped at 10000).
	filter := database.CDRListFilter{
		Limit:     10000,
		Offset:    0,
		Search:    q.Get("search"),
		Direction: direction,
		StartDate: q.Get("start_date"),
		EndDate:   q.Get("end_date"),
	}

	cdrs, _, err := s.cdrs.List(r.Context(), filter)
	if err != nil {
		slog.Error("export cdrs: failed to query", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=cdrs.csv")

	cw := csv.NewWriter(w)
	// Write header row.
	cw.Write([]string{
		"ID", "Call-ID", "Start Time", "Answer Time", "End Time",
		"Duration", "Billable Duration", "Caller Name", "Caller Number",
		"Callee", "Trunk ID", "Direction", "Disposition", "Hangup Cause",
		"Recording File",
	})

	for _, c := range cdrs {
		answerTime := ""
		if c.AnswerTime != nil {
			answerTime = c.AnswerTime.Format(time.RFC3339)
		}
		endTime := ""
		if c.EndTime != nil {
			endTime = c.EndTime.Format(time.RFC3339)
		}
		duration := ""
		if c.Duration != nil {
			duration = strconv.Itoa(*c.Duration)
		}
		billable := ""
		if c.BillableDur != nil {
			billable = strconv.Itoa(*c.BillableDur)
		}
		trunkID := ""
		if c.TrunkID != nil {
			trunkID = strconv.FormatInt(*c.TrunkID, 10)
		}

		cw.Write([]string{
			strconv.FormatInt(c.ID, 10),
			c.CallID,
			c.StartTime.Format(time.RFC3339),
			answerTime,
			endTime,
			duration,
			billable,
			c.CallerIDName,
			c.CallerIDNum,
			c.Callee,
			trunkID,
			c.Direction,
			c.Disposition,
			c.HangupCause,
			c.RecordingFile,
		})
	}

	cw.Flush()
	if err := cw.Error(); err != nil {
		slog.Error("export cdrs: csv write error", "error", err)
	}
}

// handleDashboardStats returns aggregate statistics for the admin dashboard.
func (s *Server) handleDashboardStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Count extensions.
	exts, err := s.extensions.List(ctx)
	totalExtensions := 0
	if err != nil {
		slog.Error("dashboard stats: failed to count extensions", "error", err)
	} else {
		totalExtensions = len(exts)
	}

	// Count trunks.
	allTrunks, err := s.trunks.List(ctx)
	totalTrunks := 0
	if err != nil {
		slog.Error("dashboard stats: failed to count trunks", "error", err)
	} else {
		totalTrunks = len(allTrunks)
	}

	// Active call count.
	activeCalls := 0
	if s.activeCalls != nil {
		activeCalls = s.activeCalls.GetActiveCallCount()
	}

	// Registered device count.
	registeredDevices := 0
	regCount, err := s.registrations.Count(ctx)
	if err != nil {
		slog.Error("dashboard stats: failed to count registrations", "error", err)
	} else {
		registeredDevices = int(regCount)
	}

	// Recent CDRs.
	recentCDRs, err := s.cdrs.ListRecent(ctx, 10)
	if err != nil {
		slog.Error("dashboard stats: failed to list recent cdrs", "error", err)
		recentCDRs = nil
	}

	type recentCDREntry struct {
		ID        int64  `json:"id"`
		Caller    string `json:"caller"`
		Callee    string `json:"callee"`
		Direction string `json:"direction"`
		Duration  int    `json:"duration"`
		Status    string `json:"status"`
		Timestamp string `json:"timestamp"`
	}

	cdrEntries := make([]recentCDREntry, 0, len(recentCDRs))
	for _, c := range recentCDRs {
		caller := c.CallerIDNum
		if c.CallerIDName != "" {
			caller = fmt.Sprintf("%s <%s>", c.CallerIDName, c.CallerIDNum)
		}
		dur := 0
		if c.Duration != nil {
			dur = *c.Duration
		}
		cdrEntries = append(cdrEntries, recentCDREntry{
			ID:        c.ID,
			Caller:    caller,
			Callee:    c.Callee,
			Direction: c.Direction,
			Duration:  dur,
			Status:    c.Disposition,
			Timestamp: c.StartTime.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"active_calls":       activeCalls,
		"registered_devices": registeredDevices,
		"total_extensions":   totalExtensions,
		"total_trunks":       totalTrunks,
		"recent_cdrs":        cdrEntries,
	})
}
