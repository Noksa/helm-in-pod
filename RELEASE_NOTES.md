## v0.8.1

### 🐛 Bug Fixes

- **Fix --copy with relative paths containing `..`** — when the source path passed to `--copy` contained `../..` components (common in Ansible playbooks referencing charts via relative paths), files were copied to their original absolute paths instead of the intended destination inside the pod. This caused `helm upgrade` to fail with "unable to detect chart at /tmp/chart/Chart.yaml: no such file or directory".

### 🧪 Tested Kubernetes Versions

E2E tests run against: **v1.28**, **v1.30**, **v1.32**, **v1.35**

### 🔀 Merged Pull Requests

- [#24](https://github.com/Noksa/helm-in-pod/issues/24) — fix(helmtar): --copy fails when source path contains .. components

### 🙏 Contributors

- [@adanilin](https://github.com/adanilin)

---

**Full changelog**: compare [`v0.8.0`...`v0.8.1`](https://github.com/Noksa/helm-in-pod/compare/v0.8.0...v0.8.1) on GitHub.
