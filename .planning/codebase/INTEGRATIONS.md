# External Integrations

**Analysis Date:** 2026-05-20

## APIs & External Services

**Kubernetes API (primary integration):**
- Service: the user's Kubernetes API server, reached via the active kubeconfig context.
  - SDK/Client: `sigs.k8s.io/controller-runtime` v0.23.3 + `k8s.io/client-go` v0.35.0, wrapped by `github.com/Noksa/operator-home`'s `operatorkclient.Client` (`internal/vars.go:32-41`).
  - Auth: standard kubeconfig discovery (`clientcmd.NewDefaultClientConfigLoadingRules` in `internal/vars.go:32-36`). Context override via `HELM_KUBECONTEXT` env var (`internal/vars.go:23-28`).
  - Resources created/used:
    - `core/v1.Namespace` — `helm-in-pod` (or `$HELM_IN_POD_NAMESPACE`) created in `internal/hipns/namespace.go:44+`.
    - `core/v1.ServiceAccount` — same name as the namespace (`internal/hipns/namespace.go:59-66`).
    - `rbac.authorization.k8s.io/v1.ClusterRoleBinding` — binds the SA to the built-in `cluster-admin` ClusterRole (`internal/hipns/namespace.go:77-95`); effectiveness is verified with `SelfSubjectAccessReview` polling (`waitForClusterRoleBindingEffective`).
    - `core/v1.Pod` — short-lived (exec mode) or long-lived (daemon mode), labeled with `helm-in-pod/operation-id=<UUID>` and `app.kubernetes.io/managed-by=helm-in-pod` (constants in `internal/hipconsts/consts.go:23-25`).
    - `policy/v1.PodDisruptionBudget` — optionally created per pod (`internal/hippod/pdb.go`); guarantees the pod survives voluntary disruptions during long operations.
- Streaming exec: uses `kubectl exec`-style remote shell streams (SPDY or WebSocket — see `internal/hippod/pod.go:258` comment); transport libraries are `github.com/moby/spdystream` v0.5.0 and `github.com/gorilla/websocket` v1.5.4-0.20250319132907-e064f32e3674 (both indirect).
- File copy into pod: a single tar bundle is streamed over `stdin` to a `tar -x` invocation inside the pod via `operatorkclient.ExecInPod` with `WithStdin` (`internal/hippod/pod.go:344-347`).
- File copy out of pod: `tar -c` inside the pod streamed back over `stdout` (`internal/hippod/pod.go:470-473`), unpacked on the host. Exit code is communicated via a `###HIP_EXIT_CODE:N###` marker emitted by the embedded `script.sh:54`.

**Helm 4 CLI (host-side library use):**
- Service: local Helm 4 client configuration. The plugin **does not** invoke Helm; it only reads Helm's settings.
  - SDK: `helm.sh/helm/v4/pkg/cli` v4.0.4 (`cmd/exec.go:15`, `cmd/daemon_start.go:12`, `internal/hippod/executor.go:17`).
  - Use: `cli.New().RepositoryConfig` resolves the path to the host's `repositories.yaml`, which is then tar-bundled and copied into the pod (`cmd/exec.go:141-144`, `cmd/daemon_start.go:92-95`). The pod-side boot command moves it to `$HOME/.config/helm/repositories.yaml` (`internal/hippod/pod.go:331`).
- Helm 3 is **not** supported since v0.9.0; `go.mod:18` pins `helm.sh/helm/v4 v4.0.4`.

**Container image registries (read at pod runtime):**
- Default image source: Docker Hub — `docker.io/noksa/kubectl-helm:v1.34.5-v4.1.1` (`cmd/flags.go:98`).
- Image override env var: `HELM_IN_POD_IMAGE` (`internal/hipconsts/consts.go:20`, applied at `cmd/flags.go:99-102`).
- Pull secret: optional `--image-pull-secret <secretName>` flag (`cmd/flags.go:96`); attached to the pod via `corev1.LocalObjectReference` in `internal/hippod/spec.go:220-222`.
- Pull policy: `--pull-policy` (default `IfNotPresent`, `cmd/flags.go:97`).

**GHCR (release-time integration):**
- Service: GitHub Container Registry — `ghcr.io/<owner>/helm-in-pod/in-pod` (`.github/workflows/release.yml:115`).
- Artifact: Helm 4 plugin OCI artifact, media-type `application/vnd.helm.plugin.v1+json` (`.github/workflows/release.yml:117`).
- Tool: ORAS CLI via `oras-project/setup-oras@v1` (`.github/workflows/release.yml:84-85`).
- Auth: `docker/login-action@v3` with `GITHUB_TOKEN` (`.github/workflows/release.yml:87-92`).
- Tags: `${VERSION}` always; `latest` only for non-prerelease tags (`.github/workflows/release.yml:120-122`).

**GitHub Releases (binary distribution):**
- Service: GitHub Releases on `github.com/Noksa/helm-in-pod`.
- Tool: `softprops/action-gh-release@v2` (`.github/workflows/release.yml:160`).
- Assets: `helm-in-pod_<version>_<os>_<arch>.tar.gz` for {linux,darwin,windows} × {amd64,arm64} (`scripts/make_archieve.sh:23-48`), plus the signed Helm plugin tarball `in-pod-<version>.tgz` and its `.prov` provenance file.
- Auto-generated release notes: enabled (`generate_release_notes: true` — `.github/workflows/release.yml:167`); maintainer manually replaces with curated `RELEASE_NOTES.md` (`docs/RELEASING.md`).
- The end-user install script `scripts/install.sh` downloads the matching `*.tar.gz` from `https://github.com/Noksa/helm-in-pod/releases/download/v<version>/...` using `curl` or `wget` (`scripts/install.sh:27`, `:44-58`), with a fallback to the latest release if the tag isn't published yet (`scripts/install.sh:60-87`).

**Cyberpunk theme (build-time fetch):**
- Service: `raw.githubusercontent.com/Noksa/install-scripts/main/cyberpunk.sh` (`Makefile:8`).
- Cached as `.cyber.sh` at the repo root on first `make` invocation; refreshed via `make cyber-update`.

## Data Storage

**Databases:**
- None. The plugin is stateless on the host.

**File Storage:**
- Local filesystem only:
  - Host: reads the kubeconfig (via `clientcmd`), Helm's `repositories.yaml`, and any `--copy` / `--env-file` sources. Writes `--copy-from` destinations and the build artifacts in `generated/`.
  - In-pod: writes to `/tmp/hip-*` sentinel paths and the wrapped script at `/tmp/hip-wrapped-script.sh` (constants in `internal/hipconsts/consts.go:28-46`).
- Optional pod-side volumes (declared by the user, not provisioned by the plugin) via `--volume type:name:mountPath[:ro]` for `pvc`, `secret`, `configmap`, `hostpath` types (`cmd/flags.go:103`, implemented in `internal/hippod/spec.go`).

**Caching:**
- No external cache.
- In-memory only: the per-process `invocationID` UUID used to label pods and PDBs so concurrent invocations cannot delete each other's resources (`internal/hippod/pod.go:40`).

## Authentication & Identity

**Auth Provider:**
- Kubernetes API: whatever the user's active kubeconfig negotiates (token, certificate, exec plugin, OIDC, cloud-provider auth). The plugin only consumes the resolved `*rest.Config` (`internal/vars.go:36-41`) and never reads or rewrites credentials.
- In-pod RBAC: the in-pod ServiceAccount is bound to the built-in `cluster-admin` ClusterRole (`internal/hipns/namespace.go:83-94`). This is intentional — `helm-in-pod`'s purpose is to give Helm/kubectl inside the pod the same authority the user already has.
- GHCR auth (CI only): `secrets.GITHUB_TOKEN` injected by `docker/login-action@v3` (`.github/workflows/release.yml:87-92`).
- Plugin signing identity (CI only): GPG private keyring base64-encoded in `secrets.GPG_KEYRING_BASE64`, signing email in `vars.SIGNING_KEY_EMAIL` (`.github/workflows/release.yml:97-106`). The corresponding public key is committed at `public-key.asc` and is what `helm plugin install --verify` checks against.

## Monitoring & Observability

**Error Tracking:**
- None integrated. Errors surface through zerolog stderr output and exit codes.

**Logs:**
- Structured stderr logging via `github.com/rs/zerolog` v1.34.0 (`main.go:18-41`).
- Project wrappers `logz.Host()` / `logz.Pod()` / `logz.HostAndPod()` colorize a `source` field (`[host]` cyan, `[pod]` magenta) — see `internal/logz/` and the `FormatPartValueByName` in `main.go:23-38`.
- In-pod command stdout/stderr is streamed back via the exec stream to the host's `os.Stdout` / `os.Stderr` (`internal/hippod/executor.go:398-399`).
- Verbose logging is toggled by the persistent `--verbose-logs` flag (`cmd/root.go`).
- Exit codes from the in-pod command are parsed from the `###HIP_EXIT_CODE:N###` marker (constants in `internal/hipconsts/consts.go:31-32`; parser in `internal/hiperrors/exitcode.go`) and propagated to `os.Exit(N)` in `main.go:43-46`.

## CI/CD & Deployment

**Hosting:**
- Source: GitHub — `github.com/Noksa/helm-in-pod`.
- Binary distribution: GitHub Releases (tar.gz per platform) + GHCR OCI artifact for Helm 4 `helm plugin install ghcr.io/...` users.

**CI Pipeline:**
- `.github/workflows/ci.yml` — runs on push to `main` and PRs targeting `main`:
  - `lint` job: `golangci/golangci-lint-action@v9` + `goimports`, then `make lint` (which runs `scripts/check.sh`).
  - `unit-tests` job: `make test-unit` (Ginkgo, `--skip-package=e2e`).
  - `e2e-tests` matrix job: K8s versions `v1.28.15`, `v1.30.8`, `v1.32.3`, `v1.35.1`. Uses `helm/kind-action@v1` (`install_only: true`), then `make test-e2e-setup` → `make test-e2e` → `make test-e2e-teardown`. On failure, uploads `e2e-reports/` via `actions/upload-artifact@v4`.
- `.github/workflows/release.yml` — triggered on tags `v*`:
  - Build job: per-platform binaries via `make binaries TARGET=<os>/<arch>` (matrix of 6 targets).
  - `plugin-package` job: installs Helm 4 (`azure/setup-helm@v4` at `v4.1.4`), `oras-project/setup-oras@v1`, logs into GHCR, runs `helm plugin package --sign --key "$SIGNING_KEY_EMAIL" --keyring secring.gpg plugin-staging`, then `oras push ghcr.io/.../in-pod:${VERSION}` plus a conditional `oras tag ... latest` for non-prereleases.
  - `release` job: downloads all artifacts and creates a GitHub Release with `softprops/action-gh-release@v2`.

## Environment Configuration

**Runtime environment variables (plugin):**
- `HELM_KUBECONTEXT` — kube context override.
- `HELM_IN_POD_NAMESPACE` — default namespace override (defaults to `helm-in-pod`).
- `HELM_IN_POD_IMAGE` — default container image override.
- `HELM_IN_POD_DAEMON_NAME` — default `--name` for daemon subcommands.
- `HELM_PLUGIN_DIR` — provided by Helm 4; used by `plugin.yaml` and `scripts/install.sh`.

**In-pod environment variables (set by the plugin):**
- `TIMEOUT` — passed into the embedded `script.sh` to bound the wait for the wrapped script (`internal/hipembedded/script.sh:32`).
- `WAIT_COPY_DONE` — signals copy-from mode so the pod emits `###HIP_EXIT_CODE:N###` and blocks on `/tmp/copy-done` (`internal/hipconsts/consts.go:35`, `internal/hipembedded/script.sh:53-63`).
- User-provided variables via `--env KEY=VALUE`, `--subst-env NAME` (forward host value), and `--env-file path` (`cmd/flags.go:113-115`).

**CI / release secrets:**
- `secrets.GITHUB_TOKEN` — auto-provided by GitHub Actions for GHCR login and releasing.
- `secrets.GPG_KEYRING_BASE64` — base64-encoded GPG secret keyring for plugin signing.
- `vars.SIGNING_KEY_EMAIL` — identity to sign with.
- `.env*` files: none in the repo (only the `.gitignore` entry pattern); the plugin never reads `.env` at startup.

**Secrets location:**
- GitHub Actions secrets / variables on the `Noksa/helm-in-pod` repository.
- The public PGP key used to verify the signed plugin tarball is committed at `public-key.asc`.
- No long-lived credentials are stored in the repository.

## Webhooks & Callbacks

**Incoming:**
- None. The plugin is a CLI; no HTTP server is started.

**Outgoing:**
- None initiated by the plugin runtime.
- Build/release-time outbound calls only:
  - `scripts/install.sh` → `github.com/Noksa/helm-in-pod/releases/download/...` (and `/releases/latest`) for tarball fetch.
  - `Makefile` → `raw.githubusercontent.com/Noksa/install-scripts/main/cyberpunk.sh` for the theme cache.
  - Release workflow → `ghcr.io` (push), `github.com` (release creation).

---

*Integration audit: 2026-05-20*
