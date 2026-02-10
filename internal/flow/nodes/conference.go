package nodes

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/flowpbx/flowpbx/internal/flow"
)

// ConferenceHandler handles the Conference node type. It joins the caller
// into a conference bridge. When the caller leaves (hangs up or is kicked),
// the flow continues to the "next" output edge.
type ConferenceHandler struct {
	engine *flow.Engine
	sip    flow.SIPActions
	logger *slog.Logger
}

// NewConferenceHandler creates a new ConferenceHandler.
func NewConferenceHandler(engine *flow.Engine, sip flow.SIPActions, logger *slog.Logger) *ConferenceHandler {
	return &ConferenceHandler{
		engine: engine,
		sip:    sip,
		logger: logger.With("handler", "conference"),
	}
}

// Execute resolves the conference bridge entity, joins the caller into the
// bridge, and waits until the caller leaves. Returns "next" after the caller
// has left the conference.
func (h *ConferenceHandler) Execute(ctx context.Context, callCtx *flow.CallContext, node flow.Node) (string, error) {
	h.logger.Debug("conference node executing",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
	)

	// Resolve the conference bridge entity.
	entity, err := h.engine.ResolveNodeEntity(ctx, node)
	if err != nil {
		return "", fmt.Errorf("resolving conference bridge entity: %w", err)
	}
	if entity == nil {
		return "", fmt.Errorf("conference node %s: no entity reference configured", node.ID)
	}

	bridge, ok := entity.(*models.ConferenceBridge)
	if !ok {
		return "", fmt.Errorf("conference node %s: entity is %T, expected *models.ConferenceBridge", node.ID, entity)
	}

	h.logger.Info("joining conference",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
		"conference", bridge.Name,
		"conference_id", bridge.ID,
	)

	// Join the caller into the conference bridge. This blocks until the
	// caller leaves (hangs up, is kicked, or context is cancelled).
	if err := h.sip.JoinConference(ctx, callCtx, bridge); err != nil {
		return "", fmt.Errorf("joining conference %s: %w", bridge.Name, err)
	}

	h.logger.Info("caller left conference",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
		"conference", bridge.Name,
	)

	return "next", nil
}

// Ensure ConferenceHandler satisfies the NodeHandler interface.
var _ flow.NodeHandler = (*ConferenceHandler)(nil)
