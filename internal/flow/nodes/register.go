package nodes

import (
	"log/slog"

	"github.com/flowpbx/flowpbx/internal/flow"
)

// RegisterAll registers all implemented node handlers on the flow engine.
func RegisterAll(engine *flow.Engine, logger *slog.Logger) {
	engine.RegisterHandler("inbound_number", NewInboundNumberHandler(logger))
}
