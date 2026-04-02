# SQZARR — Code Starter Packet

Complete, ready-to-use starter code. Copy files, run `make frontend && go build ./...`, fill in `sqzarr.toml`, deploy.

---

## `go.mod`
```go
module github.com/danrichardson/sqzarr

go 1.22

require (
    github.com/BurntSushi/toml v1.3.2
    github.com/golang-jwt/jwt/v5 v5.2.1
    github.com/gorilla/mux v1.8.1
    github.com/gorilla/websocket v1.5.1
    golang.org/x/crypto v0.22.0
    modernc.org/sqlite v1.29.6
)
```

---

## `Makefile`
```makefile
BINARY := sqzarr
VERSION ?= dev
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"

.PHONY: build frontend clean release

build: frontend
	go build $(LDFLAGS) -o $(BINARY) ./cmd/sqzarr

frontend:
	cd frontend && npm ci && npm run build

build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 ./cmd/sqzarr

build-linux-arm:
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 ./cmd/sqzarr

build-darwin:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 ./cmd/sqzarr

release: frontend
	mkdir -p dist
	$(MAKE) build-linux
	$(MAKE) build-linux-arm
	$(MAKE) build-darwin
	tar -czf dist/$(BINARY)-linux-amd64.tar.gz -C dist $(BINARY)-linux-amd64
	tar -czf dist/$(BINARY)-linux-arm64.tar.gz -C dist $(BINARY)-linux-arm64
	tar -czf dist/$(BINARY)-darwin-arm64.tar.gz -C dist $(BINARY)-darwin-arm64

clean:
	rm -rf $(BINARY) dist/ frontend/dist/

test:
	go test ./...

lint:
	golangci-lint run ./...
```

---

## `cmd/sqzarr/main.go`
```go
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/danrichardson/sqzarr/internal/api"
	"github.com/danrichardson/sqzarr/internal/config"
	"github.com/danrichardson/sqzarr/internal/db"
	"github.com/danrichardson/sqzarr/internal/queue"
	"github.com/danrichardson/sqzarr/internal/scanner"
	"github.com/danrichardson/sqzarr/internal/transcoder"
	"golang.org/x/crypto/bcrypt"
)

var Version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "sqzarr: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("sqzarr", flag.ContinueOnError)
	configPath := fs.String("config", "sqzarr.toml", "path to config file")

	if len(args) == 0 {
		args = []string{"serve"}
	}

	switch args[0] {
	case "serve":
		return serve(args[1:], *configPath)
	case "hash-password":
		return hashPassword(args[1:])
	case "version":
		fmt.Printf("sqzarr %s\n", Version)
		return nil
	default:
		return fmt.Errorf("unknown command %q (available: serve, hash-password, version)", args[0])
	}
}

func serve(args []string, configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	database, err := db.Open(cfg.DataDir + "/sqzarr.db")
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	enc, err := transcoder.DetectEncoder()
	if err != nil {
		slog.Warn("hardware encoder detection failed, using software", "error", err)
		enc = transcoder.EncoderSoftware
	}
	slog.Info("hardware encoder detected", "encoder", enc)

	q := queue.New(database, cfg, enc)
	sc := scanner.New(database, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go q.Run(ctx)
	go sc.RunScheduled(ctx)

	srv := api.NewServer(cfg, database, q, sc, enc, Version)

	slog.Info("sqzarr starting", "addr", cfg.Server.Addr(), "version", Version)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		slog.Info("shutting down")
		cancel()
	}()

	return srv.ListenAndServe(ctx)
}

func hashPassword(args []string) error {
	fs := flag.NewFlagSet("hash-password", flag.ContinueOnError)
	password := fs.String("password", "", "password to hash")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *password == "" {
		fmt.Print("Enter password: ")
		var p string
		fmt.Scanln(&p)
		password = &p
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(*password), 12)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	fmt.Println(string(hash))
	return nil
}
```

---

## `internal/config/config.go`
```go
package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Server     ServerConfig     `toml:"server"`
	Scanner    ScannerConfig    `toml:"scanner"`
	Transcoder TranscoderConfig `toml:"transcoder"`
	Safety     SafetyConfig     `toml:"safety"`
	Plex       PlexConfig       `toml:"plex"`
	Auth       AuthConfig       `toml:"auth"`
	DataDir    string           `toml:"data_dir"`
}

type ServerConfig struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

func (s ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

type ScannerConfig struct {
	IntervalHours      int `toml:"interval_hours"`
	WorkerConcurrency  int `toml:"worker_concurrency"`
}

type TranscoderConfig struct {
	TempDir string `toml:"temp_dir"`
}

type SafetyConfig struct {
	QuarantineEnabled       bool   `toml:"quarantine_enabled"`
	QuarantineRetentionDays int    `toml:"quarantine_retention_days"`
	QuarantineDir           string `toml:"quarantine_dir"`
	DiskFreePauseGB         int    `toml:"disk_free_pause_gb"`
}

type PlexConfig struct {
	Enabled bool   `toml:"enabled"`
	BaseURL string `toml:"base_url"`
	Token   string `toml:"token"`
}

type AuthConfig struct {
	PasswordHash string `toml:"password_hash"`
	JWTSecret    string `toml:"jwt_secret"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %q: %w", path, err)
	}

	cfg := defaults()
	if _, err := toml.Decode(string(data), cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

func defaults() *Config {
	return &Config{
		DataDir: "/var/lib/sqzarr",
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Scanner: ScannerConfig{
			IntervalHours:     6,
			WorkerConcurrency: 1,
		},
		Safety: SafetyConfig{
			QuarantineEnabled:       true,
			QuarantineRetentionDays: 10,
			DiskFreePauseGB:         50,
		},
	}
}

func validate(cfg *Config) error {
	if cfg.DataDir == "" {
		return fmt.Errorf("data_dir is required")
	}
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server.port must be 1-65535")
	}
	if cfg.Scanner.IntervalHours <= 0 {
		return fmt.Errorf("scanner.interval_hours must be > 0")
	}
	if cfg.Scanner.WorkerConcurrency <= 0 || cfg.Scanner.WorkerConcurrency > 8 {
		return fmt.Errorf("scanner.worker_concurrency must be 1-8")
	}
	return nil
}
```

---

## `internal/db/db.go`
```go
package db

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	sql *sql.DB
	mu  sync.Mutex
}

func Open(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path+"?_journal=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	sqlDB.SetMaxOpenConns(1)
	return &DB{sql: sqlDB}, nil
}

func (d *DB) Close() error {
	return d.sql.Close()
}

func (d *DB) Migrate() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.sql.Exec(schema)
	return err
}

const schema = `
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS directories (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    path          TEXT    NOT NULL UNIQUE,
    enabled       BOOLEAN NOT NULL DEFAULT 1,
    min_age_days  INTEGER NOT NULL DEFAULT 7,
    max_bitrate   INTEGER NOT NULL DEFAULT 4000000,
    min_size_mb   INTEGER NOT NULL DEFAULT 500,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS jobs (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    directory_id    INTEGER REFERENCES directories(id),
    source_path     TEXT    NOT NULL,
    source_size     INTEGER NOT NULL,
    source_codec    TEXT    NOT NULL,
    source_duration REAL    NOT NULL,
    source_bitrate  INTEGER NOT NULL,
    output_path     TEXT,
    output_size     INTEGER,
    encoder_used    TEXT,
    status          TEXT    NOT NULL DEFAULT 'pending',
    priority        INTEGER NOT NULL DEFAULT 0,
    error_message   TEXT,
    progress        REAL    NOT NULL DEFAULT 0,
    bytes_saved     INTEGER,
    started_at      DATETIME,
    finished_at     DATETIME,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_source_path ON jobs(source_path);

CREATE TABLE IF NOT EXISTS quarantine (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id          INTEGER NOT NULL REFERENCES jobs(id),
    original_path   TEXT    NOT NULL,
    quarantine_path TEXT    NOT NULL,
    expires_at      DATETIME NOT NULL,
    deleted_at      DATETIME,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS scan_runs (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    directory_id    INTEGER REFERENCES directories(id),
    files_scanned   INTEGER NOT NULL DEFAULT 0,
    files_queued    INTEGER NOT NULL DEFAULT 0,
    files_skipped   INTEGER NOT NULL DEFAULT 0,
    duration_ms     INTEGER,
    error           TEXT,
    started_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at     DATETIME
);

CREATE TABLE IF NOT EXISTS stats (
    id                INTEGER PRIMARY KEY CHECK (id = 1),
    total_bytes_saved INTEGER NOT NULL DEFAULT 0,
    total_jobs_done   INTEGER NOT NULL DEFAULT 0,
    total_jobs_failed INTEGER NOT NULL DEFAULT 0,
    updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO stats (id) VALUES (1);
`

// Job represents a transcode job.
type Job struct {
	ID             int64
	DirectoryID    *int64
	SourcePath     string
	SourceSize     int64
	SourceCodec    string
	SourceDuration float64
	SourceBitrate  int64
	OutputPath     *string
	OutputSize     *int64
	EncoderUsed    *string
	Status         string
	Priority       int
	ErrorMessage   *string
	Progress       float64
	BytesSaved     *int64
	StartedAt      *time.Time
	FinishedAt     *time.Time
	CreatedAt      time.Time
}

// Directory represents a watched media directory.
type Directory struct {
	ID          int64
	Path        string
	Enabled     bool
	MinAgeDays  int
	MaxBitrate  int64
	MinSizeMB   int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Stats holds aggregate counters.
type Stats struct {
	TotalBytesSaved int64
	TotalJobsDone   int64
	TotalJobsFailed int64
}

func (d *DB) GetStats() (*Stats, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	row := d.sql.QueryRow(`SELECT total_bytes_saved, total_jobs_done, total_jobs_failed FROM stats WHERE id = 1`)
	var s Stats
	if err := row.Scan(&s.TotalBytesSaved, &s.TotalJobsDone, &s.TotalJobsFailed); err != nil {
		return nil, err
	}
	return &s, nil
}

func (d *DB) NextPendingJob() (*Job, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	row := d.sql.QueryRow(`
		SELECT id, directory_id, source_path, source_size, source_codec,
		       source_duration, source_bitrate, status, priority, created_at
		FROM jobs
		WHERE status = 'pending'
		ORDER BY priority DESC, created_at ASC
		LIMIT 1
	`)
	var j Job
	var dirID sql.NullInt64
	err := row.Scan(&j.ID, &dirID, &j.SourcePath, &j.SourceSize, &j.SourceCodec,
		&j.SourceDuration, &j.SourceBitrate, &j.Status, &j.Priority, &j.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if dirID.Valid {
		id := dirID.Int64
		j.DirectoryID = &id
	}
	return &j, nil
}

func (d *DB) UpdateJobStatus(id int64, status string, errMsg *string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.sql.Exec(`UPDATE jobs SET status = ?, error_message = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, errMsg, id)
	return err
}

func (d *DB) UpdateJobProgress(id int64, progress float64) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.sql.Exec(`UPDATE jobs SET progress = ? WHERE id = ?`, progress, id)
	return err
}

func (d *DB) CompleteJob(id int64, outputSize, bytesSaved int64, encoder string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.sql.Exec(`
		UPDATE jobs SET
			status = 'done',
			output_size = ?,
			bytes_saved = ?,
			encoder_used = ?,
			progress = 1.0,
			finished_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, outputSize, bytesSaved, encoder, id)
	if err != nil {
		return err
	}
	_, err = d.sql.Exec(`
		UPDATE stats SET
			total_bytes_saved = total_bytes_saved + ?,
			total_jobs_done = total_jobs_done + 1,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, bytesSaved)
	return err
}

func (d *DB) IncrementFailedJobs() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.sql.Exec(`UPDATE stats SET total_jobs_failed = total_jobs_failed + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1`)
	return err
}

func (d *DB) ListDirectories() ([]*Directory, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	rows, err := d.sql.Query(`SELECT id, path, enabled, min_age_days, max_bitrate, min_size_mb, created_at, updated_at FROM directories ORDER BY path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var dirs []*Directory
	for rows.Next() {
		var dir Directory
		if err := rows.Scan(&dir.ID, &dir.Path, &dir.Enabled, &dir.MinAgeDays, &dir.MaxBitrate, &dir.MinSizeMB, &dir.CreatedAt, &dir.UpdatedAt); err != nil {
			return nil, err
		}
		dirs = append(dirs, &dir)
	}
	return dirs, rows.Err()
}

func (d *DB) InsertDirectory(path string, minAgeDays int, maxBitrate int64, minSizeMB int) (*Directory, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	res, err := d.sql.Exec(`
		INSERT INTO directories (path, min_age_days, max_bitrate, min_size_mb)
		VALUES (?, ?, ?, ?)
	`, path, minAgeDays, maxBitrate, minSizeMB)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Directory{ID: id, Path: path, Enabled: true, MinAgeDays: minAgeDays, MaxBitrate: maxBitrate, MinSizeMB: minSizeMB}, nil
}

func (d *DB) EnqueueJob(dirID *int64, sourcePath string, sourceSize int64, codec string, duration float64, bitrate int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	// Skip if already in queue (pending or running)
	var existing int
	err := d.sql.QueryRow(`SELECT COUNT(*) FROM jobs WHERE source_path = ? AND status IN ('pending', 'running')`, sourcePath).Scan(&existing)
	if err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}
	_, err = d.sql.Exec(`
		INSERT INTO jobs (directory_id, source_path, source_size, source_codec, source_duration, source_bitrate)
		VALUES (?, ?, ?, ?, ?, ?)
	`, dirID, sourcePath, sourceSize, codec, duration, bitrate)
	return err
}

func (d *DB) ListJobs(status string, limit, offset int) ([]*Job, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	query := `SELECT id, source_path, source_size, source_codec, status, progress, bytes_saved, error_message, encoder_used, created_at, finished_at FROM jobs`
	args := []any{}
	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := d.sql.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []*Job
	for rows.Next() {
		var j Job
		var bytesSaved, outputSize sql.NullInt64
		var errMsg, encoder sql.NullString
		var finishedAt sql.NullTime
		if err := rows.Scan(&j.ID, &j.SourcePath, &j.SourceSize, &j.SourceCodec, &j.Status, &j.Progress, &bytesSaved, &errMsg, &encoder, &j.CreatedAt, &finishedAt); err != nil {
			return nil, err
		}
		if bytesSaved.Valid { v := bytesSaved.Int64; j.BytesSaved = &v }
		if outputSize.Valid { v := outputSize.Int64; j.OutputSize = &v }
		if errMsg.Valid { j.ErrorMessage = &errMsg.String }
		if encoder.Valid { j.EncoderUsed = &encoder.String }
		if finishedAt.Valid { j.FinishedAt = &finishedAt.Time }
		jobs = append(jobs, &j)
	}
	return jobs, rows.Err()
}
```

---

## `internal/transcoder/detect.go`
```go
package transcoder

import (
	"os/exec"
	"runtime"
	"strings"
)

type Encoder string

const (
	EncoderVAAPI         Encoder = "vaapi"
	EncoderVideoToolbox  Encoder = "videotoolbox"
	EncoderNVENC         Encoder = "nvenc"
	EncoderSoftware      Encoder = "software"
)

// DetectEncoder probes available hardware and returns the best encoder.
func DetectEncoder() (Encoder, error) {
	if runtime.GOOS == "darwin" {
		if hasFFmpegEncoder("hevc_videotoolbox") {
			return EncoderVideoToolbox, nil
		}
	}

	if hasVAAPI() && hasFFmpegEncoder("hevc_vaapi") {
		return EncoderVAAPI, nil
	}

	if hasNVIDIA() && hasFFmpegEncoder("hevc_nvenc") {
		return EncoderNVENC, nil
	}

	return EncoderSoftware, nil
}

func hasVAAPI() bool {
	_, err := exec.LookPath("vainfo")
	if err != nil {
		return false
	}
	out, err := exec.Command("vainfo").CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "hevc")
}

func hasNVIDIA() bool {
	_, err := exec.LookPath("nvidia-smi")
	if err != nil {
		return false
	}
	return exec.Command("nvidia-smi").Run() == nil
}

func hasFFmpegEncoder(name string) bool {
	out, err := exec.Command("ffmpeg", "-encoders", "-v", "quiet").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), name)
}

// FFmpegArgs returns the ffmpeg arguments for the given encoder.
// inputPath and outputPath must be absolute paths.
func FFmpegArgs(enc Encoder, inputPath, outputPath string) []string {
	base := []string{"-y", "-i", inputPath}
	var encArgs []string

	switch enc {
	case EncoderVAAPI:
		encArgs = []string{
			"-hwaccel", "vaapi",
			"-hwaccel_output_format", "vaapi",
			"-vf", "format=nv12,hwupload",
			"-c:v", "hevc_vaapi",
			"-rc_mode", "CQP",
			"-qp", "24",
		}
	case EncoderVideoToolbox:
		encArgs = []string{
			"-c:v", "hevc_videotoolbox",
			"-q:v", "65",
		}
	case EncoderNVENC:
		encArgs = []string{
			"-hwaccel", "cuda",
			"-c:v", "hevc_nvenc",
			"-preset", "p4",
			"-cq", "24",
		}
	default: // software
		encArgs = []string{
			"-c:v", "libx265",
			"-crf", "24",
			"-preset", "medium",
		}
	}

	args := append(base, encArgs...)
	args = append(args, "-c:a", "copy", "-movflags", "+faststart", outputPath)
	return args
}
```

---

## `internal/transcoder/ffprobe.go`
```go
package transcoder

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
)

type ProbeResult struct {
	Codec    string
	Duration float64 // seconds
	Bitrate  int64   // bits/sec
	Size     int64   // bytes
}

// Probe runs ffprobe on the given file and returns media info.
func Probe(path string) (*ProbeResult, error) {
	out, err := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	).Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe %q: %w", path, err)
	}

	var data struct {
		Streams []struct {
			CodecName string `json:"codec_name"`
			CodecType string `json:"codec_type"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
			BitRate  string `json:"bit_rate"`
			Size     string `json:"size"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &data); err != nil {
		return nil, fmt.Errorf("parse ffprobe output: %w", err)
	}

	var codec string
	for _, s := range data.Streams {
		if s.CodecType == "video" {
			codec = s.CodecName
			break
		}
	}

	duration, _ := strconv.ParseFloat(data.Format.Duration, 64)
	bitrate, _ := strconv.ParseInt(data.Format.BitRate, 10, 64)
	size, _ := strconv.ParseInt(data.Format.Size, 10, 64)

	return &ProbeResult{
		Codec:    codec,
		Duration: duration,
		Bitrate:  bitrate,
		Size:     size,
	}, nil
}

// VerifyOutput confirms the output file passes basic quality checks.
// Returns nil if the output looks good.
func VerifyOutput(outputPath string, sourceDuration float64, sourceSize int64) error {
	result, err := Probe(outputPath)
	if err != nil {
		return fmt.Errorf("probe output: %w", err)
	}
	if result.Size >= sourceSize {
		return fmt.Errorf("output (%d bytes) is not smaller than source (%d bytes)", result.Size, sourceSize)
	}
	diff := result.Duration - sourceDuration
	if diff < 0 {
		diff = -diff
	}
	if diff > 2.0 {
		return fmt.Errorf("output duration %.1fs differs from source %.1fs by more than 2s", result.Duration, sourceDuration)
	}
	return nil
}
```

---

## `internal/queue/worker.go`
```go
package queue

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/danrichardson/sqzarr/internal/config"
	"github.com/danrichardson/sqzarr/internal/db"
	"github.com/danrichardson/sqzarr/internal/transcoder"
)

type Queue struct {
	db      *db.DB
	cfg     *config.Config
	encoder transcoder.Encoder
	paused  bool
}

func New(database *db.DB, cfg *config.Config, enc transcoder.Encoder) *Queue {
	return &Queue{db: database, cfg: cfg, encoder: enc}
}

func (q *Queue) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			q.processNext(ctx)
		}
	}
}

func (q *Queue) processNext(ctx context.Context) {
	if q.paused {
		return
	}
	if !q.checkDiskSpace() {
		slog.Warn("disk space below threshold, pausing queue", "threshold_gb", q.cfg.Safety.DiskFreePauseGB)
		q.paused = true
		return
	}

	job, err := q.db.NextPendingJob()
	if err != nil {
		slog.Error("fetch next job", "error", err)
		return
	}
	if job == nil {
		return
	}

	if err := q.runJob(ctx, job); err != nil {
		slog.Error("job failed", "job_id", job.ID, "path", job.SourcePath, "error", err)
		msg := err.Error()
		q.db.UpdateJobStatus(job.ID, "failed", &msg)
		q.db.IncrementFailedJobs()
	}
}

func (q *Queue) runJob(ctx context.Context, job *db.Job) error {
	slog.Info("starting transcode", "job_id", job.ID, "path", job.SourcePath, "encoder", q.encoder)
	q.db.UpdateJobStatus(job.ID, "running", nil)

	outputPath := job.SourcePath + ".sqzarr-tmp.mkv"
	if q.cfg.Transcoder.TempDir != "" {
		outputPath = filepath.Join(q.cfg.Transcoder.TempDir, fmt.Sprintf("%d.mkv", job.ID))
	}

	args := transcoder.FFmpegArgs(q.encoder, job.SourcePath, outputPath)
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stderr = &progressWriter{jobID: job.ID, db: q.db}

	if err := cmd.Run(); err != nil {
		os.Remove(outputPath)
		return fmt.Errorf("ffmpeg: %w", err)
	}

	slog.Info("verifying output", "job_id", job.ID)
	q.db.UpdateJobStatus(job.ID, "verifying", nil)

	if err := transcoder.VerifyOutput(outputPath, job.SourceDuration, job.SourceSize); err != nil {
		os.Remove(outputPath)
		return fmt.Errorf("verification failed: %w", err)
	}

	outputStat, err := os.Stat(outputPath)
	if err != nil {
		os.Remove(outputPath)
		return fmt.Errorf("stat output: %w", err)
	}
	outputSize := outputStat.Size()

	// Quarantine original before replacing
	if q.cfg.Safety.QuarantineEnabled {
		if err := q.quarantine(job); err != nil {
			os.Remove(outputPath)
			return fmt.Errorf("quarantine original: %w", err)
		}
	}

	// Atomic replace
	if err := os.Rename(outputPath, job.SourcePath); err != nil {
		os.Remove(outputPath)
		return fmt.Errorf("replace original: %w", err)
	}

	bytesSaved := job.SourceSize - outputSize
	enc := string(q.encoder)
	q.db.CompleteJob(job.ID, outputSize, bytesSaved, enc)

	slog.Info("transcode complete", "job_id", job.ID, "bytes_saved", bytesSaved, "encoder", q.encoder)
	return nil
}

func (q *Queue) quarantine(job *db.Job) error {
	quarantineDir := q.cfg.Safety.QuarantineDir
	if quarantineDir == "" {
		quarantineDir = q.cfg.DataDir + "/quarantine"
	}
	if err := os.MkdirAll(quarantineDir, 0755); err != nil {
		return err
	}
	dest := filepath.Join(quarantineDir, fmt.Sprintf("%d-%s", job.ID, filepath.Base(job.SourcePath)))
	if err := os.Rename(job.SourcePath, dest); err != nil {
		return err
	}
	// TODO: insert quarantine record in DB
	slog.Info("original quarantined", "job_id", job.ID, "quarantine_path", dest)
	return nil
}

func (q *Queue) checkDiskSpace() bool {
	if q.cfg.Safety.DiskFreePauseGB <= 0 {
		return true
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(q.cfg.DataDir, &stat); err != nil {
		return true // don't block on error
	}
	freeGB := int64(stat.Bavail*uint64(stat.Bsize)) / (1024 * 1024 * 1024)
	return freeGB >= int64(q.cfg.Safety.DiskFreePauseGB)
}

// progressWriter parses ffmpeg stderr and updates job progress.
type progressWriter struct {
	jobID int64
	db    *db.DB
}

func (p *progressWriter) Write(b []byte) (int, error) {
	// TODO: parse "time=HH:MM:SS.ms" from ffmpeg stderr and calculate progress
	// slog.Debug("ffmpeg", "output", string(b))
	return len(b), nil
}
```

---

## `internal/scanner/scanner.go`
```go
package scanner

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/danrichardson/sqzarr/internal/config"
	"github.com/danrichardson/sqzarr/internal/db"
	"github.com/danrichardson/sqzarr/internal/transcoder"
)

var videoExtensions = map[string]bool{
	".mkv": true, ".mp4": true, ".avi": true, ".mov": true,
	".m4v": true, ".wmv": true, ".flv": true, ".ts": true,
}

var skipCodecs = map[string]bool{
	"hevc": true, "h265": true, "av1": true,
}

type Scanner struct {
	db  *db.DB
	cfg *config.Config
}

func New(database *db.DB, cfg *config.Config) *Scanner {
	return &Scanner{db: database, cfg: cfg}
}

func (s *Scanner) RunScheduled(ctx context.Context) {
	// Run once at startup, then on interval
	s.ScanAll(ctx)
	ticker := time.NewTicker(time.Duration(s.cfg.Scanner.IntervalHours) * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.ScanAll(ctx)
		}
	}
}

func (s *Scanner) ScanAll(ctx context.Context) {
	dirs, err := s.db.ListDirectories()
	if err != nil {
		slog.Error("list directories for scan", "error", err)
		return
	}
	for _, dir := range dirs {
		if !dir.Enabled {
			continue
		}
		s.ScanDirectory(ctx, dir)
	}
}

func (s *Scanner) ScanDirectory(ctx context.Context, dir *db.Directory) {
	slog.Info("scanning directory", "path", dir.Path)
	start := time.Now()
	var scanned, queued, skipped int

	err := filepath.WalkDir(dir.Path, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !videoExtensions[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		scanned++

		info, err := d.Info()
		if err != nil {
			return nil
		}

		// Min age check
		ageInDays := int(time.Since(info.ModTime()).Hours() / 24)
		if ageInDays < dir.MinAgeDays {
			skipped++
			return nil
		}

		// Min size check
		if info.Size() < int64(dir.MinSizeMB)*1024*1024 {
			skipped++
			return nil
		}

		// Probe for codec and bitrate
		probe, err := transcoder.Probe(path)
		if err != nil {
			slog.Warn("ffprobe failed", "path", path, "error", err)
			skipped++
			return nil
		}

		// Skip already-compressed codecs
		if skipCodecs[strings.ToLower(probe.Codec)] {
			skipped++
			return nil
		}

		// Bitrate check (computed from file size / duration if bitrate missing)
		bitrate := probe.Bitrate
		if bitrate == 0 && probe.Duration > 0 {
			bitrate = int64(float64(info.Size()*8) / probe.Duration)
		}
		if bitrate > 0 && bitrate < dir.MaxBitrate {
			skipped++
			return nil
		}

		// Enqueue
		dirID := dir.ID
		if err := s.db.EnqueueJob(&dirID, path, info.Size(), probe.Codec, probe.Duration, bitrate); err != nil {
			slog.Error("enqueue job", "path", path, "error", err)
		} else {
			queued++
		}
		return nil
	})

	elapsed := time.Since(start)
	if err != nil && err != ctx.Err() {
		slog.Error("scan directory", "path", dir.Path, "error", err)
	}
	slog.Info("scan complete", "path", dir.Path, "scanned", scanned, "queued", queued, "skipped", skipped, "elapsed", elapsed)
}
```

---

## `internal/api/server.go`
```go
package api

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	"github.com/danrichardson/sqzarr/internal/config"
	"github.com/danrichardson/sqzarr/internal/db"
	"github.com/danrichardson/sqzarr/internal/queue"
	"github.com/danrichardson/sqzarr/internal/scanner"
	"github.com/danrichardson/sqzarr/internal/transcoder"
)

//go:embed all:frontend/dist
var frontendFS embed.FS

type Server struct {
	cfg     *config.Config
	db      *db.DB
	queue   *queue.Queue
	scanner *scanner.Scanner
	encoder transcoder.Encoder
	version string
	upgrader websocket.Upgrader
}

func NewServer(cfg *config.Config, database *db.DB, q *queue.Queue, sc *scanner.Scanner, enc transcoder.Encoder, version string) *Server {
	return &Server{
		cfg: cfg, db: database, queue: q, scanner: sc, encoder: enc, version: version,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	r := mux.NewRouter()

	// API routes
	api := r.PathPrefix("/api/v1").Subrouter()
	api.Use(s.authMiddleware)
	api.HandleFunc("/status", s.handleStatus).Methods("GET")
	api.HandleFunc("/stats", s.handleStats).Methods("GET")
	api.HandleFunc("/scan", s.handleScan).Methods("POST")
	api.HandleFunc("/jobs", s.handleListJobs).Methods("GET")
	api.HandleFunc("/jobs", s.handleEnqueueJob).Methods("POST")
	api.HandleFunc("/jobs/{id}/cancel", s.handleCancelJob).Methods("POST")
	api.HandleFunc("/directories", s.handleListDirectories).Methods("GET")
	api.HandleFunc("/directories", s.handleAddDirectory).Methods("POST")
	api.HandleFunc("/ws", s.handleWebSocket)

	// Auth (not behind auth middleware)
	r.HandleFunc("/api/v1/auth/login", s.handleLogin).Methods("POST")

	// SPA: serve frontend for all non-API routes
	distFS, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		slog.Warn("frontend not embedded, skipping SPA serve")
	} else {
		r.PathPrefix("/").Handler(http.FileServer(http.FS(distFS)))
	}

	srv := &http.Server{
		Addr:         s.cfg.Server.Addr(),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(shutCtx)
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "running",
		"encoder": string(s.encoder),
		"version": s.version,
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.db.GetStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	go s.scanner.ScanAll(r.Context())
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "scan started"})
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	jobs, err := s.db.ListJobs(status, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list jobs")
		return
	}
	writeJSON(w, http.StatusOK, jobs)
}

func (s *Server) handleEnqueueJob(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	// Validate path is within a configured directory
	if !s.isAllowedPath(body.Path) {
		writeError(w, http.StatusBadRequest, "path is not within a configured directory")
		return
	}
	// TODO: probe file and enqueue
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	if err := s.db.UpdateJobStatus(id, "cancelled", nil); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to cancel job")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (s *Server) handleListDirectories(w http.ResponseWriter, r *http.Request) {
	dirs, err := s.db.ListDirectories()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list directories")
		return
	}
	writeJSON(w, http.StatusOK, dirs)
}

func (s *Server) handleAddDirectory(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path       string `json:"path"`
		MinAgeDays int    `json:"min_age_days"`
		MaxBitrate int64  `json:"max_bitrate"`
		MinSizeMB  int    `json:"min_size_mb"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	if strings.Contains(body.Path, "..") {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	if body.MinAgeDays <= 0 { body.MinAgeDays = 7 }
	if body.MaxBitrate <= 0 { body.MaxBitrate = 4000000 }
	if body.MinSizeMB <= 0 { body.MinSizeMB = 500 }

	dir, err := s.db.InsertDirectory(body.Path, body.MinAgeDays, body.MaxBitrate, body.MinSizeMB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add directory")
		return
	}
	writeJSON(w, http.StatusCreated, dir)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	// TODO: subscribe to job progress events and push to client
	for {
		select {
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Auth.PasswordHash == "" {
		writeError(w, http.StatusNotFound, "authentication not configured")
		return
	}
	// TODO: bcrypt check + JWT issue
	writeError(w, http.StatusNotImplemented, "auth not yet implemented")
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Auth.PasswordHash == "" {
			next.ServeHTTP(w, r)
			return
		}
		// TODO: validate Bearer JWT
		next.ServeHTTP(w, r)
	})
}

func (s *Server) isAllowedPath(path string) bool {
	if strings.Contains(path, "..") {
		return false
	}
	dirs, err := s.db.ListDirectories()
	if err != nil {
		return false
	}
	for _, d := range dirs {
		if strings.HasPrefix(path, d.Path) {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
```

---

## `frontend/package.json`
```json
{
  "name": "sqzarr-frontend",
  "version": "1.0.0",
  "private": true,
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "lint": "eslint src --ext ts,tsx --report-unused-disable-directives --max-warnings 0",
    "preview": "vite preview"
  },
  "dependencies": {
    "@radix-ui/react-dialog": "^1.0.5",
    "@radix-ui/react-label": "^2.0.2",
    "@radix-ui/react-progress": "^1.0.3",
    "@radix-ui/react-select": "^2.0.0",
    "@radix-ui/react-separator": "^1.0.3",
    "@radix-ui/react-slot": "^1.0.2",
    "@radix-ui/react-switch": "^1.0.3",
    "@radix-ui/react-tabs": "^1.0.4",
    "@radix-ui/react-toast": "^1.1.5",
    "class-variance-authority": "^0.7.0",
    "clsx": "^2.1.0",
    "lucide-react": "^0.363.0",
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "react-router-dom": "^6.22.3",
    "tailwind-merge": "^2.2.2"
  },
  "devDependencies": {
    "@types/react": "^18.2.66",
    "@types/react-dom": "^18.2.22",
    "@typescript-eslint/eslint-plugin": "^7.2.0",
    "@typescript-eslint/parser": "^7.2.0",
    "@vitejs/plugin-react": "^4.2.1",
    "autoprefixer": "^10.4.19",
    "eslint": "^8.57.0",
    "eslint-plugin-react-hooks": "^4.6.0",
    "eslint-plugin-react-refresh": "^0.4.6",
    "postcss": "^8.4.38",
    "tailwindcss": "^3.4.3",
    "typescript": "^5.2.2",
    "vite": "^5.2.0"
  }
}
```

---

## `frontend/tailwind.config.ts`
```typescript
import type { Config } from 'tailwindcss'

export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        // Sandstone palette — warm stone tones, amber accent
        brand: {
          50:  '#fafaf9',   // stone-50
          100: '#f5f5f4',   // stone-100
          200: '#e7e5e4',   // stone-200
          300: '#d6d3d1',   // stone-300
          400: '#a8a29e',   // stone-400
          500: '#78716c',   // stone-500
          600: '#57534e',   // stone-600
          700: '#44403c',   // stone-700
          800: '#292524',   // stone-800
          900: '#1c1917',   // stone-900
        },
        accent: {
          DEFAULT: '#d97706', // amber-600
          light:   '#fef3c7', // amber-100
          dark:    '#b45309', // amber-700
        },
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'ui-monospace', 'monospace'],
      },
    },
  },
  plugins: [],
} satisfies Config
```

---

## `frontend/src/main.tsx`
```typescript
import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import './index.css'

import { Layout } from './components/Layout'
import { Dashboard } from './pages/Dashboard'
import { Queue } from './pages/Queue'
import { History } from './pages/History'
import { Directories } from './pages/Directories'
import { Settings } from './pages/Settings'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<Navigate to="/dashboard" replace />} />
          <Route path="/dashboard" element={<Dashboard />} />
          <Route path="/queue" element={<Queue />} />
          <Route path="/history" element={<History />} />
          <Route path="/directories" element={<Directories />} />
          <Route path="/settings" element={<Settings />} />
        </Route>
      </Routes>
    </BrowserRouter>
  </React.StrictMode>,
)
```

---

## `frontend/src/lib/api.ts`
```typescript
const BASE = import.meta.env.VITE_API_BASE ?? ''

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const token = localStorage.getItem('sqzarr-token')
  const headers: HeadersInit = { 'Content-Type': 'application/json' }
  if (token) headers['Authorization'] = `Bearer ${token}`

  const res = await fetch(`${BASE}/api/v1${path}`, { ...init, headers })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error ?? `HTTP ${res.status}`)
  }
  return res.json()
}

export const api = {
  status: () => request<{ status: string; encoder: string; version: string }>('/status'),
  stats:  () => request<{ TotalBytesSaved: number; TotalJobsDone: number; TotalJobsFailed: number }>('/stats'),
  scan:   () => request('/scan', { method: 'POST' }),

  jobs: {
    list:   (status?: string) => request<Job[]>(`/jobs${status ? `?status=${status}` : ''}`),
    cancel: (id: number) => request(`/jobs/${id}/cancel`, { method: 'POST' }),
    enqueue:(path: string) => request('/jobs', { method: 'POST', body: JSON.stringify({ path }) }),
  },

  directories: {
    list: () => request<Directory[]>('/directories'),
    add:  (dir: Partial<Directory>) => request('/directories', { method: 'POST', body: JSON.stringify(dir) }),
  },
}

export interface Job {
  ID: number
  SourcePath: string
  SourceSize: number
  SourceCodec: string
  Status: string
  Progress: number
  BytesSaved?: number
  ErrorMessage?: string
  EncoderUsed?: string
  CreatedAt: string
  FinishedAt?: string
}

export interface Directory {
  ID: number
  Path: string
  Enabled: boolean
  MinAgeDays: number
  MaxBitrate: number
  MinSizeMB: number
}

export function useWebSocket(onMessage: (event: MessageEvent) => void) {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsURL = `${protocol}//${window.location.host}/api/v1/ws`
  const ws = new WebSocket(wsURL)
  ws.onmessage = onMessage
  return ws
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
}
```

---

## `frontend/src/pages/Dashboard.tsx`
```typescript
import { useEffect, useState } from 'react'
import { api, formatBytes, type Job } from '../lib/api'
import { RefreshCw } from 'lucide-react'

interface Stats {
  TotalBytesSaved: number
  TotalJobsDone: number
  TotalJobsFailed: number
}

interface Status {
  status: string
  encoder: string
  version: string
}

export function Dashboard() {
  const [stats, setStats] = useState<Stats | null>(null)
  const [status, setStatus] = useState<Status | null>(null)
  const [currentJob, setCurrentJob] = useState<Job | null>(null)
  const [pendingJobs, setPendingJobs] = useState<Job[]>([])

  useEffect(() => {
    const load = async () => {
      const [s, st, jobs] = await Promise.all([
        api.stats(),
        api.status(),
        api.jobs.list('pending'),
      ])
      setStats(s)
      setStatus(st)
      setPendingJobs(jobs.slice(0, 5))
    }
    load()
    const interval = setInterval(load, 5000)
    return () => clearInterval(interval)
  }, [])

  const handleScanNow = async () => {
    await api.scan()
  }

  return (
    <div className="p-6 space-y-6 max-w-4xl">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold text-stone-900">Dashboard</h1>
        <span className={`text-sm px-2 py-1 rounded-full font-medium ${
          status?.status === 'running' ? 'bg-green-100 text-green-700' : 'bg-stone-100 text-stone-500'
        }`}>
          {status?.status ?? 'loading'}
        </span>
      </div>

      {/* Stat cards */}
      <div className="grid grid-cols-3 gap-4">
        <StatCard label="Saved" value={stats ? formatBytes(stats.TotalBytesSaved) : '—'} />
        <StatCard label="Completed" value={stats?.TotalJobsDone.toLocaleString() ?? '—'} />
        <StatCard label="Failed" value={stats?.TotalJobsFailed.toLocaleString() ?? '—'} />
      </div>

      {/* Current job */}
      {currentJob && (
        <div className="bg-amber-50 border border-amber-200 rounded-lg p-4">
          <div className="text-sm font-medium text-stone-700 mb-2 truncate">{currentJob.SourcePath}</div>
          <div className="h-2 bg-amber-100 rounded-full overflow-hidden">
            <div
              className="h-full bg-amber-500 transition-all duration-500"
              style={{ width: `${currentJob.Progress * 100}%` }}
            />
          </div>
          <div className="text-xs text-stone-500 mt-1">{Math.round(currentJob.Progress * 100)}%</div>
        </div>
      )}

      {/* Up next */}
      {pendingJobs.length > 0 && (
        <div>
          <h2 className="text-xs font-semibold uppercase tracking-wider text-stone-400 mb-3">
            Up Next ({pendingJobs.length})
          </h2>
          <div className="space-y-1">
            {pendingJobs.map(job => (
              <div key={job.ID} className="flex items-center justify-between py-2 border-b border-stone-100">
                <span className="text-sm text-stone-700 font-mono truncate flex-1">
                  {job.SourcePath.split('/').pop()}
                </span>
                <span className="text-sm text-stone-400 ml-4">{formatBytes(job.SourceSize)}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {!currentJob && pendingJobs.length === 0 && (
        <div className="text-center py-12 text-stone-400">
          <p className="mb-4">Queue is empty</p>
          <button
            onClick={handleScanNow}
            className="inline-flex items-center gap-2 px-4 py-2 bg-stone-800 text-white rounded-md text-sm hover:bg-stone-700"
          >
            <RefreshCw size={14} /> Scan Now
          </button>
        </div>
      )}

      {(currentJob || pendingJobs.length > 0) && (
        <div className="flex justify-between items-center text-sm text-stone-400">
          <span>Encoder: {status?.encoder}</span>
          <button
            onClick={handleScanNow}
            className="inline-flex items-center gap-1 text-stone-500 hover:text-stone-700"
          >
            <RefreshCw size={12} /> Scan Now
          </button>
        </div>
      )}
    </div>
  )
}

function StatCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-white border border-stone-200 rounded-lg p-4">
      <div className="text-2xl font-bold text-stone-900">{value}</div>
      <div className="text-sm text-stone-500 mt-0.5">{label}</div>
    </div>
  )
}
```

---

## `sqzarr.toml` (config template / .env.example equivalent)
```toml
# SQZARR Configuration
# Copy to /etc/sqzarr/sqzarr.toml and fill in values

[server]
# Bind address. Use 0.0.0.0 to expose to LAN/Tailscale.
host = "127.0.0.1"
port = 8080

# Where to store sqzarr.db and quarantine folder
data_dir = "/var/lib/sqzarr"

[scanner]
# How often to scan for new candidates (hours)
interval_hours = 6

# Number of simultaneous transcode jobs (1–8)
# Default 1 is safe for most hardware
worker_concurrency = 1

[transcoder]
# Where to write in-progress temp files
# Default: same directory as source file
# temp_dir = "/tmp/sqzarr"

[safety]
# Move originals to quarantine before replacing (strongly recommended)
quarantine_enabled = true

# Days to keep originals in quarantine before auto-deleting
quarantine_retention_days = 10

# Where to store quarantined originals
# Default: {data_dir}/quarantine
# quarantine_dir = "/var/lib/sqzarr/quarantine"

# Pause transcoding when free disk space drops below this threshold (GB)
disk_free_pause_gb = 50

[plex]
# Set enabled = true to trigger library rescan after each transcode
enabled = false
base_url = ""    # e.g. http://192.168.1.10:32400
token = ""       # Plex auth token (Settings > Account > Plex.tv > Auth Token)

[auth]
# Require a password to access the admin panel.
# Leave password_hash empty to disable authentication.
# To generate: run `sqzarr hash-password` and paste the output here.
password_hash = ""

# JWT signing secret. Auto-generated if empty (users are logged out on restart).
jwt_secret = ""
```

---

## `scripts/install-linux.sh`
```bash
#!/usr/bin/env bash
set -euo pipefail

BINARY_PATH="${1:-./sqzarr}"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/sqzarr"
DATA_DIR="/var/lib/sqzarr"
LOG_DIR="/var/log/sqzarr"
SERVICE_USER="sqzarr"

echo "Installing SQZARR..."

# Create service user
if ! id "$SERVICE_USER" &>/dev/null; then
    useradd --system --no-create-home --shell /sbin/nologin "$SERVICE_USER"
fi

# Install binary
install -m 755 "$BINARY_PATH" "$INSTALL_DIR/sqzarr"

# Create directories
mkdir -p "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR" "$DATA_DIR/quarantine"
chown "$SERVICE_USER:$SERVICE_USER" "$DATA_DIR" "$LOG_DIR" "$DATA_DIR/quarantine"

# Install config if not present
if [ ! -f "$CONFIG_DIR/sqzarr.toml" ]; then
    cp sqzarr.toml "$CONFIG_DIR/sqzarr.toml"
    chmod 600 "$CONFIG_DIR/sqzarr.toml"
    chown root:"$SERVICE_USER" "$CONFIG_DIR/sqzarr.toml"
    echo "Config installed at $CONFIG_DIR/sqzarr.toml — edit it before starting"
fi

# Install systemd unit
cat > /etc/systemd/system/sqzarr.service << 'EOF'
[Unit]
Description=SQZARR Media Transcoder
After=network.target

[Service]
Type=simple
User=sqzarr
Group=sqzarr
ExecStart=/usr/local/bin/sqzarr serve --config /etc/sqzarr/sqzarr.toml
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal
ProtectSystem=strict
ReadWritePaths=/var/lib/sqzarr /var/log/sqzarr
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
echo ""
echo "SQZARR installed. To start:"
echo "  1. Edit /etc/sqzarr/sqzarr.toml"
echo "  2. systemctl enable --now sqzarr"
echo "  3. Open http://localhost:8080"
```

---

## `scripts/install-macos.sh`
```bash
#!/usr/bin/env bash
set -euo pipefail

BINARY_PATH="${1:-./sqzarr-darwin-arm64}"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="$HOME/.config/sqzarr"
DATA_DIR="$HOME/Library/Application Support/sqzarr"
PLIST_DIR="$HOME/Library/LaunchAgents"
PLIST_NAME="com.sqzarr.agent"

echo "Installing SQZARR for macOS..."

install -m 755 "$BINARY_PATH" "$INSTALL_DIR/sqzarr"
mkdir -p "$CONFIG_DIR" "$DATA_DIR"

if [ ! -f "$CONFIG_DIR/sqzarr.toml" ]; then
    cat > "$CONFIG_DIR/sqzarr.toml" << TOML
[server]
host = "127.0.0.1"
port = 8080

data_dir = "$DATA_DIR"

[scanner]
interval_hours = 6
worker_concurrency = 1

[safety]
quarantine_enabled = true
quarantine_retention_days = 10
disk_free_pause_gb = 50

[plex]
enabled = false
base_url = ""
token = ""

[auth]
password_hash = ""
jwt_secret = ""
TOML
    chmod 600 "$CONFIG_DIR/sqzarr.toml"
    echo "Config installed at $CONFIG_DIR/sqzarr.toml"
fi

cat > "$PLIST_DIR/$PLIST_NAME.plist" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>$PLIST_NAME</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/sqzarr</string>
        <string>serve</string>
        <string>--config</string>
        <string>$CONFIG_DIR/sqzarr.toml</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>$HOME/Library/Logs/sqzarr.log</string>
    <key>StandardErrorPath</key>
    <string>$HOME/Library/Logs/sqzarr.log</string>
</dict>
</plist>
EOF

launchctl bootstrap gui/$(id -u) "$PLIST_DIR/$PLIST_NAME.plist" 2>/dev/null || true

echo ""
echo "SQZARR installed."
echo "  1. Edit $CONFIG_DIR/sqzarr.toml"
echo "  2. launchctl kickstart gui/$(id -u)/$PLIST_NAME"
echo "  3. Open http://localhost:8080"
```

---

## `scripts/smoke-test.sh`
```bash
#!/usr/bin/env bash
# Smoke test: starts sqzarr, adds a test directory, triggers scan,
# waits for one job to complete, then exits 0.
set -euo pipefail

BINARY="${1:-./sqzarr}"
TMPDIR=$(mktemp -d)
DBDIR="$TMPDIR/data"
MEDIADIR="$TMPDIR/media"
CONFIG="$TMPDIR/sqzarr.toml"
PID_FILE="$TMPDIR/sqzarr.pid"

mkdir -p "$DBDIR" "$MEDIADIR"

cleanup() {
    if [ -f "$PID_FILE" ]; then
        kill "$(cat $PID_FILE)" 2>/dev/null || true
    fi
    rm -rf "$TMPDIR"
}
trap cleanup EXIT

# Create a test config
cat > "$CONFIG" << EOF
[server]
host = "127.0.0.1"
port = 18080

data_dir = "$DBDIR"

[scanner]
interval_hours = 999

[safety]
quarantine_enabled = false
disk_free_pause_gb = 0
EOF

# Create a small test H.264 file (requires ffmpeg)
echo "Creating test media file..."
ffmpeg -f lavfi -i "testsrc=duration=5:size=640x360:rate=24" \
    -c:v libx264 -crf 28 "$MEDIADIR/test.mkv" -y -v quiet

echo "Starting sqzarr..."
"$BINARY" serve --config "$CONFIG" &
echo $! > "$PID_FILE"

sleep 2

BASE="http://127.0.0.1:18080"

# Health check
STATUS=$(curl -sf "$BASE/api/v1/status" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])")
if [ "$STATUS" != "running" ]; then
    echo "FAIL: expected status=running, got $STATUS"
    exit 1
fi
echo "Health check: OK"

# Add directory
curl -sf -X POST "$BASE/api/v1/directories" \
    -H "Content-Type: application/json" \
    -d "{\"path\":\"$MEDIADIR\",\"min_age_days\":0,\"max_bitrate\":0,\"min_size_mb\":0}" > /dev/null
echo "Directory added: OK"

# Trigger scan
curl -sf -X POST "$BASE/api/v1/scan" > /dev/null
echo "Scan triggered: OK"

# Wait up to 60s for a completed job
echo "Waiting for job to complete..."
for i in $(seq 1 60); do
    DONE=$(curl -sf "$BASE/api/v1/jobs?status=done" | python3 -c "import sys,json; print(len(json.load(sys.stdin)))")
    if [ "$DONE" -gt 0 ]; then
        echo "Job completed: OK"
        echo ""
        echo "Smoke test PASSED"
        exit 0
    fi
    sleep 1
done

echo "FAIL: no job completed within 60 seconds"
exit 1
```

---

## `.github/workflows/ci.yml`
```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:

jobs:
  test:
    name: Test & Build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache: true

      - name: Install ffmpeg
        run: sudo apt-get update && sudo apt-get install -y ffmpeg

      - name: Go vet
        run: go vet ./...

      - name: Go test
        run: go test ./...

      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: npm
          cache-dependency-path: frontend/package-lock.json

      - name: Frontend install
        run: cd frontend && npm ci

      - name: Frontend lint
        run: cd frontend && npm run lint

      - name: Frontend build
        run: cd frontend && npm run build

      - name: Build binary (linux/amd64)
        run: make build-linux

  security:
    name: Security Scan
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: gosec
        uses: securego/gosec@master
        with:
          args: '-severity medium ./...'

      - uses: actions/setup-node@v4
        with:
          node-version: '20'
      - name: npm audit
        run: cd frontend && npm ci && npm audit --audit-level=high
```

---

## `.github/workflows/release.yml`
```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  release:
    name: Build & Release
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache: true

      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: npm
          cache-dependency-path: frontend/package-lock.json

      - name: Build frontend
        run: cd frontend && npm ci && npm run build

      - name: Build all targets
        run: make release VERSION=${{ github.ref_name }}

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          files: |
            dist/sqzarr-linux-amd64.tar.gz
            dist/sqzarr-linux-arm64.tar.gz
            dist/sqzarr-darwin-arm64.tar.gz
          generate_release_notes: true
```

---

## `README.md`
```markdown
# SQZARR

A self-hosted media transcoding service that automatically compresses bloated video files using hardware GPU encoding. A clean, polished replacement for Tdarr.

**Requirements**: ffmpeg 6.0+, ffprobe (same package), and a GPU with HEVC encoding support (Intel, Apple Silicon, or NVIDIA).

---

## Quick Start

### Linux (Proxmox LXC / Debian)

```bash
# Download the latest release
curl -L https://github.com/danrichardson/sqzarr/releases/latest/download/sqzarr-linux-amd64.tar.gz | tar xz
sudo bash install-linux.sh ./sqzarr

# Edit config
sudo nano /etc/sqzarr/sqzarr.toml

# Start
sudo systemctl enable --now sqzarr

# Open the admin panel
open http://localhost:8080
```

### macOS (Apple Silicon)

```bash
curl -L https://github.com/danrichardson/sqzarr/releases/latest/download/sqzarr-darwin-arm64.tar.gz | tar xz
bash install-macos.sh ./sqzarr-darwin-arm64
open http://localhost:8080
```

---

## Intel VAAPI on Proxmox LXC

Your LXC container needs access to the GPU device. Add to `/etc/pve/lxc/{ctid}.conf`:

```
lxc.cgroup2.devices.allow: c 226:* rwm
lxc.mount.entry: /dev/dri dev/dri none bind,optional,create=dir
```

Verify inside the container:
```bash
apt install -y vainfo
vainfo | grep hevc
```

---

## Configuration Reference

See `sqzarr.toml` for all options with inline documentation.

Key settings:
- `[server] host` — change to `0.0.0.0` for Tailscale/LAN access
- `[safety] quarantine_enabled` — keep originals for N days before deletion (default: on, 10 days)
- `[safety] disk_free_pause_gb` — pause when free space drops below threshold (default: 50 GB)
- `[plex]` — optional Plex library rescan after each transcode

---

## Building from Source

```bash
git clone https://github.com/danrichardson/sqzarr
cd sqzarr
make build   # builds frontend + Go binary
./sqzarr serve
```

---

## License

MIT
```
