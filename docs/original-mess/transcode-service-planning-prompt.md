# Planning Prompt: Custom Media Transcoding Service

Use this prompt to brief a planning agent on designing a custom transcoding service. The postmortem file `tdarr-postmortem.md` in this directory has full context on what was tried and why it failed.

---

## Prompt

I need you to design a custom media transcoding service. Here's the context:

### Background
I tried to set up Tdarr for automated media transcoding and it was a complete disaster — broken node architecture, Docker networking nightmares, incompatible ffmpeg flag generation, opaque plugin systems. The postmortem is in `tdarr-postmortem.md`. Read it first.

The one thing that DID work perfectly: **ffmpeg with VAAPI hardware encoding on an Intel i5-8350U iGPU**. This command works flawlessly and encodes 1080p HEVC at 9-18x realtime:

```bash
ffmpeg -init_hw_device vaapi=va:/dev/dri/renderD128 -filter_hw_device va \
  -i INPUT.mkv \
  -vf 'format=nv12,hwupload,scale_vaapi=w=min(1920\,iw):h=-2' \
  -c:v hevc_vaapi -b:v 2300k -maxrate 4000k -bufsize 4600k \
  -c:a copy -c:s copy -max_muxing_queue_size 9999 \
  OUTPUT.mkv
```

### What I Want
A **service** (not a script, not a web UI) that:

1. **Continuously scans** configured media directories for video files
2. **Filters** candidates:
   - File mtime > 30 days old
   - Overall bitrate > 1 GB/hr (~2300 kbps)
   - Any video codec (not just H.264 — also mpeg4, avi, etc.)
   - Skip files already being processed or recently processed
3. **Transcodes** using Intel VAAPI GPU:
   - Output: HEVC (`hevc_vaapi`) at ~2300 kbps video bitrate
   - Downscale to max 1080p if source is higher
   - MKV container, copy all audio and subtitle streams
   - Replace original only if output is smaller
   - One file at a time (iGPU is modest)
4. **Is observable**:
   - Logs what it's doing
   - Can report on progress, queue size, space saved
   - Maybe a simple API or CLI to check status
5. **Is safe**:
   - Never corrupts or loses the original file
   - Handles crashes gracefully (resume or clean up temp files)
   - Respects ZFS space (configurable temp location, monitors free space)
   - Can be paused/resumed
6. **Plays nice with Plex**:
   - Optionally notify Plex to rescan after a file is replaced
   - Don't transcode while Plex is actively streaming (optional, nice-to-have)

### Environment
- **Runs on**: Proxmox host `pve2` in an unprivileged LXC container (Debian 13)
- **CPU**: Intel i5-8350U (4c/8t, 15W mobile chip — will thermal throttle under sustained load, that's fine)
- **iGPU**: Intel UHD 620, VAAPI working via `intel-media-va-driver-non-free`
- **GPU device**: `/dev/dri/renderD128` (passed through to LXC, working)
- **RAM**: 15GB total on host, ~67% used, LXC has 8GB allocated
- **Storage**: ZFS pool `storePool` with ~3.28TB free
- **Media location**: `/media/Videos/` inside the LXC (bind mount from fileserver CT 250's ZFS subvol)
- **Media structure**: Genre-based folders (Animation, Backlog, Comedy, Drama, TV, etc.) — NOT a flat Movies/TVShows split
- **File ownership**: UID 1000 / GID 100 (`users`) — the `tdarr` user (UID 1000) has r/w access
- **ffmpeg**: 7.1.3 installed natively in the LXC, all codecs and VAAPI support confirmed working
- **Plex**: Runs on separate host `pve3` (CT 330), accesses media via Samba share from fileserver
- **Network**: 192.168.29.x LAN, Tailscale overlay

### Design Questions to Answer
1. **Language/framework**: What should this be written in? Bash? Python? Go? Rust? Consider: it needs to be maintainable, handle file operations safely, and ideally be a proper daemon.
2. **Architecture**: Single binary/script? Multiple components? How does scanning, queuing, and transcoding interact?
3. **State management**: How do we track which files have been processed, which are queued, which failed? SQLite? Flat file? Filesystem markers?
4. **Configuration**: How does the user configure scan directories, bitrate targets, schedules? Config file? CLI flags?
5. **Deployment**: How is it installed and updated? systemd service? Package?
6. **Monitoring**: How does the user check what it's doing? Logs only? Simple HTTP status page? CLI tool?
7. **Safety**: How do we ensure no data loss during transcode? Atomic file replacement? Temp files? What if the process is killed mid-transcode?
8. **Plex integration**: How to notify Plex? API call? Or just rely on Plex's built-in change detection?
9. **Scalability**: This is for one machine now, but could it eventually support multiple nodes (e.g., if I add a GPU to pve3)?
10. **Naming**: This thing needs a name.

### Constraints
- **No Docker**. Native execution only.
- **No web UI frameworks**. If there's a status page, it's minimal.
- **No Tdarr, no Unmanic, no existing transcode managers**. Custom from scratch.
- **Must work in an unprivileged LXC** with iGPU passthrough.
- The existing LXC (CT 340) has everything installed (ffmpeg, VAAPI drivers, proper user/group setup). Reuse it or create a new one — your call.
