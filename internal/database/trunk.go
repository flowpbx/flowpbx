package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/flowpbx/flowpbx/internal/database/models"
)

// trunkRepo implements TrunkRepository.
type trunkRepo struct {
	db *DB
}

// NewTrunkRepository creates a new TrunkRepository.
func NewTrunkRepository(db *DB) TrunkRepository {
	return &trunkRepo{db: db}
}

// Create inserts a new trunk.
func (r *trunkRepo) Create(ctx context.Context, trunk *models.Trunk) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO trunks (name, type, enabled, host, port, transport, username,
		 password, auth_username, register_expiry, remote_hosts, local_host, codecs,
		 max_channels, caller_id_name, caller_id_num, prefix_strip, prefix_add,
		 priority, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		 datetime('now'), datetime('now'))`,
		trunk.Name, trunk.Type, trunk.Enabled, trunk.Host, trunk.Port, trunk.Transport,
		trunk.Username, trunk.Password, trunk.AuthUsername, trunk.RegisterExpiry,
		trunk.RemoteHosts, trunk.LocalHost, trunk.Codecs, trunk.MaxChannels,
		trunk.CallerIDName, trunk.CallerIDNum, trunk.PrefixStrip, trunk.PrefixAdd,
		trunk.Priority,
	)
	if err != nil {
		return fmt.Errorf("inserting trunk: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}
	trunk.ID = id
	return nil
}

// GetByID returns a trunk by ID.
func (r *trunkRepo) GetByID(ctx context.Context, id int64) (*models.Trunk, error) {
	return r.scanOne(r.db.QueryRowContext(ctx,
		`SELECT id, name, type, enabled, host, port, transport, username, password,
		 auth_username, register_expiry, remote_hosts, local_host, codecs,
		 max_channels, caller_id_name, caller_id_num, prefix_strip, prefix_add,
		 priority, created_at, updated_at
		 FROM trunks WHERE id = ?`, id,
	))
}

// List returns all trunks ordered by priority then name.
func (r *trunkRepo) List(ctx context.Context) ([]models.Trunk, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, type, enabled, host, port, transport, username, password,
		 auth_username, register_expiry, remote_hosts, local_host, codecs,
		 max_channels, caller_id_name, caller_id_num, prefix_strip, prefix_add,
		 priority, created_at, updated_at
		 FROM trunks ORDER BY priority, name`)
	if err != nil {
		return nil, fmt.Errorf("querying trunks: %w", err)
	}
	defer rows.Close()

	return r.scanMany(rows)
}

// ListEnabled returns all enabled trunks ordered by priority then name.
func (r *trunkRepo) ListEnabled(ctx context.Context) ([]models.Trunk, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, type, enabled, host, port, transport, username, password,
		 auth_username, register_expiry, remote_hosts, local_host, codecs,
		 max_channels, caller_id_name, caller_id_num, prefix_strip, prefix_add,
		 priority, created_at, updated_at
		 FROM trunks WHERE enabled = 1 ORDER BY priority, name`)
	if err != nil {
		return nil, fmt.Errorf("querying enabled trunks: %w", err)
	}
	defer rows.Close()

	return r.scanMany(rows)
}

// Update modifies an existing trunk.
func (r *trunkRepo) Update(ctx context.Context, trunk *models.Trunk) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE trunks SET name = ?, type = ?, enabled = ?, host = ?, port = ?,
		 transport = ?, username = ?, password = ?, auth_username = ?,
		 register_expiry = ?, remote_hosts = ?, local_host = ?, codecs = ?,
		 max_channels = ?, caller_id_name = ?, caller_id_num = ?, prefix_strip = ?,
		 prefix_add = ?, priority = ?, updated_at = datetime('now')
		 WHERE id = ?`,
		trunk.Name, trunk.Type, trunk.Enabled, trunk.Host, trunk.Port, trunk.Transport,
		trunk.Username, trunk.Password, trunk.AuthUsername, trunk.RegisterExpiry,
		trunk.RemoteHosts, trunk.LocalHost, trunk.Codecs, trunk.MaxChannels,
		trunk.CallerIDName, trunk.CallerIDNum, trunk.PrefixStrip, trunk.PrefixAdd,
		trunk.Priority, trunk.ID,
	)
	if err != nil {
		return fmt.Errorf("updating trunk: %w", err)
	}
	return nil
}

// Delete removes a trunk by ID.
func (r *trunkRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM trunks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting trunk: %w", err)
	}
	return nil
}

func (r *trunkRepo) scanOne(row *sql.Row) (*models.Trunk, error) {
	var t models.Trunk
	err := row.Scan(&t.ID, &t.Name, &t.Type, &t.Enabled, &t.Host, &t.Port,
		&t.Transport, &t.Username, &t.Password, &t.AuthUsername, &t.RegisterExpiry,
		&t.RemoteHosts, &t.LocalHost, &t.Codecs, &t.MaxChannels, &t.CallerIDName,
		&t.CallerIDNum, &t.PrefixStrip, &t.PrefixAdd, &t.Priority,
		&t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning trunk: %w", err)
	}
	return &t, nil
}

func (r *trunkRepo) scanMany(rows *sql.Rows) ([]models.Trunk, error) {
	var trunks []models.Trunk
	for rows.Next() {
		var t models.Trunk
		if err := rows.Scan(&t.ID, &t.Name, &t.Type, &t.Enabled, &t.Host, &t.Port,
			&t.Transport, &t.Username, &t.Password, &t.AuthUsername, &t.RegisterExpiry,
			&t.RemoteHosts, &t.LocalHost, &t.Codecs, &t.MaxChannels, &t.CallerIDName,
			&t.CallerIDNum, &t.PrefixStrip, &t.PrefixAdd, &t.Priority,
			&t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning trunk row: %w", err)
		}
		trunks = append(trunks, t)
	}
	return trunks, rows.Err()
}
