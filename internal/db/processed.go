package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ProcessedFile represents a durably tracked file that has been through the pipeline.
type ProcessedFile struct {
	ID         int64
	SourcePath string
	Status     string // "done", "excluded", "skipped"
	Reason     sql.NullString
	SourceSize int64
	SourceMtime time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// UpsertProcessedFile inserts or updates a processed_files record.
func (db *DB) UpsertProcessedFile(sourcePath, status, reason string, sourceSize int64, sourceMtime time.Time) error {
	_, err := db.conn.Exec(`
		INSERT INTO processed_files (source_path, status, reason, source_size, source_mtime, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(source_path) DO UPDATE SET
			status = excluded.status,
			reason = excluded.reason,
			source_size = excluded.source_size,
			source_mtime = excluded.source_mtime,
			updated_at = CURRENT_TIMESTAMP`,
		sourcePath, status, nullString(reason), sourceSize, sourceMtime)
	if err != nil {
		return fmt.Errorf("upsert processed file: %w", err)
	}
	return nil
}

// GetProcessedFile returns the processed_files record for a path, or nil if none exists.
func (db *DB) GetProcessedFile(sourcePath string) (*ProcessedFile, error) {
	pf := &ProcessedFile{}
	err := db.conn.QueryRow(`
		SELECT id, source_path, status, reason, source_size, source_mtime, created_at, updated_at
		FROM processed_files WHERE source_path = ?`, sourcePath).
		Scan(&pf.ID, &pf.SourcePath, &pf.Status, &pf.Reason, &pf.SourceSize, &pf.SourceMtime, &pf.CreatedAt, &pf.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get processed file: %w", err)
	}
	return pf, nil
}

// DeleteProcessedFile removes the processed_files record for a path.
func (db *DB) DeleteProcessedFile(sourcePath string) error {
	_, err := db.conn.Exec(`DELETE FROM processed_files WHERE source_path = ?`, sourcePath)
	if err != nil {
		return fmt.Errorf("delete processed file: %w", err)
	}
	return nil
}

// IsFileProcessed checks whether a file has been processed and whether it has
// changed since it was recorded. Returns (processed, error). If the file's
// current size or mtime differ from the stored record, the record is deleted
// and false is returned (the file should be re-queued).
func (db *DB) IsFileProcessed(sourcePath string, currentSize int64, currentMtime time.Time) (bool, error) {
	pf, err := db.GetProcessedFile(sourcePath)
	if err != nil {
		return false, err
	}
	if pf == nil {
		return false, nil
	}

	// If the file has changed on disk, delete the record so it can be re-processed.
	if pf.SourceSize != currentSize || !pf.SourceMtime.Equal(currentMtime) {
		if err := db.DeleteProcessedFile(sourcePath); err != nil {
			return false, err
		}
		return false, nil
	}

	return true, nil
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
