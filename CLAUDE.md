# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

The comprehensive knowledge base for this repository lives in [AGENTS.md](AGENTS.md). It covers the project overview, package layout, code map, conventions, anti-patterns, and all relevant commands.

## Quick reference

```bash
make lint           # Full check: tidy, fmt, goimports, vet+modernize, golangci-lint
make test           # Ginkgo unit tests (skips e2e)
make build          # Build bin/in-pod
make install-local  # Build and install plugin locally for testing
make test-e2e-full  # Full e2e: setup kind cluster, run tests, teardown
```

Run order: `make lint && make test && make test-e2e`.
