package nodes

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/flowpbx/flowpbx/internal/flow"
)

// TransferHandler handles the Transfer node type. It performs a blind transfer
// to the configured destination (external number or extension). This is a
// terminal node — after a successful transfer, the flow ends.
type TransferHandler struct {
	sip    flow.SIPActions
	logger *slog.Logger
}

// NewTransferHandler creates a new TransferHandler.
func NewTransferHandler(sip flow.SIPActions, logger *slog.Logger) *TransferHandler {
	return &TransferHandler{
		sip:    sip,
		logger: logger.With("handler", "transfer"),
	}
}

// Execute performs a blind transfer to the destination specified in the node
// config's "destination" field. Returns an empty output edge to indicate
// this is a terminal node.
func (h *TransferHandler) Execute(ctx context.Context, callCtx *flow.CallContext, node flow.Node) (string, error) {
	h.logger.Debug("transfer node executing",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
	)

	// Extract destination from node config.
	if node.Data.Config == nil {
		return "", fmt.Errorf("transfer node %s: no config specified", node.ID)
	}

	destination, ok := node.Data.Config["destination"]
	if !ok {
		return "", fmt.Errorf("transfer node %s: no destination configured", node.ID)
	}
	dest, ok := destination.(string)
	if !ok || dest == "" {
		return "", fmt.Errorf("transfer node %s: destination must be a non-empty string", node.ID)
	}

	h.logger.Info("transferring call",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
		"destination", dest,
	)

	if err := h.sip.BlindTransfer(ctx, callCtx, dest); err != nil {
		return "", fmt.Errorf("transferring call to %s: %w", dest, err)
	}

	// Return empty string to signal terminal node.
	return "", nil
}

// Ensure TransferHandler satisfies the NodeHandler interface.
var _ flow.NodeHandler = (*TransferHandler)(nil)
