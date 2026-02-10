package media

import (
	"encoding/binary"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecorderBasic(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.wav")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	rec, err := NewRecorder(fp, logger)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	// Feed 160 samples (20ms @ 8kHz) of PCMU silence (u-law silence = 0xFF).
	payload := make([]byte, 160)
	for i := range payload {
		payload[i] = 0xFF
	}

	// Feed several packets.
	for i := 0; i < 50; i++ {
		rec.Feed(payload, PayloadPCMU)
	}

	filePath, duration := rec.Stop()

	if filePath != fp {
		t.Errorf("FilePath = %q, want %q", filePath, fp)
	}

	// 50 packets × 160 samples = 8000 bytes = 1 second.
	if duration != 1 {
		t.Errorf("duration = %d, want 1", duration)
	}

	// Verify the WAV file is valid.
	data, err := os.ReadFile(fp)
	if err != nil {
		t.Fatalf("reading recording: %v", err)
	}

	if len(data) < wavHeaderSize {
		t.Fatalf("file too small: %d bytes", len(data))
	}

	// Check RIFF header.
	if string(data[0:4]) != "RIFF" {
		t.Error("missing RIFF marker")
	}
	if string(data[8:12]) != "WAVE" {
		t.Error("missing WAVE marker")
	}

	// Check audio format (G.711 u-law = 7).
	audioFormat := binary.LittleEndian.Uint16(data[20:22])
	if audioFormat != wavFormatPCMU {
		t.Errorf("audio format = %d, want %d", audioFormat, wavFormatPCMU)
	}

	// Check data size in header matches actual data.
	dataSize := binary.LittleEndian.Uint32(data[40:44])
	actualData := uint32(len(data) - wavHeaderSize)
	if dataSize != actualData {
		t.Errorf("header data size = %d, actual = %d", dataSize, actualData)
	}

	// Total data should be 8000 bytes (50 × 160).
	if actualData != 8000 {
		t.Errorf("total data = %d, want 8000", actualData)
	}
}

func TestRecorderPCMA(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test_pcma.wav")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	rec, err := NewRecorder(fp, logger)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	// Feed PCMA payload — should be transcoded to PCMU in the WAV.
	payload := make([]byte, 160)
	for i := range payload {
		payload[i] = 0xD5 // a-law silence
	}

	for i := 0; i < 10; i++ {
		rec.Feed(payload, PayloadPCMA)
	}

	_, duration := rec.Stop()

	// 10 × 160 = 1600 bytes, which is 0 seconds (integer division of 1600/8000).
	if duration != 0 {
		t.Errorf("duration = %d, want 0", duration)
	}

	data, err := os.ReadFile(fp)
	if err != nil {
		t.Fatalf("reading recording: %v", err)
	}

	actualData := len(data) - wavHeaderSize
	if actualData != 1600 {
		t.Errorf("total data = %d, want 1600", actualData)
	}
}

func TestRecorderDropsOnFullChannel(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test_drop.wav")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	rec, err := NewRecorder(fp, logger)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	// Feed should not block even if we send many packets rapidly.
	payload := make([]byte, 160)
	for i := 0; i < 1000; i++ {
		rec.Feed(payload, PayloadPCMU)
	}

	// Should not hang.
	rec.Stop()
}

func TestRecorderEmptyPayload(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test_empty.wav")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	rec, err := NewRecorder(fp, logger)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	// Empty payload should be ignored.
	rec.Feed(nil, PayloadPCMU)
	rec.Feed([]byte{}, PayloadPCMU)

	_, duration := rec.Stop()
	if duration != 0 {
		t.Errorf("duration = %d, want 0", duration)
	}
}

func TestRecorderDoubleStop(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test_dblstop.wav")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	rec, err := NewRecorder(fp, logger)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	rec.Stop()
	// Second stop should not panic.
	rec.Stop()
}

func TestRecorderCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "a", "b", "c", "test.wav")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	rec, err := NewRecorder(fp, logger)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}
	rec.Stop()

	if _, err := os.Stat(fp); err != nil {
		t.Errorf("recording file not created: %v", err)
	}
}

func TestRecordingPath(t *testing.T) {
	ts := time.Date(2025, 3, 15, 10, 30, 0, 0, time.UTC)
	got := RecordingPath("/data", "abc-123", ts)
	want := "/data/recordings/2025/03/15/call_abc-123.wav"
	if got != want {
		t.Errorf("RecordingPath = %q, want %q", got, want)
	}
}
