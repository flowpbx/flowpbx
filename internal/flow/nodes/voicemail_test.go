package nodes

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/flowpbx/flowpbx/internal/flow"
)

// mockVoicemailSIPActions implements flow.SIPActions with configurable
// record results for voicemail handler testing.
type mockVoicemailSIPActions struct {
	recordResult *flow.RecordResult
	recordErr    error
	mwiCalls     []mwiCall
	mwiErr       error
}

type mwiCall struct {
	Extension   *models.Extension
	NewMessages int
	OldMessages int
}

func (m *mockVoicemailSIPActions) RingExtension(_ context.Context, _ *flow.CallContext, _ *models.Extension, _ int) (*flow.RingResult, error) {
	return nil, nil
}

func (m *mockVoicemailSIPActions) RingGroup(_ context.Context, _ *flow.CallContext, _ []*models.Extension, _ int) (*flow.RingResult, error) {
	return nil, nil
}

func (m *mockVoicemailSIPActions) PlayAndCollect(_ context.Context, _ *flow.CallContext, _ string, _ bool, _ int, _ int, _ int) (*flow.CollectResult, error) {
	return nil, nil
}

func (m *mockVoicemailSIPActions) RecordMessage(_ context.Context, _ *flow.CallContext, _ string, _ int, _ string) (*flow.RecordResult, error) {
	if m.recordErr != nil {
		return nil, m.recordErr
	}
	return m.recordResult, nil
}

func (m *mockVoicemailSIPActions) SendMWI(_ context.Context, ext *models.Extension, newMsgs int, oldMsgs int) error {
	m.mwiCalls = append(m.mwiCalls, mwiCall{Extension: ext, NewMessages: newMsgs, OldMessages: oldMsgs})
	return m.mwiErr
}

func (m *mockVoicemailSIPActions) HangupCall(_ context.Context, _ *flow.CallContext, _ int, _ string) error {
	return nil
}

func (m *mockVoicemailSIPActions) BlindTransfer(_ context.Context, _ *flow.CallContext, _ string) error {
	return nil
}

func (m *mockVoicemailSIPActions) JoinConference(_ context.Context, _ *flow.CallContext, _ *models.ConferenceBridge) error {
	return nil
}

// mockVoicemailMessageRepo implements database.VoicemailMessageRepository
// for testing. It stores created messages in memory.
type mockVoicemailMessageRepo struct {
	messages  []models.VoicemailMessage
	createErr error
	nextID    int64
}

func (m *mockVoicemailMessageRepo) Create(_ context.Context, msg *models.VoicemailMessage) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.nextID++
	msg.ID = m.nextID
	m.messages = append(m.messages, *msg)
	return nil
}

func (m *mockVoicemailMessageRepo) GetByID(_ context.Context, id int64) (*models.VoicemailMessage, error) {
	for _, msg := range m.messages {
		if msg.ID == id {
			return &msg, nil
		}
	}
	return nil, nil
}

func (m *mockVoicemailMessageRepo) ListByMailbox(_ context.Context, mailboxID int64) ([]models.VoicemailMessage, error) {
	var result []models.VoicemailMessage
	for _, msg := range m.messages {
		if msg.MailboxID == mailboxID {
			result = append(result, msg)
		}
	}
	return result, nil
}

func (m *mockVoicemailMessageRepo) MarkRead(_ context.Context, id int64) error {
	return nil
}

func (m *mockVoicemailMessageRepo) Delete(_ context.Context, id int64) error {
	return nil
}

// mockExtensionRepo implements database.ExtensionRepository for testing.
type mockExtensionRepo struct {
	extensions map[int64]*models.Extension
}

func (m *mockExtensionRepo) Create(_ context.Context, _ *models.Extension) error { return nil }
func (m *mockExtensionRepo) List(_ context.Context) ([]models.Extension, error)  { return nil, nil }
func (m *mockExtensionRepo) Update(_ context.Context, _ *models.Extension) error { return nil }
func (m *mockExtensionRepo) Delete(_ context.Context, _ int64) error             { return nil }
func (m *mockExtensionRepo) GetByExtension(_ context.Context, _ string) (*models.Extension, error) {
	return nil, nil
}
func (m *mockExtensionRepo) GetBySIPUsername(_ context.Context, _ string) (*models.Extension, error) {
	return nil, nil
}

func (m *mockExtensionRepo) GetByID(_ context.Context, id int64) (*models.Extension, error) {
	if ext, ok := m.extensions[id]; ok {
		return ext, nil
	}
	return nil, nil
}

func newTestVoicemailHandler(box *models.VoicemailBox, sipActions *mockVoicemailSIPActions, msgRepo *mockVoicemailMessageRepo, extRepo *mockExtensionRepo, dataDir string) *VoicemailHandler {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	resolver := &mockEntityResolver{entity: box}
	engine := flow.NewEngine(nil, nil, resolver, logger)
	h := NewVoicemailHandler(engine, sipActions, msgRepo, extRepo, logger, dataDir)
	return h
}

func makeVoicemailNode(entityID int64) flow.Node {
	id := entityID
	return flow.Node{
		ID:   "node_vm",
		Type: "voicemail",
		Data: flow.NodeData{
			Label:      "Sales Voicemail",
			EntityID:   &id,
			EntityType: "voicemail_box",
		},
	}
}

func TestVoicemailRecordAndStore(t *testing.T) {
	dataDir := t.TempDir()

	// Create a custom greeting at the standard path.
	greetDir := filepath.Join(dataDir, "greetings")
	if err := os.MkdirAll(greetDir, 0750); err != nil {
		t.Fatalf("failed to create greetings dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(greetDir, "box_1.wav"), []byte("fake-wav"), 0640); err != nil {
		t.Fatalf("failed to write greeting file: %v", err)
	}

	box := &models.VoicemailBox{
		ID:                 1,
		Name:               "Sales Voicemail",
		MailboxNumber:      "100",
		GreetingType:       "custom",
		MaxMessageDuration: 60,
	}

	sipActions := &mockVoicemailSIPActions{
		recordResult: &flow.RecordResult{
			FilePath:     "", // will be set by handler
			DurationSecs: 15,
		},
	}

	msgRepo := &mockVoicemailMessageRepo{}
	extRepo := &mockExtensionRepo{extensions: map[int64]*models.Extension{}}

	h := newTestVoicemailHandler(box, sipActions, msgRepo, extRepo, dataDir)
	h.nowFunc = func() time.Time {
		return time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	}

	callCtx := &flow.CallContext{
		CallID:       "test-vm-1",
		CallerIDName: "John Doe",
		CallerIDNum:  "+61400000000",
	}

	edge, err := h.Execute(context.Background(), callCtx, makeVoicemailNode(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edge != "next" {
		t.Errorf("expected edge %q, got %q", "next", edge)
	}

	// Verify message was stored.
	if len(msgRepo.messages) != 1 {
		t.Fatalf("expected 1 stored message, got %d", len(msgRepo.messages))
	}

	msg := msgRepo.messages[0]
	if msg.MailboxID != 1 {
		t.Errorf("expected mailbox_id 1, got %d", msg.MailboxID)
	}
	if msg.CallerIDName != "John Doe" {
		t.Errorf("expected caller_id_name %q, got %q", "John Doe", msg.CallerIDName)
	}
	if msg.CallerIDNum != "+61400000000" {
		t.Errorf("expected caller_id_num %q, got %q", "+61400000000", msg.CallerIDNum)
	}
	if msg.Duration != 15 {
		t.Errorf("expected duration 15, got %d", msg.Duration)
	}

	// Verify recording directory was created.
	expectedDir := filepath.Join(dataDir, "voicemail", "box_1")
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Errorf("expected voicemail directory %s to exist", expectedDir)
	}
}

func TestVoicemailDefaultGreeting(t *testing.T) {
	dataDir := t.TempDir()

	box := &models.VoicemailBox{
		ID:                 2,
		Name:               "Default Greeting Box",
		MailboxNumber:      "200",
		GreetingFile:       "", // No custom greeting.
		MaxMessageDuration: 120,
	}

	sipActions := &mockVoicemailSIPActions{
		recordResult: &flow.RecordResult{DurationSecs: 10},
	}

	msgRepo := &mockVoicemailMessageRepo{}
	extRepo := &mockExtensionRepo{extensions: map[int64]*models.Extension{}}

	h := newTestVoicemailHandler(box, sipActions, msgRepo, extRepo, dataDir)

	// Verify greeting resolves to default.
	greeting := h.resolveGreeting(box)
	expected := filepath.Join(dataDir, defaultGreetingFile)
	if greeting != expected {
		t.Errorf("expected default greeting %q, got %q", expected, greeting)
	}
}

func TestVoicemailCustomGreeting(t *testing.T) {
	dataDir := t.TempDir()

	// Create the greeting file at the standard path.
	greetDir := filepath.Join(dataDir, "greetings")
	if err := os.MkdirAll(greetDir, 0750); err != nil {
		t.Fatalf("failed to create greetings dir: %v", err)
	}
	greetingPath := filepath.Join(greetDir, "box_3.wav")
	if err := os.WriteFile(greetingPath, []byte("fake-wav"), 0640); err != nil {
		t.Fatalf("failed to write greeting file: %v", err)
	}

	box := &models.VoicemailBox{
		ID:                 3,
		Name:               "Custom Greeting Box",
		MailboxNumber:      "300",
		GreetingType:       "custom",
		MaxMessageDuration: 60,
	}

	sipActions := &mockVoicemailSIPActions{
		recordResult: &flow.RecordResult{DurationSecs: 5},
	}

	msgRepo := &mockVoicemailMessageRepo{}
	extRepo := &mockExtensionRepo{extensions: map[int64]*models.Extension{}}

	h := newTestVoicemailHandler(box, sipActions, msgRepo, extRepo, dataDir)

	greeting := h.resolveGreeting(box)
	if greeting != greetingPath {
		t.Errorf("expected custom greeting %q, got %q", greetingPath, greeting)
	}
}

func TestVoicemailCustomGreetingFallback(t *testing.T) {
	dataDir := t.TempDir()

	// Do NOT create the greeting file — test fallback to default.
	box := &models.VoicemailBox{
		ID:                 3,
		Name:               "Missing Greeting Box",
		MailboxNumber:      "300",
		GreetingType:       "custom",
		MaxMessageDuration: 60,
	}

	sipActions := &mockVoicemailSIPActions{
		recordResult: &flow.RecordResult{DurationSecs: 5},
	}

	msgRepo := &mockVoicemailMessageRepo{}
	extRepo := &mockExtensionRepo{extensions: map[int64]*models.Extension{}}

	h := newTestVoicemailHandler(box, sipActions, msgRepo, extRepo, dataDir)

	greeting := h.resolveGreeting(box)
	expected := filepath.Join(dataDir, defaultGreetingFile)
	if greeting != expected {
		t.Errorf("expected fallback to default greeting %q, got %q", expected, greeting)
	}
}

func TestVoicemailDefaultMaxDuration(t *testing.T) {
	dataDir := t.TempDir()

	box := &models.VoicemailBox{
		ID:                 4,
		Name:               "No Duration Set",
		MailboxNumber:      "400",
		MaxMessageDuration: 0, // Should use default (120).
	}

	sipActions := &mockVoicemailSIPActions{
		recordResult: &flow.RecordResult{DurationSecs: 30},
	}

	msgRepo := &mockVoicemailMessageRepo{}
	extRepo := &mockExtensionRepo{extensions: map[int64]*models.Extension{}}

	h := newTestVoicemailHandler(box, sipActions, msgRepo, extRepo, dataDir)
	callCtx := &flow.CallContext{CallID: "test-vm-duration"}

	edge, err := h.Execute(context.Background(), callCtx, makeVoicemailNode(4))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edge != "next" {
		t.Errorf("expected edge %q, got %q", "next", edge)
	}
}

func TestVoicemailMWINotification(t *testing.T) {
	dataDir := t.TempDir()

	extID := int64(10)
	box := &models.VoicemailBox{
		ID:                 5,
		Name:               "MWI Box",
		MailboxNumber:      "500",
		MaxMessageDuration: 60,
		NotifyExtensionID:  &extID,
	}

	ext := &models.Extension{
		ID:        10,
		Extension: "100",
		Name:      "Nick",
	}

	sipActions := &mockVoicemailSIPActions{
		recordResult: &flow.RecordResult{DurationSecs: 20},
	}

	// Pre-populate with one unread message to verify counts.
	msgRepo := &mockVoicemailMessageRepo{
		messages: []models.VoicemailMessage{
			{ID: 99, MailboxID: 5, Read: false},
		},
		nextID: 99,
	}

	extRepo := &mockExtensionRepo{
		extensions: map[int64]*models.Extension{10: ext},
	}

	h := newTestVoicemailHandler(box, sipActions, msgRepo, extRepo, dataDir)
	callCtx := &flow.CallContext{
		CallID:       "test-vm-mwi",
		CallerIDName: "Caller",
		CallerIDNum:  "0400000000",
	}

	edge, err := h.Execute(context.Background(), callCtx, makeVoicemailNode(5))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edge != "next" {
		t.Errorf("expected edge %q, got %q", "next", edge)
	}

	// Verify MWI was sent.
	if len(sipActions.mwiCalls) != 1 {
		t.Fatalf("expected 1 MWI call, got %d", len(sipActions.mwiCalls))
	}

	mwi := sipActions.mwiCalls[0]
	if mwi.Extension.ID != 10 {
		t.Errorf("expected MWI to extension ID 10, got %d", mwi.Extension.ID)
	}
	// 2 unread: the pre-existing one + the newly recorded one.
	if mwi.NewMessages != 2 {
		t.Errorf("expected 2 new messages in MWI, got %d", mwi.NewMessages)
	}
	if mwi.OldMessages != 0 {
		t.Errorf("expected 0 old messages in MWI, got %d", mwi.OldMessages)
	}
}

func TestVoicemailNoMWIWhenNoExtensionLinked(t *testing.T) {
	dataDir := t.TempDir()

	box := &models.VoicemailBox{
		ID:                 6,
		Name:               "No MWI Box",
		MailboxNumber:      "600",
		MaxMessageDuration: 60,
		NotifyExtensionID:  nil, // No linked extension.
	}

	sipActions := &mockVoicemailSIPActions{
		recordResult: &flow.RecordResult{DurationSecs: 10},
	}

	msgRepo := &mockVoicemailMessageRepo{}
	extRepo := &mockExtensionRepo{extensions: map[int64]*models.Extension{}}

	h := newTestVoicemailHandler(box, sipActions, msgRepo, extRepo, dataDir)
	callCtx := &flow.CallContext{CallID: "test-vm-nomwi"}

	_, err := h.Execute(context.Background(), callCtx, makeVoicemailNode(6))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify no MWI was sent.
	if len(sipActions.mwiCalls) != 0 {
		t.Errorf("expected 0 MWI calls, got %d", len(sipActions.mwiCalls))
	}
}

func TestVoicemailNoEntityError(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	resolver := &mockEntityResolver{entity: nil}
	engine := flow.NewEngine(nil, nil, resolver, logger)
	sipActions := &mockVoicemailSIPActions{}
	msgRepo := &mockVoicemailMessageRepo{}
	extRepo := &mockExtensionRepo{extensions: map[int64]*models.Extension{}}

	h := NewVoicemailHandler(engine, sipActions, msgRepo, extRepo, logger, dataDir)
	callCtx := &flow.CallContext{CallID: "test-vm-noentity"}

	_, err := h.Execute(context.Background(), callCtx, makeVoicemailNode(1))
	if err == nil {
		t.Fatal("expected error for missing entity, got nil")
	}
	if !strings.Contains(err.Error(), "no entity reference configured") {
		t.Errorf("expected 'no entity reference configured' error, got: %v", err)
	}
}

func TestVoicemailRecordingError(t *testing.T) {
	dataDir := t.TempDir()

	box := &models.VoicemailBox{
		ID:                 7,
		Name:               "Error Box",
		MailboxNumber:      "700",
		MaxMessageDuration: 60,
	}

	sipActions := &mockVoicemailSIPActions{
		recordErr: fmt.Errorf("RTP recording failed"),
	}

	msgRepo := &mockVoicemailMessageRepo{}
	extRepo := &mockExtensionRepo{extensions: map[int64]*models.Extension{}}

	h := newTestVoicemailHandler(box, sipActions, msgRepo, extRepo, dataDir)
	callCtx := &flow.CallContext{CallID: "test-vm-recerr"}

	_, err := h.Execute(context.Background(), callCtx, makeVoicemailNode(7))
	if err == nil {
		t.Fatal("expected error for recording failure, got nil")
	}
	if !strings.Contains(err.Error(), "recording voicemail") {
		t.Errorf("expected 'recording voicemail' error, got: %v", err)
	}

	// Verify no message was stored.
	if len(msgRepo.messages) != 0 {
		t.Errorf("expected 0 stored messages after recording error, got %d", len(msgRepo.messages))
	}
}

func TestVoicemailFilePathContainsBoxID(t *testing.T) {
	dataDir := t.TempDir()

	box := &models.VoicemailBox{
		ID:                 42,
		Name:               "Path Test Box",
		MailboxNumber:      "4200",
		MaxMessageDuration: 60,
	}

	sipActions := &mockVoicemailSIPActions{
		recordResult: &flow.RecordResult{DurationSecs: 5},
	}

	msgRepo := &mockVoicemailMessageRepo{}
	extRepo := &mockExtensionRepo{extensions: map[int64]*models.Extension{}}

	h := newTestVoicemailHandler(box, sipActions, msgRepo, extRepo, dataDir)
	callCtx := &flow.CallContext{CallID: "test-vm-path"}

	_, err := h.Execute(context.Background(), callCtx, makeVoicemailNode(42))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file path is organized by box ID.
	if len(msgRepo.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgRepo.messages))
	}

	filePath := msgRepo.messages[0].FilePath
	if !strings.Contains(filePath, "box_42") {
		t.Errorf("expected file path to contain 'box_42', got %q", filePath)
	}
	if !strings.HasSuffix(filePath, ".wav") {
		t.Errorf("expected file path to end with '.wav', got %q", filePath)
	}
}
