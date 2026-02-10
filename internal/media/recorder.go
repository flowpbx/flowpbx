package media

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// recorderChanSize is the buffered channel capacity for incoming RTP packets.
	// At 50 packets/sec (20ms ptime), this holds ~2 seconds of audio per direction.
	recorderChanSize = 128

	// recorderFlushSize is the number of decoded samples to buffer before
	// flushing to disk. 8000 samples = 1 second at 8kHz.
	recorderFlushSize = 8000
)

// rtpPacket is a copy of an RTP packet queued for recording.
type rtpPacket struct {
	payload     []byte
	payloadType int
}

// Recorder captures an RTP stream to a WAV file. It runs a dedicated
// goroutine that reads packets from a buffered channel, decodes G.711
// audio to linear PCM, then re-encodes to G.711 u-law for WAV storage.
//
// Usage:
//
//	rec, _ := NewRecorder(filePath, logger)
//	// In the relay's forwarding loop:
//	rec.Feed(rtpPayload, payloadType)
//	// When the call ends:
//	filePath, duration := rec.Stop()
//
// Feed is non-blocking: if the goroutine falls behind, packets are dropped
// rather than blocking the relay.
//
// Thread safety: Feed may be called concurrently from multiple relay goroutines.
// Stop must be called exactly once.
type Recorder struct {
	mu       sync.Mutex
	file     *os.File
	filePath string
	dataSize uint32
	stopped  bool
	logger   *slog.Logger

	packets chan rtpPacket
	done    chan struct{}
}

// NewRecorder creates a call recorder that writes G.711 u-law WAV audio to
// the specified file path. Parent directories are created if needed.
// The recording goroutine starts immediately.
func NewRecorder(filePath string, logger *slog.Logger) (*Recorder, error) {
	// Ensure parent directory exists.
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating recording directory: %w", err)
	}

	f, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("creating recording file: %w", err)
	}

	// Write placeholder WAV header (rewritten on Stop with actual data size).
	if err := writeRecorderWAVHeader(f, 0); err != nil {
		f.Close()
		os.Remove(filePath)
		return nil, fmt.Errorf("writing wav header: %w", err)
	}

	r := &Recorder{
		file:     f,
		filePath: filePath,
		logger:   logger.With("subsystem", "call-recorder", "file", filePath),
		packets:  make(chan rtpPacket, recorderChanSize),
		done:     make(chan struct{}),
	}

	go r.writeLoop()

	r.logger.Info("call recording started")

	return r, nil
}

// Feed queues an RTP payload for recording. The payload is copied so the
// caller's buffer can be reused immediately. payloadType indicates the
// codec (PayloadPCMU or PayloadPCMA). If the write goroutine is behind,
// the packet is silently dropped to avoid blocking the relay.
func (r *Recorder) Feed(payload []byte, payloadType int) {
	if len(payload) == 0 {
		return
	}

	// Copy payload — the caller's buffer is reused on the next read.
	buf := make([]byte, len(payload))
	copy(buf, payload)

	select {
	case r.packets <- rtpPacket{payload: buf, payloadType: payloadType}:
	default:
		// Channel full — drop packet rather than blocking the relay.
	}
}

// Stop finalizes the recording: drains remaining packets, rewrites the WAV
// header with the actual data size, and closes the file. Returns the file
// path and duration in seconds. Must be called exactly once.
func (r *Recorder) Stop() (filePath string, durationSecs int) {
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return r.filePath, 0
	}
	r.stopped = true
	r.mu.Unlock()

	// Close channel to signal the write goroutine to drain and exit.
	close(r.packets)
	<-r.done

	// Rewrite WAV header with actual data size.
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := r.file.Seek(0, 0); err != nil {
		r.logger.Error("failed to seek for wav header rewrite", "error", err)
	} else if err := writeRecorderWAVHeader(r.file, r.dataSize); err != nil {
		r.logger.Error("failed to rewrite wav header", "error", err)
	}

	r.file.Close()

	durationSecs = int(r.dataSize / 8000) // 8000 bytes/sec for G.711 u-law @ 8kHz mono

	r.logger.Info("call recording stopped",
		"duration_secs", durationSecs,
		"total_bytes", r.dataSize,
	)

	return r.filePath, durationSecs
}

// FilePath returns the path to the recording file.
func (r *Recorder) FilePath() string {
	return r.filePath
}

// writeLoop is the recording goroutine. It reads RTP packets from the
// channel, decodes G.711 payloads to linear PCM, re-encodes to G.711 u-law,
// and writes to the WAV file. It exits when the channel is closed.
func (r *Recorder) writeLoop() {
	defer close(r.done)

	// Reusable buffer for encoding samples before writing to disk.
	writeBuf := make([]byte, 0, recorderFlushSize)

	flush := func() {
		if len(writeBuf) == 0 {
			return
		}
		n, err := r.file.Write(writeBuf)
		if err != nil {
			r.logger.Error("failed to write recording data", "error", err)
		}
		r.mu.Lock()
		r.dataSize += uint32(n)
		r.mu.Unlock()
		writeBuf = writeBuf[:0]
	}

	for pkt := range r.packets {
		// Decode each G.711 byte to PCM, then re-encode to u-law.
		// If the source is already PCMU, this is a passthrough (decode+encode = identity).
		// If the source is PCMA, this transcodes a-law → PCM → u-law.
		for _, b := range pkt.payload {
			var pcm int16
			switch pkt.payloadType {
			case PayloadPCMU:
				pcm = ulawToLinear[b]
			case PayloadPCMA:
				pcm = alawToLinear[b]
			default:
				// Unsupported codec — skip this packet entirely.
				break
			}
			writeBuf = append(writeBuf, linearToUlaw[uint16(pcm)])
		}

		// Flush periodically to avoid holding too much in memory.
		if len(writeBuf) >= recorderFlushSize {
			flush()
		}
	}

	// Drain any remaining samples.
	flush()
}

// RecordingPath returns the organized file path for a call recording.
// Recordings are stored by date: $dataDir/recordings/YYYY/MM/DD/call_{id}.wav
func RecordingPath(dataDir, callID string, t time.Time) string {
	return filepath.Join(
		dataDir,
		"recordings",
		t.Format("2006"),
		t.Format("01"),
		t.Format("02"),
		fmt.Sprintf("call_%s.wav", callID),
	)
}

// writeRecorderWAVHeader writes a 44-byte WAV header for G.711 u-law audio.
// 8000 Hz sample rate, mono, 8 bits per sample.
func writeRecorderWAVHeader(f *os.File, dataSize uint32) error {
	var hdr [wavHeaderSize]byte

	// RIFF header.
	copy(hdr[0:4], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:8], wavHeaderSize-8+dataSize)
	copy(hdr[8:12], "WAVE")

	// fmt sub-chunk.
	copy(hdr[12:16], "fmt ")
	binary.LittleEndian.PutUint32(hdr[16:20], 16)            // sub-chunk size
	binary.LittleEndian.PutUint16(hdr[20:22], wavFormatPCMU) // G.711 u-law
	binary.LittleEndian.PutUint16(hdr[22:24], 1)             // mono
	binary.LittleEndian.PutUint32(hdr[24:28], 8000)          // sample rate
	binary.LittleEndian.PutUint32(hdr[28:32], 8000)          // byte rate
	binary.LittleEndian.PutUint16(hdr[32:34], 1)             // block align
	binary.LittleEndian.PutUint16(hdr[34:36], 8)             // bits per sample

	// data sub-chunk.
	copy(hdr[36:40], "data")
	binary.LittleEndian.PutUint32(hdr[40:44], dataSize)

	_, err := f.Write(hdr[:])
	return err
}
