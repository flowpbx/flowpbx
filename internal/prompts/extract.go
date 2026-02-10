package prompts

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

// SystemDir returns the path to the system prompts directory.
func SystemDir(dataDir string) string {
	return filepath.Join(dataDir, "prompts", "system")
}

// CustomDir returns the path to the custom (user-uploaded) prompts directory.
func CustomDir(dataDir string) string {
	return filepath.Join(dataDir, "prompts", "custom")
}

// ExtractToDataDir copies the embedded system prompts to the data directory
// so they can be served by the media player and referenced by flow nodes.
// Files that already exist on disk are skipped, preserving any manual edits.
// The target directory is $dataDir/prompts/system/.
//
// It also creates the custom prompts directory ($dataDir/prompts/custom/) so
// it is ready for user uploads without requiring on-demand creation.
func ExtractToDataDir(dataDir string) error {
	sysDir := SystemDir(dataDir)
	if err := os.MkdirAll(sysDir, 0750); err != nil {
		return fmt.Errorf("creating system prompts directory: %w", err)
	}

	custDir := CustomDir(dataDir)
	if err := os.MkdirAll(custDir, 0750); err != nil {
		return fmt.Errorf("creating custom prompts directory: %w", err)
	}

	for _, name := range SystemPrompts {
		dest := filepath.Join(sysDir, name)

		// Skip files that already exist on disk.
		if _, err := os.Stat(dest); err == nil {
			slog.Debug("system prompt already exists, skipping", "file", name)
			continue
		}

		data, err := fs.ReadFile(SystemFS, filepath.Join("system", name))
		if err != nil {
			return fmt.Errorf("reading embedded prompt %s: %w", name, err)
		}

		if err := os.WriteFile(dest, data, 0640); err != nil {
			return fmt.Errorf("writing prompt %s: %w", name, err)
		}

		slog.Info("extracted system prompt", "file", name, "path", dest)
	}

	return nil
}
