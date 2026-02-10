package nodes

import (
	"log/slog"

	"github.com/flowpbx/flowpbx/internal/flow"
)

// RegisterAll registers all implemented node handlers on the flow engine.
// The sipActions parameter provides SIP operations needed by handlers that
// interact with the call (ringing extensions, media bridging, etc.).
func RegisterAll(engine *flow.Engine, sipActions flow.SIPActions, logger *slog.Logger) {
	engine.RegisterHandler("inbound_number", NewInboundNumberHandler(logger))
	engine.RegisterHandler("extension", NewExtensionHandler(engine, sipActions, logger))
}
