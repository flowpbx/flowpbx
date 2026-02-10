package nodes

import (
	"log/slog"

	"github.com/flowpbx/flowpbx/internal/database"
	"github.com/flowpbx/flowpbx/internal/email"
	"github.com/flowpbx/flowpbx/internal/flow"
)

// RegisterAll registers all implemented node handlers on the flow engine.
// The sipActions parameter provides SIP operations needed by handlers that
// interact with the call (ringing extensions, media bridging, etc.).
// The extensions parameter provides access to the extension repository for
// handlers that need to resolve member extensions (e.g. ring groups).
// The voicemailMessages parameter provides voicemail message storage.
// The sysConfig parameter provides access to system configuration (SMTP etc.).
// The enc parameter provides encryption/decryption for sensitive config values.
// The emailSend parameter provides email sending capability.
// The dataDir parameter is the root data directory for file storage.
func RegisterAll(
	engine *flow.Engine,
	sipActions flow.SIPActions,
	extensions database.ExtensionRepository,
	voicemailMessages database.VoicemailMessageRepository,
	sysConfig database.SystemConfigRepository,
	enc *database.Encryptor,
	emailSend *email.Sender,
	dataDir string,
	logger *slog.Logger,
) {
	engine.RegisterHandler("inbound_number", NewInboundNumberHandler(logger))
	engine.RegisterHandler("extension", NewExtensionHandler(engine, sipActions, logger))
	engine.RegisterHandler("ring_group", NewRingGroupHandler(engine, sipActions, extensions, logger))
	engine.RegisterHandler("time_switch", NewTimeSwitchHandler(engine, logger))
	engine.RegisterHandler("ivr_menu", NewIVRMenuHandler(engine, sipActions, logger))
	engine.RegisterHandler("voicemail", NewVoicemailHandler(engine, sipActions, voicemailMessages, extensions, sysConfig, enc, emailSend, logger, dataDir))
	engine.RegisterHandler("play_message", NewPlayMessageHandler(engine, sipActions, logger))
	engine.RegisterHandler("hangup", NewHangupHandler(sipActions, logger))
	engine.RegisterHandler("set_caller_id", NewSetCallerIDHandler(logger))
	engine.RegisterHandler("transfer", NewTransferHandler(sipActions, logger))
	engine.RegisterHandler("conference", NewConferenceHandler(engine, sipActions, logger))
	engine.RegisterHandler("webhook", NewWebhookHandler(logger))
	engine.RegisterHandler("queue", NewQueueHandler(logger))
}
