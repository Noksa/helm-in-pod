# Coding Conventions

**Analysis Date:** 2026-05-20

## Naming Patterns

**Files:**
- All Go files use lower-case names with underscores for multi-word concepts: `daemon_start.go`, `pod_label_test.go`, `exit_code.go`.
- Test files: `<source>_test.go` co-located with the file under test (`pod.go` ↔ `pod_test.go`).
- Suite files: every package with tests has exactly one `suite_test.go` that defines the Ginkgo `TestX` entry point (e.g. `internal/hippod/suite_test.go`).
- Cobra subcommand per file: one verb = one file (`cmd/exec.go`, `cmd/purge.go`, `cmd/daemon_start.go`). Never combine multiple commands in one file.

**Packages:**
- Internal-only packages live under `internal/` with a short `hip*` prefix to mark them as helm-in-pod-private: `hippod`, `hipns`, `hipconsts`, `hiperrors`, `hipretry`, `hipembedded`.
- The `cmd/` package is the only public-ish surface; it holds Cobra definitions and nothing else.

**Functions:**
- Exported APIs use full descriptive names: `CreateHelmPod`, `CopyFilesBundleWithBootInfo`, `SyncHelmRepositories`, `RetryWithContext`.
- Unexported helpers in the same file are short and verb-first: `parseToleration`, `isPodReady`, `buildPodSpec`, `unquote`, `expand`.
- Constructor convention: `NewManager(ctx, ...)` returns a pointer (`internal/hippod/pod.go:48`, `internal/hipns/namespace.go:25`).

**Variables:**
- Receiver `m` on Manager methods (`func (m *Manager) ...`), single-letter only when obvious.
- camelCase for local variables (`stagedRepoConfigPath`, `repoConfigStaged`, `bootInfo`).
- Pointer-only fields when nullability is meaningful (`*operatorkclient.Client kclient` — nil falls back to `DefaultClient()`).

**Types:**
- `PascalCase` for exported (`Manager`, `BackoffConfig`, `ExecOptions`, `ExitCodeError`, `UserInfo`, `BundleEntry`).
- `camelCase` for unexported types and helper structs.
- Error types end in `Error` and implement `Error() string` plus `Is(target error) bool` when comparison matters (see `internal/hiperrors/exitcode.go`).

**Constants:**
- Centralized in `internal/hipconsts/consts.go` — NEVER hard-code `"helm-in-pod"`, `/tmp/hip-*`, or any `HELM_IN_POD_*` env name elsewhere.
- Grouped `const ( ... )` blocks by concern (annotations, env vars, labels, sentinel paths).
- `hipconsts.Namespace` is the one exception — it is a `var` because it is overwritten in `internal.InitManagers` if `HELM_IN_POD_NAMESPACE` is set. Treat it as read-only everywhere else.

## Code Style

**Formatting:**
- `gofmt` via `go fmt ./...` (run by `scripts/check.sh`).
- `goimports` enforced with local-prefix grouping (`.golangci.yml:34-40`):
  - First group: stdlib.
  - Second group: third-party.
  - Third group: `github.com/noksa/helm-in-pod/...` (local).
- Example from `cmd/exec.go:1-22` — three import groups with blank-line separators.

**Linting:**
- Tool: `golangci-lint` v2 (config at `.golangci.yml`).
- Default linter set: `standard` plus opt-ins:
  - `gocritic` (with `diagnostic` and `performance` tags; `hugeParam` disabled)
  - `misspell` (US locale)
  - `nolintlint` (`require-explanation: true`, `require-specific: true`)
  - `unconvert`, `unparam`
  - `ginkgolinter` (test correctness)
  - `zerologlint` (logger misuse)
- Global build tag: `run.build-tags: [e2e]` — keeps the e2e package in the analysis graph at all times.
- Exclusions: `_test.go` files skip `errcheck` and `unparam`.

**Modernize:**
- `scripts/check.sh:31-35` runs `golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize` twice — once without tags and once with `GOFLAGS='-tags=e2e'` — to apply syntactic modernizations to both unit and e2e code.

## Import Organization

**Order (enforced by `goimports`):**
1. Standard library (`context`, `fmt`, `os`, `time`, ...).
2. Third-party (alphabetical):
   - `github.com/Noksa/operator-home/...`
   - `github.com/fatih/color`
   - `github.com/google/uuid`
   - `github.com/onsi/ginkgo/v2`, `github.com/onsi/gomega` (test-only)
   - `github.com/rs/zerolog`
   - `github.com/spf13/cobra`, `github.com/spf13/viper`
   - `go.uber.org/multierr`
   - `helm.sh/helm/v4/...`
   - `k8s.io/...`, `sigs.k8s.io/...`
3. Local — `github.com/noksa/helm-in-pod/...`.

**Path Aliases:**
- None (no `import alias "pkg"` patterns observed) — except for dot-imports in `_test.go` for Ginkgo/Gomega (see Testing).

**Test-only dot imports** (allowed exclusively in `_test.go`):
```go
. "github.com/onsi/ginkgo/v2"
. "github.com/onsi/gomega"
```

## Error Handling

**Patterns:**
- Wrap with `fmt.Errorf("...: %w", err)` to preserve unwrap chains. Examples:
  - `internal/vars.go:38` — `fmt.Errorf("failed to load kubeconfig: %w", err)`.
  - `internal/cmdoptions/envfile.go:42` — `fmt.Errorf("failed to open env file %q: %w", path, err)`.
- Use `errors.Is`, `errors.AsType[*T]` (from `operator-home`/Go 1.26 generics) instead of type assertions:
  - `main.go:44` — `errors.AsType[*hiperrors.ExitCodeError](err)`.
  - `cmd/exec.go:65` — `errors.Is(cmd.Context().Err(), context.Canceled)`.
- Permanent vs transient classification lives in `internal/hipretry/retry.go:51-125` and drives retry decisions.

**Exit-code propagation:**
- Errors that should map to a process exit code MUST be wrapped as `*hiperrors.ExitCodeError` (`internal/hiperrors/exitcode.go:12`).
- `main.go:44-46` is the single place that calls `os.Exit(int(exitErr.Code))`. Any other return path causes `log.Fatal()` with exit code 1.
- `*ExitCodeError` implements `Is(target error)` so `errors.Is` works across wrapped chains.

**Ignored errors:**
- Cleanup defers are explicitly discarded with `_ = ...`:
  - `cmd/exec.go:115-116` — temp file `Close`/`RemoveAll` on defer.
  - `internal/cmdoptions/envfile.go:44` — uses `//nolint:errcheck // read-only file` (the ONLY `nolint` in production code; both explanation and specific linter are required).
- Note: `errcheck` is excluded for `_test.go` files in `.golangci.yml`, so tests may use `_ = Run(cmd)` freely.

**Retry conventions:**
- All retries go through `internal/hipretry`. Two entry points:
  - `hipretry.RetryWithContext(ctx, maxAttempts, fn)` — default exponential backoff (`DefaultBackoff`: base 1s, multiplier 2, cap 60s).
  - `hipretry.RetryWithBackoff(ctx, cfg, maxAttempts, fn)` — custom backoff (e.g. `K8sAPIBackoff` with full jitter).
- Retry is silent by design — callers log, not `hipretry` (`internal/hippod/AGENTS.md` anti-pattern: never log inside retry callbacks; one log per attempt is wrong).
- Permanent error fast-exit: 401/403/404/422 + `meta.NoKindMatchError` stop immediately (`internal/hipretry/retry.go:51-67`).
- Transient triggers retry: `context.DeadlineExceeded`/`Canceled`, `EOF`, `connection reset`, `i/o timeout`, K8s 429/503/500, `*rest.RequestConstructionError`.
- The Manager retry pattern (canonical, `internal/hippod/executor.go:38-49`):
  ```go
  err := hipretry.RetryWithContext(m.ctx, 3, func() error {
      _, stderr, err := m.client().ExecInPod(...)
      if err != nil {
          return fmt.Errorf("%s: %w", stderr, err)
      }
      return nil
  })
  ```

**Multi-error aggregation:**
- `go.uber.org/multierr.Append` collects errors across retry attempts (`internal/hipretry/retry.go:172`).

## Logging

**Framework:** zerolog (`github.com/rs/zerolog`) configured in `main.go:17-41` with a custom `ConsoleWriter` that colorizes the `source` field.

**Helpers (centralized in `internal/logz/log.go`):**
- `logz.Host()` — host-side logs (cyan `[host]` prefix).
- `logz.Pod()` — in-pod logs (magenta `[pod]` prefix).
- `logz.HostPod()` — joint operations (`[host+pod]`).
- `logz.Suppress()` — global disable after signal interrupt to silence in-flight goroutines.
- All three accessors are `sync.OnceValue`-wrapped, so the logger is built lazily and reused.

**Patterns:**
- Never `fmt.Println` or `fmt.Print` for status output. Use `logz.Host().Info().Msg("...")` / `Msgf(...)`.
- Use `color.CyanString`, `color.GreenString`, `color.MagentaString`, `color.YellowString` for inline emphasis (see `cmd/root.go:41,49`).
- Level conventions:
  - `Info()` — milestones the user should see (command start, pod created, took N ms).
  - `Debug()` — gated by `--verbose-logs`; verbose internals (retry attempts, exec details).
  - `Warn()` — recoverable issues (e.g. `helm is not installed in the image`).
  - `Fatal()` — only in `main.go:47` as the terminal error path.
- Format strings use `Msgf("Running %v command", color.CyanString(cmd.Name()))` — always `%v` (lets `fatih/color` render correctly), never `%s`.

## Manager + Context Pattern

**Construction:**
- `internal.InitManagers()` (`internal/vars.go:31`) runs once in `cmd/root.go:PersistentPreRunE`. It:
  1. Loads kubeconfig (honoring `HELM_KUBECONTEXT`).
  2. Calls `operatorkclient.SetDefaultConfig(config)`.
  3. Reads `HELM_IN_POD_NAMESPACE` and updates `hipconsts.Namespace`.
  4. Constructs the package-global `namespace *hipns.Manager` and `pod *hippod.Manager`.
- Accessors: `internal.Pod()`, `internal.Namespace()` return the package globals.

**Context binding (mandatory):**
- Every Cobra `RunE` MUST start with `internal.UseCommandContext(cmd.Context())` (`cmd/exec.go:50`, `cmd/daemon_start.go:43`).
- `UseCommandContext` calls `pod = pod.WithContext(ctx)` and `namespace = namespace.WithContext(ctx)`, which return shallow copies bound to the Cobra context.
- Skipping this means retries inside the manager use `context.Background()` and never honor `--timeout` or `SIGINT`/`SIGTERM`.

**Manager structure (canonical, `internal/hippod/pod.go:40-69`):**
```go
type Manager struct {
    ctx          context.Context
    myHostname   string
    interrupted  atomic.Bool
    invocationID string                    // per-process UUID; isolates concurrent runs
    kclient      *operatorkclient.Client   // nil → fall back to DefaultClient()
}

func (m *Manager) WithContext(ctx context.Context) *Manager { /* shallow copy with new ctx */ }
func (m *Manager) client() *operatorkclient.Client { /* injected or default */ }
```

**Rules:**
- Never call `context.Background()` from inside Manager methods — always use `m.ctx`. (Exception: cleanup defers in `cmd/exec.go:75` deliberately use a fresh 30s context when the command context is dead.)
- Always use `m.client()`, never `operatorkclient.DefaultClient()` directly — this is what makes tests injectable (set `m.kclient` to a fake).
- `InitManagers` is one-shot. Tests construct fresh managers directly (`hippod.NewManager(ctx, hostname)`); they MUST NOT call `operatorkclient.SetDefaultConfig`.

## Cobra Command Conventions

**Layout:**
- One verb per file. `cmd/root.go` wires subcommands via `rootCmd.AddCommand(...)`.
- Persistent flags live on `rootCmd`: `--verbose-logs`, `--timeout` (`cmd/root.go:30-32`).
- Per-command flag registration lives in `cmd/flags.go`:
  - `addPodCreationFlags(cmd, opts)` — image, resources, PDB, security, tolerations.
  - `addRuntimeFlags(cmd, opts, copyRepoDefault)` — env, copy, repo (the `copyRepoDefault` argument flips between `true` for `exec`/`daemon start` and `false` for `daemon exec`).
- Never copy-paste flag registration into individual command files.

**Timeout math:**
- `cmd/exec.go:52-53` and `cmd/daemon_start.go:45-49` add `10 * time.Minute` to the user-supplied `--timeout` to cover pod creation/copy overhead.
- `cmd/daemon_exec.go` does NOT add overhead — `--timeout` applies directly to in-pod command execution.

**Dry-run:**
- When `--dry-run` is set, return after `PrintPodSpecYAML(opts, isDaemon)`. Do NOT call `PrepareNs` or create resources (`cmd/exec.go:56-58`).

**Deferred cleanup:**
- `cmd/exec.go:60-83` is the reference deferred-cleanup pattern: skip on `context.Canceled` (signal handler already cleaned up), skip on `--keep-pod`, and use a fresh 30s background context when the command context is dead.

## Function Design

**Size:** Most exported functions are 20-80 lines. The largest is `CreateHelmPod` (orchestrates apply + wait + retry); the orchestration sticks to `cmd/exec.go:RunE` which is intentionally long (~170 lines) because each branch is a distinct lifecycle step.

**Parameters:**
- Pass option structs by value when small and immutable for the function (`cmdoptions.ExecOptions`).
- Functions that mutate state (`ParseFileMappings`, `ParseEnvFiles`) hang off the options struct as methods.

**Return Values:**
- `(*T, error)` for constructors and lookups.
- `(string, string, error)` for `ExecInPod` style (stdout, stderr, err).
- Named return values are used only when needed for `defer` to inspect/modify the result (`cmd/exec.go:37` — `(returnErr error)`).

## Module Design

**Exports:**
- Package surface stays small: each `internal/*` package exports only what `cmd/` or sibling packages need.
- `internal.Pod()` / `internal.Namespace()` are the only public accessors for the managers. Tests bypass these and construct managers directly.

**Barrel Files:**
- None. No `internal/index.go`-style re-exports.

**Embedding:**
- `internal/hipembedded/script.sh` is embedded via `go:embed` in `script.go` and consumed only by `internal/hippod`. This package is a leaf — adding imports to it risks an import cycle.

## Build & Dev Loop

**Canonical pre-commit:**
```bash
make lint    # tidy + fmt + goimports + go vet (with and without -tags=e2e) + modernize (×2) + golangci-lint (×2)
make test    # alias of test-unit: ginkgo --skip-package=e2e -r
```

**Ginkgo CLI rule (critical):**
- NEVER use a globally-installed `ginkgo` binary. Always invoke `go run github.com/onsi/ginkgo/v2/ginkgo` (see `Makefile:36` and `e2e/run-tests.sh:40`).
- Reason: pins the exact Ginkgo version from `go.mod` and avoids the "CLI version mismatch" runtime warning.

**Build:**
- `make build` → `bin/in-pod` with ldflags injecting version/commit/date into `cmd.version`, `cmd.commit`, `cmd.date` (`Makefile:17-20`).
- `make binaries` → cross-compile via `scripts/make_archieve.sh` (controlled by `TARGET=os/arch`).

## Cyberpunk Theme

- All Makefile targets and bash scripts source a cached `.cyber.sh` (auto-downloaded by `Makefile:49-50` from `Noksa/install-scripts`).
- Helpers: `cyber_log`, `cyber_ok`, `cyber_err`, `cyber_step`. Use them in any new script — do not invent ad-hoc colors.
- `make help` renders a cyberpunk banner with project version/Go version.

---

*Convention analysis: 2026-05-20*
