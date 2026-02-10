package nodes

import (
	"context"
	"log/slog"

	"github.com/flowpbx/flowpbx/internal/flow"
)

// WebhookHandler is a stub for the Webhook node type. Webhook nodes will
// make HTTP callouts to external APIs and route based on the response.
// This is reserved for a future phase; the current implementation logs a
// warning and continues to the "next" output edge.
type WebhookHandler struct {
	logger *slog.Logger
}

// NewWebhookHandler creates a new WebhookHandler.
func NewWebhookHandler(logger *slog.Logger) *WebhookHandler {
	return &WebhookHandler{
		logger: logger.With("handler", "webhook"),
	}
}

// Execute logs that the webhook node is not yet implemented and continues
// to the "next" output edge.
func (h *WebhookHandler) Execute(ctx context.Context, callCtx *flow.CallContext, node flow.Node) (string, error) {
	h.logger.Warn("webhook node not yet implemented, continuing to next",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
	)

	return "next", nil
}

// Ensure WebhookHandler satisfies the NodeHandler interface.
var _ flow.NodeHandler = (*WebhookHandler)(nil)
