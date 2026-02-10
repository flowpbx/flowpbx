package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/flowpbx/flowpbx/internal/flow"
)

// timeRule represents a single time-based routing rule parsed from the
// time switch entity's Rules JSON field.
type timeRule struct {
	Label string   `json:"label"`
	Days  []string `json:"days"`  // e.g. ["mon","tue","wed","thu","fri"]
	Start string   `json:"start"` // "HH:MM" format
	End   string   `json:"end"`   // "HH:MM" format
}

// TimeSwitchHandler handles the Time Switch node type. It evaluates time-based
// rules against the current time in the configured timezone and follows the
// matching rule's output edge, or the "default" edge if no rule matches.
type TimeSwitchHandler struct {
	engine *flow.Engine
	logger *slog.Logger
	// nowFunc allows overriding the current time for testing.
	nowFunc func() time.Time
}

// NewTimeSwitchHandler creates a new TimeSwitchHandler.
func NewTimeSwitchHandler(engine *flow.Engine, logger *slog.Logger) *TimeSwitchHandler {
	return &TimeSwitchHandler{
		engine:  engine,
		logger:  logger.With("handler", "time_switch"),
		nowFunc: time.Now,
	}
}

// Execute resolves the time switch entity, evaluates rules against the current
// time in the configured timezone, and returns the matching rule label as the
// output edge name, or "default" if no rule matches.
func (h *TimeSwitchHandler) Execute(ctx context.Context, callCtx *flow.CallContext, node flow.Node) (string, error) {
	h.logger.Debug("time switch node executing",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
	)

	// Resolve the time switch entity.
	entity, err := h.engine.ResolveNodeEntity(ctx, node)
	if err != nil {
		return "", fmt.Errorf("resolving time switch entity: %w", err)
	}
	if entity == nil {
		return "", fmt.Errorf("time switch node %s: no entity reference configured", node.ID)
	}

	ts, ok := entity.(*models.TimeSwitch)
	if !ok {
		return "", fmt.Errorf("time switch node %s: entity is %T, expected *models.TimeSwitch", node.ID, entity)
	}

	// Parse the rules JSON.
	var rules []timeRule
	if err := json.Unmarshal([]byte(ts.Rules), &rules); err != nil {
		return "", fmt.Errorf("time switch node %s: parsing rules: %w", node.ID, err)
	}

	// Load the timezone.
	tz := ts.Timezone
	if tz == "" {
		tz = "Australia/Sydney"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return "", fmt.Errorf("time switch node %s: loading timezone %q: %w", node.ID, tz, err)
	}

	// Get current time in the configured timezone.
	now := h.nowFunc().In(loc)

	h.logger.Debug("evaluating time switch rules",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
		"timezone", tz,
		"local_time", now.Format("Mon 15:04"),
		"rule_count", len(rules),
	)

	// Evaluate rules top-to-bottom; first match wins.
	for _, rule := range rules {
		if matchesRule(now, rule) {
			h.logger.Info("time switch rule matched",
				"call_id", callCtx.CallID,
				"node_id", node.ID,
				"rule_label", rule.Label,
				"local_time", now.Format("Mon 15:04"),
			)
			return rule.Label, nil
		}
	}

	// No rule matched — follow the default edge.
	h.logger.Info("time switch no rule matched, using default",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
		"local_time", now.Format("Mon 15:04"),
	)

	return "default", nil
}

// matchesRule checks whether the given time matches a time rule.
// A rule matches if the current day of week is in the rule's days list
// and the current time is within the start–end range (inclusive of start,
// exclusive of end).
func matchesRule(now time.Time, rule timeRule) bool {
	// Check day of week.
	currentDay := strings.ToLower(now.Weekday().String()[:3])
	dayMatch := false
	for _, d := range rule.Days {
		if strings.ToLower(d) == currentDay {
			dayMatch = true
			break
		}
	}
	if !dayMatch {
		return false
	}

	// Parse start and end times.
	startH, startM, ok := parseHHMM(rule.Start)
	if !ok {
		return false
	}
	endH, endM, ok := parseHHMM(rule.End)
	if !ok {
		return false
	}

	// Convert to minutes since midnight for comparison.
	nowMinutes := now.Hour()*60 + now.Minute()
	startMinutes := startH*60 + startM
	endMinutes := endH*60 + endM

	// Handle overnight ranges (e.g. 22:00–06:00).
	if startMinutes > endMinutes {
		return nowMinutes >= startMinutes || nowMinutes < endMinutes
	}

	return nowMinutes >= startMinutes && nowMinutes < endMinutes
}

// parseHHMM parses a "HH:MM" time string into hours and minutes.
func parseHHMM(s string) (int, int, bool) {
	var h, m int
	n, err := fmt.Sscanf(s, "%d:%d", &h, &m)
	if err != nil || n != 2 {
		return 0, 0, false
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}

// Ensure TimeSwitchHandler satisfies the NodeHandler interface.
var _ flow.NodeHandler = (*TimeSwitchHandler)(nil)
