package pushgw

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiterConfig configures per-license-key rate limiting.
type RateLimiterConfig struct {
	// Rate is the number of requests allowed per second per license key.
	Rate rate.Limit
	// Burst is the maximum burst size per license key.
	Burst int
	// CleanupInterval is how often stale entries are removed.
	CleanupInterval time.Duration
	// MaxAge is how long an idle limiter is kept before eviction.
	MaxAge time.Duration
}

// DefaultRateLimiterConfig returns sensible defaults for push rate limiting:
// 60 requests/minute (1/sec) with burst of 10.
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		Rate:            rate.Limit(1), // 1 per second = 60 per minute
		Burst:           10,
		CleanupInterval: 5 * time.Minute,
		MaxAge:          10 * time.Minute,
	}
}

// rateLimitEntry tracks a per-key rate limiter and when it was last used.
type rateLimitEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter provides per-license-key rate limiting for the push gateway.
type RateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rateLimitEntry
	cfg     RateLimiterConfig
	stopCh  chan struct{}
}

// NewRateLimiter creates a rate limiter and starts background cleanup.
func NewRateLimiter(cfg RateLimiterConfig) *RateLimiter {
	rl := &RateLimiter{
		entries: make(map[string]*rateLimitEntry),
		cfg:     cfg,
		stopCh:  make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

// Allow checks whether a request for the given license key is allowed.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	entry, ok := rl.entries[key]
	if !ok {
		entry = &rateLimitEntry{
			limiter: rate.NewLimiter(rl.cfg.Rate, rl.cfg.Burst),
		}
		rl.entries[key] = entry
	}
	entry.lastSeen = time.Now()
	rl.mu.Unlock()

	return entry.limiter.Allow()
}

// Stop terminates the background cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// cleanupLoop periodically removes stale rate limiter entries.
func (rl *RateLimiter) cleanupLoop() {
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
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.cfg.MaxAge)
	removed := 0
	for key, entry := range rl.entries {
		if entry.lastSeen.Before(cutoff) {
			delete(rl.entries, key)
			removed++
		}
	}
	if removed > 0 {
		slog.Debug("rate limiter cleanup", "removed", removed, "remaining", len(rl.entries))
	}
}

// Middleware returns an HTTP middleware that rate limits based on the
// license key in the JSON request body. It extracts the license key by
// peeking at the X-License-Key header (preferred for cheap extraction)
// or falls through to per-IP limiting if no key is present.
//
// For the push gateway, the license key is the natural rate limit key
// since each PBX instance has its own license.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract license key from header for rate limiting.
		// The handlePush handler also reads it from the JSON body,
		// but for rate limiting we use the header to avoid consuming the body.
		key := r.Header.Get("X-License-Key")
		if key == "" {
			// Fall back to remote address if no license key header.
			key = "ip:" + r.RemoteAddr
		}

		if !rl.Allow(key) {
			slog.Warn("rate limit exceeded", "key_prefix", truncateKey(key))
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}

		next.ServeHTTP(w, r)
	})
}
