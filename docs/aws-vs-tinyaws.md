# AWS vs tiny-aws: what's the same, what's not, and why that matters

If you've used AWS (or heard people complain about their AWS bill), you already
know what it does. You click some buttons, your app runs somewhere in Virginia,
and you pay for it. Underneath all that UI and billing and regions and
availability zones, there's actual infrastructure doing actual things.

tiny-aws is what happens when you strip all that away and rebuild the core ideas
on a single Linux machine. Same concepts, drastically simpler implementation.
This doc walks through each service and honestly compares them.

---

## The 30-second version

| What you want | AWS | tiny-aws |
|---|---|---|
| Run a server | EC2 (Xen/Nitro VMs) | `tinyaws instance launch` (systemd-nspawn containers) |
| Store files | S3 (distributed object store) | Object store (C++ block engine + Rust HTTP) |
| Run a function | Lambda (Firecracker microVMs) | Lambda runtime (scheduler job in a container) |
| Queue messages | SQS (distributed queue) | SQS (SQLite-backed queue, single node) |
| Pub/sub events | SNS (fan-out) | SNS (HTTP fan-out, SQLite subscriptions) |
| Load balance | ELB/ALB | Load balancer (round-robin reverse proxy) |
| Private network | VPC | VPC (metadata in SQLite, no real isolation yet) |
| Firewall rules | Security Groups | Security groups (iptables/netsh rules) |
| Auth | IAM (policies, roles, SAML, OIDC) | IAM (api_keys table, admin/readonly roles) |
| CLI | `aws` CLI | `tinyaws` CLI |
| API gateway | API Gateway | API gateway (reverse proxy, strips /v1) |

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

### How tiny-aws does it

tiny-aws uses `systemd-nspawn` — Linux's built-in container runtime. When you
run `tinyaws instance launch`, the agent calls `unshare` to create new PID and
mount namespaces, sets up an overlayfs (so each instance gets its own writable
filesystem on top of a shared base image), does a `pivot_root` to make the
container think its filesystem is the only one, applies cgroup v2 limits for CPU
and memory, and loads a seccomp profile to restrict dangerous syscalls.

It's not a VM. There's no separate kernel. But the isolation is real enough that
you can run `apt install` inside an instance without affecting the host, and the
cgroup limits mean one instance can't eat all the machine's RAM.

### What's honestly different

- **Isolation level.** EC2 gives you hardware-level isolation (a hypervisor). tiny-aws
  gives you OS-level isolation (namespaces + cgroups). A kernel exploit could
  escape a tiny-aws container. It can't escape a Nitro VM.
- **Scale.** EC2 has millions of physical servers across dozens of regions.
  tiny-aws has your one Linux box. Maybe three if you're feeling fancy.
- **Instance types.** EC2 has hundreds (m5.xlarge, c6g.medium, p4d.24xlarge...).
  tiny-aws has five: nano, micro, small, medium, large. They just set different
  cgroup CPU/memory limits.
- **Boot time.** EC2 takes 30-90 seconds. tiny-aws containers start in under a
  second because there's no kernel to boot.
- **Networking.** EC2 instances get real virtual NICs with real IPs. tiny-aws
  instances share the host's network (with optional veth pairs for basic
  isolation).

### What's the same (conceptually)

The lifecycle is identical: launch -> provisioning -> running -> terminated.
Instance IDs follow the same `i-N` convention. You can list, terminate, get
info. The scheduler assigns instances to nodes just like EC2 places VMs on
hosts. The agent heartbeats to the registry just like EC2 instances report to
the control plane.

If you understand tiny-aws instances, you understand the *shape* of EC2. You
just don't understand the hypervisor or the hardware yet, and that's fine.

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

### How tiny-aws does it

tiny-aws has a C++ block engine that writes files to disk. A Rust HTTP server
sits on top with axum, handling PUT/GET/DELETE. SQLite stores metadata (size,
etag, content-type). Buckets are just path prefixes.

Replication exists: when you set `REPLICATION_FACTOR=2`, writes go to multiple
storage nodes (discovered from the registry). Reads fall back to peers if the
local copy is missing. Deletes fan out too.

### What's honestly different

- **Durability.** S3 promises 99.999999999% durability (eleven nines). tiny-aws
  promises "your disk didn't fail." If the disk dies, your data is gone (unless
  you set up replication across machines, and even then it's two copies, not
  three).
- **Consistency.** S3 is strongly consistent. tiny-aws is eventually consistent
  with replication — a write to node A might not be visible on node B for a
  moment.
- **Scale.** S3 handles unlimited objects. tiny-aws handles "however many fit on
  your disk."
- **Features.** S3 has versioning, lifecycle policies, event notifications, S3
  Select, transfer acceleration, storage classes... tiny-aws has PUT, GET,
  DELETE, and list. That's it.

### What's the same

The API shape: PUT an object with a key, GET it back with the same key. Buckets
as namespaces. Flat key structure (no real directories). ETags for content
verification. Content-type metadata.

If you build an app against tiny-aws's object store, porting it to S3 is mostly
changing the URL and adding AWS auth headers.

---

## Messaging: SQS and SNS

### How AWS does it

SQS is a fully managed distributed queue. Messages are replicated across
multiple servers. It supports standard queues (best-effort ordering, at-least-once
delivery) and FIFO queues (exactly-once, strict ordering). Visibility timeout
hides a message while a consumer processes it. Dead letter queues catch
repeatedly-failed messages.

SNS is pub/sub: publish a message to a topic, and it fans out to all
subscribers (SQS queues, HTTP endpoints, email, Lambda, SMS).

### How tiny-aws does it

SQS: a SQLite table of messages. Send inserts a row. Receive selects the
oldest visible message and bumps its `visible_after` by 30 seconds. Delete
marks it deleted. That's the whole thing.

SNS: another SQLite table. Subscribe adds an endpoint URL to a topic. Publish
does a goroutine fan-out — one HTTP POST per subscriber. If the POST fails,
it logs an error and moves on (no retry).

### What's honestly different

- **Reliability.** AWS SQS replicates across data centers. tiny-aws SQS is one
  SQLite file on one disk.
- **Delivery guarantees.** AWS SNS retries failed deliveries with exponential
  backoff. tiny-aws SNS fires and forgets.
- **Scale.** AWS handles billions of messages per day. tiny-aws handles as many
  as SQLite can INSERT per second on your machine (which is actually a lot —
  thousands per second).
- **Features.** No FIFO, no dead letter queues, no message attributes, no
  batching in tiny-aws.

### What's the same

The concepts are 1:1. Create a queue, send messages, receive with visibility
timeout, delete to acknowledge. Create a topic, subscribe an endpoint, publish
to fan out. The scheduler even polls the SQS queue for job submissions, exactly
like a real consumer pattern.

---

## Networking: VPC and security groups

### How AWS does it

VPC gives you a real virtual network. Your instances get real private IPs. The
VPC has real routing tables that control packet flow. Security groups are
stateful firewalls enforced at the hypervisor level — they filter packets before
they reach your instance.

AWS does this with custom networking hardware (Nitro cards again) and SDN
(software-defined networking) that programs physical switches.

### How tiny-aws does it

VPC in tiny-aws is metadata. You create a VPC with a CIDR block, subnets,
security groups, and rules. It's all stored in SQLite. The network agent
reads the security group rules and writes iptables (Linux) or netsh (Windows)
rules.

But there's no real IP allocation, no real routing, no real packet filtering
between instances. Two instances on the same machine can still talk to each
other regardless of what the security group says (unless the iptables rules
happen to block it at the host level).

### What's honestly different

- **Everything.** AWS VPC is real network infrastructure. tiny-aws VPC is a
  database pretending to be a network. The CIDR blocks are strings, not routed
  subnets.

### What's the same

The API and the mental model. You create VPCs, subnets, security groups, rules
with inbound/outbound directions and port/protocol/CIDR specifications. If you
learn to think in these terms with tiny-aws, you'll understand the AWS
networking console immediately.

And that's the point. You're learning the *concepts*, not the kernel
networking. The kernel networking is a semester-long course on its own.

---

## Auth: IAM

### How AWS does it

AWS IAM is a beast. Users, groups, roles, policies (JSON documents specifying
which actions on which resources are allowed or denied), temporary credentials
via STS, cross-account access, identity federation via SAML and OIDC, permission
boundaries, service control policies...

It's the most complicated part of AWS. People make careers out of understanding
IAM.

### How tiny-aws does it

A SQLite table called `api_keys` with three columns: `key`, `role`, `expires_at`.
Two roles: `admin` (can do everything) and `readonly` (GET requests only). Set
`TINYAWS_API_KEY` env var and all requests need a `Bearer` token.

That's it. No policies, no ARNs, no conditions, no cross-account anything.

### What's honestly different

Everything beyond "you need a credential to make API calls." AWS IAM is a
policy engine. tiny-aws IAM is a key/value lookup.

### What's the same

The idea that every API call is authenticated and authorized. The idea that
different principals have different permissions. The idea that credentials
expire. These fundamentals carry over.

---

## Lambda vs Lambda

### How AWS does it

AWS Lambda runs your function in a Firecracker microVM that boots in ~125ms.
Each invocation gets its own isolated environment. The runtime handles
downloading your code, setting up the language environment, and routing the
event to your handler. Cold starts are the time to boot a new microVM; warm
starts reuse an existing one.

### How tiny-aws does it

Lambda in tiny-aws stores function metadata in SQLite. When you invoke, it
builds a shell command that downloads your code zip from the object store,
extracts it, and calls your handler. This command is submitted as a scheduler
job, which gets picked up by an agent and run.

No microVM. No warm containers. Every invocation downloads and extracts the
code fresh.

### What's honestly different

- **Isolation.** AWS Lambda uses Firecracker (a VM). tiny-aws Lambda runs with
  the agent's full privileges (sandboxed only if the agent has sandboxing
  enabled).
- **Performance.** AWS cold starts are ~200ms. tiny-aws cold starts are however
  long it takes to download and unzip your code.
- **Runtimes.** AWS supports many. tiny-aws supports Python 3 and Node 20.

### What's the same

The workflow: upload code to a bucket, register the function with a handler name,
invoke it, get the output back. The handler signature convention (event in,
result out). The idea of FaaS.

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

If you want to understand cloud infrastructure — not just use it, but
understand it — tiny-aws is a reasonable place to start. Then go read the AWS
architecture whitepapers and you'll actually understand what they're talking
about.
