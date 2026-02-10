package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/flowpbx/flowpbx/internal/flow"
)

// IVRMenuHandler handles the IVR Menu node type. It plays a prompt, collects
// DTMF digits, and routes to the output edge matching the pressed digit.
// If the caller presses an invalid digit or times out, retries are attempted
// up to the configured maximum. Final timeout follows the "timeout" edge
// and final invalid follows the "invalid" edge.
type IVRMenuHandler struct {
	engine *flow.Engine
	sip    flow.SIPActions
	logger *slog.Logger
}

// NewIVRMenuHandler creates a new IVRMenuHandler.
func NewIVRMenuHandler(engine *flow.Engine, sip flow.SIPActions, logger *slog.Logger) *IVRMenuHandler {
	return &IVRMenuHandler{
		engine: engine,
		sip:    sip,
		logger: logger.With("handler", "ivr_menu"),
	}
}

// Execute resolves the IVR menu entity, plays the greeting prompt, collects
// DTMF input, and returns the matching digit as the output edge name.
// On timeout it returns "timeout", on invalid input after max retries it
// returns "invalid".
func (h *IVRMenuHandler) Execute(ctx context.Context, callCtx *flow.CallContext, node flow.Node) (string, error) {
	h.logger.Debug("ivr menu node executing",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
	)

	// Resolve the IVR menu entity.
	entity, err := h.engine.ResolveNodeEntity(ctx, node)
	if err != nil {
		return "", fmt.Errorf("resolving ivr menu entity: %w", err)
	}
	if entity == nil {
		return "", fmt.Errorf("ivr menu node %s: no entity reference configured", node.ID)
	}

	menu, ok := entity.(*models.IVRMenu)
	if !ok {
		return "", fmt.Errorf("ivr menu node %s: entity is %T, expected *models.IVRMenu", node.ID, entity)
	}

	// Parse the options JSON to validate digit mappings.
	var options map[string]string
	if err := json.Unmarshal([]byte(menu.Options), &options); err != nil {
		return "", fmt.Errorf("ivr menu node %s: parsing options: %w", node.ID, err)
	}

	// Determine prompt source (file path or TTS text).
	prompt := menu.GreetingFile
	isTTS := false
	if prompt == "" && menu.GreetingTTS != "" {
		prompt = menu.GreetingTTS
		isTTS = true
	}

	// Apply defaults.
	timeout := menu.Timeout
	if timeout <= 0 {
		timeout = 10
	}
	digitTimeout := menu.DigitTimeout
	if digitTimeout <= 0 {
		digitTimeout = 3
	}
	maxRetries := menu.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	h.logger.Info("ivr menu collecting input",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
		"menu", menu.Name,
		"menu_id", menu.ID,
		"timeout", timeout,
		"digit_timeout", digitTimeout,
		"max_retries", maxRetries,
		"option_count", len(options),
	)

	// Clear any stale DTMF from previous nodes.
	callCtx.ClearDTMF()

	// Retry loop: play prompt and collect digits up to maxRetries times.
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		h.logger.Debug("ivr menu attempt",
			"call_id", callCtx.CallID,
			"node_id", node.ID,
			"attempt", attempt+1,
			"max_retries", maxRetries,
		)

		// Play prompt and collect a single digit.
		result, err := h.sip.PlayAndCollect(ctx, callCtx, prompt, isTTS, timeout, digitTimeout, 1)
		if err != nil {
			return "", fmt.Errorf("ivr menu play and collect: %w", err)
		}

		if result.TimedOut && result.Digits == "" {
			h.logger.Debug("ivr menu timeout, no digit received",
				"call_id", callCtx.CallID,
				"node_id", node.ID,
				"attempt", attempt+1,
			)
			// On last attempt, follow timeout edge.
			if attempt >= maxRetries {
				h.logger.Info("ivr menu max retries exhausted (timeout)",
					"call_id", callCtx.CallID,
					"node_id", node.ID,
				)
				return "timeout", nil
			}
			continue
		}

		digit := result.Digits
		if digit == "" {
			continue
		}

		// Check if the digit matches a configured option.
		if _, matched := options[digit]; matched {
			h.logger.Info("ivr menu digit matched",
				"call_id", callCtx.CallID,
				"node_id", node.ID,
				"digit", digit,
			)
			return digit, nil
		}

		// Invalid digit pressed.
		h.logger.Debug("ivr menu invalid digit",
			"call_id", callCtx.CallID,
			"node_id", node.ID,
			"digit", digit,
			"attempt", attempt+1,
		)

		// On last attempt, follow invalid edge.
		if attempt >= maxRetries {
			h.logger.Info("ivr menu max retries exhausted (invalid)",
				"call_id", callCtx.CallID,
				"node_id", node.ID,
				"last_digit", digit,
			)
			return "invalid", nil
		}

		// Clear DTMF buffer for next attempt.
		callCtx.ClearDTMF()
	}

	// Should not be reached, but handle gracefully.
	return "timeout", nil
}

// Ensure IVRMenuHandler satisfies the NodeHandler interface.
var _ flow.NodeHandler = (*IVRMenuHandler)(nil)
