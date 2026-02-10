package media

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestDigitBuffer_InterDigitTimeout(t *testing.T) {
	ch := make(chan string, 32)
	buf := NewDigitBuffer(ch, slog.Default())
	buf.SetFirstDigitTimeout(2 * time.Second)
	buf.SetInterDigitTimeout(200 * time.Millisecond)

	// Send two digits, then let inter-digit timeout fire.
	go func() {
		ch <- "1"
		ch <- "2"
		// No more digits — inter-digit timeout should fire.
	}()

	result := buf.Collect(context.Background())
	if result.Digits != "12" {
		t.Errorf("Digits = %q, want %q", result.Digits, "12")
	}
	if !result.TimedOut {
		t.Error("expected TimedOut = true (inter-digit timeout)")
	}
}

func TestDigitBuffer_FirstDigitTimeout(t *testing.T) {
	ch := make(chan string, 32)
	buf := NewDigitBuffer(ch, slog.Default())
	buf.SetFirstDigitTimeout(100 * time.Millisecond)
	buf.SetInterDigitTimeout(2 * time.Second)

	// Send nothing — first-digit timeout should fire.
	result := buf.Collect(context.Background())
	if result.Digits != "" {
		t.Errorf("Digits = %q, want empty", result.Digits)
	}
	if !result.TimedOut {
		t.Error("expected TimedOut = true (first-digit timeout)")
	}
}

func TestDigitBuffer_ContextCancellation(t *testing.T) {
	ch := make(chan string, 32)
	buf := NewDigitBuffer(ch, slog.Default())
	buf.SetFirstDigitTimeout(5 * time.Second)
	buf.SetInterDigitTimeout(5 * time.Second)

	ctx, cancel := context.WithCancel(context.Background())

	// Send a digit, then cancel context.
	go func() {
		ch <- "5"
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result := buf.Collect(ctx)
	if result.Digits != "5" {
		t.Errorf("Digits = %q, want %q", result.Digits, "5")
	}
	if !result.TimedOut {
		t.Error("expected TimedOut = true (context cancelled)")
	}
}

func TestDigitBuffer_SourceClosed(t *testing.T) {
	ch := make(chan string, 32)
	buf := NewDigitBuffer(ch, slog.Default())
	buf.SetFirstDigitTimeout(5 * time.Second)
	buf.SetInterDigitTimeout(5 * time.Second)

	// Send digits then close the channel.
	go func() {
		ch <- "3"
		ch <- "4"
		close(ch)
	}()

	result := buf.Collect(context.Background())
	if result.Digits != "34" {
		t.Errorf("Digits = %q, want %q", result.Digits, "34")
	}
	if result.TimedOut {
		t.Error("expected TimedOut = false (source closed, not a timeout)")
	}
}

func TestDigitBuffer_MultipleDigitsWithDelay(t *testing.T) {
	ch := make(chan string, 32)
	buf := NewDigitBuffer(ch, slog.Default())
	buf.SetFirstDigitTimeout(2 * time.Second)
	buf.SetInterDigitTimeout(300 * time.Millisecond)

	// Send digits with delays shorter than inter-digit timeout.
	go func() {
		ch <- "1"
		time.Sleep(100 * time.Millisecond)
		ch <- "2"
		time.Sleep(100 * time.Millisecond)
		ch <- "3"
		// Stop sending — inter-digit timeout fires.
	}()

	result := buf.Collect(context.Background())
	if result.Digits != "123" {
		t.Errorf("Digits = %q, want %q", result.Digits, "123")
	}
	if !result.TimedOut {
		t.Error("expected TimedOut = true (inter-digit timeout)")
	}
}

func TestDigitBuffer_InterDigitTimeoutResetsOnEachDigit(t *testing.T) {
	ch := make(chan string, 32)
	buf := NewDigitBuffer(ch, slog.Default())
	buf.SetFirstDigitTimeout(2 * time.Second)
	buf.SetInterDigitTimeout(200 * time.Millisecond)

	// Send digits with delays just under the inter-digit timeout
	// to verify the timer resets on each digit.
	go func() {
		ch <- "7"
		time.Sleep(150 * time.Millisecond)
		ch <- "8"
		time.Sleep(150 * time.Millisecond)
		ch <- "9"
		// inter-digit timeout fires 200ms after "9"
	}()

	start := time.Now()
	result := buf.Collect(context.Background())
	elapsed := time.Since(start)

	if result.Digits != "789" {
		t.Errorf("Digits = %q, want %q", result.Digits, "789")
	}

	// Should have taken at least 150+150+200 = 500ms total,
	// but not much more. Allow generous upper bound for CI.
	if elapsed < 400*time.Millisecond {
		t.Errorf("completed too quickly (%v), inter-digit timer may not be resetting", elapsed)
	}
	if elapsed > 2*time.Second {
		t.Errorf("took too long (%v), inter-digit timer may not be resetting", elapsed)
	}
}

func TestDigitBuffer_SingleDigit(t *testing.T) {
	ch := make(chan string, 32)
	buf := NewDigitBuffer(ch, slog.Default())
	buf.SetFirstDigitTimeout(2 * time.Second)
	buf.SetInterDigitTimeout(200 * time.Millisecond)

	go func() {
		ch <- "0"
	}()

	result := buf.Collect(context.Background())
	if result.Digits != "0" {
		t.Errorf("Digits = %q, want %q", result.Digits, "0")
	}
	if !result.TimedOut {
		t.Error("expected TimedOut = true (inter-digit timeout after single digit)")
	}
}

func TestDigitBuffer_Reset(t *testing.T) {
	ch := make(chan string, 32)
	buf := NewDigitBuffer(ch, slog.Default())

	// Directly test Reset and Peek.
	ch <- "A"
	ch <- "B"

	buf.SetFirstDigitTimeout(2 * time.Second)
	buf.SetInterDigitTimeout(100 * time.Millisecond)

	result := buf.Collect(context.Background())
	if result.Digits != "AB" {
		t.Errorf("Digits = %q, want %q", result.Digits, "AB")
	}

	// After collection, Peek should show what was collected.
	if buf.Peek() != "AB" {
		t.Errorf("Peek = %q, want %q", buf.Peek(), "AB")
	}

	buf.Reset()
	if buf.Peek() != "" {
		t.Errorf("Peek after Reset = %q, want empty", buf.Peek())
	}
	if buf.Buffered() != 0 {
		t.Errorf("Buffered after Reset = %d, want 0", buf.Buffered())
	}
}

func TestDigitBuffer_SpecialCharacters(t *testing.T) {
	ch := make(chan string, 32)
	buf := NewDigitBuffer(ch, slog.Default())
	buf.SetFirstDigitTimeout(2 * time.Second)
	buf.SetInterDigitTimeout(200 * time.Millisecond)

	go func() {
		ch <- "*"
		ch <- "1"
		ch <- "#"
	}()

	result := buf.Collect(context.Background())
	if result.Digits != "*1#" {
		t.Errorf("Digits = %q, want %q", result.Digits, "*1#")
	}
}

func TestDigitBuffer_DefaultTimeouts(t *testing.T) {
	ch := make(chan string, 32)
	buf := NewDigitBuffer(ch, slog.Default())

	if buf.firstDigitTimeout != DefaultFirstDigitTimeout {
		t.Errorf("default firstDigitTimeout = %v, want %v", buf.firstDigitTimeout, DefaultFirstDigitTimeout)
	}
	if buf.interDigitTimeout != DefaultInterDigitTimeout {
		t.Errorf("default interDigitTimeout = %v, want %v", buf.interDigitTimeout, DefaultInterDigitTimeout)
	}
}
