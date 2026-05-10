#!/usr/bin/env bash
set -euo pipefail

# Convenience wrapper; keep host/user in env or a local wrapper script.
"$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/deploy.sh"
