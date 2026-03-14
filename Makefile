SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -euc

.DEFAULT_GOAL = help

# Cyberpunk theme cache
CYBER_CACHE := .cyber.sh
CYBER_URL := https://raw.githubusercontent.com/Noksa/install-scripts/main/cyberpunk.sh

# Project metadata
VERSION := $(shell grep 'version:' plugin.yaml | cut -d '"' -f 2)
GO_VERSION := $(shell go version | cut -d ' ' -f 3)

# Ginkgo
GINKGO_BIN := $(shell go env GOPATH)/bin/ginkgo

$(CYBER_CACHE):
	@curl -s $(CYBER_URL) > $(CYBER_CACHE)

.PHONY: help
help: $(CYBER_CACHE) ## Show help
	@source $(CYBER_CACHE) && { \
		echo ""; \
		echo -e "$${CYBER_D}╔═══════════════════════════════════════╗$${CYBER_X}"; \
		echo -e "$${CYBER_D}║$${CYBER_X}  $${CYBER_M}🚀$${CYBER_X} $${CYBER_B}$${CYBER_C}Helm In Pod$${CYBER_X}"; \
		echo -e "$${CYBER_D}╚═══════════════════════════════════════╝$${CYBER_X}"; \
		echo -e "$${CYBER_D}│$${CYBER_X} $${CYBER_W}Version:$${CYBER_X} $${CYBER_G}$(VERSION)$${CYBER_X}"; \
		echo -e "$${CYBER_D}│$${CYBER_X} $${CYBER_W}Go:$${CYBER_X} $${CYBER_G}$(GO_VERSION)$${CYBER_X}"; \
		echo ""; \
	}
	@awk 'BEGIN {FS = ":.*##"; printf "\n\033[36mUsage:\033[0m make \033[35m<target>\033[0m\n\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m \033[37m%s\033[0m\n", $$1, $$2 } /^##@/ { printf "\n\033[35m⚡ %s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: cyber-update
cyber-update: ## Update cyberpunk theme
	@rm -f $(CYBER_CACHE)
	@curl -s $(CYBER_URL) > $(CYBER_CACHE)
	@source $(CYBER_CACHE) && cyber_ok "Cyberpunk theme updated"

##@ Development

.PHONY: lint
lint: ## Run linters and formatters
	@./scripts/check.sh

.PHONY: tidy
tidy: $(CYBER_CACHE) ## Tidy go modules
	@source $(CYBER_CACHE) && cyber_log "Tidying go modules"
	@go mod tidy
	@source $(CYBER_CACHE) && cyber_ok "Modules tidied"

.PHONY: install-local
install-local: ## Build and install plugin locally for testing
	@./scripts/install-local.sh

.PHONY: install
install: ## Uninstall and install specific version (use VERSION=xxx, e.g., VERSION=main or VERSION=v0.6.0-beta)
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION is required. Usage: make install VERSION=main"; \
		exit 1; \
	fi
	@echo "Uninstalling existing helm-in-pod plugin (if exists)..."
	@helm plugin uninstall in-pod 2>/dev/null || true
	@echo "Installing helm-in-pod version: $(VERSION)"
	@helm plugin install https://github.com/Noksa/helm-in-pod --version=$(VERSION) --verify=false
	@echo "Successfully installed helm-in-pod $(VERSION)"

##@ Testing

$(GINKGO_BIN):
	@go install github.com/onsi/ginkgo/v2/ginkgo@latest

.PHONY: test-unit
test-unit: $(GINKGO_BIN) ## Run unit tests with Ginkgo
	@$(GINKGO_BIN) -r --silence-skips --skip-package=e2e

.PHONY: test-verbose
test-verbose: $(GINKGO_BIN) ## Run unit tests with verbose output
	@$(GINKGO_BIN) -r -v --skip-package=e2e

.PHONY: test-coverage
test-coverage: $(GINKGO_BIN) $(CYBER_CACHE) ## Run tests with coverage
	@$(GINKGO_BIN) -r --cover --coverprofile=coverage.out --skip-package=e2e
	@go tool cover -html=coverage.out -o coverage.html
	@source $(CYBER_CACHE) && cyber_ok "Coverage report: $${CYBER_G}coverage.html$${CYBER_X}"

.PHONY: test-plugin
test-plugin: ## Test plugin as Helm plugin (integration test)
	@./scripts/test-plugin.sh

.PHONY: test-e2e-setup
test-e2e-setup: $(CYBER_CACHE) ## Setup kind cluster for e2e tests
	@./e2e/setup-cluster.sh

.PHONY: test-e2e-teardown
test-e2e-teardown: $(CYBER_CACHE) ## Teardown kind cluster for e2e tests
	@./e2e/teardown-cluster.sh

.PHONY: test-e2e
test-e2e: $(GINKGO_BIN) $(CYBER_CACHE) ## Run e2e tests (use FOCUS="pattern" or GINKGO_ARGS="..." to filter tests)
	@if [ -n "$(FOCUS)" ]; then \
		./e2e/run-tests.sh --focus="$(FOCUS)" $(GINKGO_ARGS); \
	else \
		./e2e/run-tests.sh $(GINKGO_ARGS); \
	fi

.PHONY: test-e2e-serial
test-e2e-serial: $(GINKGO_BIN) $(CYBER_CACHE) ## Run e2e tests serially (use FOCUS="pattern" or GINKGO_ARGS="..." to filter)
	@if [ -n "$(FOCUS)" ]; then \
		./e2e/run-tests.sh --procs=1 --focus="$(FOCUS)" $(GINKGO_ARGS); \
	else \
		./e2e/run-tests.sh --procs=1 $(GINKGO_ARGS); \
	fi

.PHONY: test-e2e-focus
test-e2e-focus: $(GINKGO_BIN) ## Alias for test-e2e with FOCUS (use FOCUS="pattern" or GINKGO_ARGS="...")
	@./e2e/run-tests.sh --focus="$(FOCUS)" $(GINKGO_ARGS)

.PHONY: test-e2e-full
test-e2e-full: test-e2e-setup test-e2e test-e2e-teardown ## Full e2e flow: setup, test, teardown

.PHONY: k9s
k9s: ## Run k9s with e2e cluster kubeconfig
	@if [ ! -f e2e/.kubeconfig ]; then \
		echo "E2E kubeconfig not found. Run: make test-e2e-setup"; \
		exit 1; \
	fi
	@KUBECONFIG=e2e/.kubeconfig k9s

.PHONY: test-all
test-all: test-unit test-e2e ## Run all tests (unit + e2e)

##@ Build

.PHONY: build
build: $(CYBER_CACHE) ## Build binary for current platform
	@source $(CYBER_CACHE) && cyber_log "Building binary"
	@go build -o bin/in-pod main.go
	@source $(CYBER_CACHE) && cyber_ok "Binary: $${CYBER_G}bin/in-pod$${CYBER_X}"

.PHONY: binaries
binaries: ## Build release binaries for all platforms
	@./scripts/make_archieve.sh

##@ Cleanup

.PHONY: clean
clean: $(CYBER_CACHE) ## Clean build artifacts
	@source $(CYBER_CACHE) && cyber_log "Cleaning up"
	@rm -rf bin/ generated/ coverage.out coverage.html
	@source $(CYBER_CACHE) && cyber_ok "Cleaned"

