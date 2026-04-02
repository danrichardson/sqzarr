package queue

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/danrichardson/sqzarr/internal/config"
	"github.com/danrichardson/sqzarr/internal/db"
	"github.com/danrichardson/sqzarr/internal/transcoder"
	"github.com/danrichardson/sqzarr/internal/verifier"
)

// EventType identifies the kind of worker event.
type EventType string

const (
	EventProgress EventType = "progress"
	EventDone     EventType = "done"
	EventFailed   EventType = "failed"
	EventPaused   EventType = "paused"
)

// Event is published to observers when job state changes.
type Event struct {
	Type     EventType
	JobID    int64
	Progress float64
	Error    string
}

// Observer receives queue events.
type Observer func(Event)

// PlexNotifier is the interface for triggering Plex library rescans.
type PlexNotifier interface {
	NotifyFileReplaced(path string)
}

// Worker processes transcode jobs from the database queue.
type Worker struct {
	db         *db.DB
	cfg        *config.Config
	transcoder *transcoder.Transcoder
	plex       PlexNotifier
	log        *slog.Logger

	mu        sync.RWMutex
	observers []Observer
	paused    bool
}

// New creates a Worker.
func New(database *db.DB, cfg *config.Config, enc *transcoder.Transcoder, plex PlexNotifier, log *slog.Logger) *Worker {
	return &Worker{
		db:         database,
		cfg:        cfg,
		transcoder: enc,
		plex:       plex,
		log:        log,
	}
}

// Subscribe registers an observer for job events.
func (w *Worker) Subscribe(obs Observer) {
	w.mu.Lock()
	w.observers = append(w.observers, obs)
	w.mu.Unlock()
}

// IsPaused returns whether the worker is currently paused.
func (w *Worker) IsPaused() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.paused
}

// SetPaused pauses or resumes the worker.
func (w *Worker) SetPaused(paused bool) {
	w.mu.Lock()
	w.paused = paused
	w.mu.Unlock()
}

// Run starts the worker loop. It blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	w.log.Info("worker started")
	for {
		select {
		case <-ctx.Done():
			w.log.Info("worker stopped")
			return
		case <-time.After(5 * time.Second):
			if err := w.tick(ctx); err != nil {
				w.log.Error("worker tick error", "error", err)
			}
		}
	}
}

func (w *Worker) tick(ctx context.Context) error {
	if w.IsPaused() {
		return nil
	}

	if err := w.checkDiskSpace(); err != nil {
		w.log.Warn("disk space guard triggered, pausing queue", "error", err)
		w.SetPaused(true)
		w.emit(Event{Type: EventPaused, Error: err.Error()})
		return nil
	}

	job, err := w.db.NextPendingJob()
	if err != nil {
		return fmt.Errorf("next job: %w", err)
	}
	if job == nil {
		return nil
	}

	return w.processJob(ctx, job)
}

func (w *Worker) processJob(ctx context.Context, job *db.Job) error {
	w.log.Info("starting job", "job_id", job.ID, "source", job.SourcePath)

	if err := w.db.UpdateJobStatus(job.ID, db.JobRunning, ""); err != nil {
		return fmt.Errorf("set running: %w", err)
	}

	onProgress := func(p float64) {
		w.db.UpdateJobProgress(job.ID, p)
		w.emit(Event{Type: EventProgress, JobID: job.ID, Progress: p})
	}

	outputPath, err := w.transcoder.Run(ctx, job.SourcePath, job.SourceDuration, onProgress)
	if err != nil {
		w.log.Error("transcode failed", "job_id", job.ID, "error", err)
		w.db.UpdateJobStatus(job.ID, db.JobFailed, err.Error())
		w.db.RecordJobFailed()
		w.emit(Event{Type: EventFailed, JobID: job.ID, Error: err.Error()})
		return nil
	}

	// Verify output.
	result, err := verifier.Verify(job.SourcePath, outputPath, 1.0)
	if err != nil {
		os.Remove(outputPath)
		w.log.Error("verify error", "job_id", job.ID, "error", err)
		w.db.UpdateJobStatus(job.ID, db.JobFailed, err.Error())
		w.db.RecordJobFailed()
		w.emit(Event{Type: EventFailed, JobID: job.ID, Error: err.Error()})
		return nil
	}
	if !result.OK {
		os.Remove(outputPath)
		w.log.Error("verify failed", "job_id", job.ID, "reason", result.Reason)
		w.db.UpdateJobStatus(job.ID, db.JobFailed, result.Reason)
		w.db.RecordJobFailed()
		w.emit(Event{Type: EventFailed, JobID: job.ID, Error: result.Reason})
		return nil
	}

	// Quarantine original before replacing.
	if w.cfg.Safety.QuarantineEnabled {
		if err := w.quarantine(job); err != nil {
			os.Remove(outputPath)
			w.log.Error("quarantine failed", "job_id", job.ID, "error", err)
			w.db.UpdateJobStatus(job.ID, db.JobFailed, err.Error())
			w.db.RecordJobFailed()
			w.emit(Event{Type: EventFailed, JobID: job.ID, Error: err.Error()})
			return nil
		}
	}

	// Replace original with transcoded output (atomic on same filesystem).
	if err := os.Rename(outputPath, job.SourcePath); err != nil {
		w.log.Error("rename failed", "job_id", job.ID, "error", err)
		w.db.UpdateJobStatus(job.ID, db.JobFailed, err.Error())
		w.db.RecordJobFailed()
		w.emit(Event{Type: EventFailed, JobID: job.ID, Error: err.Error()})
		return nil
	}

	bytesSaved := result.InputSize - result.OutputSize
	encoderName := string(w.transcoder.Encoder().Type)

	if err := w.db.CompleteJob(job.ID, job.SourcePath, result.OutputSize, encoderName, bytesSaved); err != nil {
		return fmt.Errorf("complete job in db: %w", err)
	}
	w.db.RecordJobDone(bytesSaved)

	w.log.Info("job complete",
		"job_id", job.ID,
		"bytes_saved", bytesSaved,
		"encoder", encoderName,
	)
	w.emit(Event{Type: EventDone, JobID: job.ID, Progress: 1.0})

	// Notify Plex — non-fatal.
	if w.plex != nil {
		go w.plex.NotifyFileReplaced(job.SourcePath)
	}

	return nil
}

func (w *Worker) quarantine(job *db.Job) error {
	quarantineDir := w.cfg.QuarantineDir()

	// Preserve directory hierarchy relative to any configured media root.
	// We use the source path directly, stripping the leading separator.
	relPath := strings.TrimPrefix(job.SourcePath, string(filepath.Separator))
	destPath := filepath.Join(quarantineDir, relPath)

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("create quarantine dir: %w", err)
	}

	// Copy rather than move so that if the subsequent rename fails we still
	// have the original in place.
	if err := copyFile(job.SourcePath, destPath); err != nil {
		return fmt.Errorf("copy to quarantine: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(w.cfg.Safety.QuarantineRetentionDays) * 24 * time.Hour)
	rec := &db.QuarantineRecord{
		JobID:          job.ID,
		OriginalPath:   job.SourcePath,
		QuarantinePath: destPath,
		ExpiresAt:      expiresAt,
	}
	_, err := w.db.InsertQuarantine(rec)
	return err
}

func (w *Worker) checkDiskSpace() error {
	threshold := int64(w.cfg.Safety.DiskFreePauseGB) * 1024 * 1024 * 1024
	if threshold <= 0 {
		return nil
	}

	checkDir := w.cfg.Safety.QuarantineDir
	if checkDir == "" {
		checkDir = w.cfg.Server.DataDir
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(checkDir, &stat); err != nil {
		return nil // Can't check — don't block.
	}

	free := int64(stat.Bavail) * stat.Bsize
	if free < threshold {
		return fmt.Errorf("free disk space %d GB below threshold %d GB",
			free/(1024*1024*1024), w.cfg.Safety.DiskFreePauseGB)
	}
	return nil
}

func (w *Worker) emit(e Event) {
	w.mu.RLock()
	obs := make([]Observer, len(w.observers))
	copy(obs, w.observers)
	w.mu.RUnlock()

	for _, o := range obs {
		o(e)
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	buf := make([]byte, 4*1024*1024) // 4 MB buffer
	for {
		n, err := in.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}
	return out.Sync()
}
