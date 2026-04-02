# WORKFLOW.md
# SQZARR — Media Transcoder

---

## Vision

SQZARR is a self-hosted media transcoding service that automatically shrinks bloated video files using hardware GPU encoding. It replaces Tdarr — a tool that was overcomplicated, brittle, and failed in production. SQZARR is everything Tdarr should have been: focused, reliable, polished, and open source from day one. A single Go binary, a clean React admin panel, no Docker, no subscriptions, no bullshit.

**One-liner**: It scans your media library, finds files that are too big, and quietly compresses them using your GPU — safely, automatically, beautifully.

**Target users**: Technical home server owners (Proxmox, NAS, Mac Mini) with Sonarr/Plex setups who are running out of space.

---

## 1.0 Definition

### Success Criteria

- [ ] Service runs as a systemd daemon on Linux LXC and as a launchd daemon on macOS
- [ ] Scans one or more configured media directories on a configurable schedule (default: every 6 hours)
- [ ] Rules engine correctly identifies qualifying files (age, codec, bitrate, size)
- [ ] Transcodes to HEVC using Intel VAAPI (primary hardware target for 1.0 personal use)
- [ ] Hardware detection auto-selects VAAPI / VideoToolbox / NVENC / software as available
- [ ] Output verified (duration match ±1s, output smaller than input) before replacing original
- [ ] Original moved to quarantine folder; auto-deleted after configurable retention period (default: 10 days)
- [ ] Disk space guard pauses queue when free space drops below threshold
- [ ] React admin panel fully functional: dashboard, queue, history, directories, settings
- [ ] Optional admin password with JWT auth
- [ ] Optional Plex library rescan after successful replacement
- [ ] Logs structured JSON; no silent failures
- [ ] Runs cleanly in dogfood on personal library for 1 week with no data loss
- [ ] Published to GitHub with clean professional README and no AI attribution in commits

### Explicit Out-of-Scope for 1.0

- Multi-user accounts or role-based access
- Push/email/webhook notifications (logs only)
- Jellyfin / Emby API integration
- Remote or cloud storage targets
- Mobile app (responsive web UI is sufficient)
- Paid tiers, licensing, or access controls of any kind
- Automated OS package builds (apt/brew) — manual install only at 1.0

---

## Process Contract

**Collaboration model**: Owner is closely involved during spec phase (this document). Hands-off during implementation — agent builds autonomously and flags blockers.

**Notification method**: Agent logs decisions and assumptions in the Decision Log section at the bottom of this file. Owner reviews asynchronously.

**Agent delegation**: If the agent notices something broken or inconsistent outside the current task, fix it. Don't wait for permission on clearly-wrong things.

**Review cadence**: Owner reviews output at the end of each Phase (not per-step). Phase is done when Definition of Done criteria are met.

**Commit hygiene**: No "Committed by Claude" or similar AI attribution in commit messages. Clean, professional git history.

---

## Flourish Policy

**Default level**: Production quality throughout. This is not a prototype. Every layer ships as if it's the final thing.

**UI**: High quality is required and non-negotiable. Sandstone palette (warm stone tones, amber accent), clean typography, responsive. No purple. No AI emoji clusters. Reference aesthetic: Loom 2.0 — calm, professional, confident.

**Code quality**: Idiomatic Go, typed React/TypeScript, no TODO comments in shipped code, no placeholder values. Lint clean.

**Error handling**: All errors logged with context. User-facing errors are friendly and actionable. No panics in production code paths.

**Taste words**: calm, capable, considered, clean.

**Escape hatch**: If a feature is genuinely impossible to ship polished in a reasonable pass (e.g., some edge case of the Plex API), ship it disabled with a clear "coming soon" placeholder rather than shipping it broken.

---

## Tech Stack

| Layer | Choice | Rationale |
|-------|--------|-----------|
| Backend language | Go 1.22+ | Single static binary, trivial cross-compilation to linux/amd64 + darwin/arm64, zero runtime deps, strong stdlib for HTTP + subprocess |
| Frontend | React 18 + Vite 5 + TypeScript | Polished UI requirements demand a real component framework; TypeScript catches API contract bugs |
| Styling | Tailwind CSS v3 + shadcn/ui | Utility-first lets us own the sandstone palette; shadcn/ui gives accessible components without fighting opinionated defaults |
| Database | SQLite via modernc.org/sqlite | Embedded, zero-config, no separate process, single-writer — perfect for this use case |
| Config | TOML via BurntSushi/toml | Human-writable, no YAML indentation footguns |
| Transcoding | ffmpeg + ffprobe (system binaries) | Industry standard, supports all hardware encoders via flag variants |
| Process management | systemd (Linux) / launchd (macOS) | Native OS daemons, no Docker ever |
| Auth | JWT (HS256) + bcrypt | Simple, stateless, optional — only needed if user sets a password |
| Build | GNU Make + GitHub Actions | Straightforward; Actions cross-compiles all targets on tag push |

**Hard constraints**:
- No Docker. Not even as an optional install method.
- No Python, PHP, Java, Electron.
- No cloud dependencies at runtime.

---

## Database Schema

```sql
-- sqzarr.db
-- SQLite, WAL mode, foreign keys on

PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

-- Watched directories with per-directory transcoding rules
CREATE TABLE IF NOT EXISTS directories (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    path          TEXT    NOT NULL UNIQUE,
    enabled       BOOLEAN NOT NULL DEFAULT 1,
    min_age_days  INTEGER NOT NULL DEFAULT 7,
    max_bitrate   INTEGER NOT NULL DEFAULT 4000000,   -- bits/sec
    min_size_mb   INTEGER NOT NULL DEFAULT 500,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Transcode job queue and history
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

-- Quarantine records
CREATE TABLE IF NOT EXISTS quarantine (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id          INTEGER NOT NULL REFERENCES jobs(id),
    original_path   TEXT    NOT NULL,
    quarantine_path TEXT    NOT NULL,
    expires_at      DATETIME NOT NULL,
    deleted_at      DATETIME,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Scan run audit log
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

-- Singleton stats row for dashboard
CREATE TABLE IF NOT EXISTS stats (
    id                INTEGER PRIMARY KEY CHECK (id = 1),
    total_bytes_saved INTEGER NOT NULL DEFAULT 0,
    total_jobs_done   INTEGER NOT NULL DEFAULT 0,
    total_jobs_failed INTEGER NOT NULL DEFAULT 0,
    updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO stats (id) VALUES (1);
```

---

## Env Vars

SQZARR does not use environment variables for runtime config — it uses a TOML config file. The `.env`-style values below map to config file sections. Copy to `sqzarr.toml` and fill in values.

```toml
# sqzarr.toml

[server]
host = "127.0.0.1"   # change to 0.0.0.0 for Tailscale/LAN access
port = 8080
data_dir = "/var/lib/sqzarr"   # where sqzarr.db lives

[scanner]
interval_hours = 6
worker_concurrency = 1   # increase for multi-stream hardware (max 8)

[transcoder]
# temp_dir: where in-progress output files are written (default: same dir as source)
# temp_dir = "/tmp/sqzarr"

[safety]
quarantine_enabled = true
quarantine_retention_days = 10
quarantine_dir = ""   # empty = .sqzarr-quarantine/ adjacent to source
disk_free_pause_gb = 50

[plex]
enabled = false
base_url = ""   # e.g. http://192.168.1.10:32400
token = ""      # Plex auth token

[auth]
# Leave password_hash empty to disable authentication
# To set: run `sqzarr hash-password` and paste the output here
password_hash = ""
jwt_secret = ""   # auto-generated if empty (changes on restart = logs out users)
```

---

## Phase 0 — Prerequisites

- [ ] Verify Intel VAAPI passthrough in Proxmox LXC: `vainfo` exits 0 and lists `hevc_vaapi`
- [ ] Run manual ffmpeg VAAPI encode on a test file; confirm output plays and is smaller
- [ ] Research Plex library refresh endpoint — confirm `GET /library/sections/{id}/refresh` with `X-Plex-Token` works on current Plex version; document exact request
- [ ] Confirm `modernc.org/sqlite` compiles for `linux/amd64` and `darwin/arm64` without CGo
- [ ] Create GitHub repo `danrichardson/sqzarr` with `main` branch protection, PR template, issue labels (bug, enhancement, wontfix)
- [ ] Scaffold `.github/workflows/build.yml` — lint + test on every push to main; confirm it passes on empty project
- [ ] Decide on quarantine directory naming: `.sqzarr-quarantine` adjacent to source vs. single configurable global path. **Assumption**: global configurable path, default: `{data_dir}/quarantine/`. Flag for owner review.

---

## Phase 1 — Core Daemon

**Goal**: Binary that scans a directory, finds one qualifying file, transcodes it, verifies output, replaces original. No UI.

### Steps

1. Go module init: `github.com/danrichardson/sqzarr`. Directory structure:
   ```
   cmd/sqzarr/main.go
   internal/config/       TOML loader + validation
   internal/db/           SQLite schema migration + query layer
   internal/scanner/      Directory walk + rules evaluation
   internal/transcoder/   Hardware detection + ffmpeg invocation
   internal/verifier/     ffprobe output verification
   internal/queue/        Job queue worker
   internal/logger/       Structured JSON logging
   ```
2. Config loader: parse `sqzarr.toml`, validate required fields, fail fast with clear error if invalid
3. SQLite setup: run migrations on startup, WAL mode, foreign keys
4. Scanner: recursive walk, `ffprobe` each video file for codec/duration/bitrate, evaluate rules, INSERT pending jobs (skip if already in jobs table with status != failed)
5. Hardware detector: probe once at startup, expose result via `transcoder.Encoder` interface
6. Transcoder: spawn ffmpeg with appropriate flags; parse stderr for progress (`frame=`, `time=`); update `jobs.progress` in DB
7. Verifier: ffprobe output file, compare duration ±1s, confirm `output_size < source_size`
8. Replace: `os.Rename` (atomic on same filesystem); log result
9. Worker loop: select next pending job by priority + created_at; run transcode; update stats
10. CLI: `sqzarr serve`, `sqzarr scan-once`, `sqzarr hash-password`

### Integration Tests

- `TestScanFindsH264File`: scanner finds a test H.264 file, rules match, job created in DB
- `TestScanSkipsHEVC`: scanner skips a test HEVC file, no job created
- `TestScanSkipsNewFile`: scanner skips a file with mtime < min_age_days
- `TestTranscodeAndVerify`: encode a small test clip, verify output passes size + duration check
- `TestReplaceOriginal`: after verification, source file is replaced by output

### Definition of Done

- `sqzarr serve` starts, scans configured directory, transcodes one qualifying file, replaces it
- `sqzarr.db` contains a completed job with `status = 'done'` and non-null `bytes_saved`
- All integration tests pass
- No panics on invalid config or missing ffmpeg

---

## Phase 2 — File Safety

**Goal**: Quarantine folder, disk space guard, restore capability.

### Steps

1. Quarantine: after successful replace, move original to `{quarantine_dir}/{job_id}/{filename}.orig`; INSERT quarantine record with `expires_at = NOW() + retention_days`
2. Quarantine GC: background goroutine, runs every hour, deletes originals past `expires_at`
3. Disk space guard: check `syscall.Statfs` on temp dir before starting each job; if free < threshold, mark queue paused, log warning
4. Restore: `sqzarr restore {job_id}` — moves original back from quarantine, deletes transcoded file, marks job as cancelled
5. Add `--dry-run` flag to `sqzarr scan-once` for safe testing

### Integration Tests

- `TestQuarantineCreated`: after job completes, quarantine record exists and file is at quarantine path
- `TestQuarantineGC`: quarantine record with past `expires_at` gets deleted by GC
- `TestDiskGuardPausesQueue`: mock low disk, confirm next job is not started
- `TestRestoreFromQuarantine`: restore moves file back, transcoded output deleted

### Definition of Done

- Transcode a file; confirm original is in quarantine dir
- Manually expire the quarantine record; confirm GC deletes it on next tick
- Set `disk_free_pause_gb` to a high value; confirm queue pauses

---

## Phase 3 — HTTP API

**Goal**: Full REST API + WebSocket. No UI yet — testable with curl.

### Steps

1. HTTP server: `net/http` with `gorilla/mux` or stdlib `ServeMux`; bind to configured `host:port`
2. Implement all REST endpoints (see architecture doc API surface)
3. WebSocket: `/api/v1/ws`; broadcast job progress, scan events, disk warnings to all connected clients
4. Optional JWT auth: `POST /api/v1/auth/login` → JWT; middleware checks `Authorization: Bearer` header on protected routes
5. Path traversal prevention: all file paths in request bodies validated against configured directory allowlist
6. Rate limiting: 5 auth attempts per minute per IP (in-memory counter, no Redis)
7. Embed `frontend/dist` via `//go:embed` (placeholder for now — empty dir)

### Integration Tests

- `TestGetStatus`: returns 200 with correct status shape
- `TestJobsEndpoint`: creates a job via API, confirmed in DB, returned by GET /jobs
- `TestCancelJob`: cancels a pending job, status updated in DB
- `TestPathTraversalBlocked`: `POST /jobs` with `../etc/passwd` returns 400
- `TestAuthRequired`: protected endpoint returns 401 without token
- `TestAuthLogin`: correct password returns JWT; incorrect returns 401

### Definition of Done

- All endpoints return correct data verified by curl
- WebSocket delivers progress events during a live transcode
- Auth works end-to-end: login → token → protected call

---

## Phase 4 — React Admin Panel

**Goal**: Full polished UI covering all screens. Embedded in binary. Works on desktop and mobile.

### Steps

1. Vite project in `frontend/`; React 18, TypeScript, Tailwind v3, shadcn/ui
2. Configure sandstone palette in `tailwind.config.ts`
3. OpenAPI types generated from API (or hand-written TypeScript interfaces matching API responses)
4. Implement screens (in order):
   a. Dashboard — status cards, current job progress (WebSocket), disk space bar, Scan Now
   b. Queue — running job card, pending list, cancel, manual enqueue
   c. History — paginated table, expandable error rows, retry/skip
   d. Directories — list, add, edit drawer with rules, preview
   e. Settings — hardware info, scan schedule, quarantine config, Plex config, auth
   f. Login — only rendered if auth is configured
5. Responsive layout: sidebar nav on desktop, bottom tab bar on mobile (< 768px)
6. Real-time updates: WebSocket connection with reconnect, update job progress and status without full reload
7. `make frontend` builds SPA to `frontend/dist/`; `go:embed` includes it in binary
8. `VITE_API_BASE` env var for dev proxy to Go backend

### Definition of Done

- Complete user flow end-to-end in browser: add directory → preview → scan → watch transcode → see history
- Works on Chrome desktop and iOS Safari at 390px width
- No TypeScript errors, no console errors in production build
- Tailwind palette matches sandstone spec (warm stone, amber accent, no purple)

---

## Phase 5 — Plex Integration + macOS

**Goal**: Optional Plex rescan after replacement. launchd support. Tested on M4 Mac Mini.

### Steps

1. Plex notifier: `GET /library/sections` to discover sections; match replacement file path to section root; `GET /library/sections/{id}/refresh?X-Plex-Token={token}`; log result (non-fatal on failure)
2. Settings UI: test connection button; show detected section names
3. launchd plist: `scripts/com.sqzarr.agent.plist`
4. macOS install script: `scripts/install-macos.sh` — copies binary to `/usr/local/bin`, config to `~/.config/sqzarr/`, installs plist
5. Cross-compile and test `darwin/arm64` build on M4 Mac Mini
6. Test VideoToolbox hardware detection and encode

### Definition of Done

- Transcode a file on Mac Mini using VideoToolbox; Plex library refreshes automatically
- `install-macos.sh` runs cleanly on fresh macOS install
- launchd keeps service running across reboots

---

## Phase 6 — Polish + Pre-Release

**Goal**: Security-reviewed, documented, dogfooded, ready for GitHub.

### Steps

1. Run `gosec ./...`; address all HIGH and MEDIUM findings
2. Run `npm audit`; resolve all critical and high vulnerabilities
3. Manual review: all file path handling, ffmpeg subprocess invocation, JWT implementation
4. Confirm no secrets logged at any level
5. Check config file permissions enforcement (warn if world-readable)
6. Write `README.md`: what it is, requirements, Proxmox LXC setup, macOS setup, config reference, FAQ
7. GitHub Actions: `release.yml` — on `v*` tag push, build all targets, create GitHub Release with binaries
8. Smoke test script: `scripts/smoke-test.sh` — starts daemon, adds a test directory, triggers scan, waits for one completed job, confirms DB state, exits 0
9. Dogfood on personal library for 1 full week; address any issues found

### Definition of Done

- `gosec` and `npm audit` clean (or all findings documented as accepted with rationale)
- README complete and reviewed
- Smoke test passes in CI
- 1 week dogfood with no data loss or unhandled errors
- Tag `v1.0.0` pushed; GitHub Release contains all platform binaries

---

## Integration Test Setup

```go
// internal/testutil/testutil.go
package testutil

import (
    "os"
    "testing"
    "github.com/danrichardson/sqzarr/internal/db"
)

// NewTestDB creates an in-memory SQLite DB with schema applied.
func NewTestDB(t *testing.T) *db.DB {
    t.Helper()
    database, err := db.Open(":memory:")
    if err != nil {
        t.Fatalf("testutil: open db: %v", err)
    }
    t.Cleanup(func() { database.Close() })
    return database
}

// TestMediaDir creates a temp directory with sample video files for scanner tests.
// Requires ffmpeg to be on PATH.
func TestMediaDir(t *testing.T) string {
    t.Helper()
    dir := t.TempDir()
    // Create a 30-second H.264 test clip via ffmpeg
    // ffmpeg -f lavfi -i testsrc=duration=30:size=1280x720:rate=24 -c:v libx264 {dir}/test.mkv
    return dir
}
```

**Run tests**:
```bash
go test ./...
```

**Integration tests** (require ffmpeg on PATH):
```bash
go test ./... -tags integration
```

---

## CI/CD Setup

### `.github/workflows/ci.yml`
```yaml
name: CI
on:
  push:
    branches: [main]
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - name: Install ffmpeg
        run: sudo apt-get install -y ffmpeg
      - name: Go lint
        uses: golangci/golangci-lint-action@v4
      - name: Go test
        run: go test ./...
      - uses: actions/setup-node@v4
        with: { node-version: '20' }
      - name: Frontend lint + build
        run: cd frontend && npm ci && npm run lint && npm run build

  security:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - name: gosec
        uses: securego/gosec@master
        with: { args: './...' }
```

### `.github/workflows/release.yml`
```yaml
name: Release
on:
  push:
    tags: ['v*']

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - uses: actions/setup-node@v4
        with: { node-version: '20' }
      - name: Build frontend
        run: cd frontend && npm ci && npm run build
      - name: Build all targets
        run: make release
      - name: Create GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          files: dist/*
```

---

## Resume Prompt

```
You are continuing work on SQZARR, a self-hosted media transcoding service.

Project: github.com/danrichardson/sqzarr
Stack: Go 1.22 backend, React 18 + TypeScript frontend, SQLite, no Docker ever.
WORKFLOW.md is the source of truth — check the current phase and its Definition of Done.

Key constraints:
- No Docker, not even as an option
- File safety is priority #1 — never corrupt or lose originals
- Sandstone UI palette, clean and professional (see ui-mockups.md)
- No AI attribution in commits

To start: read WORKFLOW.md, identify the current phase, check its Definition of Done, and continue from where work left off. If you're unsure what's been done, run `go test ./...` and check git log.

Questions to ask before starting a new phase:
1. What does `git log --oneline -20` show?
2. What does `go test ./...` produce?
3. Is the previous phase's Definition of Done fully satisfied?
```

---

## Decision Log

| # | Decision | Rationale | Flag for review? |
|---|----------|-----------|-----------------|
| 1 | Go backend, not Python/Node | Static binary simplicity + cross-platform build; no runtime deps | No |
| 2 | modernc.org/sqlite (pure Go) over mattn/go-sqlite3 (CGo) | Simpler cross-compilation to darwin/arm64; no CGo toolchain required | Flag: verify performance is acceptable for the query patterns SQZARR uses |
| 3 | SQLite over PostgreSQL/MySQL | Single-instance app, no network DB, embedded is the right call | No |
| 4 | Quarantine dir defaults to global `{data_dir}/quarantine/` not `.sqzarr-quarantine` adjacent to source | Cleaner, avoids polluting media directories, easier to configure a separate fast disk | Flag: owner may prefer adjacent-to-source for clarity |
| 5 | No Docker install path | Owner was emphatic; we don't even document it as an option | No |
| 6 | ffmpeg is a system dependency (not vendored/bundled) | Bundling ffmpeg is complex and defeats the purpose of using the user's GPU drivers; system ffmpeg is always more up-to-date | No |
| 7 | JWT with 30-day expiry, no refresh tokens | Simple, stateless, single-user; re-login every month is acceptable | No |
| 8 | HTTP server binds `127.0.0.1` by default | Security — don't expose to LAN without explicit opt-in | No |
| 9 | ffprobe duration match ±1s for verification | Cheap, reliable check; full frame decode would be too slow for large files | Flag: should tolerance be configurable? |
| 10 | `gorilla/mux` for routing (assumption) | Clean route pattern matching, well-maintained; alternatively stdlib 1.22 pattern routing could work | Flag: stdlib routing in Go 1.22 is now capable enough; may switch to avoid the dependency |
| 11 | No real-time file watching (inotify) | Owner confirmed weekly-old files are fine; polling is simpler and more portable (works on NFS mounts) | No |
| 12 | Plex rescan on failure: log and continue, don't fail the job | Plex notification is strictly optional; job success/failure should not depend on it | No |
