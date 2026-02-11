package pushgw

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// mockLicenseStore implements LicenseStore for testing.
type mockLicenseStore struct {
	license *License
	err     error
}

func (m *mockLicenseStore) ValidateLicense(key string) (*License, error) {
	return m.license, m.err
}

func (m *mockLicenseStore) ActivateLicense(key, hostname, version string) (*Installation, error) {
	return nil, nil
}

func (m *mockLicenseStore) GetLicenseStatus(key string) (*LicenseStatus, error) {
	return nil, nil
}

// mockPushSender implements PushSender for testing.
type mockPushSender struct {
	lastPlatform string
	lastToken    string
	lastPayload  PushPayload
	sendCount    int
	err          error
}

func (m *mockPushSender) Send(platform, token string, payload PushPayload) error {
	m.lastPlatform = platform
	m.lastToken = token
	m.lastPayload = payload
	m.sendCount++
	return m.err
}

// mockPushLogger implements PushLogger for testing.
type mockPushLogger struct {
	entries []PushLogEntry
}

func (m *mockPushLogger) Log(entry PushLogEntry) error {
	m.entries = append(m.entries, entry)
	return nil
}

func validLicense() *License {
	return &License{
		ID:            1,
		Key:           "test-license-key-12345678",
		Tier:          "standard",
		MaxExtensions: 50,
		CreatedAt:     time.Now(),
	}
}

func TestHandlePush_Success(t *testing.T) {
	store := &mockLicenseStore{license: validLicense()}
	sender := &mockPushSender{}
	logger := &mockPushLogger{}
	srv := NewServer(store, sender, logger, nil)

	body := `{"license_key":"test-key","push_token":"device-token-abc","push_platform":"fcm","caller_id":"+61400000000","call_id":"call-123"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/push", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify sender received correct parameters.
	if sender.lastPlatform != "fcm" {
		t.Errorf("expected platform %q, got %q", "fcm", sender.lastPlatform)
	}
	if sender.lastToken != "device-token-abc" {
		t.Errorf("expected token %q, got %q", "device-token-abc", sender.lastToken)
	}
	if sender.lastPayload.Type != "incoming_call" {
		t.Errorf("expected payload type %q, got %q", "incoming_call", sender.lastPayload.Type)
	}
	if sender.lastPayload.CallID != "call-123" {
		t.Errorf("expected call_id %q, got %q", "call-123", sender.lastPayload.CallID)
	}
	if sender.lastPayload.CallerID != "+61400000000" {
		t.Errorf("expected caller_id %q, got %q", "+61400000000", sender.lastPayload.CallerID)
	}

	// Verify response envelope.
	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	var resp PushResponse
	data, _ := json.Marshal(env.Data)
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to decode push response: %v", err)
	}
	if !resp.Delivered {
		t.Error("expected delivered=true")
	}
	if resp.CallID != "call-123" {
		t.Errorf("expected call_id %q, got %q", "call-123", resp.CallID)
	}

	// Verify push was logged.
	if len(logger.entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(logger.entries))
	}
	if !logger.entries[0].Success {
		t.Error("expected log entry success=true")
	}
}

func TestHandlePush_APNsPlatform(t *testing.T) {
	store := &mockLicenseStore{license: validLicense()}
	sender := &mockPushSender{}
	srv := NewServer(store, sender, nil, nil)

	body := `{"license_key":"test-key","push_token":"apns-device-token","push_platform":"apns","caller_id":"100","call_id":"call-456"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/push", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if sender.lastPlatform != "apns" {
		t.Errorf("expected platform %q, got %q", "apns", sender.lastPlatform)
	}
}

func TestHandlePush_InvalidLicense(t *testing.T) {
	store := &mockLicenseStore{license: nil} // nil = invalid license
	sender := &mockPushSender{}
	logger := &mockPushLogger{}
	srv := NewServer(store, sender, logger, nil)

	body := `{"license_key":"bad-key","push_token":"token","push_platform":"fcm","call_id":"call-1"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/push", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d: %s", w.Code, w.Body.String())
	}
	if sender.sendCount != 0 {
		t.Error("expected no push to be sent for invalid license")
	}
}

func TestHandlePush_LicenseStoreError(t *testing.T) {
	store := &mockLicenseStore{err: fmt.Errorf("database connection lost")}
	sender := &mockPushSender{}
	srv := NewServer(store, sender, nil, nil)

	body := `{"license_key":"test-key","push_token":"token","push_platform":"fcm","call_id":"call-1"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/push", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlePush_MissingFields(t *testing.T) {
	store := &mockLicenseStore{license: validLicense()}
	sender := &mockPushSender{}
	srv := NewServer(store, sender, nil, nil)

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "missing license_key",
			body: `{"push_token":"tok","push_platform":"fcm","call_id":"c1"}`,
			want: "license_key is required",
		},
		{
			name: "missing push_token",
			body: `{"license_key":"key","push_platform":"fcm","call_id":"c1"}`,
			want: "push_token is required",
		},
		{
			name: "missing call_id",
			body: `{"license_key":"key","push_token":"tok","push_platform":"fcm"}`,
			want: "call_id is required",
		},
		{
			name: "invalid platform",
			body: `{"license_key":"key","push_token":"tok","push_platform":"webpush","call_id":"c1"}`,
			want: "push_platform must be fcm or apns",
		},
		{
			name: "empty platform",
			body: `{"license_key":"key","push_token":"tok","push_platform":"","call_id":"c1"}`,
			want: "push_platform must be fcm or apns",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/push", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			srv.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
			}

			var env envelope
			if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			errMsg, _ := env.Data.(string)
			// Error is in the envelope Error field, not Data.
			if env.Error == "" {
				t.Fatal("expected error message in response")
			}
			if !strings.Contains(env.Error, tt.want) {
				t.Errorf("expected error containing %q, got %q (data=%v)", tt.want, env.Error, errMsg)
			}
		})
	}
}

func TestHandlePush_InvalidJSON(t *testing.T) {
	store := &mockLicenseStore{license: validLicense()}
	sender := &mockPushSender{}
	srv := NewServer(store, sender, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/push", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestHandlePush_SenderError(t *testing.T) {
	store := &mockLicenseStore{license: validLicense()}
	sender := &mockPushSender{err: fmt.Errorf("fcm: token no longer valid")}
	logger := &mockPushLogger{}
	srv := NewServer(store, sender, logger, nil)

	body := `{"license_key":"test-key","push_token":"expired-token","push_platform":"fcm","caller_id":"100","call_id":"call-789"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/push", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d: %s", w.Code, w.Body.String())
	}

	// Verify failed push was logged.
	if len(logger.entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(logger.entries))
	}
	if logger.entries[0].Success {
		t.Error("expected log entry success=false for failed send")
	}
	if logger.entries[0].Error == "" {
		t.Error("expected error message in log entry")
	}
}

func TestHandlePush_ServiceUnavailable(t *testing.T) {
	// Server with nil store and sender.
	srv := NewServer(nil, nil, nil, nil)

	body := `{"license_key":"key","push_token":"tok","push_platform":"fcm","call_id":"c1"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/push", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandlePush_BackgroundedApp simulates the scenario where a mobile app is
// backgrounded. The PBX detects no active SIP registration and sends a push
// notification via the gateway. The test verifies the gateway correctly
// validates the license, sends the push, and returns success so the PBX knows
// to wait for the app to re-register.
func TestHandlePush_BackgroundedApp(t *testing.T) {
	store := &mockLicenseStore{license: validLicense()}
	sender := &mockPushSender{}
	logger := &mockPushLogger{}
	srv := NewServer(store, sender, logger, nil)

	// Simulate the PBX sending a push for an incoming call to a backgrounded app.
	body := `{"license_key":"lic-001","push_token":"fcm-token-bg-app","push_platform":"fcm","caller_id":"+61400111222","call_id":"bg-call-001"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/push", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-License-Key", "lic-001")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("backgrounded app push: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the payload sent to the device contains the incoming_call type
	// and caller info needed for the app to show a notification/CallKit screen.
	if sender.lastPayload.Type != "incoming_call" {
		t.Errorf("expected payload type %q, got %q", "incoming_call", sender.lastPayload.Type)
	}
	if sender.lastPayload.CallerID != "+61400111222" {
		t.Errorf("expected caller_id %q, got %q", "+61400111222", sender.lastPayload.CallerID)
	}
	if sender.lastPayload.CallID != "bg-call-001" {
		t.Errorf("expected call_id %q, got %q", "bg-call-001", sender.lastPayload.CallID)
	}
}

// TestHandlePush_KilledApp simulates the scenario where the app has been
// force-killed by the user. The push notification is still sent via FCM/APNs
// which can wake the app process on the device. The gateway's responsibility
// is just delivery — the device OS handles process revival.
func TestHandlePush_KilledApp(t *testing.T) {
	store := &mockLicenseStore{license: validLicense()}
	sender := &mockPushSender{}
	logger := &mockPushLogger{}
	srv := NewServer(store, sender, logger, nil)

	body := `{"license_key":"lic-002","push_token":"apns-token-killed","push_platform":"apns","caller_id":"200","call_id":"killed-call-001"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/push", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("killed app push: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify APNs platform was used (VoIP push type can wake killed apps on iOS).
	if sender.lastPlatform != "apns" {
		t.Errorf("expected platform %q, got %q", "apns", sender.lastPlatform)
	}

	// Verify the log entry recorded success.
	if len(logger.entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(logger.entries))
	}
	if !logger.entries[0].Success {
		t.Error("expected successful push log entry")
	}
	if logger.entries[0].Platform != "apns" {
		t.Errorf("expected log platform %q, got %q", "apns", logger.entries[0].Platform)
	}
}

// TestHandlePush_DeviceLocked simulates push delivery when the device is
// locked. Same flow as backgrounded — the push is delivered via high-priority
// FCM or VoIP APNs which bypasses doze mode and presents on the lock screen
// via CallKit (iOS) or heads-up notification (Android).
func TestHandlePush_DeviceLocked(t *testing.T) {
	store := &mockLicenseStore{license: validLicense()}
	sender := &mockPushSender{}
	srv := NewServer(store, sender, nil, nil)

	// Test both platforms for lock screen delivery.
	platforms := []struct {
		name     string
		platform string
		token    string
	}{
		{"android_locked", "fcm", "fcm-token-locked-device"},
		{"ios_locked", "apns", "apns-token-locked-device"},
	}

	for _, p := range platforms {
		t.Run(p.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"license_key":"lic-003","push_token":"%s","push_platform":"%s","caller_id":"300","call_id":"locked-%s"}`,
				p.token, p.platform, p.name)
			req := httptest.NewRequest(http.MethodPost, "/v1/push", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			srv.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("locked device push (%s): expected 200, got %d: %s", p.platform, w.Code, w.Body.String())
			}
			if sender.lastPlatform != p.platform {
				t.Errorf("expected platform %q, got %q", p.platform, sender.lastPlatform)
			}
		})
	}
}

// TestHandlePush_ExpiredToken simulates the scenario where the push token has
// expired (device reinstalled, token rotated). The sender returns an error and
// the gateway should return 502 Bad Gateway so the PBX knows the push failed.
func TestHandlePush_ExpiredToken(t *testing.T) {
	store := &mockLicenseStore{license: validLicense()}
	sender := &mockPushSender{err: fmt.Errorf("fcm: token no longer valid: Unregistered")}
	logger := &mockPushLogger{}
	srv := NewServer(store, sender, logger, nil)

	body := `{"license_key":"lic-004","push_token":"expired-fcm-token","push_platform":"fcm","caller_id":"400","call_id":"expired-call-001"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/push", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expired token: expected 502, got %d: %s", w.Code, w.Body.String())
	}

	// Verify failure was logged.
	if len(logger.entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(logger.entries))
	}
	if logger.entries[0].Success {
		t.Error("expected log entry success=false")
	}
}

func TestTruncateKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"short", "short"},
		{"12345678", "12345678"},
		{"123456789", "12345678..."},
		{"abcdefghijklmnop", "abcdefgh..."},
	}

	for _, tt := range tests {
		got := truncateKey(tt.input)
		if got != tt.want {
			t.Errorf("truncateKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
