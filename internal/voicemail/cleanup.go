package voicemail

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/flowpbx/flowpbx/internal/database"
)

// StartCleanupTicker runs a background goroutine that periodically removes
// voicemail messages that exceed their box's retention_days setting. Deleted
// messages have their WAV files removed from disk. The goroutine stops when
// the provided context is cancelled.
func StartCleanupTicker(ctx context.Context, db *database.DB, interval time.Duration) {
	msgs := database.NewVoicemailMessageRepository(db)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				paths, err := msgs.DeleteExpiredByRetention(ctx)
				if err != nil {
					slog.Error("voicemail retention cleanup failed", "error", err)
					continue
				}
				if len(paths) == 0 {
					continue
				}

				slog.Info("voicemail retention cleanup", "deleted", len(paths))

				// Remove WAV files from disk.
				for _, p := range paths {
					if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
						slog.Warn("failed to remove voicemail file", "path", p, "error", err)
					}
				}
			}
		}
	}()
}
