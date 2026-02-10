package nodes

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/flowpbx/flowpbx/internal/flow"
)

// mockEntityResolver returns a fixed entity for any resolve call.
type mockEntityResolver struct {
	entity any
	err    error
}

func (m *mockEntityResolver) ResolveEntity(_ context.Context, _ string, _ int64) (any, error) {
	return m.entity, m.err
}

func newTestTimeSwitchHandler(ts *models.TimeSwitch, nowFunc func() time.Time) *TimeSwitchHandler {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	resolver := &mockEntityResolver{entity: ts}
	engine := flow.NewEngine(nil, nil, resolver, logger)

	h := NewTimeSwitchHandler(engine, logger)
	if nowFunc != nil {
		h.nowFunc = nowFunc
	}
	return h
}

func makeNode(entityID int64) flow.Node {
	id := entityID
	return flow.Node{
		ID:   "node_ts",
		Type: "time_switch",
		Data: flow.NodeData{
			Label:      "Business Hours",
			EntityID:   &id,
			EntityType: "time_switch",
		},
	}
}

func TestTimeSwitchBusinessHoursMatch(t *testing.T) {
	ts := &models.TimeSwitch{
		ID:       1,
		Name:     "Business Hours",
		Timezone: "Australia/Sydney",
		Rules:    `[{"label":"Business Hours","days":["mon","tue","wed","thu","fri"],"start":"08:30","end":"17:00"}]`,
	}

	// Wednesday at 10:00 Sydney time should match business hours.
	loc, _ := time.LoadLocation("Australia/Sydney")
	nowFunc := func() time.Time {
		return time.Date(2025, 3, 12, 10, 0, 0, 0, loc) // Wednesday
	}

	h := newTestTimeSwitchHandler(ts, nowFunc)
	callCtx := &flow.CallContext{CallID: "test-call-1"}

	edge, err := h.Execute(context.Background(), callCtx, makeNode(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edge != "Business Hours" {
		t.Errorf("expected edge %q, got %q", "Business Hours", edge)
	}
}

func TestTimeSwitchAfterHoursDefault(t *testing.T) {
	ts := &models.TimeSwitch{
		ID:       1,
		Name:     "Business Hours",
		Timezone: "Australia/Sydney",
		Rules:    `[{"label":"Business Hours","days":["mon","tue","wed","thu","fri"],"start":"08:30","end":"17:00"}]`,
	}

	// Wednesday at 20:00 Sydney time — no rule matches.
	loc, _ := time.LoadLocation("Australia/Sydney")
	nowFunc := func() time.Time {
		return time.Date(2025, 3, 12, 20, 0, 0, 0, loc) // Wednesday evening
	}

	h := newTestTimeSwitchHandler(ts, nowFunc)
	callCtx := &flow.CallContext{CallID: "test-call-2"}

	edge, err := h.Execute(context.Background(), callCtx, makeNode(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edge != "default" {
		t.Errorf("expected edge %q, got %q", "default", edge)
	}
}

func TestTimeSwitchWeekendDefault(t *testing.T) {
	ts := &models.TimeSwitch{
		ID:       1,
		Name:     "Business Hours",
		Timezone: "Australia/Sydney",
		Rules:    `[{"label":"Business Hours","days":["mon","tue","wed","thu","fri"],"start":"08:30","end":"17:00"}]`,
	}

	// Saturday at 10:00 — not in business days.
	loc, _ := time.LoadLocation("Australia/Sydney")
	nowFunc := func() time.Time {
		return time.Date(2025, 3, 15, 10, 0, 0, 0, loc) // Saturday
	}

	h := newTestTimeSwitchHandler(ts, nowFunc)
	callCtx := &flow.CallContext{CallID: "test-call-3"}

	edge, err := h.Execute(context.Background(), callCtx, makeNode(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edge != "default" {
		t.Errorf("expected edge %q, got %q", "default", edge)
	}
}

func TestTimeSwitchMultipleRulesFirstMatchWins(t *testing.T) {
	ts := &models.TimeSwitch{
		ID:       1,
		Name:     "Complex Schedule",
		Timezone: "Australia/Sydney",
		Rules: `[
			{"label":"Early Morning","days":["mon","tue","wed","thu","fri"],"start":"06:00","end":"08:30"},
			{"label":"Business Hours","days":["mon","tue","wed","thu","fri"],"start":"08:30","end":"17:00"},
			{"label":"Evening","days":["mon","tue","wed","thu","fri"],"start":"17:00","end":"21:00"}
		]`,
	}

	loc, _ := time.LoadLocation("Australia/Sydney")

	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "early morning",
			time:     time.Date(2025, 3, 12, 7, 0, 0, 0, loc), // Wed 07:00
			expected: "Early Morning",
		},
		{
			name:     "business hours",
			time:     time.Date(2025, 3, 12, 12, 0, 0, 0, loc), // Wed 12:00
			expected: "Business Hours",
		},
		{
			name:     "evening",
			time:     time.Date(2025, 3, 12, 18, 30, 0, 0, loc), // Wed 18:30
			expected: "Evening",
		},
		{
			name:     "night default",
			time:     time.Date(2025, 3, 12, 22, 0, 0, 0, loc), // Wed 22:00
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nowFunc := func() time.Time { return tt.time }
			h := newTestTimeSwitchHandler(ts, nowFunc)
			callCtx := &flow.CallContext{CallID: "test-call"}

			edge, err := h.Execute(context.Background(), callCtx, makeNode(1))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if edge != tt.expected {
				t.Errorf("expected edge %q, got %q", tt.expected, edge)
			}
		})
	}
}

func TestTimeSwitchOvernightRange(t *testing.T) {
	ts := &models.TimeSwitch{
		ID:       1,
		Name:     "Night Shift",
		Timezone: "Australia/Sydney",
		Rules:    `[{"label":"Night Shift","days":["mon","tue","wed","thu","fri"],"start":"22:00","end":"06:00"}]`,
	}

	loc, _ := time.LoadLocation("Australia/Sydney")

	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "before midnight matches",
			time:     time.Date(2025, 3, 12, 23, 0, 0, 0, loc), // Wed 23:00
			expected: "Night Shift",
		},
		{
			name:     "after midnight matches",
			time:     time.Date(2025, 3, 13, 3, 0, 0, 0, loc), // Thu 03:00
			expected: "Night Shift",
		},
		{
			name:     "daytime no match",
			time:     time.Date(2025, 3, 12, 12, 0, 0, 0, loc), // Wed 12:00
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nowFunc := func() time.Time { return tt.time }
			h := newTestTimeSwitchHandler(ts, nowFunc)
			callCtx := &flow.CallContext{CallID: "test-call"}

			edge, err := h.Execute(context.Background(), callCtx, makeNode(1))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if edge != tt.expected {
				t.Errorf("expected edge %q, got %q", tt.expected, edge)
			}
		})
	}
}

func TestTimeSwitchTimezoneConversion(t *testing.T) {
	ts := &models.TimeSwitch{
		ID:       1,
		Name:     "US Business Hours",
		Timezone: "America/New_York",
		Rules:    `[{"label":"Open","days":["mon","tue","wed","thu","fri"],"start":"09:00","end":"17:00"}]`,
	}

	// Create a time that's 10:00 in New York.
	nyLoc, _ := time.LoadLocation("America/New_York")
	nyTime := time.Date(2025, 3, 12, 10, 0, 0, 0, nyLoc) // Wed 10:00 ET

	// Pass the same instant but expressed in UTC — the handler should convert.
	utcTime := nyTime.UTC()

	nowFunc := func() time.Time { return utcTime }
	h := newTestTimeSwitchHandler(ts, nowFunc)
	callCtx := &flow.CallContext{CallID: "test-call-tz"}

	edge, err := h.Execute(context.Background(), callCtx, makeNode(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edge != "Open" {
		t.Errorf("expected edge %q, got %q", "Open", edge)
	}
}

func TestTimeSwitchDefaultTimezone(t *testing.T) {
	ts := &models.TimeSwitch{
		ID:       1,
		Name:     "No Timezone",
		Timezone: "", // Should default to Australia/Sydney.
		Rules:    `[{"label":"Open","days":["mon","tue","wed","thu","fri"],"start":"09:00","end":"17:00"}]`,
	}

	// Create time in Sydney timezone.
	sydLoc, _ := time.LoadLocation("Australia/Sydney")
	sydTime := time.Date(2025, 3, 12, 10, 0, 0, 0, sydLoc) // Wed 10:00 AEDT

	nowFunc := func() time.Time { return sydTime }
	h := newTestTimeSwitchHandler(ts, nowFunc)
	callCtx := &flow.CallContext{CallID: "test-call-deftz"}

	edge, err := h.Execute(context.Background(), callCtx, makeNode(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edge != "Open" {
		t.Errorf("expected edge %q, got %q", "Open", edge)
	}
}

func TestMatchesRule(t *testing.T) {
	sydLoc, _ := time.LoadLocation("Australia/Sydney")

	tests := []struct {
		name     string
		now      time.Time
		rule     timeRule
		expected bool
	}{
		{
			name:     "exact start time matches",
			now:      time.Date(2025, 3, 12, 8, 30, 0, 0, sydLoc), // Wed 08:30
			rule:     timeRule{Days: []string{"wed"}, Start: "08:30", End: "17:00"},
			expected: true,
		},
		{
			name:     "one minute before end does not match",
			now:      time.Date(2025, 3, 12, 16, 59, 0, 0, sydLoc), // Wed 16:59
			rule:     timeRule{Days: []string{"wed"}, Start: "08:30", End: "17:00"},
			expected: true,
		},
		{
			name:     "exact end time does not match",
			now:      time.Date(2025, 3, 12, 17, 0, 0, 0, sydLoc), // Wed 17:00
			rule:     timeRule{Days: []string{"wed"}, Start: "08:30", End: "17:00"},
			expected: false,
		},
		{
			name:     "wrong day does not match",
			now:      time.Date(2025, 3, 15, 10, 0, 0, 0, sydLoc), // Sat 10:00
			rule:     timeRule{Days: []string{"mon", "tue", "wed", "thu", "fri"}, Start: "08:30", End: "17:00"},
			expected: false,
		},
		{
			name:     "case insensitive days",
			now:      time.Date(2025, 3, 12, 10, 0, 0, 0, sydLoc), // Wed 10:00
			rule:     timeRule{Days: []string{"Mon", "TUE", "Wed"}, Start: "08:30", End: "17:00"},
			expected: true,
		},
		{
			name:     "invalid start time",
			now:      time.Date(2025, 3, 12, 10, 0, 0, 0, sydLoc),
			rule:     timeRule{Days: []string{"wed"}, Start: "invalid", End: "17:00"},
			expected: false,
		},
		{
			name:     "invalid end time",
			now:      time.Date(2025, 3, 12, 10, 0, 0, 0, sydLoc),
			rule:     timeRule{Days: []string{"wed"}, Start: "08:30", End: "invalid"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesRule(tt.now, tt.rule)
			if result != tt.expected {
				t.Errorf("matchesRule() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestParseHHMM(t *testing.T) {
	tests := []struct {
		input  string
		h, m   int
		wantOk bool
	}{
		{"08:30", 8, 30, true},
		{"00:00", 0, 0, true},
		{"23:59", 23, 59, true},
		{"17:00", 17, 0, true},
		{"24:00", 0, 0, false},
		{"12:60", 0, 0, false},
		{"-1:00", 0, 0, false},
		{"invalid", 0, 0, false},
		{"", 0, 0, false},
		{"8:30", 8, 30, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			h, m, ok := parseHHMM(tt.input)
			if ok != tt.wantOk {
				t.Errorf("parseHHMM(%q) ok = %v, want %v", tt.input, ok, tt.wantOk)
				return
			}
			if ok && (h != tt.h || m != tt.m) {
				t.Errorf("parseHHMM(%q) = (%d, %d), want (%d, %d)", tt.input, h, m, tt.h, tt.m)
			}
		})
	}
}
