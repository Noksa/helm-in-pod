#!/usr/bin/env bash
# Common library for all scripts - handles cyberpunk theme auto-download and initialization

set -euo pipefail

# Get project root directory
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Download cyberpunk theme if not present
CYBER_LIB="${PROJECT_DIR}/.cyber.sh"
if [ ! -f "$CYBER_LIB" ]; then
    curl -s https://raw.githubusercontent.com/Noksa/install-scripts/main/cyberpunk.sh > "$CYBER_LIB"
fi

# Source the cyberpunk theme
# shellcheck disable=SC1090
source "$CYBER_LIB"

# Set up trap for clean exit
trap cyber_trap SIGINT SIGTERM
