# ⚡ Daemon Mode

> 🚀 Run commands in a persistent pod without recreation overhead!

## 💡 The Problem

Every `helm in-pod exec` command creates a new pod, copies repositories, waits for readiness, executes, and destroys the pod. For multiple operations, this adds significant overhead.

## ✨ The Solution: Daemon Mode

Start a long-running pod once, then execute unlimited commands instantly!

```bash
# Start once
helm in-pod daemon start --name dev --copy-repo

# Execute many times - NO pod creation overhead!
helm in-pod daemon exec --name dev -- "helm list -A"
helm in-pod daemon exec --name dev -- "kubectl get pods"
helm in-pod daemon exec --name dev -- "helm upgrade myapp ..."

# Stop when done
helm in-pod daemon stop --name dev
```

## 🎯 Quick Start

### 1️⃣ Start a Daemon

```bash
helm in-pod daemon start --name my-daemon --copy-repo
```

All `exec` flags work: `--image`, `--cpu-request`, `--cpu-limit`, `--memory-request`, `--memory-limit`, `--tolerations`, `--node-selector`, `--env`, `--copy`, `--create-pdb`, etc.

**Force recreate** if daemon already exists:
```bash
helm in-pod daemon start --name my-daemon --copy-repo --force
```

### 2️⃣ Execute Commands

```bash
# Basic command
helm in-pod daemon exec --name my-daemon -- "helm list -A"

# With environment variables
helm in-pod daemon exec --name my-daemon \
  -e HELM_DRIVER=sql \
  -e CONNECTION_STRING="..." -- \
  "helm upgrade myapp repo/chart"

# Copy files on-the-fly
helm in-pod daemon exec --name my-daemon \
  --copy ~/values.yaml:/tmp/values.yaml -- \
  "helm upgrade myapp repo/chart -f /tmp/values.yaml"

# Clean paths before copying (ensures fresh state)
helm in-pod daemon exec --name my-daemon \
  --clean /tmp/config --clean /tmp/data \
  --copy ~/config:/tmp/config -- \
  "helm upgrade myapp repo/chart"

# Update repositories
helm in-pod daemon exec --name my-daemon --update-all-repos -- "helm upgrade ..."
```

> ⚠️ **Important**: In `daemon exec`, `--copy-repo` defaults to `false` (unlike `exec` where it defaults to `true`). This is because the daemon pod typically already has repositories from `daemon start`. Pass `--copy-repo` explicitly if you need to re-sync repositories from the host.

### 3️⃣ Interactive Shell

```bash
# Open an interactive shell with full Helm context
helm in-pod daemon shell --name my-daemon

# Use a different shell (bash, zsh, etc.)
helm in-pod daemon shell --name my-daemon --shell bash
```

All Helm repositories and configurations are already set up. Perfect for:
- Interactive debugging
- Exploring the cluster environment
- Running multiple commands manually
- Testing Helm operations before scripting

Type `exit` or press `Ctrl+D` to close the shell.

### 4️⃣ Stop the Daemon

```bash
helm in-pod daemon stop --name my-daemon
```

## 🌍 Environment Variable

Set `HELM_IN_POD_DAEMON_NAME` to avoid repeating `--name` flag:

```bash
# Set once
export HELM_IN_POD_DAEMON_NAME=dev

# No need for --name anymore!
helm in-pod daemon exec -- "helm list -A"
helm in-pod daemon exec -- "kubectl get pods"
helm in-pod daemon stop
```

Perfect for interactive sessions and CI/CD pipelines!

## 🔥 Use Cases

### Multiple Deployments

```bash
helm in-pod daemon start --name deploy --copy-repo

# Deploy multiple apps without pod recreation
helm in-pod daemon exec --name deploy -- "helm upgrade app1 ..."
helm in-pod daemon exec --name deploy -- "helm upgrade app2 ..."
helm in-pod daemon exec --name deploy -- "helm upgrade app3 ..."

helm in-pod daemon stop --name deploy
```

### Environment-Specific Daemons

```bash
# One daemon per environment
helm in-pod daemon start --name dev --copy-repo
helm in-pod daemon start --name staging --copy-repo
helm in-pod daemon start --name prod --copy-repo

# Execute in specific environment
helm in-pod daemon exec --name prod -- "helm list -A"
```

### CI/CD Pipelines

```bash
# Setup phase
helm in-pod daemon start --name ci --copy-repo --update-all-repos

# Multiple deployment steps
helm in-pod daemon exec --name ci -- "helm upgrade backend ..."
helm in-pod daemon exec --name ci -- "helm upgrade frontend ..."
helm in-pod daemon exec --name ci -- "kubectl rollout status ..."

# Cleanup
helm in-pod daemon stop --name ci
```

### Interactive Development

```bash
# Start daemon with your tools
helm in-pod daemon start --name dev \
  --image custom/kubectl-helm:latest \
  --copy ~/.kube/config:/tmp/kubeconfig \
  --copy-repo

# Option 1: Execute commands one by one
helm in-pod daemon exec --name dev -- "helm diff upgrade ..."
helm in-pod daemon exec --name dev -- "helm upgrade --dry-run ..."
helm in-pod daemon exec --name dev -- "helm upgrade ..."

# Option 2: Open interactive shell for exploration
helm in-pod daemon shell --name dev
# Now you're inside the pod with full Helm context!
# Run commands interactively:
#   helm list -A
#   kubectl get pods
#   helm diff upgrade myapp ...
#   exit

# Keep daemon running for the day
# Stop when done
helm in-pod daemon stop --name dev
```

## 📊 Performance Comparison

| Operation | Classic `exec` | Daemon Mode |
|-----------|----------------|-------------|
| First run | ~10-15s | ~10-15s (start) |
| Second run | ~10-15s | ~1-2s ⚡ |
| Third run | ~10-15s | ~1-2s ⚡ |
| 10 commands | ~100-150s | ~10-15s + 10-20s = **~30s** 🚀 |

## 🔐 RBAC / Cluster Resources

Daemon mode uses the same RBAC setup as `exec` mode. See the [main README](README.md#-rbac--cluster-resources) for the full list of resources created.

> ⚠️ **Security Note**: Daemon pods run with `cluster-admin` privileges, just like `exec` pods.

## 🎛️ Available Flags

### `daemon start`
All flags from `exec` command:
- Pod creation: `--image`, `--cpu-request`, `--cpu-limit`, `--memory-request`, `--memory-limit`, `--tolerations`, `--node-selector`, `--host-network`
- Security: `--run-as-user`, `--run-as-group`, `--image-pull-secret`
- Metadata: `--labels`, `--annotations`
- Image: `--pull-policy` (default: `IfNotPresent`)
- Protection: `--create-pdb` (default: true) - Protects pod from voluntary disruptions
- Helm: `--copy-repo`, `--update-repo`
- Files: `--copy`
- Environment: `--env`, `--subst-env`
- `--force`, `-f` - Force recreate daemon pod if it already exists

### `daemon exec`
Runtime flags only (pod already exists):
- `--name` - Daemon name (required)
- `--env`, `-e` - Environment variables
- `--subst-env`, `-s` - Substitute from host
- `--copy`, `-c` - Copy files
- `--clean` - Paths to delete before copying files (ensures clean state)
- `--copy-repo` - Copy/replace helm repos (**default: false** — unlike `exec` where it defaults to true)
- `--update-repo` - Update specific repos
- `--update-all-repos` - Update all repos
- `--copy-attempts`, `--update-repo-attempts`

> 💡 **Tip**: `--copy-repo` defaults to `false` in `daemon exec` because the daemon pod typically already has repositories from `daemon start`. Use `--copy-repo` explicitly only when you need to re-sync repositories from the host after they've changed.

### `daemon shell`
- `--name` - Daemon name (required)
- `--shell` - Shell to use (default: sh, options: bash, zsh, etc.)

### `daemon stop`
- `--name` - Daemon name (required)

## ⏱️ Timeout Behavior

Timeout works differently across daemon subcommands:

| Subcommand     | Default Timeout | +10 min overhead | What it controls                                    |
|----------------|-----------------|------------------|-----------------------------------------------------|
| `daemon start` | 2h              | ✅ Yes            | Pod lifetime is `--timeout + 10m` (for startup, file copy, etc.) |
| `daemon exec`  | 2h              | ❌ No             | Command execution timeout only (no overhead added)  |
| `daemon stop`  | —               | —                | No timeout behavior                                 |

> 💡 In `daemon start`, the extra 10 minutes ensures the pod stays alive long enough for setup operations (startup probe, file copy, repo sync) before your timeout window begins. In `daemon exec`, the timeout applies directly to the command execution with no additional overhead.

## 🔁 Exit Code Propagation

Like `exec`, daemon mode propagates exit codes from executed commands. See the [main README](README.md#-exit-code-propagation) for details.

```bash
helm in-pod daemon exec --name ci -- "helm upgrade myapp repo/chart"
echo $?  # prints the exit code from the command inside the pod
```

## 🔧 How It Works

1. **Start**: Creates a pod with `sleep infinity` and proper signal handling
2. **Annotate**: Stores user info and helm version in pod annotations
3. **Exec**: Runs commands with environment variables exported in script
4. **Stop**: Gracefully terminates the pod

## 💡 Tips

- **Set `HELM_IN_POD_DAEMON_NAME`** environment variable to avoid repeating `--name` flag
- Use `--update-all-repos` to refresh repositories without copying from host
- Copy files during `exec` for dynamic configurations
- Use `--clean` to remove old files/folders before copying new ones (prevents stale data)
- Set environment variables per-command for different contexts
- Run multiple daemons with different names for isolation
- Daemon pods are named `daemon-<name>` - you can see them with `kubectl get pods -n helm-in-pod`
- **Use `daemon shell`** for interactive debugging and exploration with full Helm context
- Remember that `--copy-repo` defaults to `false` in `daemon exec` — pass it explicitly if you need to re-sync repos

## 🆚 When to Use What?

**Use `exec`** when:
- Running a single command
- Need complete isolation per command
- Don't care about startup time

**Use `daemon exec`** when:
- Running multiple commands
- Need fast execution
- Want to preserve state (repos, files)
- Working in CI/CD with multiple steps

**Use `daemon shell`** when:
- Need interactive debugging
- Exploring cluster environment
- Testing commands before scripting
- Running ad-hoc operations manually
