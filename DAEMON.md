# ⚡ Daemon Mode

> 🚀 **NEW**: Run commands in a persistent pod without recreation overhead!

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

All `exec` flags work: `--image`, `--cpu`, `--memory`, `--tolerations`, `--node-selector`, `--env`, `--copy`, etc.

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

# Update repositories
helm in-pod daemon exec --name my-daemon --update-all-repos -- "helm upgrade ..."
```

### 3️⃣ Stop the Daemon

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

# Iterate quickly
helm in-pod daemon exec --name dev -- "helm diff upgrade ..."
helm in-pod daemon exec --name dev -- "helm upgrade --dry-run ..."
helm in-pod daemon exec --name dev -- "helm upgrade ..."

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

## 🎛️ Available Flags

### `daemon start`
All flags from `exec` command:
- Pod creation: `--image`, `--cpu`, `--memory`, `--tolerations`, `--node-selector`, `--host-network`
- Security: `--run-as-user`, `--run-as-group`, `--image-pull-secret`
- Helm: `--copy-repo`, `--update-repo`
- Files: `--copy`
- Environment: `--env`, `--subst-env`

### `daemon exec`
Runtime flags only (pod already exists):
- `--name` - Daemon name (required)
- `--env`, `-e` - Environment variables
- `--subst-env`, `-s` - Substitute from host
- `--copy`, `-c` - Copy files
- `--copy-repo` - Copy/replace helm repos
- `--update-repo` - Update specific repos
- `--update-all-repos` - Update all repos
- `--copy-attempts`, `--update-repo-attempts`

### `daemon stop`
- `--name` - Daemon name (required)

## 🔧 How It Works

1. **Start**: Creates a pod with `sleep infinity` and proper signal handling
2. **Annotate**: Stores user info and helm version in pod annotations
3. **Exec**: Runs commands with environment variables exported in script
4. **Stop**: Gracefully terminates the pod

## 💡 Tips

- **Set `HELM_IN_POD_DAEMON_NAME`** environment variable to avoid repeating `--name` flag
- Use `--update-all-repos` to refresh repositories without copying from host
- Copy files during `exec` for dynamic configurations
- Set environment variables per-command for different contexts
- Run multiple daemons with different names for isolation
- Daemon pods are named `daemon-<name>` - you can see them with `kubectl get pods -n helm-in-pod`

## 🆚 When to Use What?

**Use `exec`** when:
- Running a single command
- Need complete isolation per command
- Don't care about startup time

**Use `daemon`** when:
- Running multiple commands
- Need fast execution
- Want to preserve state (repos, files)
- Working interactively
- CI/CD with multiple steps
