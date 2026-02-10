package nodes

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/flowpbx/flowpbx/internal/flow"
)

// ExtensionHandler handles the Extension node type. It rings all registered
// devices for the referenced extension with a configurable timeout, then
// follows either the "answered" or "no_answer" output edge.
type ExtensionHandler struct {
	engine *flow.Engine
	sip    flow.SIPActions
	logger *slog.Logger
}

// NewExtensionHandler creates a new ExtensionHandler.
func NewExtensionHandler(engine *flow.Engine, sip flow.SIPActions, logger *slog.Logger) *ExtensionHandler {
	return &ExtensionHandler{
		engine: engine,
		sip:    sip,
		logger: logger.With("handler", "extension"),
	}
}

// Execute resolves the extension entity, rings all registered devices,
// and returns "answered" or "no_answer" based on the result.
func (h *ExtensionHandler) Execute(ctx context.Context, callCtx *flow.CallContext, node flow.Node) (string, error) {
	h.logger.Debug("extension node executing",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
	)

	// Resolve the extension entity from the node's entity_id.
	entity, err := h.engine.ResolveNodeEntity(ctx, node)
	if err != nil {
		return "", fmt.Errorf("resolving extension entity: %w", err)
	}
	if entity == nil {
		return "", fmt.Errorf("extension node %s: no entity reference configured", node.ID)
	}

	ext, ok := entity.(*models.Extension)
	if !ok {
		return "", fmt.Errorf("extension node %s: entity is %T, expected *models.Extension", node.ID, entity)
	}

	// Determine ring timeout. Use the node config if specified, otherwise
	// fall back to the extension's ring_timeout setting, defaulting to 30s.
	ringTimeout := ext.RingTimeout
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

	h.logger.Info("ringing extension",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
		"extension", ext.Extension,
		"extension_id", ext.ID,
		"ring_timeout", ringTimeout,
	)

	// Ring the extension via SIP.
	result, err := h.sip.RingExtension(ctx, callCtx, ext, ringTimeout)
	if err != nil {
		return "", fmt.Errorf("ringing extension %s: %w", ext.Extension, err)
	}

	if result.Answered {
		h.logger.Info("extension answered",
			"call_id", callCtx.CallID,
			"node_id", node.ID,
			"extension", ext.Extension,
		)
		return "answered", nil
	}

	// Log the specific reason for no answer.
	reason := "timeout"
	if result.DND {
		reason = "dnd"
	} else if result.AllBusy {
		reason = "busy"
	} else if result.NoRegistrations {
		reason = "no_registrations"
	}

	h.logger.Info("extension not answered",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
		"extension", ext.Extension,
		"reason", reason,
	)

	return "no_answer", nil
}

// Ensure ExtensionHandler satisfies the NodeHandler interface.
var _ flow.NodeHandler = (*ExtensionHandler)(nil)
