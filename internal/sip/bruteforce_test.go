package sip

import (
	"fmt"
	"testing"
	"time"
)

func TestBruteForceGuard_NotBlockedInitially(t *testing.T) {
	g := NewBruteForceGuard(testLogger())

	if g.IsBlocked("192.168.1.1:5060") {
		t.Fatal("new IP should not be blocked")
	}
}

func TestBruteForceGuard_BlockAfterThreshold(t *testing.T) {
	g := NewBruteForceGuard(testLogger())
	source := "10.0.0.1:5060"

	// Record failures just below threshold — should not block.
	for i := 0; i < maxFailedAttempts-1; i++ {
		g.RecordFailure(source)
	}
	if g.IsBlocked(source) {
		t.Fatalf("should not be blocked after %d failures", maxFailedAttempts-1)
	}

	// One more failure should trigger the block.
	g.RecordFailure(source)
	if !g.IsBlocked(source) {
		t.Fatal("should be blocked after reaching threshold")
	}
}

func TestBruteForceGuard_DifferentIPsIndependent(t *testing.T) {
	g := NewBruteForceGuard(testLogger())

	// Block one IP.
	for i := 0; i < maxFailedAttempts; i++ {
		g.RecordFailure("10.0.0.1:5060")
	}

	if !g.IsBlocked("10.0.0.1:5060") {
		t.Fatal("10.0.0.1 should be blocked")
	}
	if g.IsBlocked("10.0.0.2:5060") {
		t.Fatal("10.0.0.2 should not be blocked")
	}
}

func TestBruteForceGuard_SuccessClearsFailures(t *testing.T) {
	g := NewBruteForceGuard(testLogger())
	source := "10.0.0.1:5060"

	// Record failures near threshold.
	for i := 0; i < maxFailedAttempts-1; i++ {
		g.RecordFailure(source)
	}

	// Successful auth should reset the counter.
	g.RecordSuccess(source)

	// Now another batch of failures below threshold should not block.
	for i := 0; i < maxFailedAttempts-1; i++ {
		g.RecordFailure(source)
	}
	if g.IsBlocked(source) {
		t.Fatal("should not be blocked after success reset the counter")
	}
}

func TestBruteForceGuard_BlockExpires(t *testing.T) {
	g := NewBruteForceGuard(testLogger())
	source := "10.0.0.1:5060"

	// Trigger block.
	for i := 0; i < maxFailedAttempts; i++ {
		g.RecordFailure(source)
	}
	if !g.IsBlocked(source) {
		t.Fatal("should be blocked")
	}

	// Manually expire the block by modifying the record.
	g.mu.Lock()
	ip := extractIP(source)
	rec := g.records[ip]
	rec.blockedAt = time.Now().Add(-rec.blockFor - time.Second)
	g.mu.Unlock()

	// Should no longer be blocked.
	if g.IsBlocked(source) {
		t.Fatal("block should have expired")
	}
}

func TestBruteForceGuard_ProgressiveBackoff(t *testing.T) {
	g := NewBruteForceGuard(testLogger())
	source := "10.0.0.1:5060"
	ip := extractIP(source)

	// First block.
	for i := 0; i < maxFailedAttempts; i++ {
		g.RecordFailure(source)
	}
	if !g.IsBlocked(source) {
		t.Fatal("should be blocked (first offence)")
	}

	g.mu.Lock()
	firstBlockFor := g.records[ip].blockFor
	g.mu.Unlock()

	// Expire the block.
	g.mu.Lock()
	g.records[ip].blockedAt = time.Now().Add(-g.records[ip].blockFor - time.Second)
	g.records[ip].blocked = false
	g.records[ip].failures = nil
	g.mu.Unlock()

	// Second block should have doubled duration.
	for i := 0; i < maxFailedAttempts; i++ {
		g.RecordFailure(source)
	}

	g.mu.Lock()
	secondBlockFor := g.records[ip].blockFor
	g.mu.Unlock()

	if secondBlockFor != firstBlockFor*2 {
		t.Errorf("expected progressive backoff: first=%v, second=%v, want second=%v",
			firstBlockFor, secondBlockFor, firstBlockFor*2)
	}
}

func TestBruteForceGuard_MaxBlockDurationCap(t *testing.T) {
	g := NewBruteForceGuard(testLogger())
	source := "10.0.0.1:5060"
	ip := extractIP(source)

	// Set block duration near the cap.
	g.mu.Lock()
	g.records[ip] = &ipRecord{blockFor: maxBlockDuration}
	g.mu.Unlock()

	// Trigger block.
	for i := 0; i < maxFailedAttempts; i++ {
		g.RecordFailure(source)
	}

	g.mu.Lock()
	dur := g.records[ip].blockFor
	g.mu.Unlock()

	if dur > maxBlockDuration {
		t.Errorf("block duration %v exceeds max %v", dur, maxBlockDuration)
	}
}

func TestBruteForceGuard_BlockedIPs(t *testing.T) {
	g := NewBruteForceGuard(testLogger())

	// Block two IPs.
	for _, ip := range []string{"10.0.0.1:5060", "10.0.0.2:5060"} {
		for i := 0; i < maxFailedAttempts; i++ {
			g.RecordFailure(ip)
		}
	}

	entries := g.BlockedIPs()
	if len(entries) != 2 {
		t.Fatalf("got %d blocked IPs, want 2", len(entries))
	}

	ips := make(map[string]bool)
	for _, e := range entries {
		ips[e.IP] = true
		if e.ExpiresAt.Before(e.BlockedAt) {
			t.Errorf("expires_at should be after blocked_at for %s", e.IP)
		}
	}
	if !ips["10.0.0.1"] || !ips["10.0.0.2"] {
		t.Errorf("missing expected IPs in blocked list: %v", entries)
	}
}

func TestBruteForceGuard_UnblockIP(t *testing.T) {
	g := NewBruteForceGuard(testLogger())
	source := "10.0.0.1:5060"

	// Block the IP.
	for i := 0; i < maxFailedAttempts; i++ {
		g.RecordFailure(source)
	}
	if !g.IsBlocked(source) {
		t.Fatal("should be blocked")
	}

	// Unblock it.
	if !g.UnblockIP("10.0.0.1") {
		t.Fatal("UnblockIP should return true for blocked IP")
	}
	if g.IsBlocked(source) {
		t.Fatal("should not be blocked after manual unblock")
	}

	// Unblocking a non-blocked IP returns false.
	if g.UnblockIP("10.0.0.1") {
		t.Fatal("UnblockIP should return false for non-blocked IP")
	}
	if g.UnblockIP("10.0.0.99") {
		t.Fatal("UnblockIP should return false for unknown IP")
	}
}

func TestBruteForceGuard_Cleanup(t *testing.T) {
	g := NewBruteForceGuard(testLogger())

	// Add a record with no failures and no block (should be cleaned up).
	g.mu.Lock()
	g.records["10.0.0.1"] = &ipRecord{blockFor: blockDuration}
	g.mu.Unlock()

	// Add a blocked record that is expired.
	g.mu.Lock()
	g.records["10.0.0.2"] = &ipRecord{
		blocked:   true,
		blockedAt: time.Now().Add(-blockDuration - time.Minute),
		blockFor:  blockDuration,
	}
	g.mu.Unlock()

	// Add an actively blocked record.
	g.mu.Lock()
	g.records["10.0.0.3"] = &ipRecord{
		blocked:   true,
		blockedAt: time.Now(),
		blockFor:  blockDuration,
	}
	g.mu.Unlock()

	g.Cleanup()

	g.mu.Lock()
	defer g.mu.Unlock()

	// Empty record should be cleaned.
	if _, ok := g.records["10.0.0.1"]; ok {
		t.Error("empty record should be cleaned up")
	}

	// Expired block should be cleaned (block cleared, then empty record removed).
	if _, ok := g.records["10.0.0.2"]; ok {
		t.Error("expired block record should be cleaned up")
	}

	// Active block should remain.
	if _, ok := g.records["10.0.0.3"]; !ok {
		t.Error("active block should not be cleaned up")
	}
}

func TestBruteForceGuard_BareIPAddress(t *testing.T) {
	g := NewBruteForceGuard(testLogger())

	// Test with bare IP (no port).
	for i := 0; i < maxFailedAttempts; i++ {
		g.RecordFailure("10.0.0.1")
	}
	if !g.IsBlocked("10.0.0.1") {
		t.Fatal("should be blocked with bare IP")
	}
	// Should also be blocked when checked with port.
	if !g.IsBlocked("10.0.0.1:5060") {
		t.Fatal("should be blocked when checked with port")
	}
}

func TestBruteForceGuard_EmptySource(t *testing.T) {
	g := NewBruteForceGuard(testLogger())

	// Empty source should not panic or block anything.
	g.RecordFailure("")
	g.RecordSuccess("")
	if g.IsBlocked("") {
		t.Fatal("empty source should not be blocked")
	}
}

func TestBruteForceGuard_IPv6(t *testing.T) {
	g := NewBruteForceGuard(testLogger())
	source := "[::1]:5060"

	for i := 0; i < maxFailedAttempts; i++ {
		g.RecordFailure(source)
	}
	if !g.IsBlocked(source) {
		t.Fatal("IPv6 address should be blocked")
	}
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "192.168.1.1:5060", want: "192.168.1.1"},
		{input: "10.0.0.1:1234", want: "10.0.0.1"},
		{input: "192.168.1.1", want: "192.168.1.1"},
		{input: "[::1]:5060", want: "::1"},
		{input: "::1", want: "::1"},
		{input: "", want: ""},
		{input: "not-an-ip", want: ""},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("input=%q", tt.input), func(t *testing.T) {
			got := extractIP(tt.input)
			if got != tt.want {
				t.Errorf("extractIP(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPruneOldFailures(t *testing.T) {
	now := time.Now()
	failures := []time.Time{
		now.Add(-20 * time.Minute), // outside window
		now.Add(-15 * time.Minute), // outside window
		now.Add(-5 * time.Minute),  // inside window
		now.Add(-1 * time.Minute),  // inside window
	}

	pruned := pruneOldFailures(failures, now, 10*time.Minute)
	if len(pruned) != 2 {
		t.Errorf("got %d failures, want 2", len(pruned))
	}
}
