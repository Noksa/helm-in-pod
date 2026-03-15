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

# E2E test configuration
E2E_CLUSTER_NAME="${KIND_CLUSTER:-helm-in-pod-e2e}"
E2E_DIR="${PROJECT_DIR}/e2e"
E2E_KUBECONFIG="${E2E_DIR}/.kubeconfig"
# If user passes just a version like "v1.28.15", expand to full image reference
_RAW_KIND_IMAGE="${KIND_NODE_IMAGE:-}"
if [ -n "${_RAW_KIND_IMAGE}" ] && [[ "${_RAW_KIND_IMAGE}" != */* ]]; then
    KIND_NODE_IMAGE="kindest/node:${_RAW_KIND_IMAGE}"
else
    KIND_NODE_IMAGE="${_RAW_KIND_IMAGE}"
fi
