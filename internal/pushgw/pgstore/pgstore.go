package pgstore

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/flowpbx/flowpbx/internal/pushgw"
	"github.com/google/uuid"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store implements pushgw.LicenseStore and pushgw.PushLogger using PostgreSQL.
type Store struct {
	db *sql.DB
}

// New opens a PostgreSQL connection and runs pending migrations.
func New(dsn string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening postgresql: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging postgresql: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	s := &Store{db: db}

	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	slog.Info("postgresql store opened")
	return s, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// migrate runs all pending SQL migration files in order.
func (s *Store) migrate() error {
	// Ensure schema_migrations table exists.
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	if err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("reading migrations directory: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		version := strings.TrimSuffix(entry.Name(), ".sql")

		var count int
		err := s.db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = $1", version).Scan(&count)
		if err != nil {
			return fmt.Errorf("checking migration %s: %w", version, err)
		}
		if count > 0 {
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", version, err)
		}

		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("beginning transaction for migration %s: %w", version, err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("executing migration %s: %w", version, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
			tx.Rollback()
			return fmt.Errorf("recording migration %s: %w", version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %s: %w", version, err)
		}

		slog.Info("applied migration", "version", version)
	}

	return nil
}

// ValidateLicense checks a license key and returns the license if valid.
// Returns nil, nil if the license is not found or expired.
func (s *Store) ValidateLicense(key string) (*pushgw.License, error) {
	var l pushgw.License
	err := s.db.QueryRow(
		`SELECT id, key, tier, max_extensions, expires_at, created_at
		 FROM licenses
		 WHERE key = $1
		   AND (expires_at IS NULL OR expires_at > NOW())`,
		key,
	).Scan(&l.ID, &l.Key, &l.Tier, &l.MaxExtensions, &l.ExpiresAt, &l.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying license: %w", err)
	}
	return &l, nil
}

// ActivateLicense registers a new installation for a license key.
// Returns nil, nil if the license is not found or expired.
func (s *Store) ActivateLicense(key string, hostname string, version string) (*pushgw.Installation, error) {
	license, err := s.ValidateLicense(key)
	if err != nil {
		return nil, err
	}
	if license == nil {
		return nil, nil
	}

	instanceID := uuid.NewString()
	var inst pushgw.Installation
	err = s.db.QueryRow(
		`INSERT INTO installations (license_id, instance_id, hostname, version)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, license_id, instance_id, hostname, version, activated_at, last_seen_at`,
		license.ID, instanceID, hostname, version,
	).Scan(&inst.ID, &inst.LicenseID, &inst.InstanceID, &inst.Hostname, &inst.Version, &inst.ActivatedAt, &inst.LastSeenAt)

	if err != nil {
		return nil, fmt.Errorf("inserting installation: %w", err)
	}

	return &inst, nil
}

// GetLicenseStatus returns license details and installation count.
// Returns nil, nil if the license is not found.
func (s *Store) GetLicenseStatus(key string) (*pushgw.LicenseStatus, error) {
	var ls pushgw.LicenseStatus
	var expiresAt *time.Time

	err := s.db.QueryRow(
		`SELECT l.key, l.tier, l.max_extensions, l.expires_at,
		        COUNT(i.id) AS installation_count
		 FROM licenses l
		 LEFT JOIN installations i ON i.license_id = l.id
		 WHERE l.key = $1
		 GROUP BY l.id`,
		key,
	).Scan(&ls.Key, &ls.Tier, &ls.MaxExtensions, &expiresAt, &ls.InstallationCount)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying license status: %w", err)
	}

	ls.ExpiresAt = expiresAt
	ls.Active = expiresAt == nil || expiresAt.After(time.Now())

	return &ls, nil
}

// Log records the result of a push delivery attempt.
func (s *Store) Log(entry pushgw.PushLogEntry) error {
	_, err := s.db.Exec(
		`INSERT INTO push_logs (license_key, platform, call_id, success, error)
		 VALUES ($1, $2, $3, $4, $5)`,
		entry.LicenseKey, entry.Platform, entry.CallID, entry.Success, entry.Error,
	)
	if err != nil {
		return fmt.Errorf("inserting push log: %w", err)
	}
	return nil
}
