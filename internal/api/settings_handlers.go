package api

import (
	"log/slog"
	"net/http"
)

// smtpKeys are the system_config keys that make up the SMTP configuration.
// The password key is handled specially (encrypted at rest).
var smtpKeys = []string{
	"smtp_host",
	"smtp_port",
	"smtp_from",
	"smtp_username",
	"smtp_tls",
}

const smtpPasswordKey = "smtp_password"

// settingsResponse is the shape returned by GET /settings.
type settingsResponse struct {
	SMTP smtpSettingsResponse `json:"smtp"`
}

type smtpSettingsResponse struct {
	Host        string `json:"host"`
	Port        string `json:"port"`
	From        string `json:"from"`
	Username    string `json:"username"`
	TLS         string `json:"tls"`
	HasPassword bool   `json:"has_password"`
}

// settingsRequest is the shape accepted by PUT /settings.
type settingsRequest struct {
	SMTP *smtpSettingsRequest `json:"smtp"`
}

type smtpSettingsRequest struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	From     string `json:"from"`
	Username string `json:"username"`
	Password string `json:"password"`
	TLS      string `json:"tls"`
}

// handleGetSettings returns all system settings grouped by section.
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	get := func(key string) string {
		val, _ := s.systemConfig.Get(ctx, key)
		return val
	}

	// Check whether an SMTP password is stored (without revealing it).
	pw, _ := s.systemConfig.Get(ctx, smtpPasswordKey)

	resp := settingsResponse{
		SMTP: smtpSettingsResponse{
			Host:        get("smtp_host"),
			Port:        get("smtp_port"),
			From:        get("smtp_from"),
			Username:    get("smtp_username"),
			TLS:         get("smtp_tls"),
			HasPassword: pw != "",
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleUpdateSettings saves system settings. Only provided sections are updated.
func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req settingsRequest
	if errMsg := readJSON(r, &req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	ctx := r.Context()

	if req.SMTP != nil {
		smtp := req.SMTP

		// Validate TLS mode if provided.
		if smtp.TLS != "" && smtp.TLS != "none" && smtp.TLS != "starttls" && smtp.TLS != "tls" {
			writeError(w, http.StatusBadRequest, "smtp tls must be none, starttls, or tls")
			return
		}

		pairs := map[string]string{
			"smtp_host":     smtp.Host,
			"smtp_port":     smtp.Port,
			"smtp_from":     smtp.From,
			"smtp_username": smtp.Username,
			"smtp_tls":      smtp.TLS,
		}

		for key, value := range pairs {
			if err := s.systemConfig.Set(ctx, key, value); err != nil {
				slog.Error("failed to save setting", "key", key, "error", err)
				writeError(w, http.StatusInternalServerError, "failed to save settings")
				return
			}
		}

		// Handle password separately: only update if a new value is provided.
		// An empty string means "leave unchanged".
		if smtp.Password != "" {
			value := smtp.Password
			if s.encryptor != nil {
				encrypted, err := s.encryptor.Encrypt(value)
				if err != nil {
					slog.Error("failed to encrypt smtp password", "error", err)
					writeError(w, http.StatusInternalServerError, "failed to save settings")
					return
				}
				value = encrypted
			}
			if err := s.systemConfig.Set(ctx, smtpPasswordKey, value); err != nil {
				slog.Error("failed to save smtp password", "error", err)
				writeError(w, http.StatusInternalServerError, "failed to save settings")
				return
			}
		}
	}

	slog.Info("system settings updated")

	// Return the updated settings.
	s.handleGetSettings(w, r)
}
