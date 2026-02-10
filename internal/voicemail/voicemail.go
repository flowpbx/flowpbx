// Package voicemail provides RTP-to-WAV recording for the voicemail system.
// It captures incoming RTP packets from a caller's media stream, extracts
// G.711 audio payloads, and writes them to a standard WAV file.
package voicemail

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"
)

const (
	// WAV format codes for G.711 codecs.
	wavFormatPCMU = 7 // G.711 u-law
	wavFormatPCMA = 6 // G.711 a-law

	// RTP payload types for G.711 codecs.
	payloadPCMU = 0
	payloadPCMA = 8

	// minRTPHeader is the minimum RTP header size (12 bytes, no CSRCs).
	minRTPHeader = 12

	// maxRTPPacket is the maximum UDP packet size we handle.
	maxRTPPacket = 1500

	// readTimeout is the read deadline for UDP sockets. This allows the
	// recording goroutine to periodically check for cancellation.
	readTimeout = 100 * time.Millisecond

	// silenceTimeout is the duration of consecutive silence/no-packets
	// before recording is automatically stopped.
	silenceTimeout = 5 * time.Second

	// wavHeaderSize is the size of the WAV file header we write.
	wavHeaderSize = 44
)

// Recorder captures incoming RTP audio from a UDP connection and writes
// it to a WAV file. It handles RTP payload extraction, silence detection,
// and max duration enforcement.
type Recorder struct {
	conn   *net.UDPConn
	logger *slog.Logger
}

// NewRecorder creates a recorder that reads RTP packets from the given
// UDP connection. The connection should be the caller-leg RTP socket
// from the media session.
func NewRecorder(conn *net.UDPConn, logger *slog.Logger) *Recorder {
	return &Recorder{
		conn:   conn,
		logger: logger.With("subsystem", "voicemail-recorder"),
	}
}

// RecordResult holds the outcome of a recording operation.
type RecordResult struct {
	// FilePath is the path to the recorded WAV file.
	FilePath string

	// DurationSecs is the duration of the recording in seconds.
	DurationSecs int

	// PacketsReceived is the total number of RTP packets captured.
	PacketsReceived int
}

// Record captures incoming RTP audio and writes it to a WAV file at filePath.
// Recording stops when:
//   - The context is cancelled (caller hung up)
//   - maxDuration seconds of audio have been captured
//   - No RTP packets are received for silenceTimeout
//
// The payloadType parameter specifies which RTP payload type to capture
// (payloadPCMU=0 for u-law, payloadPCMA=8 for a-law). Packets with other
// payload types are silently dropped.
//
// The WAV file is written with a proper header for G.711 audio:
// 8000 Hz, mono, 8-bit, a-law or u-law encoding.
func (r *Recorder) Record(ctx context.Context, filePath string, payloadType int, maxDuration int) (*RecordResult, error) {
	// Validate payload type.
	wavFormat, err := wavFormatForPayload(payloadType)
	if err != nil {
		return nil, err
	}

	// Create the output file. Write a placeholder WAV header that will
	// be updated with the final data size when recording completes.
	f, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("creating recording file: %w", err)
	}
	defer f.Close()

	// Write placeholder header — will be rewritten at the end.
	if err := writeWAVHeader(f, wavFormat, 0); err != nil {
		return nil, fmt.Errorf("writing wav header: %w", err)
	}

	maxDurationLimit := time.Duration(maxDuration) * time.Second
	start := time.Now()
	lastPacket := start
	totalBytes := uint32(0)
	packetsReceived := 0
	buf := make([]byte, maxRTPPacket)

	r.logger.Info("voicemail recording started",
		"file", filePath,
		"payload_type", payloadType,
		"max_duration", maxDuration,
	)

	for {
		// Check context cancellation.
		select {
		case <-ctx.Done():
			r.logger.Info("recording stopped: context cancelled",
				"packets", packetsReceived,
				"bytes", totalBytes,
			)
			goto done
		default:
		}

		// Check max duration.
		if time.Since(start) >= maxDurationLimit {
			r.logger.Info("recording stopped: max duration reached",
				"max_duration", maxDuration,
				"packets", packetsReceived,
			)
			goto done
		}

		// Check silence timeout.
		if packetsReceived > 0 && time.Since(lastPacket) >= silenceTimeout {
			r.logger.Info("recording stopped: silence timeout",
				"silence_duration", time.Since(lastPacket),
				"packets", packetsReceived,
			)
			goto done
		}

		// Read an RTP packet with timeout so we can check cancellation.
		r.conn.SetReadDeadline(time.Now().Add(readTimeout))
		n, _, err := r.conn.ReadFromUDP(buf)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				continue
			}
			// Other read errors — log and continue.
			r.logger.Debug("rtp read error during recording", "error", err)
			continue
		}

		if n < minRTPHeader {
			continue
		}

		pkt := buf[:n]

		// Check payload type — only record the expected codec.
		pt := int(pkt[1] & 0x7F)
		if pt != payloadType {
			continue
		}

		// Extract audio payload (skip RTP header).
		// Account for CSRC entries if present.
		cc := int(pkt[0] & 0x0F)
		headerLen := minRTPHeader + cc*4
		if headerLen >= n {
			continue
		}

		// Check for RTP header extension.
		if pkt[0]&0x10 != 0 {
			if headerLen+4 > n {
				continue
			}
			extLen := int(binary.BigEndian.Uint16(pkt[headerLen+2:headerLen+4])) * 4
			headerLen += 4 + extLen
			if headerLen >= n {
				continue
			}
		}

		payload := pkt[headerLen:]

		// Write the raw G.711 payload to the WAV data section.
		written, err := f.Write(payload)
		if err != nil {
			r.logger.Error("failed to write audio data", "error", err)
			goto done
		}

		totalBytes += uint32(written)
		packetsReceived++
		lastPacket = time.Now()
	}

done:
	// Rewrite the WAV header with the actual data size.
	if _, err := f.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("seeking to rewrite wav header: %w", err)
	}
	if err := writeWAVHeader(f, wavFormat, totalBytes); err != nil {
		return nil, fmt.Errorf("rewriting wav header: %w", err)
	}

	// Calculate duration: for 8-bit G.711 at 8kHz, 1 byte = 1 sample.
	durationSecs := int(totalBytes / 8000)

	r.logger.Info("voicemail recording completed",
		"file", filePath,
		"duration_secs", durationSecs,
		"packets", packetsReceived,
		"total_bytes", totalBytes,
	)

	return &RecordResult{
		FilePath:        filePath,
		DurationSecs:    durationSecs,
		PacketsReceived: packetsReceived,
	}, nil
}

// wavFormatForPayload maps an RTP payload type to a WAV audio format code.
func wavFormatForPayload(pt int) (uint16, error) {
	switch pt {
	case payloadPCMU:
		return wavFormatPCMU, nil
	case payloadPCMA:
		return wavFormatPCMA, nil
	default:
		return 0, fmt.Errorf("unsupported payload type %d for voicemail recording", pt)
	}
}

// writeWAVHeader writes a standard 44-byte WAV file header for G.711 audio.
// format is the WAV audio format code (6=a-law, 7=u-law).
// dataSize is the size of the audio data section in bytes.
//
// WAV parameters: 8000 Hz sample rate, mono, 8 bits per sample.
func writeWAVHeader(f *os.File, format uint16, dataSize uint32) error {
	var hdr [wavHeaderSize]byte

	// RIFF header.
	copy(hdr[0:4], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:8], wavHeaderSize-8+dataSize)
	copy(hdr[8:12], "WAVE")

	// fmt sub-chunk.
	copy(hdr[12:16], "fmt ")
	binary.LittleEndian.PutUint32(hdr[16:20], 16) // sub-chunk size
	binary.LittleEndian.PutUint16(hdr[20:22], format)
	binary.LittleEndian.PutUint16(hdr[22:24], 1)    // mono
	binary.LittleEndian.PutUint32(hdr[24:28], 8000) // sample rate
	binary.LittleEndian.PutUint32(hdr[28:32], 8000) // byte rate (8000 * 1 * 1)
	binary.LittleEndian.PutUint16(hdr[32:34], 1)    // block align (1 channel * 1 byte)
	binary.LittleEndian.PutUint16(hdr[34:36], 8)    // bits per sample

	// data sub-chunk.
	copy(hdr[36:40], "data")
	binary.LittleEndian.PutUint32(hdr[40:44], dataSize)

	_, err := f.Write(hdr[:])
	return err
}

// DefaultPayloadType returns the default RTP payload type for voicemail
// recording based on the system configuration. Falls back to PCMU (u-law).
func DefaultPayloadType() int {
	return payloadPCMU
}

// PayloadPCMU is the RTP payload type for G.711 u-law.
const PayloadPCMU = payloadPCMU

// PayloadPCMA is the RTP payload type for G.711 a-law.
const PayloadPCMA = payloadPCMA
