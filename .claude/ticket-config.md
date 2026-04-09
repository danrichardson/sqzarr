# Ticket Workflow Config

- Stack: Go + Vite/React
- Tickets directory: tickets/
- ID prefix: TKT-

## Commands
- Test: make test-integration
- Build: make all
- Deploy: deploy.bat
- Lint: make lint

## Preview settings
- Preview mode: individual
- Preview port base: 3000

## Preview profiles

<!-- No preview profiles — sqzarr requires ffmpeg and media files to run meaningfully. -->

## Key source locations
- cmd/ — CLI entrypoint (sqzarr serve)
- internal/scanner/ — filesystem scanner that discovers new media
- internal/transcoder/ — ffmpeg transcoding logic
- internal/queue/ — job queue and worker
- internal/verifier/ — output verification (bitrate, duration checks)
- internal/rename/ — codec-aware file renaming
- internal/api/ — HTTP API and embedded frontend
- internal/config/ — TOML config parsing
- internal/db/ — SQLite database layer
- frontend/ — React/Vite dashboard UI

## Context docs
- docs/architecture.md
- docs/code-starter-packet.md
- README.md
- WORKFLOW.md
