package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Directory struct {
	ID                 int64
	Path               string
	Enabled            bool
	MinAgeDays         int
	MaxBitrate         int64
	MinSizeMB          int64
	BitrateSkipMargin  float64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (db *DB) ListDirectories() ([]*Directory, error) {
	rows, err := db.conn.Query(`
		SELECT id, path, enabled, min_age_days, max_bitrate, min_size_mb, bitrate_skip_margin, created_at, updated_at
		FROM directories ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list directories: %w", err)
	}
	defer rows.Close()

	var dirs []*Directory
	for rows.Next() {
		d := &Directory{}
		if err := rows.Scan(&d.ID, &d.Path, &d.Enabled, &d.MinAgeDays,
			&d.MaxBitrate, &d.MinSizeMB, &d.BitrateSkipMargin, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan directory: %w", err)
		}
		dirs = append(dirs, d)
	}
	return dirs, rows.Err()
}

func (db *DB) GetDirectory(id int64) (*Directory, error) {
	d := &Directory{}
	err := db.conn.QueryRow(`
		SELECT id, path, enabled, min_age_days, max_bitrate, min_size_mb, bitrate_skip_margin, created_at, updated_at
		FROM directories WHERE id = ?`, id).
		Scan(&d.ID, &d.Path, &d.Enabled, &d.MinAgeDays,
			&d.MaxBitrate, &d.MinSizeMB, &d.BitrateSkipMargin, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get directory: %w", err)
	}
	return d, nil
}

func (db *DB) InsertDirectory(d *Directory) (int64, error) {
	res, err := db.conn.Exec(`
		INSERT INTO directories (path, enabled, min_age_days, max_bitrate, min_size_mb, bitrate_skip_margin)
		VALUES (?, ?, ?, ?, ?, ?)`,
		d.Path, d.Enabled, d.MinAgeDays, d.MaxBitrate, d.MinSizeMB, d.BitrateSkipMargin)
	if err != nil {
		return 0, fmt.Errorf("insert directory: %w", err)
	}
	return res.LastInsertId()
}

func (db *DB) UpdateDirectory(d *Directory) error {
	_, err := db.conn.Exec(`
		UPDATE directories
		SET path=?, enabled=?, min_age_days=?, max_bitrate=?, min_size_mb=?, bitrate_skip_margin=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?`,
		d.Path, d.Enabled, d.MinAgeDays, d.MaxBitrate, d.MinSizeMB, d.BitrateSkipMargin, d.ID)
	if err != nil {
		return fmt.Errorf("update directory: %w", err)
	}
	return nil
}

func (db *DB) DeleteDirectory(id int64) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck
	// Null out FK references so job/scan history is preserved.
	if _, err := tx.Exec(`UPDATE jobs SET directory_id = NULL WHERE directory_id = ?`, id); err != nil {
		return fmt.Errorf("unlink jobs: %w", err)
	}
	if _, err := tx.Exec(`UPDATE scan_runs SET directory_id = NULL WHERE directory_id = ?`, id); err != nil {
		return fmt.Errorf("unlink scan_runs: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM directories WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete directory: %w", err)
	}
	return tx.Commit()
}
