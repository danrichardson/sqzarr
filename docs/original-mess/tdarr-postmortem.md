# Tdarr Setup Postmortem — 2026-04-02

## What We Tried
Set up Tdarr (transcoding automation) in an unprivileged LXC on Proxmox host pve2, with Intel QuickSync/VAAPI hardware transcoding via the i5-8350U's iGPU.

## What Actually Worked
- LXC creation with iGPU passthrough — flawless
- ID mapping for file permissions — worked first try
- VAAPI encoding from the command line — confirmed working at 9-18x realtime
- ZFS snapshot for safety — done
- Media mounting — perfect read/write access

## What Was a Disaster

### 1. Tdarr v2.67 Native Binary Architecture is Broken
- The `Tdarr_Server` and `Tdarr_Node` binaries are **identical**. Both run a full server.
- The Node binary does NOT connect to an external server as a worker via Socket.IO — despite the server code having a full node registration system.
- Running both on the same machine causes port conflicts. Every workaround (separate directories, env var overrides, symlinks, copies) failed because the binaries resolve paths through symlinks and share config directories in unpredictable ways.
- **Hours wasted**: ~3 hours fighting binary layout, config paths, port conflicts, and discovering the node never connects.

### 2. Docker in an Unprivileged LXC is a Nightmare
- Fell back to Docker because native binaries couldn't register a node.
- Docker image pull took 30+ minutes due to flaky CDN from pve2.
- Docker's `bridge-nf-call-iptables=1` default **killed all inbound network traffic** to the LXC. The web UI became unreachable. Had to debug at the iptables level.
- `--group-add 104` for render device access only applied to the root process, NOT to the `abc` worker user that actually runs ffmpeg. Device permissions didn't propagate through Docker's process hierarchy.
- Even after fixing device permissions to 666, the `-hwaccel vaapi -hwaccel_device /dev/dri/renderD128 -hwaccel_output_format vaapi` flags (which Tdarr's flow plugins generate) fail. The alternative `-init_hw_device` approach works but Tdarr doesn't use it.
- **Hours wasted**: ~2 hours on Docker networking, permissions, and discovering the hwaccel flag incompatibility.

### 3. Tdarr's Flow System is Opaque and Fragile
- Flows created via the CRUD API don't persist (stored in SQLite but the in-memory cache doesn't sync).
- Had to write directly to SQLite to create flows.
- The library has TWO `flowId` fields (`flowId` at top level AND inside `transcodingOptions`) — updating one doesn't update the other.
- Community "Migz GPU" plugin hardcodes NVIDIA NVENC — useless for Intel VAAPI.
- Tdarr's built-in encoder detection reports `hevc_vaapi` as "not working" even though manual testing proves it works perfectly.
- The flow plugin system builds ffmpeg commands using `-hwaccel_output_format vaapi` which is incompatible with the container's ffmpeg build (Jellyfin fork).
- **Hours wasted**: ~2 hours on flow creation, assignment bugs, and discovering the hwaccel incompatibility.

### 4. tdarr.io CDN is Unreliable
- Downloads from storage.tdarr.io averaged 280KB/s with frequent connection drops.
- Required multiple retries with `--continue` to download the ~400MB of binaries.
- DNS resolution failures from inside the LXC added delays.
- **Hours wasted**: ~1 hour waiting for downloads.

## What the User Actually Wants
A service that runs on pve2 (in the existing LXC or a new one) that:

1. **Scans** media directories on the fileserver (mounted via bind mount)
2. **Filters** files that are:
   - Older than 30 days (by mtime)
   - Over 1 GB/hr bitrate (roughly >2300 kbps overall)
3. **Transcodes** matching files to:
   - HEVC via VAAPI hardware encoding (`hevc_vaapi`)
   - Target ~1 GB/hr (2300 kbps video bitrate)
   - Max 1080p resolution (downscale if higher)
   - MKV container
   - Copy audio and subtitle streams
   - Replace original file only if output is smaller
4. **GPU only** — no CPU transcoding (waste of resources on this hardware)
5. **Runs as a service** — not a one-shot script, not a web UI, just a daemon that does its job

## Environment Summary
- **Host**: pve2, Intel i5-8350U, UHD 620 iGPU, 15GB RAM (67% used)
- **Storage**: ZFS `storePool` with 3.28TB free
- **Media**: `/storePool/subvol-250-disk-0/srv/storage/Videos/` — genre-based folders, files owned by UID 1000 / GID 100 (users)
- **Existing LXC**: CT 340 (tdarr) — Debian 13, has iGPU passthrough working, VAAPI confirmed working, ffmpeg + intel-media-va-driver-non-free installed
- **What works**: `ffmpeg -init_hw_device vaapi=va:/dev/dri/renderD128 -filter_hw_device va -i INPUT -vf 'format=nv12,hwupload' -c:v hevc_vaapi -b:v 2300k -c:a copy -c:s copy OUTPUT.mkv`
- **Network**: 192.168.29.x, Tailscale available, Plex on pve3 accesses media via Samba from fileserver (CT 250)
- **Other containers**: gitea, cloudflare-ddns, wyoming, fileserver
- **Render device**: `/dev/dri/renderD128`, GID 104, needs group membership or 666 perms
- **ZFS snapshot exists**: `storePool/subvol-250-disk-0@pre-tdarr`

## Key Lessons for the Custom Solution
1. **VAAPI works** — but ONLY with `-init_hw_device vaapi=va:/dev/dri/renderD128 -filter_hw_device va` and `-vf format=nv12,hwupload`. The `-hwaccel vaapi -hwaccel_device` approach fails in this environment.
2. **No Docker** — run natively in the LXC. The iGPU, ffmpeg, and all drivers are already installed and working.
3. **Keep it simple** — a systemd service running a well-tested script/binary is infinitely more reliable than a web UI framework with broken plugin systems.
4. **File permissions** — the tdarr user (UID 1000) already has read/write access to the media. No additional mapping needed.
5. **One GPU transcode at a time** — the UHD 620 can handle 1 concurrent HEVC encode at ~9-18x realtime. A 1-hour file takes 3-7 minutes.
6. **ZFS space awareness** — the storage pool had a previous capacity crisis. Monitor temp file accumulation.
