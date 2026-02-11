package middleware

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimitConfig configures per-IP rate limiting for API endpoints.
type RateLimitConfig struct {
	// Rate is the number of requests allowed per second per IP.
	Rate rate.Limit
	// Burst is the maximum burst size per IP.
	Burst int
	// CleanupInterval is how often stale entries are removed.
	CleanupInterval time.Duration
	// MaxAge is how long an idle limiter is kept before eviction.
	MaxAge time.Duration
}

// DefaultRateLimitConfig returns sensible defaults for general API rate limiting:
// 20 requests/second with burst of 40.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Rate:            rate.Limit(20),
		Burst:           40,
		CleanupInterval: 5 * time.Minute,
		MaxAge:          10 * time.Minute,
	}
}

// AuthRateLimitConfig returns stricter limits for authentication endpoints:
// 5 requests/second with burst of 10 to mitigate brute-force attacks.
func AuthRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Rate:            rate.Limit(5),
		Burst:           10,
		CleanupInterval: 5 * time.Minute,
		MaxAge:          10 * time.Minute,
	}
}

// ipLimitEntry tracks a per-IP rate limiter and when it was last used.
type ipLimitEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// IPRateLimiter provides per-IP rate limiting for HTTP endpoints.
type IPRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*ipLimitEntry
	cfg     RateLimitConfig
	stopCh  chan struct{}
}

// NewIPRateLimiter creates a per-IP rate limiter and starts background cleanup.
func NewIPRateLimiter(cfg RateLimitConfig) *IPRateLimiter {
	rl := &IPRateLimiter{
		entries: make(map[string]*ipLimitEntry),
		cfg:     cfg,
		stopCh:  make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

// Allow checks whether a request from the given IP is allowed.
func (rl *IPRateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	entry, ok := rl.entries[ip]
	if !ok {
		entry = &ipLimitEntry{
			limiter: rate.NewLimiter(rl.cfg.Rate, rl.cfg.Burst),
		}
		rl.entries[ip] = entry
	}
	entry.lastSeen = time.Now()
	rl.mu.Unlock()

	return entry.limiter.Allow()
}

// Stop terminates the background cleanup goroutine.
func (rl *IPRateLimiter) Stop() {
	close(rl.stopCh)
}

// cleanupLoop periodically removes stale rate limiter entries.
func (rl *IPRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cfg.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stopCh:
			return
		}
	}
}

// cleanup removes entries that haven't been seen within MaxAge.
func (rl *IPRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.cfg.MaxAge)
	removed := 0
	for ip, entry := range rl.entries {
		if entry.lastSeen.Before(cutoff) {
			delete(rl.entries, ip)
			removed++
		}
	}
	if removed > 0 {
		slog.Debug("api rate limiter cleanup", "removed", removed, "remaining", len(rl.entries))
	}
}

// RateLimit returns HTTP middleware that rate limits requests by client IP.
// When the limit is exceeded, it returns 429 Too Many Requests with a
// Retry-After header.
func RateLimit(limiter *IPRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractIP(r)

			if !limiter.Allow(ip) {
				slog.Warn("rate limit exceeded",
					"ip", ip,
					"method", r.Method,
					"path", r.URL.Path,
				)
				w.Header().Set("Retry-After", "1")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(authEnvelope{Error: "rate limit exceeded"}) //nolint:errcheck
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// extractIP returns the client IP address from the request. It uses
// RemoteAddr and strips the port. The chi RealIP middleware should run
// before this to set RemoteAddr from X-Forwarded-For / X-Real-IP if
// the server is behind a reverse proxy.
func extractIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
