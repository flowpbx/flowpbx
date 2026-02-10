package nodes

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/flowpbx/flowpbx/internal/flow"
)

// defaultHangupCause is the SIP response code used when no cause code is
// configured on the hangup node.
const defaultHangupCause = 200

// defaultHangupReason is the SIP reason phrase used when none is configured.
const defaultHangupReason = "Normal clearing"

// HangupHandler handles the Hangup node type. It terminates the call with
// a configurable SIP cause code. This is a terminal node — it returns an
// empty output edge so the flow engine stops walking.
type HangupHandler struct {
	sip    flow.SIPActions
	logger *slog.Logger
}

// NewHangupHandler creates a new HangupHandler.
func NewHangupHandler(sip flow.SIPActions, logger *slog.Logger) *HangupHandler {
	return &HangupHandler{
		sip:    sip,
		logger: logger.With("handler", "hangup"),
	}
}

// Execute terminates the call by sending a SIP BYE or error response with
// the configured cause code. Returns an empty output edge to indicate this
// is a terminal node.
func (h *HangupHandler) Execute(ctx context.Context, callCtx *flow.CallContext, node flow.Node) (string, error) {
	h.logger.Debug("hangup node executing",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
	)

	// Extract cause code from node config.
	cause := defaultHangupCause
	reason := defaultHangupReason

	if node.Data.Config != nil {
		if v, ok := node.Data.Config["cause"]; ok {
			switch t := v.(type) {
			case float64:
				if t > 0 {
					cause = int(t)
				}
			case int:
				if t > 0 {
					cause = t
				}
			}
		}
		if v, ok := node.Data.Config["reason"]; ok {
			if s, ok := v.(string); ok && s != "" {
				reason = s
			}
		}
	}

	h.logger.Info("hanging up call",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
		"cause", cause,
		"reason", reason,
	)

	if err := h.sip.HangupCall(ctx, callCtx, cause, reason); err != nil {
		return "", fmt.Errorf("hanging up call: %w", err)
	}

	// Return empty string to signal terminal node.
	return "", nil
}

// Ensure HangupHandler satisfies the NodeHandler interface.
var _ flow.NodeHandler = (*HangupHandler)(nil)
