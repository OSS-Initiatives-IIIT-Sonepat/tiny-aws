# tiny-aws PRD

Product Requirements Document for continuing development.
Repo: https://github.com/OSS-Initiatives-IIIT-Sonepat/tiny-aws

Last updated: 2026-08-31
Overall progress: ~93% of full vision

---

## 1. Vision

**tiny-aws** is a real, self-hosted mini-cloud you run on your own Linux machine
(or a small cluster). You deploy apps to it the same way you'd deploy to AWS:
zip your code, push it, it runs. You get compute, storage, queuing, load
balancing, and a CLI — all on hardware you own.

### The goal in one sentence

```
tinyaws deploy ./my-web-app --service --port 3000
# → app is running, reachable, monitored, restartable
```

### Philosophy

Own every layer. No Kubernetes, no Docker, no JVM. C++ for storage, Rust for
agents, Go for control plane. SQLite for persistence. The complexity budget is
"can be read and understood in a weekend."

### Three layers

```
AWS-like interface   CLI, unified API gateway
Control logic        Registry, Scheduler, IAM, SQS, SNS, Metadata, Networking
Infrastructure       EC2 agent, Object store, Load balancer, Network agent
```

---

## 2. Tech stack

| Component | Language | Port | Persistence |
|-----------|----------|------|-------------|
| Registry | Go 1.21+ | :9000 | SQLite `registry.db` |
| Scheduler | Go 1.21+ | :9001 | SQLite `scheduler.db` |
| Controller | Go | :9002 | none |
| SQS | Go | :9003 | SQLite `sqs.db` |
| SNS | Go | :9004 | SQLite `sns.db` |
| Networking/VPC | Go | :9005 | SQLite `networking.db` |
| Metadata | Go | :9006 | none (fan-out) |
| Lambda | Go | :9007 | SQLite `lambda.db` |
| API Gateway | Go | :8000 | none |
| EC2 Agent | Rust (tokio) | configurable, default :8080 | none |
| Object Store | Rust (axum) + C++ | :7001 | C++ files + SQLite `metadata.db` |
| Load Balancer | Go | :8088 | none |

**Start order (core):** registry → ec2-agent → object-store → scheduler

**Dev scripts:** `scripts/run-local.ps1`, `Makefile`

**CI:** `.github/workflows/ci.yml` (Windows, build all + smoke test + release binary on tag)

---

## 3. What is BUILT and WORKING (as of 2026-08-30)

### 3.1 Registry ✅

- Node registration, heartbeat, health-check, stale-node pruning
- Instance launch/list/terminate (metadata — no real VM spawning)
- Instance ID sequence persisted across restarts
- Multi-key IAM: `api_keys(key, role)` table, `admin`/`readonly` roles
- `POST/DELETE/GET /iam/keys`
- SNS hook on instance launch (fires when `SNS_URL` set)
- Optional bearer auth (`TINYAWS_API_KEY`)

### 3.2 Scheduler ✅

- Job submit, poll, retry (once), 60s timeout
- Round-robin node selection
- `deploy_url` field: agent downloads zip, extracts, runs start script
- `MAX_JOBS_PER_NODE` concurrency cap
- Optional SQS consumer: polls `jobs` queue when `SQS_URL` set
- SNS hook on job done/failed
- Optional bearer auth

**Known bug:** timeout measured from `created_at` not `running_at` — a job
waiting 30s to be picked up only has 30s of real execution time.

### 3.3 EC2 Agent ✅

- Real system discovery (CPU, RAM, hostname via sysinfo crate)
- Registers with registry, heartbeats every 10s
- Polls scheduler every 3s, runs one job at a time
- Runs shell commands: `cmd /C` (Windows), `sh -c` (Linux)
- Deploy flow: download zip → extract → run `start.ps1` (Windows) / `start.sh` (Linux)
- Per-instance workspace: `/tmp/tinyaws/i-N/` created on instance launch
- Process group isolation on Unix (`setpgid`)
- HTTP `/health` and `/info` on hardcoded `127.0.0.1:8080`

**Known bugs / blockers:**
- `libc` crate used for `setpgid` but NOT in `Cargo.toml` → **won't compile on Linux**
- Agent HTTP server hardcoded to `127.0.0.1:8080` → unreachable from outside machine
- `cmd.output().await` blocks until process exits → long-running server freezes the job worker forever
- No service/daemon job type → no way to deploy a web server and keep it running

### 3.4 Object Store ✅

- C++ block engine: real file I/O (`engine/src/block_store.cpp`)
- Rust FFI wrapper, axum HTTP server
- PUT/GET/DELETE/list for flat objects and bucket-scoped objects
- SQLite metadata (size, etag, content-type)
- Registered as storage node with registry
- Bearer auth via axum middleware
- Replication: peer discovery from registry, PUT/GET/DELETE fan-out
- `REPLICATION_FACTOR` env (default 1 = no replication)

**Known limitation:** peer URL hardcoded to port 7001 — multi-port peer storage not supported.

### 3.5 CLI ✅

All commands wired and making real HTTP calls:

```
tinyaws node list [--role compute|storage] [--healthy-only]
tinyaws instance launch|list|terminate|info <id>
tinyaws object put|get <key> [--bucket name] [--data text] [--file path]
tinyaws bucket create|list
tinyaws job submit "<cmd>" [--instance i-N]
tinyaws job status <id> [--wait]
tinyaws deploy <dir> [--instance i-N] [--wait]
tinyaws auth set-key <key> [--role admin|readonly]
tinyaws auth whoami
tinyaws storage node list
tinyaws queue create|send|receive <name>
tinyaws lb create|list
tinyaws vpc create|list
tinyaws subnet create|list
tinyaws sg create|list|allow|deny
tinyaws lambda create|list|invoke
```

**Known bug:** `vpc.go` panics on nil dereference if networking service is unreachable.

Env vars controlling CLI:
- `TINYAWS_API_URL` — use API gateway for all calls (`http://host:8000`)
- `TINYAWS_API_KEY` — bearer token sent on all requests
- `REGISTRY_URL`, `SCHEDULER_URL`, `OBJECT_STORE_URL`, `SQS_URL`, `NETWORKING_URL`, `LAMBDA_URL`, `LB_URL`

### 3.6 API Gateway ✅

- Real `httputil.ReverseProxy` — strips `/v1` prefix, routes to backends
- `/v1/nodes`, `/v1/instances`, `/v1/iam` → registry :9000
- `/v1/jobs`, `/v1/schedule` → scheduler :9001
- `/v1/objects`, `/v1/buckets` → object-store :7001
- `TINYAWS_API_URL=http://host:8000` makes CLI use gateway for all calls

### 3.7 Controller ✅ (narrow scope)

- Polls registry every 15s for terminated instances
- Removes workspace dirs (`/tmp/tinyaws/<id>`)
- `POST /reconcile` for on-demand trigger

### 3.8 SQS ✅

- SQLite-backed queue: create, send, receive, ack/delete
- 30s visibility timeout
- Scheduler polls `jobs` queue when `SQS_URL` set

### 3.9 SNS ✅

- SQLite subscriptions, HTTP fan-out to subscriber endpoints
- Registry fires `instance-launch` topic
- Scheduler fires `job-status` topic on done/failed
- No retry on delivery failure (best-effort)

### 3.10 Load Balancer ✅

- Round-robin reverse proxy: polls registry for healthy compute nodes every 10s
- HTTP health-checks each agent's `/health`
- Forwards all traffic to next healthy agent

### 3.11 Networking/VPC ✅ (metadata only)

- VPC, subnet, route table, security group, SG rules stored in SQLite
- Instance-to-subnet assignment
- **No actual network isolation** — CIDR fields are strings, no kernel involvement

### 3.12 Network Agent ✅ (best-effort)

- Polls networking service every 30s for SG rules
- Applies rules via `netsh` (Windows) or `iptables` (Linux)
- Requires admin/root; only adds rules, never removes stale ones

### 3.13 Lambda ✅ (unsandboxed)

- Function metadata in SQLite
- `handleInvoke`: builds shell command that downloads zip, imports module, calls handler
- Python3 and Node20 runtimes
- Submits as a scheduler job, polls for result
- **No sandbox** — runs with full agent privileges
- **Shell injection risk** if handler name contains quotes

### 3.14 Metadata aggregator ✅

- `GET /resources` — live fan-out to registry, scheduler, networking
- No state of its own

### 3.15 Docs, tests, examples ✅

- Architecture overview, job lifecycle, storage design, ADR 001
- Smoke test (10 steps, auth-aware)
- Replication, queue, LB, SG, lambda integration tests
- Chaos test: kill agent mid-job
- Full-stack demo, S3 demo, cluster demo, hello-lambda
- CONTRIBUTING.md, distributed two-machine guide

### 3.16 Service deploy (long-running apps) ✅

- `job_type: "service"` field on jobs — agent spawns detached with `Command::spawn()`
- Worker loop stays free; service runs in background
- PID registered with registry via `POST /services`
- stdout/stderr piped to `workspace/service.log`
- Service monitored: on exit, status updated to `crashed`/`done`
- `tinyaws deploy ./app --service --port 3000`
- `tinyaws service list / stop / logs`
- LB polls `/services?status=running` and adds `hostname:port` as targets

### 3.17 Lint CI ✅

- `.github/workflows/lint.yml` — runs on push/PR
- `go vet` on all 11 Go modules
- `golangci-lint` (v2) on registry, scheduler, CLI — `govet`, `errcheck`, `staticcheck`, `ineffassign`
- `cargo clippy -- -D warnings` on ec2-agent, network-agent, common/types
- `cargo check` on object-store (skips C++ build)
- `.golangci.yml` v2 config at repo root

---

## 4. What does NOT work yet (real gaps, not scaffolding)

The primary goal **deploy a web app on Linux and have it serve traffic** is now
unblocked. The remaining gaps are about multi-machine routing, real isolation,
observability, and security.

### Gap A — Multi-machine: agent advertised address 🟡

The LB and registry derive the agent's routable address from `hostname`. On a
multi-machine cluster where the agent runs on a different box, `hostname` may
resolve to `localhost` or an internal-only name. Nothing routes correctly.

**Fix:** `AGENT_ADVERTISE_ADDR` env on the agent — the IP other services should
use to reach it. Store it in the node record; LB and services use it.

---

### Gap B — Service stop doesn't kill the process 🟡

`DELETE /services/{id}` marks the record `stopped` in the registry but does
NOT send SIGTERM to the running PID. The process keeps running on the machine.

**Fix:** the agent needs to poll `/services?node_id=X&status=stopped` and kill
matching PIDs. ~20 lines in `jobs.rs`.

---

### Gap C — Service logs are local only 🟡

`service.log` is written to the agent machine's disk. `tinyaws service logs`
tries to fetch it from the object store, but the agent never uploads it there.
The CLI fallback `cat /tmp/tinyaws/i-1/service.log` only works if you're on
the same machine.

**Fix:** agent periodically tails the log file and PUTs to
`/objects/logs/<svc_id>/service.log` in the object store. CLI fetches from
there.

---

### Gap D — No process isolation between services 🔴

Two services on the same agent share the same filesystem, PID namespace, and
network. A crash in one can see the other's files.

**Fix (lazy):** `unshare --pid --mount --fork` before exec on Linux. ~5 lines.
Real fix: Linux namespaces + cgroups (Tier K).

---

### Gap E — Lambda shell injection risk 🟡

`buildInvokeCommand` in `lambda-runtime/main.go` interpolates `handler` and
`codeURL` directly into a shell string. A handler name with a single quote
breaks out of the string.

**Fix:** pass handler and event as env vars, not shell interpolation.

---

## 5. Remaining work — prioritized

### Tier J: Multi-machine + service kill (do these next)

| # | Commit | What | Lines |
|---|--------|------|-------|
| J1 | `feat(agent): AGENT_ADVERTISE_ADDR for multi-machine routing` | Store advertised IP in node record; LB + services use it | ~30 |
| J2 | `feat(registry): store addr field on node` | Node record gains `addr` field | ~15 |
| J3 | `feat(lb): use node addr field for backend URLs` | Use `addr` not `hostname` | ~5 |
| J4 | `fix(agent): poll stopped services and kill PIDs` | `service stop` actually stops the process | ~25 |
| J5 | `feat(agent): tail service.log to object store` | `service logs` works across machines | ~30 |
| J6 | `docs: multi-machine setup guide` | How to run registry on A, agents on B/C/D | ~1 file |

---

### Tier K: Process isolation (YAGNI until you need multi-tenant)

| # | Commit | What |
|---|--------|------|
| K1 | `feat(agent): unshare --pid --mount --fork for service jobs` | Basic namespace isolation, Linux only, requires root |
| K2 | `feat(agent): cgroup v2 CPU/mem limits` | `instance_type` field drives resource limits |

Skip K2 until K1 is proven useful.

---

### Tier L: Observability (add when someone asks "why is my service slow")

| # | Commit | What |
|---|--------|------|
| L1 | `feat(agent): service logs --follow` | Tail object-store log key; poll every 2s |
| L2 | `feat(agent): report CPU/mem usage on service PATCH` | sysinfo already in agent |

---

### Tier M: Security (add before exposing to the internet)

| # | Commit | What |
|---|--------|------|
| M1 | `fix(lambda): env-var handler, no shell interpolation` | Fixes injection risk |
| M2 | `feat(iam): token expiry` | `expires_at` column on `api_keys` |
| M3 | `feat(all): TLS` | `TINYAWS_TLS_CERT`/`KEY` env vars; skip until public-facing |

---

## 6. Architecture diagram (current state)

```
                      +-----------------+
                      |  tinyaws CLI    |
                      +-------+---------+
                              |
                    +---------v---------+
                    |  API Gateway :8000|
                    |  (reverse proxy)  |
                    +-+-------+-------+-+
                      |       |       |
             +--------+  +----+  +----+--------+
             |           |           |
      +------v-----+ +---v------+ +--v-----------+
      | Registry   | | Scheduler| | Object Store |
      | Go :9000   | | Go :9001 | | Rust :7001   |
      | SQLite     | | SQLite   | | C++ + SQLite |
      +------+-----+ +----+-----+ +------+-------+
             |            |              |
    register |    poll    |              | register
    heartbeat|    jobs    |              | heartbeat
             v            v              |
      +-------------+                   |
      | EC2 Agent   |<------------------+
      | Rust :8080  |
      | run jobs    |  <-- GAP: blocks on long-running processes
      | workspaces  |  <-- GAP: no service/daemon job type
      +-------------+
      
  Also running (optional):
  Controller :9002  — workspace cleanup
  SQS        :9003  — job queue
  SNS        :9004  — event pub/sub
  Networking :9005  — VPC/SG metadata (not real isolation)
  Metadata   :9006  — resource aggregator
  Lambda     :9007  — function invoke
  LB         :8088  — round-robin proxy to agents
```

---

## 7. What works TODAY

```bash
./scripts/run-local.sh   # starts registry, agent, object-store, scheduler

tinyaws job submit "echo hello"                    # ✓ runs, returns stdout
tinyaws object put myfile --file ./data.csv        # ✓ stored on disk
tinyaws deploy ./my-script-app --wait              # ✓ zips, uploads, runs, returns
tinyaws deploy ./my-web-app --service --port 3000  # ✓ spawns detached, LB routes to it
tinyaws service list                               # ✓ shows running services
tinyaws service logs svc-1                         # ✓ reads service.log (local only for now)
tinyaws service stop svc-1                         # ✓ marks stopped in registry (Gap B: doesn't kill PID yet)
```

Multi-machine works for job distribution. LB routing to services on remote
nodes requires Gap A (AGENT_ADVERTISE_ADDR) to be fixed first.

---

## 8. Coding conventions

- **Go control plane:** stdlib `net/http`, Go 1.22+ path patterns (`GET /jobs/{id}`)
- **Rust data plane:** tokio + axum (object-store), reqwest for HTTP client; tokio only (ec2-agent)
- **Storage:** C++17 block engine via C FFI
- **Persistence:** `modernc.org/sqlite` (Go), `rusqlite` (Rust)
- **Comments:** `// comment above function` style, matching existing files
- **Variable names:** match existing codebase — `job_id`, `node_id`, `instance_id`, `deploy_url`
- **No over-engineering:** no interface with one impl, no factory for one product
- **Commits:** one logical change, conventional prefix (`feat`, `fix`, `docs`, `test`, `ci`)
- **No `Co-authored-by: Claude/Cursor`** in commits

---

## 9. Testing requirements

- `go build ./...` in each affected Go module
- `cargo check` in each affected Rust crate
- `cargo build` before testing agent changes (not just check)
- Smoke test: `./tests/integration/smoke-test.ps1` (or `.sh` once Linux script exists)
- CI must stay green

---

## 10. Progress scorecard

| Area | Status |
|------|--------|
| Registry + IAM | ✅ Done |
| Scheduler + jobs | ✅ Done |
| EC2 Agent (Linux) | ✅ Done |
| Object Store + C++ engine | ✅ Done |
| Storage replication | ✅ Done |
| API Gateway | ✅ Done |
| SQS / SNS | ✅ Done |
| Load Balancer | ✅ Done |
| VPC / Networking | ✅ Metadata only (no kernel isolation) |
| Lambda | ✅ Done (injection fixed via env vars) |
| Controller | ✅ Workspace cleanup |
| Service deploy (--service --port) | ✅ Done |
| Service stop → SIGTERM | ✅ Done |
| Service logs to object store | ✅ Done (30s poll) |
| Multi-machine routing | ✅ Done (AGENT_ADVERTISE_ADDR) |
| Lint CI | ✅ Done |
| Unit tests | ✅ 100+ tests across Go and Rust |
| LICENSE | ✅ MIT |
| **Process isolation** | 🔴 Gap D — no namespaces yet (Tier K) |
| **TLS / token expiry** | 🔴 Tier M — add before public-facing |

---

## 11. Architecture assessment

Honest evaluation of the architecture as it stands.

### What's right

**The control-plane / data-plane split is the real deal.** The separation maps
directly to how the code runs — not cargo-culted from AWS whitepapers:

- **Control plane (Go):** API gateway, registry, scheduler, SQS, SNS, VPC, IAM,
  metadata — stateless HTTP services backed by SQLite. Each one is a single
  `main.go` with routes. No frameworks, no DI containers, no generated code.
  ~150-300 lines per service.
- **Data plane (Rust + C++):** The heavy lifting — ec2-agent does real `unshare` +
  `overlayfs` + `pivot_root` + cgroups v2 + seccomp. The object store wraps a
  C++ block engine via FFI. These are the parts that touch the OS.

**Go for coordination, Rust for isolation, C++ for raw storage** — the right
language for each job.

**SQLite everywhere** as the persistence layer is the lazy-correct choice. No
Postgres, no Redis, no etcd. `modernc.org/sqlite` (pure Go) on the control
plane, `rusqlite` on the data plane. One dependency, zero ops.

### What's questionable

**Every service is its own Go module with its own `go.mod`.** That means 9+
separate Go modules in one repo with no shared library. Common types (like
`Job`, `Node`, `Instance`) are redefined in each service. A shared
`internal/types` package would cut duplication without adding complexity.

**The registry is the single point of failure.** Scheduler polls it, agent polls
it, CLI talks through it. It's an in-memory map with SQLite write-through. If
registry dies, the whole platform is blind. Fine for a demo, but the
architecture should acknowledge this explicitly.

**No service discovery, no message bus between services.** Services find each
other via hardcoded env vars (`REGISTRY_URL`, `SCHEDULER_URL`). SNS/SQS exist
as user-facing features but the platform's own services don't use them for
internal communication. Adding a 13th service requires touching env configs
everywhere.

**The agent is doing too much.** `ec2-agent` handles: node registration,
heartbeats, job polling, job execution, container building, instance lifecycle,
networking rules, sandbox isolation, and an HTTP server for direct commands.
That's 14 source files and ~1,500 lines in one binary. It works, but it's the
one place where the "one service, one job" discipline broke down.

### Architecture ceiling

The architecture's ceiling is **"small team running a handful of services on
1-3 machines."** That's exactly what the PRD targets. It doesn't pretend to be
more. The choices are defensible at this scale:

- Monorepo with independent services — fine
- HTTP + JSON everywhere — fine
- SQLite — fine
- Poll-based scheduling — fine for single-digit nodes

---

## 12. Open-source readiness assessment

Honest evaluation of whether this project is ready to be a credible
open-source project.

### What's genuinely good

- **Real substance.** 12 microservices + CLI + storage engine + container sandbox
  in ~8,500 lines is remarkable restraint. The code does what the README claims.
- **Lean and readable.** Consistent style across the entire codebase. No framework
  bloat, no over-abstraction. Every Go service follows the same pattern: struct
  types at top, `main()` sets up routes, handlers below.
- **The PRD is exceptional.** Brutally honest about what works, what doesn't, known
  bugs, and security risks. Rare in open-source.
- **"Own every layer" philosophy held.** No Docker, no K8s dependency, pure stdlib
  where possible.
- **README is accurate.** Every claim verified against the code checks out: CLI
  commands, service ports, `tinyaws.build` format, instance types, sandbox
  description.
- **Cross-platform where it matters.** Windows CI, Linux production, PowerShell +
  bash scripts for both.
- **Proper modern Go.** Go 1.22 path patterns (`"GET /jobs/{id}"`) used throughout.
  Auth middleware applied uniformly.
- **Deliberate simplifications marked.** `ponytail:` comments name the ceiling and
  upgrade path.

### What's holding it back

1. ~~**Empty LICENSE file.**~~ **Fixed.** MIT license added.
2. ~~**Zero unit tests.**~~ **Fixed.** 100+ unit tests across Go (`_test.go` in
   registry, scheduler, networking/vpc) and Rust (`#[test]` in ec2-agent:
   config, node, system, jobs). Covers stores, handlers, migrations, helpers,
   serialization, workspace management.
3. **Committed binaries and databases.** `api.exe`, `scheduler.exe`,
   `registry.exe`, `sqs.exe`, `scheduler.db`, `registry.db` are tracked in git.
   Bloats the repo and creates confusing diffs. (Binaries themselves are clean.)
4. ~~**`.env.local` tracked.**~~ **Already gitignored.**
5. **Single contributor, 31 days old, 200 commits.** It's a sprint, not a
   sustained project. No community signal yet.
6. ~~**Lambda shell injection.**~~ **Fixed.** Handler and event are now passed
   via environment variables, not shell interpolation.
7. **In-memory maps as primary state** in registry and scheduler — works for
   single-node but won't scale. Fine at this scale, but worth documenting the
   ceiling.
8. **Global mutable state everywhere** — `var nodes map[string]Node` with
   `nodesMu sync.RWMutex` at package level. Functional but not great for
   testability.

### Verdict

As a "look what one person built in a month" project, it's legitimately
impressive and well-executed. As an open-source project people should depend on
or contribute to — getting close. Remaining actions:

1. ~~Add a real license (MIT/Apache-2.0)~~ ✅ Done (MIT)
2. Remove committed binaries and databases from git history
3. ~~Gitignore `.env.local`~~ ✅ Already done
4. ~~Write unit tests for the sandbox and storage paths~~ ✅ Done (100+ tests)
5. ~~Fix the lambda injection fully~~ ✅ Done (env vars approach)
