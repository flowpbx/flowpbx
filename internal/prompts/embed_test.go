package prompts

import (
	"io/fs"
	"testing"
)

func TestSystemFSContainsAllPrompts(t *testing.T) {
	for _, name := range SystemPrompts {
		path := "system/" + name
		f, err := SystemFS.Open(path)
		if err != nil {
			t.Errorf("SystemFS.Open(%q): %v", path, err)
			continue
		}

		info, err := f.Stat()
		f.Close()
		if err != nil {
			t.Errorf("Stat(%q): %v", path, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("%s is empty", path)
		}
	}
}

func TestSystemFSWAVHeaders(t *testing.T) {
	for _, name := range SystemPrompts {
		path := "system/" + name
		data, err := fs.ReadFile(SystemFS, path)
		if err != nil {
			t.Fatalf("ReadFile(%q): %v", path, err)
		}

		// Verify RIFF/WAVE header.
		if len(data) < 44 {
			t.Errorf("%s too small for WAV header: %d bytes", name, len(data))
			continue
		}
		if string(data[0:4]) != "RIFF" {
			t.Errorf("%s: expected RIFF, got %q", name, string(data[0:4]))
		}
		if string(data[8:12]) != "WAVE" {
			t.Errorf("%s: expected WAVE, got %q", name, string(data[8:12]))
		}

		// Verify G.711 u-law format (7) in fmt chunk.
		// fmt chunk starts at offset 12: "fmt " (4) + size (4) + format (2).
		if string(data[12:16]) != "fmt " {
			t.Errorf("%s: expected fmt chunk at offset 12, got %q", name, string(data[12:16]))
			continue
		}
		audioFormat := uint16(data[20]) | uint16(data[21])<<8
		if audioFormat != 7 {
			t.Errorf("%s: expected format 7 (u-law), got %d", name, audioFormat)
		}
	}
}
