# tiny-aws

A small AWS-like cloud platform.

## Architecture

- **control-plane/registry** (Go, :9000) — node registry (SQLite-backed)
- **control-plane/scheduler** (Go, :9001) — picks healthy compute nodes
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

## Verify cluster

```powershell
curl.exe http://127.0.0.1:9000/nodes
curl.exe http://127.0.0.1:9000/nodes?role=compute
curl.exe http://127.0.0.1:9000/nodes?role=storage
curl.exe http://127.0.0.1:9001/schedule
```

## Test storage

```powershell
curl.exe -X PUT http://127.0.0.1:7001/objects/test-001 -d "hello tiny-aws"
curl.exe http://127.0.0.1:7001/objects/test-001
curl.exe http://127.0.0.1:7001/objects/test-001/meta
curl.exe http://127.0.0.1:7001/objects
```

## Environment variables

| Variable | Default | Used by |
|----------|---------|---------|
| `REGISTRY_URL` | `http://127.0.0.1:9000` | ec2-agent, object-store, scheduler |
| `OBJECT_STORE_ADDR` | `127.0.0.1:7001` | object-store |
| `STORAGE_ROOT` | `data` | object-store |
| `METADATA_DB` | `metadata.db` | object-store |
