package recording

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/flowpbx/flowpbx/internal/database"
)

// StartCleanupTicker runs a background goroutine that periodically removes
// recording files older than the configured recording_max_days setting. The
// CDR's recording_file field is cleared and the WAV file is deleted from disk.
// If recording_max_days is 0 or unset, no cleanup is performed. The goroutine
// stops when the provided context is cancelled.
func StartCleanupTicker(ctx context.Context, db *database.DB, sysConfig database.SystemConfigRepository, interval time.Duration) {
	cdrs := database.NewCDRRepository(db)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				maxDaysStr, err := sysConfig.Get(ctx, "recording_max_days")
				if err != nil {
					slog.Error("recording retention: failed to read setting", "error", err)
					continue
				}
				if maxDaysStr == "" || maxDaysStr == "0" {
					continue
				}

				maxDays, err := strconv.Atoi(maxDaysStr)
				if err != nil || maxDays <= 0 {
					continue
				}

				paths, err := cdrs.DeleteExpiredRecordings(ctx, maxDays)
				if err != nil {
					slog.Error("recording retention cleanup failed", "error", err)
					continue
				}
				if len(paths) == 0 {
					continue
				}

				slog.Info("recording retention cleanup", "deleted", len(paths), "max_days", maxDays)

				// Remove WAV files from disk.
				for _, p := range paths {
					if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
						slog.Warn("failed to remove recording file", "path", p, "error", err)
					}
				}
			}
		}
	}()
}
