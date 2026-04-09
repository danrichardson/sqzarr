//go:build integration

package db_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/danrichardson/sqzarr/internal/db"
	"github.com/danrichardson/sqzarr/internal/testutil"
)

func TestClearHistoryPreservesProcessedFiles(t *testing.T) {
	database := testutil.NewTestDB(t)

	mtime := time.Now().Truncate(time.Second)

	// Insert a job that reaches "staged" status.
	jobID, err := database.InsertJob(&db.Job{
		SourcePath:    "/media/movie.mkv",
		SourceSize:    2 * 1024 * 1024 * 1024,
		SourceCodec:   "hevc",
		SourceDuration: 7200.0,
		SourceBitrate: 5000000,
		Status:        db.JobPending,
	})
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}

	if err := database.StageJob(jobID, "/media/movie.staged.mkv", 1*1024*1024*1024, "hevc_nvenc", 1*1024*1024*1024); err != nil {
		t.Fatalf("stage job: %v", err)
	}

	// Record in processed_files (as the worker would).
	if err := database.UpsertProcessedFile("/media/movie.mkv", "done", "", 2*1024*1024*1024, mtime); err != nil {
		t.Fatalf("upsert processed: %v", err)
	}

	// Also insert an excluded job with processed_files record.
	exclID, err := database.InsertJob(&db.Job{
		SourcePath:    "/media/incompressible.mkv",
		SourceSize:    1 * 1024 * 1024 * 1024,
		SourceCodec:   "hevc",
		SourceDuration: 3600.0,
		SourceBitrate: 2000000,
		Status:        db.JobPending,
	})
	if err != nil {
		t.Fatalf("insert excluded job: %v", err)
	}
	if err := database.ExcludeJob(exclID, "uncompressible"); err != nil {
		t.Fatalf("exclude job: %v", err)
	}
	if err := database.UpsertProcessedFile("/media/incompressible.mkv", "excluded", "uncompressible", 1*1024*1024*1024, mtime); err != nil {
		t.Fatalf("upsert processed excluded: %v", err)
	}

	// Clear history — this deletes jobs rows.
	n, err := database.ClearHistory()
	if err != nil {
		t.Fatalf("clear history: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 cleared, got %d", n)
	}

	// Verify job rows are gone.
	status, err := database.SourcePathStatus("/media/movie.mkv")
	if err != nil {
		t.Fatalf("source path status: %v", err)
	}
	if status != "" {
		t.Errorf("expected empty status after clear, got %q", status)
	}

	// Verify processed_files records survive.
	processed, err := database.IsFileProcessed("/media/movie.mkv", 2*1024*1024*1024, mtime)
	if err != nil {
		t.Fatalf("is file processed (staged): %v", err)
	}
	if !processed {
		t.Error("expected staged file to still be marked as processed after clear")
	}

	processed, err = database.IsFileProcessed("/media/incompressible.mkv", 1*1024*1024*1024, mtime)
	if err != nil {
		t.Fatalf("is file processed (excluded): %v", err)
	}
	if !processed {
		t.Error("expected excluded file to still be marked as processed after clear")
	}
}

func TestClearHistoryClearsDoneAndStaged(t *testing.T) {
	database := testutil.NewTestDB(t)

	// Insert jobs in various statuses.
	statuses := map[string]db.JobStatus{
		"done":      db.JobDone,
		"staged":    db.JobStaged,
		"failed":    db.JobFailed,
		"cancelled": db.JobCancelled,
		"excluded":  db.JobExcluded,
		"pending":   db.JobPending,
		"running":   db.JobRunning,
	}

	for name, status := range statuses {
		_, err := database.InsertJob(&db.Job{
			SourcePath:    "/media/" + name + ".mkv",
			SourceSize:    1000,
			SourceCodec:   "h264",
			SourceDuration: 100.0,
			SourceBitrate: 5000000,
			Status:        status,
		})
		if err != nil {
			t.Fatalf("insert %s job: %v", name, err)
		}
	}

	n, err := database.ClearHistory()
	if err != nil {
		t.Fatalf("clear history: %v", err)
	}
	// done, staged, failed, cancelled, excluded = 5 cleared
	// pending, running = 2 kept
	if n != 5 {
		t.Errorf("expected 5 cleared, got %d", n)
	}

	// Verify pending and running still exist.
	remaining, err := database.ListJobs("", 100, 0)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(remaining) != 2 {
		t.Errorf("expected 2 remaining jobs, got %d", len(remaining))
	}
	for _, j := range remaining {
		if j.Status != db.JobPending && j.Status != db.JobRunning {
			t.Errorf("unexpected remaining job status: %s", j.Status)
		}
	}
}

func TestClearHistoryDeletesOriginalsForClearedJobs(t *testing.T) {
	database := testutil.NewTestDB(t)

	// Insert a staged job.
	jobID, err := database.InsertJob(&db.Job{
		SourcePath:    "/media/movie.mkv",
		SourceSize:    1000,
		SourceCodec:   "h264",
		SourceDuration: 100.0,
		SourceBitrate: 5000000,
		Status:        db.JobPending,
	})
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if err := database.StageJob(jobID, "/media/movie.out.mkv", 500, "hevc_nvenc", 500); err != nil {
		t.Fatalf("stage job: %v", err)
	}

	// Insert an originals record for this job.
	_, err = database.Conn().Exec(`
		INSERT INTO originals (job_id, original_path, held_path, output_path, original_size, output_size, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		jobID, "/media/movie.mkv", "/media/.processed/movie.mkv", "/media/movie.out.mkv", 1000, 500,
		time.Now().Add(7*24*time.Hour))
	if err != nil {
		t.Fatalf("insert original: %v", err)
	}

	// Clear history.
	n, err := database.ClearHistory()
	if err != nil {
		t.Fatalf("clear history: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 cleared, got %d", n)
	}

	// Verify originals record is also gone.
	var count int
	err = database.Conn().QueryRow(`SELECT COUNT(*) FROM originals WHERE job_id = ?`, jobID).Scan(&count)
	if err != nil {
		t.Fatalf("count originals: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 originals after clear, got %d", count)
	}
}

// Ensure restored status is also cleared.
func TestClearHistoryIncludesRestored(t *testing.T) {
	database := testutil.NewTestDB(t)

	id, err := database.InsertJob(&db.Job{
		SourcePath:    "/media/restored.mkv",
		SourceSize:    1000,
		SourceCodec:   "h264",
		SourceDuration: 100.0,
		SourceBitrate: 5000000,
		Status:        db.JobPending,
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := database.UpdateJobStatus(id, db.JobRestored, ""); err != nil {
		t.Fatalf("update status: %v", err)
	}

	n, err := database.ClearHistory()
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 cleared, got %d", n)
	}
}

// Suppress unused import warning — sql is used for NullInt64 in InsertJob.
var _ = sql.NullInt64{}
