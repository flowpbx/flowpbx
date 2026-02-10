package media

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"sync"
)

const (
	// wavHeaderSize is the size of the WAV file header (44 bytes).
	wavHeaderSize = 44
)

// ConferenceRecorder captures the full conference mix to a WAV file.
// It receives linear PCM samples from the mixer's mix cycle and encodes
// them to G.711 u-law for efficient storage. The WAV header is rewritten
// with the final data size when recording is stopped.
//
// Thread safety: WriteSamples may be called concurrently from the mixer
// goroutine. Stop must be called exactly once.
type ConferenceRecorder struct {
	mu       sync.Mutex
	file     *os.File
	filePath string
	dataSize uint32
	stopped  bool
	logger   *slog.Logger
}

// NewConferenceRecorder creates a recorder that writes the mixed conference
// audio to the specified WAV file. The file is created immediately with a
// placeholder header.
func NewConferenceRecorder(filePath string, logger *slog.Logger) (*ConferenceRecorder, error) {
	f, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("creating conference recording file: %w", err)
	}

	// Write placeholder WAV header (will be rewritten on Stop).
	if err := writeConferenceWAVHeader(f, 0); err != nil {
		f.Close()
		os.Remove(filePath)
		return nil, fmt.Errorf("writing wav header: %w", err)
	}

	logger.Info("conference recording started", "file", filePath)

	return &ConferenceRecorder{
		file:     f,
		filePath: filePath,
		logger:   logger,
	}, nil
}

// WriteSamples encodes linear PCM samples to G.711 u-law and appends them
// to the WAV file. This is called from the mixer's mix cycle on each tick.
// The samples represent the full mix of all participants' audio.
func (cr *ConferenceRecorder) WriteSamples(samples []int32) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if cr.stopped {
		return
	}

	// Encode int32 PCM samples to G.711 u-law bytes.
	buf := make([]byte, len(samples))
	for i, s := range samples {
		// Clamp to int16 range.
		if s > 32767 {
			s = 32767
		} else if s < -32768 {
			s = -32768
		}
		buf[i] = linearToUlaw[uint16(int16(s))]
	}

	n, err := cr.file.Write(buf)
	if err != nil {
		cr.logger.Error("failed to write conference recording data", "error", err)
		return
	}
	cr.dataSize += uint32(n)
}

// Stop finalizes the recording by rewriting the WAV header with the actual
// data size and closing the file. Returns the file path and duration in
// seconds. Must be called exactly once.
func (cr *ConferenceRecorder) Stop() (filePath string, durationSecs int) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if cr.stopped {
		return cr.filePath, int(cr.dataSize / 8000)
	}
	cr.stopped = true

	// Rewrite WAV header with actual data size.
	if _, err := cr.file.Seek(0, 0); err != nil {
		cr.logger.Error("failed to seek for wav header rewrite", "error", err)
	} else if err := writeConferenceWAVHeader(cr.file, cr.dataSize); err != nil {
		cr.logger.Error("failed to rewrite wav header", "error", err)
	}

	cr.file.Close()

	durationSecs = int(cr.dataSize / 8000)

	cr.logger.Info("conference recording stopped",
		"file", cr.filePath,
		"duration_secs", durationSecs,
		"total_bytes", cr.dataSize,
	)

	return cr.filePath, durationSecs
}

// FilePath returns the path to the recording file.
func (cr *ConferenceRecorder) FilePath() string {
	return cr.filePath
}

// writeConferenceWAVHeader writes a 44-byte WAV header for G.711 u-law audio.
// 8000 Hz sample rate, mono, 8 bits per sample.
func writeConferenceWAVHeader(f *os.File, dataSize uint32) error {
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
