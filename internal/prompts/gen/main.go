// Command gen creates default system audio prompts as G.711 u-law WAV files.
// These are silence-filled placeholder files in the correct format for RTP
// playback. Replace with real voice recordings for production use.
//
// Usage: go run ./internal/prompts/gen
package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

// prompt defines a system prompt to generate.
type prompt struct {
	filename   string
	durationMs int // silence duration in milliseconds
}

// defaultPrompts are the system prompts embedded in the binary.
// Each is a minimal G.711 u-law WAV file (8kHz, mono, 8-bit).
var defaultPrompts = []prompt{
	{"default_voicemail_greeting.wav", 3000},
	{"ivr_invalid_option.wav", 1500},
	{"ivr_timeout.wav", 1500},
	{"transfer_accept.wav", 2000},
	{"followme_confirm.wav", 2000},
}

func main() {
	dir := filepath.Join("internal", "prompts", "system")
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating directory: %v\n", err)
		os.Exit(1)
	}

	for _, p := range defaultPrompts {
		path := filepath.Join(dir, p.filename)
		if err := writeUlawWAV(path, p.durationMs); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", p.filename, err)
			os.Exit(1)
		}
		fi, _ := os.Stat(path)
		fmt.Printf("created %s (%d bytes, %dms silence)\n", path, fi.Size(), p.durationMs)
	}
}

// writeUlawWAV creates a WAV file containing G.711 u-law silence.
// G.711 u-law silence byte is 0xFF. Format: 8kHz, mono, 8-bit.
func writeUlawWAV(path string, durationMs int) error {
	// Calculate data size: 8000 samples/sec * durationMs/1000
	dataSize := uint32(8000 * durationMs / 1000)

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// RIFF header
	f.Write([]byte("RIFF"))
	binary.Write(f, binary.LittleEndian, uint32(36+dataSize)) // file size - 8
	f.Write([]byte("WAVE"))

	// fmt chunk
	f.Write([]byte("fmt "))
	binary.Write(f, binary.LittleEndian, uint32(16))   // chunk size
	binary.Write(f, binary.LittleEndian, uint16(7))    // audio format: 7 = u-law
	binary.Write(f, binary.LittleEndian, uint16(1))    // channels: mono
	binary.Write(f, binary.LittleEndian, uint32(8000)) // sample rate
	binary.Write(f, binary.LittleEndian, uint32(8000)) // byte rate (8000 * 1 * 1)
	binary.Write(f, binary.LittleEndian, uint16(1))    // block align
	binary.Write(f, binary.LittleEndian, uint16(8))    // bits per sample

	// data chunk
	f.Write([]byte("data"))
	binary.Write(f, binary.LittleEndian, dataSize)

	// Silence: 0xFF is u-law silence
	silence := make([]byte, dataSize)
	for i := range silence {
		silence[i] = 0xFF
	}
	_, err = f.Write(silence)
	return err
}
