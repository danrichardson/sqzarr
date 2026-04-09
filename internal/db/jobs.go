package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type JobStatus string

const (
	JobPending   JobStatus = "pending"
	JobRunning   JobStatus = "running"
	JobDone      JobStatus = "done"      // legacy: in-place replacement completed
	JobStaged    JobStatus = "staged"    // transcoded; original held in processed dir
	JobFailed    JobStatus = "failed"
	JobCancelled JobStatus = "cancelled"
	JobSkipped   JobStatus = "skipped"
	JobExcluded  JobStatus = "excluded"  // permanently skipped: uncompressible or repeated failure
	JobError     JobStatus = "error"     // I/O or permission error needing human intervention
	JobRestored  JobStatus = "restored"  // original moved back; transcoded copy deleted
)

type Job struct {
	ID             int64
	DirectoryID    sql.NullInt64
	SourcePath     string
	SourceSize     int64
	SourceCodec    string
	SourceDuration float64
	SourceBitrate  int64
	OutputPath     sql.NullString
	OutputSize     sql.NullInt64
	EncoderUsed    sql.NullString
	Status         JobStatus
	Priority       int
	ErrorMessage   sql.NullString
	Progress       float64
	BytesSaved     sql.NullInt64
	FailCount      int
	StartedAt      sql.NullTime
	FinishedAt     sql.NullTime
	CreatedAt      time.Time
}

const jobSelectFields = `
	id, directory_id, source_path, source_size, source_codec, source_duration,
	source_bitrate, output_path, output_size, encoder_used, status, priority,
	error_message, progress, bytes_saved, fail_count, started_at, finished_at, created_at
` // trailing newline ensures space before FROM in string concatenation

func scanJob(s interface {
	Scan(...any) error
}, j *Job) error {
	return s.Scan(
		&j.ID, &j.DirectoryID, &j.SourcePath, &j.SourceSize, &j.SourceCodec, &j.SourceDuration,
		&j.SourceBitrate, &j.OutputPath, &j.OutputSize, &j.EncoderUsed, &j.Status, &j.Priority,
		&j.ErrorMessage, &j.Progress, &j.BytesSaved, &j.FailCount, &j.StartedAt, &j.FinishedAt, &j.CreatedAt,
	)
}

func (db *DB) InsertJob(j *Job) (int64, error) {
	res, err := db.conn.Exec(`
		INSERT INTO jobs
		  (directory_id, source_path, source_size, source_codec, source_duration, source_bitrate, status, priority)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		j.DirectoryID, j.SourcePath, j.SourceSize, j.SourceCodec,
		j.SourceDuration, j.SourceBitrate, j.Status, j.Priority)
	if err != nil {
		return 0, fmt.Errorf("insert job: %w", err)
	}
	return res.LastInsertId()
}

func (db *DB) GetJob(id int64) (*Job, error) {
	j := &Job{}
	row := db.conn.QueryRow(`SELECT`+jobSelectFields+`FROM jobs WHERE id = ?`, id)
	if err := scanJob(row, j); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get job: %w", err)
	}
	return j, nil
}

// NextPendingJob returns the next job to process ordered by priority desc, created_at asc.
func (db *DB) NextPendingJob() (*Job, error) {
	j := &Job{}
	row := db.conn.QueryRow(`SELECT` + jobSelectFields + `FROM jobs WHERE status = 'pending'
		ORDER BY priority DESC, created_at ASC LIMIT 1`)
	if err := scanJob(row, j); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("next pending job: %w", err)
	}
	return j, nil
}

func (db *DB) ListJobs(status JobStatus, limit, offset int) ([]*Job, error) {
	query := `SELECT` + jobSelectFields + `FROM jobs`
	args := []any{}

	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC"
	if limit > 0 {
		query += " LIMIT ? OFFSET ?"
		args = append(args, limit, offset)
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*Job
	for rows.Next() {
		j := &Job{}
		if err := scanJob(rows, j); err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func (db *DB) UpdateJobStatus(id int64, status JobStatus, errMsg string) error {
	var err error
	terminalWithTime := status == JobDone || status == JobStaged || status == JobFailed ||
		status == JobCancelled || status == JobRestored || status == JobExcluded || status == JobError
	if status == JobRunning {
		_, err = db.conn.Exec(
			`UPDATE jobs SET status=?, started_at=CURRENT_TIMESTAMP, error_message=NULL WHERE id=?`,
			status, id)
	} else if terminalWithTime {
		var errVal any
		if errMsg != "" {
			errVal = errMsg
		}
		_, err = db.conn.Exec(
			`UPDATE jobs SET status=?, finished_at=CURRENT_TIMESTAMP, error_message=? WHERE id=?`,
			status, errVal, id)
	} else {
		_, err = db.conn.Exec(`UPDATE jobs SET status=?, error_message=? WHERE id=?`,
			status, errMsg, id)
	}
	if err != nil {
		return fmt.Errorf("update job status: %w", err)
	}
	return nil
}

func (db *DB) UpdateJobProgress(id int64, progress float64) error {
	_, err := db.conn.Exec(`UPDATE jobs SET progress=? WHERE id=?`, progress, id)
	if err != nil {
		return fmt.Errorf("update job progress: %w", err)
	}
	return nil
}

// IncrementFailCount increments fail_count and returns the new value.
func (db *DB) IncrementFailCount(id int64) (int, error) {
	_, err := db.conn.Exec(`UPDATE jobs SET fail_count = fail_count + 1 WHERE id=?`, id)
	if err != nil {
		return 0, fmt.Errorf("increment fail count: %w", err)
	}
	var count int
	err = db.conn.QueryRow(`SELECT fail_count FROM jobs WHERE id=?`, id).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("read fail count: %w", err)
	}
	return count, nil
}

// ExcludeJob marks a job as permanently excluded with a reason.
func (db *DB) ExcludeJob(id int64, reason string) error {
	_, err := db.conn.Exec(
		`UPDATE jobs SET status='excluded', error_message=?, finished_at=CURRENT_TIMESTAMP WHERE id=?`,
		reason, id)
	if err != nil {
		return fmt.Errorf("exclude job: %w", err)
	}
	return nil
}

// StageJob marks a job as staged (original held, transcoded in place).
func (db *DB) StageJob(id int64, outputPath string, outputSize int64, encoderUsed string, bytesSaved int64) error {
	_, err := db.conn.Exec(`
		UPDATE jobs
		SET status='staged', output_path=?, output_size=?, encoder_used=?,
		    bytes_saved=?, progress=1.0, finished_at=CURRENT_TIMESTAMP
		WHERE id=?`,
		outputPath, outputSize, encoderUsed, bytesSaved, id)
	if err != nil {
		return fmt.Errorf("stage job: %w", err)
	}
	return nil
}

// CompleteJob marks a job as done (used for legacy in-place replacement or testing).
func (db *DB) CompleteJob(id int64, outputPath string, outputSize int64, encoderUsed string, bytesSaved int64) error {
	_, err := db.conn.Exec(`
		UPDATE jobs
		SET status='done', output_path=?, output_size=?, encoder_used=?,
		    bytes_saved=?, progress=1.0, finished_at=CURRENT_TIMESTAMP
		WHERE id=?`,
		outputPath, outputSize, encoderUsed, bytesSaved, id)
	if err != nil {
		return fmt.Errorf("complete job: %w", err)
	}
	return nil
}

// ResetRunningJobs resets any jobs stuck in "running" state back to "pending".
// Called on startup to recover from unclean shutdowns.
func (db *DB) ResetRunningJobs() (int, error) {
	res, err := db.conn.Exec(
		`UPDATE jobs SET status='pending', started_at=NULL, progress=0 WHERE status='running'`)
	if err != nil {
		return 0, fmt.Errorf("reset running jobs: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// ClearHistory deletes terminal (non-active) jobs and their originals records.
// Processing state is preserved in the processed_files table (TKT-004), so
// cleared files will NOT be re-queued by the scanner.
func (db *DB) ClearHistory() (int, error) {
	terminalStatuses := `('done','staged','failed','cancelled','skipped','excluded','error','restored')`
	_, err := db.conn.Exec(`DELETE FROM originals WHERE job_id IN
		(SELECT id FROM jobs WHERE status IN ` + terminalStatuses + `)`)
	if err != nil {
		return 0, fmt.Errorf("clear originals refs: %w", err)
	}
	res, err := db.conn.Exec(
		`DELETE FROM jobs WHERE status IN ` + terminalStatuses)
	if err != nil {
		return 0, fmt.Errorf("clear history: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// JobMeta holds the DB fields needed for the file browser.
type JobMeta struct {
	Codec      string
	Bitrate    int64
	Duration   float64
	Status     string
	BytesSaved int64
}

// GetJobMetaByPaths returns the most recent job metadata keyed by source_path.
func (db *DB) GetJobMetaByPaths(paths []string) (map[string]*JobMeta, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	args := make([]any, len(paths))
	ph := make([]string, len(paths))
	for i, p := range paths {
		args[i] = p
		ph[i] = "?"
	}
	rows, err := db.conn.Query(fmt.Sprintf(`
		SELECT source_path, source_codec, source_bitrate, source_duration,
		       status, COALESCE(bytes_saved, 0)
		FROM jobs
		WHERE source_path IN (%s)
		ORDER BY created_at DESC`, strings.Join(ph, ",")), args...)
	if err != nil {
		return nil, fmt.Errorf("get job meta: %w", err)
	}
	defer rows.Close()

	out := make(map[string]*JobMeta, len(paths))
	for rows.Next() {
		var path string
		m := &JobMeta{}
		if err := rows.Scan(&path, &m.Codec, &m.Bitrate, &m.Duration, &m.Status, &m.BytesSaved); err != nil {
			continue
		}
		if _, exists := out[path]; !exists {
			out[path] = m
		}
	}
	return out, rows.Err()
}

// SourcePathStatus returns the current job status for path, or "" if no job exists.
// Used by the scanner to determine whether to enqueue a file.
func (db *DB) SourcePathStatus(path string) (JobStatus, error) {
	var status JobStatus
	err := db.conn.QueryRow(
		`SELECT status FROM jobs WHERE source_path=? ORDER BY created_at DESC LIMIT 1`, path).
		Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("source path status: %w", err)
	}
	return status, nil
}

// OutputPathExists returns true if a job exists with the given output_path.
// Used by the scanner to skip files that are the product of a previous transcode.
func (db *DB) OutputPathExists(path string) (bool, error) {
	var count int
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM jobs WHERE output_path=?`, path).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("output path exists: %w", err)
	}
	return count > 0, nil
}

// SourcePathExists returns true if a non-failed, non-excluded job exists for the given path.
// Deprecated: prefer SourcePathStatus for fine-grained control.
func (db *DB) SourcePathExists(path string) (bool, error) {
	status, err := db.SourcePathStatus(path)
	if err != nil {
		return false, err
	}
	if status == "" || status == JobFailed {
		return false, nil
	}
	return true, nil
}

// SavingsEntry holds per-file savings data for the breakdown view.
type SavingsEntry struct {
	ID         int64        `json:"id"`
	SourcePath string       `json:"source_path"`
	SourceSize int64        `json:"source_size"`
	OutputSize int64        `json:"output_size"`
	BytesSaved int64        `json:"bytes_saved"`
	FinishedAt sql.NullTime `json:"finished_at"`
}

// ListSavingsBreakdown returns per-file savings for jobs that saved space.
func (db *DB) ListSavingsBreakdown() ([]*SavingsEntry, error) {
	rows, err := db.conn.Query(`
		SELECT id, source_path, source_size,
		       COALESCE(output_size, 0), COALESCE(bytes_saved, 0), finished_at
		FROM jobs
		WHERE status IN ('done','staged') AND COALESCE(bytes_saved, 0) > 0
		ORDER BY bytes_saved DESC`)
	if err != nil {
		return nil, fmt.Errorf("list savings breakdown: %w", err)
	}
	defer rows.Close()

	var entries []*SavingsEntry
	for rows.Next() {
		e := &SavingsEntry{}
		if err := rows.Scan(&e.ID, &e.SourcePath, &e.SourceSize, &e.OutputSize, &e.BytesSaved, &e.FinishedAt); err != nil {
			return nil, fmt.Errorf("scan savings entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ConsecutiveFailCount returns the number of consecutive failures in the most
// recent jobs (across all files). Used for system-level auto-pause detection.
func (db *DB) ConsecutiveFailCount() (int, error) {
	// Walk back through recent finished jobs counting consecutive failures.
	rows, err := db.conn.Query(`
		SELECT status FROM jobs
		WHERE status IN ('failed','staged','done','excluded','error','restored')
		ORDER BY finished_at DESC
		LIMIT 20`)
	if err != nil {
		return 0, fmt.Errorf("consecutive fail count: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var s JobStatus
		if err := rows.Scan(&s); err != nil {
			break
		}
		if s == JobFailed || s == JobError {
			count++
		} else {
			break
		}
	}
	return count, rows.Err()
}
