<!-- refreshed: 2026-05-20 -->
# Architecture

**Analysis Date:** 2026-05-20

## System Overview

`helm-in-pod` is a **Helm 4 subprocess plugin**. The plugin manifest (`plugin.yaml`) tells Helm to invoke `${HELM_PLUGIN_DIR}/bin/in-pod`, a single static Go binary. The binary's job is to push command execution off the developer's laptop and into a Kubernetes pod that lives close to the API server, then stream output back. There is no in-cluster controller, no CRDs, no server component — only the host CLI and an ephemeral (or long-lived) pod it manages.

```text
┌──────────────────────────────────────────────────────────────────┐
│                          HOST PROCESS                            │
│  helm in-pod <verb> --flags -- <user command>                    │
│                                                                  │
│   main.go (zerolog setup, exit-code mapping)                     │
│       │                                                          │
│       ▼                                                          │
│   cmd/  (Cobra CLI: root, exec, purge, daemon …)                 │
│       │   PersistentPreRunE → internal.InitManagers              │
│       │   RunE → internal.UseCommandContext(cmd.Context())       │
│       ▼                                                          │
│   internal/ (vars.go, run.go — glue layer)                       │
│       │                                                          │
│       ▼                                                          │
│   internal/hipns       internal/hippod                           │
│   (namespace + SA      (pod CRUD, exec, copy,                    │
│    + ClusterRoleBinding) PDB, terminal)                          │
│       │                     │                                    │
└───────┼─────────────────────┼────────────────────────────────────┘
        │                     │
        │ kube-API (REST)     │ kube-API + SPDY/WebSocket exec
        ▼                     ▼
┌──────────────────────────────────────────────────────────────────┐
│                       KUBERNETES API SERVER                      │
└──────────────────────────────────────────────────────────────────┘
        │                     │
        ▼                     ▼
┌──────────────────────────────────────────────────────────────────┐
│        Namespace `helm-in-pod` (overridable via env)             │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ Pod (one container named "helm-in-pod")                    │  │
│  │   • Command: sh -cue <embedded script.sh>                  │  │
│  │   • Args: hipembedded.GetShScript()                        │  │
│  │   • Boot protocol:                                         │  │
│  │       1) touch /tmp/ready    (StartupProbe checks this)    │  │
│  │       2) wait for /tmp/hip-wrapped-script.sh               │  │
│  │       3) exec it; capture exit code                        │  │
│  │       4) (copy-from mode) emit ###HIP_EXIT_CODE:N###       │  │
│  │       5) (copy-from mode) wait for /tmp/copy-done          │  │
│  │   • ServiceAccount: helm-in-pod (bound to cluster-admin)   │  │
│  │   • Optional PDB (minAvailable=1, labeled with op-id)      │  │
│  └────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

## Component Responsibilities

| Component | Responsibility | File |
|-----------|----------------|------|
| `main` | Configure zerolog console writer with colored `[host]`/`[pod]`/`[host+pod]` sources, run `cmd.ExecuteRoot`, map `*hiperrors.ExitCodeError` to `os.Exit(N)` | `main.go` |
| `cmd` (Cobra layer) | Flag registration and command tree only — no orchestration. Each verb (`exec`, `purge`, `version`, `daemon <sub>`) lives in its own file | `cmd/*.go` |
| `cmd.ExecuteRoot` / `newRootCmd` | Build the root command, attach `--verbose-logs` + `--timeout`, run `InitManagers` in `PersistentPreRunE` | `cmd/root.go:17`, `cmd/root.go:56` |
| `internal.RunCommand` | Parse `--timeout` (default 2h), wire SIGINT/SIGTERM via `signal.NotifyContext`, wrap with `context.WithTimeout`, call `cmd.ExecuteContext(ctx)` | `internal/run.go:15` |
| `internal.InitManagers` | Load kubeconfig (honoring `HELM_KUBECONTEXT`), register `operatorkclient` default client, read `HELM_IN_POD_NAMESPACE`, construct package-level `hipns.Manager` and `hippod.Manager` | `internal/vars.go:31` |
| `internal.UseCommandContext` | Swap the package-level managers for shallow copies bound to `cmd.Context()` so retries respect `--timeout` and signals | `internal/vars.go:67` |
| `hipns.Manager` | Ensure namespace + ServiceAccount + `cluster-admin` ClusterRoleBinding exist; wait for RBAC to be effective via `SubjectAccessReview` | `internal/hipns/namespace.go:20` |
| `hippod.Manager` | Pod CRUD (`CreateHelmPod`, `CreateDaemonPod`, `DeleteHelmPods`, `DeleteKeptPods`), atomic bundle copy (`CopyFilesBundleWithBootInfo`), exec streaming (`ExecuteCommand`), signal handling, PDB lifecycle | `internal/hippod/pod.go:40` |
| `hippod.buildPodSpec` / `buildDaemonPodSpec` | Single source of truth for the pod spec (env, resources, security context, volumes, tolerations, topology spread, startup probe) | `internal/hippod/spec.go:97`, `:308` |
| `hippod.ExecuteCommand` | Move staged script → `/tmp/hip-wrapped-script.sh`, stream pod logs to stdout, intercept exit-code marker (copy-from mode), enforce timeout by `kill -term 1` inside the pod | `internal/hippod/executor.go:197` |
| `hipembedded` | Holds `script.sh` via `//go:embed` (leaf package — only `hippod` imports it) | `internal/hipembedded/script.go`, `script.sh` |
| `helmtar` | Build a single gzipped tar stream from `[]BundleEntry{Src, Dest}` for atomic copy into the pod | `internal/helmtar/tar.go` |
| `hipretry` | Context-aware exponential backoff + jitter; classifies Kubernetes errors as transient vs permanent; respects `Retry-After` | `internal/hipretry/retry.go` |
| `hiperrors` | `ExitCodeError{Code int32}` typed error — recognized by `main.go` via `errors.AsType` to set the process exit code | `internal/hiperrors/exitcode.go` |
| `hipconsts` | Central source of truth for namespace var, env-var names (`HELM_IN_POD_*`), label keys, sentinel paths, exit-code marker strings | `internal/hipconsts/consts.go` |
| `cmdoptions` | Flag structs (`ExecOptions`, `DaemonOptions`, `PurgeOptions`, `MaskOptions`) and helpers (`ParseFileMappings`, `ParseEnvFiles`) | `internal/cmdoptions/*.go` |
| `logz` | `Host()` / `Pod()` / `HostPod()` zerolog wrappers that stamp a `source` field; `Suppress()` mutes logs after interrupt | `internal/logz/log.go` |
| `helpers` | Misc cross-cutting helpers — `IsCompletionCmd`, helm-version helpers, shell-out wrapper for tests | `internal/helpers/*.go` |
| `e2e` | Build-tagged (`//go:build e2e`) Ginkgo v2 suite — builds plugin, installs it, runs feature specs against a kind cluster | `e2e/*.go`, `e2e/*.sh` |

## Pattern Overview

**Overall:** **Subprocess CLI plugin** with a **layered Manager / glue / leaf** package structure. The Cobra layer is intentionally thin and declarative; all orchestration lives in `internal/`. Side effects (Kubernetes API calls, file I/O) are concentrated in `internal/hippod` and `internal/hipns`, both of which expose a `Manager` struct that can be re-bound to a new `context.Context` via `WithContext(ctx)` for retry/timeout propagation.

**Key Characteristics:**
- **Two execution modes share one pod spec.** `exec` (one-shot, deletes pod on exit) and `daemon` (long-lived pod with `daemon=<name>` label) both build the spec via `buildPodSpec` and only differ in the container's `Command`/`Args` (the daemon overrides them to `sleep infinity`).
- **Host↔pod protocol is sentinel-file driven.** The host pushes a tarball; the embedded `script.sh` polls for `/tmp/hip-wrapped-script.sh` and (in copy-from mode) `/tmp/copy-done`. Exit codes are surfaced via a stdout marker line: `###HIP_EXIT_CODE:N###`.
- **Per-process ownership via `invocationID`.** Every `hippod.Manager` mints a UUID at construction. Labels (`helm-in-pod/operation-id`) and selectors include both the hostname and this UUID, so concurrent plugin processes on the same host never delete each other's pods.
- **Lazy `kclient`.** Both managers carry an optional `*operatorkclient.Client`; nil means use the default set up by `InitManagers`. Tests inject fakes directly.
- **Atomic bundle copy.** `helmtar` packs everything (user files + wrapped script + `repositories.yaml`) into a single tar. The pod's boot command extracts the tar and only at the very end does `mv hip-staged-script.sh hip-wrapped-script.sh`, which is the trigger for `script.sh` to start executing.

## Layers

**`cmd/` — CLI declaration layer:**
- Purpose: Declare commands, register flags, parse user input, call into `internal`.
- Location: `cmd/`
- Contains: Cobra commands (`root.go`, `exec.go`, `purge.go`, `version.go`, `daemon.go` + 6 daemon sub-subcommands), shared flag helpers (`flags.go`), table writer (`table.go`).
- Depends on: `internal`, `internal/cmdoptions`, `internal/helmtar`, `internal/hipconsts`, `internal/logz`.
- Used by: `main.go` only.

**`internal/` (root files) — Glue layer:**
- Purpose: Bootstrap managers (`vars.go`), wire signals/timeout into the command context (`run.go`).
- Location: `internal/vars.go`, `internal/run.go`.
- Contains: `InitManagers`, `Pod()`, `Namespace()`, `UseCommandContext`, `RunCommand`.
- Depends on: `internal/hipns`, `internal/hippod`, `internal/hipconsts`, `internal/logz`, `operatorkclient`, `client-go`.
- Used by: `cmd/`.

**`internal/hipns/` and `internal/hippod/` — Manager layer (side-effectful):**
- Purpose: All Kubernetes API mutations + pod-side exec/copy.
- Location: `internal/hipns/`, `internal/hippod/`.
- Contains: `hipns.Manager` (PrepareNs, CreateClusterRoleBinding, DeleteClusterRoleBinding) and `hippod.Manager` (~14 files, the heaviest package).
- Depends on: `internal/hipconsts`, `internal/hipretry`, `internal/hiperrors`, `internal/cmdoptions`, `internal/helmtar`, `internal/hipembedded`, `internal/logz`.
- Used by: `cmd/` via `internal.Pod()` / `internal.Namespace()`.

**`internal/hipembedded/` — Leaf:**
- Purpose: Hold the embedded `script.sh` boot wrapper.
- Location: `internal/hipembedded/script.go` (12 lines) + `script.sh`.
- Contains: A single exported function `GetShScript()` and the `go:embed` directive.
- Depends on: `embed` (stdlib) only.
- Used by: `internal/hippod` only (must stay a leaf to avoid import cycles).

**Utility packages (`hipconsts`, `hipretry`, `hiperrors`, `helmtar`, `cmdoptions`, `helpers`, `logz`):**
- Purpose: Small, narrowly-scoped helpers shared across layers.
- `hipconsts` imports nothing internal and may be imported by everyone.
- Other helpers may be imported by `cmd/` and `internal/hippod`/`hipns` but never the other way around.

## Data Flow

### Primary Request Path — `helm in-pod exec -- <command>`

1. **Plugin entry** — Helm 4 exec's `${HELM_PLUGIN_DIR}/bin/in-pod exec -- <args>` (`plugin.yaml`).
2. **zerolog setup** — colored `host`/`pod` source fields (`main.go:18-40`).
3. **Cobra root** — `cmd.ExecuteRoot` → `newRootCmd` → `internal.RunCommand` (`cmd/root.go:56`, `internal/run.go:15`).
4. **Timeout + signal context** — default 2h timeout, SIGINT/SIGTERM cancels root context (`internal/run.go:23-35`).
5. **`PersistentPreRunE`** — set debug log level, call `internal.InitManagers` (`cmd/root.go:33-46`).
6. **Manager init** — load kubeconfig (honor `HELM_KUBECONTEXT`), `operatorkclient.SetDefaultConfig`, build `hipns.Manager` + `hippod.Manager` with fresh `invocationID` UUID (`internal/vars.go:31-53`).
7. **`exec` `RunE`** — call `internal.UseCommandContext(cmd.Context())` to re-bind managers (`cmd/exec.go:50`); add 10 minutes to user `--timeout` for pod overhead (`cmd/exec.go:52-54`).
8. **Dry-run short-circuit** — if `--dry-run`, print pod YAML and return (`cmd/exec.go:56-58`).
9. **Defer cleanup** — install a defer that deletes the pod unless context was canceled (signal handler already cleaned up) or `--keep-pod` is set (`cmd/exec.go:60-83`).
10. **Bootstrap namespace** — `internal.Namespace().PrepareNs()` creates ns + SA + cluster-admin CRB, waits for RBAC to be effective via `SubjectAccessReview` (`internal/hipns/namespace.go:44-73`, `:100-124`).
11. **Create pod** — `buildPodSpec` → `Pods.Create` with labels `host=<hostname>`, `helm-in-pod/operation-id=<invocationID>`, `app.kubernetes.io/managed-by=helm-in-pod`; install a SIGINT goroutine that calls `DeleteHelmPods` with a fresh background context (`internal/hippod/pod.go:138-223`).
12. **Wait for readiness** — poll `pod.Status.ContainerStatuses[].Ready` (driven by the `/tmp/ready` StartupProbe) instead of streaming exec (`internal/hippod/pod.go:242-274`).
13. **Build bundle** — temp file with `#!/bin/sh\nset -eu\n<command>\n`, plus user `--file` mappings, plus `repositories.yaml` if `--copy-repo` (`cmd/exec.go:107-149`).
14. **Atomic copy + boot info** — single `kubectl exec` that (a) prints `${HOME}:::whoami:::id:::helm-version` and (b) untars the bundle into `/`. If `repoConfigStaged`, the same exec command also moves `/tmp/hip-repositories.yaml` to `${HOME}/.config/helm/repositories.yaml` (`internal/hippod/pod.go:312-389`).
15. **Optional repo sync** — if helm is present in the image and `--copy-repo` was true, `SyncHelmRepositories` runs `helm repo update` / explicit `--update-repo` lists (`internal/hippod/executor.go:79+`).
16. **Trigger execution** — move `/tmp/hip-staged-script.sh` → `/tmp/hip-wrapped-script.sh` (single exec, retried via `hipretry`). The embedded `script.sh` was already waiting on this path, so the user command starts now (`internal/hippod/executor.go:200-213`, `internal/hipembedded/script.sh:34-42`).
17. **Stream logs** — `StreamLogsFromPod` with `Follow: true` from `since := time.Now()`; loops until pod phase becomes `Succeeded` or `Failed`, or copy-from marker is detected (`internal/hippod/pod.go:553-576`, `internal/hippod/executor.go:286-316`).
18. **Exit-code interception (copy-from mode)** — `exitCodeMarkerWriter` intercepts `###HIP_EXIT_CODE:N###` on the log stream, cancels the stream context, returns `&hiperrors.ExitCodeError{Code: N}` (`internal/hippod/executor.go:567-614`).
19. **Copy back artifacts** — for every `--copy-from <pod-path>:<host-path>`, `CopyFileFromPod` tars the file inside the pod and untars on the host (`cmd/exec.go:170-198`, `internal/hippod/pod.go:439-497`).
20. **Signal copy done** — `SignalCopyDone` drops `/tmp/copy-done` so the pod's script can exit (`internal/hippod/executor.go:616+`).
21. **Deferred cleanup** — delete pod unless `--keep-pod`; use fresh 30 s background context if the original was cancelled by timeout (`cmd/exec.go:73-82`).
22. **Exit-code propagation** — `main.go:44` recognizes `*hiperrors.ExitCodeError` via `errors.AsType` and calls `os.Exit(int(exitErr.Code))`; any other error becomes `log.Fatal`.

### Daemon Flow — `helm in-pod daemon start/exec/shell/stop/status/list`

1. **`daemon start`** — same as exec steps 1-11, but the spec built by `buildDaemonPodSpec` overrides the container `Command`/`Args` to `touch /tmp/ready && trap 'exit 0' TERM INT; sleep infinity & wait` and the pod name is set to `daemon-<name>` with a `daemon=<name>` label (`internal/hippod/spec.go:308-319`, `internal/hippod/pod.go:586-641`).
2. **`daemon exec`** — `GetDaemonPod(name)` (no name → `HELM_IN_POD_DAEMON_NAME`), then `ExecuteCommandInDaemon` writes a script to `$HOME/wrapped-script.sh` (resolved from the `helm-in-pod/home-directory` annotation on the pod) and streams output. The pod is **not** deleted on exit. Default `--copy-repo=false` because repos were already synced at `daemon start` (`cmd/daemon_exec.go:18-60`, `internal/hippod/executor.go:329+`).
3. **`daemon shell`** — `OpenInteractiveShell` puts the local TTY in raw mode and pipes stdin/stdout/stderr to a pod exec session (`internal/hippod/pod.go:699-718`, `internal/hippod/terminal.go`).
4. **`daemon stop`** — `DeleteDaemonPod(name)` deletes `daemon-<name>` and its PDB (extracted from labels), then waits for the pod to disappear (`internal/hippod/pod.go:652-673`).
5. **`daemon status`** / **`daemon list`** — read `daemon=*` labels and selected annotations (`helm-in-pod/helm-found`, `helm-in-pod/helm4`, `helm-in-pod/home-directory`); `list` formats via `cmd/table.go` (`internal/hippod/pod.go:765-833`).

### Purge Flow — `helm in-pod purge [--all]`

- **`purge` (host-scoped, default):** Deletes the ClusterRoleBinding **and** all pods that match `host=<myHostname>,helm-in-pod/operation-id=<invocationID>,<labels…>` **and** all pods that match `host=<myHostname>,helm-in-pod/kept=true` (the `--keep-pod` survivors). PDBs for those pods are deleted as a side effect of `deletePodsMatchingSelector` (`cmd/purge.go:19-31`, `internal/hippod/pod.go:109-136`).
- **`purge --all` (namespace-wide):** Deletes the ClusterRoleBinding **and** all pods in the namespace regardless of host (selector is empty string), then sweeps any orphaned PDBs via `deleteAllPodDisruptionBudgets` (`internal/hippod/pod.go:110-118`).

**State Management:**
- **Global package-level state in `internal/`:** `pod *hippod.Manager`, `namespace *hipns.Manager` (set by `InitManagers`, swapped by `UseCommandContext`). `hipconsts.Namespace` is a `var` overwritten from `HELM_IN_POD_NAMESPACE`.
- **Per-process state on `hippod.Manager`:** `invocationID` (UUID), `myHostname`, `interrupted atomic.Bool`, optional `kclient` (nil → default).
- **In-cluster state:** pods, optional PDBs, the `helm-in-pod` namespace, ServiceAccount, and ClusterRoleBinding. There are no CRDs, ConfigMaps, or Secrets owned by the plugin.

## Key Abstractions

**`hippod.Manager` (`internal/hippod/pod.go:40`):**
- Purpose: Owns every Kubernetes mutation related to pods + PDBs, plus exec/copy I/O.
- Pattern: Receiver-based methods; `WithContext(ctx)` returns a shallow copy so callers can rebind the context without touching the package-level singleton.
- Carries a per-process `invocationID` UUID so concurrent processes can scope deletions to their own pods.

**`hipns.Manager` (`internal/hipns/namespace.go:20`):**
- Purpose: Bootstraps namespace + ServiceAccount + cluster-admin ClusterRoleBinding and waits for RBAC propagation via `SubjectAccessReview`.
- Pattern: Identical to `hippod.Manager` (lazy `kclient`, `WithContext`).

**`hipretry.RetryWithContext` / `RetryWithBackoff` (`internal/hipretry/retry.go:159`, `:164`):**
- Purpose: The single retry primitive in the codebase. Every Kubernetes API call inside a manager is wrapped in it.
- Pattern: Exponential backoff + optional full jitter + transient/permanent error classification (`isPermanentError` short-circuits 404/403/401/422 + `NoMatchError`). Respects server `Retry-After`. Silent — the caller logs.

**`helmtar.BundleEntry` + `CompressMulti` (`internal/helmtar/tar.go:18`, `:25`):**
- Purpose: Pack multiple `(srcPath, destPath)` pairs into a single gzip+tar stream that the pod can extract with `tar zxf - -C /` in one exec call.
- Pattern: All files staged into `/tmp/hip-staged-*` first; the boot command performs a final `mv` to "atomically" trigger execution.

**`hiperrors.ExitCodeError` (`internal/hiperrors/exitcode.go:12`):**
- Purpose: Typed error that survives `fmt.Errorf("%w", …)` wrapping and is recognized by `main.go` via `errors.AsType` to set `os.Exit(int(Code))`.
- Pattern: Exit code 0 is still success (not wrapped); any non-zero in-pod exit becomes `&ExitCodeError{Code: int32(N)}` constructed at `internal/hippod/executor.go:322`.

**Embedded boot protocol — `internal/hipembedded/script.sh`:**
- The pod container is started as `sh -cue <script.sh>` with `TIMEOUT` and optional `WAIT_COPY_DONE` env vars set in the pod spec.
- After `touch /tmp/ready` (StartupProbe satisfaction), it `tail`-waits for `/tmp/hip-wrapped-script.sh`, exec's it, captures `$?`, and in copy-from mode prints `###HIP_EXIT_CODE:N###` and blocks up to 300 s on `/tmp/copy-done`.
- Trap on `INT`/`TERM` sends signals to all non-PID-1 processes and waits up to 180 s for `hip-wrapped-script.sh` / `helm` / `kubectl` to die.

**Marker-based exit-code propagation — `###HIP_EXIT_CODE:N###`:**
- Defined as constants in `hipconsts` (`CopyFromExitCodeMarkerPrefix`, `…Suffix`).
- Only emitted in copy-from mode (when `WAIT_COPY_DONE` is set, i.e. the host has `--copy-from` flags).
- Parsed by `exitCodeMarkerWriter` (`internal/hippod/executor.go:567+`), a `io.Writer` wrapper around `stdout` that scans for the marker, cancels the streaming context, and exposes `Found()`/`ExitCode()`.
- For non-copy-from runs, exit code is derived from pod phase + the inner `exit $CMD_EXIT` of `script.sh`.

**`invocationID` label-based ownership:**
- `hippod.NewManager` generates a UUID per process.
- Every pod created by that process is labeled `helm-in-pod/operation-id=<UUID>` and PDB selectors match the same label.
- `DeleteHelmPods` filters by `host=<hostname>,helm-in-pod/operation-id=<UUID>` so concurrent plugin processes never trample each other's pods.

## Entry Points

**Binary entry:**
- Location: `main.go:16`
- Triggers: Helm invokes `${HELM_PLUGIN_DIR}/bin/in-pod` as a subprocess (`plugin.yaml:13-14`).
- Responsibilities: Set up zerolog with colored `host`/`pod` source rendering, run `cmd.ExecuteRoot`, map `*hiperrors.ExitCodeError` to `os.Exit(N)`, fatal-log any other error.

**Cobra root:**
- Location: `cmd/root.go:56` (`ExecuteRoot`), `cmd/root.go:17` (`newRootCmd`)
- Triggers: `main.go:42`.
- Responsibilities: Build root command, attach `--verbose-logs` and `--timeout` persistent flags, run `internal.InitManagers` in `PersistentPreRunE` (skipped for completion / `version`).

**Subcommands:**
- `cmd/exec.go:24` — `exec` (alias: `run`), the one-shot pod path.
- `cmd/purge.go:12` — `purge` / `purge --all`.
- `cmd/version.go` — `version`.
- `cmd/daemon.go:7` — umbrella command, wires 6 sub-subcommands: `daemon_start`, `daemon_stop`, `daemon_exec`, `daemon_shell`, `daemon_status`, `daemon_list`.

**Plugin install/update hooks:**
- `scripts/install.sh` — runs on `helm plugin install` and `helm plugin update` (referenced from `plugin.yaml:16-19`).

## Architectural Constraints

- **Threading:** A single host goroutine drives orchestration. Long-running phases spawn goroutines via `sync.WaitGroup.Go` (Go 1.26 feature) for log streaming, a deadline-watcher goroutine that `kill -term 1`s the pod on timeout, and a signal handler goroutine installed by `CreateHelmPod`. The signal handler stays alive for the lifetime of the process (not just the function) so cleanup runs even when SIGINT arrives during file copy.
- **Global state:** `internal.pod`, `internal.namespace` (package vars), `hipconsts.Namespace` (var, overwritten by `HELM_IN_POD_NAMESPACE`), and `operatorkclient.DefaultClient()` (set by `InitManagers` via `operatorkclient.SetDefaultConfig`). All are written once at startup; `UseCommandContext` replaces the manager singletons in-place per command.
- **No in-process concurrency for the user command.** Only one user command runs at a time per process; parallel pods come from running multiple `helm in-pod` processes (each with its own `invocationID`).
- **Helm 4 only.** `go.mod` pins `helm.sh/helm/v4 v4.0.4`; Helm 3 users must stay on v0.8.1 of the plugin.
- **`hipembedded` must remain a leaf.** Only `internal/hippod` and tests may import it — keeps the embedded script free of import-cycle risk.
- **`InitManagers` is one-shot.** Sub-commands must never call it; only `cmd/root.go:PersistentPreRunE` does.
- **No `context.Background()` inside Managers.** Always use `m.ctx` (the context bound by `UseCommandContext`). The only intentional exception is cleanup defers in `cmd/exec.go:73-82`, which use a fresh 30 s background context when the command context has already been canceled by timeout.

## Anti-Patterns

### Force-deleting Running or Pending pods

**What happens:** Setting `GracePeriodSeconds=0` on a pod whose phase is still `Running` or `Pending`.
**Why it's wrong:** It strands containers on the node — kubelet never gets a chance to call container stop hooks, so volumes can leak and follow-up scheduling decisions become inconsistent.
**Do this instead:** `internal/hippod/pod.go:95-99` only sets `GracePeriodSeconds=0` when phase is `Succeeded` or `Failed`. Match that conditional in any new deletion path.

### Bypassing `SignalCopyDone` in copy-from mode

**What happens:** Skipping the `/tmp/copy-done` sentinel after `--copy-from` artifacts have been pulled.
**Why it's wrong:** The pod's `script.sh` blocks for up to 300 s waiting for that file (see `script.sh:53-63`). The host appears to hang or the pod times out unnecessarily.
**Do this instead:** Always call `internal.Pod().SignalCopyDone(pod)` after the copy loop, even on partial failure (`cmd/exec.go:170-198`).

### Two-step "stage then trigger" copy

**What happens:** Copying the user script in one exec call and then "activating" it via a second exec call.
**Why it's wrong:** Between the two calls, `script.sh` may see the script file mid-write and exec a partial command — a real bug fixed by bundling everything into a single tar with `hip-staged-script.sh` and moving it atomically.
**Do this instead:** Put every file into the `helmtar.BundleEntry` slice and let the boot command do `mv /tmp/hip-staged-script.sh /tmp/hip-wrapped-script.sh` after extraction (`internal/hippod/executor.go:200-213`).

### Cross-host `host=` selectors

**What happens:** Deleting pods with a selector that omits `host=<myHostname>` (or omits `helm-in-pod/operation-id=<invocationID>`).
**Why it's wrong:** Concurrent plugin processes on the same host or on a shared CI runner will kill each other's running pods. Same root cause as the old "orphaned PDB" class of bugs.
**Do this instead:** `purge` (no flag) is host-scoped + invocation-scoped (and a separate kept-pod sweep). `purge --all` is the **only** sanctioned namespace-wide path. See `internal/hippod/pod.go:109-128`.

### Logging from inside `hipretry` callbacks

**What happens:** Adding `logz.Host().Warn().Msg(...)` inside a function passed to `RetryWithContext`.
**Why it's wrong:** The retry loop will call your function up to `maxAttempts` times → one log line per attempt. `RetryWithContext` is deliberately silent.
**Do this instead:** Log the final outcome at the call site, not inside the callback.

### Mutating `hipconsts.Namespace` outside `InitManagers`

**What happens:** Code or tests reassigning `hipconsts.Namespace = "something-else"` mid-flight.
**Why it's wrong:** It is a global var only because the environment variable `HELM_IN_POD_NAMESPACE` must override the default. Any other writer races with concurrent reads in `hippod`/`hipns`.
**Do this instead:** Set `HELM_IN_POD_NAMESPACE` before process start; treat the var as read-only elsewhere.

## Error Handling

**Strategy:** Errors flow up as plain `error` values until they reach `main.go:42`. Two categories matter at the top level:

1. **`*hiperrors.ExitCodeError`** — the user command exited non-zero. `errors.AsType` finds it through any `fmt.Errorf("%w", …)` wrap; `os.Exit(int(Code))` is called and zerolog's fatal log is skipped.
2. **All other errors** — logged via `log.Fatal().Msg(err.Error())`, which exits 1.

**Patterns:**
- **Transient API errors** are absorbed by `hipretry.RetryWithContext` (3-5 attempts depending on call site).
- **Permanent errors** (404/403/401/422 + `NoMatchError`) short-circuit retries immediately.
- **`context.DeadlineExceeded`** in `ExecuteCommand` triggers `kill -term 1` against PID 1 inside the pod (up to 20 attempts, 50 ms apart) so the wrapped script can exit cleanly.
- **`context.Canceled`** (SIGINT) is handled by the goroutine installed in `CreateHelmPod`, which calls `DeleteHelmPods` with a fresh 30 s background context and then `os.Exit(1)` on a second signal.
- **PDB orphan recovery** — `purge --all` runs `deleteAllPodDisruptionBudgets` after pod deletion to clean up PDBs whose owner pod was killed by `activeDeadlineSeconds`.

## Cross-Cutting Concerns

**Logging:**
- Single library: `github.com/rs/zerolog` configured in `main.go:18-40` with a colored console writer.
- The `source` field is rendered specially: `[host]` (cyan), `[pod]` (magenta), `[host+pod]` (both). Anything else becomes `[<source>]`.
- Helpers in `internal/logz`: `Host()`, `Pod()`, `HostPod()` return reusable loggers via `sync.OnceValue`.
- `logz.Suppress()` mutes all output globally; called by the SIGINT goroutine to silence racing goroutines.

**Validation:**
- Flag-level: `cmd/flags.go:validateResourceFlags` rejects mixing `--cpu`/`--memory` (deprecated) with `--cpu-request`/`--cpu-limit`/`--memory-request`/`--memory-limit`.
- Spec-level: `parseVolume` (`internal/hippod/spec.go:26`) validates `type:name:mountPath[:ro]` and rejects unsupported volume types; `parseToleration` enforces `key=value:effect:operator` and limits the operator to `Equal`/`Exists`.
- Bundle-level: `extractTarGz` (`internal/hippod/pod.go:506-551`) rejects tar paths that would escape the destination directory (`!strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir))`).

**Authentication / authorization:**
- Host side: standard kubeconfig discovery (`clientcmd.NewDefaultClientConfigLoadingRules`) with optional `HELM_KUBECONTEXT` override (`internal/vars.go:23-29`).
- Pod side: dedicated ServiceAccount (`helm-in-pod`) bound to the built-in `cluster-admin` ClusterRole via a single ClusterRoleBinding (`internal/hipns/namespace.go:75-98`). The plugin waits up to 30 s for the binding to propagate via `SubjectAccessReview` before creating the first pod (`:100-124`).
- `purge` and `purge --all` both delete the ClusterRoleBinding; the next run recreates it via `PrepareNs`.

**Timeout & cancellation:**
- `--timeout` on the root command (default 2h) drives a single `context.WithTimeout` on the SIGINT-aware root context (`internal/run.go:30-35`).
- `exec` and `daemon start` add 10 minutes to the user value internally (`cmd/exec.go:52-54`, `cmd/daemon_start.go:49`) to absorb pod creation + bundle copy.
- `daemon exec` does **not** add anything — the timeout applies to in-pod execution.
- `UseCommandContext` re-binds the managers each command so retry sleeps respect the same deadline.

---

*Architecture analysis: 2026-05-20*
