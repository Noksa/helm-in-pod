#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(dirname "$(realpath "$0")")/../scripts/common.sh"

cyber_step "Setting up kind cluster for e2e tests"

# Check if kind is installed
if ! command -v kind &>/dev/null; then
    cyber_err "kind is not installed"
    echo ""
    echo "Install kind:"
    echo "  macOS:   brew install kind"
    echo "  Linux:   curl -Lo ./kind https://kind.sigs.k8s.io/dl/latest/kind-linux-amd64 && chmod +x ./kind && sudo mv ./kind /usr/local/bin/"
    echo "  Windows: choco install kind"
    echo ""
    echo "Or visit: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
    exit 1
fi

# Check if cluster already exists
if kind get clusters 2>/dev/null | grep -q "^${E2E_CLUSTER_NAME}$"; then
    cyber_log "Cluster ${CYBER_G}${E2E_CLUSTER_NAME}${CYBER_X} already exists"
    # Export kubeconfig for existing cluster
    cyber_log "Exporting kubeconfig to ${CYBER_G}${E2E_KUBECONFIG}${CYBER_X}"
    kind export kubeconfig --name "${E2E_CLUSTER_NAME}" --kubeconfig "${E2E_KUBECONFIG}"
else
    cyber_log "Creating kind cluster: ${CYBER_G}${E2E_CLUSTER_NAME}${CYBER_X}"
    kind create cluster --name "${E2E_CLUSTER_NAME}" --kubeconfig "${E2E_KUBECONFIG}" --wait 60s
    cyber_ok "Cluster created with kubeconfig at ${CYBER_G}${E2E_KUBECONFIG}${CYBER_X}"
fi

# Apply inotify limits inside kind nodes (required for file watchers)
cyber_log "Applying inotify limits inside kind nodes..."
for node in $(kind get nodes --name "${E2E_CLUSTER_NAME}"); do
    docker exec "$node" sysctl -w fs.inotify.max_user_watches=524288  >/dev/null
    docker exec "$node" sysctl -w fs.inotify.max_user_instances=512    >/dev/null
    cyber_log "  ${CYBER_G}✔${CYBER_X} ${node}"
done
cyber_ok "inotify limits applied inside nodes"

# Verify cluster is accessible
cyber_log "Verifying cluster access"
if KUBECONFIG="${E2E_KUBECONFIG}" kubectl cluster-info &>/dev/null; then
    cyber_ok "Cluster is accessible"
else
    cyber_err "Failed to access cluster"
    exit 1
fi

# Display cluster info
cyber_log "Cluster information:"
KUBECONFIG="${E2E_KUBECONFIG}" kubectl cluster-info

cyber_ok "E2E cluster setup complete"
echo ""
echo "To use this cluster manually:"
echo "  ${CYBER_C}export KUBECONFIG=${E2E_KUBECONFIG}${CYBER_X}"
echo "  ${CYBER_C}kubectl get nodes${CYBER_X}"
