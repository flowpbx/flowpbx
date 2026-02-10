package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// systemStatusResponse is the shape returned by GET /system/status.
type systemStatusResponse struct {
	SIP    sipStatusResponse     `json:"sip"`
	Trunks []trunkStatusResponse `json:"trunks"`
	Stats  systemStatsResponse   `json:"stats"`
	Uptime uptimeResponse        `json:"uptime"`
}

type sipStatusResponse struct {
	UDPPort    int    `json:"udp_port"`
	TCPPort    int    `json:"tcp_port"`
	TLSPort    int    `json:"tls_port"`
	ExternalIP string `json:"external_ip"`
	TLSEnabled bool   `json:"tls_enabled"`
}

type trunkStatusResponse struct {
	TrunkID        int64   `json:"trunk_id"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	Status         string  `json:"status"`
	LastError      string  `json:"last_error,omitempty"`
	RetryAttempt   int     `json:"retry_attempt"`
	FailedAt       *string `json:"failed_at,omitempty"`
	RegisteredAt   *string `json:"registered_at,omitempty"`
	ExpiresAt      *string `json:"expires_at,omitempty"`
	LastOptionsAt  *string `json:"last_options_at,omitempty"`
	OptionsHealthy bool    `json:"options_healthy"`
}

type systemStatsResponse struct {
	ActiveCalls       int   `json:"active_calls"`
	RegisteredDevices int64 `json:"registered_devices"`
	TotalExtensions   int   `json:"total_extensions"`
	TotalTrunks       int   `json:"total_trunks"`
}

type uptimeResponse struct {
	StartedAt  string `json:"started_at"`
	UptimeSec  int64  `json:"uptime_sec"`
	UptimeText string `json:"uptime_text"`
}

// handleSystemStatus returns the current system status including SIP stack
// configuration, trunk registrations, active call stats, and uptime.
func (s *Server) handleSystemStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// SIP stack configuration from runtime config.
	sipStatus := sipStatusResponse{
		UDPPort:    s.cfg.SIPPort,
		TCPPort:    s.cfg.SIPPort,
		TLSPort:    s.cfg.SIPTLSPort,
		ExternalIP: s.cfg.MediaIP(),
		TLSEnabled: s.cfg.TLSCert != "",
	}

	// Trunk registration statuses.
	var trunkStatuses []trunkStatusResponse
	if s.trunkStatus != nil {
		entries := s.trunkStatus.GetAllTrunkStatuses()
		trunkStatuses = make([]trunkStatusResponse, len(entries))
		for i, st := range entries {
			item := trunkStatusResponse{
				TrunkID:        st.TrunkID,
				Name:           st.Name,
				Type:           st.Type,
				Status:         st.Status,
				LastError:      st.LastError,
				RetryAttempt:   st.RetryAttempt,
				OptionsHealthy: st.OptionsHealthy,
			}
			if st.FailedAt != nil {
				t := st.FailedAt.Format(time.RFC3339)
				item.FailedAt = &t
			}
			if st.RegisteredAt != nil {
				t := st.RegisteredAt.Format(time.RFC3339)
				item.RegisteredAt = &t
			}
			if st.ExpiresAt != nil {
				t := st.ExpiresAt.Format(time.RFC3339)
				item.ExpiresAt = &t
			}
			if st.LastOptionsAt != nil {
				t := st.LastOptionsAt.Format(time.RFC3339)
				item.LastOptionsAt = &t
			}
			trunkStatuses[i] = item
		}
	}
	if trunkStatuses == nil {
		trunkStatuses = []trunkStatusResponse{}
	}

	// Aggregate stats.
	activeCalls := 0
	if s.activeCalls != nil {
		activeCalls = s.activeCalls.GetActiveCallCount()
	}

	registeredDevices := int64(0)
	regCount, err := s.registrations.Count(ctx)
	if err != nil {
		slog.Error("system status: failed to count registrations", "error", err)
	} else {
		registeredDevices = regCount
	}

	totalExtensions := 0
	exts, err := s.extensions.List(ctx)
	if err != nil {
		slog.Error("system status: failed to count extensions", "error", err)
	} else {
		totalExtensions = len(exts)
	}

	totalTrunks := 0
	allTrunks, err := s.trunks.List(ctx)
	if err != nil {
		slog.Error("system status: failed to count trunks", "error", err)
	} else {
		totalTrunks = len(allTrunks)
	}

	// Uptime.
	now := time.Now()
	uptimeDur := now.Sub(s.startTime)
	uptimeSec := int64(uptimeDur.Seconds())

	resp := systemStatusResponse{
		SIP:    sipStatus,
		Trunks: trunkStatuses,
		Stats: systemStatsResponse{
			ActiveCalls:       activeCalls,
			RegisteredDevices: registeredDevices,
			TotalExtensions:   totalExtensions,
			TotalTrunks:       totalTrunks,
		},
		Uptime: uptimeResponse{
			StartedAt:  s.startTime.Format(time.RFC3339),
			UptimeSec:  uptimeSec,
			UptimeText: formatUptime(uptimeDur),
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleSystemReload triggers a hot-reload of system configuration. This
// re-reads settings from the database and restarts subsystems (trunk
// registrations, etc.) without restarting the process.
func (s *Server) handleSystemReload(w http.ResponseWriter, r *http.Request) {
	if s.configReloader == nil {
		writeError(w, http.StatusNotImplemented, "config reload not available")
		return
	}

	slog.Info("system reload requested")

	if err := s.configReloader.Reload(r.Context()); err != nil {
		slog.Error("system reload failed", "error", err)
		writeError(w, http.StatusInternalServerError, "reload failed: "+err.Error())
		return
	}

	slog.Info("system reload completed")

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"reloaded":  true,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// formatUptime returns a human-readable uptime string like "2d 5h 30m 12s".
func formatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
