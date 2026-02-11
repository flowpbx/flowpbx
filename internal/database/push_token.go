package database

import (
	"context"
	"fmt"

	"github.com/flowpbx/flowpbx/internal/database/models"
)

// pushTokenRepo implements PushTokenRepository.
type pushTokenRepo struct {
	db *DB
}

// NewPushTokenRepository creates a new PushTokenRepository.
func NewPushTokenRepository(db *DB) PushTokenRepository {
	return &pushTokenRepo{db: db}
}

// Upsert inserts or updates a push token for a given extension and device.
// If a token already exists for the same (extension_id, device_id), it is updated.
func (r *pushTokenRepo) Upsert(ctx context.Context, token *models.PushToken) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO push_tokens (extension_id, token, platform, device_id, app_version, updated_at)
		 VALUES (?, ?, ?, ?, ?, datetime('now'))
		 ON CONFLICT(extension_id, device_id) DO UPDATE SET
		   token = excluded.token,
		   platform = excluded.platform,
		   app_version = excluded.app_version,
		   updated_at = datetime('now')`,
		token.ExtensionID, token.Token, token.Platform, token.DeviceID, token.AppVersion,
	)
	if err != nil {
		return fmt.Errorf("upserting push token: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}
	token.ID = id
	return nil
}

// GetByExtensionID returns all push tokens for an extension.
func (r *pushTokenRepo) GetByExtensionID(ctx context.Context, extensionID int64) ([]models.PushToken, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, extension_id, token, platform, device_id, app_version, created_at, updated_at
		 FROM push_tokens WHERE extension_id = ? ORDER BY updated_at DESC`, extensionID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying push tokens by extension: %w", err)
	}
	defer rows.Close()

	var tokens []models.PushToken
	for rows.Next() {
		var t models.PushToken
		if err := rows.Scan(&t.ID, &t.ExtensionID, &t.Token, &t.Platform,
			&t.DeviceID, &t.AppVersion, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning push token row: %w", err)
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// DeleteByExtensionAndDevice removes a push token for a specific extension and device.
func (r *pushTokenRepo) DeleteByExtensionAndDevice(ctx context.Context, extensionID int64, deviceID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM push_tokens WHERE extension_id = ? AND device_id = ?`,
		extensionID, deviceID)
	if err != nil {
		return fmt.Errorf("deleting push token by extension and device: %w", err)
	}
	return nil
}

// DeleteByToken removes a push token by its token value. Used to invalidate
// tokens that the push gateway reports as invalid.
func (r *pushTokenRepo) DeleteByToken(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM push_tokens WHERE token = ?`, token)
	if err != nil {
		return fmt.Errorf("deleting push token by value: %w", err)
	}
	return nil
}

// DeleteByExtensionID removes all push tokens for an extension.
func (r *pushTokenRepo) DeleteByExtensionID(ctx context.Context, extensionID int64) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM push_tokens WHERE extension_id = ?`, extensionID)
	if err != nil {
		return fmt.Errorf("deleting push tokens by extension: %w", err)
	}
	return nil
}
