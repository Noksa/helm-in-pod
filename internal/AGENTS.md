# internal/

Glue layer (`vars.go`, `run.go`) plus narrowly-scoped private packages.

## STRUCTURE

```
internal/
├── vars.go              # package-global Managers (namespace, pod); InitManagers + UseCommandContext
├── run.go               # RunCommand: parse --timeout, wire SIGINT/SIGTERM, ExecuteContext
├── hippod/              # pod CRUD + spec + executor + PDB + terminal — see internal/hippod/AGENTS.md
├── hipns/               # namespace + ServiceAccount + cluster-admin ClusterRoleBinding
├── cmdoptions/          # flag structs: ExecOptions, DaemonOptions, PurgeOptions, MaskOptions + env-file parsing
├── helmtar/             # tar bundle builder for atomic file copy into the pod
├── hipembedded/         # go:embed-ed `script.sh` (in-pod boot wrapper)
├── hipconsts/           # central source of truth for namespace var, env-var names, label keys, sentinel paths
├── hiperrors/           # ExitCodeError type + ExtractExitCode parser for the `###HIP_EXIT_CODE:N###` marker
├── hipretry/            # context-aware retry-with-backoff helper (used by every retry in the codebase)
├── helpers/             # cmd helpers (IsCompletionCmd) + helm helpers + tests
└── logz/                # zerolog wrappers exposing Host(), Pod(), HostAndPod() with colored `source` fields
```

## WHERE TO LOOK

| What                              | File                                          |
|-----------------------------------|-----------------------------------------------|
| Add a global env var read         | `hipconsts/consts.go` + `vars.go:InitManagers` if it changes the manager wiring |
| Add a new flag struct field       | `cmdoptions/exec.go` (or `daemon.go`); call `ParseFileMappings`/`ParseEnvFiles` from the command |
| Add retry around a new API call   | wrap with `hipretry.RetryWithContext(m.ctx, attempts, fn)` |
| Add a new exit-code-mapped error  | construct `&hiperrors.ExitCodeError{Code: N, Err: cause}` |
| Wrap an external shell command    | `helpers/cmd.go`                              |

## CONVENTIONS

- **Manager pattern**: `hippod.Manager` and `hipns.Manager` are constructed once in `InitManagers` and stored as package vars (`pod`, `namespace`). Public accessors are `internal.Pod()` and `internal.Namespace()`. Both expose `WithContext(ctx)` which returns a shallow copy so `UseCommandContext` can rebind to the Cobra command's context.
- **kclient is lazy**: both Managers carry an optional `*operatorkclient.Client` field. When nil, methods fall back to `operatorkclient.DefaultClient()` (set up by `InitManagers` via `operatorkclient.SetDefaultConfig(config)`). Tests inject their own client; production code uses the default.
- **`hipconsts.Namespace` is a `var` on purpose** — it is overwritten in `InitManagers` if `HELM_IN_POD_NAMESPACE` is set. Every other place treats it as read-only.
- **Build-tag isolation**: nothing in `internal/` requires `-tags=e2e`. That tag is reserved for `e2e/`.

## ANTI-PATTERNS

- **Do not introduce cycles**: `hipembedded` must remain a leaf (only `hippod` and tests may import it). `hipconsts` may be imported by everyone; it imports nothing internal.
- **Do not call `context.Background()` inside Managers** — always use `m.ctx`. Exception: cleanup defers in `cmd/exec.go` deliberately use a fresh 30-second background context when the command context is dead.
- **Do not log from inside `hipretry`** — the caller logs. Otherwise every retried API call double-logs.
- **`InitManagers` is one-shot**. Do not call it from sub-commands; `cmd/root.go:PersistentPreRunE` already runs it. If a future test needs re-init, it should construct fresh Managers directly, not call `InitManagers`.
