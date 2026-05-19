# AGENTS.md

## Commands

- `make lint` — full check via `scripts/check.sh` (tidy, fmt, goimports, vet+modernize `-tags=e2e`, golangci-lint). **Run first**.
- `make test` / `make test-unit` — Ginkgo unit tests (skips e2e).
- `make test-e2e` — E2E against kind (reuses `helm-in-pod-e2e` if present). Use `FOCUS="pattern"`.
- `make test-e2e-full` — setup + test + teardown.
- `make install-local` — build + install plugin locally for `helm in-pod ...` testing.
- `make build` — builds `bin/in-pod`.

**Run order**: `make lint && make test && make test-e2e` (or at minimum `make lint` before committing).

Use `make help` for the full list.

## Testing

- Ginkgo v2 everywhere. Unit tests live in `cmd/`, e2e in `e2e/` (requires `-tags=e2e` for lint/vet).
- E2E uses `e2e/setup-cluster.sh` + kind and creates the `helm-in-pod` namespace + `cluster-admin` ServiceAccount.
- Feature-focused e2e specs: `daemon`, `copy`, `volumes`, `active-deadline`, etc.
- Always clean up with `helm in-pod purge --all` after e2e runs.

## Architecture

- Helm plugin entrypoint: `plugin.yaml` → `bin/in-pod`.
- Cobra commands: `cmd/` (`root.go`, `exec.go`, `daemon/*.go`, `purge.go`).
- Core logic: `internal/`.
- Daemon mode reuses a long-running pod — see `DAEMON.md`.
- Logging uses Zerolog with host/pod formatting.
- Exit codes from the inner command are propagated to the host.

## Local Development

- After `make install-local`, test with: `helm in-pod exec -- "kubectl get pods -A"`.
- Debug with `--verbose-logs`.
- Set `HELM_IN_POD_DAEMON_NAME` to avoid repeating `--name` on every daemon command.
- Local install hook: `scripts/install-local.sh` (release installs use `scripts/install.sh`).

## References

- `DAEMON.md`, `RELEASE_NOTES.md`, `e2e/`
- CI runs: lint → unit → e2e on multiple Kubernetes versions.
