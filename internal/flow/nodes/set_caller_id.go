package nodes

import (
	"context"
	"log/slog"

	"github.com/flowpbx/flowpbx/internal/flow"
)

// SetCallerIDHandler handles the Set Caller ID node type. It overrides the
// caller ID name and/or number on the CallContext for all downstream nodes.
// This is a passthrough node — it modifies state and returns "next".
type SetCallerIDHandler struct {
	logger *slog.Logger
}

// NewSetCallerIDHandler creates a new SetCallerIDHandler.
func NewSetCallerIDHandler(logger *slog.Logger) *SetCallerIDHandler {
	return &SetCallerIDHandler{
		logger: logger.With("handler", "set_caller_id"),
	}
}

// Execute reads the "name" and "number" fields from the node config and
// overrides the corresponding CallContext fields. Either or both fields
// may be set. Returns "next" to continue the flow.
func (h *SetCallerIDHandler) Execute(ctx context.Context, callCtx *flow.CallContext, node flow.Node) (string, error) {
	h.logger.Debug("set caller id node executing",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
	)

	originalName := callCtx.CallerIDName
	originalNum := callCtx.CallerIDNum

	if node.Data.Config != nil {
		if v, ok := node.Data.Config["name"]; ok {
			if s, ok := v.(string); ok && s != "" {
				callCtx.CallerIDName = s
			}
		}
		if v, ok := node.Data.Config["number"]; ok {
			if s, ok := v.(string); ok && s != "" {
				callCtx.CallerIDNum = s
			}
		}
	}

	h.logger.Info("caller id overridden",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
		"original_name", originalName,
		"original_num", originalNum,
		"new_name", callCtx.CallerIDName,
		"new_num", callCtx.CallerIDNum,
	)

	return "next", nil
}

// Ensure SetCallerIDHandler satisfies the NodeHandler interface.
var _ flow.NodeHandler = (*SetCallerIDHandler)(nil)
