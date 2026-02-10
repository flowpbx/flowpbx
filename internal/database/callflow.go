package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/flowpbx/flowpbx/internal/database/models"
)

// callFlowRepo implements CallFlowRepository.
type callFlowRepo struct {
	db *DB
}

// NewCallFlowRepository creates a new CallFlowRepository.
func NewCallFlowRepository(db *DB) CallFlowRepository {
	return &callFlowRepo{db: db}
}

// Create inserts a new call flow.
func (r *callFlowRepo) Create(ctx context.Context, flow *models.CallFlow) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO call_flows (name, flow_data, version, published,
		 created_at, updated_at)
		 VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))`,
		flow.Name, flow.FlowData, flow.Version, flow.Published,
	)
	if err != nil {
		return fmt.Errorf("inserting call flow: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}
	flow.ID = id
	return nil
}

// GetByID returns a call flow by ID.
func (r *callFlowRepo) GetByID(ctx context.Context, id int64) (*models.CallFlow, error) {
	return r.scanOne(r.db.QueryRowContext(ctx,
		`SELECT id, name, flow_data, version, published, published_at,
		 created_at, updated_at
		 FROM call_flows WHERE id = ?`, id,
	))
}

// GetPublished returns a call flow by ID only if it is published.
func (r *callFlowRepo) GetPublished(ctx context.Context, id int64) (*models.CallFlow, error) {
	return r.scanOne(r.db.QueryRowContext(ctx,
		`SELECT id, name, flow_data, version, published, published_at,
		 created_at, updated_at
		 FROM call_flows WHERE id = ? AND published = 1`, id,
	))
}

// List returns all call flows ordered by name.
func (r *callFlowRepo) List(ctx context.Context) ([]models.CallFlow, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, flow_data, version, published, published_at,
		 created_at, updated_at
		 FROM call_flows ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("querying call flows: %w", err)
	}
	defer rows.Close()

	var flows []models.CallFlow
	for rows.Next() {
		var f models.CallFlow
		if err := rows.Scan(&f.ID, &f.Name, &f.FlowData, &f.Version,
			&f.Published, &f.PublishedAt, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning call flow row: %w", err)
		}
		flows = append(flows, f)
	}
	return flows, rows.Err()
}

// Update modifies an existing call flow.
func (r *callFlowRepo) Update(ctx context.Context, flow *models.CallFlow) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE call_flows SET name = ?, flow_data = ?, version = version + 1,
		 updated_at = datetime('now')
		 WHERE id = ?`,
		flow.Name, flow.FlowData, flow.ID,
	)
	if err != nil {
		return fmt.Errorf("updating call flow: %w", err)
	}
	return nil
}

// Publish marks a call flow as published, snapshotting the current time.
func (r *callFlowRepo) Publish(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE call_flows SET published = 1, published_at = datetime('now'),
		 updated_at = datetime('now')
		 WHERE id = ?`, id,
	)
	if err != nil {
		return fmt.Errorf("publishing call flow: %w", err)
	}
	return nil
}

// Delete removes a call flow by ID.
func (r *callFlowRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM call_flows WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting call flow: %w", err)
	}
	return nil
}

func (r *callFlowRepo) scanOne(row *sql.Row) (*models.CallFlow, error) {
	var f models.CallFlow
	err := row.Scan(&f.ID, &f.Name, &f.FlowData, &f.Version,
		&f.Published, &f.PublishedAt, &f.CreatedAt, &f.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning call flow: %w", err)
	}
	return &f, nil
}
