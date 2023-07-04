# helm-in-pod

### A Helm plugin to run any command (helm/kubectl/etc) inside a k8s cluster

---

#### Why?

When `helm` runs its commands, it does it on localhost. If a k8s-cluster is far away from the client, it may take a lot of time (especially if a helm-release is very big and contains a lot of manifests)

This plugin solves this problem - `helm` will be run inside k8s cluster where network latency between client and api is low as possible

However, this plugin can run **any** command in a pod, not only `helm`

---

#### Requirements

* Helm3 should be installed on host

---

#### How to install/update

Run the following command to install/update the plugin.

You can specify any existing version (check releases)
```shell
(helm plugin uninstall in-pod || true) && helm plugin install --version "main" https://github.com/Noksa/helm-in-pod
```

---

#### Usage

The plugin contains several sub-commands which can be obtained by running the following command:
```shell
helm in-pod --help
```

To run helm (or any other) commands inside k8s clusters use `exec|run` sub-command

All commands including their arguments should be passed after `--`

Before `--` you can pass flags to `exec|run` command

It is possible to copy any folder/file from host to a pod before running any command

It is possible to set any environment variable in a pod before running any command

It is possible to use custom image (for pod) instead of default one

Check examples to find an appropriate case

---

When `exec|run` command is called, the following things happen:
* A new `helm-in-pod` pod will be created in a kubernetes cluster in `helm-in-pod` namespace
* All existing helm repositories (including private) on host are copied to the pod (so there is no need to add helm repositories in the pod, they are just blindly copied from host)
* Updates from specified (using `--update-repo` flag) helm repositories will be fetched
* Specified directories/files (using `--copy|-c`) will be copied from host to pod
* Specified command (after `--`) will be run

---

##### Examples

* Get all pods
```shell
helm in-pod exec -- "kubectl get pods -A"
```

* List all helm releases
```shell
helm in-pod exec -- "helm list -A"
```

* Install bitnami nginx from a pod without custom values file
```shell
# Add bitnami repo on host first
helm repo add bitnami https://charts.bitnami.com/bitnami --force-update
# Install nginx using a pod in a k8s cluster
# 'helm' is omitted in arguments because it is called automatically
# So, just pass arguments to 'helm' command after '--'

helm in-pod exec --update-repo bitnami -- "helm install -n bitnami-nginx --create-namespace bitnami/nginx nginx"
```

* Install/upgrade bitnami nginx from a pod **with custom values file**
```shell
helm repo add bitnami https://charts.bitnami.com/bitnami --force-update
# Note that we use --copy flag to copy custom values file from host to a pod using specific path
# In '--values|-f' flag in helm we should specify path in the pod (NOT ON HOST) where we copied the file in '--copy' flag 
helm in-pod exec --copy /home/alexandr/bitnami/nginx_values.yaml:/tmp/nginx_values.yaml \
--update-repo bitnami -- "helm upgrade -i -n bitnami-nginx --create-namespace bitnami/nginx nginx -f /tmp/nginx_values.yaml"
```

* Install/upgrade bitnami nginx from a pod **with custom values file** and **sql** backend for helm releases
```shell
helm repo add bitnami https://charts.bitnami.com/bitnami --force-update
# Note that we use '--env|-e' flag to set environment variables in a container with helm
# to use SQL backend instead of secrets  
helm in-pod exec -e "HELM_DRIVER=sql" \
-e "HELM_DRIVER_SQL_CONNECTION_STRING=postgresql://helmpostgres.helmpostgres:5432/db?user=user&password=password" \
--copy /home/alexandr/bitnami/nginx_values.yaml:/tmp/nginx_values.yaml \
--update-repo bitnami -- "helm upgrade -i -n bitnami-nginx --create-namespace bitnami/nginx nginx -f /tmp/nginx_values.yaml"
```

* Install/upgrade bitnami nginx from a pod **with custom values file** and **sql** backend for helm releases
```shell
helm repo add bitnami https://charts.bitnami.com/bitnami --force-update
# Note that we use '--env|-e' flag to set environment variables in a container with helm
# to use SQL backend instead of secrets  
helm in-pod exec -e "HELM_DRIVER=sql" \
-e "HELM_DRIVER_SQL_CONNECTION_STRING=postgresql://helmpostgres.helmpostgres:5432/db?user=user&password=password" \
--copy /home/alexandr/bitnami/nginx_values.yaml:/tmp/nginx_values.yaml \
--update-repo bitnami -- "helm upgrade -i -n bitnami-nginx --create-namespace bitnami/nginx nginx -f /tmp/nginx_values.yaml"
```

* Install/upgrade bitnami nginx from a pod **with custom values file** and **sql** backend for helm releases but substitute env values from host 
```shell
helm repo add bitnami https://charts.bitnami.com/bitnami --force-update
# Note that we use '--subst-env|-s' flag to add environment variables with values from host in a container with helm
# to use SQL backend instead of secrets
export HELM_DRIVER=sql
export HELM_DRIVER_SQL_CONNECTION_STRING=postgresql://helmpostgres.helmpostgres:5432/db?user=user&password=password  
helm in-pod exec -s "HELM_DRIVER,HELM_DRIVER_SQL_CONNECTION_STRING" \
--copy /home/alexandr/bitnami/nginx_values.yaml:/tmp/nginx_values.yaml \
--update-repo bitnami -- "helm upgrade -i -n bitnami-nginx --create-namespace bitnami/nginx nginx -f /tmp/nginx_values.yaml"
```

* Run helm diff **with custom values file**
```shell
helm repo add bitnami https://charts.bitnami.com/bitnami --force-update
# Note that we use '--env|-e' flag to set helm diff environment variables in a container with helm
# to change diff default behaviour  
helm in-pod exec -e "HELM_DIFF_NORMALIZE_MANIFESTS=true,HELM_DIFF_USE_UPGRADE_DRY_RUN=true,HELM_DIFF_THREE_WAY_MERGE=true" \
--copy /home/alexandr/bitnami/nginx_values.yaml:/tmp/nginx_values.yaml \
--update-repo bitnami -- "helm diff upgrade -n bitnami-nginx --create-namespace bitnami/nginx nginx -f /tmp/nginx_values.yaml"
```

* Run kubectl inside k8s
```shell
helm in-pod exec -- "kubectl get pods -A"
```

* Use custom image with specific `helm` version
```shell
helm in-pod exec -i "alpine/helm:3.12.1" -- "helm list -A" 
```

* Use custom image and run any command
```shell
helm in-pod exec -i "alpine:3.18" -- "apk add curl --no-cache && curl google.com" 
```
