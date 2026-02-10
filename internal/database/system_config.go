package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/flowpbx/flowpbx/internal/database/models"
)

// systemConfigRepo implements SystemConfigRepository with an in-memory cache.
type systemConfigRepo struct {
	db    *DB
	mu    sync.RWMutex
	cache map[string]string
}

// NewSystemConfigRepository creates a new SystemConfigRepository backed by the
// given database. It loads all config into memory on creation.
func NewSystemConfigRepository(ctx context.Context, db *DB) (SystemConfigRepository, error) {
	repo := &systemConfigRepo{
		db:    db,
		cache: make(map[string]string),
	}

	if err := repo.loadAll(ctx); err != nil {
		return nil, fmt.Errorf("loading system config: %w", err)
	}

	return repo, nil
}

// Get returns the value for the given key. Returns empty string if not found.
func (r *systemConfigRepo) Get(_ context.Context, key string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cache[key], nil
}

// Set inserts or updates a key-value pair in both the database and cache.
func (r *systemConfigRepo) Set(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO system_config (key, value, updated_at)
		 VALUES (?, ?, datetime('now'))
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("setting config %q: %w", key, err)
	}

	r.mu.Lock()
	r.cache[key] = value
	r.mu.Unlock()

	return nil
}

// GetAll returns all system config entries.
func (r *systemConfigRepo) GetAll(ctx context.Context) ([]models.SystemConfig, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, key, value, updated_at FROM system_config ORDER BY key")
	if err != nil {
		return nil, fmt.Errorf("querying system config: %w", err)
	}
	defer rows.Close()

	var configs []models.SystemConfig
	for rows.Next() {
		var c models.SystemConfig
		if err := rows.Scan(&c.ID, &c.Key, &c.Value, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning system config row: %w", err)
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

// loadAll reads all config entries from the database into the in-memory cache.
func (r *systemConfigRepo) loadAll(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, "SELECT key, value FROM system_config")
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return fmt.Errorf("querying system config: %w", err)
	}
	defer rows.Close()

	r.mu.Lock()
	defer r.mu.Unlock()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return fmt.Errorf("scanning config row: %w", err)
		}
		r.cache[key] = value
	}

	return rows.Err()
}
