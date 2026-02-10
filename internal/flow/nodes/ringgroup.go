package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/flowpbx/flowpbx/internal/database"
	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/flowpbx/flowpbx/internal/flow"
)

// RingGroupHandler handles the Ring Group node type. It resolves the ring
// group entity, loads all member extensions, and rings them using the
// configured strategy (ring_all, round_robin, etc.). The handler follows
// either the "answered" or "no_answer" output edge based on the result.
type RingGroupHandler struct {
	engine     *flow.Engine
	sip        flow.SIPActions
	extensions database.ExtensionRepository
	logger     *slog.Logger

	// rrCounters tracks round-robin state per ring group ID. Each entry
	// is an *atomic.Uint64 counter that advances on each call. The
	// starting member index is counter % len(members). State resets on
	// process restart, which is standard PBX behavior.
	rrCounters sync.Map
}

// NewRingGroupHandler creates a new RingGroupHandler.
func NewRingGroupHandler(engine *flow.Engine, sip flow.SIPActions, extensions database.ExtensionRepository, logger *slog.Logger) *RingGroupHandler {
	return &RingGroupHandler{
		engine:     engine,
		sip:        sip,
		extensions: extensions,
		logger:     logger.With("handler", "ring_group"),
	}
}

// Execute resolves the ring group entity, loads member extensions, applies
// the configured ring strategy, and returns "answered" or "no_answer".
func (h *RingGroupHandler) Execute(ctx context.Context, callCtx *flow.CallContext, node flow.Node) (string, error) {
	h.logger.Debug("ring group node executing",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
	)

	// Resolve the ring group entity from the node's entity_id.
	entity, err := h.engine.ResolveNodeEntity(ctx, node)
	if err != nil {
		return "", fmt.Errorf("resolving ring group entity: %w", err)
	}
	if entity == nil {
		return "", fmt.Errorf("ring group node %s: no entity reference configured", node.ID)
	}

	rg, ok := entity.(*models.RingGroup)
	if !ok {
		return "", fmt.Errorf("ring group node %s: entity is %T, expected *models.RingGroup", node.ID, entity)
	}

	// Parse member extension IDs from JSON.
	var memberIDs []int64
	if err := json.Unmarshal([]byte(rg.Members), &memberIDs); err != nil {
		return "", fmt.Errorf("ring group %s: parsing members json: %w", rg.Name, err)
	}

	if len(memberIDs) == 0 {
		h.logger.Warn("ring group has no members",
			"call_id", callCtx.CallID,
			"node_id", node.ID,
			"ring_group", rg.Name,
		)
		return "no_answer", nil
	}

	// Load all member extensions.
	members := make([]*models.Extension, 0, len(memberIDs))
	for _, extID := range memberIDs {
		ext, err := h.extensions.GetByID(ctx, extID)
		if err != nil {
			h.logger.Error("failed to load ring group member extension",
				"call_id", callCtx.CallID,
				"ring_group", rg.Name,
				"extension_id", extID,
				"error", err,
			)
			continue
		}
		if ext == nil {
			h.logger.Warn("ring group member extension not found",
				"call_id", callCtx.CallID,
				"ring_group", rg.Name,
				"extension_id", extID,
			)
			continue
		}
		members = append(members, ext)
	}

	if len(members) == 0 {
		h.logger.Warn("ring group has no valid member extensions",
			"call_id", callCtx.CallID,
			"node_id", node.ID,
			"ring_group", rg.Name,
		)
		return "no_answer", nil
	}

	// Determine ring timeout. Use the node config if specified, otherwise
	// fall back to the ring group's ring_timeout setting, defaulting to 30s.
	ringTimeout := rg.RingTimeout
	if ringTimeout <= 0 {
		ringTimeout = 30
	}
	if v, ok := node.Data.Config["ring_timeout"]; ok {
		switch t := v.(type) {
		case float64:
			if t > 0 {
				ringTimeout = int(t)
			}
		case int:
			if t > 0 {
				ringTimeout = t
			}
		}
	}

	h.logger.Info("ringing group",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
		"ring_group", rg.Name,
		"strategy", rg.Strategy,
		"members", len(members),
		"ring_timeout", ringTimeout,
	)

	// Apply caller ID mode before ringing.
	h.applyCallerIDMode(callCtx, rg)

	// Dispatch to the appropriate strategy.
	switch rg.Strategy {
	case "round_robin":
		return h.executeRoundRobin(ctx, callCtx, node, rg, members, ringTimeout)
	default:
		// ring_all is the default strategy (also used for unrecognized values).
		return h.executeRingAll(ctx, callCtx, node, rg, members, ringTimeout)
	}
}

// executeRingAll rings all members simultaneously. The first device to answer
// wins; all other forks are cancelled.
func (h *RingGroupHandler) executeRingAll(ctx context.Context, callCtx *flow.CallContext, node flow.Node, rg *models.RingGroup, members []*models.Extension, ringTimeout int) (string, error) {
	result, err := h.sip.RingGroup(ctx, callCtx, members, ringTimeout)
	if err != nil {
		return "", fmt.Errorf("ringing group %s: %w", rg.Name, err)
	}

	return h.evaluateResult(callCtx, node, rg, result)
}

// executeRoundRobin rings members one at a time, starting from the next
// member in rotation. Each member gets the full ring timeout. The counter
// advances on each call so that the starting member rotates.
func (h *RingGroupHandler) executeRoundRobin(ctx context.Context, callCtx *flow.CallContext, node flow.Node, rg *models.RingGroup, members []*models.Extension, ringTimeout int) (string, error) {
	// Load or create the round-robin counter for this ring group.
	val, _ := h.rrCounters.LoadOrStore(rg.ID, &atomic.Uint64{})
	counter := val.(*atomic.Uint64)

	// Advance the counter and determine the starting index.
	seq := counter.Add(1) - 1
	n := uint64(len(members))
	startIdx := int(seq % n)

	h.logger.Debug("round_robin starting member",
		"call_id", callCtx.CallID,
		"ring_group", rg.Name,
		"start_index", startIdx,
		"sequence", seq,
	)

	// Ring each member in order, wrapping around from startIdx.
	for i := 0; i < len(members); i++ {
		idx := (startIdx + i) % len(members)
		member := members[idx]

		h.logger.Debug("round_robin ringing member",
			"call_id", callCtx.CallID,
			"ring_group", rg.Name,
			"extension", member.Extension,
			"member_index", idx,
		)

		result, err := h.sip.RingExtension(ctx, callCtx, member, ringTimeout)
		if err != nil {
			h.logger.Error("round_robin ring extension failed",
				"call_id", callCtx.CallID,
				"ring_group", rg.Name,
				"extension", member.Extension,
				"error", err,
			)
			continue
		}

		if result.Answered {
			h.logger.Info("ring group answered",
				"call_id", callCtx.CallID,
				"node_id", node.ID,
				"ring_group", rg.Name,
				"answered_by", member.Extension,
				"strategy", "round_robin",
			)
			return "answered", nil
		}

		// If the member was busy or had DND, move to next member immediately.
		if result.AllBusy || result.DND || result.NoRegistrations {
			h.logger.Debug("round_robin member unavailable, trying next",
				"call_id", callCtx.CallID,
				"ring_group", rg.Name,
				"extension", member.Extension,
				"busy", result.AllBusy,
				"dnd", result.DND,
				"no_registrations", result.NoRegistrations,
			)
			continue
		}

		// Timed out — try next member.
	}

	h.logger.Info("ring group not answered",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
		"ring_group", rg.Name,
		"reason", "all_members_tried",
		"strategy", "round_robin",
	)

	return "no_answer", nil
}

// evaluateResult maps a RingResult to the appropriate output edge and logs
// the outcome. Used by ring_all and other simultaneous-ring strategies.
func (h *RingGroupHandler) evaluateResult(callCtx *flow.CallContext, node flow.Node, rg *models.RingGroup, result *flow.RingResult) (string, error) {
	if result.Answered {
		h.logger.Info("ring group answered",
			"call_id", callCtx.CallID,
			"node_id", node.ID,
			"ring_group", rg.Name,
		)
		return "answered", nil
	}

	reason := "timeout"
	if result.AllBusy {
		reason = "busy"
	} else if result.NoRegistrations {
		reason = "no_registrations"
	}

	h.logger.Info("ring group not answered",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
		"ring_group", rg.Name,
		"reason", reason,
	)

	return "no_answer", nil
}

// applyCallerIDMode modifies the call context's caller ID based on the ring
// group's caller_id_mode setting.
func (h *RingGroupHandler) applyCallerIDMode(callCtx *flow.CallContext, rg *models.RingGroup) {
	switch rg.CallerIDMode {
	case "prepend":
		// Prepend the ring group name to the caller ID display name.
		if callCtx.CallerIDName != "" {
			callCtx.CallerIDName = rg.Name + ": " + callCtx.CallerIDName
		} else {
			callCtx.CallerIDName = rg.Name
		}
	case "pass", "":
		// Keep original caller ID — nothing to do.
	}
}

// Ensure RingGroupHandler satisfies the NodeHandler interface.
var _ flow.NodeHandler = (*RingGroupHandler)(nil)
