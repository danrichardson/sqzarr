//go:build integration

package scanner_test

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/danrichardson/sqzarr/internal/db"
	"github.com/danrichardson/sqzarr/internal/scanner"
	"github.com/danrichardson/sqzarr/internal/testutil"
)

func testLog(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.Default()
}

func TestScanFindsH264File(t *testing.T) {
	database := testutil.NewTestDB(t)
	dir := t.TempDir()

	clipPath := testutil.MakeTestClip(t, dir, 30)
	past := time.Now().Add(-10 * 24 * time.Hour)
	os.Chtimes(clipPath, past, past)

	dirID, err := database.InsertDirectory(&db.Directory{
		Path:       dir,
		Enabled:    true,
		MinAgeDays: 7,
		MaxBitrate: 0, // disabled — test clip bitrate is not deterministic
		MinSizeMB:  0,
	})
	if err != nil {
		t.Fatal(err)
	}

	dbDir, _ := database.GetDirectory(dirID)
	s := scanner.New(database, ".processed", testLog(t))
	result, err := s.ScanDirectory(context.Background(), dbDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.FilesQueued != 1 {
		t.Errorf("expected 1 queued, got %d", result.FilesQueued)
	}

	jobs, err := database.ListJobs(db.JobPending, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 pending job, got %d", len(jobs))
	}
	if jobs[0].SourcePath != clipPath {
		t.Errorf("expected source path %s, got %s", clipPath, jobs[0].SourcePath)
	}
}

// TestScannerEnqueuesOversizedHEVC verifies that an HEVC file above the bitrate
// threshold IS enqueued — codec alone must not cause a skip.
func TestScannerEnqueuesOversizedHEVC(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH")
	}

	database := testutil.NewTestDB(t)
	dir := t.TempDir()

	// Encode a short HEVC clip at a relatively high bitrate.
	clipPath := filepath.Join(dir, "hevc_big.mkv")
	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi",
		"-i", "testsrc=duration=10:size=1920x1080:rate=25",
		"-c:v", "libx265", "-b:v", "8000k",
		clipPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot create HEVC test clip: %v\n%s", err, out)
	}

	past := time.Now().Add(-10 * 24 * time.Hour)
	os.Chtimes(clipPath, past, past)

	dirID, _ := database.InsertDirectory(&db.Directory{
		Path:       dir,
		Enabled:    true,
		MinAgeDays: 7,
		MaxBitrate: 2_222_000, // 1 GB/hr — clip is well above this
	})
	dbDir, _ := database.GetDirectory(dirID)

	s := scanner.New(database, ".processed", testLog(t))
	result, _ := s.ScanDirectory(context.Background(), dbDir)

	if result.FilesQueued != 1 {
		t.Errorf("expected oversized HEVC to be queued, got FilesQueued=%d", result.FilesQueued)
	}
}

// TestScannerSkipsUndersizedHEVC verifies that an HEVC file below the bitrate
// threshold is NOT enqueued (bitrate gate works regardless of codec).
func TestScannerSkipsUndersizedHEVC(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH")
	}

	database := testutil.NewTestDB(t)
	dir := t.TempDir()

	clipPath := filepath.Join(dir, "hevc_small.mkv")
	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi",
		"-i", "testsrc=duration=10:size=640x480:rate=25",
		"-c:v", "libx265", "-crf", "40",
		clipPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot create HEVC test clip: %v\n%s", err, out)
	}

	past := time.Now().Add(-10 * 24 * time.Hour)
	os.Chtimes(clipPath, past, past)

	dirID, _ := database.InsertDirectory(&db.Directory{
		Path:       dir,
		Enabled:    true,
		MinAgeDays: 7,
		MaxBitrate: 2_222_000,
	})
	dbDir, _ := database.GetDirectory(dirID)

	s := scanner.New(database, ".processed", testLog(t))
	result, _ := s.ScanDirectory(context.Background(), dbDir)

	if result.FilesQueued != 0 {
		t.Errorf("expected low-bitrate HEVC to be skipped, got FilesQueued=%d", result.FilesQueued)
	}
}

// TestScannerEnqueuesOversizedAV1 verifies that AV1 files are subject to the
// same bitrate gate as any other codec.
func TestScannerEnqueuesOversizedAV1(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH")
	}

	database := testutil.NewTestDB(t)
	dir := t.TempDir()

	clipPath := filepath.Join(dir, "av1_big.mkv")
	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi",
		"-i", "testsrc=duration=10:size=1920x1080:rate=25",
		"-c:v", "libaom-av1", "-b:v", "5000k", "-cpu-used", "8",
		clipPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot create AV1 test clip (libaom-av1 may not be available): %v\n%s", err, out)
	}

	past := time.Now().Add(-10 * 24 * time.Hour)
	os.Chtimes(clipPath, past, past)

	dirID, _ := database.InsertDirectory(&db.Directory{
		Path:       dir,
		Enabled:    true,
		MinAgeDays: 7,
		MaxBitrate: 2_222_000,
	})
	dbDir, _ := database.GetDirectory(dirID)

	s := scanner.New(database, ".processed", testLog(t))
	result, _ := s.ScanDirectory(context.Background(), dbDir)

	if result.FilesQueued != 1 {
		t.Errorf("expected oversized AV1 to be queued, got FilesQueued=%d", result.FilesQueued)
	}
}

func TestScanSkipsNewFile(t *testing.T) {
	database := testutil.NewTestDB(t)
	dir := t.TempDir()

	testutil.MakeTestClip(t, dir, 10)
	// mtime is now — must fail the min_age_days=7 check.

	dirID, _ := database.InsertDirectory(&db.Directory{
		Path:       dir,
		Enabled:    true,
		MinAgeDays: 7,
		MaxBitrate: 0, // disabled — we're testing age, not bitrate
	})
	dbDir, _ := database.GetDirectory(dirID)

	s := scanner.New(database, ".processed", testLog(t))
	result, _ := s.ScanDirectory(context.Background(), dbDir)

	if result.FilesQueued != 0 {
		t.Errorf("expected 0 queued for new file, got %d", result.FilesQueued)
	}
}

func TestScanDeduplicates(t *testing.T) {
	database := testutil.NewTestDB(t)
	dir := t.TempDir()

	clipPath := testutil.MakeTestClip(t, dir, 30)
	past := time.Now().Add(-10 * 24 * time.Hour)
	os.Chtimes(clipPath, past, past)

	dirID, _ := database.InsertDirectory(&db.Directory{
		Path:       dir,
		Enabled:    true,
		MinAgeDays: 7,
		MaxBitrate: 0, // disabled
	})
	dbDir, _ := database.GetDirectory(dirID)
	s := scanner.New(database, ".processed", testLog(t))

	s.ScanDirectory(context.Background(), dbDir)
	result, _ := s.ScanDirectory(context.Background(), dbDir)

	if result.FilesQueued != 0 {
		t.Errorf("expected 0 queued on second scan, got %d", result.FilesQueued)
	}
}

// TestClearHistoryDoesNotRequeueExcluded verifies that excluded files with a
// processed_files record are NOT re-queued after ClearHistory deletes the
// job rows.
func TestClearHistoryDoesNotRequeueExcluded(t *testing.T) {
	database := testutil.NewTestDB(t)
	dir := t.TempDir()

	clipPath := testutil.MakeTestClip(t, dir, 30)
	past := time.Now().Add(-10 * 24 * time.Hour)
	os.Chtimes(clipPath, past, past)

	dirID, _ := database.InsertDirectory(&db.Directory{
		Path:       dir,
		Enabled:    true,
		MinAgeDays: 7,
		MaxBitrate: 0,
	})
	dbDir, _ := database.GetDirectory(dirID)
	s := scanner.New(database, ".processed", testLog(t))

	// First scan queues the file.
	s.ScanDirectory(context.Background(), dbDir)

	// Simulate the worker excluding the file: mark the job as excluded and
	// upsert a durable processed_files record.
	jobs, _ := database.ListJobs(db.JobPending, 10, 0)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 pending job, got %d", len(jobs))
	}
	database.ExcludeJob(jobs[0].ID, "uncompressible")

	fi, _ := os.Stat(clipPath)
	database.UpsertProcessedFile(clipPath, "excluded", "uncompressible", fi.Size(), fi.ModTime())

	// Clear history — this deletes the excluded job row from the jobs table.
	n, err := database.ClearHistory()
	if err != nil {
		t.Fatalf("clear history: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 deleted, got %d", n)
	}

	// Scan again — the file should NOT be re-queued because the
	// processed_files record still exists.
	result, err := s.ScanDirectory(context.Background(), dbDir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if result.FilesQueued != 0 {
		t.Errorf("expected 0 queued after clear+rescan, got %d (bug: excluded file re-queued)", result.FilesQueued)
	}
}
