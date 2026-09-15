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

## Architecture: how they're built

### Coolify

Coolify is a **Laravel monolith** with a Livewire + Alpine.js frontend.

| Layer | Technology |
|-------|------------|
| UI | Livewire, Alpine.js, Blade templates, Tailwind CSS |
| Backend | Laravel 11 (PHP 8.4) |
| Database | PostgreSQL 15 |
| Cache / real-time | Redis 7 + Soketi (WebSocket server) |
| Process supervisor | S6 Overlay |
| Web server | Nginx |
| Container runtime | Docker + Docker Compose |
| CI/CD | GitHub Actions |

When you deploy an app on Coolify, here's what actually happens:

1. Coolify's PHP backend clones your git repo (or receives a webhook)
2. It generates a `Dockerfile` or `docker-compose.yml` (or uses yours)
3. It SSHes into your target server
4. It runs `docker build` and `docker compose up`
5. It configures Traefik (the built-in reverse proxy) to route your domain
6. It provisions Let's Encrypt SSL certs automatically

Docker does all the heavy lifting. Coolify is the orchestration layer on top.
The isolation, the networking, the image building — that's all Docker.

This is a completely reasonable architecture. Docker is battle-tested. Traefik
is battle-tested. Laravel is battle-tested. Coolify stands on giants and adds
a beautiful management layer.

### tiny-aws

tiny-aws is **12 independent microservices** with no shared framework.

| Layer | Technology |
|-------|------------|
| Control plane | Go stdlib `net/http` (registry, scheduler, SQS, SNS, VPC, controller, metadata, API gateway) |
| Data plane | Rust with tokio (EC2 agent, network agent), Rust + axum (object store) |
| Storage engine | C++17 block engine via FFI |
| Persistence | SQLite everywhere (modernc.org/sqlite for Go, rusqlite for Rust) |
| Container runtime | `systemd-nspawn`, `unshare`, `overlayfs`, `pivot_root`, `cgroups v2`, `seccomp` |
| UI | None. CLI only. |
| Reverse proxy | Custom Go load balancer (round-robin) |

When you deploy an app on tiny-aws, here's what actually happens:

1. The CLI zips your directory and uploads it to the object store
2. It submits a job to the scheduler with the object URL
3. The scheduler picks a healthy compute node (via the registry)
4. The agent on that node polls for the job, downloads the zip, extracts it
5. If it's a service: the agent spawns a detached process, registers it with
   the registry, and the load balancer discovers it
6. If it's a regular job: the agent runs the command, captures stdout/stderr,
   reports completion

No Docker anywhere. The isolation is done with raw Linux kernel primitives.
The storage is a custom C++ engine. The scheduling is a custom Go service.
Everything is built from scratch.

---

## What they share

Despite being very different projects, they do share some DNA:

**Self-hosted.** Both run on hardware you control. No vendor lock-in. If you
stop using either one, your stuff keeps running (on Docker containers or
nspawn containers respectively).

**SSH-based server management.** Coolify SSHes into target servers to run Docker
commands. tiny-aws agents register with a central registry over HTTP — similar
concept, different mechanism.

**Push to deploy.** Both support "push code, it runs." Coolify does it via git
webhooks and Docker builds. tiny-aws does it via `tinyaws deploy ./dir` which
zips and uploads.

**Open source.** Coolify is Apache-2.0. tiny-aws is MIT. Both accept
contributions.

**Single-developer origins.** Coolify was started by Andras Bacsai. tiny-aws was
started as a student project at IIIT Sonepat. Both grew from one person's
vision.

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

**Coolify** is a platform. It answers "how do I get my Next.js app running on
my server?" with a complete solution: git integration, build pipeline, SSL,
domains, databases, monitoring, backups, team management, webhooks, API.

**tiny-aws** is infrastructure. It answers "how does a cloud work underneath?"
with a collection of primitives: compute, storage, networking, queuing, events.
You assemble them yourself.

Coolify gives you 280+ one-click service templates (WordPress, Grafana,
PostgreSQL, Redis...). tiny-aws gives you `tinyaws job submit "echo hello"`.
Different worlds.

### Web UI vs CLI

Coolify has a gorgeous web dashboard. Real-time terminal in the browser. Drag
and drop. Notifications. Team collaboration. It's a product designed for
daily use.

tiny-aws has a command-line tool and JSON responses. It looks like the AWS CLI
because it's modeled on the AWS CLI. If you want a pretty interface, you build
one.

### Language and complexity

Coolify is a Laravel monolith — one language (PHP), one framework, one
database (Postgres). The frontend uses Livewire for reactivity. This is a
well-understood stack that many PHP developers can contribute to.

tiny-aws is three languages (Go, Rust, C++) with zero shared frameworks. Each
service is its own Go module or Rust crate. This makes it harder to contribute
to but means each component uses the right tool: Go for HTTP coordination,
Rust for system-level work, C++ for raw I/O.

### Community and maturity

Coolify has 62k GitHub stars, 17k commits, 500+ contributors, paid cloud
hosting, sponsors, a Discord with 20k+ members. It's a real product used by
real companies.

tiny-aws has ~300 commits, is a month old, and was built by students. It's a
teaching project that happens to work end-to-end.

---

## When you'd use which

### Use Coolify when:

- You need to deploy a real app to production, today
- You want a Heroku/Vercel alternative you control
- You want one-click databases (Postgres, Redis, MongoDB...)
- You want SSL, domains, and git push deploys out of the box
- You want a web dashboard, not a terminal
- You want 280+ service templates
- You're comfortable with Docker and just want it managed better

### Use tiny-aws when:

- You're learning how cloud infrastructure works
- You want to understand what Docker does underneath
- You want to see how EC2, S3, SQS, Lambda actually work
- You're in a systems programming or cloud computing course
- You want to understand scheduling, replication, and service discovery
- You want to read an entire cloud platform's source code in a weekend
- You care about Linux internals (namespaces, cgroups, overlayfs, seccomp)

### Use both when:

Honestly? Use Coolify to deploy your apps and tiny-aws to understand what's
happening underneath. They complement each other. Coolify is the car you
drive to work. tiny-aws is the engine you take apart in your garage to
understand how cars work.

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

## Technical comparison table

| Dimension | Coolify | tiny-aws |
|-----------|---------|----------|
| **Goal** | Production PaaS | Educational infrastructure |
| **Container runtime** | Docker | systemd-nspawn + raw namespaces |
| **Storage** | Docker volumes, S3 backups | Custom C++ block engine |
| **Database** | PostgreSQL | SQLite (in every service) |
| **Language** | PHP (Laravel) | Go + Rust + C++ |
| **Frontend** | Livewire + Alpine.js + Tailwind | None (CLI only) |
| **Reverse proxy** | Traefik (automatic SSL) | Custom Go round-robin LB |
| **Git integration** | GitHub, GitLab, Bitbucket, Gitea | None (zip upload) |
| **SSL** | Let's Encrypt (automatic) | None |
| **Service templates** | 280+ one-click | Build your own |
| **Lines of code** | ~200k+ (estimated) | ~8,500 (production) |
| **GitHub stars** | 62k | New project |
| **License** | Apache-2.0 | MIT |
| **Monitoring** | Built-in (disk, deployments) | None |
| **Team management** | Roles, permissions | admin/readonly keys |
| **Queuing** | Redis-backed (Laravel queues) | Custom SQS service (SQLite) |
| **Pub/sub** | None built-in | Custom SNS service (HTTP fan-out) |
| **VPC/Networking** | Docker networks | Custom VPC metadata + iptables |
| **Serverless / FaaS** | No | Yes (Lambda runtime) |
| **Multi-server** | Yes (SSH to remote servers) | Yes (agent registration) |

---

## What each teaches you

If you study **Coolify's architecture**, you learn:
- How to build a production PaaS with Laravel
- How to orchestrate Docker over SSH
- How Traefik handles dynamic routing and SSL
- How to build real-time UIs with WebSockets
- How a modern PHP application is structured
- The value of standing on existing infrastructure (Docker, Traefik, Postgres)

If you study **tiny-aws's architecture**, you learn:
- How cloud services are actually built from scratch
- Linux container primitives (what Docker does under the hood)
- Multi-language system design (Go for HTTP, Rust for systems, C++ for storage)
- Distributed systems concepts (replication, scheduling, health checks, retries)
- How S3, SQS, SNS, EC2, Lambda, ELB, VPC work at a fundamental level
- The cost of building everything yourself (and when that cost is worth it)

---

## Final thought

There's a classic tradeoff in engineering: do you use the tool, or do you build
the tool?

Coolify says: use the tools (Docker, Traefik, Postgres), and we'll build a
great experience on top.

tiny-aws says: build the tools, so you understand what the tools do.

Neither answer is wrong. The best engineers do both — they use Coolify on
Monday to ship their app, and read tiny-aws's source code on Saturday to
understand what's happening underneath.
