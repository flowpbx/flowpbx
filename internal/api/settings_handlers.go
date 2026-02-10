package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

const smtpPasswordKey = "smtp_password"

// settingsResponse is the shape returned by GET /settings.
type settingsResponse struct {
	SIP       sipSettingsResponse       `json:"sip"`
	Codecs    codecsSettingsResponse    `json:"codecs"`
	Recording recordingSettingsResponse `json:"recording"`
	SMTP      smtpSettingsResponse      `json:"smtp"`
	License   licenseSettingsResponse   `json:"license"`
	Push      pushSettingsResponse      `json:"push"`
}

type sipSettingsResponse struct {
	UDPPort    string `json:"udp_port"`
	TCPPort    string `json:"tcp_port"`
	TLSPort    string `json:"tls_port"`
	TLSCert    string `json:"tls_cert"`
	TLSKey     string `json:"tls_key"`
	ExternalIP string `json:"external_ip"`
	Hostname   string `json:"hostname"`
}

type codecsSettingsResponse struct {
	Audio string `json:"audio"` // comma-separated list, e.g. "g711u,g711a,opus"
}

type recordingSettingsResponse struct {
	StoragePath string `json:"storage_path"`
	Format      string `json:"format"`
	MaxDays     string `json:"max_days"`
}

type smtpSettingsResponse struct {
	Host        string `json:"host"`
	Port        string `json:"port"`
	From        string `json:"from"`
	Username    string `json:"username"`
	TLS         string `json:"tls"`
	HasPassword bool   `json:"has_password"`
}

type licenseSettingsResponse struct {
	Key        string `json:"key"`
	HasKey     bool   `json:"has_key"`
	InstanceID string `json:"instance_id"`
}

type pushSettingsResponse struct {
	GatewayURL string `json:"gateway_url"`
}

// settingsRequest is the shape accepted by PUT /settings.
type settingsRequest struct {
	SIP       *sipSettingsRequest       `json:"sip"`
	Codecs    *codecsSettingsRequest    `json:"codecs"`
	Recording *recordingSettingsRequest `json:"recording"`
	SMTP      *smtpSettingsRequest      `json:"smtp"`
	License   *licenseSettingsRequest   `json:"license"`
	Push      *pushSettingsRequest      `json:"push"`
}

type sipSettingsRequest struct {
	UDPPort    string `json:"udp_port"`
	TCPPort    string `json:"tcp_port"`
	TLSPort    string `json:"tls_port"`
	TLSCert    string `json:"tls_cert"`
	TLSKey     string `json:"tls_key"`
	ExternalIP string `json:"external_ip"`
	Hostname   string `json:"hostname"`
}

type codecsSettingsRequest struct {
	Audio string `json:"audio"`
}

type recordingSettingsRequest struct {
	StoragePath string `json:"storage_path"`
	Format      string `json:"format"`
	MaxDays     string `json:"max_days"`
}

type smtpSettingsRequest struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	From     string `json:"from"`
	Username string `json:"username"`
	Password string `json:"password"`
	TLS      string `json:"tls"`
}

type licenseSettingsRequest struct {
	Key string `json:"key"`
}

type pushSettingsRequest struct {
	GatewayURL string `json:"gateway_url"`
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

	// Check whether a license key is stored (without revealing it).
	licenseKey, _ := s.systemConfig.Get(ctx, "license_key")

	resp := settingsResponse{
		SIP: sipSettingsResponse{
			UDPPort:    get("sip_port"),
			TCPPort:    get("sip_tcp_port"),
			TLSPort:    get("sip_tls_port"),
			TLSCert:    get("sip_tls_cert"),
			TLSKey:     get("sip_tls_key"),
			ExternalIP: get("sip_external_ip"),
			Hostname:   get("hostname"),
		},
		Codecs: codecsSettingsResponse{
			Audio: get("codecs_audio"),
		},
		Recording: recordingSettingsResponse{
			StoragePath: get("recording_storage_path"),
			Format:      get("recording_format"),
			MaxDays:     get("recording_max_days"),
		},
		SMTP: smtpSettingsResponse{
			Host:        get("smtp_host"),
			Port:        get("smtp_port"),
			From:        get("smtp_from"),
			Username:    get("smtp_username"),
			TLS:         get("smtp_tls"),
			HasPassword: pw != "",
		},
		License: licenseSettingsResponse{
			HasKey:     licenseKey != "",
			InstanceID: get("license_instance_id"),
		},
		Push: pushSettingsResponse{
			GatewayURL: get("push_gateway_url"),
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

	// Helper to save a batch of key-value pairs.
	save := func(pairs map[string]string) error {
		for key, value := range pairs {
			if err := s.systemConfig.Set(ctx, key, value); err != nil {
				return err
			}
		}
		return nil
	}

	// SIP settings.
	if req.SIP != nil {
		sip := req.SIP

		// Validate port numbers if provided.
		for _, p := range []struct {
			name string
			val  string
		}{
			{"sip udp_port", sip.UDPPort},
			{"sip tcp_port", sip.TCPPort},
			{"sip tls_port", sip.TLSPort},
		} {
			if p.val != "" {
				port, err := strconv.Atoi(p.val)
				if err != nil || port < 1 || port > 65535 {
					writeError(w, http.StatusBadRequest, p.name+" must be a valid port (1-65535)")
					return
				}
			}
		}

		if err := save(map[string]string{
			"sip_port":        sip.UDPPort,
			"sip_tcp_port":    sip.TCPPort,
			"sip_tls_port":    sip.TLSPort,
			"sip_tls_cert":    sip.TLSCert,
			"sip_tls_key":     sip.TLSKey,
			"sip_external_ip": sip.ExternalIP,
			"hostname":        sip.Hostname,
		}); err != nil {
			slog.Error("failed to save sip settings", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to save settings")
			return
		}
	}

	// Codecs settings.
	if req.Codecs != nil {
		audio := strings.TrimSpace(req.Codecs.Audio)
		if audio != "" {
			// Validate codec names.
			validCodecs := map[string]bool{
				"g711u": true, "g711a": true, "opus": true,
			}
			for _, c := range strings.Split(audio, ",") {
				c = strings.TrimSpace(c)
				if !validCodecs[c] {
					writeError(w, http.StatusBadRequest, "unknown codec: "+c)
					return
				}
			}
		}
		if err := s.systemConfig.Set(ctx, "codecs_audio", audio); err != nil {
			slog.Error("failed to save codecs settings", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to save settings")
			return
		}
	}

	// Recording settings.
	if req.Recording != nil {
		rec := req.Recording

		if rec.Format != "" {
			validFormats := map[string]bool{"wav": true, "mp3": true}
			if !validFormats[rec.Format] {
				writeError(w, http.StatusBadRequest, "recording format must be wav or mp3")
				return
			}
		}

		if rec.MaxDays != "" {
			days, err := strconv.Atoi(rec.MaxDays)
			if err != nil || days < 0 {
				writeError(w, http.StatusBadRequest, "recording max_days must be a non-negative integer")
				return
			}
		}

		if err := save(map[string]string{
			"recording_storage_path": rec.StoragePath,
			"recording_format":       rec.Format,
			"recording_max_days":     rec.MaxDays,
		}); err != nil {
			slog.Error("failed to save recording settings", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to save settings")
			return
		}
	}

	// SMTP settings.
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

		if err := save(pairs); err != nil {
			slog.Error("failed to save smtp settings", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to save settings")
			return
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

	// License settings.
	if req.License != nil {
		// License key is encrypted at rest.
		if req.License.Key != "" {
			value := req.License.Key
			if s.encryptor != nil {
				encrypted, err := s.encryptor.Encrypt(value)
				if err != nil {
					slog.Error("failed to encrypt license key", "error", err)
					writeError(w, http.StatusInternalServerError, "failed to save settings")
					return
				}
				value = encrypted
			}
			if err := s.systemConfig.Set(ctx, "license_key", value); err != nil {
				slog.Error("failed to save license key", "error", err)
				writeError(w, http.StatusInternalServerError, "failed to save settings")
				return
			}
		}
	}

	// Push gateway settings.
	if req.Push != nil {
		if err := s.systemConfig.Set(ctx, "push_gateway_url", req.Push.GatewayURL); err != nil {
			slog.Error("failed to save push gateway settings", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to save settings")
			return
		}
	}

	slog.Info("system settings updated")

	// Return the updated settings.
	s.handleGetSettings(w, r)
}
