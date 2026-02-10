package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/flowpbx/flowpbx/internal/database/models"
)

// voicemailMessageRepo implements VoicemailMessageRepository.
type voicemailMessageRepo struct {
	db *DB
}

// NewVoicemailMessageRepository creates a new VoicemailMessageRepository.
func NewVoicemailMessageRepository(db *DB) VoicemailMessageRepository {
	return &voicemailMessageRepo{db: db}
}

// Create inserts a new voicemail message.
func (r *voicemailMessageRepo) Create(ctx context.Context, msg *models.VoicemailMessage) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO voicemail_messages (mailbox_id, caller_id_name, caller_id_num,
		 timestamp, duration, file_path, read, read_at, transcription, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, 0, NULL, '', datetime('now'))`,
		msg.MailboxID, msg.CallerIDName, msg.CallerIDNum,
		msg.Timestamp, msg.Duration, msg.FilePath,
	)
	if err != nil {
		return fmt.Errorf("inserting voicemail message: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}
	msg.ID = id
	return nil
}

// GetByID returns a voicemail message by ID.
func (r *voicemailMessageRepo) GetByID(ctx context.Context, id int64) (*models.VoicemailMessage, error) {
	return r.scanOne(r.db.QueryRowContext(ctx,
		`SELECT id, mailbox_id, caller_id_name, caller_id_num, timestamp,
		 duration, file_path, read, read_at, transcription, created_at
		 FROM voicemail_messages WHERE id = ?`, id,
	))
}

// ListByMailbox returns all messages for a given mailbox, ordered by timestamp descending.
func (r *voicemailMessageRepo) ListByMailbox(ctx context.Context, mailboxID int64) ([]models.VoicemailMessage, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, mailbox_id, caller_id_name, caller_id_num, timestamp,
		 duration, file_path, read, read_at, transcription, created_at
		 FROM voicemail_messages WHERE mailbox_id = ? ORDER BY timestamp DESC`, mailboxID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying voicemail messages: %w", err)
	}
	defer rows.Close()

	var msgs []models.VoicemailMessage
	for rows.Next() {
		var m models.VoicemailMessage
		if err := rows.Scan(&m.ID, &m.MailboxID, &m.CallerIDName, &m.CallerIDNum,
			&m.Timestamp, &m.Duration, &m.FilePath, &m.Read, &m.ReadAt,
			&m.Transcription, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning voicemail message row: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// MarkRead marks a voicemail message as read.
func (r *voicemailMessageRepo) MarkRead(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE voicemail_messages SET read = 1, read_at = datetime('now') WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("marking voicemail message as read: %w", err)
	}
	return nil
}

// Delete removes a voicemail message by ID.
func (r *voicemailMessageRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM voicemail_messages WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting voicemail message: %w", err)
	}
	return nil
}

func (r *voicemailMessageRepo) scanOne(row *sql.Row) (*models.VoicemailMessage, error) {
	var m models.VoicemailMessage
	err := row.Scan(&m.ID, &m.MailboxID, &m.CallerIDName, &m.CallerIDNum,
		&m.Timestamp, &m.Duration, &m.FilePath, &m.Read, &m.ReadAt,
		&m.Transcription, &m.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning voicemail message: %w", err)
	}
	return &m, nil
}
