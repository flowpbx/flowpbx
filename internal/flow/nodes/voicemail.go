package nodes

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/flowpbx/flowpbx/internal/database"
	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/flowpbx/flowpbx/internal/email"
	"github.com/flowpbx/flowpbx/internal/flow"
	"github.com/flowpbx/flowpbx/internal/prompts"
)

// defaultMaxMessageDuration is the default maximum voicemail recording length
// in seconds, used when the voicemail box has no override configured.
const defaultMaxMessageDuration = 120

// defaultGreetingFile is the path to the built-in greeting played when a
// voicemail box has no custom greeting configured.
const defaultGreetingFile = "prompts/system/default_voicemail_greeting.wav"

// VoicemailHandler handles the Voicemail node type. It plays the greeting
// for the target voicemail box, records the caller's message to a WAV file,
// stores the message metadata, and triggers MWI notification to the linked
// extension if configured. When the voicemail box has email notification
// enabled and SMTP is configured, it sends an email with optional WAV
// attachment.
type VoicemailHandler struct {
	engine     *flow.Engine
	sip        flow.SIPActions
	messages   database.VoicemailMessageRepository
	extensions database.ExtensionRepository
	sysConfig  database.SystemConfigRepository
	enc        *database.Encryptor
	emailSend  *email.Sender
	logger     *slog.Logger
	dataDir    string
	nowFunc    func() time.Time // injectable for testing
}

// NewVoicemailHandler creates a new VoicemailHandler.
func NewVoicemailHandler(
	engine *flow.Engine,
	sip flow.SIPActions,
	messages database.VoicemailMessageRepository,
	extensions database.ExtensionRepository,
	sysConfig database.SystemConfigRepository,
	enc *database.Encryptor,
	emailSend *email.Sender,
	logger *slog.Logger,
	dataDir string,
) *VoicemailHandler {
	return &VoicemailHandler{
		engine:     engine,
		sip:        sip,
		messages:   messages,
		extensions: extensions,
		sysConfig:  sysConfig,
		enc:        enc,
		emailSend:  emailSend,
		logger:     logger.With("handler", "voicemail"),
		dataDir:    dataDir,
		nowFunc:    time.Now,
	}
}

// Execute resolves the voicemail box entity, plays its greeting, records the
// caller's message, stores the message metadata, and sends MWI if a linked
// extension is configured. Voicemail is a terminal node — it returns "next"
// so the flow can optionally continue (e.g. to a hangup node).
func (h *VoicemailHandler) Execute(ctx context.Context, callCtx *flow.CallContext, node flow.Node) (string, error) {
	h.logger.Debug("voicemail node executing",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
	)

	// Resolve the voicemail box entity.
	entity, err := h.engine.ResolveNodeEntity(ctx, node)
	if err != nil {
		return "", fmt.Errorf("resolving voicemail box entity: %w", err)
	}
	if entity == nil {
		return "", fmt.Errorf("voicemail node %s: no entity reference configured", node.ID)
	}

	box, ok := entity.(*models.VoicemailBox)
	if !ok {
		return "", fmt.Errorf("voicemail node %s: entity is %T, expected *models.VoicemailBox", node.ID, entity)
	}

	// Check if the mailbox has reached its max_messages limit.
	if box.MaxMessages > 0 {
		count, err := h.messages.CountByMailbox(ctx, box.ID)
		if err != nil {
			return "", fmt.Errorf("checking mailbox message count: %w", err)
		}
		if count >= int64(box.MaxMessages) {
			h.logger.Warn("voicemail box full, rejecting recording",
				"call_id", callCtx.CallID,
				"mailbox_id", box.ID,
				"max_messages", box.MaxMessages,
				"current_count", count,
			)
			return "", fmt.Errorf("voicemail box %d is full (%d/%d messages)", box.ID, count, box.MaxMessages)
		}
	}

	// Determine the greeting to play.
	greeting := h.resolveGreeting(box)

	// Determine max recording duration.
	maxDuration := box.MaxMessageDuration
	if maxDuration <= 0 {
		maxDuration = defaultMaxMessageDuration
	}

	// Build the recording file path: voicemail/box_<id>/msg_<timestamp>.wav
	now := h.nowFunc()
	recordingDir := filepath.Join(h.dataDir, "voicemail", fmt.Sprintf("box_%d", box.ID))
	if err := os.MkdirAll(recordingDir, 0750); err != nil {
		return "", fmt.Errorf("creating voicemail directory: %w", err)
	}
	recordingFile := filepath.Join(recordingDir, fmt.Sprintf("msg_%d.wav", now.UnixMilli()))

	h.logger.Info("recording voicemail",
		"call_id", callCtx.CallID,
		"node_id", node.ID,
		"mailbox", box.MailboxNumber,
		"mailbox_id", box.ID,
		"greeting", greeting,
		"max_duration", maxDuration,
		"file", recordingFile,
	)

	// Play greeting and record the message.
	result, err := h.sip.RecordMessage(ctx, callCtx, greeting, maxDuration, recordingFile)
	if err != nil {
		return "", fmt.Errorf("recording voicemail: %w", err)
	}

	// Store the voicemail message metadata.
	msg := &models.VoicemailMessage{
		MailboxID:    box.ID,
		CallerIDName: callCtx.CallerIDName,
		CallerIDNum:  callCtx.CallerIDNum,
		Timestamp:    now,
		Duration:     result.DurationSecs,
		FilePath:     recordingFile,
	}
	if err := h.messages.Create(ctx, msg); err != nil {
		h.logger.Error("failed to save voicemail message metadata",
			"call_id", callCtx.CallID,
			"mailbox_id", box.ID,
			"error", err,
		)
		// Don't fail the node — the recording was captured even if metadata save failed.
	} else {
		h.logger.Info("voicemail message saved",
			"call_id", callCtx.CallID,
			"mailbox_id", box.ID,
			"message_id", msg.ID,
			"duration", result.DurationSecs,
		)
	}

	// Send MWI notification to the linked extension, if configured.
	if box.NotifyExtensionID != nil {
		h.sendMWI(ctx, box, *box.NotifyExtensionID)
	}

	// Send email notification if enabled for this box.
	if box.EmailNotify && box.EmailAddress != "" {
		h.sendEmailNotification(ctx, box, msg)
	}

	return "next", nil
}

// resolveGreeting returns the greeting file path for the voicemail box.
// When the greeting type is "custom", the handler checks for a greeting file
// at the standard path $DATA_DIR/greetings/box_{id}.wav. If that file exists,
// it is used. Otherwise, the default system greeting is played.
func (h *VoicemailHandler) resolveGreeting(box *models.VoicemailBox) string {
	if box.GreetingType == "custom" {
		greetingPath := prompts.GreetingPath(h.dataDir, box.ID)
		if _, err := os.Stat(greetingPath); err == nil {
			return greetingPath
		}
		h.logger.Warn("custom greeting file not found, falling back to default",
			"mailbox_id", box.ID,
			"expected_path", greetingPath,
		)
	}
	return filepath.Join(h.dataDir, defaultGreetingFile)
}

// sendMWI looks up the linked extension and sends a SIP NOTIFY to update
// the message waiting indicator. Errors are logged but do not fail the node.
func (h *VoicemailHandler) sendMWI(ctx context.Context, box *models.VoicemailBox, extensionID int64) {
	ext, err := h.extensions.GetByID(ctx, extensionID)
	if err != nil {
		h.logger.Error("failed to resolve MWI extension",
			"mailbox_id", box.ID,
			"extension_id", extensionID,
			"error", err,
		)
		return
	}
	if ext == nil {
		h.logger.Warn("MWI extension not found",
			"mailbox_id", box.ID,
			"extension_id", extensionID,
		)
		return
	}

	// Count messages in the mailbox for the MWI summary.
	messages, err := h.messages.ListByMailbox(ctx, box.ID)
	if err != nil {
		h.logger.Error("failed to count voicemail messages for MWI",
			"mailbox_id", box.ID,
			"error", err,
		)
		return
	}

	newCount := 0
	oldCount := 0
	for _, m := range messages {
		if m.Read {
			oldCount++
		} else {
			newCount++
		}
	}

	if err := h.sip.SendMWI(ctx, ext, newCount, oldCount); err != nil {
		h.logger.Error("failed to send MWI notification",
			"mailbox_id", box.ID,
			"extension", ext.Extension,
			"error", err,
		)
		return
	}

	h.logger.Info("MWI notification sent",
		"mailbox_id", box.ID,
		"extension", ext.Extension,
		"new_messages", newCount,
		"old_messages", oldCount,
	)
}

// sendEmailNotification loads SMTP configuration and sends an email
// notification for the new voicemail message. Errors are logged but do not
// fail the node.
func (h *VoicemailHandler) sendEmailNotification(ctx context.Context, box *models.VoicemailBox, msg *models.VoicemailMessage) {
	if h.emailSend == nil || h.sysConfig == nil {
		h.logger.Debug("email notification skipped: email sender or system config not available",
			"mailbox_id", box.ID,
		)
		return
	}

	cfg, err := h.loadSMTPConfig(ctx)
	if err != nil {
		h.logger.Error("failed to load smtp config for voicemail email",
			"mailbox_id", box.ID,
			"error", err,
		)
		return
	}

	if !cfg.Valid() {
		h.logger.Debug("email notification skipped: smtp not configured",
			"mailbox_id", box.ID,
		)
		return
	}

	notif := email.VoicemailNotification{
		To:           box.EmailAddress,
		BoxName:      box.Name,
		CallerIDName: msg.CallerIDName,
		CallerIDNum:  msg.CallerIDNum,
		Timestamp:    msg.Timestamp,
		DurationSecs: msg.Duration,
		AudioFile:    msg.FilePath,
		AttachAudio:  box.EmailAttachAudio,
	}

	if err := h.emailSend.SendVoicemailNotification(ctx, cfg, notif); err != nil {
		h.logger.Error("failed to send voicemail email notification",
			"mailbox_id", box.ID,
			"to", box.EmailAddress,
			"error", err,
		)
		return
	}

	h.logger.Info("voicemail email notification sent",
		"mailbox_id", box.ID,
		"to", box.EmailAddress,
	)
}

// loadSMTPConfig reads SMTP settings from system_config, decrypting the
// password if an encryptor is available.
func (h *VoicemailHandler) loadSMTPConfig(ctx context.Context) (email.SMTPConfig, error) {
	get := func(key string) string {
		val, _ := h.sysConfig.Get(ctx, key)
		return val
	}

	cfg := email.SMTPConfig{
		Host:     get("smtp_host"),
		Port:     get("smtp_port"),
		From:     get("smtp_from"),
		Username: get("smtp_username"),
		TLS:      get("smtp_tls"),
	}

	password := get("smtp_password")
	if password != "" && h.enc != nil {
		decrypted, err := h.enc.Decrypt(password)
		if err != nil {
			return cfg, fmt.Errorf("decrypting smtp password: %w", err)
		}
		cfg.Password = decrypted
	} else {
		cfg.Password = password
	}

	return cfg, nil
}

// Ensure VoicemailHandler satisfies the NodeHandler interface.
var _ flow.NodeHandler = (*VoicemailHandler)(nil)
