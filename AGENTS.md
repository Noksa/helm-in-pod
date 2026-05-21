# helm-in-pod KNOWLEDGE BASE

**Generated:** 2026-05-19 | **Commit:** `ef2c015` | **Branch:** `main`

## OVERVIEW

Helm 4 plugin (`in-pod`) that runs any command (helm, kubectl, etc.) inside a long-lived or one-shot Kubernetes pod to minimize host↔API latency. Pure Go binary at `bin/in-pod`, registered through `plugin.yaml`. Talks to the cluster via the controller-runtime client wrapper from `Noksa/operator-home`, streams stdout/stderr back through `kubectl exec`-style pod exec, and propagates the inner command's exit code.

## STRUCTURE

```
helm-in-pod/
├── main.go              # zerolog setup + cmd.ExecuteRoot + exit-code propagation
├── plugin.yaml          # Helm plugin manifest (subprocess runtime → bin/in-pod)
├── public-key.asc       # PGP key used by Helm 4 to verify the signed plugin tarball
├── Makefile             # cyberpunk-themed targets; canonical workflow
├── cmd/                 # Cobra commands (see cmd/AGENTS.md)
├── internal/            # Glue + private packages (see internal/AGENTS.md)
├── e2e/                 # Build-tagged e2e suite (see e2e/AGENTS.md)
├── scripts/             # check.sh, install*.sh, make_archieve.sh, test-plugin.sh
├── docs/RELEASING.md    # Manual release procedure (tag, notes, GHCR push)
├── DAEMON.md            # Daemon-mode user docs (`daemon start/exec/shell/...`)
├── CLAUDE.md            # Short AI-assistant quick reference; full content is in AGENTS.md
├── .github/workflows/   # ci.yml (lint+unit+e2e matrix), release.yml (build+sign+OCI push)
├── .golangci.yml        # v2 config; `build-tags: [e2e]` is set globally
└── .helm-plugin-dev/    # local-only plugin symlink target (created by install-local.sh)
```

## WHERE TO LOOK

| Task                                           | Location                                                   |
|------------------------------------------------|------------------------------------------------------------|
| Add/modify a CLI subcommand or flag            | `cmd/` → see [cmd/AGENTS.md](cmd/AGENTS.md)                |
| Change pod spec, exec/copy, PDB, terminal      | `internal/hippod/` → see [internal/hippod/AGENTS.md]       |
| Change a flag struct or env-file parsing       | `internal/cmdoptions/`                                     |
| Change cluster bootstrap (namespace/SA/CRB)    | `internal/hipns/namespace.go`                              |
| Edit the in-pod shell wrapper                  | `internal/hipembedded/script.sh` (`go:embed` via `script.go`) |
| Tar-bundle layout for atomic file copy         | `internal/helmtar/`                                        |
| Retry logic                                    | `internal/hipretry/`                                       |
| Exit-code parsing from exec stderr             | `internal/hiperrors/exitcode.go`                           |
| Add/run an e2e spec                            | `e2e/` → see [e2e/AGENTS.md](e2e/AGENTS.md)                |
| Build/release tweaks                           | `Makefile`, `.github/workflows/release.yml`, `scripts/make_archieve.sh` |
| Lint config / global build tags                | `.golangci.yml`, `scripts/check.sh`                        |

## CODE MAP

| Symbol                              | Type           | Location                                       | Role                                                         |
|-------------------------------------|----------------|------------------------------------------------|--------------------------------------------------------------|
| `main`                              | func           | `main.go:16`                                   | zerolog console writer with `[host]`/`[pod]` colored sources |
| `cmd.ExecuteRoot`                   | func           | `cmd/root.go:56`                               | builds root + subcommands, calls `internal.RunCommand`       |
| `cmd.newExecCmd` / `newDaemonCmd`   | func           | `cmd/exec.go`, `cmd/daemon.go`                 | one-shot and daemon entry points                             |
| `internal.RunCommand`               | func           | `internal/run.go:15`                           | parses `--timeout`, wires SIGINT/SIGTERM → context           |
| `internal.InitManagers`             | func           | `internal/vars.go:31`                          | builds kube client from `HELM_KUBECONTEXT`, sets pkg vars    |
| `internal.UseCommandContext`        | func           | `internal/vars.go:67`                          | swaps managers to Cobra-context copies (must be called per-cmd) |
| `hippod.Manager`                    | struct         | `internal/hippod/pod.go:40`                    | owns pod CRUD + exec + copy; carries `invocationID` UUID     |
| `hippod.Manager.CreateHelmPod`      | method         | `internal/hippod/pod.go`                       | builds spec, applies, waits for Running                      |
| `hippod.Manager.CopyFilesBundleWithBootInfo` | method | `internal/hippod/pod.go`                       | atomic tar copy + collects HOME/user/helm version in one exec |
| `hippod.Manager.ExecuteCommand`     | method         | `internal/hippod/executor.go`                  | streams stdout/stderr; extracts `###HIP_EXIT_CODE:N###` marker |
| `hippod.Manager.SignalCopyDone`     | method         | `internal/hippod/executor.go`                  | drops `/tmp/copy-done` so the pod's `script.sh` can exit     |
| `hipns.Manager.PrepareNs`           | method         | `internal/hipns/namespace.go:44`               | ensures namespace + SA + `cluster-admin` CRB exist + effective |
| `hipembedded.GetShScript`           | func           | `internal/hipembedded/script.go`               | exposes embedded `script.sh` (waits for wrapped script, handles signals) |
| `hiperrors.ExitCodeError`           | struct         | `internal/hiperrors/exitcode.go`               | typed error checked by `main.go` to set `os.Exit(N)`         |
| `hipconsts` constants               | vars/consts    | `internal/hipconsts/consts.go`                 | namespace, container name, sentinel paths, env-var names, marker strings |

## CONVENTIONS

- **Module path & local prefix**: `github.com/noksa/helm-in-pod`. `goimports` is configured with this prefix in `.golangci.yml`.
- **Go**: 1.26 (see `go.mod`, `go-version: '1.26'` in both CI workflows).
- **Cobra subcommands**: each lives in its own file (`cmd/<verb>.go`); flags split into `addPodCreationFlags` + `addRuntimeFlags` so daemon variants reuse them.
- **Manager pattern**: `hippod.Manager`/`hipns.Manager` hold `ctx` + optional `kclient`. Always call `UseCommandContext(cmd.Context())` inside `RunE` so retries respect `--timeout` and signals — `cmd/exec.go:50` and `cmd/daemon_start.go:43` are the reference call sites.
- **Logging**: never use `fmt.Println` for status; use `logz.Host()` or `logz.Pod()` (zerolog with colored `[host]`/`[pod]` source fields). Source `host+pod` exists for joint actions.
- **All retries** go through `hipretry.RetryWithContext` (context-aware backoff).
- **Constants are central**: never hard-code `"helm-in-pod"`, `/tmp/hip-*`, or `HELM_IN_POD_*` env names — reach for `internal/hipconsts/consts.go`.
- **Errors that should set a non-zero exit code**: wrap as `*hiperrors.ExitCodeError`. `main.go:44` recognizes the type via `errors.AsType` and calls `os.Exit(int(exitErr.Code))`.

## ANTI-PATTERNS (THIS PROJECT)

- **Do not call `internal/hipembedded` from anywhere except `internal/hippod`** — it must stay a leaf so the embedded script has zero import-cycle risk.
- **Do not mutate `hipconsts.Namespace`** outside `internal.InitManagers` (it is a package-level `var` only because of `HELM_IN_POD_NAMESPACE`).
- **Never use the installed `ginkgo` CLI** in scripts or Make targets. Always `go run github.com/onsi/ginkgo/v2/ginkgo` to pin the version from `go.mod` (avoids the "CLI version mismatch" runtime warning).
- **Never delete a Running/Pending pod with `GracePeriodSeconds=0`**. `internal/hippod/pod.go` only force-deletes Succeeded/Failed pods on purpose — kubelet needs the default grace period to clean container state.
- **Do not bypass `purge` semantics**: `purge` is host-scoped (matches `myHostname`); `purge --all` is namespace-wide. Mixing these in tests created the old "orphaned PDB" class of bugs.
- **Do not delete failing e2e specs** to make CI green; `e2e-reports/` artifacts upload only on failure and are how the maintainer debugs.
- **`//nolint:` requires both an explanation and a specific linter name** (`nolintlint` is enabled with `require-explanation: true` and `require-specific: true`).

## UNIQUE STYLES

- **Cyberpunk theme** for `make help`, `scripts/check.sh`, `scripts/install-local.sh`, `e2e/setup-cluster.sh` etc. Source the cached `.cyber.sh` (auto-downloaded by Makefile from `Noksa/install-scripts`) before using `cyber_log`/`cyber_ok`/`cyber_err`/`cyber_step`.
- **`script.sh` boot protocol** (`internal/hipembedded/script.sh`): after pod boot the container `tail`-waits for `/tmp/hip-wrapped-script.sh`, executes it, and if `WAIT_COPY_DONE` is set emits `###HIP_EXIT_CODE:N###` and blocks on `/tmp/copy-done`. The host parses the marker via `hiperrors.ExtractExitCode`.
- **Atomic bundle copy**: `helmtar` builds a single tar with `hip-staged-script.sh` + optional `hip-repositories.yaml` + user files. The pod's boot command moves staged files into place last, so a partial copy never triggers execution.
- **Per-process `invocationID`** (UUID) in `hippod.Manager` is added as a pod label so concurrent plugin processes never delete each other's pods.

## COMMANDS

```bash
# Canonical pre-commit order
make lint                      # tidy + fmt + goimports + vet + modernize (with and without -tags=e2e) + golangci-lint
make test                      # alias of test-unit: Ginkgo --skip-package=e2e
make test-e2e                  # reuses kind cluster `helm-in-pod-e2e` if present; FOCUS="..." filters
make test-e2e-full             # setup-cluster.sh → test-e2e → teardown-cluster.sh

# Local dev loop
make install-local             # builds bin/in-pod, installs into .helm-plugin-dev/ (no install hooks)
helm in-pod exec -- "kubectl get pods -A"
helm in-pod exec --verbose-logs -- "..."

# Coverage / focus
make test-coverage             # generates coverage.html
make test-focus FOCUS="copy"   # focused unit tests
FOCUS="daemon" make test-e2e   # focused e2e
GINKGO_PROCS=1 make test-e2e   # serial e2e (default 5 parallel processes)

# Release artifacts
make build                     # bin/in-pod with ldflags (version/commit/date)
make binaries TARGET=linux/amd64
make help                      # cyberpunk-themed target list
```

## NOTES

- **Two install scripts** look similar but aren't: `scripts/install.sh` is what end users run via Helm plugin install (release path), `scripts/install-local.sh` is dev-only and writes a stripped `plugin.yaml` to `.helm-plugin-dev/` without install hooks.
- **`coverage.html` / `coverage.out` / `inpod` / `inpodw`** at the repo root are build/test artifacts — don't commit them (they're in `.gitignore`).
- **Release pipeline** (`.github/workflows/release.yml`): per-platform binary build → PGP-signed plugin tarball via `helm plugin package --sign` → push to GHCR via `oras` → GitHub release with auto notes. The release notes in `RELEASE_NOTES.md` are pasted manually after the workflow runs (see `docs/RELEASING.md`).
- **E2E matrix**: K8s `v1.28.15`, `v1.30.8`, `v1.32.3`, `v1.35.1`. Kind cluster is reused between runs unless you call `make test-e2e-teardown` explicitly.
- **Helm 3** is unsupported in this codebase since v0.9.0 (`go.mod` pins `helm.sh/helm/v4`). The README points Helm 3 users to v0.8.1.
