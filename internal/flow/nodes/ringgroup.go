package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/flowpbx/flowpbx/internal/database"
	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/flowpbx/flowpbx/internal/flow"
)

// RingGroupHandler handles the Ring Group node type. It resolves the ring
// group entity, loads all member extensions, and rings them using the
// ring_all strategy — all members are rung simultaneously, and the first
// device to answer wins. The handler follows either the "answered" or
// "no_answer" output edge based on the result.
type RingGroupHandler struct {
	engine     *flow.Engine
	sip        flow.SIPActions
	extensions database.ExtensionRepository
	logger     *slog.Logger
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
// the ring_all strategy, and returns "answered" or "no_answer".
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

	// Ring all members simultaneously.
	result, err := h.sip.RingGroup(ctx, callCtx, members, ringTimeout)
	if err != nil {
		return "", fmt.Errorf("ringing group %s: %w", rg.Name, err)
	}

	if result.Answered {
		h.logger.Info("ring group answered",
			"call_id", callCtx.CallID,
			"node_id", node.ID,
			"ring_group", rg.Name,
		)
		return "answered", nil
	}

	// Log the specific reason for no answer.
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
