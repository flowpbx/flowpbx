package voicemail

import (
	"context"
	"encoding/binary"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteWAVHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.wav")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := writeWAVHeader(f, wavFormatPCMU, 8000); err != nil {
		t.Fatal(err)
	}
	f.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) != wavHeaderSize {
		t.Fatalf("expected header size %d, got %d", wavHeaderSize, len(data))
	}

	// Verify RIFF header.
	if string(data[0:4]) != "RIFF" {
		t.Error("missing RIFF marker")
	}
	if string(data[8:12]) != "WAVE" {
		t.Error("missing WAVE marker")
	}

	// Verify file size field: total file size - 8.
	fileSize := binary.LittleEndian.Uint32(data[4:8])
	if fileSize != wavHeaderSize-8+8000 {
		t.Errorf("expected file size %d, got %d", wavHeaderSize-8+8000, fileSize)
	}

	// Verify fmt chunk.
	if string(data[12:16]) != "fmt " {
		t.Error("missing fmt chunk")
	}
	audioFormat := binary.LittleEndian.Uint16(data[20:22])
	if audioFormat != wavFormatPCMU {
		t.Errorf("expected format %d, got %d", wavFormatPCMU, audioFormat)
	}
	channels := binary.LittleEndian.Uint16(data[22:24])
	if channels != 1 {
		t.Errorf("expected 1 channel, got %d", channels)
	}
	sampleRate := binary.LittleEndian.Uint32(data[24:28])
	if sampleRate != 8000 {
		t.Errorf("expected sample rate 8000, got %d", sampleRate)
	}
	bitsPerSample := binary.LittleEndian.Uint16(data[34:36])
	if bitsPerSample != 8 {
		t.Errorf("expected 8 bits per sample, got %d", bitsPerSample)
	}

	// Verify data chunk.
	if string(data[36:40]) != "data" {
		t.Error("missing data chunk")
	}
	dataSize := binary.LittleEndian.Uint32(data[40:44])
	if dataSize != 8000 {
		t.Errorf("expected data size 8000, got %d", dataSize)
	}
}

func TestWavFormatForPayload(t *testing.T) {
	tests := []struct {
		pt      int
		want    uint16
		wantErr bool
	}{
		{payloadPCMU, wavFormatPCMU, false},
		{payloadPCMA, wavFormatPCMA, false},
		{111, 0, true}, // unsupported
	}

	for _, tt := range tests {
		got, err := wavFormatForPayload(tt.pt)
		if tt.wantErr {
			if err == nil {
				t.Errorf("wavFormatForPayload(%d) expected error", tt.pt)
			}
			continue
		}
		if err != nil {
			t.Errorf("wavFormatForPayload(%d) unexpected error: %v", tt.pt, err)
			continue
		}
		if got != tt.want {
			t.Errorf("wavFormatForPayload(%d) = %d, want %d", tt.pt, got, tt.want)
		}
	}
}

// buildTestRTPPacket creates a minimal RTP packet with the given payload type
// and audio payload data.
func buildTestRTPPacket(pt int, seq uint16, ts uint32, payload []byte) []byte {
	pkt := make([]byte, minRTPHeader+len(payload))
	pkt[0] = 0x80 // V=2, P=0, X=0, CC=0
	pkt[1] = byte(pt & 0x7F)
	binary.BigEndian.PutUint16(pkt[2:4], seq)
	binary.BigEndian.PutUint32(pkt[4:8], ts)
	binary.BigEndian.PutUint32(pkt[8:12], 0x12345678) // SSRC
	copy(pkt[minRTPHeader:], payload)
	return pkt
}

func TestRecordCapturesRTP(t *testing.T) {
	// Set up a UDP listener to act as the RTP receiver.
	listenAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	conn, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	// Create a sender connection.
	senderConn, err := net.DialUDP("udp", nil, localAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer senderConn.Close()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test_recording.wav")

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	recorder := NewRecorder(conn, logger)

	// Send RTP packets in a goroutine.
	go func() {
		time.Sleep(50 * time.Millisecond) // Let the recorder start.
		payload := make([]byte, 160)      // 20ms of G.711 at 8kHz
		for i := range payload {
			payload[i] = 0xFF // u-law silence
		}

		for seq := uint16(0); seq < 10; seq++ {
			pkt := buildTestRTPPacket(payloadPCMU, seq, uint32(seq)*160, payload)
			senderConn.Write(pkt)
			time.Sleep(20 * time.Millisecond)
		}
	}()

	// Record with a short max duration.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	result, err := recorder.Record(ctx, filePath, payloadPCMU, 2)
	if err != nil {
		t.Fatal(err)
	}

	if result.PacketsReceived == 0 {
		t.Error("expected to receive some packets")
	}

	// Verify the WAV file was created and has a valid header.
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) < wavHeaderSize {
		t.Fatalf("file too small: %d bytes", len(data))
	}

	if string(data[0:4]) != "RIFF" {
		t.Error("missing RIFF marker in output")
	}
	if string(data[8:12]) != "WAVE" {
		t.Error("missing WAVE marker in output")
	}

	// Check that we got some audio data beyond the header.
	audioFormat := binary.LittleEndian.Uint16(data[20:22])
	if audioFormat != wavFormatPCMU {
		t.Errorf("expected u-law format %d, got %d", wavFormatPCMU, audioFormat)
	}

	dataSize := binary.LittleEndian.Uint32(data[40:44])
	if dataSize == 0 {
		t.Error("expected non-zero data size in WAV header")
	}

	// Data size in header should match actual data written.
	actualData := len(data) - wavHeaderSize
	if uint32(actualData) != dataSize {
		t.Errorf("header data size %d != actual data %d", dataSize, actualData)
	}
}

func TestRecordMaxDuration(t *testing.T) {
	listenAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	conn, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	senderConn, err := net.DialUDP("udp", nil, localAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer senderConn.Close()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test_maxdur.wav")

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	recorder := NewRecorder(conn, logger)

	// Send continuous RTP packets.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(50 * time.Millisecond)
		payload := make([]byte, 160)
		seq := uint16(0)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			pkt := buildTestRTPPacket(payloadPCMU, seq, uint32(seq)*160, payload)
			senderConn.Write(pkt)
			seq++
			time.Sleep(20 * time.Millisecond)
		}
	}()

	// Record with a 1-second max duration.
	result, err := recorder.Record(context.Background(), filePath, payloadPCMU, 1)
	cancel()
	if err != nil {
		t.Fatal(err)
	}

	// Should have stopped around 1 second.
	if result.DurationSecs > 2 {
		t.Errorf("expected ~1 second recording, got %d seconds", result.DurationSecs)
	}
}

func TestRecordRejectsInvalidPayload(t *testing.T) {
	listenAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	conn, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	recorder := NewRecorder(conn, logger)

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test_invalid.wav")

	_, err = recorder.Record(context.Background(), filePath, 111, 10)
	if err == nil {
		t.Error("expected error for unsupported payload type")
	}
}
