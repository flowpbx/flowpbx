package prompts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractToDataDir(t *testing.T) {
	dataDir := t.TempDir()

	// First extraction should create all files.
	if err := ExtractToDataDir(dataDir); err != nil {
		t.Fatalf("ExtractToDataDir() error: %v", err)
	}

	systemDir := filepath.Join(dataDir, "prompts", "system")
	for _, name := range SystemPrompts {
		path := filepath.Join(systemDir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected file %s to exist: %v", name, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("expected file %s to be non-empty", name)
		}
	}

	// Custom prompts directory should also be created.
	customDir := filepath.Join(dataDir, "prompts", "custom")
	info, err := os.Stat(customDir)
	if err != nil {
		t.Fatalf("expected custom prompts directory to exist: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("expected %s to be a directory", customDir)
	}
}

func TestExtractToDataDir_SkipsExisting(t *testing.T) {
	dataDir := t.TempDir()

	// Run initial extraction.
	if err := ExtractToDataDir(dataDir); err != nil {
		t.Fatalf("ExtractToDataDir() first call error: %v", err)
	}

	// Overwrite one file with custom content.
	customPath := filepath.Join(dataDir, "prompts", "system", SystemPrompts[0])
	custom := []byte("custom content")
	if err := os.WriteFile(customPath, custom, 0640); err != nil {
		t.Fatalf("writing custom file: %v", err)
	}

	// Second extraction should not overwrite the custom file.
	if err := ExtractToDataDir(dataDir); err != nil {
		t.Fatalf("ExtractToDataDir() second call error: %v", err)
	}

	got, err := os.ReadFile(customPath)
	if err != nil {
		t.Fatalf("reading custom file: %v", err)
	}
	if string(got) != string(custom) {
		t.Errorf("expected custom content to be preserved, got %q", string(got))
	}
}

func TestSystemDir(t *testing.T) {
	got := SystemDir("/data")
	want := filepath.Join("/data", "prompts", "system")
	if got != want {
		t.Errorf("SystemDir() = %q, want %q", got, want)
	}
}

func TestCustomDir(t *testing.T) {
	got := CustomDir("/data")
	want := filepath.Join("/data", "prompts", "custom")
	if got != want {
		t.Errorf("CustomDir() = %q, want %q", got, want)
	}
}

func TestExtractToDataDir_CreatesDirectory(t *testing.T) {
	dataDir := t.TempDir()
	nested := filepath.Join(dataDir, "deep", "nested")

	if err := ExtractToDataDir(nested); err != nil {
		t.Fatalf("ExtractToDataDir() error: %v", err)
	}

	systemDir := filepath.Join(nested, "prompts", "system")
	info, err := os.Stat(systemDir)
	if err != nil {
		t.Fatalf("expected directory %s to exist: %v", systemDir, err)
	}
	if !info.IsDir() {
		t.Errorf("expected %s to be a directory", systemDir)
	}
}
