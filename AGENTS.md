# AGENTS.md

## Development Commands

- `make install-local`: Build + install plugin locally for testing (`helm in-pod ...`). Always run this after changes before manual testing.
- `make lint`: Full verification (go mod tidy, fmt, goimports, vet, modernize, golangci-lint). **Run first**. Includes e2e build tag.
- `make test` / `make test-unit`: Ginkgo unit tests (skips e2e package).
- `make test-e2e`: E2E tests against kind cluster (`helm-in-pod-e2e`). Use `FOCUS="pattern"` to run specific tests.
- `make test-e2e-full`: Full cycle (setup, test, teardown).
- `make build`: Builds `bin/in-pod`.

Run in this order: `make lint && make test && make test-e2e` (or subset).

Use `make help` for all targets.

## Testing Quirks

- **Ginkgo v2** everywhere. Tests in `cmd/` (unit) + `e2e/` (integration against real k8s).
- Build tag `e2e` required for e2e package (`-tags=e2e` in vet/lint).
- E2E uses `e2e/setup-cluster.sh` + kind. Cluster is reused if present (`make test-e2e-prepare`).
- Many focused test files (`*_test.go`) for specific features (daemon, copy, volumes, active-deadline, etc.).
- Tests create `helm-in-pod` namespace + `cluster-admin` ServiceAccount. Cleanup via `helm in-pod purge --all`.

## Architecture

- Helm plugin (`plugin.yaml` points to `bin/in-pod`).
- Cobra commands in `cmd/` (`root.go`, `exec.go`, `daemon/*.go`, `purge.go`).
- Core Kubernetes/pod logic in `internal/`.
- Daemon mode (`daemon start/exec/shell`) reuses long-running pod for speed.
- Logging via zerolog with custom host/pod formatting.
- Exit codes from inner command are propagated exactly.

## Local Dev Gotchas

- After `make install-local`, test with `helm in-pod exec -- "kubectl get pods -A"`.
- Use `--verbose-logs` for debug output.
- `HELM_IN_POD_DAEMON_NAME` env var for daemon workflows.
- Plugin hooks run `scripts/install.sh` on `helm plugin install` (downloads release). Local dev bypasses this via `install-local.sh`.

## Verification

Always run `make lint` before committing. CI runs lint → unit → e2e on multiple k8s versions.

See `DAEMON.md`, `RELEASE_NOTES.md`, and `e2e/` for deeper feature specifics.
