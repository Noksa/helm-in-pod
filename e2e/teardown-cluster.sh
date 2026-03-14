#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(dirname "$(realpath "$0")")/../scripts/common.sh"

cyber_step "Tearing down kind cluster"

# Check if cluster exists
if kind get clusters 2>/dev/null | grep -q "^${E2E_CLUSTER_NAME}$"; then
    cyber_log "Deleting kind cluster: ${CYBER_G}${E2E_CLUSTER_NAME}${CYBER_X}"
    kind delete cluster --name "${E2E_CLUSTER_NAME}"
    cyber_ok "Cluster deleted"
else
    cyber_warn "Cluster ${CYBER_G}${E2E_CLUSTER_NAME}${CYBER_X} does not exist"
fi

# Remove kubeconfig file
if [ -f "${E2E_KUBECONFIG}" ]; then
    cyber_log "Removing kubeconfig: ${CYBER_G}${E2E_KUBECONFIG}${CYBER_X}"
    rm -f "${E2E_KUBECONFIG}"
    cyber_ok "Kubeconfig removed"
fi

cyber_ok "E2E cluster teardown complete"
