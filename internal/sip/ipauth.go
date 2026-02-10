package sip

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"sync"

	"github.com/flowpbx/flowpbx/internal/database/models"
)

// IPAuthMatcher maintains a set of IP-auth trunks and provides efficient
// matching of source IP addresses against their remote_hosts ACLs. This
// is used to identify which trunk an inbound SIP request belongs to when
// the trunk uses IP-based authentication (no SIP registration required).
type IPAuthMatcher struct {
	mu      sync.RWMutex
	entries []ipAuthEntry
	logger  *slog.Logger
}

// ipAuthEntry holds a parsed ACL for a single IP-auth trunk.
type ipAuthEntry struct {
	trunkID  int64
	name     string
	priority int
	prefixes []netip.Prefix
}

// NewIPAuthMatcher creates an IP-auth matcher.
func NewIPAuthMatcher(logger *slog.Logger) *IPAuthMatcher {
	return &IPAuthMatcher{
		logger: logger.With("subsystem", "ip-auth"),
	}
}

// AddTrunk parses the trunk's remote_hosts JSON and adds it to the matcher.
// The remote_hosts field is a JSON array of IP addresses and/or CIDR ranges,
// e.g. ["203.0.113.10", "198.51.100.0/24"].
func (m *IPAuthMatcher) AddTrunk(trunk models.Trunk) error {
	if trunk.Type != "ip" {
		return fmt.Errorf("trunk %q is type %q, not ip", trunk.Name, trunk.Type)
	}

	prefixes, err := parseRemoteHosts(trunk.RemoteHosts)
	if err != nil {
		return fmt.Errorf("parsing remote_hosts for trunk %q: %w", trunk.Name, err)
	}

	if len(prefixes) == 0 {
		return fmt.Errorf("trunk %q has no remote hosts configured", trunk.Name)
	}

	entry := ipAuthEntry{
		trunkID:  trunk.ID,
		name:     trunk.Name,
		priority: trunk.Priority,
		prefixes: prefixes,
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Replace existing entry for the same trunk ID if present.
	replaced := false
	for i, e := range m.entries {
		if e.trunkID == trunk.ID {
			m.entries[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		m.entries = append(m.entries, entry)
	}

	m.logger.Info("ip-auth trunk added",
		"trunk", trunk.Name,
		"trunk_id", trunk.ID,
		"prefixes", len(prefixes),
	)

	return nil
}

// RemoveTrunk removes a trunk from the matcher.
func (m *IPAuthMatcher) RemoveTrunk(trunkID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, e := range m.entries {
		if e.trunkID == trunkID {
			m.entries = append(m.entries[:i], m.entries[i+1:]...)
			m.logger.Info("ip-auth trunk removed", "trunk_id", trunkID, "trunk", e.name)
			return
		}
	}
}

// MatchIP checks whether the given IP address matches any IP-auth trunk's
// ACL. If multiple trunks match, the one with the lowest priority value
// (highest priority) is returned. Returns the trunk ID and true if matched,
// or 0 and false if no trunk matches.
func (m *IPAuthMatcher) MatchIP(ipStr string) (int64, bool) {
	addr, err := parseAddr(ipStr)
	if err != nil {
		m.logger.Warn("failed to parse source ip for acl match", "ip", ipStr, "error", err)
		return 0, false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var bestID int64
	bestPriority := int(^uint(0) >> 1) // max int
	found := false

	for _, entry := range m.entries {
		for _, prefix := range entry.prefixes {
			if prefix.Contains(addr) {
				if !found || entry.priority < bestPriority {
					bestID = entry.trunkID
					bestPriority = entry.priority
					found = true
				}
				break // no need to check other prefixes for this trunk
			}
		}
	}

	return bestID, found
}

// MatchIPTrunk is like MatchIP but returns the full trunk name alongside the ID.
func (m *IPAuthMatcher) MatchIPTrunk(ipStr string) (trunkID int64, name string, ok bool) {
	addr, err := parseAddr(ipStr)
	if err != nil {
		m.logger.Warn("failed to parse source ip for acl match", "ip", ipStr, "error", err)
		return 0, "", false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	bestPriority := int(^uint(0) >> 1)
	found := false

	for _, entry := range m.entries {
		for _, prefix := range entry.prefixes {
			if prefix.Contains(addr) {
				if !found || entry.priority < bestPriority {
					trunkID = entry.trunkID
					name = entry.name
					bestPriority = entry.priority
					found = true
				}
				break
			}
		}
	}

	return trunkID, name, found
}

// Count returns the number of IP-auth trunks currently loaded.
func (m *IPAuthMatcher) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entries)
}

// parseRemoteHosts parses the JSON remote_hosts field into a slice of
// netip.Prefix values. Supports both plain IP addresses (treated as /32
// or /128) and CIDR notation.
func parseRemoteHosts(remoteHosts string) ([]netip.Prefix, error) {
	if remoteHosts == "" {
		return nil, nil
	}

	var hosts []string
	if err := json.Unmarshal([]byte(remoteHosts), &hosts); err != nil {
		return nil, fmt.Errorf("unmarshalling remote_hosts json: %w", err)
	}

	prefixes := make([]netip.Prefix, 0, len(hosts))
	for _, h := range hosts {
		prefix, err := parseCIDROrIP(h)
		if err != nil {
			return nil, fmt.Errorf("invalid remote host %q: %w", h, err)
		}
		prefixes = append(prefixes, prefix)
	}

	return prefixes, nil
}

// parseCIDROrIP parses a string as either a CIDR prefix or a single IP address.
// Single IPs are converted to /32 (IPv4) or /128 (IPv6) prefixes.
func parseCIDROrIP(s string) (netip.Prefix, error) {
	// Try CIDR first.
	prefix, err := netip.ParsePrefix(s)
	if err == nil {
		return prefix, nil
	}

	// Try as a plain IP address.
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("not a valid ip or cidr: %s", s)
	}

	return netip.PrefixFrom(addr, addr.BitLen()), nil
}

// parseAddr parses an IP string that may include a port (e.g. "192.168.1.1:5060")
// and returns just the address portion.
func parseAddr(ipStr string) (netip.Addr, error) {
	// Try parsing as addr:port first.
	if host, _, err := net.SplitHostPort(ipStr); err == nil {
		return netip.ParseAddr(host)
	}
	// Try as plain address.
	return netip.ParseAddr(ipStr)
}
