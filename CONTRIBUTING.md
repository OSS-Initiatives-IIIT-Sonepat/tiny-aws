# Contributing to tiny-aws

## Prerequisites

- Go 1.21+
- Rust (latest stable)
- CMake + C++17 compiler (Visual Studio Build Tools on Windows)

## Build everything

```powershell
cd control-plane/registry   && go build ./...
cd control-plane/scheduler  && go build ./...
cd control-plane/cli        && go build ./...
cd control-plane/api        && go build ./...
cd control-plane/controller && go build ./...
cd data-plane/compute/ec2-agent        && cargo build
cd data-plane/storage/object-store     && cargo build
```

## Running locally

```powershell
.\scripts\run-local.ps1
```

Then verify with:

```powershell
.\tests\integration\smoke-test.ps1
```

## Code conventions

- **Go:** stdlib `net/http` with Go 1.22+ path patterns; `modernc.org/sqlite` for SQLite.
- **Rust:** tokio + axum; `rusqlite` for SQLite.
- **No unrequested abstractions.** One interface per real need.
- **Comments:** `// comment over function` style, matching existing files.
- **Commits:** one logical change per commit; conventional prefixes (`feat`, `fix`, `docs`, `test`, `ci`).
- Do NOT add `Co-authored-by: Cursor` or `Co-authored-by: Claude` to commits.

## Commit message format

```
feat(component): short description
fix(component): what was broken
docs: what was documented
test: what is tested
ci: CI change
```

## Adding a new service

1. Create `control-plane/<name>/main.go` (or `data-plane/...`).
2. `go mod init github.com/OSS-Initiatives-IIIT-Sonepat/tiny-aws/control-plane/<name>`.
3. Add a `GET /health` endpoint returning `{"status":"healthy","service":"<name>"}`.
4. Add a build step in `.github/workflows/ci.yml`.
5. Document the port in `docs/architecture/overview.md`.

## Pull requests

- One feature per PR.
- All Go modules must `go build ./...` without errors.
- All Rust crates must `cargo check` without errors.
- Smoke test must pass: `.\tests\integration\smoke-test.ps1`.
