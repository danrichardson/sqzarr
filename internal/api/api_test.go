package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danrichardson/sqzarr/internal/api"
	"github.com/danrichardson/sqzarr/internal/config"
	"github.com/danrichardson/sqzarr/internal/db"
	"github.com/danrichardson/sqzarr/internal/queue"
	"github.com/danrichardson/sqzarr/internal/scanner"
	"github.com/danrichardson/sqzarr/internal/transcoder"
	"github.com/danrichardson/sqzarr/internal/testutil"
)

func newTestServer(t *testing.T) (*api.Server, *db.DB) {
	t.Helper()
	database := testutil.NewTestDB(t)
	cfg := config.Defaults()
	cfg.Server.DataDir = t.TempDir()
	cfg.Auth.JWTSecret = "test-secret"

	enc := &transcoder.Encoder{
		Type:        transcoder.EncoderSoftware,
		DisplayName: "Software (test)",
		BuildArgs:   func(in, out string) []string { return nil },
	}
	log := slog.Default()
	worker := queue.New(database, cfg, transcoder.New(enc, "", log), nil, log)
	scan := scanner.New(database, log)
	return api.New(cfg, database, worker, scan, enc, log), database
}

func TestGetStatus(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/status", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if _, ok := result["version"]; !ok {
		t.Error("expected version field in status response")
	}
}

func TestJobsEndpoint(t *testing.T) {
	srv, database := newTestServer(t)

	dir := t.TempDir()
	database.InsertDirectory(&db.Directory{
		Path:       dir,
		Enabled:    true,
		MinAgeDays: 7,
		MaxBitrate: 4_000_000,
	})

	body, _ := json.Marshal(map[string]string{"path": dir + "/test.mkv"})
	req := httptest.NewRequest("POST", "/api/v1/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	req2 := httptest.NewRequest("GET", "/api/v1/jobs", nil)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("expected 200 from GET /jobs, got %d", w2.Code)
	}
}

func TestCancelJob(t *testing.T) {
	srv, database := newTestServer(t)

	jobID, _ := database.InsertJob(&db.Job{
		SourcePath:     "/media/Videos/test.mkv",
		SourceSize:     100,
		SourceCodec:    "h264",
		SourceDuration: 30,
		SourceBitrate:  8_000_000,
		Status:         db.JobPending,
	})

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/jobs/%d", jobID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	job, _ := database.GetJob(jobID)
	if job.Status != db.JobCancelled {
		t.Errorf("expected job status cancelled, got %s", job.Status)
	}
}

func TestPathTraversalBlocked(t *testing.T) {
	srv, _ := newTestServer(t)

	body, _ := json.Marshal(map[string]string{"path": "../etc/passwd"})
	req := httptest.NewRequest("POST", "/api/v1/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for path traversal, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthRequired(t *testing.T) {
	database := testutil.NewTestDB(t)
	cfg := config.Defaults()
	cfg.Server.DataDir = t.TempDir()
	cfg.Auth.PasswordHash = "$2a$10$aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	cfg.Auth.JWTSecret = "test-secret"

	log := slog.Default()
	enc := &transcoder.Encoder{Type: transcoder.EncoderSoftware, DisplayName: "test",
		BuildArgs: func(in, out string) []string { return nil }}
	worker := queue.New(database, cfg, transcoder.New(enc, "", log), nil, log)
	scan := scanner.New(database, log)
	srv := api.New(cfg, database, worker, scan, enc, log)

	req := httptest.NewRequest("GET", "/api/v1/status", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", w.Code)
	}
}

func TestAuthLogin(t *testing.T) {
	database := testutil.NewTestDB(t)
	cfg := config.Defaults()
	cfg.Server.DataDir = t.TempDir()
	// bcrypt hash of "testpassword"
	cfg.Auth.PasswordHash = "$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi"
	cfg.Auth.JWTSecret = "test-secret"

	log := slog.Default()
	enc := &transcoder.Encoder{Type: transcoder.EncoderSoftware, DisplayName: "test",
		BuildArgs: func(in, out string) []string { return nil }}
	worker := queue.New(database, cfg, transcoder.New(enc, "", log), nil, log)
	scan := scanner.New(database, log)
	srv := api.New(cfg, database, worker, scan, enc, log)

	// Wrong password — should get 401.
	body := strings.NewReader(`{"password":"wrongpassword"}`)
	req := httptest.NewRequest("POST", "/api/v1/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("wrong password should return 401, got %d: %s", w.Code, w.Body)
	}
}
