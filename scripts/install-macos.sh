#!/usr/bin/env bash
# SQZARR macOS installer — tested on macOS 14+ (Apple Silicon)
set -euo pipefail

BINARY="./sqzarr-darwin-arm64"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/Users/Shared/sqzarr"
LOG_DIR="/var/log/sqzarr"
PLIST_SRC="$(dirname "$0")/com.sqzarr.agent.plist"
PLIST_DEST="/Library/LaunchDaemons/com.sqzarr.agent.plist"

if [[ "$(uname -s)" != "Darwin" ]]; then
    echo "error: this script is for macOS only" >&2
    exit 1
fi

if [[ ! -f "$BINARY" ]]; then
    echo "error: $BINARY not found — build it first with 'make build-darwin'" >&2
    exit 1
fi

echo "Installing SQZARR..."

# Binary
sudo install -m 755 "$BINARY" "$INSTALL_DIR/sqzarr"
echo "  [ok] binary -> $INSTALL_DIR/sqzarr"

# Config directory
sudo mkdir -p "$CONFIG_DIR"
if [[ ! -f "$CONFIG_DIR/sqzarr.toml" ]]; then
    sudo tee "$CONFIG_DIR/sqzarr.toml" > /dev/null <<TOML
[server]
host = "127.0.0.1"
port = 8080
data_dir = "/Users/Shared/sqzarr"

[scanner]
interval_hours = 6
worker_concurrency = 1

[safety]
quarantine_enabled = true
quarantine_retention_days = 10
disk_free_pause_gb = 50

[plex]
enabled = false
base_url = ""
token = ""

[auth]
password_hash = ""
jwt_secret = ""
TOML
    echo "  [ok] config -> $CONFIG_DIR/sqzarr.toml (edit before starting service)"
else
    echo "  [skip] config already exists at $CONFIG_DIR/sqzarr.toml"
fi

# Log directory
sudo mkdir -p "$LOG_DIR"
echo "  [ok] log dir -> $LOG_DIR"

# LaunchDaemon plist
sudo install -m 644 "$PLIST_SRC" "$PLIST_DEST"
sudo launchctl load -w "$PLIST_DEST" 2>/dev/null || true
echo "  [ok] launchd -> $PLIST_DEST"

echo ""
echo "SQZARR installed."
echo ""
echo "Edit $CONFIG_DIR/sqzarr.toml to add your media directories."
echo "Then: sudo launchctl kickstart -k system/com.sqzarr.agent"
echo ""
echo "Admin panel: http://localhost:8080"
