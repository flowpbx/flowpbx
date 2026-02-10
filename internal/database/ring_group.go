package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/flowpbx/flowpbx/internal/database/models"
)

// ringGroupRepo implements RingGroupRepository.
type ringGroupRepo struct {
	db *DB
}

// NewRingGroupRepository creates a new RingGroupRepository.
func NewRingGroupRepository(db *DB) RingGroupRepository {
	return &ringGroupRepo{db: db}
}

// Create inserts a new ring group.
func (r *ringGroupRepo) Create(ctx context.Context, rg *models.RingGroup) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO ring_groups (name, strategy, ring_timeout, members, caller_id_mode,
		 created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		rg.Name, rg.Strategy, rg.RingTimeout, rg.Members, rg.CallerIDMode,
	)
	if err != nil {
		return fmt.Errorf("inserting ring group: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}
	rg.ID = id
	return nil
}

// GetByID returns a ring group by ID.
func (r *ringGroupRepo) GetByID(ctx context.Context, id int64) (*models.RingGroup, error) {
	return r.scanOne(r.db.QueryRowContext(ctx,
		`SELECT id, name, strategy, ring_timeout, members, caller_id_mode,
		 created_at, updated_at
		 FROM ring_groups WHERE id = ?`, id,
	))
}

// List returns all ring groups ordered by name.
func (r *ringGroupRepo) List(ctx context.Context) ([]models.RingGroup, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, strategy, ring_timeout, members, caller_id_mode,
		 created_at, updated_at
		 FROM ring_groups ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("querying ring groups: %w", err)
	}
	defer rows.Close()

	var groups []models.RingGroup
	for rows.Next() {
		var g models.RingGroup
		if err := rows.Scan(&g.ID, &g.Name, &g.Strategy, &g.RingTimeout,
			&g.Members, &g.CallerIDMode, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning ring group row: %w", err)
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// Update modifies an existing ring group.
func (r *ringGroupRepo) Update(ctx context.Context, rg *models.RingGroup) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE ring_groups SET name = ?, strategy = ?, ring_timeout = ?,
		 members = ?, caller_id_mode = ?, updated_at = datetime('now')
		 WHERE id = ?`,
		rg.Name, rg.Strategy, rg.RingTimeout, rg.Members, rg.CallerIDMode, rg.ID,
	)
	if err != nil {
		return fmt.Errorf("updating ring group: %w", err)
	}
	return nil
}

// Delete removes a ring group by ID.
func (r *ringGroupRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM ring_groups WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting ring group: %w", err)
	}
	return nil
}

func (r *ringGroupRepo) scanOne(row *sql.Row) (*models.RingGroup, error) {
	var g models.RingGroup
	err := row.Scan(&g.ID, &g.Name, &g.Strategy, &g.RingTimeout,
		&g.Members, &g.CallerIDMode, &g.CreatedAt, &g.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning ring group: %w", err)
	}
	return &g, nil
}
