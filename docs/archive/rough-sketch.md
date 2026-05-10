# SQZARR — Architecture Rough Sketch

## Recommended Stack

| Layer | Choice | Rationale |
|-------|--------|-----------|
| Backend language | Go | Single static binary, easy cross-platform build (Linux LXC + macOS ARM64), strong concurrency model for job worker, zero runtime dependencies |
| Frontend framework | React + Vite | Fast dev loop, component ecosystem, straightforward to build a polished UI |
| Styling | Tailwind CSS | Utility-first, easy to enforce sandstone palette without fighting a design system, great for responsive layout |
| Database | SQLite (modernc/go-sqlite3) | Embedded, zero config, no separate process, perfect for single-instance local app |
| Transcoding engine | ffmpeg (system binary) | Industry standard, supports VAAPI/VideoToolbox/NVENC via flags, proven |
| Process manager | systemd (Linux) / launchd (macOS) | Native OS daemons, no Docker, no extra runtime |
| Config format | TOML | Human-writable, no YAML footguns, good Go library support (BurntSushi/toml) |

## Data Flow

```
[File System]
      |
      | (periodic timer, configurable interval)
      v
[Directory Scanner]
      |
      | (walk dirs, stat files)
      v
[Rules Engine]  <-- config: codec excludes, min age, max bitrate, file extensions
      |
      | (qualifying candidates)
      v
[Job Queue (SQLite)] <-- also accepts manual triggers from UI
      |
      | (one worker by default, configurable concurrency)
      v
[Transcoder Worker]
      |  \
      |   [Hardware Detector] --> picks: VAAPI / VideoToolbox / NVENC / software
      |
      | (shells out to ffmpeg)
      v
[Output Verifier]  (ffprobe: duration match ±1s, output < original size)
      |
      | (pass) --> [Atomic Replace] --> [Move original to quarantine]
      | (fail) --> [Delete output, mark job failed, keep original]
      |
      v
[Quarantine GC] -- background timer, deletes originals past retention window
      |
      v
[Plex Notifier] (optional: POST /library/sections/{id}/refresh)
      |
      v
[Job Log (SQLite)]

[React UI] <--HTTP/WS--> [Go HTTP Server] <--> [SQLite]
```

## Major Components

- **Scanner** — walks configured directories recursively, stats each video file, checks against rules engine, enqueues new candidates (deduplicates by path + mtime)
- **Rules Engine** — evaluates per-file: codec (skip HEVC/AV1), bitrate (file_size_bytes / duration_seconds), file age, file size floor; per-directory rule overrides supported
- **Job Queue** — SQLite table, states: pending → running → done/failed/skipped; supports manual priority override from UI
- **Transcoder** — spawns ffmpeg subprocess with appropriate hardware flags; streams stderr for progress parsing; respects disk space threshold before starting
- **Hardware Detector** — probes at startup: tries `vainfo` (VAAPI), checks macOS + Apple Silicon (VideoToolbox), checks `nvidia-smi` (NVENC); stores result, re-probes on demand
- **Verifier** — runs `ffprobe` on output file; checks duration within tolerance, output file exists and is smaller
- **Quarantine Handler** — moves original to `.sqzarr-quarantine/` in same directory (configurable location); SQLite record tracks path and expiry; background timer purges expired entries
- **Disk Space Guard** — checks `syscall.Statfs` before each job; pauses queue if available bytes below threshold
- **Plex Notifier** — HTTP client call to Plex API; optional config; fires after successful replacement
- **HTTP API** — REST + WebSocket; serves React SPA; endpoints for config, queue, history, manual trigger, system status
- **React Admin Panel** — dashboard (live status, space saved), queue view (with pause/cancel), history table, directory/rules config, settings page

## Key Integrations

- **ffmpeg + ffprobe** — must be installed and on PATH; version check at startup
- **Intel VAAPI** — requires `/dev/dri/renderD128` accessible in LXC; detected via `vainfo` binary
- **Apple VideoToolbox** — macOS only; detected by ffmpeg capability probe (`ffmpeg -encoders | grep hevc_videotoolbox`)
- **NVIDIA NVENC** — detected via `nvidia-smi` presence and ffmpeg encoder list
- **Plex Media Server** — optional; requires Plex base URL + auth token in config; calls `GET /library/sections` then refresh endpoint
- **System disk stats** — uses OS `statfs` syscall; no external dependency

## What Can Be Built in a Weekend

- Go project scaffold: module, SQLite setup, config loading from TOML, HTTP server skeleton
- File scanner walking one directory and printing candidates
- ffmpeg invocation with hardware encoder detection and basic progress parsing
- React app with a single dashboard showing hardcoded mock data, wired to `/api/status`
- systemd unit file that starts the binary

## What Takes Longer

- Full rules engine with bitrate calculation (needs ffprobe duration) and per-directory overrides
- Output verification pipeline with ffprobe integration
- Quarantine folder with auto-expiry background worker
- Polished React UI: all screens, responsive layout, real-time WebSocket updates, sandstone design system
- Disk space monitoring and job pause/resume logic
- Plex API integration and library section detection
- launchd plist and install script for macOS
- Security hardening: path traversal prevention, config validation, optional password auth with bcrypt
- Pre-release security review and documentation cleanup for GitHub publish
