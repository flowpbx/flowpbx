package sip

import (
	"log/slog"
	"net/netip"
	"os"
	"testing"

	"github.com/flowpbx/flowpbx/internal/database/models"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestParseRemoteHosts(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "empty", input: "", want: 0},
		{name: "single ip", input: `["192.168.1.1"]`, want: 1},
		{name: "single cidr", input: `["10.0.0.0/8"]`, want: 1},
		{name: "mixed", input: `["192.168.1.1","10.0.0.0/24","203.0.113.50"]`, want: 3},
		{name: "ipv6", input: `["::1","2001:db8::/32"]`, want: 2},
		{name: "invalid json", input: `not json`, wantErr: true},
		{name: "invalid ip", input: `["not-an-ip"]`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefixes, err := parseRemoteHosts(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(prefixes) != tt.want {
				t.Errorf("got %d prefixes, want %d", len(prefixes), tt.want)
			}
		})
	}
}

func TestParseCIDROrIP(t *testing.T) {
	tests := []struct {
		input   string
		want    netip.Prefix
		wantErr bool
	}{
		{input: "192.168.1.1", want: netip.MustParsePrefix("192.168.1.1/32")},
		{input: "10.0.0.0/8", want: netip.MustParsePrefix("10.0.0.0/8")},
		{input: "::1", want: netip.MustParsePrefix("::1/128")},
		{input: "2001:db8::/32", want: netip.MustParsePrefix("2001:db8::/32")},
		{input: "garbage", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseCIDROrIP(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseAddr(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "192.168.1.1", want: "192.168.1.1"},
		{input: "192.168.1.1:5060", want: "192.168.1.1"},
		{input: "::1", want: "::1"},
		{input: "[::1]:5060", want: "::1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseAddr(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tt.want {
				t.Errorf("got %s, want %s", got.String(), tt.want)
			}
		})
	}
}

func TestIPAuthMatcher_MatchIP(t *testing.T) {
	m := NewIPAuthMatcher(testLogger())

	// Add two trunks with different IPs and priorities.
	trunk1 := models.Trunk{
		ID:          1,
		Name:        "Provider A",
		Type:        "ip",
		Enabled:     true,
		RemoteHosts: `["203.0.113.0/24"]`,
		Priority:    10,
	}
	trunk2 := models.Trunk{
		ID:          2,
		Name:        "Provider B",
		Type:        "ip",
		Enabled:     true,
		RemoteHosts: `["198.51.100.10","198.51.100.20"]`,
		Priority:    20,
	}

	if err := m.AddTrunk(trunk1); err != nil {
		t.Fatalf("AddTrunk(1): %v", err)
	}
	if err := m.AddTrunk(trunk2); err != nil {
		t.Fatalf("AddTrunk(2): %v", err)
	}

	tests := []struct {
		name   string
		ip     string
		wantID int64
		wantOK bool
	}{
		{name: "match trunk1 cidr", ip: "203.0.113.50", wantID: 1, wantOK: true},
		{name: "match trunk1 first ip", ip: "203.0.113.1", wantID: 1, wantOK: true},
		{name: "match trunk2 exact", ip: "198.51.100.10", wantID: 2, wantOK: true},
		{name: "match trunk2 second ip", ip: "198.51.100.20", wantID: 2, wantOK: true},
		{name: "no match", ip: "10.0.0.1", wantID: 0, wantOK: false},
		{name: "with port", ip: "203.0.113.50:5060", wantID: 1, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := m.MatchIP(tt.ip)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if id != tt.wantID {
				t.Errorf("id = %d, want %d", id, tt.wantID)
			}
		})
	}
}

func TestIPAuthMatcher_PrioritySelection(t *testing.T) {
	m := NewIPAuthMatcher(testLogger())

	// Both trunks match the same IP range, but trunk2 has higher priority (lower number).
	trunk1 := models.Trunk{
		ID:          1,
		Name:        "Low Priority",
		Type:        "ip",
		RemoteHosts: `["10.0.0.0/8"]`,
		Priority:    20,
	}
	trunk2 := models.Trunk{
		ID:          2,
		Name:        "High Priority",
		Type:        "ip",
		RemoteHosts: `["10.0.0.0/8"]`,
		Priority:    5,
	}

	if err := m.AddTrunk(trunk1); err != nil {
		t.Fatal(err)
	}
	if err := m.AddTrunk(trunk2); err != nil {
		t.Fatal(err)
	}

	id, ok := m.MatchIP("10.1.2.3")
	if !ok {
		t.Fatal("expected match")
	}
	if id != 2 {
		t.Errorf("got trunk %d, want trunk 2 (higher priority)", id)
	}
}

func TestIPAuthMatcher_RemoveTrunk(t *testing.T) {
	m := NewIPAuthMatcher(testLogger())

	trunk := models.Trunk{
		ID:          1,
		Name:        "Removable",
		Type:        "ip",
		RemoteHosts: `["10.0.0.1"]`,
		Priority:    10,
	}

	if err := m.AddTrunk(trunk); err != nil {
		t.Fatal(err)
	}

	id, ok := m.MatchIP("10.0.0.1")
	if !ok || id != 1 {
		t.Fatal("expected match before removal")
	}

	m.RemoveTrunk(1)

	_, ok = m.MatchIP("10.0.0.1")
	if ok {
		t.Fatal("expected no match after removal")
	}

	if m.Count() != 0 {
		t.Errorf("count = %d, want 0", m.Count())
	}
}

func TestIPAuthMatcher_ReplaceTrunk(t *testing.T) {
	m := NewIPAuthMatcher(testLogger())

	trunk := models.Trunk{
		ID:          1,
		Name:        "Provider",
		Type:        "ip",
		RemoteHosts: `["10.0.0.1"]`,
		Priority:    10,
	}

	if err := m.AddTrunk(trunk); err != nil {
		t.Fatal(err)
	}

	// Update to different IP range.
	trunk.RemoteHosts = `["192.168.0.0/16"]`
	if err := m.AddTrunk(trunk); err != nil {
		t.Fatal(err)
	}

	// Old IP should no longer match.
	_, ok := m.MatchIP("10.0.0.1")
	if ok {
		t.Fatal("old IP should not match after replacement")
	}

	// New IP should match.
	id, ok := m.MatchIP("192.168.1.1")
	if !ok || id != 1 {
		t.Fatal("new IP should match after replacement")
	}

	if m.Count() != 1 {
		t.Errorf("count = %d, want 1 (should not duplicate)", m.Count())
	}
}

func TestIPAuthMatcher_AddTrunkErrors(t *testing.T) {
	m := NewIPAuthMatcher(testLogger())

	// Wrong type.
	err := m.AddTrunk(models.Trunk{ID: 1, Name: "Reg", Type: "register"})
	if err == nil {
		t.Fatal("expected error for register-type trunk")
	}

	// Empty remote hosts.
	err = m.AddTrunk(models.Trunk{ID: 2, Name: "Empty", Type: "ip", RemoteHosts: ""})
	if err == nil {
		t.Fatal("expected error for empty remote hosts")
	}

	// Invalid JSON.
	err = m.AddTrunk(models.Trunk{ID: 3, Name: "Bad", Type: "ip", RemoteHosts: "not-json"})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestIPAuthMatcher_MatchIPTrunk(t *testing.T) {
	m := NewIPAuthMatcher(testLogger())

	trunk := models.Trunk{
		ID:          42,
		Name:        "My Provider",
		Type:        "ip",
		RemoteHosts: `["10.0.0.1"]`,
		Priority:    10,
	}
	if err := m.AddTrunk(trunk); err != nil {
		t.Fatal(err)
	}

	id, name, ok := m.MatchIPTrunk("10.0.0.1")
	if !ok {
		t.Fatal("expected match")
	}
	if id != 42 {
		t.Errorf("id = %d, want 42", id)
	}
	if name != "My Provider" {
		t.Errorf("name = %q, want %q", name, "My Provider")
	}

	_, _, ok = m.MatchIPTrunk("10.0.0.2")
	if ok {
		t.Fatal("expected no match")
	}
}
