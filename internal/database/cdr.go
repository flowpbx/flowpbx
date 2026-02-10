package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/flowpbx/flowpbx/internal/database/models"
)

// cdrRepo implements CDRRepository.
type cdrRepo struct {
	db *DB
}

// NewCDRRepository creates a new CDRRepository.
func NewCDRRepository(db *DB) CDRRepository {
	return &cdrRepo{db: db}
}

// Create inserts a new call detail record.
func (r *cdrRepo) Create(ctx context.Context, cdr *models.CDR) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO cdrs (call_id, start_time, answer_time, end_time, duration,
		 billable_dur, caller_id_name, caller_id_num, callee, trunk_id,
		 direction, disposition, recording_file, flow_path, hangup_cause)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cdr.CallID, cdr.StartTime, cdr.AnswerTime, cdr.EndTime, cdr.Duration,
		cdr.BillableDur, cdr.CallerIDName, cdr.CallerIDNum, cdr.Callee,
		cdr.TrunkID, cdr.Direction, cdr.Disposition, cdr.RecordingFile,
		cdr.FlowPath, cdr.HangupCause,
	)
	if err != nil {
		return fmt.Errorf("inserting cdr: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}
	cdr.ID = id
	return nil
}

// GetByID returns a CDR by ID.
func (r *cdrRepo) GetByID(ctx context.Context, id int64) (*models.CDR, error) {
	return r.scanOne(r.db.QueryRowContext(ctx,
		`SELECT id, call_id, start_time, answer_time, end_time, duration,
		 billable_dur, caller_id_name, caller_id_num, callee, trunk_id,
		 direction, disposition, recording_file, flow_path, hangup_cause
		 FROM cdrs WHERE id = ?`, id,
	))
}

// GetByCallID returns a CDR by SIP Call-ID.
func (r *cdrRepo) GetByCallID(ctx context.Context, callID string) (*models.CDR, error) {
	return r.scanOne(r.db.QueryRowContext(ctx,
		`SELECT id, call_id, start_time, answer_time, end_time, duration,
		 billable_dur, caller_id_name, caller_id_num, callee, trunk_id,
		 direction, disposition, recording_file, flow_path, hangup_cause
		 FROM cdrs WHERE call_id = ?`, callID,
	))
}

// Update modifies an existing CDR.
func (r *cdrRepo) Update(ctx context.Context, cdr *models.CDR) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE cdrs SET call_id = ?, start_time = ?, answer_time = ?, end_time = ?,
		 duration = ?, billable_dur = ?, caller_id_name = ?, caller_id_num = ?,
		 callee = ?, trunk_id = ?, direction = ?, disposition = ?,
		 recording_file = ?, flow_path = ?, hangup_cause = ?
		 WHERE id = ?`,
		cdr.CallID, cdr.StartTime, cdr.AnswerTime, cdr.EndTime, cdr.Duration,
		cdr.BillableDur, cdr.CallerIDName, cdr.CallerIDNum, cdr.Callee,
		cdr.TrunkID, cdr.Direction, cdr.Disposition, cdr.RecordingFile,
		cdr.FlowPath, cdr.HangupCause, cdr.ID,
	)
	if err != nil {
		return fmt.Errorf("updating cdr: %w", err)
	}
	return nil
}

func (r *cdrRepo) scanOne(row *sql.Row) (*models.CDR, error) {
	var c models.CDR
	err := row.Scan(&c.ID, &c.CallID, &c.StartTime, &c.AnswerTime, &c.EndTime,
		&c.Duration, &c.BillableDur, &c.CallerIDName, &c.CallerIDNum,
		&c.Callee, &c.TrunkID, &c.Direction, &c.Disposition,
		&c.RecordingFile, &c.FlowPath, &c.HangupCause)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning cdr: %w", err)
	}
	return &c, nil
}
