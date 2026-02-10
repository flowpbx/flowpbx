package nodes

import (
	"log/slog"

	"github.com/flowpbx/flowpbx/internal/database"
	"github.com/flowpbx/flowpbx/internal/flow"
)

// RegisterAll registers all implemented node handlers on the flow engine.
// The sipActions parameter provides SIP operations needed by handlers that
// interact with the call (ringing extensions, media bridging, etc.).
// The extensions parameter provides access to the extension repository for
// handlers that need to resolve member extensions (e.g. ring groups).
// The voicemailMessages parameter provides voicemail message storage.
// The dataDir parameter is the root data directory for file storage.
func RegisterAll(
	engine *flow.Engine,
	sipActions flow.SIPActions,
	extensions database.ExtensionRepository,
	voicemailMessages database.VoicemailMessageRepository,
	dataDir string,
	logger *slog.Logger,
) {
	engine.RegisterHandler("inbound_number", NewInboundNumberHandler(logger))
	engine.RegisterHandler("extension", NewExtensionHandler(engine, sipActions, logger))
	engine.RegisterHandler("ring_group", NewRingGroupHandler(engine, sipActions, extensions, logger))
	engine.RegisterHandler("time_switch", NewTimeSwitchHandler(engine, logger))
	engine.RegisterHandler("ivr_menu", NewIVRMenuHandler(engine, sipActions, logger))
	engine.RegisterHandler("voicemail", NewVoicemailHandler(engine, sipActions, voicemailMessages, extensions, logger, dataDir))
}
