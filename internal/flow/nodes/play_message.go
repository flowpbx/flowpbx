package nodes

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/flowpbx/flowpbx/internal/flow"
)

// PlayMessageHandler handles the Play Message node type. It plays an audio
// file or TTS text to the caller and then continues to the "next" output edge.
type PlayMessageHandler struct {
	engine *flow.Engine
	sip    flow.SIPActions
	logger *slog.Logger
}

// NewPlayMessageHandler creates a new PlayMessageHandler.
func NewPlayMessageHandler(engine *flow.Engine, sip flow.SIPActions, logger *slog.Logger) *PlayMessageHandler {
	return &PlayMessageHandler{
		engine: engine,
		sip:    sip,
		logger: logger.With("handler", "play_message"),
	}
}

// Execute plays the configured audio file or TTS text, then returns "next"
// to continue the flow. The prompt is configured via the node's Config map
// with keys "file" (audio file path) or "tts" (TTS text). If both are set,
// "file" takes precedence.
func (h *PlayMessageHandler) Execute(ctx context.Context, callCtx *flow.CallContext, node flow.Node) (string, error) {
	h.logger.Debug("play message node executing",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
	)

	// Determine the prompt source from node config.
	prompt, isTTS, err := h.resolvePrompt(node)
	if err != nil {
		return "", fmt.Errorf("play message node %s: %w", node.ID, err)
	}

	h.logger.Info("playing message",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
		"prompt", prompt,
		"is_tts", isTTS,
	)

	// Play the audio using PlayAndCollect with maxDigits=0 (no collection).
	// We ignore the collect result since we only want playback.
	_, err = h.sip.PlayAndCollect(ctx, callCtx, prompt, isTTS, 0, 0, 0)
	if err != nil {
		return "", fmt.Errorf("playing message: %w", err)
	}

	return "next", nil
}

// resolvePrompt extracts the audio file path or TTS text from the node config.
func (h *PlayMessageHandler) resolvePrompt(node flow.Node) (prompt string, isTTS bool, err error) {
	if node.Data.Config == nil {
		return "", false, fmt.Errorf("no config specified")
	}

	// Check for audio file path first (takes precedence).
	if v, ok := node.Data.Config["file"]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s, false, nil
		}
	}

	// Fall back to TTS text.
	if v, ok := node.Data.Config["tts"]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s, true, nil
		}
	}

	return "", false, fmt.Errorf("no 'file' or 'tts' prompt configured")
}

// Ensure PlayMessageHandler satisfies the NodeHandler interface.
var _ flow.NodeHandler = (*PlayMessageHandler)(nil)
