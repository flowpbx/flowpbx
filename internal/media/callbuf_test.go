package media

import (
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestCallDTMFManager_AcquireAndInject(t *testing.T) {
	mgr := NewCallDTMFManager(slog.Default())

	ch := mgr.Acquire("call-1")
	if ch == nil {
		t.Fatal("Acquire returned nil channel")
	}
	if !mgr.Has("call-1") {
		t.Error("Has returned false for acquired call")
	}
	if mgr.ActiveCount() != 1 {
		t.Errorf("ActiveCount = %d, want 1", mgr.ActiveCount())
	}

	// Inject a digit and read it from the channel.
	mgr.Inject("call-1", "5")

	select {
	case digit := <-ch:
		if digit != "5" {
			t.Errorf("digit = %q, want %q", digit, "5")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for digit")
	}
}

func TestCallDTMFManager_AcquireReusesExisting(t *testing.T) {
	mgr := NewCallDTMFManager(slog.Default())

	ch1 := mgr.Acquire("call-1")
	ch2 := mgr.Acquire("call-1")

	// Both calls should return the same channel.
	mgr.Inject("call-1", "1")

	select {
	case digit := <-ch1:
		if digit != "1" {
			t.Errorf("digit from ch1 = %q, want %q", digit, "1")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out reading from ch1")
	}

	mgr.Inject("call-1", "2")

	select {
	case digit := <-ch2:
		if digit != "2" {
			t.Errorf("digit from ch2 = %q, want %q", digit, "2")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out reading from ch2")
	}

	if mgr.ActiveCount() != 1 {
		t.Errorf("ActiveCount = %d, want 1 (should not duplicate)", mgr.ActiveCount())
	}
}

func TestCallDTMFManager_Release(t *testing.T) {
	mgr := NewCallDTMFManager(slog.Default())

	mgr.Acquire("call-1")
	mgr.Release("call-1")

	if mgr.Has("call-1") {
		t.Error("Has returned true after Release")
	}
	if mgr.ActiveCount() != 0 {
		t.Errorf("ActiveCount = %d, want 0", mgr.ActiveCount())
	}

	// Double release should not panic.
	mgr.Release("call-1")
}

func TestCallDTMFManager_InjectDroppedWhenNoBuffer(t *testing.T) {
	mgr := NewCallDTMFManager(slog.Default())

	// Inject to a call with no buffer — should not panic.
	mgr.Inject("nonexistent", "1")
}

func TestCallDTMFManager_InjectDroppedWhenBufferFull(t *testing.T) {
	mgr := NewCallDTMFManager(slog.Default())

	mgr.Acquire("call-1")

	// Fill the buffer to capacity.
	for i := 0; i < callBufferSize; i++ {
		mgr.Inject("call-1", "0")
	}

	// Next inject should be dropped without blocking.
	done := make(chan struct{})
	go func() {
		mgr.Inject("call-1", "overflow")
		close(done)
	}()

	select {
	case <-done:
		// Inject returned without blocking — correct behavior.
	case <-time.After(time.Second):
		t.Fatal("Inject blocked when buffer was full")
	}
}

func TestCallDTMFManager_InjectAfterRelease(t *testing.T) {
	mgr := NewCallDTMFManager(slog.Default())

	ch := mgr.Acquire("call-1")
	mgr.Release("call-1")

	// Inject after release should be silently dropped.
	mgr.Inject("call-1", "1")

	// Channel should be empty.
	select {
	case digit := <-ch:
		t.Errorf("unexpected digit %q after release", digit)
	default:
		// Expected — no digits.
	}
}

func TestCallDTMFManager_MultipleCalls(t *testing.T) {
	mgr := NewCallDTMFManager(slog.Default())

	ch1 := mgr.Acquire("call-1")
	ch2 := mgr.Acquire("call-2")

	if mgr.ActiveCount() != 2 {
		t.Errorf("ActiveCount = %d, want 2", mgr.ActiveCount())
	}

	// Digits should be isolated between calls.
	mgr.Inject("call-1", "A")
	mgr.Inject("call-2", "B")

	select {
	case digit := <-ch1:
		if digit != "A" {
			t.Errorf("call-1 digit = %q, want %q", digit, "A")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for call-1 digit")
	}

	select {
	case digit := <-ch2:
		if digit != "B" {
			t.Errorf("call-2 digit = %q, want %q", digit, "B")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for call-2 digit")
	}
}

func TestCallDTMFManager_ConcurrentInject(t *testing.T) {
	mgr := NewCallDTMFManager(slog.Default())

	ch := mgr.Acquire("call-1")

	// Inject digits concurrently from multiple goroutines.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mgr.Inject("call-1", "1")
		}()
	}
	wg.Wait()

	// All 10 digits should be in the channel.
	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			goto done
		}
	}
done:
	if count != 10 {
		t.Errorf("received %d digits, want 10", count)
	}
}

func TestCallDTMFManager_Drain(t *testing.T) {
	mgr := NewCallDTMFManager(slog.Default())

	mgr.Acquire("call-1")
	mgr.Acquire("call-2")
	mgr.Acquire("call-3")

	mgr.Drain()

	if mgr.ActiveCount() != 0 {
		t.Errorf("ActiveCount after Drain = %d, want 0", mgr.ActiveCount())
	}
	if mgr.Has("call-1") || mgr.Has("call-2") || mgr.Has("call-3") {
		t.Error("Has returned true after Drain")
	}
}

func TestCallDTMFManager_IntegrationWithDigitBuffer(t *testing.T) {
	mgr := NewCallDTMFManager(slog.Default())

	ch := mgr.Acquire("call-1")
	buf := NewDigitBuffer(ch, slog.Default())
	buf.SetFirstDigitTimeout(2 * time.Second)
	buf.SetInterDigitTimeout(200 * time.Millisecond)
	buf.SetMaxDigits(4)

	// Simulate SIP INFO and RFC 2833 injecting digits.
	go func() {
		mgr.Inject("call-1", "1") // SIP INFO
		mgr.Inject("call-1", "2") // RFC 2833
		mgr.Inject("call-1", "3") // SIP INFO
		mgr.Inject("call-1", "4") // RFC 2833
	}()

	result := buf.Collect(t.Context())

	if result.Digits != "1234" {
		t.Errorf("Digits = %q, want %q", result.Digits, "1234")
	}
	if result.TimedOut {
		t.Error("expected TimedOut = false (max digits reached)")
	}

	mgr.Release("call-1")
	if mgr.Has("call-1") {
		t.Error("buffer still active after release")
	}
}
