package queue

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/danrichardson/sqzarr/internal/db"
)

// QuarantineGC runs as a background goroutine, deleting expired quarantine
// originals every hour.
type QuarantineGC struct {
	db  *db.DB
	log *slog.Logger
}

// NewQuarantineGC creates a QuarantineGC.
func NewQuarantineGC(database *db.DB, log *slog.Logger) *QuarantineGC {
	return &QuarantineGC{db: database, log: log}
}

// Run starts the GC loop. Blocks until ctx is cancelled.
func (gc *QuarantineGC) Run(ctx context.Context) {
	gc.log.Info("quarantine GC started")
	// Run immediately on startup, then every hour.
	gc.Sweep()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			gc.log.Info("quarantine GC stopped")
			return
		case <-ticker.C:
			gc.Sweep()
		}
	}
}

// Sweep deletes all expired quarantine files immediately.
// Exported for use in tests.
func (gc *QuarantineGC) Sweep() {
	gc.sweep()
}

func (gc *QuarantineGC) sweep() {
	records, err := gc.db.ExpiredQuarantines()
	if err != nil {
		gc.log.Error("quarantine GC: list expired", "error", err)
		return
	}
	for _, r := range records {
		if err := os.Remove(r.QuarantinePath); err != nil && !os.IsNotExist(err) {
			gc.log.Error("quarantine GC: remove file", "path", r.QuarantinePath, "error", err)
			continue
		}
		if err := gc.db.MarkQuarantineDeleted(r.ID); err != nil {
			gc.log.Error("quarantine GC: mark deleted", "id", r.ID, "error", err)
			continue
		}
		gc.log.Info("quarantine expired", "path", r.QuarantinePath, "job_id", r.JobID)
	}
}
