package media

import (
	"log/slog"
	"testing"
	"time"
)

func TestSessionActivity(t *testing.T) {
	session := &Session{
		ID:        "test-activity",
		CallID:    "call-activity",
		CreatedAt: time.Now(),
		state:     SessionStateNew,
	}

	// Before any activity, LastActivity should return CreatedAt.
	if got := session.LastActivity(); !got.Equal(session.CreatedAt) {
		t.Errorf("LastActivity() = %v, want CreatedAt %v", got, session.CreatedAt)
	}

	// Touch activity and verify it updates.
	time.Sleep(time.Millisecond)
	session.TouchActivity()

	last := session.LastActivity()
	if !last.After(session.CreatedAt) {
		t.Errorf("LastActivity() should be after CreatedAt after TouchActivity()")
	}

	// Touch again and verify it advances.
	time.Sleep(time.Millisecond)
	session.TouchActivity()

	newer := session.LastActivity()
	if !newer.After(last) {
		t.Errorf("LastActivity() should advance on subsequent TouchActivity()")
	}
}

func TestReaperCleansOrphanedSessions(t *testing.T) {
	logger := slog.Default()

	proxy, err := NewProxy(18000, 18100, logger)
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}

	mgr := NewSessionManager(proxy, logger)
	mgr.SetSessionTimeout(100 * time.Millisecond)

	// Allocate two sessions.
	s1, err := mgr.Allocate("session-1", "call-1")
	if err != nil {
		t.Fatalf("Allocate session-1: %v", err)
	}

	s2, err := mgr.Allocate("session-2", "call-2")
	if err != nil {
		t.Fatalf("Allocate session-2: %v", err)
	}

	if mgr.Count() != 2 {
		t.Fatalf("expected 2 sessions, got %d", mgr.Count())
	}

	// Touch session-2 to keep it alive.
	s2.TouchActivity()
	_ = s1 // s1 has no activity — will be reaped.

	// Wait for the timeout to expire for session-1 (whose last activity is CreatedAt).
	time.Sleep(150 * time.Millisecond)

	// Keep session-2 alive.
	s2.TouchActivity()

	// Run the reaper manually.
	mgr.reapOrphaned()

	// session-1 should have been reaped; session-2 should remain.
	if mgr.Count() != 1 {
		t.Fatalf("expected 1 session after reap, got %d", mgr.Count())
	}

	if mgr.Get("session-1") != nil {
		t.Error("session-1 should have been reaped")
	}
	if mgr.Get("session-2") == nil {
		t.Error("session-2 should still exist")
	}
}

func TestReaperStartStop(t *testing.T) {
	logger := slog.Default()

	proxy, err := NewProxy(18200, 18300, logger)
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}

	mgr := NewSessionManager(proxy, logger)
	mgr.SetSessionTimeout(50 * time.Millisecond)

	_, err = mgr.Allocate("session-orphan", "call-orphan")
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}

	if mgr.Count() != 1 {
		t.Fatalf("expected 1 session, got %d", mgr.Count())
	}

	// Start the reaper and let it run long enough to trigger at least one cycle.
	mgr.StartReaper()

	// Wait for timeout + reap interval to pass. The default reap interval
	// is 30s which is too slow for tests, so we call reapOrphaned directly
	// as a supplementary check and rely on the start/stop lifecycle working.
	time.Sleep(100 * time.Millisecond)
	mgr.reapOrphaned()

	mgr.StopReaper()

	if mgr.Count() != 0 {
		t.Errorf("expected 0 sessions after reaper, got %d", mgr.Count())
	}
}

func TestSessionStats(t *testing.T) {
	session := &Session{
		ID:        "test-stats",
		CallID:    "call-stats",
		CreatedAt: time.Now(),
		state:     SessionStateNew,
	}

	// Initial stats should be zero.
	stats := session.Stats()
	if stats.TotalPackets() != 0 {
		t.Errorf("initial TotalPackets() = %d, want 0", stats.TotalPackets())
	}
	if stats.TotalBytes() != 0 {
		t.Errorf("initial TotalBytes() = %d, want 0", stats.TotalBytes())
	}

	// Record some packets in each direction.
	session.RecordPacket("caller→callee", 160)
	session.RecordPacket("caller→callee", 160)
	session.RecordPacket("callee→caller", 320)
	session.RecordDrop()
	session.RecordDrop()

	stats = session.Stats()

	if stats.PacketsCallerToCallee != 2 {
		t.Errorf("PacketsCallerToCallee = %d, want 2", stats.PacketsCallerToCallee)
	}
	if stats.PacketsCalleeToCaller != 1 {
		t.Errorf("PacketsCalleeToCaller = %d, want 1", stats.PacketsCalleeToCaller)
	}
	if stats.BytesCallerToCallee != 320 {
		t.Errorf("BytesCallerToCallee = %d, want 320", stats.BytesCallerToCallee)
	}
	if stats.BytesCalleeToCaller != 320 {
		t.Errorf("BytesCalleeToCaller = %d, want 320", stats.BytesCalleeToCaller)
	}
	if stats.PacketsDropped != 2 {
		t.Errorf("PacketsDropped = %d, want 2", stats.PacketsDropped)
	}
	if stats.TotalPackets() != 3 {
		t.Errorf("TotalPackets() = %d, want 3", stats.TotalPackets())
	}
	if stats.TotalBytes() != 640 {
		t.Errorf("TotalBytes() = %d, want 640", stats.TotalBytes())
	}
}

func TestSessionStatsUnknownDirection(t *testing.T) {
	session := &Session{
		ID:        "test-stats-unknown",
		CallID:    "call-stats-unknown",
		CreatedAt: time.Now(),
		state:     SessionStateNew,
	}

	// Unknown direction should be a no-op (no panic, no increment).
	session.RecordPacket("unknown", 100)

	stats := session.Stats()
	if stats.TotalPackets() != 0 {
		t.Errorf("TotalPackets() = %d, want 0 after unknown direction", stats.TotalPackets())
	}
}

func TestStopReaperWithoutStart(t *testing.T) {
	logger := slog.Default()

	proxy, err := NewProxy(18400, 18500, logger)
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}

	mgr := NewSessionManager(proxy, logger)

	// StopReaper should be safe to call even if StartReaper was never called.
	mgr.StopReaper()
}
