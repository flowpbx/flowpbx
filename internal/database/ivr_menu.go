package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/flowpbx/flowpbx/internal/database/models"
)

// ivrMenuRepo implements IVRMenuRepository.
type ivrMenuRepo struct {
	db *DB
}

// NewIVRMenuRepository creates a new IVRMenuRepository.
func NewIVRMenuRepository(db *DB) IVRMenuRepository {
	return &ivrMenuRepo{db: db}
}

// Create inserts a new IVR menu.
func (r *ivrMenuRepo) Create(ctx context.Context, ivr *models.IVRMenu) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO ivr_menus (name, greeting_file, greeting_tts, timeout, max_retries,
		 digit_timeout, options, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		ivr.Name, ivr.GreetingFile, ivr.GreetingTTS, ivr.Timeout,
		ivr.MaxRetries, ivr.DigitTimeout, ivr.Options,
	)
	if err != nil {
		return fmt.Errorf("inserting ivr menu: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}
	ivr.ID = id
	return nil
}

// GetByID returns an IVR menu by ID.
func (r *ivrMenuRepo) GetByID(ctx context.Context, id int64) (*models.IVRMenu, error) {
	return r.scanOne(r.db.QueryRowContext(ctx,
		`SELECT id, name, greeting_file, greeting_tts, timeout, max_retries,
		 digit_timeout, options, created_at, updated_at
		 FROM ivr_menus WHERE id = ?`, id,
	))
}

// List returns all IVR menus ordered by name.
func (r *ivrMenuRepo) List(ctx context.Context) ([]models.IVRMenu, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, greeting_file, greeting_tts, timeout, max_retries,
		 digit_timeout, options, created_at, updated_at
		 FROM ivr_menus ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("querying ivr menus: %w", err)
	}
	defer rows.Close()

	var menus []models.IVRMenu
	for rows.Next() {
		var m models.IVRMenu
		if err := rows.Scan(&m.ID, &m.Name, &m.GreetingFile, &m.GreetingTTS,
			&m.Timeout, &m.MaxRetries, &m.DigitTimeout, &m.Options,
			&m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning ivr menu row: %w", err)
		}
		menus = append(menus, m)
	}
	return menus, rows.Err()
}

// Update modifies an existing IVR menu.
func (r *ivrMenuRepo) Update(ctx context.Context, ivr *models.IVRMenu) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE ivr_menus SET name = ?, greeting_file = ?, greeting_tts = ?,
		 timeout = ?, max_retries = ?, digit_timeout = ?, options = ?,
		 updated_at = datetime('now')
		 WHERE id = ?`,
		ivr.Name, ivr.GreetingFile, ivr.GreetingTTS, ivr.Timeout,
		ivr.MaxRetries, ivr.DigitTimeout, ivr.Options, ivr.ID,
	)
	if err != nil {
		return fmt.Errorf("updating ivr menu: %w", err)
	}
	return nil
}

// Delete removes an IVR menu by ID.
func (r *ivrMenuRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM ivr_menus WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting ivr menu: %w", err)
	}
	return nil
}

func (r *ivrMenuRepo) scanOne(row *sql.Row) (*models.IVRMenu, error) {
	var m models.IVRMenu
	err := row.Scan(&m.ID, &m.Name, &m.GreetingFile, &m.GreetingTTS,
		&m.Timeout, &m.MaxRetries, &m.DigitTimeout, &m.Options,
		&m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning ivr menu: %w", err)
	}
	return &m, nil
}
