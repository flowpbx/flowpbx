package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/flowpbx/flowpbx/internal/database/models"
)

// timeSwitchRepo implements TimeSwitchRepository.
type timeSwitchRepo struct {
	db *DB
}

// NewTimeSwitchRepository creates a new TimeSwitchRepository.
func NewTimeSwitchRepository(db *DB) TimeSwitchRepository {
	return &timeSwitchRepo{db: db}
}

// Create inserts a new time switch.
func (r *timeSwitchRepo) Create(ctx context.Context, ts *models.TimeSwitch) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO time_switches (name, timezone, rules, overrides, default_dest,
		 created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		ts.Name, ts.Timezone, ts.Rules, ts.Overrides, ts.DefaultDest,
	)
	if err != nil {
		return fmt.Errorf("inserting time switch: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}
	ts.ID = id
	return nil
}

// GetByID returns a time switch by ID.
func (r *timeSwitchRepo) GetByID(ctx context.Context, id int64) (*models.TimeSwitch, error) {
	return r.scanOne(r.db.QueryRowContext(ctx,
		`SELECT id, name, timezone, rules, overrides, default_dest,
		 created_at, updated_at
		 FROM time_switches WHERE id = ?`, id,
	))
}

// List returns all time switches ordered by name.
func (r *timeSwitchRepo) List(ctx context.Context) ([]models.TimeSwitch, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, timezone, rules, overrides, default_dest,
		 created_at, updated_at
		 FROM time_switches ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("querying time switches: %w", err)
	}
	defer rows.Close()

	var switches []models.TimeSwitch
	for rows.Next() {
		var ts models.TimeSwitch
		if err := rows.Scan(&ts.ID, &ts.Name, &ts.Timezone, &ts.Rules,
			&ts.Overrides, &ts.DefaultDest, &ts.CreatedAt, &ts.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning time switch row: %w", err)
		}
		switches = append(switches, ts)
	}
	return switches, rows.Err()
}

// Update modifies an existing time switch.
func (r *timeSwitchRepo) Update(ctx context.Context, ts *models.TimeSwitch) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE time_switches SET name = ?, timezone = ?, rules = ?,
		 overrides = ?, default_dest = ?, updated_at = datetime('now')
		 WHERE id = ?`,
		ts.Name, ts.Timezone, ts.Rules, ts.Overrides, ts.DefaultDest, ts.ID,
	)
	if err != nil {
		return fmt.Errorf("updating time switch: %w", err)
	}
	return nil
}

// Delete removes a time switch by ID.
func (r *timeSwitchRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM time_switches WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting time switch: %w", err)
	}
	return nil
}

func (r *timeSwitchRepo) scanOne(row *sql.Row) (*models.TimeSwitch, error) {
	var ts models.TimeSwitch
	err := row.Scan(&ts.ID, &ts.Name, &ts.Timezone, &ts.Rules,
		&ts.Overrides, &ts.DefaultDest, &ts.CreatedAt, &ts.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning time switch: %w", err)
	}
	return &ts, nil
}
