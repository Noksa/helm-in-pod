# Release Process

## Overview

1. Write `RELEASE_NOTES.md`
2. Bump version in `plugin.yaml`
3. Commit & push to `main`
4. Create & push tag

## Step 1: Write RELEASE_NOTES.md

Overwrite the file with notes for the new version. Follow this structure:

```markdown
## v0.X.0

### ⚡ Performance

- **Short title** — one-sentence explanation of what changed and why it matters to users.

### 🛡️ Reliability

- **Short title** — one-sentence explanation.

### ✨ Features

- **Short title** — one-sentence explanation.

### 🐛 Bug Fixes

- **Short title** — one-sentence explanation.

### 🧪 Tested Kubernetes Versions

E2E tests run against: **v1.28**, **v1.30**, **v1.32**, **v1.35**

### 🔀 Merged Pull Requests

- [#N](https://github.com/Noksa/helm-in-pod/pull/N) — Short description
- [#M](https://github.com/Noksa/helm-in-pod/pull/M) — Short description

### 🙏 Contributors

- [@username](https://github.com/username)

---

**Full changelog**: compare [`vPREV`...`vNEW`](https://github.com/Noksa/helm-in-pod/compare/vPREV...vNEW) on GitHub.
```

### Guidelines

- Only include sections that have content (skip empty categories).
- Write for users, not developers — explain the impact, not the implementation.
- Bold the title of each bullet, then a dash and a plain-English sentence.
- Keep the PR list for traceability; link every PR.
- Update the "Tested Kubernetes Versions" line from CI matrix.

### Example

See the current [RELEASE_NOTES.md](../RELEASE_NOTES.md) for a real example.

## Step 2: Bump version

Edit `plugin.yaml`:

```yaml
version: "0.X.0"
```

## Step 3: Commit & push

```bash
git add plugin.yaml RELEASE_NOTES.md
git commit -m "chore: bump version to 0.X.0"
git push
```

## Step 4: Tag & push

```bash
git tag v0.X.0
git push origin v0.X.0
```

The release workflow will build binaries and create a GitHub Release with auto-generated notes from PR titles.

## Step 5: Update release notes

1. Go to https://github.com/Noksa/helm-in-pod/releases
2. Edit the newly created release
3. Replace the auto-generated body with the content from `RELEASE_NOTES.md`
4. Save

## After release

Wait ~2 minutes for CI to finish. Verify at https://github.com/Noksa/helm-in-pod/releases.
