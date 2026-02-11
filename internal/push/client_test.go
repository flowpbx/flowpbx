package push

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSendPush_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/push" {
			t.Errorf("expected path /v1/push, got %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("X-License-Key") != "test-license" {
			t.Errorf("expected X-License-Key %q, got %q", "test-license", r.Header.Get("X-License-Key"))
		}

		// Decode and verify request body.
		var req PushRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.LicenseKey != "test-license" {
			t.Errorf("expected license_key %q, got %q", "test-license", req.LicenseKey)
		}
		if req.PushToken != "device-token" {
			t.Errorf("expected push_token %q, got %q", "device-token", req.PushToken)
		}
		if req.PushPlatform != "fcm" {
			t.Errorf("expected push_platform %q, got %q", "fcm", req.PushPlatform)
		}
		if req.CallerID != "+61400000000" {
			t.Errorf("expected caller_id %q, got %q", "+61400000000", req.CallerID)
		}
		if req.CallID != "call-123" {
			t.Errorf("expected call_id %q, got %q", "call-123", req.CallID)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(envelope{
			Data: json.RawMessage(`{"delivered":true,"call_id":"call-123"}`),
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-license")
	delivered, err := client.SendPush(context.Background(), "device-token", "fcm", "+61400000000", "call-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !delivered {
		t.Error("expected delivered=true")
	}
}

func TestSendPush_APNsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(envelope{
			Data: json.RawMessage(`{"delivered":true,"call_id":"call-apns"}`),
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "lic-key")
	delivered, err := client.SendPush(context.Background(), "apns-device-token", "apns", "100", "call-apns")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !delivered {
		t.Error("expected delivered=true")
	}
}

func TestSendPush_GatewayError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(envelope{Error: "invalid or expired license key"})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "bad-license")
	delivered, err := client.SendPush(context.Background(), "token", "fcm", "100", "call-1")
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if delivered {
		t.Error("expected delivered=false for error response")
	}
}

func TestSendPush_GatewayErrorWithMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(envelope{Error: "push delivery failed"})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "lic")
	_, err := client.SendPush(context.Background(), "token", "fcm", "100", "call-1")
	if err == nil {
		t.Fatal("expected error for 502 response")
	}
}

func TestSendPush_GatewayErrorNoBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "lic")
	_, err := client.SendPush(context.Background(), "token", "fcm", "100", "call-1")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestSendPush_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow gateway — sleep longer than context timeout.
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "lic")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.SendPush(ctx, "token", "fcm", "100", "call-timeout")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestSendPush_ConnectionRefused(t *testing.T) {
	// Use a URL that will refuse connections.
	client := NewClient("http://127.0.0.1:1", "lic")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.SendPush(ctx, "token", "fcm", "100", "call-refuse")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestSendPush_DeliveredFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(envelope{
			Data: json.RawMessage(`{"delivered":false,"call_id":"call-fail"}`),
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "lic")
	delivered, err := client.SendPush(context.Background(), "token", "fcm", "100", "call-fail")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if delivered {
		t.Error("expected delivered=false")
	}
}

func TestConfigured(t *testing.T) {
	tests := []struct {
		name       string
		baseURL    string
		licenseKey string
		want       bool
	}{
		{"both set", "https://push.example.com", "lic-key", true},
		{"missing url", "", "lic-key", false},
		{"missing key", "https://push.example.com", "", false},
		{"both empty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClient(tt.baseURL, tt.licenseKey)
			if c.Configured() != tt.want {
				t.Errorf("Configured() = %v, want %v", c.Configured(), tt.want)
			}
		})
	}
}

// TestSendPush_BackgroundedAppFlow simulates the complete push delivery flow
// for a backgrounded app: PBX sends push via client → gateway delivers → client
// gets success response, enabling the PBX to wait for the app to re-register.
func TestSendPush_BackgroundedAppFlow(t *testing.T) {
	var receivedReq PushRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(envelope{
			Data: json.RawMessage(`{"delivered":true,"call_id":"` + receivedReq.CallID + `"}`),
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "production-license")

	// Simulate push for multiple devices (Android FCM and iOS APNs).
	devices := []struct {
		token    string
		platform string
	}{
		{"fcm-token-pixel-8", "fcm"},
		{"apns-token-iphone-15", "apns"},
	}

	for _, dev := range devices {
		delivered, err := client.SendPush(
			context.Background(),
			dev.token,
			dev.platform,
			"+61400333444",
			"bg-call-multi-device",
		)
		if err != nil {
			t.Fatalf("push to %s device failed: %v", dev.platform, err)
		}
		if !delivered {
			t.Errorf("push to %s device: expected delivered=true", dev.platform)
		}
	}
}

// TestSendPush_KilledAppFlow simulates push delivery when app process is killed.
// The push is still delivered via FCM/APNs but the app may fail to register
// within the timeout. The gateway still returns success (delivery to platform
// succeeded), the PBX-side timeout logic handles the rest.
func TestSendPush_KilledAppFlow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(envelope{
			Data: json.RawMessage(`{"delivered":true,"call_id":"killed-call-001"}`),
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "lic-killed")
	delivered, err := client.SendPush(context.Background(), "apns-voip-token", "apns", "500", "killed-call-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !delivered {
		t.Error("expected delivered=true — gateway accepted push even though app is killed")
	}
}
