# Verification Checklist: TKT-001 through TKT-009

Chain shipped 2026-04-09. Deployed to 192.168.29.211.

## Backend (Go)

- [x] **TKT-001**: `GET /jobs/savings` returns per-file breakdown; totals match dashboard stat
- [x] **TKT-003**: Scanner skips files within bitrate margin — check logs for skip messages with margin info
- [x] **TKT-004**: `processed_files` table created on startup; clearing history doesn't re-queue files on next scan
- [x] **TKT-006**: Clear history now includes `done`/`staged` jobs; processed_files survives clearing
- [x] **TKT-007**: Permission-denied files show status `error` (not `excluded`)
- [x] **TKT-008**: `GET /encoders` returns available HW + SW encoders; changing encoder in settings persists to `sqzarr.toml` and takes effect immediately
- [x] **TKT-009**: `POST /auth/change-password` works; old password required; new hash written to `sqzarr.toml`

## Frontend (React)

- [x] **TKT-001**: Space Saved card on dashboard is clickable, navigates to `/savings` detail page
- [x] **TKT-002**: Jobs Done links to `/history?status=done`; Failed links to `/history?status=failed`; filter pills work
- [x] **TKT-003**: Directory form shows "Skip margin (%)" input; default 10%
- [x] **TKT-005**: Copy button on directory cards copies settings to add form; batch add works
- [x] **TKT-007**: Red "Error" badge appears for I/O error jobs (distinct from grey "Excluded")
- [x] **TKT-008**: Encoder dropdown in Settings populated from API; selection saves and persists
- [x] **TKT-009**: Password change form in Settings (current + new + confirm); validation and error feedback work

## Database Migration

- [x] **TKT-003**: `bitrate_skip_margin` column present on directories table
- [x] **TKT-004**: `processed_files` table + index exist

## Deployment

- [x] Built frontend
- [x] Copied source to server
- [x] Built binary on server
- [x] Service restarted and active

## Notes

Tickets were shipped (merged + archived) before manual verification. If any item above fails, reopen the ticket with `/tro NNN` and fix.
