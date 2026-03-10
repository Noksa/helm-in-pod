# 🚀 helm-in-pod

> ⚡ A Helm plugin to run any command (helm/kubectl/etc) inside a Kubernetes cluster

---

## 🤔 Why?

<details>
<summary>📡 <strong>Network Latency Problem</strong></summary>

When `helm` runs commands from your local machine, network latency to distant Kubernetes clusters can significantly slow down operations, especially with large releases containing many manifests.

**helm-in-pod** solves this by running Helm commands directly inside the cluster, minimizing network latency between the client and Kubernetes API.

</details>

### ✨ Key Benefits

| Feature                      | Description                                         |
|------------------------------|-----------------------------------------------------|
| 🏃‍♂️ **Fast Execution**     | Run commands inside the cluster for minimal latency |
| 🔧 **Any Command**           | Execute helm, kubectl, or any other command         |
| 📦 **Repository Sync**       | Automatically copies all host Helm repositories     |
| 📁 **File Transfer**         | Copy files/folders from host to pod                 |
| 🌍 **Environment Variables** | Set custom environment variables in the pod         |
| 🐳 **Custom Images**         | Use any Docker image for execution                  |

---

## 📋 Requirements

- 🎯 **Helm 3** installed on host machine

---

## 🚀 Installation

<details>
<summary>📥 <strong>Quick Install/Update</strong></summary>

```bash
# Install or update the plugin
(helm plugin uninstall in-pod || true) && helm plugin install --verify=false https://github.com/Noksa/helm-in-pod
```

> 💡 You can specify any existing version from the releases page

</details>

---

## ⚡ NEW: Daemon Mode

> 🔥 **Run multiple commands without pod recreation overhead!**

```bash
# Start once
helm in-pod daemon start --name dev --copy-repo

# Execute many times - INSTANT! ⚡
helm in-pod daemon exec --name dev -- "helm list -A"
helm in-pod daemon exec --name dev -- "helm upgrade myapp ..."

# Or open an interactive shell 🐚
helm in-pod daemon shell --name dev

# Stop when done
helm in-pod daemon stop --name dev
```

**10x faster** for multiple operations! Perfect for CI/CD, interactive development, and batch deployments.

👉 **[Read Full Daemon Mode Documentation](DAEMON.md)**

---

## 📖 Usage

### 🔍 Getting Help

```bash
helm in-pod --help
```

### 🎯 Basic Syntax

```bash
helm in-pod exec [FLAGS] -- "COMMAND"
```

### 🔧 Available Flags

| Flag              | Short | Description                                |
|-------------------|-------|--------------------------------------------|
| `--copy`          | `-c`  | Copy files/folders from host to pod        |
| `--env`           | `-e`  | Set environment variables                  |
| `--subst-env`     | `-s`  | Substitute environment variables from host |
| `--image`         | `-i`  | Use custom Docker image                    |
| `--update-repo`   |       | Update specified Helm repositories         |
| `--tolerations`   |       | Pod tolerations for node taints            |
| `--node-selector` |       | Pod node selectors for node targeting      |
| `--host-network`  |       | Use host network in pod                    |

---

## ⚙️ How It Works

<details>
<summary>🔄 <strong>Execution Flow</strong></summary>

When you run `helm in-pod exec`, the following happens:

1. 🏗️ **Pod Creation**: Creates a new `helm-in-pod` pod in the `helm-in-pod` namespace
2. 📚 **Repository Sync**: Copies all existing Helm repositories from host to pod
3. 🔄 **Repository Updates**: Fetches updates for specified repositories
4. 📁 **File Transfer**: Copies specified files/directories to the pod
5. ▶️ **Command Execution**: Runs your specified command inside the pod

</details>

---

## 💡 Examples

### 🔍 Basic Operations

<details>
<summary><strong>Get all pods</strong></summary>

```bash
helm in-pod exec -- "kubectl get pods -A"
```

</details>

<details>
<summary><strong>List Helm releases</strong></summary>

```bash
helm in-pod exec -- "helm list -A"
```

</details>

### 📦 Installing Charts

<details>
<summary><strong>Simple installation</strong></summary>

```bash
# Add repository on host
helm repo add bitnami https://charts.bitnami.com/bitnami --force-update

# Install from pod
helm in-pod exec --update-repo bitnami -- \
  "helm install -n bitnami-nginx --create-namespace bitnami/nginx nginx"
```

</details>

<details>
<summary><strong>Installation with custom values</strong></summary>

```bash
helm repo add bitnami https://charts.bitnami.com/bitnami --force-update

# Copy values file and install
helm in-pod exec \
  --copy /home/alexandr/bitnami/nginx_values.yaml:/tmp/nginx_values.yaml \
  --update-repo bitnami -- \
  "helm upgrade -i -n bitnami-nginx --create-namespace bitnami/nginx nginx -f /tmp/nginx_values.yaml"
```

> ⚠️ **Important**: Use the pod path (`/tmp/nginx_values.yaml`) in the helm command, not the host path

</details>

### 🗄️ SQL Backend Configuration

<details>
<summary><strong>Using environment variables</strong></summary>

```bash
helm in-pod exec \
  -e "HELM_DRIVER=sql" \
  -e "HELM_DRIVER_SQL_CONNECTION_STRING=postgresql://helmpostgres.helmpostgres:5432/db?user=user&password=password" \
  --copy /home/alexandr/bitnami/nginx_values.yaml:/tmp/nginx_values.yaml \
  --update-repo bitnami -- \
  "helm upgrade -i -n bitnami-nginx --create-namespace bitnami/nginx nginx -f /tmp/nginx_values.yaml"
```

</details>

<details>
<summary><strong>Using host environment substitution</strong></summary>

```bash
# Set environment variables on host
export HELM_DRIVER=sql
export HELM_DRIVER_SQL_CONNECTION_STRING=postgresql://helmpostgres.helmpostgres:5432/db?user=user&password=password

# Use them in pod
helm in-pod exec \
  -s "HELM_DRIVER,HELM_DRIVER_SQL_CONNECTION_STRING" \
  --copy /home/alexandr/bitnami/nginx_values.yaml:/tmp/nginx_values.yaml \
  --update-repo bitnami -- \
  "helm upgrade -i -n bitnami-nginx --create-namespace bitnami/nginx nginx -f /tmp/nginx_values.yaml"
```

</details>

### 🔍 Advanced Operations

<details>
<summary><strong>Using host network</strong></summary>

```bash
# Run with host network for network troubleshooting
helm in-pod exec --host-network -- "kubectl get pods -A"

# Access services on host network
helm in-pod exec --host-network -- "curl http://localhost:6443"

# Test DNS from host perspective
helm in-pod exec --host-network -- "nslookup kubernetes.default.svc.cluster.local"
```

</details>

<details>
<summary><strong>Running on tainted nodes</strong></summary>

```bash
# Tolerate all taints
helm in-pod exec --tolerations "::Exists" -- "helm list -A"

# Tolerate specific key with any effect
helm in-pod exec --tolerations "key=:NoSchedule:Exists" -- "helm list -A"

# Tolerate specific key-value pair
helm in-pod exec --tolerations "key=value:NoSchedule:Equal" -- "helm list -A"

# Multiple tolerations
helm in-pod exec \
  --tolerations "node-role.kubernetes.io/control-plane=:NoSchedule:Exists" \
  --tolerations "dedicated=special:NoExecute:Equal" -- \
  "helm list -A"
```

</details>

<details>
<summary><strong>Targeting specific nodes with node selectors</strong></summary>

```bash
# Run on nodes with specific label
helm in-pod exec --node-selector "disktype=ssd" -- "helm list -A"

# Run on control plane nodes (empty value)
helm in-pod exec --node-selector "node-role.kubernetes.io/control-plane=" -- "helm list -A"

# Multiple node selectors
helm in-pod exec \
  --node-selector "disktype=ssd,environment=production" -- \
  "helm list -A"

# Combine with tolerations for control plane
helm in-pod exec \
  --node-selector "node-role.kubernetes.io/control-plane=" \
  --tolerations "node-role.kubernetes.io/control-plane=:NoSchedule:Exists" -- \
  "helm list -A"
```

</details>

<details>
<summary><strong>Helm diff with custom configuration</strong></summary>

```bash
helm in-pod exec \
  -e "HELM_DIFF_NORMALIZE_MANIFESTS=true,HELM_DIFF_USE_UPGRADE_DRY_RUN=true,HELM_DIFF_THREE_WAY_MERGE=true" \
  --copy /home/alexandr/bitnami/nginx_values.yaml:/tmp/nginx_values.yaml \
  --update-repo bitnami -- \
  "helm diff upgrade -n bitnami-nginx --create-namespace bitnami/nginx nginx -f /tmp/nginx_values.yaml"
```

</details>

<details>
<summary><strong>Custom Docker images</strong></summary>

```bash
# Specific Helm version
helm in-pod exec -i "alpine/helm:3.12.1" -- "helm list -A"

# Custom image with additional tools
helm in-pod exec -i "alpine:3.18" -- "apk add curl --no-cache && curl google.com"
```

</details>
