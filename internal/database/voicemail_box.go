package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/flowpbx/flowpbx/internal/database/models"
)

// voicemailBoxRepo implements VoicemailBoxRepository.
type voicemailBoxRepo struct {
	db *DB
}

// NewVoicemailBoxRepository creates a new VoicemailBoxRepository.
func NewVoicemailBoxRepository(db *DB) VoicemailBoxRepository {
	return &voicemailBoxRepo{db: db}
}

// Create inserts a new voicemail box.
func (r *voicemailBoxRepo) Create(ctx context.Context, box *models.VoicemailBox) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO voicemail_boxes (name, mailbox_number, pin, greeting_file,
		 greeting_type, email_notify, email_address, email_attach_audio,
		 max_message_duration, max_messages, retention_days, notify_extension_id,
		 created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		box.Name, box.MailboxNumber, box.PIN, box.GreetingFile,
		box.GreetingType, box.EmailNotify, box.EmailAddress, box.EmailAttachAudio,
		box.MaxMessageDuration, box.MaxMessages, box.RetentionDays, box.NotifyExtensionID,
	)
	if err != nil {
		return fmt.Errorf("inserting voicemail box: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}
	box.ID = id
	return nil
}

// GetByID returns a voicemail box by ID.
func (r *voicemailBoxRepo) GetByID(ctx context.Context, id int64) (*models.VoicemailBox, error) {
	return r.scanOne(r.db.QueryRowContext(ctx,
		`SELECT id, name, mailbox_number, pin, greeting_file, greeting_type,
		 email_notify, email_address, email_attach_audio, max_message_duration,
		 max_messages, retention_days, notify_extension_id, created_at, updated_at
		 FROM voicemail_boxes WHERE id = ?`, id,
	))
}

// List returns all voicemail boxes ordered by name.
func (r *voicemailBoxRepo) List(ctx context.Context) ([]models.VoicemailBox, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, mailbox_number, pin, greeting_file, greeting_type,
		 email_notify, email_address, email_attach_audio, max_message_duration,
		 max_messages, retention_days, notify_extension_id, created_at, updated_at
		 FROM voicemail_boxes ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("querying voicemail boxes: %w", err)
	}
	defer rows.Close()

	var boxes []models.VoicemailBox
	for rows.Next() {
		var b models.VoicemailBox
		if err := rows.Scan(&b.ID, &b.Name, &b.MailboxNumber, &b.PIN, &b.GreetingFile,
			&b.GreetingType, &b.EmailNotify, &b.EmailAddress, &b.EmailAttachAudio,
			&b.MaxMessageDuration, &b.MaxMessages, &b.RetentionDays, &b.NotifyExtensionID,
			&b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning voicemail box row: %w", err)
		}
		boxes = append(boxes, b)
	}
	return boxes, rows.Err()
}

// Update modifies an existing voicemail box.
func (r *voicemailBoxRepo) Update(ctx context.Context, box *models.VoicemailBox) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE voicemail_boxes SET name = ?, mailbox_number = ?, pin = ?,
		 greeting_file = ?, greeting_type = ?, email_notify = ?, email_address = ?,
		 email_attach_audio = ?, max_message_duration = ?, max_messages = ?,
		 retention_days = ?, notify_extension_id = ?, updated_at = datetime('now')
		 WHERE id = ?`,
		box.Name, box.MailboxNumber, box.PIN, box.GreetingFile, box.GreetingType,
		box.EmailNotify, box.EmailAddress, box.EmailAttachAudio,
		box.MaxMessageDuration, box.MaxMessages, box.RetentionDays,
		box.NotifyExtensionID, box.ID,
	)
	if err != nil {
		return fmt.Errorf("updating voicemail box: %w", err)
	}
	return nil
}

// Delete removes a voicemail box by ID.
func (r *voicemailBoxRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM voicemail_boxes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting voicemail box: %w", err)
	}
	return nil
}

func (r *voicemailBoxRepo) scanOne(row *sql.Row) (*models.VoicemailBox, error) {
	var b models.VoicemailBox
	err := row.Scan(&b.ID, &b.Name, &b.MailboxNumber, &b.PIN, &b.GreetingFile,
		&b.GreetingType, &b.EmailNotify, &b.EmailAddress, &b.EmailAttachAudio,
		&b.MaxMessageDuration, &b.MaxMessages, &b.RetentionDays, &b.NotifyExtensionID,
		&b.CreatedAt, &b.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning voicemail box: %w", err)
	}
	return &b, nil
}
