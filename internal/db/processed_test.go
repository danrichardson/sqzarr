//go:build integration

package db_test

import (
	"testing"
	"time"

	"github.com/danrichardson/sqzarr/internal/testutil"
)

func TestUpsertAndGetProcessedFile(t *testing.T) {
	database := testutil.NewTestDB(t)

	mtime := time.Now().Truncate(time.Second)
	err := database.UpsertProcessedFile("/media/movie.mkv", "done", "", 1024*1024*1024, mtime)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	pf, err := database.GetProcessedFile("/media/movie.mkv")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if pf == nil {
		t.Fatal("expected record, got nil")
	}
	if pf.Status != "done" {
		t.Errorf("expected status 'done', got %q", pf.Status)
	}
	if pf.SourceSize != 1024*1024*1024 {
		t.Errorf("expected source_size 1073741824, got %d", pf.SourceSize)
	}
}

func TestUpsertUpdatesExisting(t *testing.T) {
	database := testutil.NewTestDB(t)

	mtime := time.Now().Truncate(time.Second)
	database.UpsertProcessedFile("/media/movie.mkv", "done", "", 1000, mtime)
	database.UpsertProcessedFile("/media/movie.mkv", "excluded", "uncompressible", 1000, mtime)

	pf, _ := database.GetProcessedFile("/media/movie.mkv")
	if pf.Status != "excluded" {
		t.Errorf("expected status 'excluded' after upsert, got %q", pf.Status)
	}
}

func TestDeleteProcessedFile(t *testing.T) {
	database := testutil.NewTestDB(t)

	mtime := time.Now().Truncate(time.Second)
	database.UpsertProcessedFile("/media/movie.mkv", "done", "", 1000, mtime)

	err := database.DeleteProcessedFile("/media/movie.mkv")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	pf, _ := database.GetProcessedFile("/media/movie.mkv")
	if pf != nil {
		t.Error("expected nil after delete")
	}
}

func TestIsFileProcessedMatchingFile(t *testing.T) {
	database := testutil.NewTestDB(t)

	mtime := time.Now().Truncate(time.Second)
	database.UpsertProcessedFile("/media/movie.mkv", "done", "", 1000, mtime)

	processed, err := database.IsFileProcessed("/media/movie.mkv", 1000, mtime)
	if err != nil {
		t.Fatalf("is file processed: %v", err)
	}
	if !processed {
		t.Error("expected true for matching size/mtime")
	}
}

func TestIsFileProcessedChangedFile(t *testing.T) {
	database := testutil.NewTestDB(t)

	mtime := time.Now().Truncate(time.Second)
	database.UpsertProcessedFile("/media/movie.mkv", "done", "", 1000, mtime)

	// Different size — should return false and delete the record.
	processed, err := database.IsFileProcessed("/media/movie.mkv", 2000, mtime)
	if err != nil {
		t.Fatalf("is file processed: %v", err)
	}
	if processed {
		t.Error("expected false for changed file size")
	}

	// Record should be gone.
	pf, _ := database.GetProcessedFile("/media/movie.mkv")
	if pf != nil {
		t.Error("expected record to be deleted after mismatch")
	}
}

func TestIsFileProcessedChangedMtime(t *testing.T) {
	database := testutil.NewTestDB(t)

	mtime := time.Now().Truncate(time.Second)
	database.UpsertProcessedFile("/media/movie.mkv", "done", "", 1000, mtime)

	newMtime := mtime.Add(1 * time.Hour)
	processed, err := database.IsFileProcessed("/media/movie.mkv", 1000, newMtime)
	if err != nil {
		t.Fatalf("is file processed: %v", err)
	}
	if processed {
		t.Error("expected false for changed mtime")
	}
}

func TestGetProcessedFileNotFound(t *testing.T) {
	database := testutil.NewTestDB(t)

	pf, err := database.GetProcessedFile("/nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pf != nil {
		t.Error("expected nil for nonexistent file")
	}
}
