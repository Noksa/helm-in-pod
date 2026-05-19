# internal/hippod/

Pod lifecycle, exec/copy I/O, PDB management, and interactive shell. Largest internal package (~4,270 lines across 15 files); concentrated complexity.

## STRUCTURE

```
hippod/
├── pod.go              # Manager (40), CreateHelmPod, CreateDaemonPod, DeleteHelmPods, DeleteKeptPods, GetDaemonPod, ListDaemonPods, GetDaemonStatus, AnnotatePod, CopyFilesBundleWithBootInfo, CopyFileFromPod
├── spec.go             # buildPodSpec, parseVolume (pvc/secret/configmap/hostpath), PrintPodSpecYAML
├── executor.go         # GetPodUserInfo, SyncHelmRepositories, ExecuteCommand, ExecuteCommandInDaemon, SignalCopyDone, OpenInteractiveShell helpers
├── pdb.go              # CreatePodDisruptionBudget, DeletePodDisruptionBudgets, deleteAllPodDisruptionBudgets (used by purge --all)
├── terminal.go         # raw-mode TTY plumbing for `daemon shell`
└── *_test.go           # 9 unit-test files: spec, pod, pod_label, pdb, pdb_crud, executor, extract, volume, wait_pod
```

## WHERE TO LOOK

| Concern                                       | File                                  |
|-----------------------------------------------|---------------------------------------|
| Add a new pod env-var / resource / volume     | `spec.go:buildPodSpec`                |
| Change boot command (which paths get moved)   | `spec.go` boot script (uses `hipembedded.GetShScript`) |
| Change file-copy retry/atomicity              | `pod.go:CopyFilesBundleWithBootInfo`  |
| Change exit-code marker parsing               | `executor.go:ExecuteCommand` + `hiperrors/exitcode.go` |
| Change PDB lifecycle                          | `pdb.go`                              |
| Change interactive shell handling             | `terminal.go`                         |
| Add a new volume type (e.g. `csi:`)           | `spec.go:parseVolume` switch          |

## CONVENTIONS

- **Always wrap kube-API calls in `hipretry.RetryWithContext(m.ctx, attempts, fn)`** — see `executor.go:GetPodUserInfo` for the canonical 3-retry pattern.
- **Use `m.client()` accessor** — never `operatorkclient.DefaultClient()` directly. Tests inject `m.kclient`; production gets the default.
- **Selectors must include host + invocation**: `DeleteHelmPods` filters by `myHostname` + per-process `invocationID` UUID. Adding a new "delete pods" path? Match that pattern or you'll kill another process's pod.
- **`buildPodSpec` is exported via `PrintPodSpecYAML`** for `--dry-run`; do not duplicate the spec elsewhere.
- **The single container is always named `hipconsts.ContainerName`** (`"helm-in-pod"`) — `ExecInPod` calls need that exact name even when `Namespace` is overridden.
- **Boot info collection is consolidated**: `CopyFilesBundleWithBootInfo` returns HOME, whoami, id, helm-found, helm4 in ONE `kubectl exec` call. Do not split this back into multiple round-trips.

## ANTI-PATTERNS

- **Do not force-delete Running/Pending pods.** `pod.go:95-99` deliberately leaves grace period default unless phase is `Succeeded` or `Failed`. Force-deleting Running pods strands containers on the node.
- **Do not bypass `SignalCopyDone`.** When `--copy-from` is set, the in-pod `script.sh` emits `###HIP_EXIT_CODE:N###` and blocks waiting for `/tmp/copy-done` (300s max). If the host doesn't drop that sentinel, the pod will sit there until either copy-done appears or the script's internal timeout fires.
- **Do not stage script and "trigger" paths in two different exec calls.** `helmtar` bundles everything; the boot command in `spec.go` does the atomic `mv /tmp/hip-staged-script.sh /tmp/hip-wrapped-script.sh` at the very end. Splitting this re-introduces the partial-copy race.
- **Do not log inside retry callbacks.** `RetryWithContext` is silent by design; logging from inside causes one log per attempt.
- **Tests must NOT call `operatorkclient.SetDefaultConfig`.** Inject a fake `kclient` directly on the Manager struct.
