package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/flowpbx/flowpbx/internal/database/models"
)

// inboundNumberRepo implements InboundNumberRepository.
type inboundNumberRepo struct {
	db *DB
}

// NewInboundNumberRepository creates a new InboundNumberRepository.
func NewInboundNumberRepository(db *DB) InboundNumberRepository {
	return &inboundNumberRepo{db: db}
}

// Create inserts a new inbound number.
func (r *inboundNumberRepo) Create(ctx context.Context, num *models.InboundNumber) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO inbound_numbers (number, name, trunk_id, flow_id, flow_entry_node,
		 enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		num.Number, num.Name, num.TrunkID, num.FlowID, num.FlowEntryNode, num.Enabled,
	)
	if err != nil {
		return fmt.Errorf("inserting inbound number: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}
	num.ID = id
	return nil
}

// GetByID returns an inbound number by ID.
func (r *inboundNumberRepo) GetByID(ctx context.Context, id int64) (*models.InboundNumber, error) {
	return r.scanOne(r.db.QueryRowContext(ctx,
		`SELECT id, number, name, trunk_id, flow_id, flow_entry_node,
		 enabled, created_at, updated_at
		 FROM inbound_numbers WHERE id = ?`, id,
	))
}

// GetByNumber returns an inbound number by its number string.
func (r *inboundNumberRepo) GetByNumber(ctx context.Context, number string) (*models.InboundNumber, error) {
	return r.scanOne(r.db.QueryRowContext(ctx,
		`SELECT id, number, name, trunk_id, flow_id, flow_entry_node,
		 enabled, created_at, updated_at
		 FROM inbound_numbers WHERE number = ?`, number,
	))
}

// List returns all inbound numbers.
func (r *inboundNumberRepo) List(ctx context.Context) ([]models.InboundNumber, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, number, name, trunk_id, flow_id, flow_entry_node,
		 enabled, created_at, updated_at
		 FROM inbound_numbers ORDER BY number`)
	if err != nil {
		return nil, fmt.Errorf("querying inbound numbers: %w", err)
	}
	defer rows.Close()

	var nums []models.InboundNumber
	for rows.Next() {
		var n models.InboundNumber
		if err := rows.Scan(&n.ID, &n.Number, &n.Name, &n.TrunkID, &n.FlowID,
			&n.FlowEntryNode, &n.Enabled, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning inbound number row: %w", err)
		}
		nums = append(nums, n)
	}
	return nums, rows.Err()
}

// Update modifies an existing inbound number.
func (r *inboundNumberRepo) Update(ctx context.Context, num *models.InboundNumber) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE inbound_numbers SET number = ?, name = ?, trunk_id = ?, flow_id = ?,
		 flow_entry_node = ?, enabled = ?, updated_at = datetime('now')
		 WHERE id = ?`,
		num.Number, num.Name, num.TrunkID, num.FlowID, num.FlowEntryNode,
		num.Enabled, num.ID,
	)
	if err != nil {
		return fmt.Errorf("updating inbound number: %w", err)
	}
	return nil
}

// Delete removes an inbound number by ID.
func (r *inboundNumberRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM inbound_numbers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting inbound number: %w", err)
	}
	return nil
}

func (r *inboundNumberRepo) scanOne(row *sql.Row) (*models.InboundNumber, error) {
	var n models.InboundNumber
	err := row.Scan(&n.ID, &n.Number, &n.Name, &n.TrunkID, &n.FlowID,
		&n.FlowEntryNode, &n.Enabled, &n.CreatedAt, &n.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning inbound number: %w", err)
	}
	return &n, nil
}
