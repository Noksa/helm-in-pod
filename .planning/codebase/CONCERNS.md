# Codebase Concerns

**Analysis Date:** 2026-05-20

## Tech Debt

**Deprecated `--cpu` / `--memory` flags still wired:**
- Issue: `cmd/flags.go:82-85` keeps `--cpu` and `--memory` flags alive with `MarkDeprecated`, but they are still parsed and merged into the new `--cpu-request` / `--cpu-limit` / `--memory-request` / `--memory-limit` flags via custom logic in `validateResourceFlags`. The dual-default behavior (apply default `1100m`/`500Mi` only when no flags changed) is non-trivial and easy to break.
- Files: `cmd/flags.go:32-72`, `internal/cmdoptions/exec.go:16-21`
- Impact: Mutually-exclusive validation logic must be kept in sync with default-application logic; a future contributor moving the default into the flag declaration will silently double-apply it.
- Fix approach: Schedule a major version removal; in the meantime, lock validation behind a single test that exercises every combination (some matrix already exists in `cmd/flags_resource_test.go`).

**Two near-identical helm-version detection paths:**
- Issue: `internal/hippod/pod.go:300` defines `helmMajorVersionRe` and parses helm version inline as part of `CopyFilesBundleWithBootInfo`, while `internal/helpers/helm.go:14-56` keeps an older `GetHelmMajorVersion` / `IsHelm4` pair that does two extra `ExecInPod` round-trips (`helm --help` then `helm version`). The helpers package version is no longer the primary path but still ships as exported API.
- Files: `internal/hippod/pod.go:300-371`, `internal/helpers/helm.go`
- Impact: Two regexes, two parsing paths, two failure modes; tests in `internal/helpers/` may pass even if the production path in `hippod` regresses.
- Fix approach: Either delete `internal/helpers/helm.go` (no current callers in `cmd/` or `internal/` other than tests) or downgrade it to call into `hippod` so there is a single source of truth.

**Embedded `script.sh` blocks on signal cleanup for 180 s:**
- Issue: `internal/hipembedded/script.sh:5-26` busy-loops every 3 s for 3 minutes sending `INT`+`TERM` to PID-1 broadcast, then `exit 1`. There is no acknowledgement protocol between host and pod; the kubelet's `TerminationGracePeriodSeconds=300` (`internal/hippod/spec.go:211`) is wider than this window, but the relationship is implicit.
- Files: `internal/hipembedded/script.sh`, `internal/hippod/spec.go:211`
- Impact: Long-running helm/kubectl commands hold the pod for up to 3 minutes after SIGTERM before forced exit; the host-side timeout handler in `internal/hippod/executor.go:263-284` then sends `kill -term 1` 20 times with 50 ms delay — these two retry loops do not coordinate.
- Fix approach: Document the relationship between the script's 180 s window, the pod spec's 300 s `TerminationGracePeriodSeconds`, and the host's 20-attempt kill loop; consider a single configurable value sourced from `hipconsts`.

**`cmd/exec.go:121-126` rewrites the wrapped script even though `CopyFilesBundleWithBootInfo` already accepts a script entry:**
- Issue: `exec.go` builds a temp file with `#!/bin/sh\nset -eu\n…` and adds it to the bundle, then `executor.go:200-213` moves it from staged → wrapped paths with another exec call. Daemon path (`executor.go:329-407`) generates a *different* script with extra `echo "[date]" > /proc/1/fd/1` markers. Two divergent script-generation paths.
- Files: `cmd/exec.go:107-138`, `internal/hippod/executor.go:194-236, 329-407`
- Impact: Bug fixes in one path (e.g. shebang change, `set -eu` removal, env-var export order) silently miss the other.
- Fix approach: Extract a single `BuildWrappedScript(cmd, opts, runtime DaemonRuntime|OneShotRuntime)` constructor and call it from both places.

**Empty-line dead code in `cmd/exec.go:126`:**
- Issue: `_ = tempScriptFile.Close()` is followed by a deferred `tempScriptFile.Close()` (`exec.go:115`). The double close is harmless but signals that the file lifetime is unclear.
- Files: `cmd/exec.go:107-126`
- Fix approach: Either drop the explicit close (defer is enough) or restructure into a helper that returns `[]byte` so no file is needed on the host.

## Known Bugs

**`scripts/install.sh:94-97` checks `$?` after `&&`-chained pipeline:**
- Symptoms: The `$?` check after `rm -rf bin && mkdir bin && tar xzvf ... > /dev/null && rm -f ...` is always `0` if the chain completed and a non-`set -e` shell would have already exited otherwise (since the script runs `set -e` at line 3). The check is dead code.
- Files: `scripts/install.sh:91-97`
- Trigger: Any tar extraction failure already aborts due to `set -e`; the printed "an error has occured" message never fires.
- Workaround: None needed (behavior is correct due to `set -e`); but the message is unreachable.

**`scripts/install.sh` fallback-to-latest path is silent UX:**
- Symptoms: When the requested version is not yet published, the script switches to "latest" and only prints a warning. The user thinks they installed `vX.Y.Z` but actually got `vA.B.C`.
- Files: `scripts/install.sh:60-87`
- Trigger: Running `helm plugin install` immediately after a tag is pushed but before the release workflow finishes.
- Workaround: Re-run `helm plugin update in-pod` after a few minutes (printed in the warning).
- Recommendation: Fail loudly instead — the user explicitly opted into a specific version.

**`internal/hippod/executor.go:541-565` exit-code parser is heuristic:**
- Symptoms: `parseExitCodeFromError` does a `strings.LastIndex(msg, "exit code ")` on the error string and walks digits. Any error message containing the substring "exit code N" will be matched, even from unrelated wrapped errors.
- Files: `internal/hippod/executor.go:541-565`
- Trigger: A kubectl/helm subcommand whose stderr happens to contain that exact phrase (e.g. log lines from CronJobs or other containers if `kubectl logs` is the user command).
- Workaround: Use `ExitCodeError` propagation from terminal pod phase (`handleTerminalPhase`) where possible — that path reads the structured container status.

**`internal/cmdoptions/exec.go:52-65` silently drops malformed `--copy` entries:**
- Symptoms: `ParseFileMappings` only adds to `FilesAsMap` when `len(parts) == 2`; a value like `/host/path` with no `:` is silently dropped.
- Files: `internal/cmdoptions/exec.go:57-63`
- Trigger: User typo such as `--copy /etc/foo` (no destination).
- Fix approach: Return `error` and surface it from `ParseFileMappings` callers (`cmd/exec.go:86`, `cmd/daemon_start.go:61`).

## Security Considerations

**`cluster-admin` ClusterRoleBinding to plugin ServiceAccount (intentional, but worth flagging):**
- Risk: `internal/hipns/namespace.go:75-98` creates a `ClusterRoleBinding` named `helm-in-pod` granting `cluster-admin` to the `helm-in-pod` `ServiceAccount`. Every pod the plugin creates auto-mounts this SA token (`AutomountServiceAccountToken: true` in `internal/hippod/spec.go:210`). Anyone with shell access to the pod (`helm in-pod daemon shell`, or `kubectl exec`) inherits cluster-admin.
- Files: `internal/hipns/namespace.go:75-98`, `internal/hippod/spec.go:179-211`
- Current mitigation: ClusterRoleBinding is only created on first use; `purge --all` removes it; the namespace is dedicated.
- Recommendations:
  - Document this prominently in README — currently it is buried.
  - Offer an opt-in `--service-account` flag that already exists (`cmd/flags.go:104`) but the README does not advertise its security implications.
  - Consider creating a minimal Role (pods/exec, configmaps read, secrets read in target ns) as a documented alternative.

**`--privileged` flag exists with no guardrails:**
- Risk: `cmd/flags.go:107` exposes `--privileged` which sets `securityContext.Privileged=true` (`internal/hippod/spec.go:163-165`). Combined with the default `cluster-admin` SA and `--host-network` (`cmd/flags.go:93`), this is a one-flag escape from the pod to the node.
- Files: `cmd/flags.go:93,107`, `internal/hippod/spec.go:163-165, 242-244`
- Current mitigation: Defaults are `false`; flag is opt-in.
- Recommendations: Warn at log-level when either flag is set; document that PodSecurity admission `restricted`/`baseline` will reject these pods so users running in hardened clusters get a clear failure.

**Install script does not verify checksums or signatures:**
- Risk: `scripts/install.sh` downloads the release tarball over HTTPS and extracts it without any checksum or signature verification, even though the release pipeline (`.github/workflows/release.yml:94-107`) signs the *plugin* tarball with PGP and pushes a `.prov` file. The Go-binary tarballs that `install.sh` consumes are unsigned, and Helm's plugin tarball signature is verified only when users explicitly `helm plugin install --verify` from OCI.
- Files: `scripts/install.sh:42-87`, `.github/workflows/release.yml:94-107`, `public-key.asc`, `plugin.yaml:13-19`
- Current mitigation: HTTPS to `github.com`; PGP key (`public-key.asc`) shipped in repo for users who manually verify.
- Recommendations:
  - Generate and publish per-asset `.sha256` files; verify in `install.sh`.
  - Document `helm plugin install --verify --keyring <ring> oci://ghcr.io/noksa/helm-in-pod/in-pod` as the recommended path.
  - Pin `public-key.asc` fingerprint in `install.sh` and verify before extract.

**`install.sh` uses unauthenticated GitHub redirect parsing for "latest":**
- Risk: `scripts/install.sh:64-66` parses the `Location` header of `releases/latest` to discover the latest tag, then downloads it. A MITM (or compromised CDN) could redirect to an arbitrary tag.
- Files: `scripts/install.sh:60-87`
- Current mitigation: Curl/wget over HTTPS; GitHub Releases are content-addressed only inside the tarball, not in the script.
- Recommendations: Use the GitHub Releases API JSON endpoint with a proper JSON parser instead of header scraping; require explicit version pinning by default.

**Shell command construction in `ExecInPod` uses string interpolation:**
- Risk: Several `ExecInPod` callers build commands via `fmt.Sprintf` with user-controlled values:
  - `internal/hippod/executor.go:173`: `fmt.Sprintf("rm -rf %s", strings.Join(cleanPaths, " "))` where `cleanPaths` comes from the `--clean` flag.
  - `internal/hippod/executor.go:204`: `fmt.Sprintf("mv %s %s", ...)` — values are package constants here, safer.
  - `internal/hippod/pod.go:404`: `fmt.Sprintf("mkdir -p %s && tar zxf - -C /", dir)` where `dir = filepath.Dir(destPath)` and `destPath` comes from `--copy /host:/pod`.
  - `internal/hippod/pod.go:454,458,462`: `fmt.Sprintf("tar czf - -C %s %s", ...)` with `--copy-from` paths.
- Files: `internal/hippod/executor.go:170-179, 197-213`, `internal/hippod/pod.go:391-497`
- Current mitigation: All callers are the local CLI user, who already has cluster-admin via the SA; the threat surface is "user feeds odd characters into their own flag values."
- Recommendations: Either shell-escape the path with `strconv.Quote`/`shellescape`, or pass paths through `operatorkclient.WithStdin` and a static command. Long-term, push more of the boot logic into the embedded `script.sh` so that paths flow as env vars rather than command tokens.

**No `ReadOnlyRootFilesystem`, no seccomp profile, no `runAsNonRoot`:**
- Risk: `internal/hippod/spec.go:156-165` only sets `RunAsUser`/`RunAsGroup`/`Privileged` when the user passes flags. There is no default `seccompProfile`, no `readOnlyRootFilesystem`, no `runAsNonRoot`. The pod runs whatever the image (defaults to `docker.io/noksa/kubectl-helm:v1.34.5-v4.1.1`) chose, often as root.
- Files: `internal/hippod/spec.go:156-218`
- Current mitigation: User can override via image / `--run-as-user`.
- Recommendations: Add an opt-in `--secure-defaults` mode that sets `runAsNonRoot=true`, `readOnlyRootFilesystem=true` (with appropriate tmpfs mounts for `/tmp`), and the `RuntimeDefault` seccomp profile.

**`HELM_KUBECONTEXT` env var consumed unsanitized:**
- Risk: `internal/vars.go:25` reads `HELM_KUBECONTEXT` from the environment and uses it as `ConfigOverrides.CurrentContext`. The value is treated as a kubeconfig context name — if a malicious env contains an invalid context, the only outcome is connection failure; not a vulnerability, but worth documenting.
- Files: `internal/vars.go:21-29`, `internal/vars_test.go`
- Current mitigation: Behavior is purely lookup-based; no shell evaluation.

## Performance Bottlenecks

**Watch+poll dual-path in `waitForPodCompletion`:**
- Problem: `internal/hippod/executor.go:418-504` opens a `Watch` per pod, then falls back to a 500 ms poll when the Watch channel closes. On clusters where Watch is broken or rate-limited, every concurrent invocation polls in parallel.
- Files: `internal/hippod/executor.go:418-504`
- Cause: No backoff in fallback path; no shared informer (each invocation watches one pod).
- Improvement path: Switch to a shared informer for the namespace when many invocations co-exist; or back off polling exponentially when Get returns errors.

**`waitUntilPodIsRunning` polls every 1 second for up to 5 minutes:**
- Problem: `internal/hippod/pod.go:242-274` uses `wait.PollUntilContextTimeout` with a 1 s interval. With 100 parallel invocations, the API server sees 100 GETs/sec for up to 5 minutes per startup.
- Files: `internal/hippod/pod.go:242-274`
- Cause: GET-based readiness rather than Watch-based.
- Improvement path: Convert to Watch on the pod (as `waitForPodCompletion` already does) with the same fallback.

**`StreamLogsFromPod` is restarted in a hot loop on error:**
- Problem: `internal/hippod/executor.go:286-315` loops in a goroutine, restarting `StreamLogsFromPod` whenever it errors, with a 25 ms sleep between restarts. If the API server is briefly unreachable, this hammers it.
- Files: `internal/hippod/executor.go:286-315`
- Cause: No exponential backoff on the inner restart loop.
- Improvement path: Use `hipretry.RetryWithBackoff` here instead of bare `time.Sleep(25ms)`.

**Tar bundle is held in memory before exec:**
- Problem: `internal/hippod/pod.go:319-388` builds the entire tar bundle into a `bytes.Buffer` and passes it via `WithStdin(bytes.NewReader(tarBytes))`. For large `--copy` payloads (e.g. multi-GB Helm chart directories), this doubles host memory usage.
- Files: `internal/hippod/pod.go:319-388`, `internal/helmtar/tar.go:25-39`
- Cause: `gzip.Writer` is buffered in memory before send.
- Improvement path: Stream the gzip output directly to the exec stdin via `io.Pipe` so memory stays bounded.

## Fragile Areas

**`###HIP_EXIT_CODE:N###` marker collision:**
- Files: `internal/hipembedded/script.sh:54`, `internal/hippod/executor.go:581-606`, `internal/hipconsts/consts.go:31-32`
- Why fragile: The pod streams the marker through stdout/stderr; if the user command (e.g. `kubectl logs`, `cat /etc/motd`) happens to emit a string matching `###HIP_EXIT_CODE:<digits>###` on its own line, the host-side `exitCodeMarkerWriter` will capture it as the real exit code and cancel the log stream early.
- Safe modification: Treat the marker as advisory only; cross-check with `handleTerminalPhase`'s container status read. The current `copy-from` path relies solely on the marker because the pod is intentionally kept alive past the user command (so terminal phase isn't available yet).
- Test coverage: `internal/hippod/executor_test.go` exercises happy-path marker extraction but no test exists for marker forgery (e.g. user command echoing the marker before exiting).

**Per-process `invocationID` label is the only thing preventing concurrent runs from killing each other:**
- Files: `internal/hippod/pod.go:48-54, 109-128`, `internal/hipconsts/consts.go:23`
- Why fragile: `DeleteHelmPods` selector is `host=<hostname>,helm-in-pod/operation-id=<uuid>,<extra-labels>`. If a future contributor adds a code path that omits the operation-id selector — or sets it from the wrong source — concurrent helm-in-pod invocations from the same host will start deleting each other's pods at startup.
- Safe modification: Always go through `DeleteHelmPods` / `deletePodsMatchingSelector`; never call `Pods().DeleteCollection()` with a partial selector. Add a regression test in `internal/hippod/pod_label_test.go` for any new deletion path.
- Test coverage: Exists in `e2e/` ("Concurrent invocation isolation does not delete a sibling pod"); no unit test gating the selector format.

**Atomic bundle copy depends on staged-path move ordering:**
- Files: `internal/hippod/pod.go:312-389`, `internal/hippod/spec.go` (boot command), `internal/hippod/executor.go:200-213`
- Why fragile: The wrapped script lives at `/tmp/hip-staged-script.sh` until the host explicitly executes `mv /tmp/hip-staged-script.sh /tmp/hip-wrapped-script.sh`. The in-pod `script.sh` polls for `/tmp/hip-wrapped-script.sh`. Any future change that ships the wrapped script directly at the final path (or that bundles it without the staged indirection) re-introduces the partial-copy race the design eliminated.
- Safe modification: Keep `StagedScriptPath`/`WrappedScriptPath` separate; the move must be a single rename within the same filesystem (both are in `/tmp`).
- Test coverage: `internal/hippod/spec_test.go`, `e2e/bundle_copy_test.go` exist but neither asserts that partial copy cannot trigger execution.

**`copy-done` sentinel requires the host to always reach `SignalCopyDone`:**
- Files: `internal/hippod/executor.go:618-627`, `internal/hipembedded/script.sh:53-64`
- Why fragile: The pod waits up to 300 s for `/tmp/copy-done` before exiting. If the host crashes between marker detection and `SignalCopyDone`, the pod sits idle for 5 minutes (then forcibly exits). The host signaller ignores errors (`logz.Host().Debug()` only), so a permanently broken signaller path won't surface in tests.
- Safe modification: Either reduce the wait window or convert the signal to a watched file-creation event with a shorter, configurable timeout.

**`buildPodSpec` hard-codes container name "helm-in-pod" at line 186, parallel to `hipconsts.ContainerName`:**
- Files: `internal/hippod/spec.go:186`, `internal/hipconsts/consts.go:11`
- Why fragile: `spec.go:186` uses the literal `"helm-in-pod"` instead of `hipconsts.ContainerName`. If anyone overrides the constant (or renames it), the spec build silently diverges and `ExecInPod` calls (which use the constant) fail with "container not found."
- Safe modification: Replace the literal with `hipconsts.ContainerName`.

**`hipconsts.Namespace` is a `var`, not a `const`:**
- Files: `internal/hipconsts/consts.go:6`, `internal/vars.go:43-45`
- Why fragile: The maintainer documents this is intentional (overridable via `HELM_IN_POD_NAMESPACE`), but it is a hidden mutable global. Any goroutine that reads `hipconsts.Namespace` after `InitManagers` has run gets the user-set value; any read before gets `"helm-in-pod"`. Test isolation requires careful sequencing.
- Safe modification: Restrict writes to `InitManagers` (current contract); reads everywhere else.

**Signal handler goroutine leaks across `CreateHelmPod` returns:**
- Files: `internal/hippod/pod.go:186-219`
- Why fragile: `CreateHelmPod` registers `signal.Notify(c, os.Interrupt, syscall.SIGTERM)` and spawns a goroutine that lives for the rest of the process. Calling `CreateHelmPod` twice in one process (e.g. tests, or a future `exec --retry`) registers two handlers; both will run their cleanup on a single signal, racing on `DeleteHelmPods`.
- Safe modification: Either deduplicate (use `signal.NotifyContext` like `internal/run.go:30`) or document that `CreateHelmPod` is single-call per process.

## Scaling Limits

**Tar bundle size limited by host RAM:**
- Current capacity: ~host-RAM / 2 (because `bytes.Buffer` holds the gzipped form before exec).
- Limit: OOM at ~half of available RAM; in CI runners (4 GB) the practical max is ~1 GB compressed payload.
- Scaling path: Switch to `io.Pipe` between `helmtar.CompressMulti` and the exec stdin reader.

**Per-process operation-id assumes single helm-in-pod binary per Go runtime:**
- Current capacity: Multiple goroutines safely share one Manager because `invocationID` is set once at `NewManager`.
- Limit: A single process that wants to run multiple isolated "invocations" in parallel cannot (they share the UUID and would delete each other on cleanup).
- Scaling path: Move `invocationID` from the Manager onto the `ExecOptions` / per-call context.

**Streaming logs via `bufio.Reader.ReadBytes('\n')`:**
- Current capacity: Lines must fit in `bufio.Reader`'s default 4 KB buffer.
- Limit: Lines longer than 4 KB cause silent split into multiple writes; the `exitCodeMarkerWriter` checks each chunk for the marker, so a long log line containing the marker might be missed if the marker straddles a chunk boundary.
- Scaling path: Switch to a length-prefixed framing or buffer accumulation that looks back across writes.

## Dependencies at Risk

**`helm.sh/helm/v4` is locked to Helm 4 only:**
- Risk: As of v0.9.0, Helm 3 is dropped (`README.md` directs Helm 3 users to v0.8.1). The plugin will not even load against a Helm 3 client because `plugin.yaml`'s `apiVersion: v1` + `type: cli/v1` schema is Helm 4 only.
- Files: `plugin.yaml:1-2`, `go.mod` (helm.sh/helm/v4)
- Impact: Existing Helm 3 deployments cannot upgrade in-place.
- Migration plan: Documented in README; v0.8.1 remains available.

**`Noksa/operator-home` is an external personal-org dependency:**
- Risk: `internal/vars.go:8` and most of `internal/hippod` depend on `github.com/Noksa/operator-home/pkg/operatorkclient`. If that repository moves or is deleted, build breaks.
- Files: `go.mod`, every `internal/hippod/*.go`
- Impact: Build/CI fails; cannot release.
- Migration plan: Vendor the package, or rewrite the exec wrapper using upstream `client-go`'s `remotecommand`.

**`go-helpers/helpers/gopointer` and `go.uber.org/multierr` are tiny helper libs:**
- Risk: Both are stable but add transitive dependencies for trivial functionality (pointer constructor; error joining — the latter is now in stdlib via `errors.Join`).
- Files: `internal/hippod/spec.go:9`, `internal/hipretry/retry.go:12`
- Impact: Low; supply-chain surface area only.
- Migration plan: Replace `multierr` with stdlib `errors.Join` (already used elsewhere in the codebase); replace `gopointer.NewOf(x)` with a local `ptr[T any](v T) *T` helper.

## Missing Critical Features

**No checksum verification in install path:**
- Problem: See "Security Considerations".
- Blocks: Verifiable supply chain story for users running in regulated environments.

**No release notes automation cross-check:**
- Problem: `docs/RELEASING.md` requires a manual paste of `RELEASE_NOTES.md` content into the GitHub release UI after the workflow auto-generates notes. There is no CI check that the file exists or matches the tag.
- Blocks: Releases can ship without curated notes; PR list drifts silently.

**No `--service-account` validation against the actual namespace:**
- Problem: `cmd/flags.go:104` accepts any SA name; if it does not exist, pod creation fails with a generic error.
- Blocks: Clear error UX for users running with their own SA.

## Test Coverage Gaps

**Exit-code marker forgery:**
- What's not tested: A user command that emits `###HIP_EXIT_CODE:42###` on its own line should NOT be misinterpreted as the wrapped script exit code in non-copy-from mode (only `--copy-from` enables marker mode, but the writer logic is shared).
- Files: `internal/hippod/executor.go:567-606`
- Risk: A malicious chart could emit the marker to fake a successful exit while crashing.
- Priority: Medium.

**Install-script verification:**
- What's not tested: Neither `scripts/install.sh` nor `scripts/install-local.sh` has a CI integration test that the resulting binary actually loads as a Helm plugin and that the version reported matches the manifest.
- Files: `scripts/install.sh`, `scripts/install-local.sh`, `scripts/test-plugin.sh`
- Risk: Release ships with a broken plugin manifest and nobody notices until a user reports.
- Priority: High.

**Bundle-copy race window:**
- What's not tested: There is no test that asserts execution does NOT start before the staged → wrapped move (the design contract). A unit test could mock `ExecInPod` to return the staged file as present and assert the script does not run.
- Files: `internal/hippod/pod.go:312-389`, `internal/hippod/executor.go:194-236`
- Risk: A refactor that bypasses the staged path would re-introduce the partial-copy race silently.
- Priority: Medium.

**Concurrent signal handlers in one process:**
- What's not tested: Two `CreateHelmPod` calls in the same process register two signal handlers; on SIGINT, both run cleanup. No test reproduces the double-cleanup race.
- Files: `internal/hippod/pod.go:191-219`
- Risk: Future code that loops `CreateHelmPod` (e.g. retry-with-fresh-pod) will leak handlers.
- Priority: Low.

**PodSecurity admission rejection paths:**
- What's not tested: When `--privileged` or `--host-network` is set in a cluster with `restricted` PodSecurity, the create call fails. No test asserts the error message is actionable.
- Files: `internal/hippod/spec.go:163-165, 242-244`
- Risk: Users get cryptic "violates PodSecurity" errors and file bug reports.
- Priority: Low.

**`HELM_KUBECONTEXT` mis-set behavior:**
- What's not tested: When the env var names a non-existent kubeconfig context, the user-facing error is buried inside `clientcmd`. `internal/vars_test.go` only covers the happy-path override.
- Files: `internal/vars.go:21-45`
- Risk: Cryptic startup failure.
- Priority: Low.

## Process / Repository Concerns

**Build artifacts committed under `.git`-tracked paths:**
- Files: `inpod` (84 MB), `inpodw`, `coverage.html`, `coverage.out` are present at repo root.
- Status: All are gitignored (`.gitignore`), so they will not be added accidentally, but their presence in working trees confuses tooling that scans the repo (e.g. `grep`, IDE indexers).
- Recommendation: Remove these artifacts during `make clean`; consider an explicit clean target.

**`CLAUDE.md` duplicates `AGENTS.md` content:**
- Files: `CLAUDE.md`, `AGENTS.md`.
- Status: `AGENTS.md` is the canonical knowledge base; `CLAUDE.md` predates it. Both are tracked.
- Risk: Drift between the two leads to contradictory guidance for different AI tools.
- Recommendation: Replace `CLAUDE.md` content with a symlink-style `See AGENTS.md` pointer, or delete it.

**Only one `//nolint` in the entire codebase, in `internal/cmdoptions/envfile.go:44`:**
- Status: Complies with `nolintlint`'s `require-explanation: true` and `require-specific: true` (specifies `errcheck` and "read-only file"). No tech debt here, but worth noting because the codebase is unusually clean of suppressions and any future addition should follow the same format.

---

*Concerns audit: 2026-05-20*
