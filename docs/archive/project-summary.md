# SQZARR — Project Summary

## Vision

SQZARR is a self-hosted media transcoding service for home server owners drowning in oversized video files. The problem it solves is simple: you've got hundreds of TV shows and movies eating your storage at 5–10 GB per episode, downloaded by Sonarr or similar tools, and you want them compressed down using the GPU sitting idle in your server. Existing tools like Tdarr are overcomplicated, brittle, and require too much configuration to get right. SQZARR does one thing well: finds bloated files, shrinks them safely, and stays out of your way. It ships looking polished — not a proof-of-concept. When it hits GitHub, people should say "whoa, someone actually made this right."

## Core Loop

1. SQZARR scans your configured media directories on a schedule (every few hours is fine — no real-time monitoring needed)
2. It identifies files that meet your rules: too big for their runtime, old enough to have settled, not already HEVC or AV1
3. It transcodes each qualifying file using whatever GPU it detects — Intel Quick Sync (VAAPI), Apple VideoToolbox, or NVIDIA NVENC — falling back to software if nothing is available
4. It verifies the output: smaller file size, correct duration, playable video
5. It replaces the original — optionally moving it to a quarantine folder first so you have a safety window to reject the transcode
6. Optionally, it calls the Plex API to trigger a library rescan so your media manager sees the updated file

## Key Risks

**File corruption or loss** is the #1 concern — especially for home videos or anything irreplaceable. Mitigation: SQZARR never touches the original until output is verified smaller and confirmed playable. An optional quarantine folder holds originals for a configurable retention period (default: 10 days) before automatic deletion.

**Running out of disk space during transcoding** is what caused the Tdarr failure this project is replacing. Mitigation: SQZARR checks free space before starting each job and pauses with an alert if it drops below a configurable threshold.

**Misconfigured rules targeting the wrong files** could result in unexpected transcoding. Mitigation: the UI shows a preview of which files would match before any job runs, conservative defaults, and the quarantine folder as a catch-all safety net.

## Open Questions

- **Plex API endpoint**: What exact API call triggers a library rescan after in-place file replacement? Needs research before implementation. (Tdarr used a community plugin — endpoint exists but needs verification.)
- **Playability verification method**: Duration comparison with ffprobe is cheap and reliable. Should we also decode a sample of frames to confirm? Trade-off between thoroughness and speed.
- **Quarantine auto-deletion**: 10-day default before purging originals? Should this be configurable per directory or globally?
- **Hardware fallback behavior**: If no hardware encoder is found, should SQZARR fail clearly or silently fall back to slow software encoding? User preference may vary.

## Scope

**In 1.0:**
- Directory scanning with configurable rules (file age, size, codec, bitrate threshold)
- Hardware-agnostic transcoding: Intel VAAPI, Apple VideoToolbox, NVIDIA NVENC, software fallback
- File safety pipeline: output verification before replacement, optional quarantine folder with configurable retention
- Clean React admin panel: live status dashboard, space-saved counter, job history, manual transcode trigger
- Disk space monitoring with configurable pause threshold
- Optional admin password (single-user, no accounts)
- Optional Plex library rescan notification after replacement
- Runs natively: systemd service on Linux, launchd daemon on macOS

**Not in 1.0:**
- Multi-user accounts or role-based access control
- Push/email/webhook notifications — logs only
- Jellyfin or Emby-specific integrations
- Remote/cloud storage targets
- Mobile app
- Paid tiers or licensing of any kind
