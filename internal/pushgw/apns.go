package pushgw

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

const (
	apnsProductionURL = "https://api.push.apple.com"
	apnsSandboxURL    = "https://api.sandbox.push.apple.com"

	// APNs provider tokens are valid for up to 60 minutes.
	// Refresh at 50 minutes to avoid edge-case expiry.
	apnsTokenRefreshInterval = 50 * time.Minute
)

// APNsSender sends push notifications via Apple Push Notification service
// using the token-based (JWT) HTTP/2 provider API.
type APNsSender struct {
	client  *http.Client
	baseURL string
	topic   string // APNs topic (app bundle ID)

	// JWT signing fields.
	key    *ecdsa.PrivateKey
	keyID  string
	teamID string

	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

// APNsConfig holds the configuration for creating an APNsSender.
type APNsConfig struct {
	// KeyFile is the path to the .p8 private key file from Apple.
	KeyFile string
	// KeyID is the 10-character key identifier from Apple.
	KeyID string
	// TeamID is the 10-character Apple Developer Team ID.
	TeamID string
	// BundleID is the app's bundle identifier, used as the APNs topic.
	BundleID string
	// Sandbox uses the APNs sandbox environment instead of production.
	Sandbox bool
}

// NewAPNsSender creates an APNsSender from the given configuration.
func NewAPNsSender(cfg APNsConfig) (*APNsSender, error) {
	if cfg.KeyFile == "" {
		return nil, fmt.Errorf("apns: key file path is required")
	}
	if cfg.KeyID == "" {
		return nil, fmt.Errorf("apns: key id is required")
	}
	if cfg.TeamID == "" {
		return nil, fmt.Errorf("apns: team id is required")
	}
	if cfg.BundleID == "" {
		return nil, fmt.Errorf("apns: bundle id is required")
	}

	keyData, err := os.ReadFile(cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("apns: reading key file: %w", err)
	}

	key, err := parseP8PrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("apns: parsing p8 key: %w", err)
	}

	baseURL := apnsProductionURL
	if cfg.Sandbox {
		baseURL = apnsSandboxURL
	}

	slog.Info("apns sender initialised", "key_id", cfg.KeyID, "team_id", cfg.TeamID, "topic", cfg.BundleID, "sandbox", cfg.Sandbox)

	return &APNsSender{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: baseURL,
		topic:   cfg.BundleID,
		key:     key,
		keyID:   cfg.KeyID,
		teamID:  cfg.TeamID,
	}, nil
}

// Send delivers a push notification to the given APNs device token.
// It only handles the "apns" platform; FCM tokens are rejected.
func (a *APNsSender) Send(platform, token string, payload PushPayload) error {
	if platform != "apns" {
		return fmt.Errorf("apns sender: unsupported platform %q", platform)
	}

	providerToken, err := a.getProviderToken()
	if err != nil {
		return fmt.Errorf("apns: generating provider token: %w", err)
	}

	body, err := buildAPNsPayload(payload)
	if err != nil {
		return fmt.Errorf("apns: building payload: %w", err)
	}

	url := fmt.Sprintf("%s/3/device/%s", a.baseURL, token)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("apns: creating request: %w", err)
	}

	req.Header.Set("Authorization", "bearer "+providerToken)
	req.Header.Set("apns-topic", a.topic+".voip")
	req.Header.Set("apns-push-type", "voip")
	req.Header.Set("apns-priority", "10")
	req.Header.Set("apns-expiration", "0")
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("apns: sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		apnsID := resp.Header.Get("apns-id")
		slog.Debug("apns notification sent", "apns_id", apnsID, "call_id", payload.CallID)
		return nil
	}

	// Read the error response body.
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	var apnsErr apnsErrorResponse
	if err := json.Unmarshal(respBody, &apnsErr); err == nil && apnsErr.Reason != "" {
		if apnsErr.Reason == "Unregistered" || apnsErr.Reason == "BadDeviceToken" || apnsErr.Reason == "ExpiredProviderToken" {
			return fmt.Errorf("apns: %s (status %d)", apnsErr.Reason, resp.StatusCode)
		}
		return fmt.Errorf("apns: %s (status %d)", apnsErr.Reason, resp.StatusCode)
	}

	return fmt.Errorf("apns: unexpected status %d: %s", resp.StatusCode, string(respBody))
}

// getProviderToken returns a cached JWT provider token, refreshing it
// when nearing expiry.
func (a *APNsSender) getProviderToken() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cachedToken != "" && time.Now().Before(a.tokenExpiry) {
		return a.cachedToken, nil
	}

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:   a.teamID,
		IssuedAt: jwt.NewNumericDate(now),
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tok.Header["kid"] = a.keyID

	signed, err := tok.SignedString(a.key)
	if err != nil {
		return "", fmt.Errorf("signing jwt: %w", err)
	}

	a.cachedToken = signed
	a.tokenExpiry = now.Add(apnsTokenRefreshInterval)

	return signed, nil
}

// apnsErrorResponse represents the JSON error body returned by APNs.
type apnsErrorResponse struct {
	Reason    string `json:"reason"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

// apnsVoIPPayload is the JSON payload sent to APNs for VoIP push notifications.
type apnsVoIPPayload struct {
	Type     string `json:"type"`
	CallID   string `json:"call_id"`
	CallerID string `json:"caller_id"`
}

// buildAPNsPayload creates the JSON body for an APNs VoIP push notification.
func buildAPNsPayload(p PushPayload) ([]byte, error) {
	payload := apnsVoIPPayload{
		Type:     p.Type,
		CallID:   p.CallID,
		CallerID: p.CallerID,
	}
	return json.Marshal(payload)
}

// parseP8PrivateKey parses an Apple .p8 private key file (PKCS#8 PEM-encoded
// ECDSA P-256 key) and returns the *ecdsa.PrivateKey.
func parseP8PrivateKey(pemData []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing PKCS8 key: %w", err)
	}

	ecKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not ECDSA")
	}

	return ecKey, nil
}
