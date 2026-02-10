package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/smtp"
	"os"
	"strings"
	"testing"
	"time"
)

// mockSMTPClient implements smtpClient for testing.
type mockSMTPClient struct {
	helloCalled  bool
	tlsCalled    bool
	authCalled   bool
	mailFrom     string
	rcptTo       string
	dataWritten  []byte
	quitCalled   bool
	closeCalled  bool
	authErr      error
	mailErr      error
	rcptErr      error
	dataErr      error
	dataWriteErr error
}

func (m *mockSMTPClient) Hello(_ string) error { m.helloCalled = true; return nil }
func (m *mockSMTPClient) Extension(ext string) (bool, string) {
	if ext == "STARTTLS" {
		return true, ""
	}
	return false, ""
}
func (m *mockSMTPClient) StartTLS(_ *tls.Config) error { m.tlsCalled = true; return nil }
func (m *mockSMTPClient) Auth(_ smtp.Auth) error {
	m.authCalled = true
	return m.authErr
}
func (m *mockSMTPClient) Mail(from string) error {
	m.mailFrom = from
	return m.mailErr
}
func (m *mockSMTPClient) Rcpt(to string) error {
	m.rcptTo = to
	return m.rcptErr
}
func (m *mockSMTPClient) Data() (io.WriteCloser, error) {
	if m.dataErr != nil {
		return nil, m.dataErr
	}
	return &mockWriteCloser{mock: m}, nil
}
func (m *mockSMTPClient) Quit() error  { m.quitCalled = true; return nil }
func (m *mockSMTPClient) Close() error { m.closeCalled = true; return nil }

type mockWriteCloser struct {
	mock *mockSMTPClient
}

func (w *mockWriteCloser) Write(p []byte) (int, error) {
	if w.mock.dataWriteErr != nil {
		return 0, w.mock.dataWriteErr
	}
	w.mock.dataWritten = append(w.mock.dataWritten, p...)
	return len(p), nil
}

func (w *mockWriteCloser) Close() error { return nil }

func newTestSender(mock *mockSMTPClient) *Sender {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	s := NewSender(logger)
	s.dialFunc = func(_ string, _ *tls.Config, _ string) (smtpClient, error) {
		return mock, nil
	}
	return s
}

func TestSendVoicemailNotificationPlainText(t *testing.T) {
	mock := &mockSMTPClient{}
	sender := newTestSender(mock)

	cfg := SMTPConfig{
		Host:     "mail.example.com",
		Port:     "587",
		From:     "pbx@example.com",
		Username: "user",
		Password: "pass",
		TLS:      "starttls",
	}

	notif := VoicemailNotification{
		To:           "admin@example.com",
		BoxName:      "Sales Voicemail",
		CallerIDName: "John Doe",
		CallerIDNum:  "+61400000000",
		Timestamp:    time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC),
		DurationSecs: 45,
		AttachAudio:  false, // no attachment
	}

	err := sender.SendVoicemailNotification(context.Background(), cfg, notif)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !mock.helloCalled {
		t.Error("expected Hello to be called")
	}
	if !mock.tlsCalled {
		t.Error("expected StartTLS to be called")
	}
	if !mock.authCalled {
		t.Error("expected Auth to be called")
	}
	if mock.mailFrom != "pbx@example.com" {
		t.Errorf("expected mail from %q, got %q", "pbx@example.com", mock.mailFrom)
	}
	if mock.rcptTo != "admin@example.com" {
		t.Errorf("expected rcpt to %q, got %q", "admin@example.com", mock.rcptTo)
	}
	if !mock.quitCalled {
		t.Error("expected Quit to be called")
	}

	body := string(mock.dataWritten)
	if !strings.Contains(body, "Subject: New voicemail from John Doe <+61400000000>") {
		t.Errorf("expected subject line in email body, got:\n%s", body)
	}
	if !strings.Contains(body, "Sales Voicemail") {
		t.Errorf("expected box name in email body, got:\n%s", body)
	}
	if !strings.Contains(body, "45s") {
		t.Errorf("expected duration in email body, got:\n%s", body)
	}
	if strings.Contains(body, "multipart/mixed") {
		t.Error("expected plain text email, got multipart")
	}
}

func TestSendVoicemailNotificationWithAttachment(t *testing.T) {
	mock := &mockSMTPClient{}
	sender := newTestSender(mock)

	// Create a temporary WAV file to attach.
	tmpDir := t.TempDir()
	wavFile := tmpDir + "/msg_001.wav"
	if err := os.WriteFile(wavFile, []byte("RIFF-fake-wav-data"), 0640); err != nil {
		t.Fatalf("failed to create test WAV file: %v", err)
	}

	cfg := SMTPConfig{
		Host: "mail.example.com",
		Port: "587",
		From: "pbx@example.com",
		TLS:  "none",
	}

	notif := VoicemailNotification{
		To:           "admin@example.com",
		BoxName:      "Main Voicemail",
		CallerIDName: "",
		CallerIDNum:  "0400000000",
		Timestamp:    time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC),
		DurationSecs: 125,
		AudioFile:    wavFile,
		AttachAudio:  true,
	}

	err := sender.SendVoicemailNotification(context.Background(), cfg, notif)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := string(mock.dataWritten)
	if !strings.Contains(body, "multipart/mixed") {
		t.Error("expected multipart email with attachment")
	}
	if !strings.Contains(body, "audio/wav") {
		t.Error("expected audio/wav content type in attachment")
	}
	if !strings.Contains(body, "msg_001.wav") {
		t.Error("expected filename in attachment headers")
	}
	if !strings.Contains(body, "Content-Transfer-Encoding: base64") {
		t.Error("expected base64 content transfer encoding")
	}
	// No auth called since no username/password.
	if mock.authCalled {
		t.Error("expected no Auth call when credentials are empty")
	}
}

func TestSendVoicemailNotificationNoSMTPConfig(t *testing.T) {
	mock := &mockSMTPClient{}
	sender := newTestSender(mock)

	cfg := SMTPConfig{} // empty config
	notif := VoicemailNotification{To: "admin@example.com"}

	err := sender.SendVoicemailNotification(context.Background(), cfg, notif)
	if err == nil {
		t.Fatal("expected error for empty SMTP config")
	}
	if !strings.Contains(err.Error(), "smtp not configured") {
		t.Errorf("expected 'smtp not configured' error, got: %v", err)
	}
}

func TestSendVoicemailNotificationNoRecipient(t *testing.T) {
	mock := &mockSMTPClient{}
	sender := newTestSender(mock)

	cfg := SMTPConfig{Host: "mail.example.com", Port: "587", From: "pbx@example.com"}
	notif := VoicemailNotification{To: ""} // no recipient

	err := sender.SendVoicemailNotification(context.Background(), cfg, notif)
	if err == nil {
		t.Fatal("expected error for empty recipient")
	}
	if !strings.Contains(err.Error(), "no recipient") {
		t.Errorf("expected 'no recipient' error, got: %v", err)
	}
}

func TestSendVoicemailNotificationAuthError(t *testing.T) {
	mock := &mockSMTPClient{authErr: fmt.Errorf("invalid credentials")}
	sender := newTestSender(mock)

	cfg := SMTPConfig{
		Host:     "mail.example.com",
		Port:     "587",
		From:     "pbx@example.com",
		Username: "user",
		Password: "wrong",
		TLS:      "none",
	}

	notif := VoicemailNotification{
		To:      "admin@example.com",
		BoxName: "Test",
	}

	err := sender.SendVoicemailNotification(context.Background(), cfg, notif)
	if err == nil {
		t.Fatal("expected auth error")
	}
	if !strings.Contains(err.Error(), "smtp auth") {
		t.Errorf("expected 'smtp auth' error, got: %v", err)
	}
}

func TestSendVoicemailNotificationMissingAudioFile(t *testing.T) {
	mock := &mockSMTPClient{}
	sender := newTestSender(mock)

	cfg := SMTPConfig{
		Host: "mail.example.com",
		Port: "587",
		From: "pbx@example.com",
		TLS:  "none",
	}

	notif := VoicemailNotification{
		To:          "admin@example.com",
		BoxName:     "Test",
		AudioFile:   "/nonexistent/path/msg.wav",
		AttachAudio: true,
	}

	err := sender.SendVoicemailNotification(context.Background(), cfg, notif)
	if err == nil {
		t.Fatal("expected error for missing audio file")
	}
	if !strings.Contains(err.Error(), "reading audio file") {
		t.Errorf("expected 'reading audio file' error, got: %v", err)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		secs     int
		expected string
	}{
		{0, "0s"},
		{5, "5s"},
		{59, "59s"},
		{60, "1m"},
		{61, "1m 1s"},
		{125, "2m 5s"},
		{3600, "60m"},
	}

	for _, tc := range tests {
		result := formatDuration(tc.secs)
		if result != tc.expected {
			t.Errorf("formatDuration(%d) = %q, want %q", tc.secs, result, tc.expected)
		}
	}
}

func TestSMTPConfigValid(t *testing.T) {
	tests := []struct {
		name  string
		cfg   SMTPConfig
		valid bool
	}{
		{"full config", SMTPConfig{Host: "mail.example.com", Port: "587", From: "test@example.com"}, true},
		{"missing host", SMTPConfig{Port: "587", From: "test@example.com"}, false},
		{"missing port", SMTPConfig{Host: "mail.example.com", From: "test@example.com"}, false},
		{"missing from", SMTPConfig{Host: "mail.example.com", Port: "587"}, false},
		{"empty", SMTPConfig{}, false},
	}

	for _, tc := range tests {
		if tc.cfg.Valid() != tc.valid {
			t.Errorf("%s: expected Valid() = %v", tc.name, tc.valid)
		}
	}
}

func TestCallerDisplayFormat(t *testing.T) {
	mock := &mockSMTPClient{}
	sender := newTestSender(mock)

	cfg := SMTPConfig{Host: "mail.example.com", Port: "587", From: "pbx@example.com", TLS: "none"}

	// Test with only caller number (no name).
	notif := VoicemailNotification{
		To:           "admin@example.com",
		BoxName:      "Test",
		CallerIDNum:  "0400000000",
		Timestamp:    time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC),
		DurationSecs: 10,
		AttachAudio:  false,
	}

	err := sender.SendVoicemailNotification(context.Background(), cfg, notif)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := string(mock.dataWritten)
	if !strings.Contains(body, "Subject: New voicemail from 0400000000") {
		t.Errorf("expected caller number only in subject, got:\n%s", body)
	}
}
