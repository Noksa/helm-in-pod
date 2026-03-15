# v0.6.0-beta2

## Installation

```shell
helm plugin uninstall in-pod 2>/dev/null || true
# helm 4
helm plugin install https://github.com/Noksa/helm-in-pod --version=v0.6.0-beta2 --verify=false
# helm 3
helm plugin install https://github.com/Noksa/helm-in-pod --version=v0.6.0-beta2
```

## Features

### PodDisruptionBudget Protection
Executor pods are now automatically protected from voluntary disruptions (like node drains) during operations. Disable with `--create-pdb=false` if needed.

### Exit Code Propagation
Commands now properly return their exit codes, enabling correct error handling in CI/CD pipelines and scripts.

### Separate Resource Requests and Limits
New flags for independent CPU and memory configuration:
- `--cpu-request` / `--cpu-limit`
- `--memory-request` / `--memory-limit`

Old `--cpu` and `--memory` flags are deprecated but still work.

### Volume Mounts (`--volume`)
Mount volumes in executor and daemon pods. Format: `type:name:mountPath[:ro]`

Supported types: `pvc`, `secret`, `configmap`, `hostpath`

```bash
helm in-pod exec --volume pvc:my-claim:/data -- helm install ...
helm in-pod exec --volume secret:my-secret:/etc/creds:ro -- helm install ...
helm in-pod exec --volume configmap:my-cm:/etc/config -- helm install ...
helm in-pod exec --volume hostpath:/var/log:/host-logs:ro -- helm install ...
```

Multiple volumes can be specified by repeating the flag:
```bash
helm in-pod exec --volume pvc:data:/data --volume secret:creds:/etc/creds:ro -- helm install ...
```

### Service Account (`--service-account`)
Specify a custom Kubernetes service account for the executor or daemon pod. Defaults to `helm-in-pod`.

```bash
helm in-pod exec --service-account my-sa -- helm install ...
helm in-pod daemon start --name my-daemon --service-account my-sa
```

### Dry Run (`--dry-run`)
Print the pod spec as YAML without creating anything. Works with both `exec` and `daemon start`.

```bash
helm in-pod exec --dry-run -- helm install ...
helm in-pod daemon start --name my-daemon --dry-run
```

### Copy From Pod (`--copy-from`)
Copy files or directories from the pod back to the host after command execution. Format: `/pod/path:/host/path`. Works with both `exec` and `daemon exec`. Files are copied even if the command fails — useful for test artifacts, logs, etc.

```bash
helm in-pod exec --copy-from /tmp/output.yaml:./output.yaml -- helm template ...
helm in-pod exec --copy-from /tmp/report.html:./report.html \
                 --copy-from /tmp/logs:/tmp/local-logs -- my-command
helm in-pod daemon exec --name my-daemon \
                 --copy-from /tmp/result.txt:./result.txt -- generate-report
```

### Daemon Status & List Commands
New subcommands for managing daemon pods:
- `helm in-pod daemon list` (alias: `ls`) — shows all daemon pods in a table with name, pod, phase, node, age, helm version, and image
- `helm in-pod daemon status --name <name>` — shows detailed status of a specific daemon pod

## Testing

- 251 unit test specs (up from 179) covering all new features
- New e2e test suites: volume mounts, service account, dry-run, daemon status/list, copy-from

## Examples

```bash
# Set different requests and limits
helm in-pod exec --cpu-request 500m --cpu-limit 2000m \
                 --memory-request 512Mi --memory-limit 2Gi -- helm install ...

# Disable PDB if needed
helm in-pod exec --create-pdb=false -- helm install ...

# Mount a PVC and a read-only secret
helm in-pod exec --volume pvc:my-data:/data \
                 --volume secret:my-secret:/etc/creds:ro -- helm install ...

# Use a custom service account
helm in-pod exec --service-account deploy-sa -- helm upgrade myapp repo/chart

# Preview the pod spec without creating it
helm in-pod exec --dry-run -- helm install myapp repo/chart

# Copy test artifacts back from the pod
helm in-pod exec --copy-from /tmp/results.xml:./results.xml -- run-tests

# List all running daemons
helm in-pod daemon list

# Check status of a specific daemon
helm in-pod daemon status --name my-daemon
```
