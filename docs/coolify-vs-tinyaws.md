# Coolify vs tiny-aws: two different answers to "I don't want to pay AWS"

Both projects let you deploy apps on your own hardware. Both are open source.
Both have a CLI. That's roughly where the similarity ends, because they're
solving fundamentally different problems, built with completely different
philosophies, and aimed at different people.

This isn't a "which one is better" doc. It's a "what are you actually looking
at" doc.

---

## The one-paragraph version

**Coolify** is a polished PaaS that wraps Docker with a web UI. You push code,
it builds and deploys. It's Heroku/Vercel/Netlify for your own server.
62k GitHub stars, Laravel + PHP backend, Docker Compose under the hood,
beautiful dashboard, 280+ one-click service templates.

**tiny-aws** is a teaching platform that reimplements cloud primitives from
scratch. No Docker. No frameworks. Go + Rust + C++, raw Linux syscalls,
SQLite everywhere. 12 microservices in ~8,500 lines. You learn how the cloud
works by seeing every layer.

Coolify is a product. tiny-aws is an education.

---

## The size difference

This is the most striking thing, so let's put it upfront.

| | Coolify | tiny-aws |
|---|---|---|
| **Production code** | ~200,000+ lines (estimated) | ~8,500 lines |
| **Languages** | PHP, JavaScript, Bash, Blade | Go, Rust, C++ |
| **Files** | 2,000+ | ~64 production files |
| **Dependencies** | composer.json + package.json (hundreds of PHP/JS packages) | 1 Go dep (modernc.org/sqlite), a few Rust crates (tokio, axum, serde) |
| **External services required** | Docker, Docker Compose, Traefik, PostgreSQL, Redis, Soketi, Nginx, S6 Overlay | None. Everything is built from scratch. SQLite files on disk. |
| **GitHub stars** | 62,000+ | New project |
| **Commits** | 17,000+ | ~300 |
| **Contributors** | 500+ | A handful of students |
| **Age** | 4+ years (started ~2022) | ~1 month |
| **Tests** | PHPUnit + Dusk (browser tests) | 323 unit tests + 17 integration scripts |
| **License** | Apache-2.0 | MIT |

Coolify is a mature product with a company behind it (coolLabs Solutions Kft),
paid cloud hosting, sponsors, and a 20,000+ member Discord. tiny-aws is a
student project that happens to work end-to-end.

They shouldn't be compared as products. They should be compared as
architectures.

---

## Architecture: how they're built

### Coolify

Coolify is a **Laravel monolith** with a Livewire + Alpine.js frontend.

| Layer | Technology | What it does |
|-------|------------|-------------|
| Web UI | Livewire, Alpine.js, Blade, Tailwind CSS | Real-time dashboard, terminal, team management |
| Backend | Laravel 11 (PHP 8.4) | Business logic, SSH orchestration, API |
| Database | PostgreSQL 15 | All state: servers, apps, deployments, teams |
| Cache / real-time | Redis 7 + Soketi (WebSocket server) | Session cache, queues, live terminal, notifications |
| Process supervisor | S6 Overlay | Keeps PHP, Nginx, Soketi alive inside the Coolify container |
| Web server | Nginx | Serves the PHP app, handles HTTP |
| Container runtime | Docker + Docker Compose | All app isolation, building, networking |
| Reverse proxy | Traefik | Dynamic routing, SSL termination, Let's Encrypt |
| CI/CD | GitHub Actions | Coolify's own build pipeline |

When you deploy an app on Coolify, here's what actually happens:

1. You push to GitHub (or manually trigger in the UI)
2. Coolify's PHP backend receives the webhook
3. It SSHes into your target server
4. It clones your repo on the server
5. It generates a `Dockerfile` (or uses yours, or uses Nixpacks to auto-detect)
6. It runs `docker build` to create an image
7. It runs `docker compose up` to start the container
8. It configures Traefik labels so your domain routes to the container
9. Traefik automatically provisions Let's Encrypt SSL certificates
10. The UI shows deployment status in real-time via WebSocket

Docker does all the heavy lifting for isolation. Traefik does all the networking.
PostgreSQL stores all the state. Coolify is the orchestration and UX layer that
ties it all together.

This is a completely reasonable architecture. Docker is battle-tested. Traefik
is battle-tested. Laravel is battle-tested. Coolify stands on giants and adds
a beautiful management layer.

**The dependency count reflects this:** Coolify's `composer.json` lists dozens
of PHP packages (Laravel framework, Livewire, SSH libraries, YAML parsers,
HTTP clients...). Its `package.json` has Tailwind, Vite, Alpine.js, and more.
It works because each dependency is well-tested and maintained.

### tiny-aws

tiny-aws is **12 independent microservices** with no shared framework.

| Layer | Technology | What it does |
|-------|------------|-------------|
| Control plane | Go stdlib `net/http` | Registry, scheduler, SQS, SNS, VPC, controller, metadata, API gateway (8 services) |
| Compute agent | Rust + tokio | Job execution, container management, heartbeats |
| Object store | Rust + axum + C++ FFI | File storage with block engine |
| Network agent | Rust + tokio | iptables/netsh rule enforcement |
| Shared types | Rust (serde) | Common structs for agent communication |
| CLI | Go stdlib | Command-line interface for all operations |
| Persistence | SQLite (modernc.org/sqlite for Go, rusqlite for Rust) | Every service that needs state |

When you deploy an app on tiny-aws, here's what actually happens:

1. You run `tinyaws deploy ./my-app --service --port 3000`
2. The CLI zips your directory
3. It uploads the zip to the object store (PUT /buckets/deployments/objects/...)
4. It submits a job to the scheduler with the object URL and `job_type: "service"`
5. The scheduler queries the registry for healthy compute nodes
6. It picks one (round-robin) and creates a "pending" job record in SQLite
7. The agent on that node polls GET /jobs?node_id=X&status=pending (every 3s)
8. It picks up the job, downloads the zip, extracts to a workspace directory
9. It spawns the start script as a detached process with `Command::spawn()`
10. It registers the service with the registry (POST /services with port + PID)
11. It starts uploading service.log to the object store every 30 seconds
12. The load balancer discovers the new service (polls registry every 10s)
13. Traffic starts routing to your app

No Docker anywhere. No framework. The isolation is done with raw Linux kernel
primitives (if sandbox mode is on). The storage is a custom C++ engine. The
scheduling is a custom Go service with SQLite. Everything is built from scratch.

**The dependency count reflects this:** Go services have exactly one external
dependency — `modernc.org/sqlite` (a pure-Go SQLite driver). The CLI has zero
external dependencies. Rust crates use tokio, axum, reqwest, serde — standard
async Rust, no frameworks.

---

## What they share

Despite being very different projects, they do share some DNA:

**Self-hosted.** Both run on hardware you control. No vendor lock-in. If you
stop using either one, your stuff keeps running (on Docker containers or
nspawn containers respectively).

**Server management.** Coolify SSHes into target servers to run Docker
commands. tiny-aws agents register with a central registry over HTTP and poll
for work. Different mechanism, same idea: a control plane talking to worker
nodes.

**Push to deploy.** Both support "push code, it runs." Coolify does it via git
webhooks and Docker builds. tiny-aws does it via `tinyaws deploy ./dir` which
zips, uploads, and schedules.

**Service discovery.** Coolify uses Docker labels and Traefik to route traffic.
tiny-aws uses registry polling and a custom load balancer. Both achieve the
same result: your app gets a routable address.

**Health checks.** Coolify monitors Docker container health. tiny-aws agents
heartbeat to the registry and the LB health-checks agents and services.

**Open source.** Both accept contributions, both have public repos.

**Single-developer origins.** Coolify was started by Andras Bacsai. tiny-aws was
started as a student project at IIIT Sonepat. Both grew from one person's
vision, though Coolify has since grown into a company.

---

## Where they diverge (the real comparison)

### Docker vs no Docker

This is the fundamental architectural difference and it drives everything else.

**Coolify** treats Docker as infrastructure. It doesn't build containers from
scratch — it uses Docker the way Docker was designed to be used. `docker build`,
`docker compose up`, `docker network create`. Coolify is Docker's best friend.

**tiny-aws** deliberately has no Docker dependency. The PRD literally says: "No
Kubernetes, no Docker, no JVM." When tiny-aws isolates a process, it calls
`unshare(2)` directly. When it builds a filesystem, it uses `overlayfs`
directly. When it limits CPU, it writes to `/sys/fs/cgroup` directly.

Why? Because Docker is a ~20,000 line daemon that does these same things but
hides them behind an API. If you want to *use* containers, Docker is great. If
you want to *understand* containers, you need to see the syscalls.

This is not a criticism of either approach. They're serving different purposes.

### Scope: platform vs infrastructure

**Coolify** is a complete platform. It answers "how do I get my Next.js app
running on my server?" with a turnkey solution:

- Git integration (GitHub, GitLab, Bitbucket, Gitea)
- Automatic builds (Nixpacks auto-detection, Dockerfiles, docker-compose)
- SSL certificates (Let's Encrypt, automatic renewal)
- Custom domains (Traefik dynamic routing)
- Databases (PostgreSQL, MySQL, MongoDB, Redis, ClickHouse — one click)
- Backups (S3-compatible, automatic scheduling)
- Team management (roles, permissions, shared projects)
- Monitoring (deployment status, disk usage, container health)
- Notifications (Discord, Telegram, email)
- Real-time terminal (in-browser SSH via WebSocket)
- API (full REST API for automation)
- 280+ one-click service templates (WordPress, Grafana, n8n, Plausible...)

**tiny-aws** is raw infrastructure. It answers "how does a cloud work
underneath?" with a collection of primitives:

- Compute (instances with isolation)
- Storage (object store with replication)
- Scheduling (job submission and dispatch)
- Queuing (SQS-style message queue)
- Pub/sub (SNS-style event fan-out)
- Networking (VPC metadata, security group rules)
- Auth (API keys with expiry)
- Lambda (function-as-a-service)
- Load balancing (round-robin to healthy targets)
- CLI (20+ commands mapping to API calls)

Coolify gives you WordPress in one click. tiny-aws gives you
`tinyaws job submit "echo hello"`. Different worlds entirely.

### Web UI vs CLI

Coolify has a gorgeous web dashboard. You can see all your servers, apps,
databases, and services in a visual layout. There's a real-time terminal in the
browser. You can drag and drop, get notifications, collaborate with your team.
It's a product designed for daily use by developers who want to ship, not tinker.

tiny-aws has a command-line tool and JSON responses. It looks like the AWS CLI
because it's modeled on the AWS CLI. There is no web interface. If you want a
pretty dashboard, you build one yourself (and that would actually be a great
learning exercise).

### Database architecture

Coolify uses **PostgreSQL** as a proper relational database, with Laravel's
Eloquent ORM, migrations, and a well-designed schema for servers, applications,
deployments, teams, and settings. Redis handles caching and real-time features.

tiny-aws uses **SQLite files** — one per service. The registry has `registry.db`,
the scheduler has `scheduler.db`, SQS has `sqs.db`, etc. No ORM, no migrations
framework — just raw `CREATE TABLE IF NOT EXISTS` with `ALTER TABLE ADD COLUMN`
for backwards compatibility. This is deliberately simple (and a conscious
tradeoff: no multi-writer concurrency, no query optimization, but also zero
ops burden).

### How they handle builds

Coolify builds your app into a Docker image:
1. Clones your repo on the server
2. Uses Nixpacks (auto-detect language + framework), your Dockerfile, or your
   docker-compose.yml
3. Runs `docker build` to create a layered image
4. Runs `docker compose up` with health checks and rollback

tiny-aws doesn't build anything:
1. Zips your source directory as-is
2. Uploads the zip to the object store
3. Agent downloads and extracts
4. Runs your `start.sh` (or `start.ps1` on Windows)

If your app needs dependencies installed (`npm install`, `pip install`), you
either include them in the zip or put the install commands in your start script.
Coolify handles this through Docker build layers. tiny-aws makes you handle it
yourself.

### How they handle SSL and domains

**Coolify:** Traefik handles everything. You set a domain in the UI, Coolify
configures Traefik labels on the Docker container, Traefik automatically
provisions Let's Encrypt certificates, and traffic routes to your app over
HTTPS. The whole thing is automatic.

**tiny-aws:** No SSL. No domain management. The load balancer does plain HTTP
round-robin. If you want HTTPS, you put a reverse proxy (nginx, caddy) in
front. The PRD lists TLS as a Tier M item — "add before exposing to the
internet."

### How they handle databases

**Coolify:** First-class database support. One-click PostgreSQL, MySQL,
MariaDB, MongoDB, Redis, ClickHouse, DragonFly deployments. Automatic backups
to S3-compatible storage. UI to manage connection strings. This is one of
Coolify's strongest features.

**tiny-aws:** No database service. You could run PostgreSQL inside a tiny-aws
instance, but there's no managed offering, no automatic backups, no UI. The
object store can store data, but it's not a database.

### How they handle multi-server

**Coolify:** You add servers by providing SSH credentials. Coolify SSHes in,
installs Docker if needed, and then manages everything over SSH. You can have
dozens of servers, each running different apps.

**tiny-aws:** You start an agent on each machine. The agent registers with the
central registry over HTTP (using `AGENT_ADVERTISE_ADDR` for its routable IP).
The scheduler distributes jobs across healthy agents. No SSH involved — it's
agent-based, not SSH-based.

| | Coolify | tiny-aws |
|---|---|---|
| Discovery | SSH connection you configure | Agent self-registration |
| Communication | SSH commands | HTTP polling (agents poll scheduler) |
| Requirements | Docker on target server | Rust agent binary on target server |
| Auth | SSH keys | API key (Bearer token) |
| Agent install | Automatic via install script | Manual (cargo build or download binary) |

---

## Infrastructure requirements

| | Coolify | tiny-aws |
|---|---|---|
| **Minimum server** | 2 vCPU, 2 GB RAM, 30 GB disk (recommended) | Any Linux box with Go + Rust installed |
| **OS** | Ubuntu, Debian, Fedora, CentOS, Arch, SUSE, Raspberry Pi | Linux (for full features), Windows (partial, no isolation) |
| **Runtime dependencies** | Docker, Docker Compose (installed automatically) | None (all binaries are self-contained) |
| **Database** | PostgreSQL 15 (included in Docker install) | SQLite (file on disk, no setup) |
| **Ports** | 8000 (UI), 80/443 (Traefik), 6001 (WebSocket), 5432 (Postgres) | 9000-9007 (services), 7001 (object store), 8080 (agent), 8088 (LB), 8000 (gateway) |
| **Docker required** | Yes (the entire architecture depends on it) | No |
| **Root access** | Yes (Docker requires root or docker group) | Yes (for cgroup/namespace isolation), no (for basic job execution) |
| **RAM overhead** | ~500 MB+ (Postgres, Redis, Soketi, PHP, Nginx) | ~50 MB (all services combined, SQLite is nearly free) |

---

## The dependency philosophy

This might be the most interesting architectural difference.

**Coolify depends on everything battle-tested.** Docker for containers. Traefik
for routing. PostgreSQL for state. Redis for caching. Laravel for the web
framework. Livewire for reactivity. This is the "stand on the shoulders of
giants" approach, and it works incredibly well for a product.

**tiny-aws depends on almost nothing.** Go services use stdlib `net/http` — no
gin, no echo, no chi. Persistence is SQLite — no Postgres, no Redis, no etcd.
The CLI has zero external dependencies. The entire control plane's only external
dependency is a single SQLite driver. This is the "own every layer" approach,
and it works incredibly well for education.

The tradeoff is real:
- Coolify inherits Docker's reliability, Traefik's features, and Postgres's
  ACID guarantees — but also their complexity, their memory footprint, and their
  upgrade requirements.
- tiny-aws owns every line of code and every architectural decision — but also
  every bug, every missing feature, and every security gap.

---

## Technical comparison table

| Dimension | Coolify | tiny-aws |
|-----------|---------|----------|
| **Goal** | Production PaaS | Educational infrastructure |
| **Architecture** | Laravel monolith + Docker | 12 independent microservices |
| **Production code** | ~200,000+ lines | ~8,500 lines |
| **Languages** | PHP, JavaScript, Bash | Go, Rust, C++ |
| **Container runtime** | Docker | systemd-nspawn + raw namespaces |
| **Storage** | Docker volumes, S3 backups | Custom C++ block engine |
| **Database** | PostgreSQL 15 | SQLite (one file per service) |
| **Cache** | Redis 7 | None |
| **Frontend** | Livewire + Alpine.js + Tailwind | None (CLI only) |
| **Reverse proxy** | Traefik (automatic SSL, Let's Encrypt) | Custom Go round-robin LB (no SSL) |
| **Git integration** | GitHub, GitLab, Bitbucket, Gitea | None (zip upload) |
| **SSL** | Let's Encrypt (automatic) | None |
| **Domains** | Custom domain routing via Traefik | No domain management |
| **Service templates** | 280+ one-click | Build your own |
| **Database hosting** | PostgreSQL, MySQL, MongoDB, Redis, ClickHouse (one-click) | None |
| **Backups** | S3-compatible (automatic) | None |
| **External dependencies** | Docker, Traefik, PostgreSQL, Redis, Soketi, Nginx | SQLite (that's it) |
| **RAM overhead** | ~500 MB+ | ~50 MB |
| **Monitoring** | Built-in (disk, deployments, health) | Agent heartbeats only |
| **Notifications** | Discord, Telegram, email | None |
| **Team management** | Roles, permissions, collaboration | admin/readonly API keys |
| **Real-time UI** | WebSocket terminal, live deployment logs | None |
| **Queuing** | Redis-backed (Laravel queues) | Custom SQS service (SQLite) |
| **Pub/sub** | None built-in | Custom SNS service (HTTP fan-out) |
| **VPC/Networking** | Docker networks | Custom VPC metadata + iptables |
| **Serverless / FaaS** | No | Yes (Lambda runtime) |
| **Multi-server** | Yes (SSH to remote servers) | Yes (agent registration + HTTP polling) |
| **Build system** | Nixpacks / Dockerfile / docker-compose | Zip and upload |
| **API** | Full REST API with OpenAPI spec | REST API (per-service) |
| **Install command** | `curl -fsSL https://cdn.coollabs.io/coolify/install.sh \| bash` | `git clone` + `go build` + `cargo build` |
| **Time to first deploy** | ~5 minutes (install + UI setup) | ~20 minutes (build everything + start services) |
| **GitHub stars** | 62,000+ | New project |
| **Contributors** | 500+ | A handful of students |
| **License** | Apache-2.0 | MIT |

---

## When you'd use which

### Use Coolify when:

- You need to deploy a real app to production, today
- You want a Heroku/Vercel alternative you control
- You want one-click databases (Postgres, Redis, MongoDB...)
- You want SSL, domains, and git push deploys out of the box
- You want a web dashboard, not a terminal
- You want 280+ service templates
- You want team collaboration and permissions
- You're comfortable with Docker and just want it managed better
- You want automatic backups and monitoring
- You want notifications when deployments fail

### Use tiny-aws when:

- You're learning how cloud infrastructure works
- You want to understand what Docker does underneath (namespaces, cgroups, overlayfs)
- You want to see how EC2, S3, SQS, SNS, Lambda, ELB, VPC actually work
- You're in a systems programming or cloud computing course
- You want to understand scheduling, replication, and service discovery
- You want to read an entire cloud platform's source code in a weekend
- You care about Linux internals (unshare, pivot_root, seccomp, cgroups v2)
- You want to understand multi-language system design (Go + Rust + C++)
- You want to build something on top of raw infrastructure

### Use both when:

Honestly? Use Coolify to deploy your apps and tiny-aws to understand what's
happening underneath. They complement each other. Coolify is the car you
drive to work. tiny-aws is the engine you take apart in your garage to
understand how cars work.

---

## What each teaches you

If you study **Coolify's architecture**, you learn:
- How to build a production PaaS with Laravel
- How to orchestrate Docker over SSH at scale
- How Traefik handles dynamic routing, SSL termination, and Let's Encrypt
- How to build real-time UIs with Livewire, Alpine.js, and WebSockets
- How a modern PHP monolith is structured (routes, controllers, models, jobs, events)
- How PostgreSQL + Redis work together for state + cache
- The value of standing on existing infrastructure (Docker, Traefik, Postgres)
- How to build a product that thousands of people actually use

If you study **tiny-aws's architecture**, you learn:
- How cloud services are actually built from scratch
- Linux container primitives (what Docker does under the hood)
- Multi-language system design (Go for HTTP, Rust for systems, C++ for storage)
- Distributed systems concepts (replication, scheduling, health checks, retries)
- How S3, SQS, SNS, EC2, Lambda, ELB, VPC work at a fundamental level
- How SQLite works as a universal persistence layer
- The cost of building everything yourself (and when that cost is worth it)
- Why projects like Coolify choose Docker instead of building isolation from scratch

---

## The philosophical difference

Coolify's philosophy is: **Docker solved containers. We'll solve everything
around it** — the deployment pipeline, the SSL, the monitoring, the UI, the
team management. Docker is a reliable black box, and Coolify is the great
experience on top.

tiny-aws's philosophy is: **Own every layer.** No Docker, no Kubernetes, no
frameworks. Build the container runtime. Build the storage engine. Build the
scheduler. Build the queue. The point is not to build something better than
Docker — it's to understand what Docker does by building it yourself.

Coolify asks "how do we make self-hosting as easy as Vercel?"

tiny-aws asks "how does the thing underneath Vercel actually work?"

Both are valid questions. They just lead to very different projects.

---

## Final thought

There's a classic tradeoff in engineering: do you use the tool, or do you build
the tool?

Coolify says: use the tools (Docker, Traefik, Postgres), and we'll build a
great experience on top. This is the right call for a product. Nobody wants to
implement TLS from scratch when Let's Encrypt exists.

tiny-aws says: build the tools, so you understand what the tools do. This is
the right call for education. Nobody learns how containers work by typing
`docker run`.

The best engineers do both — they use Coolify on Monday to ship their app, and
read tiny-aws's source code on Saturday to understand what's happening
underneath. That's not a contradiction. That's growth.
