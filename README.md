<div align="center">

 <img width="200" height="200" alt="patrick_from_screen" src="https://github.com/user-attachments/assets/a9f2887d-8ac5-4e50-9beb-be75b5ae2cb3" />

<h1> tiny-aws</h1>
</br>
The minimum amount of code required to turn your Linux machine into a tiny cloud.

EC2-like compute. Lambda-like functions. Your machine.

</div>

```bash
tinyaws instance launch --type medium
tinyaws deploy ./my-web-app --service --port 3000 --instance i-1
tinyaws service list
curl http://localhost:8088/   # load balancer routes to it
```

---

## What it is

tiny-aws is a self-hosted mini-AWS that runs on a single Linux box (or a small
cluster). It gives you:

| AWS equivalent | tiny-aws |
|----------------|----------|
| EC2 instances | `systemd-nspawn` containers, own filesystem + network |
| S3 | C++ block engine + Rust HTTP API |
| ECS / deploy | `tinyaws deploy ./app --service --port N` |
| ELB | Round-robin reverse proxy |
| SQS | SQLite-backed queue |
| SNS | HTTP fan-out pub/sub |
| IAM | Bearer tokens with roles + expiry |
| VPC / SGs | Metadata + `iptables` rules via network agent |

---

## Quick start (Linux / Arch)

### 1. Install dependencies

**Arch Linux:**
```bash
sudo pacman -S go rust cmake base-devel debootstrap unzip netcat
```

**Debian / Ubuntu:**
```bash
sudo apt install golang-go rustc cargo cmake build-essential debootstrap unzip netcat-openbsd
```

### 2. Get the code and build

```bash
git clone https://github.com/OSS-Initiatives-IIIT-Sonepat/tiny-aws.git
cd tiny-aws

# build Rust crates (2-5 min first time)
cd data-plane/compute/ec2-agent    && cargo build && cd ../..
cd data-plane/storage/object-store && cargo build && cd ../..
```

### 3. Bootstrap base rootfs (for real compute instances)

```bash
# Debian (default)
sudo ./scripts/bootstrap-rootfs.sh

# Ubuntu
sudo ./scripts/bootstrap-rootfs.sh ubuntu jammy

# Custom base dir
sudo TINYAWS_ROOTFS_BASE=/opt/tinyaws/base ./scripts/bootstrap-rootfs.sh
```

This runs `debootstrap` once into `/var/lib/tinyaws/base`. Every instance you
launch clones from this. Takes 3-5 minutes, needs internet.

### 4. Start the stack

```bash
# optionally configure env first
cp .env.example .env.local
# edit .env.local — set TINYAWS_API_KEY, AGENT_ADVERTISE_ADDR, etc.

./scripts/run-local.sh
```

Starts: registry (:9000), ec2-agent (:8080), object-store (:7001),
scheduler (:9001), controller (:9002), load-balancer (:8088).

### 5. Install the CLI

```bash
cd control-plane/cli
go install .
export PATH=$PATH:$(go env GOPATH)/bin
tinyaws --help
```

### 6. Verify

```bash
./tests/integration/smoke-test.sh
```

---

## Deploy your first app

```bash
# create a simple app
mkdir my-app
cat > my-app/start.sh << 'EOF'
#!/bin/bash
python3 -m http.server 3000
EOF
chmod +x my-app/start.sh

# launch an instance (real container)
tinyaws instance launch --type small
# wait ~10s for status to go provisioning → running
tinyaws instance list

# deploy as a long-running service
tinyaws deploy ./my-app --service --port 3000 --instance i-1

# check it's running
tinyaws service list

# hit it through the load balancer
curl http://localhost:8088/

# or direct
curl http://localhost:3000/
```

---

## Instance types

| Type | CPU quota | RAM | Use for |
|------|-----------|-----|---------|
| `nano` | 25% | 128 MB | tiny scripts |
| `micro` | 50% | 256 MB | light APIs |
| `small` | 100% (1 core) | 512 MB | web apps |
| `medium` | 200% (2 cores) | 1 GB | k8s workers |
| `large` | 400% (4 cores) | 2 GB | k8s server |

```bash
tinyaws instance launch --type large
tinyaws instance info i-1      # shows cpu_limit, mem_limit_mb, status
tinyaws instance shell i-1     # prints: sudo machinectl shell i-1
tinyaws instance terminate i-1 # stops container, frees disk
```

---

## Deploy a Kubernetes cluster

```bash
chmod +x examples/kubernetes/deploy-cluster.sh
./examples/kubernetes/deploy-cluster.sh             # 1 server + 2 workers
./examples/kubernetes/deploy-cluster.sh --workers 4 # 1 server + 4 workers
```

Then:
```bash
sudo machinectl shell i-1   # shell into the k3s server
kubectl get nodes
```

See [examples/kubernetes/README.md](examples/kubernetes/README.md) for details.

---

## All CLI commands

```
tinyaws instance launch [--type nano|micro|small|medium|large]
tinyaws instance list
tinyaws instance info <id>
tinyaws instance shell <id>        # prints machinectl command
tinyaws instance terminate <id>

tinyaws deploy <dir> [--instance i-1] [--service] [--port N] [--wait]

tinyaws service list
tinyaws service stop <id>
tinyaws service logs <id>

tinyaws job submit "<cmd>" [--instance i-1]
tinyaws job status <id> [--wait]

tinyaws object put <key> [--data text] [--file path] [--bucket name]
tinyaws object get <key> [--bucket name]
tinyaws bucket create <name>
tinyaws bucket list

tinyaws node list [--role compute|storage] [--healthy-only]
tinyaws storage node list

tinyaws auth set-key <key> [--role admin|readonly] [--expires 2027-01-01T00:00:00Z]
tinyaws auth whoami

tinyaws queue create <name>
tinyaws queue send <name> <message>
tinyaws queue receive <name>

tinyaws vpc create <name> <cidr>
tinyaws subnet create <vpc-id> <cidr>
tinyaws sg create <vpc-id> <name>
tinyaws sg allow <sg-id> inbound tcp 80 0.0.0.0/0

tinyaws lambda create <name> --runtime python3|node20 [--file code.zip]
tinyaws lambda invoke <name> [--event '{"key":"val"}']

tinyaws lb list
```

---

## Auth (optional)

Set a shared API key to require authentication on all APIs:

```bash
# .env.local
TINYAWS_API_KEY=your-secret

# or per-session
export TINYAWS_API_KEY=your-secret
```

Agents (register, heartbeat, job poll) are exempt — they don't need the key.

Add additional keys with roles:
```bash
tinyaws auth set-key readonly-key --role readonly
tinyaws auth set-key temp-key --role admin --expires 2027-01-01T00:00:00Z
tinyaws auth whoami
```

---

## Multi-machine cluster

Run the control plane on one machine, agents on others:

**Machine A (control plane):**
```bash
export OBJECT_STORE_ADDR=0.0.0.0:7001
./scripts/run-local.sh
```

**Machine B, C… (compute agents):**
```bash
export REGISTRY_URL=http://<machine-a-ip>:9000
export SCHEDULER_URL=http://<machine-a-ip>:9001
export OBJECT_STORE_URL=http://<machine-a-ip>:7001
export AGENT_ADVERTISE_ADDR=<this-machine-ip>

cd data-plane/compute/ec2-agent
./target/debug/ec2-agent
```

Jobs distribute across all agents. The load balancer routes traffic to
whichever agent hosts each service.

See [tests/distributed/README.md](tests/distributed/README.md) for full setup.

---

## Service port reference

| Service | Port | What it does |
|---------|------|--------------|
| Registry | :9000 | Node registry, instance records, IAM keys |
| Scheduler | :9001 | Job queue, assignment, retry |
| Controller | :9002 | Workspace cleanup, reconcile loop |
| SQS | :9003 | Message queue (optional) |
| SNS | :9004 | Event pub/sub (optional) |
| Networking | :9005 | VPC/subnet/SG metadata (optional) |
| Metadata | :9006 | Resource aggregator (optional) |
| Lambda | :9007 | Function invoke (optional) |
| API Gateway | :8000 | Single URL for all services (optional) |
| EC2 Agent | :8080 | Compute agent, runs on every node |
| Object Store | :7001 | S3-like storage |
| Load Balancer | :8088 | Routes traffic to deployed services |

---

## Environment variables

See [.env.example](.env.example) for the full list with descriptions.
Key ones:

| Variable | Default | What it does |
|----------|---------|--------------|
| `TINYAWS_API_KEY` | none | Bearer auth on all APIs |
| `AGENT_ADVERTISE_ADDR` | hostname | IP other services use to reach this agent |
| `TINYAWS_ISOLATE` | 0 | Set to `1` to run services in `unshare` namespaces |
| `TINYAWS_ROOTFS_BASE` | `/var/lib/tinyaws/base` | Base rootfs for instances |
| `JOB_TIMEOUT_SECS` | 3600 | How long a job can run before timeout |
| `MAX_JOBS_PER_NODE` | 1 | Concurrent jobs per compute node |
| `REPLICATION_FACTOR` | 1 | Object store replication copies |
