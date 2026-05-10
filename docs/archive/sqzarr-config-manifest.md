# Configuration Manifest — SQZARR Transcoder
**Generated:** 2026-04-02
**Source transcript:** c:\src\_project-sqzarr\docs\downloaded-prompts\dan-transcript.md
**Prompt version:** Improved (config_manifest v1)

---

## People

- **Primary user:** Dan (the founder) — single technical user running a home media server
- **Named users:** OPEN_ITEM: not applicable for 1.0 — single-user system with optional admin password. No named user accounts required.
- **User count:** 1 (confirmed: "single user — the technical person in the house"). Open source release implies future multi-user installs but each installation is single-user.

---

## What It Does

- **Core loop:** Service scans configured media directories on a schedule → finds video files meeting configurable criteria (age, size, codec) → transcodes to HEVC using available hardware GPU → verifies output (size + playability check) → replaces original if smaller and verified → notifies Plex to rescan → logs result with bytes saved.
- **Vote/rating mechanism:** N/A — not a voting application.
- **Data sources:** Local filesystem only. No external data dependencies at runtime. Optional Plex API call (library rescan) after replacement.

---

## Technical Decisions

- **Stack:** Go backend, React frontend, runs as systemd service (Linux) or launchd daemon (macOS). No Docker. No containers of any kind.
- **Auth method:** Optional admin password on the web panel (bcrypt-hashed). No user accounts. Founder: "maybe have an option for a password in the admin panel because like you don't want the kids to fuck things up."
- **External dependencies:**
  - ffmpeg (transcoding engine — required)
  - Intel VAAPI drivers (Linux/Proxmox LXC — primary hardware path)
  - Apple VideoToolbox (macOS M4 Pro — secondary hardware path)
  - NVENC (Nvidia — tertiary, for other users)
  - AMD/ATI VAAPI ("if it can work on ATI even better")
  - Plex API (optional — rescan after replacement)
  - Tailscale (for remote admin access — founder's network setup, not a service dependency)
- **Explicitly forbidden:** Docker ("no Docker anywhere ever — don't even want it to work with Docker even if the user wants it"); Python; Java; PHP; AI-attribution in commit messages; paywalled features

---

## Scope

- **In 1.0:**
  - Directory configuration via web UI (add, edit, remove scan directories)
  - Configurable scan criteria per directory (age, bitrate, codec exclusions)
  - Hardware-agnostic transcoding engine (auto-detect VAAPI / VideoToolbox / NVENC / software fallback)
  - File safety: verify output before replacing original; optional quarantine/trash folder with configurable retention period before deleting original
  - ZFS/disk space monitoring — pause jobs if free space drops below configurable threshold
  - One transcode at a time by default; configurable queue depth / parallel streams
  - Web admin panel: dashboard with status at a glance, queue view, history/logs, settings
  - Space-saved running tally ("we compressed things down like 70 gigabytes")
  - Manual "scan now" trigger from UI
  - Manual per-file transcode trigger from UI ("boom, transcode this file right now")
  - Plex library rescan API call after replacement (optional, configurable)
  - Cross-platform: Linux LXC on Proxmox, macOS (M4 Pro), not locked to Proxmox
  - Open source; clean professional presentation; security review before GitHub publish
  - Sandstone/clean color palette; no purple AI-aesthetic
  - HEVC as output codec (confirmed)
  - Skip files already encoded in HEVC or AV1 (stated in Stage 1)

- **Explicitly out of 1.0:** OPEN_ITEM: Stage 4 was abandoned — founder said "I'm kind of getting bored with this interview so let's just move on with it." No exclusions were formally logged. Builder must infer conservatively. Scope is undefined beyond the features explicitly stated above.

---

## Open Items

1. **1.0 scope boundary** — Stage 4 ("three things NOT in 1.0") was never completed. No explicit exclusion list exists. The agent had to infer 1.0 scope from conversational context.
2. **Additional excluded codecs beyond HEVC/AV1** — Founder said "HEVC/AV1: skip them" but VP9 and other modern codecs were not addressed. Should VP9, AV1, or any other codec be in the skip list?
3. **Quarantine UI** — Backend quarantine was described ("move to temp location, delete after 10 days if not rejected") but no UI for viewing/managing quarantined files was specified.
4. **Settings UI vs. config file boundary** — Founder said "configure directories on my location... when it meets the criteria that I set." It was never resolved which settings live in the web UI vs. a config file (e.g., sqzarr.toml). Specifically: concurrency, disk threshold, scan interval, Plex URL/token.
5. **Scan schedule UI** — Founder mentioned "schedule this scan instead of waiting till Friday." It is unclear whether this means a one-time manual trigger or a recurring schedule editor in the UI.
6. **Name finalization** — Founder confirmed "SQZARR" (p-d-a-r-r) but tagline was not resolved. Founder suggested "save space R Us" and "transcode your shit and it works" as jokes; no final tagline locked.
7. **CT 340 on pve2** — Founder said "tear that shit down, delete that." Whether to rebuild clean in a new CT or reuse the number was not specified.
8. **Mac Mini transition timeline** — Founder mentioned moving to Mac Mini M4 Pro "at some point." Whether this is a 1.0 target or a later migration is unclear.
9. **Security review process** — Founder requires security review before GitHub publish. No reviewer, checklist, or tool was named.

---

## Assumptions Made

1. ASSUMPTION: AV1 and HEVC are the skip-list codecs. VP9 was not mentioned in either direction and was not added. Interviewer's closing summary generalized to "configurable criteria (codec)" without listing the specific exclusions.
2. ASSUMPTION: "Output codec is HEVC" — interviewer inferred this from Tdarr context and the founder's complaints about large file sizes. The founder never explicitly said "transcode to HEVC." The original ffmpeg command from the postmortem used HEVC; this was treated as confirmed.
3. ASSUMPTION: Single transcode at a time is the default; parallelism is configurable. Founder said "process one file at a time for now but have the option to do it in a queue." The boundary between "1.0 default" and "configurable at launch" was not locked.
4. ASSUMPTION: The admin password is optional, not required. Founder said "maybe have an option." The word "maybe" was interpreted as optional feature, not a requirement.
5. ASSUMPTION: Plex integration is optional (the service "won't give a shit if you're running Plex or not"). Founder's statement about not caring about Plex/Jellyfin was interpreted as "make Plex optional, don't require it."
6. ASSUMPTION: "Runs natively as systemd/launchd" means no containerization of any kind, including non-Docker containers. Founder's Docker hatred was interpreted broadly.
7. ASSUMPTION: The existing ffmpeg VAAPI invocation from the postmortem is the baseline command. Founder said "I don't care if you use ffmpeg or not" but also required hardware transcoding on Intel, Apple Silicon, and Nvidia — ffmpeg is the only practical engine covering all three. This was treated as implicitly confirmed.
