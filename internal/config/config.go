package config

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime configuration for the FlowPBX server.
// Precedence: CLI flags > env vars > defaults.
type Config struct {
	DataDir        string
	HTTPPort       int
	SIPPort        int
	SIPTLSPort     int
	RTPPortMin     int
	RTPPortMax     int
	TLSCert        string
	TLSKey         string
	LogLevel       string
	CORSOrigins    string
	ExternalIP     string // public IP for SDP rewriting (media proxy)
	EncryptionKey  string // 32-byte hex-encoded key for AES-256-GCM
	PushGatewayURL string // URL of the push gateway service (e.g., "https://push.flowpbx.com")
	LicenseKey     string // license key for the push gateway
	JWTSecret      string // hex-encoded 32-byte secret for mobile app JWT signing
	ACMEDomain     string // domain for automatic Let's Encrypt certificate (e.g., "pbx.example.com")
	ACMEEmail      string // contact email for Let's Encrypt account notifications
	LogFormat      string // log output format: "text" or "json"
}

// defaults
const (
	defaultDataDir    = "./data"
	defaultHTTPPort   = 8080
	defaultSIPPort    = 5060
	defaultSIPTLSPort = 5061
	defaultRTPPortMin = 10000
	defaultRTPPortMax = 20000
	defaultLogLevel   = "info"
	defaultLogFormat  = "text"
)

// envPrefix is the prefix for all FlowPBX environment variables.
const envPrefix = "FLOWPBX_"

// Load parses configuration from CLI flags and environment variables.
// Precedence: CLI flags > env vars > defaults.
func Load() (*Config, error) {
	cfg := &Config{}

	fs := flag.NewFlagSet("flowpbx", flag.ContinueOnError)

	fs.StringVar(&cfg.DataDir, "data-dir", defaultDataDir, "data directory for database and file storage")
	fs.IntVar(&cfg.HTTPPort, "http-port", defaultHTTPPort, "HTTP server listen port")
	fs.IntVar(&cfg.SIPPort, "sip-port", defaultSIPPort, "SIP UDP/TCP listen port")
	fs.IntVar(&cfg.SIPTLSPort, "sip-tls-port", defaultSIPTLSPort, "SIP TLS listen port")
	fs.IntVar(&cfg.RTPPortMin, "rtp-port-min", defaultRTPPortMin, "minimum UDP port for RTP media relay")
	fs.IntVar(&cfg.RTPPortMax, "rtp-port-max", defaultRTPPortMax, "maximum UDP port for RTP media relay")
	fs.StringVar(&cfg.TLSCert, "tls-cert", "", "path to TLS certificate file")
	fs.StringVar(&cfg.TLSKey, "tls-key", "", "path to TLS private key file")
	fs.StringVar(&cfg.LogLevel, "log-level", defaultLogLevel, "log level (debug, info, warn, error)")
	fs.StringVar(&cfg.CORSOrigins, "cors-origins", "", "comma-separated list of allowed CORS origins (use * for all)")
	fs.StringVar(&cfg.ExternalIP, "external-ip", "", "public IP address for SDP rewriting (auto-detected if empty)")
	fs.StringVar(&cfg.EncryptionKey, "encryption-key", "", "hex-encoded 32-byte key for AES-256-GCM encryption of sensitive fields")
	fs.StringVar(&cfg.PushGatewayURL, "push-gateway-url", "", "URL of the push gateway service for mobile push notifications")
	fs.StringVar(&cfg.LicenseKey, "license-key", "", "license key for authenticating with the push gateway")
	fs.StringVar(&cfg.JWTSecret, "jwt-secret", "", "hex-encoded 32-byte secret for mobile app JWT signing (auto-generated if empty)")
	fs.StringVar(&cfg.ACMEDomain, "acme-domain", "", "domain for automatic Let's Encrypt TLS certificate (e.g., pbx.example.com)")
	fs.StringVar(&cfg.ACMEEmail, "acme-email", "", "contact email for Let's Encrypt account notifications")
	fs.StringVar(&cfg.LogFormat, "log-format", defaultLogFormat, "log output format (text, json)")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return nil, fmt.Errorf("parsing flags: %w", err)
	}

	// Apply env var overrides for any flags not explicitly set on the command line.
	// CLI flags take precedence over env vars.
	applyEnvOverrides(fs, cfg)

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

// applyEnvOverrides checks environment variables for any flag that was not
// explicitly provided on the command line. This preserves the precedence:
// CLI flags > env vars > defaults.
func applyEnvOverrides(fs *flag.FlagSet, cfg *Config) {
	// Track which flags were explicitly set via CLI.
	set := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		set[f.Name] = true
	})

	// Map of flag name to env var name.
	envMap := map[string]string{
		"data-dir":         envPrefix + "DATA_DIR",
		"http-port":        envPrefix + "HTTP_PORT",
		"sip-port":         envPrefix + "SIP_PORT",
		"sip-tls-port":     envPrefix + "SIP_TLS_PORT",
		"rtp-port-min":     envPrefix + "RTP_PORT_MIN",
		"rtp-port-max":     envPrefix + "RTP_PORT_MAX",
		"tls-cert":         envPrefix + "TLS_CERT",
		"tls-key":          envPrefix + "TLS_KEY",
		"log-level":        envPrefix + "LOG_LEVEL",
		"cors-origins":     envPrefix + "CORS_ORIGINS",
		"external-ip":      envPrefix + "EXTERNAL_IP",
		"encryption-key":   envPrefix + "ENCRYPTION_KEY",
		"push-gateway-url": envPrefix + "PUSH_GATEWAY_URL",
		"license-key":      envPrefix + "LICENSE_KEY",
		"jwt-secret":       envPrefix + "JWT_SECRET",
		"acme-domain":      envPrefix + "ACME_DOMAIN",
		"acme-email":       envPrefix + "ACME_EMAIL",
		"log-format":       envPrefix + "LOG_FORMAT",
	}

	for flagName, envVar := range envMap {
		if set[flagName] {
			continue
		}
		val, ok := os.LookupEnv(envVar)
		if !ok || val == "" {
			continue
		}
		switch flagName {
		case "data-dir":
			cfg.DataDir = val
		case "http-port":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.HTTPPort = v
			}
		case "sip-port":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.SIPPort = v
			}
		case "sip-tls-port":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.SIPTLSPort = v
			}
		case "rtp-port-min":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.RTPPortMin = v
			}
		case "rtp-port-max":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.RTPPortMax = v
			}
		case "tls-cert":
			cfg.TLSCert = val
		case "tls-key":
			cfg.TLSKey = val
		case "log-level":
			cfg.LogLevel = val
		case "cors-origins":
			cfg.CORSOrigins = val
		case "external-ip":
			cfg.ExternalIP = val
		case "encryption-key":
			cfg.EncryptionKey = val
		case "push-gateway-url":
			cfg.PushGatewayURL = val
		case "license-key":
			cfg.LicenseKey = val
		case "jwt-secret":
			cfg.JWTSecret = val
		case "acme-domain":
			cfg.ACMEDomain = val
		case "acme-email":
			cfg.ACMEEmail = val
		case "log-format":
			cfg.LogFormat = val
		}
	}
}

// validate checks that the config values are sane.
func (c *Config) validate() error {
	if c.HTTPPort < 1 || c.HTTPPort > 65535 {
		return fmt.Errorf("http-port must be between 1 and 65535, got %d", c.HTTPPort)
	}
	if c.SIPPort < 1 || c.SIPPort > 65535 {
		return fmt.Errorf("sip-port must be between 1 and 65535, got %d", c.SIPPort)
	}
	if c.SIPTLSPort < 1 || c.SIPTLSPort > 65535 {
		return fmt.Errorf("sip-tls-port must be between 1 and 65535, got %d", c.SIPTLSPort)
	}
	if c.RTPPortMin < 1024 || c.RTPPortMin > 65534 {
		return fmt.Errorf("rtp-port-min must be between 1024 and 65534, got %d", c.RTPPortMin)
	}
	if c.RTPPortMax < c.RTPPortMin+2 || c.RTPPortMax > 65535 {
		return fmt.Errorf("rtp-port-max must be between rtp-port-min+2 and 65535, got %d", c.RTPPortMax)
	}
	// RTP ports must be even (RTP uses even ports, RTCP uses the next odd port).
	if c.RTPPortMin%2 != 0 {
		return fmt.Errorf("rtp-port-min must be even, got %d", c.RTPPortMin)
	}
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[strings.ToLower(c.LogLevel)] {
		return fmt.Errorf("log-level must be one of debug, info, warn, error; got %q", c.LogLevel)
	}
	c.LogLevel = strings.ToLower(c.LogLevel)

	validFormats := map[string]bool{"text": true, "json": true}
	if !validFormats[strings.ToLower(c.LogFormat)] {
		return fmt.Errorf("log-format must be one of text, json; got %q", c.LogFormat)
	}
	c.LogFormat = strings.ToLower(c.LogFormat)

	// TLS cert and key must both be set or both be empty.
	if (c.TLSCert == "") != (c.TLSKey == "") {
		return fmt.Errorf("tls-cert and tls-key must both be provided or both be omitted")
	}

	// ACME domain and manual TLS cert/key are mutually exclusive.
	if c.ACMEDomain != "" && c.TLSCert != "" {
		return fmt.Errorf("acme-domain and tls-cert/tls-key are mutually exclusive")
	}

	return nil
}

// TLSEnabled returns true if either manual TLS certificates or automatic
// ACME (Let's Encrypt) certificates are configured.
func (c *Config) TLSEnabled() bool {
	return c.TLSCert != "" || c.ACMEDomain != ""
}

// EncryptionKeyBytes returns the decoded 32-byte encryption key, or nil if
// no key is configured.
func (c *Config) EncryptionKeyBytes() ([]byte, error) {
	if c.EncryptionKey == "" {
		return nil, nil
	}
	key, err := hex.DecodeString(c.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("decoding encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must decode to 32 bytes, got %d", len(key))
	}
	return key, nil
}

// JWTSecretBytes returns the decoded 32-byte JWT signing secret.
// If no secret is configured, it generates a random 32-byte key and stores
// the hex-encoded value back in the config for the process lifetime.
func (c *Config) JWTSecretBytes() ([]byte, error) {
	if c.JWTSecret == "" {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("generating jwt secret: %w", err)
		}
		c.JWTSecret = hex.EncodeToString(key)
		slog.Warn("no jwt-secret configured, generated ephemeral key (tokens will not survive restart)")
		return key, nil
	}
	key, err := hex.DecodeString(c.JWTSecret)
	if err != nil {
		return nil, fmt.Errorf("decoding jwt secret: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("jwt secret must decode to 32 bytes, got %d", len(key))
	}
	return key, nil
}

// SIPHost returns the hostname to use for the SIP User-Agent. It defaults
// to the machine hostname if not set via system config.
func (c *Config) SIPHost() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "localhost"
	}
	return hostname
}

// MediaIP returns the IP address to use in SDP for the media proxy.
// If ExternalIP is configured, it is returned directly. Otherwise the
// function attempts to detect the machine's primary non-loopback IPv4 address.
// Falls back to "127.0.0.1" if detection fails.
func (c *Config) MediaIP() string {
	if c.ExternalIP != "" {
		return c.ExternalIP
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

// SlogHandler returns a slog.Handler configured with the appropriate format
// (text or json) and log level.
func (c *Config) SlogHandler(w *os.File) slog.Handler {
	opts := &slog.HandlerOptions{Level: c.SlogLevel()}
	if c.LogFormat == "json" {
		return slog.NewJSONHandler(w, opts)
	}
	return slog.NewTextHandler(w, opts)
}

// SlogLevel returns the slog.Level corresponding to the configured log level.
func (c *Config) SlogLevel() slog.Level {
	switch c.LogLevel {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
