package sip

import (
	"testing"
	"time"
)

func TestBackoff_ExponentialGrowth(t *testing.T) {
	b := newBackoff()

	// Collect delays without jitter influence by checking bounds.
	// Base delay is 5s, each attempt doubles: 5, 10, 20, 40, 80, 160, 300(max).
	expectedBase := []time.Duration{
		5 * time.Second,
		10 * time.Second,
		20 * time.Second,
		40 * time.Second,
		80 * time.Second,
		160 * time.Second,
		300 * time.Second, // capped at maxDelay
		300 * time.Second, // remains at max
	}

	for i, expected := range expectedBase {
		d := b.next()
		// Allow ±20% jitter tolerance.
		low := time.Duration(float64(expected) * 0.75)
		high := time.Duration(float64(expected) * 1.25)
		if d < low || d > high {
			t.Errorf("attempt %d: got %v, want %v ±20%% (range %v to %v)",
				i, d, expected, low, high)
		}
	}
}

func TestBackoff_Reset(t *testing.T) {
	b := newBackoff()

	// Advance a few attempts.
	for i := 0; i < 5; i++ {
		b.next()
	}

	b.reset()

	if b.attempt != 0 {
		t.Errorf("after reset: attempt = %d, want 0", b.attempt)
	}

	// Next delay should be near the base delay.
	d := b.next()
	low := time.Duration(float64(5*time.Second) * 0.75)
	high := time.Duration(float64(5*time.Second) * 1.25)
	if d < low || d > high {
		t.Errorf("after reset: got %v, want ~5s (range %v to %v)", d, low, high)
	}
}

func TestBackoff_MaxDelayCap(t *testing.T) {
	b := newBackoff()

	// Advance well past the cap.
	for i := 0; i < 20; i++ {
		b.next()
	}

	d := b.current()
	maxWithJitter := time.Duration(float64(5*time.Minute) * 1.25)
	if d > maxWithJitter {
		t.Errorf("delay %v exceeds max delay with jitter %v", d, maxWithJitter)
	}
}

func TestBackoff_JitterVariance(t *testing.T) {
	// Run multiple backoffs at the same attempt level and verify we get
	// different values (jitter is working).
	seen := make(map[time.Duration]bool)
	for i := 0; i < 20; i++ {
		b := newBackoff()
		d := b.next()
		seen[d] = true
	}

	// With jitter, we should see more than one unique value across 20 samples.
	if len(seen) < 2 {
		t.Errorf("expected jitter to produce varying delays, got %d unique values", len(seen))
	}
}

func TestParseContactExpires(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"<sip:user@host>;expires=3600", 3600},
		{"<sip:user@host>;Expires=120", 120},
		{"<sip:user@host>", 0},
		{"<sip:user@host>;expires=0", 0},
		{"<sip:user@host>;expires=60;q=0.5", 60},
		{"", 0},
	}

	for _, tt := range tests {
		got := parseContactExpires(tt.input)
		if got != tt.want {
			t.Errorf("parseContactExpires(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseExpiresHeader(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"3600", 3600},
		{" 120 ", 120},
		{"", 0},
		{"abc", 0},
	}

	for _, tt := range tests {
		got := parseExpiresHeader(tt.input)
		if got != tt.want {
			t.Errorf("parseExpiresHeader(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
