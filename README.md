# tiny-aws

A small AWS-like cloud platform.

## Architecture

- **control-plane/registry** (Go, :9000) — node registry (SQLite-backed)
- **control-plane/scheduler** (Go, :9001) — picks healthy compute nodes
- **control-plane/cli** — `tinyaws` command-line tool
- **data-plane/compute/ec2-agent** (Rust, :8080) — compute agent
- **data-plane/storage/object-store** (Rust + C++, :7001) — object storage

## Prerequisites

- Go 1.21+
- Rust (latest stable)
- CMake + C++17 compiler (Visual Studio Build Tools on Windows)

## Start order (important)

1. Registry first
2. EC2 agent second
3. Object store third
4. Scheduler (optional, after registry + agent)

## Run locally

### Terminal 1 — Registry

```powershell
cd control-plane/registry
go run .
```

Wait for: `node registry listening on :9000`

### Terminal 2 — EC2 Agent

```powershell
cd data-plane/compute/ec2-agent
cargo run
```

### Terminal 3 — Object Store

```powershell
cd data-plane/storage/object-store
cargo run
```

### Terminal 4 — Scheduler (optional)

```powershell
cd control-plane/scheduler
go run .
```

## CLI

```powershell
cd control-plane/cli
go run . node list
go run . node list --role compute
go run . object put my-key --data "hello"
go run . object get my-key
go run . job submit "echo hello"
go run . job status job-1 --wait
```

## Verify cluster

```powershell
curl.exe http://127.0.0.1:9000/health
curl.exe http://127.0.0.1:8080/health
curl.exe http://127.0.0.1:9001/health
curl.exe http://127.0.0.1:9000/nodes
curl.exe http://127.0.0.1:9000/nodes?role=compute
curl.exe http://127.0.0.1:9000/nodes?role=storage
curl.exe http://127.0.0.1:9001/schedule
```

## Integration smoke test

Start the full stack, then run:

```powershell
.\scripts\run-local.ps1
# wait for all services to start, then:
.\tests\integration\smoke-test.ps1
```

## Scheduler API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Scheduler health |
| GET | `/schedule` | Pick a healthy compute node |
| POST | `/jobs` | Submit a job (`{"command":"echo hello"}`) |
| GET | `/jobs` | List submitted jobs |

```powershell
curl.exe -X POST http://127.0.0.1:9001/jobs -H "Content-Type: application/json" -d "{\"command\":\"echo hello\"}"
curl.exe http://127.0.0.1:9001/jobs
```

## Test storage

```powershell
curl.exe -X PUT http://127.0.0.1:7001/objects/test-001 -d "hello tiny-aws"
curl.exe http://127.0.0.1:7001/objects/test-001
curl.exe http://127.0.0.1:7001/objects/test-001/meta
curl.exe http://127.0.0.1:7001/objects
```

### Buckets

```powershell
curl.exe -X PUT http://127.0.0.1:7001/buckets/my-bucket
curl.exe -X PUT http://127.0.0.1:7001/buckets/my-bucket/objects/file.txt -d "data"
curl.exe http://127.0.0.1:7001/buckets/my-bucket/objects/file.txt
curl.exe http://127.0.0.1:7001/buckets
```

## Environment variables

| Variable | Default | Used by |
|----------|---------|---------|
| `REGISTRY_URL` | `http://127.0.0.1:9000` | ec2-agent, object-store, scheduler, cli |
| `SCHEDULER_URL` | `http://127.0.0.1:9001` | ec2-agent, cli |
| `OBJECT_STORE_URL` | `http://127.0.0.1:7001` | cli |
| `OBJECT_STORE_ADDR` | `127.0.0.1:7001` | object-store |
| `STORAGE_ROOT` | `data` | object-store |
| `METADATA_DB` | `metadata.db` | object-store |

## Job lifecycle

1. Client submits: `POST /jobs` with `{"command":"echo hello"}`
2. Scheduler assigns to a healthy compute node (`status: pending`)
3. EC2 agent polls: `GET /jobs?node_id=<id>&status=pending`
4. Agent marks running: `PATCH /jobs/{id}` with `{"status":"running"}`
5. Agent runs command locally, reports: `PATCH /jobs/{id}` with `status`, `exit_code`, `stdout`, `stderr`
6. Client checks: `GET /jobs/{id}`

| Status | Meaning |
|--------|---------|
| `pending` | Assigned, waiting for agent |
| `running` | Agent is executing |
| `done` | Finished (exit code 0) |
| `failed` | Command failed |

```powershell
$j = Invoke-RestMethod -Uri "http://127.0.0.1:9001/jobs" -Method Post -ContentType "application/json" -Body '{"command":"echo hello"}'
Invoke-RestMethod "http://127.0.0.1:9001/jobs/$($j.job_id)"