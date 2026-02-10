package nodes

import (
	"context"
	"log/slog"

	"github.com/flowpbx/flowpbx/internal/flow"
)

// QueueHandler is a stub for the Queue (ACD) node type. Queue nodes will
// place callers in a hold queue with music and distribute calls to agents.
// This is reserved for a future phase; the current implementation logs a
// warning and follows the "timeout" output edge since no agents are available.
type QueueHandler struct {
	logger *slog.Logger
}

// NewQueueHandler creates a new QueueHandler.
func NewQueueHandler(logger *slog.Logger) *QueueHandler {
	return &QueueHandler{
		logger: logger.With("handler", "queue"),
	}
}

// Execute logs that the queue node is not yet implemented and follows the
// "timeout" output edge to allow the flow to continue gracefully.
func (h *QueueHandler) Execute(ctx context.Context, callCtx *flow.CallContext, node flow.Node) (string, error) {
	h.logger.Warn("queue node not yet implemented, following timeout edge",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
	)

	return "timeout", nil
}

// Ensure QueueHandler satisfies the NodeHandler interface.
var _ flow.NodeHandler = (*QueueHandler)(nil)
