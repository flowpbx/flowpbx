package nodes

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/flowpbx/flowpbx/internal/flow"
)

// InboundNumberHandler handles the Inbound Number entry point node.
// It verifies DID matching and passes through to the "next" output edge.
type InboundNumberHandler struct {
	logger *slog.Logger
}

// NewInboundNumberHandler creates a new InboundNumberHandler.
func NewInboundNumberHandler(logger *slog.Logger) *InboundNumberHandler {
	return &InboundNumberHandler{
		logger: logger.With("handler", "inbound_number"),
	}
}

// Execute verifies the inbound number entity matches the call context DID,
// then returns the "next" output edge to continue the flow.
func (h *InboundNumberHandler) Execute(ctx context.Context, callCtx *flow.CallContext, node flow.Node) (string, error) {
	h.logger.Debug("inbound number node executing",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
		"callee", callCtx.Callee,
	)

	// The inbound number entity is already resolved on the CallContext by the
	// SIP invite handler before the flow engine starts. Verify it matches
	// the node's entity reference if one is configured.
	if node.Data.EntityID != nil {
		if callCtx.InboundNumber == nil {
			return "", fmt.Errorf("inbound number node %s: no inbound number on call context", node.ID)
		}

		if callCtx.InboundNumber.ID != *node.Data.EntityID {
			h.logger.Warn("inbound number entity mismatch",
				"call_id", callCtx.CallID,
				"node_entity_id", *node.Data.EntityID,
				"call_ctx_inbound_id", callCtx.InboundNumber.ID,
			)
		}
	}

	// Log the DID info for tracing.
	var didNumber string
	if callCtx.InboundNumber != nil {
		didNumber = callCtx.InboundNumber.Number
	}

	h.logger.Info("inbound number node matched",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
		"did", didNumber,
		"caller_name", callCtx.CallerIDName,
		"caller_num", callCtx.CallerIDNum,
	)

	// Pass through to the next node via the "next" output edge.
	return "next", nil
}

// Ensure InboundNumberHandler satisfies the NodeHandler interface.
var _ flow.NodeHandler = (*InboundNumberHandler)(nil)
