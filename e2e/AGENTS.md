# e2e/

Build-tagged (`//go:build e2e`) Ginkgo v2 suite. 22 Go files (~4,755 lines): 19 spec files + suite root + 2 helper files. 3 shell scripts orchestrate the kind cluster.

## STRUCTURE

```
e2e/
├── e2e_suite_test.go        # TestE2E + SynchronizedBeforeSuite/AfterSuite (build plugin, install, create namespace; delete namespace + plugin on exit)
├── e2e_test.go              # shared helpers: createNamespace, deleteNamespace, createTestChart, cleanupChart, logOnFailure, runDiagnostic
├── utils.go                 # Run(cmd), RunWithExitCode(cmd), BuildHelmInPodCommand(...), BuildDaemonStartCommand(...), GetProjectDir, LoadImageToKindClusterWithName
├── setup-cluster.sh         # cyber-themed kind cluster bootstrap; honors KIND_NODE_IMAGE; sets inotify sysctls
├── run-tests.sh             # forces `go run .../ginkgo --tags=e2e --procs=${GINKGO_PROCS:-5} --timeout=20m`
├── teardown-cluster.sh      # kind delete cluster helm-in-pod-e2e
└── <feature>_test.go        # one feature per file (active_deadline, bundle_copy, copy_flags, copy_from, daemon_flags, daemon_status_list, dry_run, env_flags, exitcode, helm_commands, helm_diff, helm_repo, keep_pod, kubecontext, new_flags, pdb, service_account, timeout, volume)
```

## WHERE TO LOOK

| Concern                                        | File                                        |
|------------------------------------------------|---------------------------------------------|
| Add a new feature spec                         | `<feature>_test.go` (one Describe per file) |
| Change how the plugin is built/installed for tests | `e2e_suite_test.go:SynchronizedBeforeSuite` |
| Cluster name / kubeconfig path / KIND_NODE_IMAGE handling | `../scripts/common.sh` (sourced by `setup-cluster.sh`) |
| Add a kube/helm shell-out helper               | `utils.go`                                  |
| Add a per-test chart fixture                   | `e2e_test.go:createTestChart` (creates ephemeral charts on disk) |

## CONVENTIONS

- **Every file MUST start with `//go:build e2e`**. Unit `make lint` skips e2e, but `scripts/check.sh` runs vet/modernize/golangci-lint with `-tags=e2e` so the suite must compile cleanly.
- **Use `utils.Run(cmd)` / `RunWithExitCode(cmd)`** for every shell-out, never raw `exec.Cmd.Run()`. `Run` sets `cmd.Dir` to the project root and forwards `os.Environ()`.
- **Use `utils.BuildHelmInPodCommand(...)` / `BuildDaemonStartCommand(...)`** to construct `helm in-pod ...` calls — they centralize the resource/timeout defaults that keep tests light on CI (no copy-repo, low CPU/memory requests).
- **Generate unique test names** with `generateNamespace(prefix)` / `generateReleaseName(prefix)` / `generateTestLabel()` so parallel Ginkgo workers don't collide.
- **Diagnostics on failure**: `e2e_test.go:logOnFailure` dumps cluster state into `e2e-reports/` (overridable with `E2E_REPORTS_DIR`). CI uploads this directory only when the job fails.
- **Synchronized suite hooks**: cluster-touching setup goes in the `SynchronizedBeforeSuite` first callback (runs on proc 1 only); per-process setup goes in the second callback.

## ANTI-PATTERNS

- **Do not assume `--copy-repo` defaults to true.** `exec` and `daemon start` default to `true`, but `daemon exec` defaults to `false`. The e2e helpers in `utils.go` pass `--copy-repo=false` explicitly to keep CI fast — don't override that without a reason.
- **Do not delete the `helm-in-pod` namespace yourself** in test teardown. `SynchronizedAfterSuite` does it once at the end; per-test deletes will break parallel workers.
- **Do not skip `helm in-pod purge --all`** at the end of long-running test sessions. Tests that leave `--keep-pod` resources behind are caught by `pdb_test.go` cleanup assertions.
- **Do not assume serial execution.** Default `GINKGO_PROCS=5` parallelizes specs across processes; anything sharing a namespace name or release name across specs will flake. Set `GINKGO_PROCS=1` (via `make test-e2e-serial`) for explicit serial runs.
- **Do not install the kind-action with `install_only: false`** — CI uses `install_only: true` and `make test-e2e-setup` actually creates the cluster, because `setup-cluster.sh` applies inotify sysctls inside the nodes which the action skips.

## NOTES

- Kind cluster name: `helm-in-pod-e2e` (from `scripts/common.sh:E2E_CLUSTER_NAME`). Reused between `make test-e2e` invocations.
- K8s matrix (CI): `v1.28.15`, `v1.30.8`, `v1.32.3`, `v1.35.1`. Each runs as a separate matrix job.
- `helm diff` plugin is auto-installed by the suite (`e2e_suite_test.go`) because `helm_diff_test.go` depends on it.
- Largest specs: `active_deadline_test.go` (529), `pdb_test.go` (484), `env_flags_test.go` (360), `dry_run_test.go` (349). Most others are 80-265 lines.
