# Tdarr LXC Setup on pve2 - Claude Code Execution Plan

## Mission

Install and configure Tdarr (transcoding automation) in an unprivileged LXC container on Proxmox host `pve2`, with Intel QuickSync hardware transcoding enabled. The goal is to automatically transcode older media files (H.264 -> H.265/HEVC) to reclaim storage space on the ZFS pool.

---

## Host Environment - READ THIS FIRST

### pve2 Hardware

- **CPU**: Intel Core i5-8350U @ 1.70GHz (8th gen Kaby Lake R) - 4 cores / 8 threads
- **iGPU**: Intel UHD Graphics 620 - supports QuickSync H.265 encode/decode
- **RAM**: ~15.37 GiB total (currently ~67% used)
- **Root disk**: 67.73 GiB (36% used)
- **Kernel**: Linux 6.17.9-1-pve
- **Proxmox**: pve-manager 9.1.5
- **Boot**: EFI (Secure Boot)
- **Uptime**: 51+ days as of 2026-04-01

### Storage Architecture

- pve2 uses **ZFS** for storage pools. There is a `storePool` visible in the Proxmox sidebar.
- **CRITICAL**: This ZFS pool recently had a capacity crisis (Samba recycle bin accumulated 3TB+). Be conservative with any temp/cache directory sizing. Do NOT place Tdarr's transcode temp on the root disk - it needs to go on the storage pool where there is space, or on a dedicated temp location.
- There is a `fileserver` container (CT 250) that serves media via **Samba** shares.

### Existing Containers on pve2

| CT ID | Name | Purpose |
|-------|------|---------|
| 100 | gitea | Git server |
| 210 | cloudflare-ddns | DDNS updater |
| 220 | wyoming | Wyoming (voice assistant) |
| 250 | fileserver | Samba file server (media storage) |
| 310 | torrentbox | Torrent client |
| 320 | sonarr | Sonarr PVR |

### pve3 (separate host)

| CT ID | Name | Purpose |
|-------|------|---------|
| 230 | mqtt | MQTT broker |
| 240 | haos-12.0 | Home Assistant OS |
| 330 | plex | Plex Media Server |

### Network Context

- Datacenter name: `plymptonia`
- Tailscale is in use across the network
- Plex runs on pve3 but accesses media stored on pve2's fileserver
- Sonarr (CT 320) on pve2 manages TV show acquisition

---

## Phase 0: Reconnaissance (MANDATORY - DO FIRST)

Before creating anything, gather the information needed to configure the LXC correctly. **Do not skip any of these steps.** The idmap configuration WILL break if these values are wrong.

### 0.1 - Determine Available CT ID

```bash
# SSH to pve2
# Find next available CT ID (existing: 100, 210, 220, 250, 310, 320)
# Suggest CT 340 for tdarr, but verify it's free
pct list
```

### 0.2 - Identify Storage Pools and Media Paths

```bash
# List ZFS pools and their usage
zfs list

# Find where media is actually stored
# We need the HOST path to the media files that Plex/Sonarr use
# Check the fileserver CT's mount points
cat /etc/pve/lxc/250.conf

# Also check sonarr's mounts to understand the media path structure
cat /etc/pve/lxc/320.conf

# Check available space
df -h
zfs list -o name,used,avail,mountpoint
```

**STOP AND REPORT** the media paths and available space before proceeding. We need to know:
- Where Movies live on the host filesystem
- Where TV Shows live on the host filesystem
- How much free space is available on the storage pool
- What mount points the fileserver and sonarr containers use

### 0.3 - Identify Group IDs

```bash
# Find the render group GID on the HOST
getent group render

# Find the video group GID on the HOST
getent group video

# Check if there's already a 'media' group (or equivalent) for shared file access
getent group media
# If no media group, check what group the media files use:
ls -la /path/to/media/  # use actual path from step 0.2

# Check existing subuid/subgid mappings
cat /etc/subuid
cat /etc/subgid
```

**STOP AND REPORT** the GIDs before proceeding. We need:
- render group GID (commonly 104 or 105)
- video group GID (commonly 44)
- media/shared file group GID and name
- Current subuid/subgid contents

### 0.4 - Verify iGPU Availability

```bash
# Confirm the iGPU device nodes exist on the host
ls -la /dev/dri/

# Check if any other container is already using the iGPU
# (It can be shared - this is just to understand the current state)
grep -r "dev/dri" /etc/pve/lxc/*.conf

# Verify QuickSync support on the host
vainfo 2>&1 | head -20
```

### 0.5 - Check Available Templates

```bash
# List available CT templates
pveam list local
# If no Debian 13 (trixie) template, we may need Debian 12 (bookworm)
# Debian 12 works fine for Tdarr
pveam available | grep debian
```

---

## Phase 1: Create the LXC Container

Based on the recon data from Phase 0, create the container. Adjust values based on what you discovered.

### 1.1 - Download Template (if needed)

```bash
# Download Debian 12 or 13 template if not present
pveam download local debian-12-standard_12.7-1_amd64.tar.zst
# OR for Debian 13:
# pveam download local debian-13-standard_13.0-1_amd64.tar.zst
```

### 1.2 - Create Container

```bash
# Use CT ID determined in Phase 0 (assumed 340 here)
# Use storage pool identified in Phase 0
# Allocate 8GB RAM, 4 cores, 20GB root disk
pct create 340 local:vztmpl/debian-12-standard_12.7-1_amd64.tar.zst \
  --hostname tdarr \
  --memory 8192 \
  --swap 2048 \
  --cores 4 \
  --rootfs storePool:20 \
  --net0 name=eth0,bridge=vmbr0,ip=dhcp \
  --unprivileged 1 \
  --features nesting=1 \
  --start 0
```

**NOTE**: Adjust `--rootfs` storage name based on Phase 0 findings. It might be `local-zfs`, `storePool`, or something else.

### 1.3 - Configure LXC for iGPU Passthrough and Media Access

**This is the most critical step.** Edit the container config using values from Phase 0.

```bash
nano /etc/pve/lxc/340.conf
```

Add the following lines (REPLACE placeholder values with actual values from Phase 0):

```ini
# --- Media bind mounts ---
# REPLACE these paths with actual media locations from Phase 0
mp0: /path/to/host/movies,mp=/media/Movies
mp1: /path/to/host/tvshows,mp=/media/TVShows

# --- iGPU passthrough ---
lxc.mount.entry: /dev/dri/ dev/dri/ none bind,optional,create=dir
lxc.cgroup2.devices.allow: c 226:* rwm

# --- ID mapping ---
# These MUST be calculated from Phase 0 values
# The pattern maps container GIDs to host GIDs for render and media groups
#
# Template (fill in actual values):
#   RENDER_HOST_GID = from 'getent group render' on host
#   MEDIA_HOST_GID  = from media file group
#   RENDER_CT_GID   = will be set to match inside container (typically 104 or 105)
#   MEDIA_CT_GID    = will be set to match inside container
#
# Default UID mapping (usually fine as-is):
lxc.idmap: u 0 100000 65536
#
# GID mapping (MUST be customized):
# Map GIDs 0 through (RENDER_CT_GID - 1) to unprivileged range
# Then map RENDER_CT_GID to HOST render GID
# Then map gap to unprivileged range
# Then map MEDIA_CT_GID to HOST media GID
# Then map remainder to unprivileged range
#
# EXAMPLE for render=104 on host, render=104 in CT, media=1001 on both:
# lxc.idmap: g 0 100000 104
# lxc.idmap: g 104 104 1
# lxc.idmap: g 105 100105 896
# lxc.idmap: g 1001 1001 1
# lxc.idmap: g 1002 101002 64534
#
# YOU MUST CALCULATE THESE FROM ACTUAL VALUES
```

### 1.4 - Update Host subgid

```bash
# Add entries to /etc/subgid to allow the mappings
# Check existing content first (from Phase 0)
cat /etc/subgid

# Add required entries (adjust GIDs to match your actual values):
# root:RENDER_HOST_GID:1
# root:MEDIA_HOST_GID:1
# (only add if not already present)
```

**STOP AND VERIFY**: Before starting the container, review the full config:
```bash
cat /etc/pve/lxc/340.conf
cat /etc/subgid
```

Confirm:
- Media paths match actual host paths
- GID mappings are mathematically correct (all ranges must be contiguous, cover 0-65535, no overlaps)
- subgid entries exist for all directly-mapped GIDs

---

## Phase 2: Configure the LXC Interior

### 2.1 - Start and Enter Container

```bash
pct start 340
pct enter 340
```

### 2.2 - System Setup

```bash
apt update && apt upgrade -y

# Add non-free repos for Intel media driver
# For Debian 12 (bookworm):
sed -i 's/main$/main contrib non-free non-free-firmware/' /etc/apt/sources.list
# For Debian 13 (trixie) - edit sources format may differ, check first:
# cat /etc/apt/sources.list.d/debian.sources

apt update

# Install dependencies
apt install -y \
  sudo curl unzip wget ca-certificates gnupg \
  vainfo libva2 intel-media-va-driver-non-free libva-drm2 \
  pciutils handbrake-cli ffmpeg mkvtoolnix \
  mediainfo
```

### 2.3 - Create Users and Groups

```bash
# Create media group matching host GID (use actual GID from Phase 0)
addgroup --gid 1001 media  # REPLACE 1001 with actual media GID

# Create tdarr system user
useradd -r -m -d /opt/tdarr -s /usr/sbin/nologin tdarr

# Add tdarr to required groups
usermod -aG render,video,media tdarr

# Verify
groups tdarr
# Expected output: tdarr : tdarr video render media
id tdarr
```

### 2.4 - Verify iGPU Access Inside Container

```bash
# Check device nodes exist
ls -la /dev/dri/

# Test QuickSync
sudo -u tdarr env LIBVA_DRIVER_NAME=iHD \
  vainfo --display drm --device /dev/dri/renderD128 2>&1 | head -30
```

**Expected**: You should see `Intel iHD driver` and HEVC encode/decode profiles listed.

**If vainfo fails**: Check that the lxc.mount.entry and cgroup lines are correct, and that the intel-media-va-driver-non-free package installed properly.

**NOTE for i5-8350U**: This is 8th gen (Kaby Lake R). The iHD driver should work, but if it doesn't, try:
```bash
# Fallback to i965 driver
apt install -y i965-va-driver
LIBVA_DRIVER_NAME=i965 vainfo --display drm --device /dev/dri/renderD128
```

### 2.5 - Verify Media Access

```bash
# Check media mounts are visible
ls -la /media/Movies/
ls -la /media/TVShows/

# Verify tdarr user can read them
sudo -u tdarr ls /media/Movies/ | head -5
sudo -u tdarr ls /media/TVShows/ | head -5
```

If permission denied: the idmap GID mapping is wrong. Go back to Phase 1.3.

---

## Phase 3: Install Tdarr

### 3.1 - Download and Extract

```bash
cd /opt/tdarr

# Get latest Tdarr updater
wget https://storage.tdarr.io/versions/2.17.01/linux_x64/Tdarr_Updater.zip

# NOTE: Check https://home.tdarr.io for the latest version URL
# If 2.17.01 fails, browse to find the current release

unzip Tdarr_Updater.zip

# Create working directories
mkdir -p temp cache configs logs

# Make updater executable and run it
chmod +x Tdarr_Updater
./Tdarr_Updater
```

### 3.2 - Initial Run to Generate Config Files

```bash
# Run server briefly to generate config
/opt/tdarr/Tdarr_Server/Tdarr_Server &
sleep 15
kill %1

# Run node briefly to generate config
/opt/tdarr/Tdarr_Node/Tdarr_Node &
sleep 15
kill %1
```

### 3.3 - Fix Ownership

```bash
chown -R tdarr:tdarr /opt/tdarr/
```

### 3.4 - Configure Tdarr Server

Get the container's IP first:
```bash
hostname -I
```

Edit the server config:
```bash
nano /opt/tdarr/configs/Tdarr_Server_Config.json
```

Set these values (replace IP):
```json
{
  "serverPort": "8266",
  "webUIPort": "8265",
  "serverIP": "CONTAINER_IP_HERE",
  "serverBindIP": false,
  "serverDualStack": false,
  "handbrakePath": "/usr/bin/HandBrakeCLI",
  "ffmpegPath": "/usr/bin/ffmpeg",
  "logLevel": "INFO",
  "mkvpropeditPath": "/usr/bin/mkvpropedit",
  "ccextractorPath": "",
  "openBrowser": false,
  "cronPluginUpdate": "",
  "auth": false,
  "authSecretKey": "tsec_NotARealKey",
  "maxLogSizeMB": 10,
  "seededApiKey": ""
}
```

### 3.5 - Configure Tdarr Node

```bash
nano /opt/tdarr/configs/Tdarr_Node_Config.json
```

```json
{
  "nodeName": "pve2-tdarr",
  "serverURL": "http://CONTAINER_IP_HERE:8266",
  "serverIP": "CONTAINER_IP_HERE",
  "serverPort": "8266",
  "handbrakePath": "/usr/bin/HandBrakeCLI",
  "ffmpegPath": "/usr/bin/ffmpeg",
  "mkvpropeditPath": "/usr/bin/mkvpropedit",
  "pathTranslators": [
    {
      "server": "",
      "node": ""
    }
  ],
  "nodeType": "mapped",
  "unmappedNodeCache": "/opt/tdarr/cache",
  "logLevel": "INFO",
  "priority": -1,
  "cronPluginUpdate": "",
  "apiKey": "",
  "maxLogSizeMB": 10,
  "pollInterval": 2000,
  "startPaused": false
}
```

### 3.6 - Create systemd Services

**Server service:**
```bash
cat > /etc/systemd/system/tdarr-server.service << 'EOF'
[Unit]
Description=Tdarr Server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=tdarr
Group=tdarr
WorkingDirectory=/opt/tdarr/Tdarr_Server
Environment=TDARR_DATA=/opt/tdarr/configs
Environment=TDARR_LOGS=/opt/tdarr/logs
ExecStart=/opt/tdarr/Tdarr_Server/Tdarr_Server
Restart=on-failure

# Resource limits - conservative for i5-8350U
CPUAccounting=true
MemoryAccounting=true
CPUQuota=80%
MemoryMax=2G

[Install]
WantedBy=multi-user.target
EOF
```

**Node service:**
```bash
cat > /etc/systemd/system/tdarr-node.service << 'EOF'
[Unit]
Description=Tdarr Node
After=tdarr-server.service
Wants=tdarr-server.service

[Service]
Type=simple
User=tdarr
Group=tdarr
WorkingDirectory=/opt/tdarr/Tdarr_Node
Environment=TDARR_NODE_NAME=pve2-tdarr
Environment=TDARR_SERVER_HOST=CONTAINER_IP_HERE
Environment=TDARR_FFMPEG=/usr/bin/ffmpeg
Environment=TDARR_LOGS=/opt/tdarr/logs
Environment=TDARR_TEMP=/opt/tdarr/temp
Environment=TDARR_CACHE=/opt/tdarr/cache
ExecStart=/opt/tdarr/Tdarr_Node/Tdarr_Node
Restart=on-failure

# Resource limits - leave headroom for pve2's other containers
CPUAccounting=true
MemoryAccounting=true
CPUQuota=120%
MemoryMax=4G
IOSchedulingClass=best-effort
IOSchedulingPriority=7

[Install]
WantedBy=multi-user.target
EOF
```

**NOTE**: Replace `CONTAINER_IP_HERE` in the node service file.

### 3.7 - Enable and Start

```bash
systemctl daemon-reload
systemctl enable tdarr-server
systemctl start tdarr-server

# Wait a few seconds for server to initialize
sleep 10

systemctl enable tdarr-node
systemctl start tdarr-node

# Verify both are running
systemctl status tdarr-server
systemctl status tdarr-node
```

---

## Phase 4: Configure Tdarr via Web UI (Manual Step)

**STOP HERE AND REPORT** the Tdarr web UI URL (http://CONTAINER_IP:8265) to the user.

The following configuration should be done via the web UI, but document what needs to be set:

### 4.1 - Node Configuration

1. Go to the Nodes tab
2. Click on the "pve2-tdarr" node
3. Under Transcode Options, set Hardware Encoding to "Any (nvenc,qsv,vaapi)"
4. Set GPU workers to 1 (the i5-8350U's iGPU can handle 1 concurrent HW transcode comfortably)
5. Set CPU workers to 1 (leave headroom for the other containers on pve2)

### 4.2 - Library Setup

1. Add library "Movies" pointing to `/media/Movies`
2. Add library "TV Shows" pointing to `/media/TVShows`
3. Set both to scan on a schedule (e.g., every 12 hours)

### 4.3 - Recommended Plugin Stack (Transcode Flow)

Set up this flow for each library:

1. **Pre-check: Skip if already HEVC** - Don't re-transcode files that are already H.265
2. **Pre-check: Skip if file is smaller than 500MB** - Don't bother with small files
3. **Transcode: FFmpeg H.265 via QSV** - Use Intel QuickSync for hardware-accelerated transcoding
   - CRF: 23 (good quality/size balance)
   - Preset: medium
   - Container: MKV (preserves all metadata, subtitles, audio tracks)
4. **Post-check: Verify file health** - Make sure the output isn't corrupt
5. **Post-check: Verify file is smaller** - Don't replace if the transcode is somehow bigger

### 4.4 - Performance Expectations for i5-8350U

- The UHD 620 iGPU will handle QuickSync H.265 encoding but it's 8th gen - expect roughly **0.5-1.5x realtime** for 1080p content (a 2hr movie takes 1-3 hours)
- **Limit to 1 GPU transcode at a time** - the iGPU is modest
- Schedule heavy transcoding for off-hours when Plex isn't streaming
- Monitor pve2's overall load - with 67% RAM already used, keep Tdarr's memory capped

---

## Phase 5: Age-Based Transcoding (Optional Enhancement)

Tdarr doesn't natively filter by file age. Here are two approaches:

### Option A: Tdarr Watches Everything, Filters by Codec

The simplest approach - just let Tdarr process ALL non-HEVC files regardless of age. The "skip if already HEVC" pre-check means it only touches files that need conversion. This is the recommended approach since you want to reclaim space across your whole library anyway.

### Option B: Cron Script for Age-Based Staging

If you truly only want to transcode files older than 30 days, create a helper script:

```bash
cat > /opt/tdarr/age-filter.sh << 'SCRIPT'
#!/bin/bash
# Move files older than 30 days to a staging directory that Tdarr watches
# Original files stay in place - this creates symlinks instead

STAGING="/opt/tdarr/staging"
MEDIA_DIRS="/media/Movies /media/TVShows"
AGE_DAYS=30

mkdir -p "$STAGING"

for dir in $MEDIA_DIRS; do
  find "$dir" -type f \( -name "*.mkv" -o -name "*.mp4" -o -name "*.avi" \) \
    -mtime +${AGE_DAYS} -exec ln -sf {} "$STAGING/" \;
done
SCRIPT

chmod +x /opt/tdarr/age-filter.sh
chown tdarr:tdarr /opt/tdarr/age-filter.sh
```

Then point a Tdarr library at `/opt/tdarr/staging/` and run the script on a cron.

**NOTE**: Option A is recommended. Option B adds complexity and the symlink approach has edge cases with Tdarr's file replacement logic.

---

## Troubleshooting Reference

### iGPU Not Detected in Container

```bash
# On HOST - verify device nodes
ls -la /dev/dri/
# Should show: card0, card1, renderD128

# In CONTAINER - verify mount
ls -la /dev/dri/
# If empty, check lxc.mount.entry in CT config

# In CONTAINER - check driver
vainfo --display drm --device /dev/dri/renderD128
# If "libva error: /usr/lib/x86_64-linux-gnu/dri/iHD_drv_video.so init failed"
# Try i965 driver instead (8th gen is on the boundary):
apt install i965-va-driver
LIBVA_DRIVER_NAME=i965 vainfo --display drm --device /dev/dri/renderD128
```

### Permission Denied on Media Files

```bash
# In container, check effective GID
id tdarr
# Verify the media group GID matches what the files actually use
ls -ln /media/Movies/ | head -5
# The GID column should match tdarr's media group GID
```

### Tdarr Can't Write Transcoded Files Back

The tdarr user needs WRITE access to the media directories. If the Samba share or host filesystem is mounted read-only or with the wrong group, transcoded files can't replace originals.

```bash
# Test write access
sudo -u tdarr touch /media/Movies/test_write && rm /media/Movies/test_write
```

### ZFS Space Monitoring

Given the previous ZFS capacity crisis, keep an eye on space:

```bash
# On HOST
zfs list
# Watch for temp file accumulation in the container
du -sh /opt/tdarr/temp/ /opt/tdarr/cache/
```

### Plex Not Picking Up Changes

After Tdarr replaces a file, Plex may still think it's the old format. Options:
- Enable Plex's "Detect changes" in library settings
- Trigger a Plex library scan via API after transcodes complete
- Tdarr has a Plex refresh plugin - search community plugins for "Plex notify"

---

## Important Warnings

1. **BACKUP FIRST**: Before Tdarr processes any files, verify you have backups or are OK with the originals being replaced. Tdarr replaces the source file by default.

2. **TEST WITH ONE FILE**: Before unleashing Tdarr on the full library, manually test one transcode and verify quality in Plex.

3. **ZFS SNAPSHOTS**: Consider taking a ZFS snapshot of the media pool before the first batch run. This gives you a rollback path if quality is unacceptable.

4. **RAM PRESSURE**: pve2 is already at 67% RAM usage. The Tdarr node is capped at 4GB in the systemd service above. Monitor for OOM kills after starting. Reduce to 2GB if needed.

5. **CPU/THERMAL**: The i5-8350U is a 15W mobile chip. Sustained transcoding will thermal throttle. This is fine - it just means transcodes take longer. The systemd CPUQuota limits help prevent starving other containers.

6. **PLEX INTERACTION**: Tdarr lives on pve2 but Plex is on pve3. If Plex accesses media via a Samba share from the fileserver, Tdarr's file replacements should be transparent. But verify Plex's media path doesn't cache file metadata that would go stale.

---

## Success Criteria

The setup is complete when:

- [ ] LXC container is running with iGPU passthrough working
- [ ] `vainfo` shows HEVC encode support inside the container
- [ ] Tdarr web UI is accessible at http://CONTAINER_IP:8265
- [ ] Tdarr node shows as connected with GPU worker enabled
- [ ] Media libraries are visible and scanned in Tdarr
- [ ] One test file transcodes successfully from H.264 to H.265 via QuickSync
- [ ] The transcoded file plays correctly in Plex
- [ ] Tdarr services survive a container reboot