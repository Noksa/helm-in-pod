#!/usr/bin/env bash

# shellcheck disable=SC1091
source "$(dirname "$(realpath "$0")")/common.sh"

cyber_step "Install Plugin Locally"

cyber_log "Building binary"
go build -o bin/in-pod main.go

cyber_log "Uninstalling existing plugin"
helm plugin uninstall in-pod 2>/dev/null || true

cyber_log "Installing plugin from ${CYBER_G}${PROJECT_DIR}${CYBER_X}"

# Create a minimal plugin.yaml without install hooks in a persistent location
INSTALL_DIR="${PROJECT_DIR}/.helm-plugin-dev"
rm -rf "$INSTALL_DIR"
mkdir -p "$INSTALL_DIR/bin"

cat > "$INSTALL_DIR/plugin.yaml" << EOF
name: "in-pod"
version: "0.0.0-dev"
usage: "Run any command using privileged pod in a k8s cluster"
description: "Run any command using privileged pod in a k8s cluster"
command: "\$HELM_PLUGIN_DIR/bin/in-pod"
useTunnel: false
ignoreFlags: false
EOF

cp bin/in-pod "$INSTALL_DIR/bin/"

helm plugin install "$INSTALL_DIR"

cyber_ok "Plugin installed successfully"
cyber_log "Test with: ${CYBER_C}helm in-pod exec -- kubectl get pods${CYBER_X}"
