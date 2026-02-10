package prompts

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

// ExtractToDataDir copies the embedded system prompts to the data directory
// so they can be served by the media player and referenced by flow nodes.
// Files that already exist on disk are skipped, preserving any manual edits.
// The target directory is $dataDir/prompts/system/.
func ExtractToDataDir(dataDir string) error {
	dir := filepath.Join(dataDir, "prompts", "system")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("creating prompts directory: %w", err)
	}

	for _, name := range SystemPrompts {
		dest := filepath.Join(dir, name)

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
