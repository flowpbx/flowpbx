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

// List returns CDRs matching the filter, along with the total count.
func (r *cdrRepo) List(ctx context.Context, filter CDRListFilter) ([]models.CDR, int, error) {
	where := "1=1"
	args := []any{}

	if filter.Direction != "" {
		where += " AND direction = ?"
		args = append(args, filter.Direction)
	}
	if filter.Search != "" {
		where += " AND (caller_id_name LIKE ? OR caller_id_num LIKE ? OR callee LIKE ?)"
		s := "%" + filter.Search + "%"
		args = append(args, s, s, s)
	}
	if filter.StartDate != "" {
		where += " AND start_time >= ?"
		args = append(args, filter.StartDate)
	}
	if filter.EndDate != "" {
		where += " AND start_time <= ?"
		args = append(args, filter.EndDate)
	}

	// Count total matching rows.
	var total int
	countQuery := "SELECT COUNT(*) FROM cdrs WHERE " + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting cdrs: %w", err)
	}

	// Fetch the page of results.
	query := `SELECT id, call_id, start_time, answer_time, end_time, duration,
		 billable_dur, caller_id_name, caller_id_num, callee, trunk_id,
		 direction, disposition, recording_file, flow_path, hangup_cause
		 FROM cdrs WHERE ` + where + ` ORDER BY start_time DESC LIMIT ? OFFSET ?`
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing cdrs: %w", err)
	}
	defer rows.Close()

	var cdrs []models.CDR
	for rows.Next() {
		var c models.CDR
		if err := rows.Scan(&c.ID, &c.CallID, &c.StartTime, &c.AnswerTime, &c.EndTime,
			&c.Duration, &c.BillableDur, &c.CallerIDName, &c.CallerIDNum,
			&c.Callee, &c.TrunkID, &c.Direction, &c.Disposition,
			&c.RecordingFile, &c.FlowPath, &c.HangupCause); err != nil {
			return nil, 0, fmt.Errorf("scanning cdr row: %w", err)
		}
		cdrs = append(cdrs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating cdr rows: %w", err)
	}

	return cdrs, total, nil
}

// ListWithRecordings returns CDRs that have a non-empty recording_file,
// with the same filtering and pagination as List.
func (r *cdrRepo) ListWithRecordings(ctx context.Context, filter CDRListFilter) ([]models.CDR, int, error) {
	where := "recording_file IS NOT NULL AND recording_file != ''"
	args := []any{}

	if filter.Search != "" {
		where += " AND (caller_id_name LIKE ? OR caller_id_num LIKE ? OR callee LIKE ?)"
		s := "%" + filter.Search + "%"
		args = append(args, s, s, s)
	}
	if filter.StartDate != "" {
		where += " AND start_time >= ?"
		args = append(args, filter.StartDate)
	}
	if filter.EndDate != "" {
		where += " AND start_time <= ?"
		args = append(args, filter.EndDate)
	}

	// Count total matching rows.
	var total int
	countQuery := "SELECT COUNT(*) FROM cdrs WHERE " + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting recordings: %w", err)
	}

	// Fetch the page of results.
	query := `SELECT id, call_id, start_time, answer_time, end_time, duration,
		 billable_dur, caller_id_name, caller_id_num, callee, trunk_id,
		 direction, disposition, recording_file, flow_path, hangup_cause
		 FROM cdrs WHERE ` + where + ` ORDER BY start_time DESC LIMIT ? OFFSET ?`
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing recordings: %w", err)
	}
	defer rows.Close()

	var cdrs []models.CDR
	for rows.Next() {
		var c models.CDR
		if err := rows.Scan(&c.ID, &c.CallID, &c.StartTime, &c.AnswerTime, &c.EndTime,
			&c.Duration, &c.BillableDur, &c.CallerIDName, &c.CallerIDNum,
			&c.Callee, &c.TrunkID, &c.Direction, &c.Disposition,
			&c.RecordingFile, &c.FlowPath, &c.HangupCause); err != nil {
			return nil, 0, fmt.Errorf("scanning recording row: %w", err)
		}
		cdrs = append(cdrs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating recording rows: %w", err)
	}

	return cdrs, total, nil
}

// ListRecent returns the most recent CDRs up to the given limit.
func (r *cdrRepo) ListRecent(ctx context.Context, limit int) ([]models.CDR, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, call_id, start_time, answer_time, end_time, duration,
		 billable_dur, caller_id_name, caller_id_num, callee, trunk_id,
		 direction, disposition, recording_file, flow_path, hangup_cause
		 FROM cdrs ORDER BY start_time DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("listing recent cdrs: %w", err)
	}
	defer rows.Close()

	var cdrs []models.CDR
	for rows.Next() {
		var c models.CDR
		if err := rows.Scan(&c.ID, &c.CallID, &c.StartTime, &c.AnswerTime, &c.EndTime,
			&c.Duration, &c.BillableDur, &c.CallerIDName, &c.CallerIDNum,
			&c.Callee, &c.TrunkID, &c.Direction, &c.Disposition,
			&c.RecordingFile, &c.FlowPath, &c.HangupCause); err != nil {
			return nil, fmt.Errorf("scanning recent cdr row: %w", err)
		}
		cdrs = append(cdrs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating recent cdr rows: %w", err)
	}

	return cdrs, nil
}

// CountRecordings returns the number of CDRs that have a non-empty recording_file.
func (r *cdrRepo) CountRecordings(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM cdrs WHERE recording_file IS NOT NULL AND recording_file != ''`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting recordings: %w", err)
	}
	return count, nil
}

// DeleteExpiredRecordings clears the recording_file field on CDRs whose
// start_time is older than the given number of days and that have a non-empty
// recording_file. Returns the file paths of the cleared recordings so callers
// can remove the WAV files from disk.
func (r *cdrRepo) DeleteExpiredRecordings(ctx context.Context, days int) ([]string, error) {
	// Select recording file paths that will be cleared.
	rows, err := r.db.QueryContext(ctx,
		`SELECT recording_file FROM cdrs
		 WHERE recording_file IS NOT NULL AND recording_file != ''
		 AND start_time < datetime('now', '-' || ? || ' days')`, days)
	if err != nil {
		return nil, fmt.Errorf("querying expired recordings: %w", err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scanning expired recording path: %w", err)
		}
		paths = append(paths, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating expired recording rows: %w", err)
	}

	if len(paths) == 0 {
		return nil, nil
	}

	// Clear the recording_file field on expired CDRs (don't delete the CDR itself).
	_, err = r.db.ExecContext(ctx,
		`UPDATE cdrs SET recording_file = ''
		 WHERE recording_file IS NOT NULL AND recording_file != ''
		 AND start_time < datetime('now', '-' || ? || ' days')`, days)
	if err != nil {
		return nil, fmt.Errorf("clearing expired recording paths: %w", err)
	}

	return paths, nil
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
