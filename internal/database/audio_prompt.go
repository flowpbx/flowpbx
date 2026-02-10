package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/flowpbx/flowpbx/internal/database/models"
)

// audioPromptRepo implements AudioPromptRepository.
type audioPromptRepo struct {
	db *DB
}

// NewAudioPromptRepository creates a new AudioPromptRepository.
func NewAudioPromptRepository(db *DB) AudioPromptRepository {
	return &audioPromptRepo{db: db}
}

// Create inserts a new audio prompt record.
func (r *audioPromptRepo) Create(ctx context.Context, prompt *models.AudioPrompt) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO audio_prompts (name, filename, format, file_size, file_path, created_at)
		 VALUES (?, ?, ?, ?, ?, datetime('now'))`,
		prompt.Name, prompt.Filename, prompt.Format, prompt.FileSize, prompt.FilePath,
	)
	if err != nil {
		return fmt.Errorf("inserting audio prompt: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}
	prompt.ID = id
	return nil
}

// GetByID returns an audio prompt by ID.
func (r *audioPromptRepo) GetByID(ctx context.Context, id int64) (*models.AudioPrompt, error) {
	return r.scanOne(r.db.QueryRowContext(ctx,
		`SELECT id, name, filename, format, file_size, file_path, created_at
		 FROM audio_prompts WHERE id = ?`, id,
	))
}

// List returns all audio prompts ordered by name.
func (r *audioPromptRepo) List(ctx context.Context) ([]models.AudioPrompt, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, filename, format, file_size, file_path, created_at
		 FROM audio_prompts ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("querying audio prompts: %w", err)
	}
	defer rows.Close()

	var prompts []models.AudioPrompt
	for rows.Next() {
		var p models.AudioPrompt
		if err := rows.Scan(&p.ID, &p.Name, &p.Filename, &p.Format, &p.FileSize,
			&p.FilePath, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning audio prompt row: %w", err)
		}
		prompts = append(prompts, p)
	}
	return prompts, rows.Err()
}

// Delete removes an audio prompt by ID.
func (r *audioPromptRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM audio_prompts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting audio prompt: %w", err)
	}
	return nil
}

func (r *audioPromptRepo) scanOne(row *sql.Row) (*models.AudioPrompt, error) {
	var p models.AudioPrompt
	err := row.Scan(&p.ID, &p.Name, &p.Filename, &p.Format, &p.FileSize,
		&p.FilePath, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning audio prompt: %w", err)
	}
	return &p, nil
}
