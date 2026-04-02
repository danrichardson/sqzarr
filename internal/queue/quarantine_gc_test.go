//go:build integration

package queue_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danrichardson/sqzarr/internal/db"
	"github.com/danrichardson/sqzarr/internal/queue"
	"github.com/danrichardson/sqzarr/internal/testutil"
)

func TestQuarantineGCSweep(t *testing.T) {
	database := testutil.NewTestDB(t)
	dir := t.TempDir()

	// Create a fake quarantine file.
	quarPath := filepath.Join(dir, "media", "test.mkv")
	if err := os.MkdirAll(filepath.Dir(quarPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(quarPath, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Insert a job so we can create a quarantine record.
	jobID, err := database.InsertJob(&db.Job{
		SourcePath:     "/media/Videos/test.mkv",
		SourceSize:     1000,
		SourceCodec:    "h264",
		SourceDuration: 30,
		SourceBitrate:  8_000_000,
		Status:         db.JobDone,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Insert quarantine record already expired.
	_, err = database.InsertQuarantine(&db.QuarantineRecord{
		JobID:          jobID,
		OriginalPath:   "/media/Videos/test.mkv",
		QuarantinePath: quarPath,
		ExpiresAt:      time.Now().Add(-1 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify file exists before GC.
	if _, err := os.Stat(quarPath); os.IsNotExist(err) {
		t.Fatal("quarantine file should exist before GC")
	}

	gc := queue.NewQuarantineGC(database, testLog(t))
	gc.Sweep()

	// File should be deleted.
	if _, err := os.Stat(quarPath); !os.IsNotExist(err) {
		t.Error("quarantine file should be deleted after GC sweep")
	}

	// Record should be marked deleted.
	records, err := database.ExpiredQuarantines()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 expired records after GC, got %d", len(records))
	}
}
