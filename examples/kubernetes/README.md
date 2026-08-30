# Kubernetes on tiny-aws

Deploy a k3s cluster where each node is a real systemd-nspawn container
managed by tiny-aws.

## Prerequisites

```bash
# tiny-aws stack running
./scripts/run-local.sh

# base rootfs bootstrapped (Debian or Ubuntu)
sudo ./scripts/bootstrap-rootfs.sh debian bookworm
# or: sudo ./scripts/bootstrap-rootfs.sh ubuntu jammy

# CLI on PATH
cd control-plane/cli && go install . && cd ../..
```

## Deploy

```bash
chmod +x examples/kubernetes/deploy-cluster.sh
./examples/kubernetes/deploy-cluster.sh
```

Default: 1 server + 2 workers. Change worker count:

```bash
./examples/kubernetes/deploy-cluster.sh --workers 3
```

## What it does

```
tinyaws instance launch --type large   → i-1  (k3s server, 400% CPU, 2GB RAM)
tinyaws instance launch --type medium  → i-2  (k3s worker, 200% CPU, 1GB RAM)
tinyaws instance launch --type medium  → i-3  (k3s worker, 200% CPU, 1GB RAM)

deploys start.sh into each instance:
  i-1: curl -sfL https://get.k3s.io | sh -          (server)
  i-2: K3S_URL=... K3S_TOKEN=... sh -               (agent, joins i-1)
  i-3: K3S_URL=... K3S_TOKEN=... sh -               (agent, joins i-1)
```

Each instance is a real Linux container with its own filesystem and network.
k3s runs inside it. The containers are isolated from each other and from the host.

## After deploy

```bash
# shell into the server
sudo machinectl shell i-1

# inside i-1:
kubectl get nodes
# NAME   STATUS   ROLES                  AGE
# i-1    Ready    control-plane,master   2m
# i-2    Ready    <none>                 1m
# i-3    Ready    <none>                 1m

kubectl run nginx --image=nginx --port=80
kubectl get pods
```

## Resource allocation

| Instance type | CPU quota | RAM   | Good for         |
|---------------|-----------|-------|------------------|
| nano          | 25%       | 128MB | tiny test pods   |
| micro         | 50%       | 256MB | light workloads  |
| small         | 100%      | 512MB | dev nodes        |
| medium        | 200%      | 1GB   | k3s workers      |
| large         | 400%      | 2GB   | k3s server       |

CPU quota is a systemd `CPUQuota` — `200%` means 2 full cores on a multi-core host.

## Elastic scaling

Add a worker at any time:

```bash
tinyaws instance launch --type medium    # i-4
# then deploy the worker join script manually or re-run deploy-cluster.sh
```

Remove a worker:

```bash
tinyaws instance terminate i-4
# k3s will mark the node NotReady automatically after ~40s
```

## Access from outside the host

If `AGENT_ADVERTISE_ADDR` is set to your machine's LAN IP, the kubeconfig
`server` field will point to that IP and you can reach the cluster from
other machines on the network.

```bash
export AGENT_ADVERTISE_ADDR=192.168.1.10
./examples/kubernetes/deploy-cluster.sh
```

Then on another machine:
```bash
export KUBECONFIG=./k3s.yaml   # copied from i-1
kubectl get nodes
```

## Tear down

```bash
tinyaws instance terminate i-1
tinyaws instance terminate i-2
tinyaws instance terminate i-3
# containers stopped, rootfs deleted, resources freed
```
