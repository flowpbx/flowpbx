package nodes

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/flowpbx/flowpbx/internal/flow"
)

// mockSIPActions is a minimal SIPActions mock for testing node handlers
// that need PlayAndCollect. RingExtension and RingGroup are not used by
// the IVR handler and return nil.
type mockSIPActions struct {
	collectResults []*flow.CollectResult // results returned in order
	collectErrors  []error
	callIdx        int
}

func (m *mockSIPActions) RingExtension(_ context.Context, _ *flow.CallContext, _ *models.Extension, _ int) (*flow.RingResult, error) {
	return nil, nil
}

func (m *mockSIPActions) RingGroup(_ context.Context, _ *flow.CallContext, _ []*models.Extension, _ int) (*flow.RingResult, error) {
	return nil, nil
}

func (m *mockSIPActions) PlayAndCollect(_ context.Context, callCtx *flow.CallContext, _ string, _ bool, _ int, _ int, _ int) (*flow.CollectResult, error) {
	idx := m.callIdx
	m.callIdx++

	if idx < len(m.collectErrors) && m.collectErrors[idx] != nil {
		return nil, m.collectErrors[idx]
	}
	if idx < len(m.collectResults) {
		return m.collectResults[idx], nil
	}
	// Default: timeout with no digits.
	return &flow.CollectResult{TimedOut: true}, nil
}

func (m *mockSIPActions) RecordMessage(_ context.Context, _ *flow.CallContext, _ string, _ int, _ string) (*flow.RecordResult, error) {
	return &flow.RecordResult{}, nil
}

func (m *mockSIPActions) SendMWI(_ context.Context, _ *models.Extension, _ int, _ int) error {
	return nil
}

func (m *mockSIPActions) HangupCall(_ context.Context, _ *flow.CallContext, _ int, _ string) error {
	return nil
}

func (m *mockSIPActions) BlindTransfer(_ context.Context, _ *flow.CallContext, _ string) error {
	return nil
}

func (m *mockSIPActions) JoinConference(_ context.Context, _ *flow.CallContext, _ *models.ConferenceBridge) error {
	return nil
}

func (m *mockSIPActions) RingFollowMe(_ context.Context, _ *flow.CallContext, _ []models.FollowMeNumber, _ string, _ string, _ bool) (*flow.RingResult, error) {
	return &flow.RingResult{Answered: false}, nil
}

func (m *mockSIPActions) RingFollowMeSimultaneous(_ context.Context, _ *flow.CallContext, _ []models.FollowMeNumber, _ string, _ string, _ bool) (*flow.RingResult, error) {
	return &flow.RingResult{Answered: false}, nil
}

func newTestIVRHandler(menu *models.IVRMenu, sip *mockSIPActions) *IVRMenuHandler {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	resolver := &mockEntityResolver{entity: menu}
	engine := flow.NewEngine(nil, nil, resolver, logger)
	return NewIVRMenuHandler(engine, sip, logger)
}

func makeIVRNode(entityID int64) flow.Node {
	id := entityID
	return flow.Node{
		ID:   "node_ivr",
		Type: "ivr_menu",
		Data: flow.NodeData{
			Label:      "Main Menu",
			EntityID:   &id,
			EntityType: "ivr_menu",
		},
	}
}

func TestIVRMenuDigitMatch(t *testing.T) {
	menu := &models.IVRMenu{
		ID:           1,
		Name:         "Main Menu",
		GreetingFile: "/audio/greeting.wav",
		Timeout:      10,
		MaxRetries:   3,
		DigitTimeout: 3,
		Options:      `{"1":"sales","2":"support","3":"billing"}`,
	}

	sip := &mockSIPActions{
		collectResults: []*flow.CollectResult{
			{Digits: "2"},
		},
	}

	h := newTestIVRHandler(menu, sip)
	callCtx := &flow.CallContext{CallID: "test-ivr-1"}

	edge, err := h.Execute(context.Background(), callCtx, makeIVRNode(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edge != "2" {
		t.Errorf("expected edge %q, got %q", "2", edge)
	}
}

func TestIVRMenuTimeoutAfterRetries(t *testing.T) {
	menu := &models.IVRMenu{
		ID:           1,
		Name:         "Main Menu",
		GreetingFile: "/audio/greeting.wav",
		Timeout:      10,
		MaxRetries:   2,
		DigitTimeout: 3,
		Options:      `{"1":"sales","2":"support"}`,
	}

	// All attempts time out with no digits.
	sip := &mockSIPActions{
		collectResults: []*flow.CollectResult{
			{TimedOut: true},
			{TimedOut: true},
			{TimedOut: true},
		},
	}

	h := newTestIVRHandler(menu, sip)
	callCtx := &flow.CallContext{CallID: "test-ivr-2"}

	edge, err := h.Execute(context.Background(), callCtx, makeIVRNode(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edge != "timeout" {
		t.Errorf("expected edge %q, got %q", "timeout", edge)
	}
}

func TestIVRMenuInvalidDigitAfterRetries(t *testing.T) {
	menu := &models.IVRMenu{
		ID:           1,
		Name:         "Main Menu",
		GreetingFile: "/audio/greeting.wav",
		Timeout:      10,
		MaxRetries:   2,
		DigitTimeout: 3,
		Options:      `{"1":"sales","2":"support"}`,
	}

	// Caller keeps pressing "9" which is not a valid option.
	sip := &mockSIPActions{
		collectResults: []*flow.CollectResult{
			{Digits: "9"},
			{Digits: "9"},
			{Digits: "9"},
		},
	}

	h := newTestIVRHandler(menu, sip)
	callCtx := &flow.CallContext{CallID: "test-ivr-3"}

	edge, err := h.Execute(context.Background(), callCtx, makeIVRNode(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edge != "invalid" {
		t.Errorf("expected edge %q, got %q", "invalid", edge)
	}
}

func TestIVRMenuInvalidThenValidDigit(t *testing.T) {
	menu := &models.IVRMenu{
		ID:           1,
		Name:         "Main Menu",
		GreetingFile: "/audio/greeting.wav",
		Timeout:      10,
		MaxRetries:   3,
		DigitTimeout: 3,
		Options:      `{"1":"sales","2":"support"}`,
	}

	// First attempt invalid, second attempt valid.
	sip := &mockSIPActions{
		collectResults: []*flow.CollectResult{
			{Digits: "5"},
			{Digits: "1"},
		},
	}

	h := newTestIVRHandler(menu, sip)
	callCtx := &flow.CallContext{CallID: "test-ivr-4"}

	edge, err := h.Execute(context.Background(), callCtx, makeIVRNode(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edge != "1" {
		t.Errorf("expected edge %q, got %q", "1", edge)
	}
}

func TestIVRMenuTimeoutThenValidDigit(t *testing.T) {
	menu := &models.IVRMenu{
		ID:           1,
		Name:         "Main Menu",
		GreetingFile: "/audio/greeting.wav",
		Timeout:      10,
		MaxRetries:   3,
		DigitTimeout: 3,
		Options:      `{"1":"sales","2":"support"}`,
	}

	// First attempt times out, second attempt valid.
	sip := &mockSIPActions{
		collectResults: []*flow.CollectResult{
			{TimedOut: true},
			{Digits: "2"},
		},
	}

	h := newTestIVRHandler(menu, sip)
	callCtx := &flow.CallContext{CallID: "test-ivr-5"}

	edge, err := h.Execute(context.Background(), callCtx, makeIVRNode(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edge != "2" {
		t.Errorf("expected edge %q, got %q", "2", edge)
	}
}

func TestIVRMenuTTSFallback(t *testing.T) {
	menu := &models.IVRMenu{
		ID:           1,
		Name:         "TTS Menu",
		GreetingFile: "", // No file — should use TTS.
		GreetingTTS:  "Press 1 for sales, 2 for support",
		Timeout:      10,
		MaxRetries:   3,
		DigitTimeout: 3,
		Options:      `{"1":"sales","2":"support"}`,
	}

	sip := &mockSIPActions{
		collectResults: []*flow.CollectResult{
			{Digits: "1"},
		},
	}

	h := newTestIVRHandler(menu, sip)
	callCtx := &flow.CallContext{CallID: "test-ivr-6"}

	edge, err := h.Execute(context.Background(), callCtx, makeIVRNode(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edge != "1" {
		t.Errorf("expected edge %q, got %q", "1", edge)
	}
}

func TestIVRMenuDefaultTimeoutValues(t *testing.T) {
	// Timeout and digit_timeout are 0 — should use defaults (10 and 3).
	menu := &models.IVRMenu{
		ID:           1,
		Name:         "Default Menu",
		GreetingFile: "/audio/greeting.wav",
		Timeout:      0,
		MaxRetries:   0,
		DigitTimeout: 0,
		Options:      `{"1":"sales"}`,
	}

	sip := &mockSIPActions{
		collectResults: []*flow.CollectResult{
			{Digits: "1"},
		},
	}

	h := newTestIVRHandler(menu, sip)
	callCtx := &flow.CallContext{CallID: "test-ivr-7"}

	edge, err := h.Execute(context.Background(), callCtx, makeIVRNode(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edge != "1" {
		t.Errorf("expected edge %q, got %q", "1", edge)
	}
}

func TestIVRMenuNoEntityError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	resolver := &mockEntityResolver{entity: nil}
	engine := flow.NewEngine(nil, nil, resolver, logger)
	sip := &mockSIPActions{}

	h := NewIVRMenuHandler(engine, sip, logger)
	callCtx := &flow.CallContext{CallID: "test-ivr-8"}

	_, err := h.Execute(context.Background(), callCtx, makeIVRNode(1))
	if err == nil {
		t.Fatal("expected error for missing entity, got nil")
	}
}

func TestIVRMenuStarAndHashOptions(t *testing.T) {
	menu := &models.IVRMenu{
		ID:           1,
		Name:         "Special Keys Menu",
		GreetingFile: "/audio/greeting.wav",
		Timeout:      10,
		MaxRetries:   3,
		DigitTimeout: 3,
		Options:      `{"1":"sales","*":"operator","0":"reception"}`,
	}

	sip := &mockSIPActions{
		collectResults: []*flow.CollectResult{
			{Digits: "*"},
		},
	}

	h := newTestIVRHandler(menu, sip)
	callCtx := &flow.CallContext{CallID: "test-ivr-9"}

	edge, err := h.Execute(context.Background(), callCtx, makeIVRNode(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edge != "*" {
		t.Errorf("expected edge %q, got %q", "*", edge)
	}
}
