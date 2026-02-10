package config

import (
	"log/slog"
	"os"
	"testing"
)

func TestDefaults(t *testing.T) {
	// Clear any env vars that might interfere.
	for _, env := range []string{
		"FLOWPBX_DATA_DIR", "FLOWPBX_HTTP_PORT", "FLOWPBX_SIP_PORT",
		"FLOWPBX_SIP_TLS_PORT", "FLOWPBX_TLS_CERT", "FLOWPBX_TLS_KEY",
		"FLOWPBX_LOG_LEVEL",
	} {
		t.Setenv(env, "")
		os.Unsetenv(env)
	}

	os.Args = []string{"flowpbx"}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DataDir != defaultDataDir {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, defaultDataDir)
	}
	if cfg.HTTPPort != defaultHTTPPort {
		t.Errorf("HTTPPort = %d, want %d", cfg.HTTPPort, defaultHTTPPort)
	}
	if cfg.SIPPort != defaultSIPPort {
		t.Errorf("SIPPort = %d, want %d", cfg.SIPPort, defaultSIPPort)
	}
	if cfg.SIPTLSPort != defaultSIPTLSPort {
		t.Errorf("SIPTLSPort = %d, want %d", cfg.SIPTLSPort, defaultSIPTLSPort)
	}
	if cfg.TLSCert != "" {
		t.Errorf("TLSCert = %q, want empty", cfg.TLSCert)
	}
	if cfg.TLSKey != "" {
		t.Errorf("TLSKey = %q, want empty", cfg.TLSKey)
	}
	if cfg.LogLevel != defaultLogLevel {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, defaultLogLevel)
	}
}

func TestEnvVarOverride(t *testing.T) {
	os.Args = []string{"flowpbx"}
	t.Setenv("FLOWPBX_HTTP_PORT", "9090")
	t.Setenv("FLOWPBX_DATA_DIR", "/tmp/flowpbx-test")
	t.Setenv("FLOWPBX_LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTPPort != 9090 {
		t.Errorf("HTTPPort = %d, want 9090", cfg.HTTPPort)
	}
	if cfg.DataDir != "/tmp/flowpbx-test" {
		t.Errorf("DataDir = %q, want /tmp/flowpbx-test", cfg.DataDir)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug", cfg.LogLevel)
	}
}

func TestCLIFlagsPrecedence(t *testing.T) {
	// CLI flags should override env vars.
	os.Args = []string{"flowpbx", "--http-port", "3000", "--log-level", "warn"}
	t.Setenv("FLOWPBX_HTTP_PORT", "9090")
	t.Setenv("FLOWPBX_LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTPPort != 3000 {
		t.Errorf("HTTPPort = %d, want 3000 (CLI should override env)", cfg.HTTPPort)
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want warn (CLI should override env)", cfg.LogLevel)
	}
}

func TestValidateInvalidPort(t *testing.T) {
	os.Args = []string{"flowpbx", "--http-port", "99999"}
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid port, got nil")
	}
}

func TestValidateInvalidLogLevel(t *testing.T) {
	os.Args = []string{"flowpbx", "--log-level", "verbose"}
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid log level, got nil")
	}
}

func TestValidateTLSMismatch(t *testing.T) {
	os.Args = []string{"flowpbx", "--tls-cert", "cert.pem"}
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when tls-cert provided without tls-key")
	}
}

func TestSlogLevel(t *testing.T) {
	tests := []struct {
		level string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			cfg := &Config{LogLevel: tt.level}
			if got := cfg.SlogLevel(); got != tt.want {
				t.Errorf("SlogLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}
