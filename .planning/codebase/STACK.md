# Technology Stack

**Analysis Date:** 2026-05-20

## Languages

**Primary:**
- Go 1.26 — all production code under `cmd/`, `internal/`, and `main.go`. Module path: `github.com/noksa/helm-in-pod` (see `go.mod:1-3`).

**Secondary:**
- POSIX shell (`/usr/bin/env sh`) — embedded in-pod boot wrapper (`internal/hipembedded/script.sh`), install script (`scripts/install.sh`).
- Bash (`/usr/bin/env bash`) — host-side tooling: `Makefile`, `scripts/check.sh`, `scripts/install-local.sh`, `scripts/make_archieve.sh`, `scripts/test-plugin.sh`, `e2e/setup-cluster.sh`, `e2e/teardown-cluster.sh`, `e2e/run-tests.sh`.
- YAML — Helm plugin manifest (`plugin.yaml`), GitHub Actions workflows (`.github/workflows/ci.yml`, `.github/workflows/release.yml`), golangci-lint v2 config (`.golangci.yml`).

## Runtime

**Environment:**
- Host: Go 1.26 binary `bin/in-pod` invoked as a Helm 4 plugin subprocess via `${HELM_PLUGIN_DIR}/bin/in-pod` (`plugin.yaml:13-14`).
- In-pod: any container image that ships `/bin/sh`, `kubectl`, and `helm` v4 — default `docker.io/noksa/kubectl-helm:v1.34.5-v4.1.1` (`cmd/flags.go:98`).
- Plugin distribution: Helm 4 plugin OCI artifact at `ghcr.io/noksa/helm-in-pod/in-pod:<version>` (`.github/workflows/release.yml:115-122`) plus per-platform `.tar.gz` archives attached to GitHub Releases (`scripts/make_archieve.sh:42-47`).

**Package Manager:**
- Go modules — `go.mod` / `go.sum` checked in; `make tidy` wraps `go mod tidy`.
- Helm plugin install hooks invoke `scripts/install.sh` on `install` and `update` (`plugin.yaml:15-19`), which fetches the matching release tarball from GitHub Releases.
- Lockfile: `go.sum` present at repo root (71KB).

## Frameworks

**Core CLI / orchestration:**
- `github.com/spf13/cobra` v1.10.1 — root + every subcommand (`cmd/root.go`, `cmd/exec.go`, `cmd/daemon*.go`).
- `github.com/spf13/viper` v1.17.0 — indirect / config glue inherited from Cobra patterns.
- `github.com/spf13/pflag` v1.0.10 (indirect) — flag parsing.

**Kubernetes client stack:**
- `helm.sh/helm/v4` v4.0.4 — only the `helm.sh/helm/v4/pkg/cli` package is imported (`cmd/exec.go:15`, `cmd/daemon_start.go:12`, `internal/hippod/executor.go:17`) to read the host's Helm `RepositoryConfig` path. Helm 3 is intentionally unsupported.
- `sigs.k8s.io/controller-runtime` v0.23.3 — accessed indirectly through `Noksa/operator-home`.
- `github.com/Noksa/operator-home` v0.18.5-0.20260315163707-6bbd75fa2b1b — wraps controller-runtime's typed/dynamic clients and exposes `operatorkclient.Client`, `SetDefaultConfig`, and `ExecInPod` options used throughout `internal/hippod/` and `internal/hipns/`.
- `k8s.io/client-go` v0.35.0 — pulled in for `tools/clientcmd` (kubeconfig loading in `internal/vars.go:32-36`) and typed clientsets used in tests.
- `k8s.io/api` v0.35.0 — `corev1`, `policy/v1` (PodDisruptionBudget — `internal/hippod/pdb.go`), `rbac/v1` (ClusterRoleBinding — `internal/hipns/namespace.go`).
- `k8s.io/apimachinery` v0.35.0 — `metav1`, labels, `runtime/schema`.

**Logging:**
- `github.com/rs/zerolog` v1.34.0 — console writer with custom `source` field colorizer in `main.go:16-41`; project-local wrappers in `internal/logz/` expose `Host()`, `Pod()`, `HostAndPod()`.
- `github.com/fatih/color` v1.18.0 — ANSI coloring for log sources and inline highlights.

**Testing:**
- `github.com/onsi/ginkgo/v2` v2.28.1 — BDD test runner; invoked via `go run github.com/onsi/ginkgo/v2/ginkgo` from the `Makefile` (`Makefile:36`) to pin to the version in `go.mod`.
- `github.com/onsi/gomega` v1.39.1 — matchers.
- `k8s.io/client-go/kubernetes/fake` (indirect via Noksa/operator-home) — used in unit tests for hippod/hipns Managers.

**Other libraries:**
- `github.com/google/uuid` v1.6.0 — per-process `invocationID` UUID in `hippod.Manager` (`internal/hippod/pod.go:40`).
- `github.com/olekukonko/tablewriter` v1.1.4 — daemon list / status tables (`cmd/table.go`).
- `github.com/noksa/go-helpers` v0.0.0-20221015170552-c776d64423ef — utility helpers.
- `go.uber.org/multierr` v1.11.0 — aggregated cleanup errors.
- `golang.org/x/term` v0.39.0 — terminal detection for `daemon shell`.
- `sigs.k8s.io/yaml` v1.6.0 — YAML marshalling for `--dry-run` pod spec output.

**Build/Dev:**
- `golangci-lint` v2 (`.golangci.yml:1`) — enables `gocritic`, `misspell`, `nolintlint`, `unconvert`, `unparam`, `ginkgolinter`, `zerologlint`. Global build tag `e2e` is set so e2e files are linted too.
- `goimports` (`golang.org/x/tools/cmd/goimports@latest` installed via CI) — with `local-prefixes: github.com/noksa/helm-in-pod` enforced via `.golangci.yml:38-40`.
- `kind` (`helm/kind-action@v1`) — provisions ephemeral clusters for e2e (`.github/workflows/ci.yml:74-77`, `e2e/setup-cluster.sh`).
- `k9s` — optional dev tool, wired into `Makefile:166-172` for the e2e cluster.

## Key Dependencies

**Critical (direct, from `go.mod:5-24`):**
- `github.com/Noksa/operator-home` v0.18.5-0.20260315163707-6bbd75fa2b1b — owns the kube client + pod-exec abstraction; replacing it would force a rewrite of `internal/hippod/executor.go` and the `internal/hipns` manager.
- `helm.sh/helm/v4` v4.0.4 — locates the host `repositories.yaml` for `--copy-repo`; the Helm 3 dependency was removed at v0.9.0.
- `sigs.k8s.io/controller-runtime` v0.23.3 — transitive contract via operator-home; controls REST client semantics.
- `k8s.io/api` v0.35.0, `k8s.io/apimachinery` v0.35.0, `k8s.io/client-go` v0.35.0 — pinned to the same minor across the three modules.
- `github.com/spf13/cobra` v1.10.1 — root command and every subcommand.
- `github.com/rs/zerolog` v1.34.0 — production logger; `main.go` configures the console writer once.
- `github.com/onsi/ginkgo/v2` v2.28.1 + `github.com/onsi/gomega` v1.39.1 — every `_test.go` file is Ginkgo-based.

**Infrastructure / indirect highlights:**
- `github.com/gorilla/websocket` v1.5.4-0.20250319132907-e064f32e3674 — used by `client-go`'s exec stream (SPDY/WebSocket; see comment in `internal/hippod/pod.go:258`).
- `github.com/moby/spdystream` v0.5.0 — alternate exec transport.
- `github.com/fluxcd/cli-utils` v0.36.0-flux.14 — pulled in through operator-home / helm.
- `k8s.io/kubectl` v0.34.2, `k8s.io/cli-runtime` v0.34.2 — used transitively by helm.
- `github.com/prometheus/client_golang` v1.23.2 — indirect via controller-runtime.
- `github.com/google/uuid` v1.6.0 — per-process invocation IDs that label pods.

## Configuration

**Environment variables consumed by `in-pod`:**
- `HELM_KUBECONTEXT` — selects kube-context at startup (`internal/vars.go:25-27`); if unset, falls back to the kubeconfig's `current-context`.
- `HELM_IN_POD_NAMESPACE` — overrides the default `helm-in-pod` namespace (`internal/vars.go:43-45`, default constant at `internal/hipconsts/consts.go:6`).
- `HELM_IN_POD_IMAGE` — default value for the `--image` / `-i` flag (`cmd/flags.go:99-102`, constant name in `internal/hipconsts/consts.go:20`).
- `HELM_IN_POD_DAEMON_NAME` — default daemon name when `--name` is not supplied (`cmd/flags.go` `getDaemonName`, constant in `internal/hipconsts/consts.go:19`).
- `HELM_PLUGIN_DIR` — set by Helm 4; consumed by `scripts/install.sh:5` and the manifest's `platformCommand` (`plugin.yaml:14`).
- `WAIT_COPY_DONE` — set inside the pod by `hippod` when `--copy-from` is requested; checked by `internal/hipembedded/script.sh:53` to emit the `###HIP_EXIT_CODE:N###` marker and block on `/tmp/copy-done`.
- `TIMEOUT` — injected into the pod environment so `script.sh:32` knows how long to wait for the wrapped script.

**Environment files:**
- No `.env` files are read or written by the project itself. Users may pass `--env-file path/to/file` to `exec` / `daemon`; parsing happens in `internal/cmdoptions/` (KEY=VALUE with comments + quotes).

**Build:**
- `Makefile` (canonical workflow) — embeds version/commit/date via ldflags `-X github.com/noksa/helm-in-pod/cmd.{version,commit,date}` (`Makefile:17-20`).
- `scripts/make_archieve.sh` — per-platform builds with `CGO_ENABLED=0`, packages tarballs into `generated/`.
- `plugin.yaml` — declares `apiVersion: v1`, `type: cli/v1`, `runtime: subprocess`, install/update hooks.
- `.golangci.yml` — lint configuration (v2 schema).
- `.github/workflows/ci.yml` — lint + unit + e2e matrix (k8s `v1.28.15`, `v1.30.8`, `v1.32.3`, `v1.35.1`).
- `.github/workflows/release.yml` — per-platform build, `helm plugin package --sign`, `oras push` to GHCR, GitHub Release.

## Platform Requirements

**Development:**
- Go 1.26 toolchain.
- `make`, `bash`, `git`, `tar` (GNU `gtar` auto-detected and preferred on macOS — `scripts/make_archieve.sh:18-21`).
- Docker — for the kind cluster used by e2e.
- `kind` and `kubectl` — installed via `make test-e2e-setup` / `helm/kind-action`.
- Helm 4 CLI — for `make install-local` and `make test-plugin`.
- Optional: `k9s`, `curl` or `wget` (the plugin install script needs one; `tar` is mandatory — `scripts/install.sh:37-58`).

**Production:**
- Helm 4 client (`v4.1.4` is the version used in CI for plugin packaging — `.github/workflows/release.yml:80-82`).
- Kubernetes cluster reachable via the user's active kubeconfig context. Tested against k8s `v1.28.15`, `v1.30.8`, `v1.32.3`, `v1.35.1` (`.github/workflows/ci.yml:58-63`).
- Cluster permission to create: `Namespace`, `ServiceAccount` and `ClusterRoleBinding` (binds the SA to `cluster-admin` — see `internal/hipns/namespace.go:75-95`), `Pod`, and `policy/v1 PodDisruptionBudget`.
- The default in-pod image is pulled from Docker Hub (`docker.io/noksa/kubectl-helm:v1.34.5-v4.1.1`); air-gapped environments must override `HELM_IN_POD_IMAGE` and optionally `--image-pull-secret`.
- Per-platform binaries published: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64 (`.github/workflows/release.yml:22-34`).

---

*Stack analysis: 2026-05-20*
