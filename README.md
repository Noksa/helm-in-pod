# helm-in-pod

### A Helm plugin to run helm commands inside k8s clusters

---

#### Why?

When helm runs its commands, it does it on localhost. If a k8s-cluster is far away from the client, it may take a lot of time (especially if a helm-release is really big and contains a lot of manifests).

This plugin solves this problem because helm runs inside k8s cluster where network latency between client and api is lowest.

