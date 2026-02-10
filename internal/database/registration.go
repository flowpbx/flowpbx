package database

import (
	"context"
	"fmt"

	"github.com/flowpbx/flowpbx/internal/database/models"
)

// registrationRepo implements RegistrationRepository.
type registrationRepo struct {
	db *DB
}

// NewRegistrationRepository creates a new RegistrationRepository.
func NewRegistrationRepository(db *DB) RegistrationRepository {
	return &registrationRepo{db: db}
}

// Create inserts a new registration.
func (r *registrationRepo) Create(ctx context.Context, reg *models.Registration) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO registrations (extension_id, contact_uri, transport, user_agent,
		 source_ip, source_port, expires, registered_at, push_token, push_platform, device_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'), ?, ?, ?)`,
		reg.ExtensionID, reg.ContactURI, reg.Transport, reg.UserAgent,
		reg.SourceIP, reg.SourcePort, reg.Expires,
		reg.PushToken, reg.PushPlatform, reg.DeviceID,
	)
	if err != nil {
		return fmt.Errorf("inserting registration: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}
	reg.ID = id
	return nil
}

// GetByExtensionID returns all active registrations for an extension.
func (r *registrationRepo) GetByExtensionID(ctx context.Context, extensionID int64) ([]models.Registration, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, extension_id, contact_uri, transport, user_agent,
		 source_ip, source_port, expires, registered_at, push_token, push_platform, device_id
		 FROM registrations WHERE extension_id = ? ORDER BY registered_at DESC`, extensionID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying registrations by extension: %w", err)
	}
	defer rows.Close()

	var regs []models.Registration
	for rows.Next() {
		var reg models.Registration
		if err := rows.Scan(&reg.ID, &reg.ExtensionID, &reg.ContactURI, &reg.Transport,
			&reg.UserAgent, &reg.SourceIP, &reg.SourcePort, &reg.Expires,
			&reg.RegisteredAt, &reg.PushToken, &reg.PushPlatform, &reg.DeviceID); err != nil {
			return nil, fmt.Errorf("scanning registration row: %w", err)
		}
		regs = append(regs, reg)
	}
	return regs, rows.Err()
}

// DeleteByID removes a registration by ID.
func (r *registrationRepo) DeleteByID(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM registrations WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting registration: %w", err)
	}
	return nil
}

// DeleteExpired removes all registrations whose expires time has passed.
// Returns the number of rows deleted.
func (r *registrationRepo) DeleteExpired(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM registrations WHERE expires < datetime('now')`)
	if err != nil {
		return 0, fmt.Errorf("deleting expired registrations: %w", err)
	}
	return result.RowsAffected()
}

// DeleteAll removes all registrations. Used on startup to clear stale state.
func (r *registrationRepo) DeleteAll(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM registrations`)
	if err != nil {
		return 0, fmt.Errorf("deleting all registrations: %w", err)
	}
	return result.RowsAffected()
}

// DeleteByExtensionAndContact removes a registration matching an extension
// and contact URI. Used to update/re-register from the same device.
func (r *registrationRepo) DeleteByExtensionAndContact(ctx context.Context, extensionID int64, contactURI string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM registrations WHERE extension_id = ? AND contact_uri = ?`,
		extensionID, contactURI)
	if err != nil {
		return fmt.Errorf("deleting registration by extension and contact: %w", err)
	}
	return nil
}

// CountByExtensionID returns the number of active registrations for an extension.
func (r *registrationRepo) CountByExtensionID(ctx context.Context, extensionID int64) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM registrations WHERE extension_id = ?`, extensionID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting registrations: %w", err)
	}
	return count, nil
}
