# Testing Patterns

**Analysis Date:** 2026-05-20

## Test Framework

**Runner:** Ginkgo v2 (BDD spec runner)
- Source: `github.com/onsi/ginkgo/v2` (version pinned in `go.mod`).
- Config: per-package `suite_test.go` files (no centralized `ginkgo.config.yaml`).

**Assertion Library:** Gomega (`github.com/onsi/gomega`).
- Imported via dot-import inside `_test.go` only.

**Linters:** `ginkgolinter` and `zerologlint` enabled in `.golangci.yml`. These catch Ginkgo-specific anti-patterns (e.g. assertions outside `It`, missing `BeforeEach` calls).

**Run Commands (all via Makefile):**
```bash
make test                          # alias for test-unit
make test-unit                     # ginkgo --skip-package=e2e -r (race off)
make test-short                    # adds --short
make test-verbose                  # adds -v
make test-focus FOCUS="pattern"    # --focus "pattern"
make test-coverage                 # --cover --coverprofile=coverage.out; renders coverage.html
make test-ci                       # --race --trace --randomize-all --keep-going --cover --json-report=report.json
make test-e2e                      # provisions kind cluster if missing, then runs e2e/ specs
make test-e2e-serial               # GINKGO_PROCS=1 forces serial e2e
make test-e2e-full                 # setup-cluster.sh → test-e2e → teardown-cluster.sh
```

**Ginkgo invocation rule (CRITICAL):**
- NEVER use the installed `ginkgo` CLI. Always: `go run github.com/onsi/ginkgo/v2/ginkgo`.
- Reason: pins the exact version from `go.mod`, prevents the "CLI version mismatch" warning.
- Reference: `Makefile:36` (`GINKGO_RUN := go run github.com/onsi/ginkgo/v2/ginkgo`) and `e2e/run-tests.sh:40`.

**Default Ginkgo flags** (`Makefile:31`):
```
--silence-skips --procs=${GINKGO_PROCS:-5} --randomize-all
```
Optional `RACE=1` adds `--race --trace`.

## Test File Organization

**Location:** Co-located with production code in the same package.
- `internal/hipretry/retry.go` ↔ `internal/hipretry/retry_test.go`.
- `internal/hippod/spec.go` ↔ `internal/hippod/spec_test.go`.
- `cmd/version.go` ↔ `cmd/version_test.go`.

**Naming:** `<source>_test.go` for unit tests; `suite_test.go` for the Ginkgo entry per package.

**Structure (full inventory):**
```
.                                  (62 *_test.go files total; 24 in e2e/)
├── cmd/
│   ├── suite_test.go                  # TestCmd entry
│   ├── copy_from_test.go
│   ├── daemon_helpers_test.go
│   ├── daemon_start_test.go
│   ├── expand_test.go
│   ├── flags_daemon_test.go
│   ├── flags_registration_test.go
│   ├── flags_resource_test.go
│   ├── purge_test.go
│   └── version_test.go
├── internal/
│   ├── suite_test.go                  # TestInternal entry
│   ├── run_test.go
│   ├── vars_test.go
│   ├── cmdoptions/
│   │   ├── suite_test.go
│   │   ├── envfile_test.go
│   │   ├── exec_test.go
│   │   └── mask_test.go
│   ├── helmtar/{suite_test.go, tar_test.go}
│   ├── helpers/{suite_test.go, cmd_test.go}
│   ├── hipembedded/{suite_test.go, script_test.go}
│   ├── hiperrors/exitcode_test.go     # no suite_test.go — uses package's existing suite
│   ├── hipns/namespace_test.go
│   ├── hippod/
│   │   ├── suite_test.go
│   │   ├── executor_test.go
│   │   ├── extract_test.go
│   │   ├── pdb_test.go
│   │   ├── pdb_crud_test.go
│   │   ├── pod_test.go
│   │   ├── pod_delete_test.go
│   │   ├── pod_label_test.go
│   │   ├── spec_test.go
│   │   ├── volume_test.go
│   │   └── wait_pod_test.go
│   └── hipretry/{suite_test.go, retry_test.go}
└── e2e/                                # ALL files start with //go:build e2e
    ├── e2e_suite_test.go              # TestE2E + SynchronizedBefore/AfterSuite
    ├── e2e_test.go                    # shared fixtures (createNamespace, logOnFailure, ...)
    ├── utils.go                       # Run, RunWithExitCode, BuildHelmInPodCommand, ...
    └── <feature>_test.go × 19         # one feature/Describe per file
```

## Test Suite Entry Point

**Canonical pattern** (every test package, e.g. `internal/hippod/suite_test.go`):
```go
package hippod

import (
    "testing"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

func TestHippod(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "Hippod Suite")
}
```

**Naming:** `TestX` where X is a PascalCase variant of the package, suite name is `"X Suite"`.

## Test Structure

**Top-level container:** Always `Describe("<symbol or behavior>", func() { ... })` as a top-level `var _ = Describe(...)` assignment.

**Reference (from `internal/hippod/spec_test.go`):**
```go
var _ = Describe("buildPodSpec", func() {
    var baseOpts func() cmdoptions.ExecOptions

    BeforeEach(func() {
        baseOpts = func() cmdoptions.ExecOptions {
            return cmdoptions.ExecOptions{ /* sensible defaults */ }
        }
    })

    Context("environment variables", func() {
        It("should set explicit env vars from --env flag", func() {
            opts := baseOpts()
            opts.Env = map[string]string{"FOO": "bar"}
            spec, err := buildPodSpec(opts, false)
            Expect(err).NotTo(HaveOccurred())
            Expect(findEnvVar(spec.Containers[0].Env, "FOO")).To(Equal("bar"))
        })
    })

    Context("resource requests and limits", func() { ... })
})
```

**Patterns observed:**
- `BeforeEach` sets up fresh state per spec; often as a factory closure (`baseOpts func() cmdoptions.ExecOptions`) to avoid shared mutation.
- `Context("<branch>", ...)` groups related `It` blocks under a behavioral axis.
- `It("should <verb>", func() { ... })` — every spec name starts with "should" and describes observable behavior.
- Local helpers live at the bottom of the file (`func envVarNames(...) []string`, `func findEnvVar(...) string`).
- `DeferCleanup(func() { ... })` for per-spec teardown (preferred over `AfterEach` when scoped to a single `It`).

**RunCommand cobra-context tests (`internal/run_test.go`):**
- Build a probe `*cobra.Command` with a `RunE` that captures `ctx` properties.
- Mutate `os.Args` per spec, restore in `AfterEach`.
- Assert deadline ranges with `BeNumerically(">", ...)` / `BeNumerically("<=", ...)` rather than exact equality.

## Mocking

**No mocking library is used** (no `gomock`, `testify/mock`, or `mockery` in `go.sum`). Two strategies replace mocks:

**1. Constructor injection on Managers** (`internal/hippod/AGENTS.md` rule):
- `Manager.kclient *operatorkclient.Client` is optional; nil falls back to `operatorkclient.DefaultClient()`.
- Production code never sets `kclient`; tests inject a fake `*operatorkclient.Client` directly on the Manager struct.
- Tests MUST NOT call `operatorkclient.SetDefaultConfig` — that's `InitManagers`'s job.

**2. Pure-function decomposition:**
- Anything testable as a pure function is extracted to be tested without I/O.
- `buildConfigOverrides()` in `internal/vars.go:23` is explicitly "extracted for testability".
- `parseToleration`, `parseExitCodeFromError`, `isPodReady`, `exitCodeFromContainerStatuses`, `buildPodSpec` — all pure, all unit-tested.

**No mock of in-cluster behavior:**
- Anything that genuinely requires a cluster lives in `e2e/` behind the `e2e` build tag and uses a real kind cluster.

## Fixtures and Factories

**Temporary directories:**
```go
tmpDir = GinkgoT().TempDir()                      // auto-cleaned per spec (envfile_test.go:17)
tmpDir, _ = os.MkdirTemp("", "helmtar-test-*")    // explicit; pair with DeferCleanup (tar_test.go:52-54)
```

**Test data via inline closures:**
- `baseOpts func() cmdoptions.ExecOptions` factories return fresh copies per spec to avoid cross-test mutation (see `spec_test.go:16-32`, `spec_test.go:677-691`).

**Helper writers (file-local):**
```go
writeFile := func(name, content string) string {
    p := filepath.Join(tmpDir, name)
    Expect(os.WriteFile(p, []byte(content), 0644)).To(Succeed())
    return p
}
```
(from `internal/cmdoptions/envfile_test.go:20-24`).

**No shared fixtures directory** — every test creates exactly what it needs.

## Coverage

**Generate:**
```bash
make test-coverage    # → coverage.out + coverage.html (rendered with `go tool cover -html`)
```

**View:** open `coverage.html` (committed-but-gitignored artifact at the repo root).

**Coverage in CI:** `make test-ci` writes both `coverage.out` and `report.json`. There is no enforced minimum coverage threshold today.

## Test Types

**Unit tests (everything outside `e2e/`):**
- Scope: pure functions, parser logic, flag wiring, error classification, retry behavior, Cobra command construction.
- No cluster, no Kubernetes API calls (Managers tested via injected fake `kclient`).
- 38 `_test.go` files under `cmd/` and `internal/` (62 total minus 24 in `e2e/`).
- Default execution: 5 Ginkgo processes in parallel (`GINKGO_PROCS=5`), `--randomize-all`.

**E2E tests (`e2e/` package):**
- Build tag: every file starts with `//go:build e2e`.
- Skipped by all unit-test targets via `--skip-package=e2e`.
- Requires a kind cluster named `helm-in-pod-e2e` (set up by `e2e/setup-cluster.sh`).
- Default execution: 5 processes via `e2e/run-tests.sh` (`GINKGO_PROCS=5 ... --timeout=20m`).
- K8s matrix in CI: v1.28.15, v1.30.8, v1.32.3, v1.35.1.
- Reuses the cluster between `make test-e2e` runs; use `make test-e2e-teardown` to wipe.

**No separate integration tier** — everything sits in either unit or e2e.

## E2E Patterns

**Shell-out as the test boundary:** e2e specs do not import the plugin's Go API; they shell out to the real `helm in-pod ...` binary installed by `SynchronizedBeforeSuite`.

**`SynchronizedBeforeSuite` / `SynchronizedAfterSuite`** (`e2e/e2e_suite_test.go:24-103`):
- Process-1-only setup: build the plugin, install it into Helm, create the `helm-in-pod` namespace, install `helm-diff`.
- Per-process setup (runs in every Ginkgo worker): set Gomega defaults (`SetDefaultEventuallyTimeout(30s)`, `SetDefaultEventuallyPollingInterval(500ms)`), point `KUBECONFIG` at `e2e/.kubeconfig`.
- Process-1-only teardown: delete the namespace and uninstall the plugin.

**Helper API (`e2e/utils.go`):**
- `Run(cmd *exec.Cmd) (string, error)` — always sets `cmd.Dir = projectRoot`, forwards `os.Environ()`, prints `→` to `GinkgoWriter`. Fails on non-zero exit.
- `RunWithExitCode(cmd) (string, int)` — same but returns the exit code instead of erroring.
- `BuildHelmInPodCommand(args...)` / `BuildDaemonStartCommand(args...)` — inject CI-safe defaults (`--copy-repo=false`, low CPU/memory) so specs don't repeat them.
- `e2eResourceFlags` — `--cpu-request=50m --cpu-limit=0 --memory-request=64Mi --memory-limit=0` (kept in one var so all specs share the same constrained resource profile).

**Parallel-safety:**
- Unique-name helpers in `e2e/e2e_test.go`:
  - `randomString(n)`, `generateReleaseName(prefix)`, `generateNamespace(prefix)`, `generateTestLabel()`.
- Every test gets its own namespace; the shared `helm-in-pod` namespace is only deleted in `SynchronizedAfterSuite`.

**Failure diagnostics:**
- `logOnFailure(ns)` in `e2e_test.go:94-119` — call from `AfterEach`. On failure, dumps `kubectl get pods / describe pods / get events` into `e2e-reports/<spec-name>.txt`.
- Path overridable with `E2E_REPORTS_DIR`. CI uploads this directory only when the job fails.

## Common Patterns

**Async / Eventually:**
- Defaults set in `SynchronizedBeforeSuite`: `Eventually` polls every 500ms for up to 30s; `Consistently` for 5s with 500ms polling.
- Use `Eventually(func() error { ... }).Should(Succeed())` for "wait for K8s state to converge" patterns.

**Error Testing:**
```go
_, err := buildPodSpec(opts, false)
Expect(err).To(HaveOccurred())
Expect(err.Error()).To(ContainSubstring("--cpu-request"))
Expect(err.Error()).To(ContainSubstring("badvalue"))
```
- Match the flag name AND the offending value in the error message — both are user-facing contracts.

**Exit-code assertions:**
- E2E uses `RunWithExitCode`:
  ```go
  output, exitCode := RunWithExitCode(cmd)
  Expect(exitCode).To(Equal(0), "Expected successful repo list, output: %s", output)
  ```
- Unit tests of `ExitCodeError` use `errors.Is` semantics (see `internal/hiperrors/exitcode_test.go`).

**Configuration save/restore** (when a spec mutates package-level vars):
```go
origVersion, origCommit, origDate := version, commit, date
DeferCleanup(func() {
    version, commit, date = origVersion, origCommit, origDate
})
version = "v1.2.3"
```
(from `cmd/version_test.go:46-52`).

**Output capture for Cobra commands:**
```go
cmd := newVersionCmd()
var out bytes.Buffer
cmd.SetOut(&out)
Expect(cmd.RunE(cmd, nil)).To(Succeed())
Expect(out.String()).To(ContainSubstring("helm-in-pod "))
```

## Anti-Patterns (Do Not Do)

- **Do not use `go test ./...` directly.** It bypasses Ginkgo's parallelization and randomization. Use `make test` / `make test-e2e`.
- **Do not invoke the installed `ginkgo` binary.** Use `go run github.com/onsi/ginkgo/v2/ginkgo` (pinned via `go.mod`).
- **Do not delete e2e specs to make CI green.** `e2e-reports/` is uploaded on failure for diagnostics.
- **Do not call `operatorkclient.SetDefaultConfig` from tests.** Inject `kclient` onto the Manager directly.
- **Do not log inside `hipretry` callbacks.** It produces one log line per attempt; the caller is responsible for logging.
- **Do not assume serial e2e execution.** Default is 5 parallel processes. Anything that hard-codes a namespace or release name will flake — use the generator helpers.
- **Do not delete the `helm-in-pod` namespace in per-test teardown.** That's `SynchronizedAfterSuite`'s job; per-test deletes break parallel workers.

---

*Testing analysis: 2026-05-20*
