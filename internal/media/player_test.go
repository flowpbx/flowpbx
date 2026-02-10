package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// createTestWAV creates a minimal G.711 WAV file with the specified format
// and sample count. Returns the path to the temporary file.
func createTestWAV(t *testing.T, format uint16, sampleRate uint32, channels uint16, bitsPerSample uint16, numSamples int) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.wav")

	// Build WAV data.
	data := make([]byte, numSamples)
	for i := range data {
		data[i] = 0x80 // fill with a non-zero value
	}

	var buf bytes.Buffer

	// fmt chunk.
	var fmtBuf bytes.Buffer
	binary.Write(&fmtBuf, binary.LittleEndian, format)
	binary.Write(&fmtBuf, binary.LittleEndian, channels)
	binary.Write(&fmtBuf, binary.LittleEndian, sampleRate)
	byteRate := sampleRate * uint32(channels) * uint32(bitsPerSample) / 8
	binary.Write(&fmtBuf, binary.LittleEndian, byteRate)
	blockAlign := channels * bitsPerSample / 8
	binary.Write(&fmtBuf, binary.LittleEndian, blockAlign)
	binary.Write(&fmtBuf, binary.LittleEndian, bitsPerSample)

	// data chunk.
	dataChunkSize := uint32(numSamples)

	// Calculate total RIFF size: 4 (WAVE) + 8 (fmt hdr) + fmt size + 8 (data hdr) + data size.
	riffSize := uint32(4 + 8 + fmtBuf.Len() + 8 + len(data))

	// RIFF header.
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, riffSize)
	buf.WriteString("WAVE")

	// fmt chunk.
	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(fmtBuf.Len()))
	buf.Write(fmtBuf.Bytes())

	// data chunk.
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, dataChunkSize)
	buf.Write(data)

	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestParseWAVHeader_ValidPCMU(t *testing.T) {
	path := createTestWAV(t, wavFormatPCMU, 8000, 1, 8, 1600) // 200ms of audio

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	hdr, err := parseWAVHeader(f)
	if err != nil {
		t.Fatal(err)
	}

	if hdr.AudioFormat != wavFormatPCMU {
		t.Errorf("AudioFormat = %d, want %d", hdr.AudioFormat, wavFormatPCMU)
	}
	if hdr.NumChannels != 1 {
		t.Errorf("NumChannels = %d, want 1", hdr.NumChannels)
	}
	if hdr.SampleRate != 8000 {
		t.Errorf("SampleRate = %d, want 8000", hdr.SampleRate)
	}
	if hdr.BitsPerSample != 8 {
		t.Errorf("BitsPerSample = %d, want 8", hdr.BitsPerSample)
	}
	if hdr.DataSize != 1600 {
		t.Errorf("DataSize = %d, want 1600", hdr.DataSize)
	}
}

func TestParseWAVHeader_ValidPCMA(t *testing.T) {
	path := createTestWAV(t, wavFormatPCMA, 8000, 1, 8, 800) // 100ms of audio

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	hdr, err := parseWAVHeader(f)
	if err != nil {
		t.Fatal(err)
	}

	if hdr.AudioFormat != wavFormatPCMA {
		t.Errorf("AudioFormat = %d, want %d", hdr.AudioFormat, wavFormatPCMA)
	}
	if hdr.DataSize != 800 {
		t.Errorf("DataSize = %d, want 800", hdr.DataSize)
	}
}

func TestParseWAVHeader_NotRIFF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.wav")
	os.WriteFile(path, []byte("not a wav file at all"), 0644)

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	_, err = parseWAVHeader(f)
	if err == nil {
		t.Error("expected error for non-RIFF file")
	}
}

func TestValidateWAVFile(t *testing.T) {
	// 8000 samples = 1 second of audio.
	path := createTestWAV(t, wavFormatPCMU, 8000, 1, 8, 8000)

	pt, dur, err := ValidateWAVFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if pt != PayloadPCMU {
		t.Errorf("payload type = %d, want %d", pt, PayloadPCMU)
	}
	if dur != time.Second {
		t.Errorf("duration = %v, want 1s", dur)
	}
}

func TestValidateWAVFile_WrongSampleRate(t *testing.T) {
	path := createTestWAV(t, wavFormatPCMU, 16000, 1, 8, 1600)
	_, _, err := ValidateWAVFile(path)
	if err == nil {
		t.Error("expected error for 16kHz file")
	}
}

func TestValidateWAVFile_Stereo(t *testing.T) {
	path := createTestWAV(t, wavFormatPCMU, 8000, 2, 8, 1600)
	_, _, err := ValidateWAVFile(path)
	if err == nil {
		t.Error("expected error for stereo file")
	}
}

func TestValidateWAVFile_UnsupportedFormat(t *testing.T) {
	path := createTestWAV(t, 1, 8000, 1, 16, 1600) // PCM 16-bit
	_, _, err := ValidateWAVFile(path)
	if err == nil {
		t.Error("expected error for PCM 16-bit format")
	}
}

func TestBuildRTPHeader(t *testing.T) {
	buf := make([]byte, rtpHeaderSize)

	buildRTPHeader(buf, PayloadPCMU, true, 100, 1600, 0x12345678)

	// Version 2, no padding, no extension, no CSRCs.
	if buf[0] != 0x80 {
		t.Errorf("byte 0 = 0x%02x, want 0x80", buf[0])
	}
	// Marker bit set, PT = 0 (PCMU).
	if buf[1] != 0x80 {
		t.Errorf("byte 1 = 0x%02x, want 0x80", buf[1])
	}
	// Sequence number 100.
	seq := binary.BigEndian.Uint16(buf[2:4])
	if seq != 100 {
		t.Errorf("seq = %d, want 100", seq)
	}
	// Timestamp 1600.
	ts := binary.BigEndian.Uint32(buf[4:8])
	if ts != 1600 {
		t.Errorf("ts = %d, want 1600", ts)
	}
	// SSRC.
	ssrc := binary.BigEndian.Uint32(buf[8:12])
	if ssrc != 0x12345678 {
		t.Errorf("ssrc = 0x%08x, want 0x12345678", ssrc)
	}
}

func TestBuildRTPHeader_NoMarker(t *testing.T) {
	buf := make([]byte, rtpHeaderSize)

	buildRTPHeader(buf, PayloadPCMA, false, 200, 3200, 0xAABBCCDD)

	// No marker bit, PT = 8 (PCMA).
	if buf[1] != 0x08 {
		t.Errorf("byte 1 = 0x%02x, want 0x08", buf[1])
	}
}

func TestPlayer_PlayFile(t *testing.T) {
	// Create a short test WAV: 320 samples = 2 packets (40ms).
	path := createTestWAV(t, wavFormatPCMU, 8000, 1, 8, 320)

	// Set up a UDP listener to receive RTP packets.
	listenAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	remoteAddr := listener.LocalAddr().(*net.UDPAddr)

	// Set up sender socket.
	sendAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	sender, err := net.ListenUDP("udp", sendAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	player := NewPlayer(sender, remoteAddr, logger)

	// Play the file.
	result, err := player.PlayFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}

	if result.PacketsSent != 2 {
		t.Errorf("PacketsSent = %d, want 2", result.PacketsSent)
	}

	// Read the received packets.
	buf := make([]byte, maxRTPPacket)
	listener.SetReadDeadline(time.Now().Add(time.Second))

	for i := 0; i < 2; i++ {
		n, _, err := listener.ReadFromUDP(buf)
		if err != nil {
			t.Fatalf("reading packet %d: %v", i, err)
		}

		// Each packet should be RTP header (12) + payload (160).
		if n != rtpHeaderSize+samplesPerPacket {
			t.Errorf("packet %d size = %d, want %d", i, n, rtpHeaderSize+samplesPerPacket)
		}

		// Check version.
		if buf[0]&0xC0 != 0x80 {
			t.Errorf("packet %d: bad rtp version", i)
		}

		// Check payload type.
		pt := int(buf[1] & 0x7F)
		if pt != PayloadPCMU {
			t.Errorf("packet %d: pt = %d, want %d", i, pt, PayloadPCMU)
		}

		// First packet should have marker bit.
		if i == 0 && buf[1]&0x80 == 0 {
			t.Error("first packet missing marker bit")
		}
		if i == 1 && buf[1]&0x80 != 0 {
			t.Error("second packet should not have marker bit")
		}
	}
}

func TestPlayer_PlayFile_Cancelled(t *testing.T) {
	// Create a longer file: 8000 samples = 1 second (50 packets).
	path := createTestWAV(t, wavFormatPCMU, 8000, 1, 8, 8000)

	// Set up UDP sockets.
	listenAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	listener, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	remoteAddr := listener.LocalAddr().(*net.UDPAddr)

	sendAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	sender, err := net.ListenUDP("udp", sendAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	player := NewPlayer(sender, remoteAddr, logger)

	// Cancel after 50ms (should allow ~2-3 packets).
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result, err := player.PlayFile(ctx, path)
	if err == nil {
		t.Error("expected context error")
	}
	if result == nil {
		t.Fatal("expected non-nil result even on cancellation")
	}

	// Should have sent fewer than all 50 packets.
	if result.PacketsSent >= 50 {
		t.Errorf("expected fewer than 50 packets on cancellation, got %d", result.PacketsSent)
	}
}

func TestPlayer_PlayData(t *testing.T) {
	// 160 bytes = exactly 1 packet.
	data := make([]byte, 160)
	for i := range data {
		data[i] = 0x80
	}

	listenAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	listener, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	remoteAddr := listener.LocalAddr().(*net.UDPAddr)

	sendAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	sender, err := net.ListenUDP("udp", sendAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	player := NewPlayer(sender, remoteAddr, logger)

	result, err := player.PlayData(context.Background(), bytes.NewReader(data), PayloadPCMU, 160)
	if err != nil {
		t.Fatal(err)
	}
	if result.PacketsSent != 1 {
		t.Errorf("PacketsSent = %d, want 1", result.PacketsSent)
	}
}

func TestPayloadTypeForWAV(t *testing.T) {
	tests := []struct {
		format  uint16
		want    int
		wantErr bool
	}{
		{wavFormatPCMU, PayloadPCMU, false},
		{wavFormatPCMA, PayloadPCMA, false},
		{1, 0, true},  // PCM
		{3, 0, true},  // IEEE float
		{99, 0, true}, // unknown
	}

	for _, tt := range tests {
		pt, err := payloadTypeForWAV(tt.format)
		if tt.wantErr {
			if err == nil {
				t.Errorf("payloadTypeForWAV(%d): expected error", tt.format)
			}
			continue
		}
		if err != nil {
			t.Errorf("payloadTypeForWAV(%d): unexpected error: %v", tt.format, err)
			continue
		}
		if pt != tt.want {
			t.Errorf("payloadTypeForWAV(%d) = %d, want %d", tt.format, pt, tt.want)
		}
	}
}
