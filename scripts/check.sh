#!/usr/bin/env bash

# shellcheck disable=SC1091
source "$(dirname "$(realpath "$0")")/common.sh"

cyber_step "Linting & Formatting"

if ! command -v goimports &>/dev/null; then
    cyber_log "Installing goimports..."
    go install golang.org/x/tools/cmd/goimports@latest
fi

cyber_log "Running go mod tidy"
go mod tidy

cyber_log "Running go fmt"
go fmt ./...

cyber_log "Running goimports"
goimports -w .

cyber_log "Running go vet"
go vet ./...

cyber_log "Running modernize"
go run golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest -fix ./...

cyber_log "Running golangci-lint"
golangci-lint run

cyber_ok "All checks passed"
