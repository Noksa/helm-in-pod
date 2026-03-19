#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(dirname "$(realpath "$0")")/../scripts/common.sh"

# Allow GINKGO_ARGS to be passed from environment or default to empty
GINKGO_ARGS="${GINKGO_ARGS:-}"

cyber_step "Running E2E Tests"

# Check if kubeconfig exists
if [ ! -f "${E2E_KUBECONFIG}" ]; then
    cyber_err "Kubeconfig not found at ${E2E_KUBECONFIG}"
    echo ""
    echo "Run setup first:"
    echo "  ${CYBER_C}./e2e/setup-cluster.sh${CYBER_X}"
    exit 1
fi

# Verify cluster is accessible
cyber_log "Verifying cluster access"
if ! KUBECONFIG="${E2E_KUBECONFIG}" kubectl cluster-info &>/dev/null; then
    cyber_err "Cannot access cluster. It may have been deleted."
    echo ""
    echo "Run setup again:"
    echo "  ${CYBER_C}./e2e/setup-cluster.sh${CYBER_X}"
    exit 1
fi

# Check if ginkgo is installed
if ! command -v ginkgo &>/dev/null; then
    cyber_log "Installing ginkgo..."
    go install github.com/onsi/ginkgo/v2/ginkgo@latest
fi

# Export kubeconfig for tests
export KUBECONFIG="${E2E_KUBECONFIG}"
export KIND_CLUSTER="${E2E_CLUSTER_NAME}"

# Run tests
cyber_log "Running e2e tests with kubeconfig: ${CYBER_G}${E2E_KUBECONFIG}${CYBER_X}"
# Run with parallel execution for speed (each Describe gets its own process)
# shellcheck disable=SC2086
GINKGO_PROCS="${GINKGO_PROCS:-5}"
ginkgo --tags=e2e --procs="${GINKGO_PROCS}" --silence-skips --timeout=20m $GINKGO_ARGS "$@" "${E2E_DIR}/"

cyber_ok "E2E tests completed"
