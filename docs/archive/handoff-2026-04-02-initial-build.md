# Handoff — Initial Build Session (2026-04-02)

This doc captures the work completed in the session that built SQZARR from spec to running service. The codebase has evolved significantly since — see git log for current state.

---

## Outcome

Built and shipped SQZARR v0 — all 6 phases of WORKFLOW.md completed in one sitting. Service running in production on a fresh Proxmox LXC container. Repo published to GitHub.

**Live service:** CT 260 (pve2), 192.168.29.211:8080
**Repo:** https://github.com/danrichardson/sqzarr

---

## Infrastructure work

- Deleted CT 340 (old Tdarr container)
- Provisioned **CT 260** on pve2 (Debian 13, 2 cores, 4 GB RAM, 20 GB on storePool, DHCP on vmbr0)
- Replicated CT 340's GPU passthrough exactly: `/dev/dri/` bind mount, cgroup2 device allow, full UID/GID idmap (UID 1000 + render/video groups passthrough)
- Bind-mounted `/storePool/subvol-250-disk-0/srv/storage/Videos` → `/media/Videos`
- Installed Go 1.22, Node 20, ffmpeg, vainfo, **`intel-media-va-driver-non-free`** (this was the key — the free Intel driver only provides HEVC decode, the non-free variant adds `VAEntrypointEncSlice`)
- Enabled `non-free` and `non-free-firmware` apt components in Debian 13 sources
- Installed sqzarr binary as systemd service running as `sqzarr` user with `video,render` supplementary groups
- Created GitHub repo via `gh` CLI, set up local git in working directory

---

## Code work — phases delivered

| Phase | Deliverables |
|---|---|
| **1 — Core daemon** | `internal/{config,db,scanner,transcoder,verifier,queue,logger}` packages, `cmd/sqzarr/main.go` with `serve`/`scan-once`/`hash-password` subcommands. SQLite schema with WAL mode, jobs/directories/quarantine/scan_runs/stats tables. Hardware encoder detection (VAAPI/VideoToolbox/NVENC/software fallback). ffmpeg progress streaming via `-progress pipe:2`. ffprobe-based duration verification. |
| **2 — File safety** | `internal/queue/quarantine_gc.go` (background sweep, hourly), `sqzarr restore <job-id>` CLI, `--dry-run` flag for `scan-once`, disk space guard already in worker. |
| **3 — HTTP API** | `internal/api/` — full REST surface using **Go 1.22 stdlib `ServeMux`** (no gorilla/mux), JWT HS256 auth (optional via password_hash), path traversal prevention against directory allowlist, **SSE event stream** (chose over WebSocket since events are server→client only — zero external deps). All endpoints: status, jobs CRUD, directories CRUD, scan trigger, queue pause/resume, stats, auth/login. |
| **4 — React admin panel** | `frontend/` — Vite 8 + React 19 + TypeScript + Tailwind v3 + Radix UI + lucide-react. Sandstone palette per spec. Pages: Dashboard, Queue, History, Directories, Settings, Login. Layout component with sidebar (desktop) + bottom tab bar (mobile <768px). Real-time SSE updates via `useSSE` hook. Build outputs to `internal/api/frontend_dist/` for Go embed. |
| **5 — Plex + macOS** | `internal/plex/plex.go` (section discovery via `/library/sections`, refresh via `/library/sections/{id}/refresh`, non-fatal on errors). `scripts/com.sqzarr.agent.plist` (launchd), `scripts/install-macos.sh`, `scripts/sqzarr.service` (systemd with `LIBVA_DRIVER_NAME=iHD` env, `SupplementaryGroups=video render`). |
| **6 — Polish** | README with full Proxmox LXC + macOS install instructions, FAQ, security notes. `sqzarr.toml.example`. `.github/workflows/{ci.yml,release.yml}` (test+frontend build+gosec; cross-compile linux/amd64 + darwin/arm64 on tag push). `scripts/smoke-test.sh`. `.gitignore` for node_modules + dist + db files. |

---

## Decisions logged in WORKFLOW.md

Added entries 13–17 to the Decision Log:
- **#13** CT ID 260 (240 was Home Assistant on pve3)
- **#14** `intel-media-va-driver-non-free` required for HEVC encode
- **#15** DHCP for CT 260 (per owner preference)
- **#16** stdlib `ServeMux` instead of gorilla/mux (zero deps)
- **#17** SSE instead of WebSocket (one-way events, zero deps)

---

## Tests

13 integration tests, all passing on CT 260 at end of session:
- `internal/api/`: status, jobs CRUD, cancel, path traversal, auth required, login
- `internal/queue/`: quarantine GC sweep
- `internal/scanner/`: H.264 enqueue, HEVC skip, fresh-file skip, dedup
- `internal/verifier/`: valid output passes, output-larger fails

Run: `go test -tags integration $(go list ./... | grep -v /frontend/)`

---

## Known unfinished items at handoff

1. **1-week dogfood period** — Phase 6 DoD requires 1 full week of clean operation on real library before tagging v1.0.0
2. **macOS VideoToolbox** — could not test, no SSH access to the M4 Mac Mini
3. **gosec** — not run manually before handoff; CI workflow will catch it on first push
4. **Smoke test in CI** — not yet wired into the workflow file

---

## Things that broke during the session and how they were fixed

- **HEVC encode missing in vainfo** — installed `intel-media-va-driver-non-free` after enabling Debian non-free repo
- **CT 240 already taken** — used 260 (next available pve2 2xx slot), documented in decision log
- **`go test ./...` picked up Go file in `frontend/node_modules`** — added `.gitignore` and `GO_PKGS := $(shell go list ./... | grep -v /frontend/)` to Makefile
- **Service failed with "readonly database"** — `chown -R sqzarr:sqzarr /var/lib/sqzarr` fixed it
- **Port 8080 already in use** — leftover manual `sqzarr serve` from testing; killed by PID and restarted via systemd
- **CRLF warnings everywhere** — added `.gitattributes` with `text=auto`
- **`TestScanFindsH264File` failed initially** — synthetic 640x480 testsrc clip was below the 4 Mbps bitrate threshold; fix was to set `MaxBitrate: 0` (disabled) in tests that aren't specifically testing the bitrate filter
- **TypeScript build errors** — unused imports, `verbatimModuleSyntax` requiring type-only imports, `DirForm` type mismatch with `Partial<Directory>` (fixed by introducing a separate `DirectoryInput` snake_case type matching the Go JSON tags)

---

## Workflow used

- All Go and React source written locally in `c:\src\_project-sqzarr\` (visible to user in IDE)
- Committed and pushed to GitHub from local Windows machine
- SSH'd to PVE host (192.168.29.66) and used `pct exec 260` to run commands inside the container
- Pulled from GitHub on the container, ran `go mod tidy` + `go build` + `go test` there
- Frontend built on the container (`npm run build`) — outputs go directly into `internal/api/frontend_dist/` which gets embedded into the Go binary

---

## Session ended with

- Service running cleanly via systemd
- VAAPI encoder detected and selected
- `/media/Videos` configured as a watched directory
- Plex notifier configured (server: 192.168.29.61, token captured)
- All tests green
- 7 commits on `main` (Phases 1–6 plus fixes)
