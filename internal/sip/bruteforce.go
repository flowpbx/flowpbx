package sip

import (
	"log/slog"
	"net"
	"sync"
	"time"
)

const (
	// maxFailedAttempts is the number of failed SIP auth attempts before an
	// IP address is blocked. Mirrors fail2ban's "maxretry" setting.
	maxFailedAttempts = 10

	// blockDuration is how long an IP remains blocked after exceeding the
	// failure threshold. Starts at this base value and doubles on repeat
	// offences (progressive backoff).
	blockDuration = 5 * time.Minute

	// maxBlockDuration caps the progressive backoff at 24 hours.
	maxBlockDuration = 24 * time.Hour

	// failureWindow is the sliding window in which failures are counted.
	// Failures older than this are forgotten automatically.
	failureWindow = 10 * time.Minute
)

// ipRecord tracks per-IP authentication failure state.
type ipRecord struct {
	failures  []time.Time   // timestamps of recent failures within the window
	blocked   bool          // whether the IP is currently blocked
	blockedAt time.Time     // when the block was applied
	blockFor  time.Duration // how long this block lasts (progressive)
}

// BruteForceGuard tracks failed SIP authentication attempts per source IP
// and automatically blocks IPs that exceed the failure threshold. It
// implements fail2ban-style progressive blocking:
//
//   - After maxFailedAttempts failures within failureWindow, the IP is blocked
//     for blockDuration.
//   - Repeated offences double the block duration up to maxBlockDuration.
//   - Blocks expire automatically and the failure counter resets.
type BruteForceGuard struct {
	mu      sync.Mutex
	records map[string]*ipRecord
	logger  *slog.Logger
}

// NewBruteForceGuard creates a new guard with empty state.
func NewBruteForceGuard(logger *slog.Logger) *BruteForceGuard {
	return &BruteForceGuard{
		records: make(map[string]*ipRecord),
		logger:  logger.With("subsystem", "bruteforce"),
	}
}

// IsBlocked returns true if the given source address is currently blocked.
// The source may be "ip:port" or just "ip".
func (g *BruteForceGuard) IsBlocked(source string) bool {
	ip := extractIP(source)
	if ip == "" {
		return false
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	rec, ok := g.records[ip]
	if !ok {
		return false
	}

	if !rec.blocked {
		return false
	}

	// Check if the block has expired.
	if time.Since(rec.blockedAt) > rec.blockFor {
		rec.blocked = false
		rec.failures = nil
		return false
	}

	return true
}

// RecordFailure records a failed authentication attempt from the given source.
// If the failure count exceeds the threshold, the IP is blocked automatically.
func (g *BruteForceGuard) RecordFailure(source string) {
	ip := extractIP(source)
	if ip == "" {
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	rec, ok := g.records[ip]
	if !ok {
		rec = &ipRecord{blockFor: blockDuration}
		g.records[ip] = rec
	}

	// If already blocked, nothing more to do.
	if rec.blocked {
		return
	}

	now := time.Now()

	// Prune failures outside the sliding window.
	rec.failures = pruneOldFailures(rec.failures, now, failureWindow)

	// Record this failure.
	rec.failures = append(rec.failures, now)

	if len(rec.failures) >= maxFailedAttempts {
		rec.blocked = true
		rec.blockedAt = now
		rec.failures = nil

		g.logger.Warn("ip blocked due to excessive failed sip auth attempts",
			"ip", ip,
			"block_duration", rec.blockFor.String(),
		)

		// Progressive backoff: double the block duration for next offence.
		nextBlock := rec.blockFor * 2
		if nextBlock > maxBlockDuration {
			nextBlock = maxBlockDuration
		}
		rec.blockFor = nextBlock
	}
}

// RecordSuccess clears the failure counter for a source IP on successful auth.
// The progressive block duration is preserved so repeat offenders still get
// longer blocks if they fail again.
func (g *BruteForceGuard) RecordSuccess(source string) {
	ip := extractIP(source)
	if ip == "" {
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	rec, ok := g.records[ip]
	if !ok {
		return
	}
	rec.failures = nil
}

// Cleanup removes expired blocks and stale records. Should be called
// periodically (e.g. alongside nonce cleanup).
func (g *BruteForceGuard) Cleanup() {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	for ip, rec := range g.records {
		if rec.blocked {
			if now.Sub(rec.blockedAt) > rec.blockFor {
				rec.blocked = false
				rec.failures = nil
			}
		}

		// Remove records that have no active block and no recent failures.
		if !rec.blocked && len(rec.failures) == 0 {
			delete(g.records, ip)
		}
	}
}

// BlockedIPs returns a snapshot of currently blocked IP addresses and when
// their block expires. Useful for admin visibility.
func (g *BruteForceGuard) BlockedIPs() []BlockedIPEntry {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	var entries []BlockedIPEntry
	for ip, rec := range g.records {
		if rec.blocked && now.Sub(rec.blockedAt) <= rec.blockFor {
			entries = append(entries, BlockedIPEntry{
				IP:        ip,
				BlockedAt: rec.blockedAt,
				ExpiresAt: rec.blockedAt.Add(rec.blockFor),
			})
		}
	}
	return entries
}

// UnblockIP manually removes a block for the given IP address. Returns true
// if the IP was found and unblocked.
func (g *BruteForceGuard) UnblockIP(ip string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	rec, ok := g.records[ip]
	if !ok {
		return false
	}

	if !rec.blocked {
		return false
	}

	rec.blocked = false
	rec.failures = nil
	g.logger.Info("ip manually unblocked", "ip", ip)
	return true
}

// BlockedIPEntry represents a single blocked IP for admin display.
type BlockedIPEntry struct {
	IP        string    `json:"ip"`
	BlockedAt time.Time `json:"blocked_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// extractIP parses the IP from a "host:port" string or returns the raw
// string if it's already an IP.
func extractIP(source string) string {
	if source == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(source)
	if err != nil {
		// Might already be a bare IP.
		if net.ParseIP(source) != nil {
			return source
		}
		return ""
	}
	return host
}

// pruneOldFailures returns only failures within the given window.
func pruneOldFailures(failures []time.Time, now time.Time, window time.Duration) []time.Time {
	cutoff := now.Add(-window)
	var pruned []time.Time
	for _, t := range failures {
		if t.After(cutoff) {
			pruned = append(pruned, t)
		}
	}
	return pruned
}
