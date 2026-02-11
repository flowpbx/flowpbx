package pushgw

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestRateLimiter_Allow(t *testing.T) {
	cfg := RateLimiterConfig{
		Rate:            rate.Limit(10), // 10 per second
		Burst:           2,
		CleanupInterval: time.Hour, // won't trigger during test
		MaxAge:          time.Hour,
	}

	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	// First two requests should be allowed (burst = 2).
	if !rl.Allow("key-1") {
		t.Error("expected first request to be allowed")
	}
	if !rl.Allow("key-1") {
		t.Error("expected second request to be allowed (within burst)")
	}

	// Third request immediately should be rejected (burst exhausted).
	if rl.Allow("key-1") {
		t.Error("expected third immediate request to be rejected")
	}
}

func TestRateLimiter_SeparateKeys(t *testing.T) {
	cfg := RateLimiterConfig{
		Rate:            rate.Limit(10),
		Burst:           1,
		CleanupInterval: time.Hour,
		MaxAge:          time.Hour,
	}

	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	// Each key has its own limiter — both first requests should pass.
	if !rl.Allow("key-a") {
		t.Error("expected key-a first request allowed")
	}
	if !rl.Allow("key-b") {
		t.Error("expected key-b first request allowed")
	}

	// Second requests should be rejected for both (burst=1).
	if rl.Allow("key-a") {
		t.Error("expected key-a second request rejected")
	}
	if rl.Allow("key-b") {
		t.Error("expected key-b second request rejected")
	}
}

func TestRateLimiter_Recovery(t *testing.T) {
	cfg := RateLimiterConfig{
		Rate:            rate.Limit(100), // 100/sec = 10ms per token
		Burst:           1,
		CleanupInterval: time.Hour,
		MaxAge:          time.Hour,
	}

	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	// Exhaust burst.
	rl.Allow("key-recover")

	// Wait for token replenishment.
	time.Sleep(20 * time.Millisecond)

	if !rl.Allow("key-recover") {
		t.Error("expected request to be allowed after token replenishment")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	cfg := RateLimiterConfig{
		Rate:            rate.Limit(1),
		Burst:           1,
		CleanupInterval: time.Hour, // won't auto-trigger
		MaxAge:          10 * time.Millisecond,
	}

	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	rl.Allow("stale-key")

	// Wait for entry to become stale.
	time.Sleep(20 * time.Millisecond)

	// Manually trigger cleanup.
	rl.cleanup()

	// Verify entry was removed by checking that a new request creates a fresh limiter.
	rl.mu.Lock()
	_, exists := rl.entries["stale-key"]
	rl.mu.Unlock()

	if exists {
		t.Error("expected stale entry to be cleaned up")
	}
}

func TestRateLimiter_Middleware(t *testing.T) {
	cfg := RateLimiterConfig{
		Rate:            rate.Limit(10),
		Burst:           1,
		CleanupInterval: time.Hour,
		MaxAge:          time.Hour,
	}

	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request with license key — should pass.
	req := httptest.NewRequest(http.MethodPost, "/v1/push", nil)
	req.Header.Set("X-License-Key", "test-lic")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Second request immediately — should be rate limited.
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
}

func TestRateLimiter_MiddlewareFallsBackToIP(t *testing.T) {
	cfg := RateLimiterConfig{
		Rate:            rate.Limit(10),
		Burst:           1,
		CleanupInterval: time.Hour,
		MaxAge:          time.Hour,
	}

	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request without license key header — falls back to IP.
	req := httptest.NewRequest(http.MethodPost, "/v1/push", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestMultiSender_RoutesCorrectly(t *testing.T) {
	fcm := &mockPushSender{}
	apns := &mockPushSender{}

	multi := NewMultiSender(map[string]PushSender{
		"fcm":  fcm,
		"apns": apns,
	})

	payload := PushPayload{CallID: "call-1", CallerID: "100", Type: "incoming_call"}

	// Send via FCM.
	if err := multi.Send("fcm", "fcm-token", payload); err != nil {
		t.Fatalf("fcm send failed: %v", err)
	}
	if fcm.sendCount != 1 {
		t.Error("expected fcm sender to be called")
	}
	if apns.sendCount != 0 {
		t.Error("expected apns sender to not be called")
	}

	// Send via APNs.
	if err := multi.Send("apns", "apns-token", payload); err != nil {
		t.Fatalf("apns send failed: %v", err)
	}
	if apns.sendCount != 1 {
		t.Error("expected apns sender to be called")
	}
}

func TestMultiSender_UnknownPlatform(t *testing.T) {
	multi := NewMultiSender(map[string]PushSender{})

	err := multi.Send("webpush", "token", PushPayload{})
	if err == nil {
		t.Fatal("expected error for unknown platform")
	}
}

func TestDefaultRateLimiterConfig(t *testing.T) {
	cfg := DefaultRateLimiterConfig()

	if cfg.Rate != rate.Limit(1) {
		t.Errorf("expected rate 1, got %v", cfg.Rate)
	}
	if cfg.Burst != 10 {
		t.Errorf("expected burst 10, got %d", cfg.Burst)
	}
	if cfg.CleanupInterval != 5*time.Minute {
		t.Errorf("expected cleanup interval 5m, got %v", cfg.CleanupInterval)
	}
	if cfg.MaxAge != 10*time.Minute {
		t.Errorf("expected max age 10m, got %v", cfg.MaxAge)
	}
}
