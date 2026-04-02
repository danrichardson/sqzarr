package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"golang.org/x/crypto/bcrypt"

	"github.com/danrichardson/sqzarr/internal/api"
	"github.com/danrichardson/sqzarr/internal/config"
	"github.com/danrichardson/sqzarr/internal/db"
	"github.com/danrichardson/sqzarr/internal/logger"
	"github.com/danrichardson/sqzarr/internal/plex"
	"github.com/danrichardson/sqzarr/internal/queue"
	"github.com/danrichardson/sqzarr/internal/scanner"
	"github.com/danrichardson/sqzarr/internal/transcoder"
)

const usage = `sqzarr — self-hosted GPU media transcoder

Usage:
  sqzarr serve              Start the HTTP server and worker daemon
  sqzarr scan-once          Run a single scan pass and exit
  sqzarr restore <job-id>   Restore original file from quarantine
  sqzarr hash-password      Hash a password for use in sqzarr.toml

Flags:
  -config string   Path to sqzarr.toml (default: /etc/sqzarr/sqzarr.toml)
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	cfgPath := flag.String("config", "/etc/sqzarr/sqzarr.toml", "path to sqzarr.toml")

	switch os.Args[1] {
	case "serve":
		flag.CommandLine.Parse(os.Args[2:])
		runServe(*cfgPath)
	case "scan-once":
		dryRun := flag.Bool("dry-run", false, "scan without enqueuing jobs")
		flag.CommandLine.Parse(os.Args[2:])
		runScanOnce(*cfgPath, *dryRun)
	case "restore":
		flag.CommandLine.Parse(os.Args[2:])
		args := flag.Args()
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "usage: sqzarr restore <job-id>")
			os.Exit(1)
		}
		runRestore(*cfgPath, args[0])
	case "hash-password":
		runHashPassword()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", os.Args[1], usage)
		os.Exit(1)
	}
}

func runServe(cfgPath string) {
	log := logger.New()

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Error("load config", "error", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(cfg.Server.DataDir, 0o750); err != nil {
		log.Error("create data dir", "error", err)
		os.Exit(1)
	}

	database, err := db.Open(cfg.DBPath())
	if err != nil {
		log.Error("open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	enc, err := transcoder.Detect()
	if err != nil {
		log.Error("detect encoder", "error", err)
		os.Exit(1)
	}
	log.Info("encoder selected", "encoder", enc.DisplayName)

	t := transcoder.New(enc, cfg.Transcoder.TempDir, log)

	var plexNotifier queue.PlexNotifier
	if cfg.Plex.Enabled {
		plexNotifier = plex.New(cfg.Plex.BaseURL, cfg.Plex.Token, log)
	}

	worker := queue.New(database, cfg, t, plexNotifier, log)
	scan := scanner.New(database, log)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Info("sqzarr starting", "addr", cfg.Addr(), "data_dir", cfg.Server.DataDir)

	// Start quarantine GC in background.
	if cfg.Safety.QuarantineEnabled {
		gc := queue.NewQuarantineGC(database, log)
		go gc.Run(ctx)
	}

	// Start HTTP server in background.
	httpServer := api.New(cfg, database, worker, scan, enc, log)
	go func() {
		if err := httpServer.Start(ctx); err != nil {
			log.Error("HTTP server error", "error", err)
		}
	}()

	worker.Run(ctx)
}

func runScanOnce(cfgPath string, dryRun bool) {
	log := logger.New()

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Error("load config", "error", err)
		os.Exit(1)
	}

	database, err := db.Open(cfg.DBPath())
	if err != nil {
		log.Error("open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	s := scanner.New(database, log)
	dirs, err := database.ListDirectories()
	if err != nil {
		log.Error("list directories", "error", err)
		os.Exit(1)
	}

	if len(dirs) == 0 {
		log.Info("no directories configured")
		return
	}

	ctx := context.Background()
	for _, d := range dirs {
		if dryRun {
			log.Info("dry-run scan", "directory", d.Path)
		}
		result, err := s.ScanDirectory(ctx, d)
		if err != nil {
			log.Error("scan directory", "directory", d.Path, "error", err)
			continue
		}
		log.Info("scan result",
			"directory", d.Path,
			"scanned", result.FilesScanned,
			"queued", result.FilesQueued,
			"skipped", result.FilesSkipped,
		)
	}
}

func runRestore(cfgPath string, jobIDStr string) {
	log := logger.New()

	jobID, err := strconv.ParseInt(jobIDStr, 10, 64)
	if err != nil {
		log.Error("invalid job ID", "value", jobIDStr)
		os.Exit(1)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Error("load config", "error", err)
		os.Exit(1)
	}

	database, err := db.Open(cfg.DBPath())
	if err != nil {
		log.Error("open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	job, err := database.GetJob(jobID)
	if err != nil || job == nil {
		log.Error("job not found", "job_id", jobID)
		os.Exit(1)
	}

	q, err := database.GetQuarantineByJobID(jobID)
	if err != nil || q == nil {
		log.Error("no quarantine record for job", "job_id", jobID)
		os.Exit(1)
	}

	// Move original back from quarantine to its original path.
	if err := os.MkdirAll(fmt.Sprintf("%s/..", q.OriginalPath), 0o755); err != nil {
		log.Error("create parent dir", "error", err)
		os.Exit(1)
	}
	if err := os.Rename(q.QuarantinePath, q.OriginalPath); err != nil {
		log.Error("restore file", "error", err)
		os.Exit(1)
	}

	// Remove transcoded file (now at job.SourcePath after replacement).
	if job.OutputPath.Valid && job.OutputPath.String != q.OriginalPath {
		os.Remove(job.OutputPath.String)
	}

	if err := database.UpdateJobStatus(jobID, db.JobCancelled, "restored from quarantine"); err != nil {
		log.Error("update job status", "error", err)
		os.Exit(1)
	}

	log.Info("restore complete", "job_id", jobID, "path", q.OriginalPath)
}

func runHashPassword() {
	var password string
	fmt.Print("Password: ")
	fmt.Scan(&password)
	if password == "" {
		fmt.Fprintln(os.Stderr, "password must not be empty")
		os.Exit(1)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hash error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("password_hash = %q\n", string(hash))
}
