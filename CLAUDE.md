# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`helm-in-pod` is a Helm 4 plugin that runs any command (helm, kubectl, etc.) inside a Kubernetes cluster pod to minimize network latency. The binary is `bin/in-pod`, registered via `plugin.yaml` as a Helm CLI plugin named `in-pod`.

## Commands

```bash
make lint           # Full check: tidy, fmt, goimports, vet+modernize (with e2e tags), golangci-lint
make test           # Ginkgo unit tests (skips e2e)
make test-verbose   # Unit tests with verbose output
make test-coverage  # Tests with HTML coverage report
make test-focus FOCUS="pattern"  # Run focused unit tests matching pattern
make build          # Build bin/in-pod
make install-local  # Build and install plugin locally (use: helm in-pod ...)
make test-e2e       # E2E tests against existing kind cluster (reuses if helm-in-pod-e2e exists)
make test-e2e-full  # Full e2e: setup kind cluster, run tests, teardown
```

Run order: `make lint && make test && make test-e2e`.

To run a single Ginkgo test file: `go run github.com/onsi/ginkgo/v2/ginkgo --focus "pattern" -r --skip-package=e2e`

## Architecture

### Entry point flow
`main.go` → `cmd.ExecuteRoot()` → `internal.RunCommand(rootCmd)` (sets context with `--timeout` and SIGINT/SIGTERM handling) → Cobra command tree.

### Package layout

| Package | Purpose |
|---|---|
| `cmd/` | Cobra command definitions: `root.go`, `exec.go`, `purge.go`, `daemon*.go`, `flags.go`, `table.go` |
| `internal/` | Top-level glue: `vars.go` (global `namespace`/`pod` managers, `InitManagers`), `run.go` (context+signal setup) |
| `internal/hippod/` | Core pod logic: `spec.go` (pod spec builder), `executor.go` (exec/copy/sync), `pod.go` (CRUD), `pdb.go` (PodDisruptionBudget), `terminal.go` (interactive shell) |
| `internal/hipns/` | Namespace + RBAC setup (`PrepareNs`) |
| `internal/cmdoptions/` | Flag structs: `ExecOptions`, `DaemonOptions`, `PurgeOptions`, `MaskOptions` |
| `internal/helmtar/` | Tar bundle creation for atomic file copy to pod |
| `internal/hipconsts/` | Shared constants (namespace, labels, annotations, env var names, sentinel paths) |
| `internal/hipembedded/` | `script.sh` embedded into the binary; it runs inside the pod, waits for `/tmp/hip-wrapped-script.sh`, executes it, and handles signal propagation |
| `internal/hiperrors/` | Exit code extraction from wrapped command errors |
| `internal/hipretry/` | Context-aware retry helper |
| `internal/logz/` | Zerolog setup with host/pod log prefixes |

### Exec flow (one-shot mode)
1. `PrepareNs` — creates `helm-in-pod` namespace, ServiceAccount, ClusterRoleBinding if missing.
2. `CreateHelmPod` — builds pod spec (image, resources, volumes, PDB) and waits for `Running`.
3. Bundle copy — tar of user files + wrapped user script (`hip-staged-script.sh`) + optional `repositories.yaml` is streamed into the pod atomically, then moved to trigger execution.
4. `SyncHelmRepositories` — copies helm repos and optionally runs `helm repo update`.
5. `ExecuteCommand` — streams stdout/stderr; propagates exit code via `hiperrors`.
6. `CopyFileFromPod` (if `--copy-from`) — copies artifacts back; signals pod via `/tmp/copy-done`.
7. Cleanup — `DeleteHelmPods` unless `--keep-pod`.

### Daemon mode
Daemon subcommands (`daemon start/stop/exec/shell/list/status`) reuse a long-running pod. `daemon start` runs the same pod creation flow but leaves the pod alive. `daemon exec` skips pod creation and streams commands directly into the existing pod. Pod identity is tracked by name label; `HELM_IN_POD_DAEMON_NAME` env var sets the default name.

### Key constants (hipconsts)
- Default namespace: `helm-in-pod` (overridable via `HELM_IN_POD_NAMESPACE`)
- Wrapped script path inside pod: `/tmp/hip-wrapped-script.sh`
- Staged paths during bundle copy: `/tmp/hip-staged-script.sh`, `/tmp/hip-repositories.yaml`
- Copy-from sentinel: `/tmp/copy-done`; exit code marker: `###HIP_EXIT_CODE:N###`

## Testing

- All tests use **Ginkgo v2** with `go run github.com/onsi/ginkgo/v2/ginkgo` (not the installed CLI, to avoid version mismatch).
- Unit tests live in `cmd/` and `internal/` packages; e2e tests in `e2e/` require `-tags=e2e`.
- E2E uses `e2e/setup-cluster.sh` to create a kind cluster named `helm-in-pod-e2e`, then `e2e/run-tests.sh`.
- E2E cleanup: `helm in-pod purge --all`.
- Focused e2e: `make test-e2e FOCUS="pattern"` or set env `FOCUS="pattern"`.
- `GINKGO_PROCS` controls parallel test workers (default: 5); `E2E_TIMEOUT` controls per-test timeout (default: 10m).

## Linting rules

- `golangci-lint` with `goimports` (local prefix: `github.com/noksa/helm-in-pod`), `gocritic`, `misspell` (US), `nolintlint`, `unconvert`, `unparam`, `ginkgolinter`, `zerologlint`.
- `errcheck` and `unparam` are excluded in `_test.go` files.
- Lint runs with `-tags=e2e` for the `e2e/` directory.
- `nolintlint` requires both an explanation and a specific linter name on `//nolint:` directives.

## Local dev

- After code changes: `make install-local` → test with `helm in-pod exec -- "kubectl get pods -A"`.
- Debug output: add `--verbose-logs`.
- The `.helm-plugin-dev/` directory is a local dev plugin symlink; `bin/in-pod` is the built binary.
- `HELM_KUBECONTEXT` overrides the active kube context; `HELM_IN_POD_DAEMON_NAME` sets default daemon name.
