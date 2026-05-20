#!/usr/bin/env bash

# shellcheck disable=SC1091
source "$(dirname "$(realpath "$0")")/common.sh"

cyber_step "Linting & Formatting"

if ! command -v goimports &>/dev/null; then
    cyber_log "Installing goimports..."
    go install golang.org/x/tools/cmd/goimports@latest
fi

if ! command -v golangci-lint &>/dev/null; then
    cyber_log "Installing golangci-lint..."
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
fi

cyber_log "Running go mod tidy"
go mod tidy

cyber_log "Running go fmt"
go fmt ./...

cyber_log "Running goimports"
goimports -w .

cyber_log "Running go vet"
go vet ./...
go vet -tags=e2e ./e2e/

cyber_log "Running modernize"
go run golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest -fix ./...
# Modernize's own -tags flag is a no-op (deprecated); use GOFLAGS to pass build tags
# to the underlying go/packages loader so e2e-tagged files are actually analyzed.
GOFLAGS='-tags=e2e' go run golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest -fix ./...

cyber_log "Running golangci-lint"
golangci-lint run
golangci-lint run --build-tags=e2e ./e2e/

cyber_ok "All checks passed"
