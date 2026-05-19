# cmd/

Cobra CLI definitions only. Business logic lives in `internal/`.

## STRUCTURE

```
cmd/
├── root.go                # newRootCmd + ExecuteRoot; persistent --verbose-logs + --timeout; PersistentPreRunE calls internal.InitManagers
├── exec.go                # exec (alias: run) — the one-shot pod path
├── purge.go               # purge / purge --all
├── version.go             # version (build-time ldflags: version / commit / date)
├── daemon.go              # umbrella daemon command, wires the 6 sub-subcommands
├── daemon_start.go        # start a long-lived pod with daemon=<name> label
├── daemon_stop.go         # stop / delete one daemon pod
├── daemon_exec.go         # exec a command in an existing daemon pod
├── daemon_shell.go        # interactive shell into the daemon pod
├── daemon_status.go       # detailed status of one daemon
├── daemon_list.go         # tabular list of daemons (uses table.go)
├── flags.go               # addPodCreationFlags + addRuntimeFlags + getDaemonName + validateResourceFlags
├── table.go               # olekukonko/tablewriter helper for list/status output
└── *_test.go              # Ginkgo unit tests (flags registration, expand, daemon helpers, version, copy-from)
```

## COMMAND TREE

```
in-pod
├── exec (alias: run)              [cmd/exec.go]
├── purge [--all]                  [cmd/purge.go]
├── version                        [cmd/version.go]
└── daemon                         [cmd/daemon.go]
    ├── start                      [cmd/daemon_start.go]
    ├── stop                       [cmd/daemon_stop.go]
    ├── exec                       [cmd/daemon_exec.go]
    ├── shell                      [cmd/daemon_shell.go]
    ├── status                     [cmd/daemon_status.go]
    └── list (alias: ls)           [cmd/daemon_list.go]
```

## CONVENTIONS

- **Flag registration**: `addPodCreationFlags` (image/resources/PDB/security/etc.) and `addRuntimeFlags` (env/copy/repo) are reused between `exec` and every `daemon start`. New flags MUST be added once in `flags.go`, never copy-pasted into individual command files.
- **Daemon-name resolution**: every daemon sub-subcommand calls `getDaemonName(opts.Name)` which falls back to `HELM_IN_POD_DAEMON_NAME`. Do not re-implement.
- **Context propagation**: `RunE` MUST start with `internal.UseCommandContext(cmd.Context())` so retries inside `hippod`/`hipns` honor `--timeout` and SIGINT/SIGTERM.
- **Timeout math is path-specific**:
  - `exec` (`cmd/exec.go:52-54`) and `daemon start` (`cmd/daemon_start.go:45-50`) add `10 * time.Minute` to the user-supplied `--timeout` for pod creation/copy overhead.
  - `daemon exec` (`cmd/daemon_exec.go`) does NOT add anything — the timeout applies directly to in-pod command execution.
- **`--copy-repo` default flips per command**: `exec` and `daemon start` default it to `true`; `daemon exec` defaults to `false` (the daemon already has repos from `daemon start`). Reference: the `copyRepoDefault` argument to `addRuntimeFlags(cmd, opts, copyRepoDefault)`.
- **Daemon-exec-only flags** (not on `exec`): `--update-all-repos`, `--clean <path>` (deletes paths inside the daemon pod before copying). Live in `daemon_exec.go`, not `flags.go`.
- **Resource flag validation**: `validateResourceFlags` enforces that the deprecated `--cpu`/`--memory` flags cannot be combined with the new `--cpu-request`/`--cpu-limit`/`--memory-request`/`--memory-limit` flags.
- **Dry-run short-circuit**: when `--dry-run` is set, return after `PrintPodSpecYAML`; never call `PrepareNs` or create resources.

## ANTI-PATTERNS

- **Do not import** `github.com/onsi/ginkgo/v2` outside `_test.go` files in this package — only test files use Ginkgo here.
- **Do not put orchestration logic in `cmd/`**. Every interesting branch lives in `internal/hippod` or `internal/hipns`; `cmd/` should read like a flag map + a few `internal.Pod().X()` calls.
- **Do not call `internal.Pod()` / `internal.Namespace()` before `InitManagers` has run** — `root.go`'s `PersistentPreRunE` is what calls it, so anything before that (e.g. flag parsing) cannot touch managers.
- **Do not skip the deferred cleanup pattern** in `exec.go:60-83`. The pod must be deleted unless `--keep-pod` is set OR the context was canceled (SIGINT handler already deleted it) OR a timeout fired (use a fresh background context for cleanup).
