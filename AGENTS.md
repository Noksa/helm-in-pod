# AGENTS.md

## Development Commands

- `make install-local`: Build + install plugin locally (`helm in-pod ...`). Run after changes for testing.
- `make lint`: Full verification (go mod tidy, fmt, goimports, vet, modernize, golangci-lint). Includes e2e build tag. Run first.
- `make test` / `make test-unit`: Ginkgo unit tests (skips e2e).
- `make test-e2e`: E2E tests against kind cluster. Use `FOCUS="pattern"` for specific tests. Reuses cluster if exists.
- `make test-e2e-full`: Full e2e cycle (setup, test, teardown).
- `make build`: Builds `bin/in-pod`.

Run order: `make lint && make test && make test-e2e`.

Use `make help` for all targets.

## Testing Quirks

- Ginkgo v2 for all tests. Unit in `cmd/`, e2e in `e2e/` (requires `-tags=e2e` for lint/vet).
- E2E: Uses `e2e/setup-cluster.sh` + kind. Creates `helm-in-pod` namespace + `cluster-admin` ServiceAccount.
- Focused e2e files for features: daemon, copy, volumes, active-deadline, etc.
- Cleanup e2e: `helm in-pod purge --all`.

## Architecture

- Helm plugin: `plugin.yaml` points to `bin/in-pod`.
- Cobra commands: `cmd/` (root.go, exec.go, daemon/*.go, purge.go).
- Core logic: `internal/`.
- Daemon: Reuses long-running pod. See DAEMON.md.
- Logging: Zerolog with host/pod formatting.
- Exit codes: Propagated from inner command.

## Local Dev Gotchas

- Test after `make install-local`: `helm in-pod exec -- "kubectl get pods -A"`.
- Debug: `--verbose-logs`.
- Daemon: Set `HELM_IN_POD_DAEMON_NAME` env var.
- Install hook: `scripts/install.sh` downloads release; local dev uses `install-local.sh`.

## After Making Changes

- Run `make lint` at minimum; ensure it passes.
- For significant changes, run `make test` and verify success.
- Also run `make test-e2e` to catch issues early (will fail in MR anyway).

## Verification

- Always `make lint` before committing.
- CI: lint → unit → e2e on multiple k8s versions.

See DAEMON.md, RELEASE_NOTES.md, e2e/ for details.
