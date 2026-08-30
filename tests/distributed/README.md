# Distributed multi-machine setup

## Requirements

- **Machine A** (control plane): registry, scheduler, object-store, api-gateway
- **Machine B+** (compute): ec2-agent — one per machine you want to run jobs on

Both machines must reach each other over TCP on the ports listed below.

## Machine A — control plane

```bash
export OBJECT_STORE_ADDR="0.0.0.0:7001"

cd control-plane/registry  && go run . &   # :9000
cd data-plane/storage/object-store && cargo run &  # :7001
cd control-plane/scheduler && go run . &   # :9001
# optional:
cd control-plane/api && go run . &         # :8000 (gateway)
cd data-plane/networking/load-balancer && go run . &  # :8088
```

## Machine B (and C, D…) — compute agent

```bash
export REGISTRY_URL="http://<machine-a-ip>:9000"
export SCHEDULER_URL="http://<machine-a-ip>:9001"
export OBJECT_STORE_URL="http://<machine-a-ip>:7001"

# IMPORTANT: set the IP that Machine A and the LB can use to reach THIS machine
export AGENT_ADVERTISE_ADDR="<machine-b-ip>"

cd data-plane/compute/ec2-agent && cargo run
```

`AGENT_ADVERTISE_ADDR` is what the load balancer uses to route traffic to your
deployed services. Without it, the LB uses the hostname, which may not be
routable across machines.

## Deploy a service across machines

From any machine with the CLI:

```bash
export REGISTRY_URL="http://<machine-a-ip>:9000"
export SCHEDULER_URL="http://<machine-a-ip>:9001"
export OBJECT_STORE_URL="http://<machine-a-ip>:7001"

tinyaws instance launch
tinyaws deploy ./my-web-app --service --port 3000 --instance i-1

# LB on Machine A routes to the service on Machine B:
curl http://<machine-a-ip>:8088/
```

## Verify

```bash
# healthy nodes — should show Machine B
curl http://<machine-a-ip>:9000/nodes?role=compute

# running services — should show svc-1 with Machine B's IP
curl http://<machine-a-ip>:9000/services

# submit a job — runs on Machine B
curl -X POST http://<machine-a-ip>:9001/jobs \
  -H "Content-Type: application/json" \
  -d '{"command":"hostname"}'
```

## Firewall

Open inbound TCP on each machine:

| Machine | Ports |
|---------|-------|
| A (control) | 9000, 9001, 7001, 8000, 8088 |
| B+ (agent) | 8080, plus any service ports (3000, 8000, etc.) |

## Auth

If `TINYAWS_API_KEY` is set on Machine A, set it on CLI calls too:

```bash
export TINYAWS_API_KEY="your-key"
```

Agent register/heartbeat are exempt from auth — agents don't need the key.
