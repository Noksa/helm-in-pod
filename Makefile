SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -euc

.DEFAULT_GOAL = help

# Colors for pretty output
CYAN := \033[36m
YELLOW := \033[1;33m
GREEN := \033[32m
RED := \033[31m
BLUE := \033[1;34m
MAGENTA := \033[35m
RESET := \033[0m

# Project metadata
VERSION := $(shell grep 'version:' plugin.yaml | cut -d '"' -f 2)
GO_VERSION := $(shell go version | cut -d ' ' -f 3)

##@ 🎯 Help & Information
.PHONY: help
help: ## Show this help message with available targets
	@printf "\n$(BLUE)╔══════════════════════════════════════════════════════════════╗$(RESET)\n"
	@printf "$(BLUE)║  🚀 Helm In Pod - Run Helm commands inside Kubernetes pods  ║$(RESET)\n"
	@printf "$(BLUE)╚══════════════════════════════════════════════════════════════╝$(RESET)\n"
	@printf "\n$(MAGENTA)📦 Version:$(RESET) $(VERSION)  $(MAGENTA)🔧 Go:$(RESET) $(GO_VERSION)\n"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(CYAN)%-20s$(RESET) %s\n", $$1, $$2 } /^##@/ { printf "\n$(YELLOW)%s$(RESET)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
	@printf "\n"

.PHONY: version
version: ## Show current version
	@printf "$(GREEN)Version: $(VERSION)$(RESET)\n"

##@ 🛠️  Development
.PHONY: lint
lint: ## Run linters and formatters
	@printf "$(BLUE)🔍 Running linters and formatters...$(RESET)\n"
	@./scripts/check.sh
	@printf "$(GREEN)✓ Linting complete!$(RESET)\n"

.PHONY: tidy
tidy: ## Tidy go modules
	@printf "$(BLUE)📦 Tidying go modules...$(RESET)\n"
	@go mod tidy
	@printf "$(GREEN)✓ Modules tidied!$(RESET)\n"

##@ 🧪 Testing
.PHONY: test
test: ## Run tests
	@printf "$(BLUE)🧪 Running tests...$(RESET)\n"
	@go test -v ./...
	@printf "$(GREEN)✓ Tests passed!$(RESET)\n"

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@printf "$(BLUE)📊 Running tests with coverage...$(RESET)\n"
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@printf "$(GREEN)✓ Coverage report generated: coverage.html$(RESET)\n"

##@ 🏗️  Build
.PHONY: build
build: ## Build binary for current platform
	@printf "$(BLUE)🔨 Building binary...$(RESET)\n"
	@go build -o bin/in-pod main.go
	@printf "$(GREEN)✓ Binary built: bin/in-pod$(RESET)\n"

.PHONY: binaries
binaries: version ## Build release binaries for all platforms
	@printf "$(BLUE)📦 Building release binaries for all platforms...$(RESET)\n"
	@cd scripts && ./make_archieve.sh
	@printf "$(GREEN)✓ Binaries created in generated/$(RESET)\n"
	@ls -lh generated/

##@ 🧹 Cleanup
.PHONY: clean
clean: ## Clean build artifacts and generated files
	@printf "$(BLUE)🧹 Cleaning up...$(RESET)\n"
	@rm -rf bin/ generated/ coverage.out coverage.html
	@printf "$(GREEN)✓ Cleaned!$(RESET)\n"
