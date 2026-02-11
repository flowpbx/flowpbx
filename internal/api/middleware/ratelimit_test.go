package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestIPRateLimiter_Allow(t *testing.T) {
	cfg := RateLimitConfig{
		Rate:            rate.Limit(2),
		Burst:           2,
		CleanupInterval: 1 * time.Hour,
		MaxAge:          1 * time.Hour,
	}
	rl := NewIPRateLimiter(cfg)
	defer rl.Stop()

	// First two requests should be allowed (burst = 2).
	if !rl.Allow("192.168.1.1") {
		t.Fatal("expected first request to be allowed")
	}
	if !rl.Allow("192.168.1.1") {
		t.Fatal("expected second request to be allowed")
	}

	// Third request should exceed burst.
	if rl.Allow("192.168.1.1") {
		t.Fatal("expected third request to be rate limited")
	}

	// Different IP should still be allowed.
	if !rl.Allow("192.168.1.2") {
		t.Fatal("expected request from different IP to be allowed")
	}
}

func TestIPRateLimiter_Cleanup(t *testing.T) {
	cfg := RateLimitConfig{
		Rate:            rate.Limit(10),
		Burst:           10,
		CleanupInterval: 1 * time.Hour,
		MaxAge:          0, // expire immediately
	}
	rl := NewIPRateLimiter(cfg)
	defer rl.Stop()

	rl.Allow("10.0.0.1")

	rl.mu.Lock()
	count := len(rl.entries)
	rl.mu.Unlock()

	if count != 1 {
		t.Fatalf("expected 1 entry, got %d", count)
	}

	// Run cleanup — entries should be evicted since MaxAge is 0.
	rl.cleanup()

	rl.mu.Lock()
	count = len(rl.entries)
	rl.mu.Unlock()

	if count != 0 {
		t.Fatalf("expected 0 entries after cleanup, got %d", count)
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	cfg := RateLimitConfig{
		Rate:            rate.Limit(1),
		Burst:           1,
		CleanupInterval: 1 * time.Hour,
		MaxAge:          1 * time.Hour,
	}
	rl := NewIPRateLimiter(cfg)
	defer rl.Stop()

	handler := RateLimit(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request should succeed.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/extensions", nil)
	req.RemoteAddr = "10.0.0.5:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Second request should be rate limited.
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}

	if rec.Header().Get("Retry-After") != "1" {
		t.Fatalf("expected Retry-After header, got %q", rec.Header().Get("Retry-After"))
	}
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		remoteAddr string
		want       string
	}{
		{"192.168.1.1:8080", "192.168.1.1"},
		{"[::1]:8080", "::1"},
		{"10.0.0.1", "10.0.0.1"}, // no port
	}

	for _, tt := range tests {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = tt.remoteAddr
		got := extractIP(r)
		if got != tt.want {
			t.Errorf("extractIP(%q) = %q, want %q", tt.remoteAddr, got, tt.want)
		}
	}
}
