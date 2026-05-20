# Codebase Structure

**Analysis Date:** 2026-05-20

## Directory Layout

```
helm-in-pod/
├── main.go                          # Process entry: zerolog setup, ExecuteRoot, exit-code mapping
├── plugin.yaml                      # Helm 4 subprocess plugin manifest (points to bin/in-pod)
├── public-key.asc                   # PGP key used to verify the signed plugin tarball
├── go.mod / go.sum                  # Go 1.26 module; pins helm.sh/helm/v4 v4.0.4, k8s.io v0.35.0
├── Makefile                         # Cyberpunk-themed canonical workflow (lint/test/install-local/build)
├── README.md                        # User documentation
├── DAEMON.md                        # User docs for daemon-mode commands
├── AGENTS.md                        # Top-level curated knowledge map
├── CLAUDE.md                        # Legacy AI-assistant notes (kept for compat)
├── LICENSE                          # Apache-2.0
├── RELEASE_NOTES.md                 # Hand-edited notes for the next release
├── opencode.json                    # opencode workspace config
├── .golangci.yml                    # golangci-lint v2; build-tags: [e2e] set globally
├── .gitignore                       # Ignores bin/, *.html, coverage.out, .helm-plugin-dev/, etc.
│
├── cmd/                             # Cobra CLI declarations (one verb per file)
├── internal/                        # All orchestration; private to the module
│   ├── vars.go                      # Package singletons: pod, namespace + InitManagers/UseCommandContext
│   ├── run.go                       # RunCommand: --timeout parsing + SIGINT/SIGTERM wiring
│   ├── hippod/                      # Pod CRUD + exec/copy I/O + PDB + interactive shell (largest pkg)
│   ├── hipns/                       # Namespace + ServiceAccount + cluster-admin ClusterRoleBinding
│   ├── hipembedded/                 # go:embed-ed script.sh (leaf — only hippod may import)
│   ├── hipconsts/                   # Central constants (namespace var, env names, label keys, sentinels)
│   ├── hiperrors/                   # ExitCodeError typed error
│   ├── hipretry/                    # Context-aware retry with exponential backoff + jitter
│   ├── helmtar/                     # Multi-entry tar bundle builder for atomic file copy
│   ├── cmdoptions/                  # Flag structs + env-file parsing + secret masking
│   ├── helpers/                     # Misc helpers (IsCompletionCmd, helm-version, shell-out)
│   └── logz/                        # zerolog wrappers (Host / Pod / HostPod / Suppress)
│
├── e2e/                             # Build-tagged Ginkgo v2 suite + kind-cluster shell scripts
├── scripts/                         # check.sh, install.sh, install-local.sh, make_archieve.sh, test-plugin.sh, common.sh
├── docs/                            # RELEASING.md (manual release procedure)
├── assets/                          # banner.png
├── generated/                       # Generated artifacts (gitignored content)
├── bin/                             # Build output: bin/in-pod (gitignored)
├── e2e-reports/                     # Diagnostics dumped on e2e failure (gitignored)
├── .helm-plugin-dev/                # Dev-only stripped plugin install used by scripts/install-local.sh
├── .github/workflows/               # ci.yml (lint+unit+e2e matrix), release.yml (sign+OCI push), opencode.yml
├── .claude/ .agents/ .planning/     # Tooling/state directories (not user-edited)
└── coverage.out / coverage.html / inpod / inpodw   # Build/test artifacts (gitignored)
```

## Directory Purposes

**`/` (root):**
- Purpose: Module root with the binary entry point, plugin manifest, signing key, and top-level documentation.
- Contains: `main.go`, `plugin.yaml`, `go.mod`, `Makefile`, `README.md`, `DAEMON.md`, curated `AGENTS.md`, `RELEASE_NOTES.md`, license, golangci config.
- Key files: `main.go` (49 lines), `plugin.yaml` (19 lines), `Makefile`.

**`cmd/`:**
- Purpose: Cobra command tree. Business logic is **not** allowed here — every interesting branch lives in `internal/`.
- Contains: One file per verb plus `flags.go` and `table.go`. Tests are co-located as `*_test.go`.
- Key files: `cmd/root.go`, `cmd/exec.go`, `cmd/purge.go`, `cmd/daemon.go`, `cmd/daemon_*.go`, `cmd/flags.go`, `cmd/version.go`.

**`internal/`:**
- Purpose: All orchestration. Two files at the top (`vars.go`, `run.go`) form the glue layer; subdirectories are narrowly-scoped private packages.
- Contains: `vars.go`, `run.go`, and 10 subpackages.
- Key files: `internal/vars.go` (Manager singletons), `internal/run.go` (timeout + signal wiring).

**`internal/hippod/`:**
- Purpose: The pod-management core. Largest internal package (~4,270 LOC, 15 files).
- Contains: Manager struct, pod CRUD, daemon CRUD, atomic bundle copy, exec streaming, PDB CRUD, interactive shell, copy-from artifact retrieval.
- Key files: `pod.go` (833 lines), `executor.go` (627 lines), `spec.go` (319 lines), `pdb.go` (78 lines), `terminal.go` (32 lines).

**`internal/hipns/`:**
- Purpose: Bootstrap and cleanup of the `helm-in-pod` namespace + ServiceAccount + cluster-admin ClusterRoleBinding.
- Contains: A single `Manager` with `PrepareNs`, `CreateClusterRoleBinding`, `DeleteClusterRoleBinding`, `waitForClusterRoleBindingEffective`.
- Key files: `namespace.go` (138 lines).

**`internal/hipembedded/`:**
- Purpose: Hold the `go:embed`-ed shell wrapper that runs inside every pod.
- Contains: `script.go` exposes `GetShScript()`; `script.sh` is the actual bash boot wrapper.
- Key files: `script.sh` (66 lines), `script.go` (12 lines).

**`internal/hipconsts/`:**
- Purpose: Single source of truth for the namespace var, env-var names, label keys, sentinel paths, and exit-code marker strings.
- Contains: `Namespace var`, constants for `EnvDaemonName`, `EnvImage`, `EnvNamespace`, `LabelOperationID`, `LabelManagedBy`, `LabelKept`, `CopyFromDoneFile`, `CopyFromExitCodeMarkerPrefix/Suffix`, `EnvWaitCopyDone`, `WrappedScriptPath`, `StagedScriptPath`, `StagedRepoConfigPath`.
- Key files: `consts.go` (47 lines).

**`internal/hiperrors/`:**
- Purpose: The typed `ExitCodeError` that lets non-zero in-pod exits propagate through `fmt.Errorf("%w", …)` chains and reach `os.Exit(N)` in `main.go`.
- Contains: `ExitCodeError` struct + `Is` method.
- Key files: `exitcode.go` (27 lines).

**`internal/hipretry/`:**
- Purpose: The single retry primitive used by every kube-API call. Exponential backoff + jitter + transient/permanent classification.
- Contains: `RetryWithContext`, `RetryWithBackoff`, `BackoffConfig`, `DefaultBackoff`, `K8sAPIBackoff`, `IsPermanentError`, `IsTransientError`.
- Key files: `retry.go` (222 lines).

**`internal/helmtar/`:**
- Purpose: Build a single gzipped tar from `[]BundleEntry{Src, Dest}` for the atomic in-pod copy.
- Contains: `BundleEntry`, `CompressMulti`, internal `addToTar` walker.
- Key files: `tar.go` (~100 lines).

**`internal/cmdoptions/`:**
- Purpose: Flag struct definitions and their parsing helpers (env files, file mappings, secret masking).
- Contains: `ExecOptions`, `DaemonOptions` (embeds `ExecOptions`), `PurgeOptions`, env-file parser, `MaskSetValues` (secret masking for log output).
- Key files: `exec.go`, `daemon.go`, `purge.go`, `envfile.go`, `mask.go`.

**`internal/helpers/`:**
- Purpose: Small cross-cutting helpers.
- Contains: `IsCompletionCmd` (skip InitManagers for completion subcommands), helm version detection, shell-out wrapper.
- Key files: `cmd.go`, `helm.go`.

**`internal/logz/`:**
- Purpose: zerolog wrappers that attach a `source` field (`host`, `pod`, `host+pod`) so `main.go`'s console writer can color them.
- Contains: `Host()`, `Pod()`, `HostPod()` (each a `sync.OnceValue`), `Suppress()`.
- Key files: `log.go` (32 lines).

**`e2e/`:**
- Purpose: Ginkgo v2 end-to-end suite gated by `//go:build e2e`. Tests the real plugin against a kind cluster.
- Contains: `e2e_suite_test.go` (suite root), `e2e_test.go` (shared helpers), `utils.go` (`Run`, `BuildHelmInPodCommand`, kind helpers), 19 feature specs, three shell scripts (`setup-cluster.sh`, `run-tests.sh`, `teardown-cluster.sh`).
- Generated: No (committed).
- Committed: Yes.

**`scripts/`:**
- Purpose: Shell automation.
- Contains: `check.sh` (lint orchestrator), `install.sh` (production plugin install hook), `install-local.sh` (dev plugin install via `.helm-plugin-dev/`), `make_archieve.sh` (release tarball builder), `test-plugin.sh`, `common.sh` (sourced — cluster name, kubeconfig path).

**`docs/`:**
- Purpose: Project documentation.
- Contains: `RELEASING.md` (manual release procedure), `superpowers/` (additional docs).

**`bin/`:**
- Purpose: Build output.
- Contains: `bin/in-pod` after `make build`.
- Generated: Yes — gitignored.

**`.helm-plugin-dev/`:**
- Purpose: Dev-only plugin install target used by `scripts/install-local.sh` (writes a stripped `plugin.yaml` without install hooks).
- Generated: Yes — gitignored.

**`.github/workflows/`:**
- Purpose: CI / release automation.
- Contains: `ci.yml` (lint + unit + e2e matrix across K8s v1.28.15 / v1.30.8 / v1.32.3 / v1.35.1), `release.yml` (per-platform build, PGP sign, push to GHCR via `oras`, GitHub release), `opencode.yml`.

## Key File Locations

**Entry Points:**
- `main.go`: Process entry; configures zerolog, calls `cmd.ExecuteRoot`, maps `*hiperrors.ExitCodeError` to `os.Exit(N)`.
- `cmd/root.go`: `newRootCmd` + `ExecuteRoot`; persistent `--verbose-logs` and `--timeout`; calls `internal.InitManagers` in `PersistentPreRunE`.
- `internal/run.go`: `RunCommand` — parses `--timeout`, wraps in `signal.NotifyContext` + `context.WithTimeout`, calls `cmd.ExecuteContext(ctx)`.
- `internal/vars.go`: `InitManagers`, `Pod()`, `Namespace()`, `UseCommandContext`.

**Configuration:**
- `plugin.yaml`: Helm 4 subprocess plugin manifest; declares `bin/in-pod` as the platform command and `scripts/install.sh` as the install/update hook.
- `public-key.asc`: PGP key embedded in the published plugin tarball for `helm plugin install --verify`.
- `go.mod`: Go 1.26; pins `helm.sh/helm/v4 v4.0.4`, `k8s.io/api v0.35.0`, `sigs.k8s.io/controller-runtime v0.23.3`, `github.com/Noksa/operator-home`, `github.com/onsi/ginkgo/v2`.
- `.golangci.yml`: golangci-lint v2 with global `build-tags: [e2e]`.
- `Makefile`: Canonical workflow (`make lint`, `make test`, `make test-e2e`, `make install-local`, `make build`).

**Core Logic:**
- `internal/hippod/pod.go`: `Manager`, `CreateHelmPod`, `CreateDaemonPod`, `DeleteHelmPods`, `DeleteKeptPods`, `CopyFilesBundleWithBootInfo`, `CopyFileFromPod`, `StreamLogsFromPod`, `GetPodPhase`, `waitUntilPodIsRunning`, `waitUntilPodIsDeleted`, `AnnotatePod`, `OpenInteractiveShell`, `PrintPodSpecYAML`, `ListDaemonPods`, `GetDaemonStatus`.
- `internal/hippod/spec.go`: `buildPodSpec`, `buildDaemonPodSpec`, `parseVolume`, `parseToleration`. **The single source of truth for pod spec.**
- `internal/hippod/executor.go`: `GetPodUserInfo`, `SyncHelmRepositories`, `ExecuteCommand`, `ExecuteCommandInDaemon`, `SignalCopyDone`, `exitCodeMarkerWriter`.
- `internal/hippod/pdb.go`: `CreatePodDisruptionBudget`, `DeletePodDisruptionBudgets`, `deleteAllPodDisruptionBudgets` (used by `purge --all`).
- `internal/hippod/terminal.go`: Raw-mode TTY plumbing for `daemon shell`.
- `internal/hipns/namespace.go`: `PrepareNs`, `CreateClusterRoleBinding`, `DeleteClusterRoleBinding`, `waitForClusterRoleBindingEffective`.
- `internal/hipembedded/script.sh`: The pod's boot wrapper. Handles `/tmp/ready`, `/tmp/hip-wrapped-script.sh`, `/tmp/copy-done`, exit-code marker, SIGINT/SIGTERM trap.
- `internal/hipembedded/script.go`: `go:embed` + `GetShScript()`.
- `internal/hipconsts/consts.go`: Every magic string used in the plugin lives here.

**Testing:**
- Unit tests: `*_test.go` co-located with sources (Ginkgo v2).
- Suite roots: `cmd/suite_test.go`, `internal/suite_test.go`, `internal/<pkg>/suite_test.go`.
- E2E suite: `e2e/e2e_suite_test.go`, `e2e/e2e_test.go`, `e2e/utils.go`, 19 feature spec files.
- Kind orchestration: `e2e/setup-cluster.sh`, `e2e/run-tests.sh`, `e2e/teardown-cluster.sh`.

## Naming Conventions

**Files:**
- `lower_snake_case.go` for production files (`pod.go`, `daemon_start.go`, `pdb_crud_test.go`).
- One verb per file in `cmd/` (e.g. `exec.go`, `purge.go`, `daemon_exec.go`).
- Test files use the `_test.go` suffix and are co-located with the code under test.
- The Ginkgo suite root is always `suite_test.go` (one per package).
- E2E spec files end with `_test.go` and are gated by `//go:build e2e` on the very first line.

**Directories:**
- All internal subpackages share the `hip` prefix (`hippod`, `hipns`, `hipembedded`, `hipconsts`, `hiperrors`, `hipretry`) to make import paths unambiguous and prevent collisions with stdlib / k8s names.
- Exceptions: `helmtar` (because it is helm-specific tar work), `cmdoptions` (flag structs), `helpers` (genuine miscellany), `logz` (logging).

**Packages:**
- Package names are short and lowercase; match the directory name exactly.
- `internal/` keeps everything below the module from leaking into external consumers.

**Symbols:**
- Exported types use `PascalCase` (`Manager`, `ExecOptions`, `BundleEntry`, `ExitCodeError`).
- Unexported helpers use `camelCase` (`buildPodSpec`, `parseVolume`, `extractTarGz`, `isPodReady`).
- Constants in `hipconsts` are `PascalCase` with semantic prefixes: `Annotation*`, `Env*`, `Label*`, `CopyFrom*`, `*Path`.
- Cobra constructors are `newXxxCmd()` (`newRootCmd`, `newExecCmd`, `newPurgeCmd`, `newDaemonCmd`, `newDaemonStartCmd`, …).

**Labels & annotations on Kubernetes resources:**
- Labels: `host`, `daemon`, `helm-in-pod/operation-id`, `helm-in-pod/kept`, `app.kubernetes.io/managed-by`.
- Annotations: `helm-in-pod/home-directory`, `helm-in-pod/helm-found`, `helm-in-pod/helm4`, `helm-in-pod/last-repo-update-time`.
- All defined as constants in `hipconsts/consts.go`.

**Environment variables:**
- All plugin-specific env vars are `HELM_IN_POD_*` (`HELM_IN_POD_NAMESPACE`, `HELM_IN_POD_IMAGE`, `HELM_IN_POD_DAEMON_NAME`).
- The host also honors Helm's own `HELM_KUBECONTEXT`.

**Sentinel paths inside the pod:**
- All sentinel files live under `/tmp/`: `/tmp/ready`, `/tmp/hip-wrapped-script.sh`, `/tmp/hip-staged-script.sh`, `/tmp/hip-repositories.yaml`, `/tmp/copy-done`.

## Where to Add New Code

**A new Cobra subcommand (top-level or under `daemon`):**
- Primary code: `cmd/<verb>.go` with a `newXxxCmd()` constructor.
- Wire it into `cmd/root.go:newRootCmd` (for top-level) or `cmd/daemon.go:newDaemonCmd` (for daemon sub-subcommands).
- Tests: `cmd/<verb>_test.go` (Ginkgo).
- Do **not** put orchestration in `cmd/` — every interesting branch goes into `internal/hippod` or `internal/hipns`.

**A new pod-level flag (image / resource / volume / security):**
- Add the field to `internal/cmdoptions/exec.go:ExecOptions` (or `daemon.go:DaemonOptions`).
- Register the flag once in `cmd/flags.go:addPodCreationFlags` so `exec` and `daemon start` both pick it up.
- Read the flag in `internal/hippod/spec.go:buildPodSpec` (or `buildDaemonPodSpec` if daemon-specific).
- E2E coverage: add a spec in `e2e/<feature>_test.go`.

**A runtime flag (env / copy / repo handling):**
- Add the field to `ExecOptions` (or `DaemonOptions`).
- Register in `cmd/flags.go:addRuntimeFlags(cmd, opts, copyRepoDefault)`. Remember `copyRepoDefault` is `true` for `exec`/`daemon start` and `false` for `daemon exec`.

**A daemon-exec-only flag (e.g. `--clean`, `--update-all-repos`):**
- Add the field to `DaemonOptions`.
- Register the flag inline in `cmd/daemon_exec.go`, **not** in `cmd/flags.go`.

**A new external API call:**
- Wrap it in `hipretry.RetryWithContext(m.ctx, attempts, fn)` — the canonical 3-retry pattern is in `internal/hippod/executor.go:GetPodUserInfo`.
- Use `m.client()` (the lazy accessor) — never `operatorkclient.DefaultClient()` directly.
- Do not log inside the retry callback.

**A new global env var read:**
- Add the constant to `internal/hipconsts/consts.go`.
- Read it in `internal/vars.go:InitManagers` if it changes manager wiring; otherwise read it where used.

**A new error that should map to a non-zero exit code:**
- Construct `&hiperrors.ExitCodeError{Code: N}` at the point of failure.
- `main.go` already handles `errors.AsType[*hiperrors.ExitCodeError]` — no other wiring needed.

**A new pod volume type:**
- Extend the `switch volType {…}` block in `internal/hippod/spec.go:parseVolume`.
- Add a unit test in `internal/hippod/volume_test.go`.

**A new e2e feature spec:**
- New file `e2e/<feature>_test.go` with `//go:build e2e` on the first line.
- One `Describe` block per file is the convention.
- Use `utils.Run` / `utils.RunWithExitCode` / `utils.BuildHelmInPodCommand`; never raw `exec.Cmd.Run()`.
- Generate unique resource names with the helpers in `e2e/e2e_test.go` so parallel workers do not collide.

**A change to the in-pod boot protocol:**
- Edit `internal/hipembedded/script.sh` directly — it is `go:embed`-ed so no codegen step is required.
- Mirror any new sentinel paths or env vars in `internal/hipconsts/consts.go`.
- If you change the marker format, update the parser in `internal/hippod/executor.go:exitCodeMarkerWriter`.

## Special Directories

**`.helm-plugin-dev/`:**
- Purpose: Local dev-only plugin install target.
- Generated: Yes — created by `scripts/install-local.sh`, which writes a stripped `plugin.yaml` (no install hooks) and symlinks `bin/in-pod`.
- Committed: No (gitignored).

**`bin/`:**
- Purpose: Build output directory for `bin/in-pod`.
- Generated: Yes — produced by `make build` / `make install-local`.
- Committed: No.

**`e2e-reports/`:**
- Purpose: Diagnostics dumped by `e2e_test.go:logOnFailure` on test failure (cluster state, pod logs, describe output).
- Generated: Yes — overridable via `E2E_REPORTS_DIR`. CI uploads this directory only when the job fails.
- Committed: No.

**`generated/`:**
- Purpose: Generated artifacts.
- Generated: Yes (directory exists but contents are not committed).
- Committed: No.

**`.planning/`, `.claude/`, `.agents/`, `.omo/`, `.weave/`:**
- Purpose: Tooling state for AI-assisted workflows.
- Generated: Maintained by tooling, not by hand.
- Committed: Mixed — review `.gitignore` before committing.

**`coverage.html`, `coverage.out`, `inpod`, `inpodw` (at root):**
- Purpose: Test/build artifacts.
- Generated: Yes — produced by `make test-coverage` / older build invocations.
- Committed: No (in `.gitignore`).

---

*Structure analysis: 2026-05-20*
