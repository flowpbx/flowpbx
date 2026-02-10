package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/flowpbx/flowpbx/internal/database/models"
)

// extensionRepo implements ExtensionRepository.
type extensionRepo struct {
	db *DB
}

// NewExtensionRepository creates a new ExtensionRepository.
func NewExtensionRepository(db *DB) ExtensionRepository {
	return &extensionRepo{db: db}
}

// Create inserts a new extension.
func (r *extensionRepo) Create(ctx context.Context, ext *models.Extension) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO extensions (extension, name, email, sip_username, sip_password,
		 ring_timeout, dnd, follow_me_enabled, follow_me_numbers, follow_me_strategy,
		 recording_mode, max_registrations, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		ext.Extension, ext.Name, ext.Email, ext.SIPUsername, ext.SIPPassword,
		ext.RingTimeout, ext.DND, ext.FollowMeEnabled, ext.FollowMeNumbers,
		ext.FollowMeStrategy, ext.RecordingMode, ext.MaxRegistrations,
	)
	if err != nil {
		return fmt.Errorf("inserting extension: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}
	ext.ID = id
	return nil
}

// GetByID returns an extension by ID.
func (r *extensionRepo) GetByID(ctx context.Context, id int64) (*models.Extension, error) {
	return r.scanOne(r.db.QueryRowContext(ctx,
		`SELECT id, extension, name, email, sip_username, sip_password,
		 ring_timeout, dnd, follow_me_enabled, follow_me_numbers, follow_me_strategy,
		 recording_mode, max_registrations, created_at, updated_at
		 FROM extensions WHERE id = ?`, id,
	))
}

// GetByExtension returns an extension by its extension number.
func (r *extensionRepo) GetByExtension(ctx context.Context, ext string) (*models.Extension, error) {
	return r.scanOne(r.db.QueryRowContext(ctx,
		`SELECT id, extension, name, email, sip_username, sip_password,
		 ring_timeout, dnd, follow_me_enabled, follow_me_numbers, follow_me_strategy,
		 recording_mode, max_registrations, created_at, updated_at
		 FROM extensions WHERE extension = ?`, ext,
	))
}

// GetBySIPUsername returns an extension by SIP username.
func (r *extensionRepo) GetBySIPUsername(ctx context.Context, username string) (*models.Extension, error) {
	return r.scanOne(r.db.QueryRowContext(ctx,
		`SELECT id, extension, name, email, sip_username, sip_password,
		 ring_timeout, dnd, follow_me_enabled, follow_me_numbers, follow_me_strategy,
		 recording_mode, max_registrations, created_at, updated_at
		 FROM extensions WHERE sip_username = ?`, username,
	))
}

// List returns all extensions ordered by extension number.
func (r *extensionRepo) List(ctx context.Context) ([]models.Extension, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, extension, name, email, sip_username, sip_password,
		 ring_timeout, dnd, follow_me_enabled, follow_me_numbers, follow_me_strategy,
		 recording_mode, max_registrations, created_at, updated_at
		 FROM extensions ORDER BY extension`)
	if err != nil {
		return nil, fmt.Errorf("querying extensions: %w", err)
	}
	defer rows.Close()

	var exts []models.Extension
	for rows.Next() {
		var e models.Extension
		if err := rows.Scan(&e.ID, &e.Extension, &e.Name, &e.Email, &e.SIPUsername,
			&e.SIPPassword, &e.RingTimeout, &e.DND, &e.FollowMeEnabled,
			&e.FollowMeNumbers, &e.FollowMeStrategy, &e.RecordingMode,
			&e.MaxRegistrations, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning extension row: %w", err)
		}
		exts = append(exts, e)
	}
	return exts, rows.Err()
}

// Update modifies an existing extension.
func (r *extensionRepo) Update(ctx context.Context, ext *models.Extension) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE extensions SET extension = ?, name = ?, email = ?, sip_username = ?,
		 sip_password = ?, ring_timeout = ?, dnd = ?, follow_me_enabled = ?,
		 follow_me_numbers = ?, follow_me_strategy = ?, recording_mode = ?,
		 max_registrations = ?, updated_at = datetime('now')
		 WHERE id = ?`,
		ext.Extension, ext.Name, ext.Email, ext.SIPUsername, ext.SIPPassword,
		ext.RingTimeout, ext.DND, ext.FollowMeEnabled, ext.FollowMeNumbers,
		ext.FollowMeStrategy, ext.RecordingMode, ext.MaxRegistrations, ext.ID,
	)
	if err != nil {
		return fmt.Errorf("updating extension: %w", err)
	}
	return nil
}

// Delete removes an extension by ID.
func (r *extensionRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM extensions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting extension: %w", err)
	}
	return nil
}

func (r *extensionRepo) scanOne(row *sql.Row) (*models.Extension, error) {
	var e models.Extension
	err := row.Scan(&e.ID, &e.Extension, &e.Name, &e.Email, &e.SIPUsername,
		&e.SIPPassword, &e.RingTimeout, &e.DND, &e.FollowMeEnabled,
		&e.FollowMeNumbers, &e.FollowMeStrategy, &e.RecordingMode,
		&e.MaxRegistrations, &e.CreatedAt, &e.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning extension: %w", err)
	}
	return &e, nil
}
