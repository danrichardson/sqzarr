# SQZARR Fix Brief
**Generated:** 2026-04-03  
**Purpose:** Complete spec for a new agent to fix the five deficiencies identified in `deficiency-report.md`  
**Source site:** http://192.168.29.211:8080/  
**Grade received:** Solid implementation with 5 gaps. Backend is excellent. Gaps are all fixable without architectural changes.

---

## What Is Working — Do Not Touch

- Stone/sandstone palette is correct. Do not change colors.
- SSE live updates on Dashboard/Queue work correctly.
- Hardware encoder detection (VAAPI → VideoToolbox → NVENC → software) is correct.
- Directory CRUD with per-directory criteria is solid.
- Verifier (size + duration check before replacement) is correct.
- Plex rescan after replacement is correct.
- JWT auth with bcrypt is correct.
- History page with retry/error details is good UX.
- All backend logic in `internal/queue/worker.go` is correct — quarantine, disk guard, Plex notify.

---

## Deficiency 1 — Codec Skip Logic Is Wrong; Default Bitrate Threshold Is Too High
**Severity: High**  
**File:** `internal/scanner/scanner.go`, `internal/db/db.go`

### Root cause

The deficiency report got this backwards. The codec skip check is wrong because **codec doesn't matter — bitrate/size does**. The founder's actual statement:

> *"I don't want to store anything that's greater than like a gigabyte an hour for a stupid TV show"*

A 5GB HEVC file for a 1-hour show is oversized and **should** be transcoded. Skipping it because it's already HEVC defeats the entire purpose of the app.

The "skip HEVC/AV1" assumption was introduced by the interviewer and never confirmed by the founder. It was the interviewer's assumption, not a requirement.

### What to fix

#### 1A — Remove the codec-based skip entirely

**File:** `internal/scanner/scanner.go`, function `maybeEnqueue()`

```go
// REMOVE THIS BLOCK ENTIRELY:
if strings.EqualFold(probe.codec, "hevc") {
    return false, "already hevc", nil
}
```

The correct skip criteria is already implemented right after this block (bitrate check). The codec check just short-circuits it incorrectly.

After removing the codec check, `maybeEnqueue()` correctly:
1. Skips if file age < `min_age_days` ✓
2. Skips if file size < `min_size_mb` ✓  
3. Skips if bitrate ≤ `max_bitrate` ✓ ← this is the real gate
4. Otherwise enqueues ✓

#### 1B — Fix the default `max_bitrate`

**File:** `internal/db/db.go` (schema) and `sqzarr.toml.example`

Current default: `max_bitrate = 4000000` bits/sec = **4 Mbps** = ~1.8 GB/hour

Founder said: "1 gigabyte per hour" = **2,222,000 bits/sec** ≈ 2.2 Mbps

Fix the schema default:
```sql
-- internal/db/db.go, in the CREATE TABLE statement:
max_bitrate   INTEGER NOT NULL DEFAULT 2222000,   -- bits/sec (~1 GB/hr)
```

Fix `sqzarr.toml.example`:
```toml
[directories]
# Files with average bitrate above this will be transcoded.
# Default: 2,222,000 bits/sec ≈ 1 GB per hour of video.
# Example: a 5GB file for a 1-hour TV show is ~11 Mbps — well above threshold.
default_max_bitrate = 2222000
```

#### 1C — Update the scanner test

**File:** `internal/scanner/scanner_test.go`

Remove any `TestScannerSkipsHEVC` test (or rename it to verify HEVC files ARE enqueued when over-bitrate). Add:

```
TestScannerEnqueuesOversizedHEVC  — HEVC file at 10 Mbps should be enqueued
TestScannerSkipsUndersizedHEVC    — HEVC file at 1 Mbps should be skipped (below threshold)
TestScannerEnqueuesOversizedAV1   — AV1 file at 5 Mbps should be enqueued
```

---

## Deficiency 2 — No Per-File Manual Transcode in the UI
**Severity: High**  
**File:** `frontend/src/pages/Queue.tsx`

### What exists already

The backend is complete:
- `POST /api/v1/jobs` accepts `{ "path": "/absolute/path/to/file.mkv" }`, inserts job with `priority=1`
- `api.createJob(path: string)` is already in `frontend/src/lib/api.ts`

### What to add

Add a "Queue a file" form at the **top of the Queue page**, above the running jobs section.

**Component spec:**

```tsx
// At the top of Queue.tsx, before the running/pending job lists:

const [manualPath, setManualPath] = useState('')
const [submitting, setSubmitting] = useState(false)
const [submitError, setSubmitError] = useState<string | null>(null)
const [submitSuccess, setSubmitSuccess] = useState(false)

const handleManualSubmit = async (e: React.FormEvent) => {
  e.preventDefault()
  if (!manualPath.trim()) return
  setSubmitting(true)
  setSubmitError(null)
  setSubmitSuccess(false)
  try {
    await api.createJob(manualPath.trim())
    setManualPath('')
    setSubmitSuccess(true)
    setTimeout(() => setSubmitSuccess(false), 3000)
    // reload pending jobs list
  } catch (err: any) {
    setSubmitError(err.message || 'Failed to queue file')
  } finally {
    setSubmitting(false)
  }
}
```

**UI layout:**

```
┌─────────────────────────────────────────────┐
│ Queue a file                                  │
│ ┌───────────────────────────────────┐ [Queue]│
│ │ /absolute/path/to/file.mkv        │        │
│ └───────────────────────────────────┘        │
│ ✓ Added to queue   OR   ✗ Error message here  │
└─────────────────────────────────────────────┘
```

The text input should:
- Accept a full absolute filesystem path
- Show inline error beneath input (red, small) on failure — exact server error message
- Show brief success confirmation (green checkmark + "Added to queue") that fades after 3s on success
- Clear the input on success
- Disable button while submitting

Error cases from the API:
- `400` body missing path → "Path is required"
- `400` path outside configured directory → "Path is not in a configured directory"  
- `409` already queued → "A job for this file already exists"

---

## Deficiency 3 — Quarantine Has No UI Management
**Severity: Medium**

### What exists already

Backend is complete:
- `internal/db/quarantine.go` has `InsertQuarantine`, `ExpiredQuarantines`, `MarkQuarantineDeleted`
- `QuarantineRecord` struct: `{ID, JobID, OriginalPath, QuarantinePath, ExpiresAt, DeletedAt, CreatedAt}`
- GC in `internal/queue/quarantine_gc.go` auto-deletes on `ExpiresAt`

### What to add

#### 3A — New API endpoints

Add three endpoints to `internal/api/handlers.go` and register in `internal/api/server.go`:

**GET /api/v1/quarantine** — list active quarantine records

```go
func (s *Server) handleListQuarantine(w http.ResponseWriter, r *http.Request) {
    // Query: WHERE deleted_at IS NULL ORDER BY expires_at ASC
    records, err := s.db.ActiveQuarantines()
    // Return []QuarantineRecord with DaysRemaining computed
}
```

Add `ActiveQuarantines()` to `internal/db/quarantine.go`:
```go
func (d *DB) ActiveQuarantines() ([]*QuarantineRecord, error) {
    rows, err := d.db.QueryContext(ctx,
        `SELECT id, job_id, original_path, quarantine_path, expires_at, created_at 
         FROM quarantine WHERE deleted_at IS NULL ORDER BY expires_at ASC`)
    // ...
}
```

**DELETE /api/v1/quarantine/{id}** — confirm and delete early (purge original now)

```go
func (s *Server) handleDeleteQuarantine(w http.ResponseWriter, r *http.Request) {
    id := getIDParam(r)
    record, err := s.db.GetQuarantine(id)
    // Delete the file at record.QuarantinePath
    if err := os.Remove(record.QuarantinePath); err != nil && !os.IsNotExist(err) {
        // handle error
    }
    s.db.MarkQuarantineDeleted(id)
    w.WriteHeader(http.StatusNoContent)
}
```

**POST /api/v1/quarantine/{id}/release** — restore original (reject the transcode)

```go
func (s *Server) handleReleaseQuarantine(w http.ResponseWriter, r *http.Request) {
    id := getIDParam(r)
    record, err := s.db.GetQuarantine(id)
    // Move quarantine file BACK to original path (overwrite the transcode)
    if err := os.Rename(record.QuarantinePath, record.OriginalPath); err != nil {
        // handle error
    }
    s.db.MarkQuarantineDeleted(id)
    // Update the job status to show it was released
    w.WriteHeader(http.StatusNoContent)
}
```

Add `GetQuarantine(id int64)` to `internal/db/quarantine.go`.

Register routes in `server.go`:
```go
r.Get("/api/v1/quarantine", auth(s.handleListQuarantine))
r.Delete("/api/v1/quarantine/{id}", auth(s.handleDeleteQuarantine))
r.Post("/api/v1/quarantine/{id}/release", auth(s.handleReleaseQuarantine))
```

#### 3B — Frontend type in api.ts

```typescript
export interface QuarantineRecord {
  ID: number
  JobID: number
  OriginalPath: string
  QuarantinePath: string
  ExpiresAt: string
  CreatedAt: string
  DaysRemaining: number  // computed server-side
}
```

Add to api object:
```typescript
listQuarantine: () => request<QuarantineRecord[]>('GET', '/quarantine'),
deleteQuarantine: (id: number) => request<void>('DELETE', `/quarantine/${id}`),
releaseQuarantine: (id: number) => request<void>('POST', `/quarantine/${id}/release`),
```

#### 3C — New Quarantine.tsx page

**File to create:** `frontend/src/pages/Quarantine.tsx`

```
┌─────────────────────────────────────────────────────────┐
│ Quarantine                                               │
│ 3 originals held · auto-deletes when timer expires       │
├──────────────────────────┬───────┬───────────────────────┤
│ File                     │ Size  │ Expires    │ Actions   │
├──────────────────────────┼───────┼────────────┼───────────┤
│ Show.S01E01.mkv          │ 8.2GB │ 7 days     │ [Release] │
│ /media/TV/...            │       │            │ [Purge]   │
├──────────────────────────┼───────┼────────────┼───────────┤
│ Movie.2024.mkv           │ 22GB  │ 2 days     │ [Release] │
│ /media/Movies/...        │       │            │ [Purge]   │
└──────────────────────────┴───────┴────────────┴───────────┘
```

Each row shows:
- Filename (basename, truncated) + full path below in smaller text
- Original file size (formatted: GB/MB)
- Expires: "N days" or "today" if < 1 day
- **Release** button: restores original, deletes transcode. Confirm dialog: "This will restore the original file and discard the transcoded version. Are you sure?"
- **Purge** button: deletes original early. Confirm dialog: "This will permanently delete the original. The transcoded version stays. Are you sure?"

Empty state: "No files in quarantine" centered.

If `quarantine_enabled = false` in config: "Quarantine is disabled. Enable it in sqzarr.toml to use this feature."

Check if quarantine is enabled via `GET /api/v1/status` — add `QuarantineEnabled bool` to the status response.

#### 3D — Wire into navigation

Add "Quarantine" to the sidebar nav in `App.tsx`. Show a badge with count if `records.length > 0`. Route: `/quarantine`.

---

## Deficiency 4 — Settings Page Is Read-Only
**Severity: Medium**

### What to add

#### 4A — Runtime config API

Add `GET /api/v1/config` and `PUT /api/v1/config` to expose runtime-editable settings.

**GET /api/v1/config** — returns current values of UI-editable settings only:

```go
type RuntimeConfig struct {
    WorkerConcurrency       int    `json:"worker_concurrency"`
    ScanIntervalHours       int    `json:"scan_interval_hours"`
    QuarantineEnabled       bool   `json:"quarantine_enabled"`
    QuarantineRetentionDays int    `json:"quarantine_retention_days"`
    PlexEnabled             bool   `json:"plex_enabled"`
    PlexBaseURL             string `json:"plex_base_url"`
    PlexToken               string `json:"plex_token"`  // masked: return "" if set, "SET" indicator
}
```

**PUT /api/v1/config** — accepts partial JSON, updates in-memory config and writes to `sqzarr.toml`.

Important: `password_hash` and `jwt_secret` are **NOT** included. Those remain CLI-only.

For writing back to `sqzarr.toml`, use a simple template approach — read the existing file, update the specific lines, write it back. Or keep an in-memory `RuntimeConfig` and apply it on restart (simpler, safer). Add a banner: "Changes require service restart to take effect for: scan interval, worker concurrency."

Concurrency changes CAN be applied live (just update the worker's semaphore). Implement live update for concurrency if straightforward; otherwise note "restart required" in UI.

#### 4B — Update Settings.tsx

Replace the read-only info display with an editable form:

```
┌─────────────────────────────────────────────┐
│ Transcoding                                   │
│ Active encoder: hevc_vaapi           [auto]  │
│ Worker concurrency:  [1 ▼] (1–8)             │
├─────────────────────────────────────────────┤
│ Scanning                                      │
│ Scan interval:  [6 ▼] hours                  │
│ Next scan:      in 3h 12m                     │
│                          [Scan Now]          │
├─────────────────────────────────────────────┤
│ Safety                                        │
│ Quarantine:     [enabled toggle]             │
│ Retention:      [10] days                    │
├─────────────────────────────────────────────┤
│ Plex                                          │
│ Plex enabled:   [enabled toggle]             │
│ Plex URL:       [http://192.168.1.10:32400]  │
│ Plex token:     [●●●●●●●● SET]              │
├─────────────────────────────────────────────┤
│                             [Save Settings]  │
└─────────────────────────────────────────────┘
```

Notes:
- Concurrency: number input, min=1, max=8 — applied live (no restart needed)
- Scan interval: select or number input (hours)
- Quarantine enabled: toggle switch
- Retention: number input (days), min=1
- Plex token: password input; if already set, show "●●●●●●●● (set)" placeholder; only send to server if user types a new value
- Save button calls `PUT /api/v1/config`
- Show success toast on save; show error if save fails

#### 4C — Add "Next scan in X" to Dashboard

The Dashboard already shows "Scan Now" button. Add next scan timing below it.

The server needs to expose this. Add `NextScanAt time.Time` and `LastScanAt *time.Time` to the status endpoint response (`GET /api/v1/status`). The scanner tracks these internally — expose them.

Dashboard display:
```
[Scan Now]   Last scan: 2h ago · Next: in 3h 45m
```

---

## Deficiency 5 — No Scan Schedule UI
**Severity: Low-Medium**  
**Largely covered by DEF-4 implementation above.**

Additional items:

1. **Next scan indicator on Dashboard** — covered in DEF-4C above
2. **Scan interval in Settings** — covered in DEF-4B above
3. **Last scan summary** — add to Dashboard: most recent scan run (files scanned / files queued)

For the last scan summary, query `GET /api/v1/scan/last` — add this endpoint to return the most recent `scan_runs` row:

```go
// GET /api/v1/scan/last
type LastScanResult struct {
    DirectoryID   int64     `json:"directory_id"`
    FilesScanned  int       `json:"files_scanned"`
    FilesQueued   int       `json:"files_queued"`
    FilesSkipped  int       `json:"files_skipped"`
    DurationMs    int64     `json:"duration_ms"`
    StartedAt     time.Time `json:"started_at"`
}
```

---

## Deficiency 6 — No Directory Browser / Filesystem Picker
**Severity: Medium**  
**File:** `internal/api/handlers.go`, `internal/api/server.go`, `frontend/src/pages/Directories.tsx`

### What to add

Users currently must type absolute paths like `/volume1/Media/TV/` manually. For a NAS app this is a significant friction point.

#### 6A — Filesystem browse API

Add `GET /api/v1/fs?path=/` endpoint:

```go
func (s *Server) handleBrowseFS(w http.ResponseWriter, r *http.Request) {
    reqPath := r.URL.Query().Get("path")
    if reqPath == "" {
        reqPath = "/"
    }
    // Security: resolve to absolute, don't allow symlink traversal outside fs
    abs, err := filepath.Abs(reqPath)
    
    entries, err := os.ReadDir(abs)
    var dirs []string
    for _, e := range entries {
        if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
            dirs = append(dirs, filepath.Join(abs, e.Name()))
        }
    }
    json.NewEncoder(w).Encode(map[string]any{
        "current": abs,
        "parent":  filepath.Dir(abs),
        "dirs":    dirs,
    })
}
```

Register: `r.Get("/api/v1/fs", auth(s.handleBrowseFS))`

#### 6B — Directory picker modal in frontend

On the Directories page, when user clicks "Add Directory" (or the path input), show a modal:

```
┌─────────────────────────────────────────┐
│ Browse for directory               [✕]  │
│ /volume1/Media                          │
├─────────────────────────────────────────┤
│ 📁 Movies                               │
│ 📁 TV                                   │
│ 📁 Documentaries                        │
│ 📁 Home Videos                          │
├─────────────────────────────────────────┤
│ [← Parent]           [Select This Folder]│
└─────────────────────────────────────────┘
```

- Opens at `/` or the last-used path
- Clicking a folder navigates into it
- "← Parent" goes up one level
- Breadcrumb shows current path
- "Select This Folder" closes modal and populates the path input
- User can also still type the path manually

Add to `frontend/src/lib/api.ts`:
```typescript
browseFS: (path: string) => request<{ current: string; parent: string; dirs: string[] }>('GET', `/fs?path=${encodeURIComponent(path)}`),
```

---

## Deficiency 7 — Disk Pause Threshold Was Never Requested; Feature Is Counterproductive
**Severity: Medium**  
**File:** `internal/queue/worker.go`, `sqzarr.toml.example`, `frontend/src/pages/Settings.tsx`

### Root cause

`sqzarr.toml.example` has `disk_free_pause_gb = 50` — this value was invented by the implementation agent. The founder never mentioned 50 GB or any percentage. The interviewer asked "how should the service behave when free space drops below a threshold?" and the founder said "yeah we got to watch the temp file thing" then moved on to queue concurrency — no threshold was given.

Worse, the feature is counterproductive. The founder's whole goal is to compress bloated files **to save space**. If the drive is getting full, that's exactly when SQZARR should be transcoding hardest, not stopping.

### What to fix

#### 7A — Remove the blanket disk-pause feature

Remove `disk_free_pause_gb` from:
- `sqzarr.toml.example` (delete the line)
- The config struct in `internal/config/config.go`
- Any code in `internal/queue/worker.go` that pauses the queue based on this setting

#### 7B — Replace with per-job temp space check

Before starting each individual transcode job, check if there's enough temp space for that specific file:

```go
// In worker.go, before starting a transcode:
func hasTempSpace(sourcePath string) (bool, error) {
    info, err := os.Stat(sourcePath)
    if err != nil {
        return false, err
    }
    needed := uint64(float64(info.Size()) * 1.2)  // 20% headroom

    // Get free space on the partition where temp files are written
    tempDir := filepath.Dir(sourcePath)  // or config.TempDir if set
    var stat syscall.Statfs_t
    if err := syscall.Statfs(tempDir, &stat); err != nil {
        return false, err
    }
    free := stat.Bavail * uint64(stat.Bsize)
    return free >= needed, nil
}
```

If `hasTempSpace` returns false, skip the job and log:
```
skipping [file]: insufficient temp space (need 8.5 GB, have 3.2 GB free)
```

This protects against disk-full crashes without ever pausing the queue.

#### 7C — Remove from Definition of Done checklist

Remove the disk-pause verification step from the DoD checklist at the bottom of this brief.

---

## Implementation Order

Do these in order — each is independent except DEF-4 depends slightly on DEF-5 (they share the Settings page).

1. **DEF-1** — Remove codec skip, fix default bitrate to 2.2 Mbps (30 min, backend only, zero risk)
2. **DEF-7** — Remove disk_free_pause_gb, add per-job temp space pre-check (30 min, backend only)
3. **DEF-2** — Manual queue UI (45 min, frontend only, API already exists)
4. **DEF-6** — Filesystem browser API + directory picker modal (2 hours)
5. **DEF-3** — Quarantine UI (2–3 hours: 3 API endpoints + new frontend page)
6. **DEF-4 + DEF-5** — Settings editable + scan schedule (2–3 hours: config API + Settings page rewrite + Dashboard additions)

Total estimated work: ~9–10 hours for a competent agent starting from scratch.

---

## File Change Summary

| File | Change Type | Change |
|------|-------------|--------|
| `internal/scanner/scanner.go` | Edit | Remove HEVC codec skip entirely; bitrate check is the correct gate |
| `internal/db/db.go` | Edit | Change `max_bitrate` default from 4000000 to 2222000 (1 GB/hr) |
| `internal/scanner/scanner_test.go` | Edit | Replace `TestScannerSkipsHEVC` with bitrate-based tests; add HEVC/AV1 over-bitrate enqueue tests |
| `sqzarr.toml.example` | Edit | Update default bitrate comment; remove `disk_free_pause_gb` |
| `internal/config/config.go` | Edit | Remove `DiskFreePauseGB` field from config struct |
| `internal/queue/worker.go` | Edit | Remove blanket disk-pause logic; add `hasTempSpace()` per-job check |
| `internal/api/handlers.go` | Edit | Add `handleBrowseFS`, `handleListQuarantine`, `handleDeleteQuarantine`, `handleReleaseQuarantine`, `handleGetConfig`, `handleUpdateConfig`, `handleLastScan` |
| `internal/api/server.go` | Edit | Register new routes; add `NextScanAt`/`LastScanAt` to status response |
| `internal/db/quarantine.go` | Edit | Add `ActiveQuarantines()`, `GetQuarantine(id)` methods |
| `frontend/src/lib/api.ts` | Edit | Add `QuarantineRecord` type, quarantine API calls, config API calls, browseFS call, lastScan call |
| `frontend/src/pages/Directories.tsx` | Edit | Add directory picker modal using new browseFS API |
| `frontend/src/pages/Queue.tsx` | Edit | Add manual file submit form at top |
| `frontend/src/pages/Settings.tsx` | Edit | Replace read-only display with editable form; load/save via config API; no disk threshold field |
| `frontend/src/pages/Dashboard.tsx` | Edit | Add "Next scan in X · Last scan Y ago" below Scan Now button |
| `frontend/src/pages/Quarantine.tsx` | Create | New page: list + release + purge |
| `frontend/src/App.tsx` | Edit | Add `/quarantine` route + nav link with badge |

---

## Definition of Done

All deficiencies resolved when these pass:

- [ ] Create a directory in SQZARR. Add a test file with `codec=hevc` and bitrate > 2.2 Mbps. Trigger scan. Confirm it DOES appear in Queue (HEVC is no longer skipped by codec).
- [ ] Add a test file with `codec=av1` and bitrate > 2.2 Mbps. Confirm it DOES appear in Queue.
- [ ] Add a test file (any codec) with bitrate < 2.2 Mbps. Confirm it does NOT appear in Queue (bitrate gate still works).
- [ ] Transcode a real oversized file. Verify the temp space check logs "Skipping: insufficient temp space" when a file requires more disk than is available (simulate by pointing temp_dir at a nearly-full partition).
- [ ] Navigate to Queue page. Enter an absolute path to a valid video file in a configured directory. Click "Queue File". Confirm job appears in Pending immediately.
- [ ] Enter a path outside any configured directory. Confirm inline error "Path is not in a configured directory" appears.
- [ ] Navigate to Quarantine page. Confirm it shows files with sizes, days remaining, Release and Purge buttons.
- [ ] Click Release on a quarantined file. Confirm original is restored and file disappears from quarantine list.
- [ ] Click Purge on a quarantined file. Confirm it disappears from list and the original file is gone from disk.
- [ ] Navigate to Settings. Confirm concurrency, scan interval, quarantine settings, and Plex settings are all editable fields. Confirm there is NO "disk pause threshold" field.
- [ ] Change concurrency. Confirm it applies without a restart.
- [ ] Change scan interval to 2 hours. Save. Confirm value persists after page reload.
- [ ] Dashboard shows "Next scan in X · Last scan Y ago" below the Scan Now button.
- [ ] Verify `sqzarr.toml.example` has no `disk_free_pause_gb` line.
