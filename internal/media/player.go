package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"os"
	"time"
)

// WAV format codes for G.711 codecs.
const (
	wavFormatPCMU = 7 // G.711 u-law (PCMU)
	wavFormatPCMA = 6 // G.711 a-law (PCMA)

	// samplesPerPacket is the number of audio samples per RTP packet.
	// At 8 kHz sample rate with 20ms ptime, each packet carries 160 samples.
	// For G.711, each sample is 1 byte, so each packet payload is 160 bytes.
	samplesPerPacket = 160

	// packetDuration is the duration of one RTP packet (20ms at 8kHz).
	packetDuration = 20 * time.Millisecond

	// rtpHeaderSize is the fixed RTP header size (no CSRCs, no extensions).
	rtpHeaderSize = 12

	// rtpVersion is the RTP protocol version (always 2).
	rtpVersion = 2

	// timestampIncrement is the RTP timestamp increment per packet.
	// At 8 kHz clock rate with 20ms ptime: 8000 * 0.020 = 160.
	timestampIncrement = 160
)

// wavHeader holds the parsed fields from a WAV file header that we need
// for audio playback validation.
type wavHeader struct {
	AudioFormat   uint16 // 6 = A-law, 7 = u-law
	NumChannels   uint16
	SampleRate    uint32
	ByteRate      uint32
	BlockAlign    uint16
	BitsPerSample uint16
	DataSize      uint32 // size of the "data" chunk in bytes
}

// parseWAVHeader reads and validates a WAV file header, returning the
// format information and positioning the reader at the start of audio data.
func parseWAVHeader(r io.ReadSeeker) (*wavHeader, error) {
	// RIFF header: "RIFF" + size + "WAVE"
	var riffHeader [12]byte
	if _, err := io.ReadFull(r, riffHeader[:]); err != nil {
		return nil, fmt.Errorf("reading riff header: %w", err)
	}
	if string(riffHeader[0:4]) != "RIFF" {
		return nil, errors.New("not a RIFF file")
	}
	if string(riffHeader[8:12]) != "WAVE" {
		return nil, errors.New("not a WAVE file")
	}

	// Walk chunks to find "fmt " and "data".
	hdr := &wavHeader{}
	foundFmt := false
	foundData := false

	for !foundData {
		var chunkID [4]byte
		var chunkSize uint32

		if _, err := io.ReadFull(r, chunkID[:]); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return nil, fmt.Errorf("reading chunk id: %w", err)
		}
		if err := binary.Read(r, binary.LittleEndian, &chunkSize); err != nil {
			return nil, fmt.Errorf("reading chunk size: %w", err)
		}

		switch string(chunkID[:]) {
		case "fmt ":
			if chunkSize < 16 {
				return nil, fmt.Errorf("fmt chunk too small: %d bytes", chunkSize)
			}
			if err := binary.Read(r, binary.LittleEndian, &hdr.AudioFormat); err != nil {
				return nil, fmt.Errorf("reading audio format: %w", err)
			}
			if err := binary.Read(r, binary.LittleEndian, &hdr.NumChannels); err != nil {
				return nil, fmt.Errorf("reading num channels: %w", err)
			}
			if err := binary.Read(r, binary.LittleEndian, &hdr.SampleRate); err != nil {
				return nil, fmt.Errorf("reading sample rate: %w", err)
			}
			if err := binary.Read(r, binary.LittleEndian, &hdr.ByteRate); err != nil {
				return nil, fmt.Errorf("reading byte rate: %w", err)
			}
			if err := binary.Read(r, binary.LittleEndian, &hdr.BlockAlign); err != nil {
				return nil, fmt.Errorf("reading block align: %w", err)
			}
			if err := binary.Read(r, binary.LittleEndian, &hdr.BitsPerSample); err != nil {
				return nil, fmt.Errorf("reading bits per sample: %w", err)
			}
			// Skip any extra fmt bytes.
			if chunkSize > 16 {
				if _, err := r.Seek(int64(chunkSize-16), io.SeekCurrent); err != nil {
					return nil, fmt.Errorf("skipping extra fmt data: %w", err)
				}
			}
			foundFmt = true

		case "data":
			hdr.DataSize = chunkSize
			foundData = true
			// Reader is now positioned at the start of audio data.

		default:
			// Skip unknown chunks. Pad to even boundary per WAV spec.
			skip := int64(chunkSize)
			if chunkSize%2 != 0 {
				skip++
			}
			if _, err := r.Seek(skip, io.SeekCurrent); err != nil {
				return nil, fmt.Errorf("skipping chunk %q: %w", string(chunkID[:]), err)
			}
		}
	}

	if !foundFmt {
		return nil, errors.New("wav file missing fmt chunk")
	}
	if !foundData {
		return nil, errors.New("wav file missing data chunk")
	}

	return hdr, nil
}

// payloadTypeForWAV maps a WAV audio format code to its RTP payload type.
// Returns an error if the format is not a supported G.711 variant.
func payloadTypeForWAV(format uint16) (int, error) {
	switch format {
	case wavFormatPCMU:
		return PayloadPCMU, nil
	case wavFormatPCMA:
		return PayloadPCMA, nil
	default:
		return 0, fmt.Errorf("unsupported wav format %d: only G.711 a-law (6) and u-law (7) are supported", format)
	}
}

// ValidateWAVFile opens a WAV file and validates it is in a supported
// G.711 format (alaw or ulaw, 8kHz, mono, 8-bit). Returns the payload
// type and duration, or an error if the file is invalid.
func ValidateWAVFile(path string) (payloadType int, duration time.Duration, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, fmt.Errorf("opening wav file: %w", err)
	}
	defer f.Close()

	hdr, err := parseWAVHeader(f)
	if err != nil {
		return 0, 0, fmt.Errorf("parsing wav header: %w", err)
	}

	pt, err := payloadTypeForWAV(hdr.AudioFormat)
	if err != nil {
		return 0, 0, err
	}

	if hdr.NumChannels != 1 {
		return 0, 0, fmt.Errorf("wav file must be mono, got %d channels", hdr.NumChannels)
	}
	if hdr.SampleRate != 8000 {
		return 0, 0, fmt.Errorf("wav file must be 8000 Hz, got %d Hz", hdr.SampleRate)
	}
	if hdr.BitsPerSample != 8 {
		return 0, 0, fmt.Errorf("wav file must be 8-bit, got %d-bit", hdr.BitsPerSample)
	}

	// Duration = total samples / sample rate. For 8-bit G.711, 1 byte = 1 sample.
	dur := time.Duration(hdr.DataSize) * time.Second / time.Duration(hdr.SampleRate)

	return pt, dur, nil
}

// ValidateWAVData validates in-memory WAV data is in a supported G.711
// format (alaw or ulaw, 8kHz, mono, 8-bit). Returns an error describing
// the validation failure, or nil if the data is valid.
func ValidateWAVData(data []byte) error {
	r := bytes.NewReader(data)

	hdr, err := parseWAVHeader(r)
	if err != nil {
		return fmt.Errorf("invalid wav: %w", err)
	}

	if _, err := payloadTypeForWAV(hdr.AudioFormat); err != nil {
		return err
	}
	if hdr.NumChannels != 1 {
		return fmt.Errorf("wav file must be mono, got %d channels", hdr.NumChannels)
	}
	if hdr.SampleRate != 8000 {
		return fmt.Errorf("wav file must be 8000 Hz, got %d Hz", hdr.SampleRate)
	}
	if hdr.BitsPerSample != 8 {
		return fmt.Errorf("wav file must be 8-bit, got %d-bit", hdr.BitsPerSample)
	}

	return nil
}

// buildRTPHeader writes a 12-byte RTP header into buf.
// marker should be true for the first packet of a talkspurt.
func buildRTPHeader(buf []byte, pt int, marker bool, seq uint16, ts uint32, ssrc uint32) {
	// Byte 0: V=2, P=0, X=0, CC=0 → 0x80
	buf[0] = rtpVersion << 6
	// Byte 1: M + PT
	buf[1] = byte(pt & 0x7F)
	if marker {
		buf[1] |= 0x80
	}
	// Bytes 2-3: sequence number (big-endian)
	binary.BigEndian.PutUint16(buf[2:4], seq)
	// Bytes 4-7: timestamp (big-endian)
	binary.BigEndian.PutUint32(buf[4:8], ts)
	// Bytes 8-11: SSRC (big-endian)
	binary.BigEndian.PutUint32(buf[8:12], ssrc)
}

// Player streams an audio file as RTP packets to a remote endpoint.
// It reads G.711 WAV files, packetizes them into 20ms RTP packets,
// and sends them with proper timing to maintain real-time playback.
type Player struct {
	conn   *net.UDPConn
	remote *net.UDPAddr
	logger *slog.Logger

	ssrc uint32
	seq  uint16
	ts   uint32
}

// NewPlayer creates an audio player that sends RTP packets from the
// given UDP connection to the specified remote address.
func NewPlayer(conn *net.UDPConn, remote *net.UDPAddr, logger *slog.Logger) *Player {
	return &Player{
		conn:   conn,
		remote: remote,
		logger: logger.With("subsystem", "audio-player"),
		ssrc:   rand.Uint32(),
		seq:    uint16(rand.UintN(65536)),
		ts:     rand.Uint32(),
	}
}

// PlayResult holds the outcome of an audio playback operation.
type PlayResult struct {
	// PacketsSent is the number of RTP packets transmitted.
	PacketsSent int
	// Duration is the actual playback duration.
	Duration time.Duration
}

// PlayFile reads a G.711 WAV file and streams it as RTP to the remote endpoint.
// The context can be used to cancel playback early (e.g., when DTMF is detected).
// Returns the number of packets sent and the playback duration.
func (p *Player) PlayFile(ctx context.Context, path string) (*PlayResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening audio file: %w", err)
	}
	defer f.Close()

	hdr, err := parseWAVHeader(f)
	if err != nil {
		return nil, fmt.Errorf("parsing wav header: %w", err)
	}

	pt, err := payloadTypeForWAV(hdr.AudioFormat)
	if err != nil {
		return nil, err
	}

	if hdr.NumChannels != 1 {
		return nil, fmt.Errorf("wav file must be mono, got %d channels", hdr.NumChannels)
	}
	if hdr.SampleRate != 8000 {
		return nil, fmt.Errorf("wav file must be 8000 Hz, got %d Hz", hdr.SampleRate)
	}
	if hdr.BitsPerSample != 8 {
		return nil, fmt.Errorf("wav file must be 8-bit, got %d-bit", hdr.BitsPerSample)
	}

	p.logger.Info("playing audio file",
		"path", path,
		"format", hdr.AudioFormat,
		"payload_type", pt,
		"data_bytes", hdr.DataSize,
	)

	return p.streamAudio(ctx, f, pt, hdr.DataSize)
}

// PlayData streams raw G.711 audio data (already decoded from WAV) as RTP
// to the remote endpoint. payloadType must be PayloadPCMU or PayloadPCMA.
// dataSize is the number of bytes to read from r.
func (p *Player) PlayData(ctx context.Context, r io.Reader, payloadType int, dataSize uint32) (*PlayResult, error) {
	if payloadType != PayloadPCMU && payloadType != PayloadPCMA {
		return nil, fmt.Errorf("unsupported payload type %d for playback", payloadType)
	}
	return p.streamAudio(ctx, r, payloadType, dataSize)
}

// streamAudio reads audio samples from r and sends them as RTP packets
// with 20ms pacing. Each packet carries 160 bytes (160 samples at 8kHz).
func (p *Player) streamAudio(ctx context.Context, r io.Reader, pt int, dataSize uint32) (*PlayResult, error) {
	pkt := make([]byte, rtpHeaderSize+samplesPerPacket)
	sent := 0
	remaining := dataSize
	start := time.Now()
	marker := true // First packet gets the marker bit.

	for remaining > 0 {
		// Check for cancellation.
		select {
		case <-ctx.Done():
			p.logger.Info("playback cancelled",
				"packets_sent", sent,
				"remaining_bytes", remaining,
			)
			return &PlayResult{
				PacketsSent: sent,
				Duration:    time.Since(start),
			}, ctx.Err()
		default:
		}

		// Read up to one packet's worth of samples.
		toRead := uint32(samplesPerPacket)
		if remaining < toRead {
			toRead = remaining
		}

		n, err := io.ReadFull(r, pkt[rtpHeaderSize:rtpHeaderSize+toRead])
		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("reading audio data: %w", err)
		}

		if n == 0 {
			break
		}

		// If we read fewer than 160 bytes (end of file), pad with silence.
		// G.711 u-law silence = 0xFF, G.711 a-law silence = 0xD5.
		if n < samplesPerPacket {
			silence := byte(0xFF) // u-law silence
			if pt == PayloadPCMA {
				silence = 0xD5 // a-law silence
			}
			for i := rtpHeaderSize + n; i < rtpHeaderSize+samplesPerPacket; i++ {
				pkt[i] = silence
			}
		}

		// Build RTP header.
		buildRTPHeader(pkt[:rtpHeaderSize], pt, marker, p.seq, p.ts, p.ssrc)
		marker = false // Only first packet is marked.

		// Send the packet.
		if _, err := p.conn.WriteToUDP(pkt, p.remote); err != nil {
			return nil, fmt.Errorf("sending rtp packet: %w", err)
		}

		sent++
		p.seq++
		p.ts += timestampIncrement
		remaining -= uint32(n)

		// Pace packets at 20ms intervals. Use wall-clock timing to avoid
		// drift from processing overhead.
		elapsed := time.Since(start)
		expected := time.Duration(sent) * packetDuration
		if sleep := expected - elapsed; sleep > 0 {
			time.Sleep(sleep)
		}
	}

	duration := time.Since(start)
	p.logger.Info("playback complete",
		"packets_sent", sent,
		"duration", duration,
	)

	return &PlayResult{
		PacketsSent: sent,
		Duration:    duration,
	}, nil
}
