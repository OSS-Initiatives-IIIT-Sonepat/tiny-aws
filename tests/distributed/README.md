# Distributed two-machine setup

## Requirements

- Machine A: runs registry, scheduler, object-store, api-gateway
- Machine B: runs ec2-agent (points to Machine A registry/scheduler)

Both machines must be able to reach each other over TCP.

## Machine A — control plane

```powershell
# set listen addresses to bind on all interfaces
$env:OBJECT_STORE_ADDR = "0.0.0.0:7001"

cd control-plane/registry; go run .   # :9000
cd data-plane/storage/object-store; cargo run   # :7001
cd control-plane/scheduler; go run .   # :9001
```

## Machine B — compute agent

```powershell
$env:REGISTRY_URL   = "http://<machine-a-ip>:9000"
$env:SCHEDULER_URL  = "http://<machine-a-ip>:9001"

cd data-plane/compute/ec2-agent; cargo run
```

## Verify

From Machine A:

```powershell
# should show Machine B's hostname as a healthy compute node
Invoke-RestMethod "http://127.0.0.1:9000/nodes?role=compute"

# submit a job — it will run on Machine B
$j = Invoke-RestMethod -Uri "http://127.0.0.1:9001/jobs" -Method Post `
  -ContentType "application/json" -Body '{"command":"hostname"}'
Invoke-RestMethod "http://127.0.0.1:9001/jobs/$($j.job_id)" | Select-Object status, stdout
```

## Notes

- Firewall: open TCP 9000, 9001, 7001, 8080 between machines.
- Agent register/heartbeat are exempt from API key auth.
- Run the smoke test from Machine A: `.\tests\integration\smoke-test.ps1`
