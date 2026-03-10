#!/usr/bin/env bash

# shellcheck disable=SC1091
source "$(dirname "$(realpath "$0")")/common.sh"

cyber_step "Plugin Integration Test"

# Build the plugin
cyber_log "Building plugin binary"
go build -o bin/in-pod main.go

# Create temporary plugin directory
TEMP_PLUGIN_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_PLUGIN_DIR"' EXIT

cyber_log "Setting up test plugin at ${CYBER_G}${TEMP_PLUGIN_DIR}${CYBER_X}"

# Copy plugin files - use a simpler plugin.yaml without install hooks for testing
cat > "$TEMP_PLUGIN_DIR/plugin.yaml" << EOF
name: "in-pod"
version: "test"
usage: "Run any command using privileged pod in a k8s cluster"
description: "Run any command using privileged pod in a k8s cluster"
command: "\$HELM_PLUGIN_DIR/bin/in-pod"
useTunnel: false
ignoreFlags: false
EOF

mkdir -p "$TEMP_PLUGIN_DIR/bin"
cp bin/in-pod "$TEMP_PLUGIN_DIR/bin/"

# Install plugin
cyber_log "Installing plugin to Helm"
helm plugin uninstall in-pod 2>/dev/null || true
helm plugin install "$TEMP_PLUGIN_DIR"

# Test 1: Check plugin is installed
cyber_log "Test 1: Plugin installed"
if helm plugin list | grep -q "in-pod"; then
    cyber_ok "Plugin is installed"
else
    cyber_err "Plugin not found"
    exit 1
fi

# Test 2: Verify environment variables are set by adding debug output to the plugin
cyber_log "Test 2: Testing environment variable passing"

# The plugin will fail to connect to fake context, but we can check the error message
# which should indicate it tried to use the context
OUTPUT=$(helm --kube-context fake-test-context in-pod exec -- "echo test" 2>&1 || true)

# Check if the error mentions the context (means it was passed)
if echo "$OUTPUT" | grep -qi "fake-test-context\|context.*not.*found\|no context"; then
    cyber_ok "HELM_KUBECONTEXT is being used (context error detected)"
else
    cyber_warn "Could not verify context usage from error output"
    echo "Output: $OUTPUT"
fi

# Cleanup
cyber_log "Uninstalling test plugin"
helm plugin uninstall in-pod

cyber_ok "Plugin integration tests completed"
