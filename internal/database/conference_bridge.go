package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/flowpbx/flowpbx/internal/database/models"
)

// conferenceBridgeRepo implements ConferenceBridgeRepository.
type conferenceBridgeRepo struct {
	db *DB
}

// NewConferenceBridgeRepository creates a new ConferenceBridgeRepository.
func NewConferenceBridgeRepository(db *DB) ConferenceBridgeRepository {
	return &conferenceBridgeRepo{db: db}
}

// Create inserts a new conference bridge.
func (r *conferenceBridgeRepo) Create(ctx context.Context, bridge *models.ConferenceBridge) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO conference_bridges (name, extension, pin, max_members, record,
		 mute_on_join, announce_joins, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'))`,
		bridge.Name, bridge.Extension, bridge.PIN, bridge.MaxMembers,
		bridge.Record, bridge.MuteOnJoin, bridge.AnnounceJoins,
	)
	if err != nil {
		return fmt.Errorf("inserting conference bridge: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}
	bridge.ID = id
	return nil
}

// GetByID returns a conference bridge by ID.
func (r *conferenceBridgeRepo) GetByID(ctx context.Context, id int64) (*models.ConferenceBridge, error) {
	return r.scanOne(r.db.QueryRowContext(ctx,
		`SELECT id, name, extension, pin, max_members, record,
		 mute_on_join, announce_joins, created_at
		 FROM conference_bridges WHERE id = ?`, id,
	))
}

// GetByExtension returns a conference bridge by its dial-in extension.
func (r *conferenceBridgeRepo) GetByExtension(ctx context.Context, ext string) (*models.ConferenceBridge, error) {
	return r.scanOne(r.db.QueryRowContext(ctx,
		`SELECT id, name, extension, pin, max_members, record,
		 mute_on_join, announce_joins, created_at
		 FROM conference_bridges WHERE extension = ?`, ext,
	))
}

// List returns all conference bridges ordered by name.
func (r *conferenceBridgeRepo) List(ctx context.Context) ([]models.ConferenceBridge, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, extension, pin, max_members, record,
		 mute_on_join, announce_joins, created_at
		 FROM conference_bridges ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("querying conference bridges: %w", err)
	}
	defer rows.Close()

	var bridges []models.ConferenceBridge
	for rows.Next() {
		var b models.ConferenceBridge
		if err := rows.Scan(&b.ID, &b.Name, &b.Extension, &b.PIN,
			&b.MaxMembers, &b.Record, &b.MuteOnJoin, &b.AnnounceJoins,
			&b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning conference bridge row: %w", err)
		}
		bridges = append(bridges, b)
	}
	return bridges, rows.Err()
}

// Update modifies an existing conference bridge.
func (r *conferenceBridgeRepo) Update(ctx context.Context, bridge *models.ConferenceBridge) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE conference_bridges SET name = ?, extension = ?, pin = ?,
		 max_members = ?, record = ?, mute_on_join = ?, announce_joins = ?
		 WHERE id = ?`,
		bridge.Name, bridge.Extension, bridge.PIN, bridge.MaxMembers,
		bridge.Record, bridge.MuteOnJoin, bridge.AnnounceJoins, bridge.ID,
	)
	if err != nil {
		return fmt.Errorf("updating conference bridge: %w", err)
	}
	return nil
}

// Delete removes a conference bridge by ID.
func (r *conferenceBridgeRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM conference_bridges WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting conference bridge: %w", err)
	}
	return nil
}

func (r *conferenceBridgeRepo) scanOne(row *sql.Row) (*models.ConferenceBridge, error) {
	var b models.ConferenceBridge
	err := row.Scan(&b.ID, &b.Name, &b.Extension, &b.PIN,
		&b.MaxMembers, &b.Record, &b.MuteOnJoin, &b.AnnounceJoins,
		&b.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning conference bridge: %w", err)
	}
	return &b, nil
}
