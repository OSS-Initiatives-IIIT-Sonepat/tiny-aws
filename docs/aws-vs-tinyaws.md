# AWS vs tiny-aws: what's the same, what's not, and why that matters

If you've used AWS (or heard people complain about their AWS bill), you already
know what it does. You click some buttons, your app runs somewhere in Virginia,
and you pay for it. Underneath all that UI and billing and regions and
availability zones, there's actual infrastructure doing actual things.

tiny-aws is what happens when you strip all that away and rebuild the core ideas
on a single Linux machine. Same concepts, drastically simpler implementation.
This doc walks through each service and honestly compares them.

---

## The scale difference (let's get this out of the way)

| | AWS | tiny-aws |
|---|---|---|
| **Engineers** | ~60,000+ (Amazon's cloud division) | A few students at IIIT Sonepat |
| **Codebase size** | Hundreds of millions of lines (estimated) | ~8,500 lines of production code |
| **Services** | 200+ (EC2, S3, RDS, DynamoDB, Kinesis, Redshift...) | 12 (registry, scheduler, agent, object store, SQS, SNS, VPC, LB, Lambda, controller, metadata, API gateway) |
| **Data centers** | 33 regions, 105 availability zones, millions of servers | Your one Linux box. Maybe three if you're feeling ambitious. |
| **Revenue** | ~$100 billion/year | $0. It's free. MIT license. |
| **Languages** | Java, C++, Rust, Go, Python, internal tools... | Go (~5,000 LOC), Rust (~3,300 LOC), C++ (~220 LOC) |
| **Uptime SLA** | 99.99% (contractual, with financial penalties) | "Hopefully it doesn't crash" |
| **Cost to run** | $5/month for a t3.micro, scaling to millions | $0 (your electricity bill) |
| **Test count** | Unknown (probably millions) | 323 unit tests + 17 integration scripts |
| **First release** | 2006 (S3), 2006 (EC2) | 2026 (a month ago) |
| **Customers** | Millions of businesses, governments, startups | Students learning how clouds work |

It's not a fair comparison. It's not meant to be. tiny-aws exists so you can
understand what those 60,000 engineers are building.

---

## The 30-second service map

| What you want | AWS | tiny-aws | Lines in tiny-aws |
|---|---|---|---|
| Run a server | EC2 (Xen/Nitro VMs) | `tinyaws instance launch` (nspawn containers) | ~1,500 (Rust agent) |
| Store files | S3 (distributed object store) | Object store (C++ engine + Rust HTTP) | ~1,200 (Rust + C++) |
| Schedule work | ECS / internal placement | Scheduler | ~600 (Go) |
| Service registry | Internal (not public) | Registry | ~500 (Go) |
| Run a function | Lambda (Firecracker microVMs) | Lambda runtime | ~236 (Go) |
| Queue messages | SQS (distributed queue) | SQS (SQLite queue) | ~187 (Go) |
| Pub/sub events | SNS (fan-out) | SNS (HTTP fan-out) | ~175 (Go) |
| Load balance | ELB/ALB/NLB | Load balancer (round-robin proxy) | ~180 (Go) |
| Private network | VPC (SDN + custom hardware) | VPC (metadata only, SQLite) | ~280 (Go) |
| Firewall rules | Security Groups (Nitro-enforced) | Security groups (iptables/netsh) | ~60 (Go + Rust agent) |
| Auth | IAM (policies, roles, SAML, OIDC...) | IAM (api_keys table, 2 roles) | ~80 (Go) |
| Workspace cleanup | Internal lifecycle mgmt | Controller | ~80 (Go) |
| Resource aggregation | CloudWatch / Resource Explorer | Metadata service | ~60 (Go) |
| Unified API | API Gateway (REST/HTTP/WebSocket) | API gateway (reverse proxy) | ~60 (Go) |
| CLI | `aws` (Python, ~2M lines with SDKs) | `tinyaws` (Go, ~800 lines) | ~800 (Go) |
| Network enforcement | VPC flow logs, NACLs | Network agent (iptables writer) | ~100 (Rust) |

AWS has ~200 services. tiny-aws has 12. But those 12 cover the foundational
layer that everything else in AWS is built on top of.

---

## Compute: EC2 vs instances

### How AWS does it

EC2 runs actual virtual machines. Under the hood, AWS built custom hardware
(Nitro cards) that offloads networking and storage to dedicated chips, so the
hypervisor doesn't eat your CPU. Each instance gets its own kernel, its own
memory space, its own virtual network interface. You pick an AMI (a disk image),
choose an instance type (how much CPU and RAM), and AWS boots a real VM on a
physical server in a data center.

The isolation is hardware-enforced. One EC2 instance literally cannot see another
instance's memory, even if they're on the same physical machine. That's the
whole point.

EC2 has hundreds of instance types: general purpose (m5, m6i, m7g), compute
optimized (c5, c6g), memory optimized (r5, x2idn), storage optimized (i3, d3),
GPU (p4d, g5), and more. Prices range from $0.0042/hr (t4g.nano) to $32.77/hr
(p4d.24xlarge).

### How tiny-aws does it

tiny-aws uses `systemd-nspawn` — Linux's built-in container runtime. When you
run `tinyaws instance launch`, the agent:

1. Calls `unshare(CLONE_NEWPID | CLONE_NEWNS)` to create new PID and mount namespaces
2. Sets up an overlayfs (writable layer on top of a shared base image)
3. Does `pivot_root` to make the container think its filesystem is the only one
4. Writes cgroup v2 limits to `/sys/fs/cgroup` for CPU and memory
5. Loads a seccomp profile (JSON) to restrict dangerous syscalls
6. Optionally creates a veth pair for network isolation

It's not a VM. There's no separate kernel. But the isolation is real enough that
you can run `apt install` inside an instance without affecting the host.

### The numbers

| | AWS EC2 | tiny-aws instances |
|---|---|---|
| Isolation | Hardware VM (Nitro hypervisor) | OS containers (namespaces + cgroups) |
| Instance types | 500+ (t3.micro to p4d.24xlarge) | 5 (nano, micro, small, medium, large) |
| Boot time | 30-90 seconds | <1 second |
| Max memory | 24 TB (u-24tb1.metal) | Whatever your machine has |
| Max vCPUs | 448 (u7in-32tb.224xlarge) | Whatever your machine has |
| Regions | 33 | 1 (your house) |
| Base images | 100,000+ AMIs | Debian rootfs via debootstrap |
| Live migration | Yes (transparent) | No |
| Persistent storage | EBS volumes (replicated SSDs) | Host filesystem via overlayfs |
| GPU support | Yes (NVIDIA, AMD, custom Inferentia) | No |
| Nested virtualization | Yes | No (containers only) |
| Cost | $0.004 - $32.77/hr | Free |
| Code to implement | Millions of lines + custom hardware | ~1,500 lines of Rust |

### What's the same (conceptually)

The lifecycle is identical: launch -> provisioning -> running -> terminated.
Instance IDs follow the same `i-N` convention. You can list, terminate, get
info. The scheduler assigns instances to nodes just like EC2 places VMs on
hosts. The agent heartbeats to the registry just like EC2 instances report to
the control plane.

If you understand tiny-aws instances, you understand the *shape* of EC2.

---

## Storage: S3 vs object store

### How AWS does it

S3 is one of the most impressive distributed systems ever built. Your object
gets split across multiple disks, in multiple facilities, in multiple
availability zones. S3 replicates everything at least three times. It does
consistent reads (since 2020). It handles trillions of objects. The internal
architecture uses a custom request router, a placement service, and a storage
engine that writes to physical spinning disks and SSDs.

You never see any of this. You just PUT an object and GET it back.

S3 has versioning, lifecycle policies (auto-delete after N days, transition to
cheaper storage), event notifications (trigger Lambda on upload), S3 Select
(query CSV/JSON in place), transfer acceleration, cross-region replication,
storage classes (Standard, Infrequent Access, Glacier, Deep Archive), and more.

### How tiny-aws does it

tiny-aws has a C++ block engine (`block_store.cpp`, ~150 lines) that writes
files to disk via raw `fstream`. A Rust HTTP server sits on top with axum,
handling PUT/GET/DELETE/list. SQLite stores metadata (size, etag, content-type).
Buckets are just path prefixes.

Replication exists: when you set `REPLICATION_FACTOR=2`, writes go to multiple
storage nodes (discovered from the registry). Reads fall back to peers if the
local copy is missing. Deletes fan out too.

### The numbers

| | AWS S3 | tiny-aws object store |
|---|---|---|
| Durability | 99.999999999% (eleven nines) | "Your disk didn't fail" |
| Availability | 99.99% | "The process is running" |
| Consistency | Strong (since Dec 2020) | Eventual (with replication) |
| Max object size | 5 TB | Disk space |
| Storage classes | 8 (Standard thru Deep Archive) | 1 |
| Versioning | Yes | No |
| Lifecycle policies | Yes | No |
| Event notifications | Yes (Lambda, SQS, SNS) | No |
| Encryption | SSE-S3, SSE-KMS, SSE-C | No |
| Replication | Built-in, cross-region | Manual, same-region, configurable factor |
| Access control | Bucket policies, ACLs, IAM | Bearer token (one key for all) |
| Cost | $0.023/GB/month (Standard) | Free (your disk) |
| Implementation | Millions of lines + custom hardware | ~1,200 lines (Rust + C++) |

### What's the same

The API shape: PUT an object with a key, GET it back with the same key. Buckets
as namespaces. Flat key structure (no real directories). ETags for content
verification. Content-type metadata. Bearer auth headers.

---

## Scheduling and job execution

### How AWS does it

AWS doesn't expose its internal scheduler. But internally, when you launch an
EC2 instance, a placement engine picks which physical host to put it on based on
capacity, locality, spread constraints, and dedicated tenancy rules. ECS and
EKS do visible scheduling for containers — bin-packing tasks onto EC2 instances
based on CPU/memory requirements.

### How tiny-aws does it

The scheduler is a Go service (~600 lines) that:
- Receives job submissions via POST /jobs
- Queries the registry for healthy compute nodes
- Round-robin picks a node (or targets a specific instance)
- Stores the job in SQLite with status "pending"
- Agents poll GET /jobs?node_id=X&status=pending every 3 seconds
- Enforces MAX_JOBS_PER_NODE concurrency limit
- Supports retry (once) on failure
- Timeout watchdog marks stale running jobs as failed
- Optionally polls an SQS queue for job submissions
- Fires SNS notifications on job completion/failure

| | AWS (internal + ECS) | tiny-aws scheduler |
|---|---|---|
| Algorithm | Bin-packing, spread, affinity rules | Round-robin |
| Concurrency | Thousands of tasks per cluster | MAX_JOBS_PER_NODE (default 1) |
| Retry | Configurable (ECS: up to 10) | Once |
| Timeout | Configurable per task | JOB_TIMEOUT_SECS (default 3600) |
| Queue integration | SQS, EventBridge | SQS (built-in polling) |
| Job types | Tasks, services, cron | run (one-shot), service (long-running) |
| Implementation | Proprietary | ~600 lines of Go |

---

## Messaging: SQS and SNS

### How AWS does it

SQS is a fully managed distributed queue. Messages are replicated across
multiple servers in multiple AZs. It supports standard queues (best-effort
ordering, at-least-once delivery) and FIFO queues (exactly-once, strict
ordering). Visibility timeout hides a message while a consumer processes it.
Dead letter queues catch repeatedly-failed messages.

SNS is pub/sub: publish a message to a topic, and it fans out to all
subscribers (SQS queues, HTTP endpoints, email, Lambda, SMS, mobile push).

### How tiny-aws does it

SQS: a SQLite table of messages (~187 lines of Go). Send inserts a row. Receive
selects the oldest visible message and bumps its `visible_after` by 30 seconds.
Delete marks it deleted. That's the whole thing.

SNS: another SQLite table (~175 lines of Go). Subscribe adds an endpoint URL to
a topic. Publish does a goroutine fan-out — one HTTP POST per subscriber. If the
POST fails, it logs and moves on (best-effort, no retry).

### The numbers

| | AWS SQS | tiny-aws SQS | AWS SNS | tiny-aws SNS |
|---|---|---|---|---|
| Queue types | Standard + FIFO | Standard only | - | - |
| Delivery | At-least-once (std), exactly-once (FIFO) | At-least-once | Best-effort with retry | Best-effort, no retry |
| Visibility timeout | Configurable (0s-12hr) | 30s (hardcoded) | - | - |
| Dead letter queue | Yes | No | - | - |
| Message size | 256 KB | SQLite TEXT (unlimited-ish) | 256 KB | Unlimited |
| Subscribers | - | - | SQS, HTTP, Lambda, email, SMS | HTTP only |
| Throughput | Unlimited (standard) | SQLite write speed (~thousands/s) | Unlimited | SQLite write speed |
| Cost | $0.40 per million msgs | Free | $0.50 per million | Free |
| Implementation | Proprietary distributed | ~187 lines Go + SQLite | Proprietary distributed | ~175 lines Go + SQLite |

---

## Load balancing: ELB vs load balancer

### How AWS does it

AWS has three load balancers:
- **ALB** (Application Load Balancer): Layer 7, HTTP/HTTPS, path-based routing,
  host-based routing, WebSocket support, sticky sessions
- **NLB** (Network Load Balancer): Layer 4, TCP/UDP, millions of requests per
  second, static IPs
- **CLB** (Classic Load Balancer): Legacy, both L4 and L7

All are fully managed, auto-scaling, multi-AZ, with health checks and SSL
termination.

### How tiny-aws does it

A single Go service (~180 lines) that:
- Polls the registry every 10 seconds for healthy compute nodes
- Health-checks each agent's `/health` endpoint
- Also discovers running services from the registry
- Round-robin forwards HTTP requests to the next healthy target
- Exposes `/targets` to see current backend list

| | AWS ELB/ALB | tiny-aws LB |
|---|---|---|
| Layer | L4 (NLB) or L7 (ALB) | L7 (HTTP only) |
| Algorithm | Round-robin, least connections, flow hash | Round-robin only |
| Health checks | TCP, HTTP, HTTPS, gRPC | HTTP GET /health |
| SSL termination | Yes (ACM certificates) | No |
| Auto-scaling | Yes | No |
| Sticky sessions | Yes | No |
| WebSocket | Yes (ALB) | No |
| Static IP | Yes (NLB) | Yes (your machine's IP) |
| Cost | ~$16/month + data | Free |
| Implementation | Proprietary + custom hardware | ~180 lines of Go |

---

## Networking: VPC and security groups

### How AWS does it

VPC gives you a real virtual network. Your instances get real private IPs. The
VPC has real routing tables that control packet flow. Security groups are
stateful firewalls enforced at the hypervisor level — they filter packets before
they reach your instance.

AWS does this with custom networking hardware (Nitro cards) and SDN
(software-defined networking) that programs physical switches and routers.
It handles ARP, DHCP, DNS, NAT, internet gateways, VPN connections, VPC
peering, transit gateways, PrivateLink...

### How tiny-aws does it

VPC in tiny-aws is metadata (~280 lines of Go). You create a VPC with a CIDR
block, subnets, route tables, security groups, and rules. It's all stored in
SQLite. The network agent (Rust, ~100 lines) reads the security group rules and
writes iptables (Linux) or netsh (Windows) rules.

But there's no real IP allocation, no real routing, no real packet filtering
between instances. Two instances on the same machine can still talk to each
other regardless of what the security group says (unless the iptables rules
happen to block it at the host level).

| | AWS VPC | tiny-aws VPC |
|---|---|---|
| IP allocation | Real (DHCP within CIDR) | Strings in SQLite |
| Routing | Real (route tables, IGW, NAT) | Metadata only |
| Security groups | Stateful, hypervisor-enforced | iptables/netsh rules (host-level) |
| Subnets | Real, AZ-scoped | Metadata (CIDR strings) |
| VPC peering | Yes | No |
| VPN / Direct Connect | Yes | No |
| Network ACLs | Yes (stateless) | No |
| Flow logs | Yes | No |
| DNS | Route 53 integration | No |
| Implementation | Custom hardware + SDN | ~380 lines (Go + Rust) |

### What's the same

The API and the mental model. You create VPCs, subnets, security groups, rules
with inbound/outbound directions and port/protocol/CIDR specifications. If you
learn to think in these terms with tiny-aws, you'll understand the AWS
networking console immediately.

---

## Auth: IAM

### How AWS does it

AWS IAM is a beast. Users, groups, roles, policies (JSON documents specifying
which actions on which resources are allowed or denied), temporary credentials
via STS, cross-account access, identity federation via SAML and OIDC, permission
boundaries, service control policies, session policies, resource-based
policies...

It's the most complicated part of AWS. People make entire careers out of
understanding IAM. The policy language alone has its own evaluation logic with
explicit deny > explicit allow > implicit deny.

### How tiny-aws does it

A SQLite table called `api_keys` with three columns: `key`, `role`, `expires_at`.
Two roles: `admin` (can do everything) and `readonly` (GET requests only). Set
`TINYAWS_API_KEY` env var and all requests need a `Bearer` token. Keys can
expire.

| | AWS IAM | tiny-aws IAM |
|---|---|---|
| Principals | Users, groups, roles, federated identities | API keys |
| Permissions | JSON policies (Allow/Deny per action per resource) | 2 roles: admin, readonly |
| Temporary credentials | STS (AssumeRole, GetSessionToken) | expires_at field |
| Cross-account | Yes | No |
| Identity federation | SAML, OIDC, AWS SSO | No |
| MFA | Yes | No |
| Policy evaluation | 5-step logic with explicit deny | key lookup in SQLite |
| Implementation | Proprietary (massive) | ~80 lines of Go |

---

## Lambda vs Lambda

### How AWS does it

AWS Lambda runs your function in a Firecracker microVM that boots in ~125ms.
Each invocation gets its own isolated environment. The runtime handles
downloading your code, setting up the language environment, and routing the
event to your handler. Cold starts are the time to boot a new microVM; warm
starts reuse an existing one.

Lambda supports Python, Node.js, Java, C#, Go, Ruby, and custom runtimes.
It scales automatically from zero to thousands of concurrent invocations.
You pay per 1ms of compute time.

### How tiny-aws does it

Lambda in tiny-aws (~236 lines of Go) stores function metadata in SQLite. When
you invoke, it builds a shell command that downloads your code zip from the
object store, extracts it, and calls your handler. This command is submitted as
a scheduler job, which gets picked up by an agent and run.

No microVM. No warm containers. Every invocation downloads and extracts the
code fresh. Handler and event are passed as environment variables (not shell
interpolation — that was an injection risk that got fixed).

| | AWS Lambda | tiny-aws Lambda |
|---|---|---|
| Isolation | Firecracker microVM | Agent process (optionally sandboxed) |
| Cold start | ~200ms | Download + unzip time (seconds) |
| Warm start | ~1ms | Not supported (always cold) |
| Runtimes | Python, Node, Java, C#, Go, Ruby, custom | Python 3, Node 20 |
| Concurrency | 1,000+ (auto-scaling) | 1 (one agent job at a time) |
| Max duration | 15 minutes | JOB_TIMEOUT_SECS (default 3600) |
| Memory | 128 MB - 10 GB | No limit (host memory) |
| Layers | Yes (shared dependencies) | No |
| Triggers | API GW, S3, SQS, SNS, DynamoDB, 100+ | Manual invoke only |
| Cost | $0.20 per 1M invocations + $0.0000166667/GB-s | Free |
| Implementation | Firecracker + proprietary | ~236 lines of Go |

---

## The other services: controller, metadata, API gateway

These don't have exact AWS equivalents that are user-facing, but they map to
internal AWS infrastructure:

### Controller (~80 lines of Go)
Polls the registry every 15 seconds for terminated instances and removes their
workspace directories. This is the same reconciliation loop that runs inside
AWS to clean up after terminated instances — deleting EBS volumes, releasing
IPs, removing ENIs. AWS does it across millions of resources. tiny-aws does it
with `os.RemoveAll()`.

### Metadata service (~60 lines of Go)
Fans out to registry, scheduler, and networking to aggregate all resources into
one response. Similar to AWS Resource Explorer or the EC2 instance metadata
service (169.254.169.254). Except tiny-aws's version is 60 lines, not a
distributed system.

### API gateway (~60 lines of Go)
A `httputil.ReverseProxy` that strips `/v1` and routes to backend services.
AWS API Gateway is a full product: REST APIs, HTTP APIs, WebSocket APIs,
throttling, caching, request/response transforms, authorization, usage plans.
tiny-aws's version is literally "strip prefix, forward request."

### Service deploy (long-running apps)
AWS has ECS (Elastic Container Service) and EKS (Kubernetes) for running
long-running services. tiny-aws has `--service --port 3000` on the deploy
command, which spawns a detached process, registers it with the registry, and
the load balancer discovers it. Same concept — service discovery + health
checks + load balancing — at a tiny fraction of the complexity.

---

## What AWS has that tiny-aws doesn't (and probably never will)

- **Databases as a service** (RDS, DynamoDB, Aurora, ElastiCache, Neptune, Redshift, DocumentDB)
- **Container orchestration** (ECS, EKS, Fargate)
- **CI/CD** (CodePipeline, CodeBuild, CodeDeploy)
- **Monitoring** (CloudWatch, X-Ray, CloudTrail)
- **CDN** (CloudFront)
- **DNS** (Route 53)
- **Email** (SES)
- **Search** (OpenSearch, CloudSearch)
- **ML/AI** (SageMaker, Bedrock, Rekognition, Textract...)
- **IoT** (IoT Core, Greengrass)
- **Blockchain** (QLDB, Managed Blockchain)
- **Satellite** (Ground Station)
- **Quantum computing** (Braket)
- **200+ more services**

tiny-aws has the foundation layer. AWS has the foundation plus twenty years of
features on top.

---

## The cost comparison (this one's easy)

| | AWS | tiny-aws |
|---|---|---|
| Compute (1 vCPU, 1 GB) | ~$7.50/month (t3.micro) | Free (your hardware) |
| Storage (100 GB) | ~$2.30/month (S3 Standard) | Free (your disk) |
| Load balancer | ~$16/month (ALB) | Free |
| SQS (1M messages) | $0.40 | Free |
| Lambda (1M invocations) | $0.20 | Free |
| Data transfer (100 GB out) | ~$9.00 | Free (your network) |
| Total for a small app | ~$35-50/month | Electricity + hardware you already own |
| At scale (real company) | $10K - $10M+/month | You need to buy more machines |

The tradeoff: AWS costs money but requires zero hardware. tiny-aws is free but
requires you to own and maintain a Linux machine.

---

## The honest summary

tiny-aws is not a replacement for AWS. It's a working model of AWS. The
difference is like a model airplane vs a 737. The model airplane has wings,
a fuselage, a tail, and it flies. But you wouldn't put passengers in it.

What tiny-aws teaches you:

1. **How the services relate to each other.** Registry talks to scheduler talks
   to agent. Jobs flow through queues. Events fan out through topics. This is
   the exact same service topology as real AWS.
2. **What the APIs look like.** REST endpoints, JSON payloads, status codes,
   auth headers. The tiny-aws CLI maps almost 1:1 to the real AWS CLI.
3. **What "serverless" actually means.** It means "someone else's server." In
   tiny-aws, that someone is you, and you can see the server.
4. **What isolation really is.** Namespaces, cgroups, overlayfs — the same
   primitives Docker and Kubernetes use.
5. **Where the complexity hides.** Replication. Consistency. Failure recovery.
   Networking. Auth policies. These are the things that make AWS hard, and
   tiny-aws deliberately skips most of them so you can see the shape without
   drowning in the details.
6. **What all those AWS services cost to build.** 8,500 lines gets you a working
   prototype. Getting from prototype to production-grade is the other 99.99% of
   the work. Understanding that gap is maybe the most important lesson.

If you want to understand cloud infrastructure — not just use it, but
understand it — tiny-aws is a reasonable place to start. Then go read the AWS
architecture whitepapers and you'll actually understand what they're talking
about.
