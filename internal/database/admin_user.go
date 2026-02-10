package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/flowpbx/flowpbx/internal/database/models"
)

// adminUserRepo implements AdminUserRepository.
type adminUserRepo struct {
	db *DB
}

// NewAdminUserRepository creates a new AdminUserRepository.
func NewAdminUserRepository(db *DB) AdminUserRepository {
	return &adminUserRepo{db: db}
}

// Create inserts a new admin user.
func (r *adminUserRepo) Create(ctx context.Context, user *models.AdminUser) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO admin_users (username, password_hash, totp_secret, created_at, updated_at)
		 VALUES (?, ?, ?, datetime('now'), datetime('now'))`,
		user.Username, user.PasswordHash, user.TOTPSecret,
	)
	if err != nil {
		return fmt.Errorf("inserting admin user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}
	user.ID = id
	return nil
}

// GetByID returns an admin user by ID.
func (r *adminUserRepo) GetByID(ctx context.Context, id int64) (*models.AdminUser, error) {
	var u models.AdminUser
	err := r.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, totp_secret, created_at, updated_at
		 FROM admin_users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.TOTPSecret, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying admin user by id: %w", err)
	}
	return &u, nil
}

// GetByUsername returns an admin user by username.
func (r *adminUserRepo) GetByUsername(ctx context.Context, username string) (*models.AdminUser, error) {
	var u models.AdminUser
	err := r.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, totp_secret, created_at, updated_at
		 FROM admin_users WHERE username = ?`, username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.TOTPSecret, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying admin user by username: %w", err)
	}
	return &u, nil
}

// List returns all admin users.
func (r *adminUserRepo) List(ctx context.Context) ([]models.AdminUser, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, username, password_hash, totp_secret, created_at, updated_at
		 FROM admin_users ORDER BY username`)
	if err != nil {
		return nil, fmt.Errorf("querying admin users: %w", err)
	}
	defer rows.Close()

	var users []models.AdminUser
	for rows.Next() {
		var u models.AdminUser
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.TOTPSecret, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning admin user row: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// Update modifies an existing admin user.
func (r *adminUserRepo) Update(ctx context.Context, user *models.AdminUser) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE admin_users SET username = ?, password_hash = ?, totp_secret = ?, updated_at = datetime('now')
		 WHERE id = ?`,
		user.Username, user.PasswordHash, user.TOTPSecret, user.ID,
	)
	if err != nil {
		return fmt.Errorf("updating admin user: %w", err)
	}
	return nil
}

// Delete removes an admin user by ID.
func (r *adminUserRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM admin_users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting admin user: %w", err)
	}
	return nil
}

// Count returns the total number of admin users.
func (r *adminUserRepo) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_users`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting admin users: %w", err)
	}
	return count, nil
}
