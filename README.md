# tiny-aws

A small AWS-like cloud platform.

## Architecture

- **control-plane/registry** (Go, :9000) — node registry
- **data-plane/compute/ec2-agent** (Rust, :8080) — compute agent
- **data-plane/storage/object-store** (Rust + C++, :7001) — object storage

## Prerequisites

- Go 1.21+
- Rust (latest stable)
- CMake + C++17 compiler (Visual Studio Build Tools on Windows)

## Run locally

### Terminal 1 — Registry

```powershell
cd control-plane/registry
go run .