# What tiny-aws is missing

This is the honest list. tiny-aws is an educational platform, not a production cloud. These are the known gaps — things that exist in AWS but are deliberately absent, partially implemented, or architecturally impossible in tiny-aws.

---

## Security

- **No TLS/SSL anywhere.** All service-to-service communication is plain HTTP. All CLI calls are plain HTTP. Bearer tokens travel in cleartext. Do not expose any port to the internet.
- **Shared kernel.** Instances share the host kernel. A kernel exploit breaks all isolation instantly. AWS uses Nitro hardware VMs precisely to prevent this. tiny-aws cannot offer this guarantee.
- **No encryption at rest.** Object store writes raw bytes to disk. SQLite databases are unencrypted files. No KMS equivalent.
- **No audit log.** ~~No record of who called what API when. AWS CloudTrail logs every API call. tiny-aws logs nothing persistently.~~ **Implemented.** The API gateway logs every request to SQLite (`audit.db`): timestamp, method, path, query, source IP, identity (masked API key), status code, latency. Query via `GET /v1/audit?limit=50`. No log aggregation, no alerting, no cross-service correlation — but the CloudTrail pattern is real.
- **IAM is two roles.** `admin` and `readonly`. No resource-level permissions, no policy language, no cross-account, no temporary credentials beyond `expires_at`. Real IAM is a distributed system evaluated on every API call.
- **No secret rotation.** API keys are static. No AWS Secrets Manager equivalent.
- **seccomp profile is best-effort.** The syscall filter exists but has not been audited against a threat model.

---

## Networking

- **VPC is metadata, not networking.** CIDRs, subnets, and route tables are strings in SQLite. There is no real IP allocation, no DHCP, no routing enforcement between instances. Two instances on the same machine can communicate regardless of security group rules.
- **No real IP isolation between instances.** Instances on the same host share the host network namespace unless you explicitly configure veth pairs.
- **iptables rules are host-level only.** Security group enforcement happens at the host, not at the hypervisor. This means rules only apply to traffic leaving/entering the host, not between co-located instances.
- **No IPv6.**
- **No VPC peering, Transit Gateway, Direct Connect, or VPN.**
- **No DNS.** No Route 53 equivalent. Service addresses are IPs and ports, not names.
- **Load balancer has no SSL termination.** Plain HTTP only. No ACM, no Let's Encrypt.

---

## Storage

- **Single-node durability.** Object store writes to local disk. One copy unless you configure replication. Disk fails, data gone.
- **Replication is eventually consistent with no conflict resolution.** If two nodes diverge, there is no automatic reconciliation or quorum read.
- **No versioning.** Overwrite an object, previous version is gone.
- **No lifecycle policies.** Objects do not auto-expire or transition to cheaper storage.
- **No multipart upload.** Large file uploads are a single PUT. No resumable uploads.
- **No server-side encryption.**
- **No cross-region replication** (there are no regions).
- **No bucket policies or ACLs.** One API key controls everything.

---

## Compute

- **No live migration.** Instances cannot move between nodes without termination.
- **No GPU support.**
- **No persistent volumes.** Instance filesystem is ephemeral overlayfs. On termination, all data is gone. No EBS equivalent.
- **No snapshots or AMI creation from running instances.**
- **No auto-scaling.** Instance count is always manual.
- **No spot/preemptible instances.**
- **Windows support is partial.** Namespace isolation and cgroup limits require Linux. Windows agents run jobs without sandbox.
- **No nested virtualization.**

---

## Scheduling

- **Round-robin only.** No bin-packing, no spread constraints, no affinity/anti-affinity rules, no capacity-aware placement.
- **No priority queues.** All jobs are equal. No way to say "this job is urgent."
- **Retry is once.** Failed jobs retry exactly one time. No exponential backoff, no DLQ for failed jobs.
- **No preemption.** A running job cannot be displaced to make room for a higher-priority one.
- **MAX_JOBS_PER_NODE defaults to 1.** Concurrency is very conservative by default.

---

## Messaging

- **SQS has no dead letter queue.** ~~Messages that fail processing are lost after visibility timeout expires and the consumer doesn't delete them — they just reappear indefinitely.~~ **Implemented.** Queues can be created with `dead_letter_queue` and `max_receive_count`. Messages that exceed the receive limit are moved to the DLQ automatically. No redrive policy (moving messages back from DLQ to source).
- **SQS has no FIFO queue.** No ordering guarantees. No exactly-once delivery.
- **SQS visibility timeout is hardcoded at 30 seconds.** Not configurable per message or per queue.
- **SNS has no retry.** If a subscriber HTTP endpoint returns an error, the notification is dropped. Real SNS retries with exponential backoff and has a DLQ for failed deliveries.
- **SNS supports HTTP subscribers only.** No email, SMS, Lambda, SQS, or mobile push.
- **No message deduplication.**

---

## Lambda

- **Always cold.** Every invocation downloads the zip and starts fresh. No warm container pool. No sub-100ms starts.
- **Python 3 and Node 20 only.** No Java, C#, Go, Ruby, or custom runtimes.
- **No layers.** No shared dependency packages across functions.
- **No automatic triggers.** ~~Functions must be invoked manually. No S3 → Lambda, no SQS → Lambda wiring.~~ **Implemented.** Lambda supports event triggers: create a trigger with `POST /triggers` specifying `event_source: "s3:ObjectCreated"` and a bucket name. Object store fires SNS on PUT, lambda subscribes and invokes matching functions automatically. No SQS → Lambda wiring yet, no filter rules.
- **Single concurrent invocation per agent.** No parallel invocations.
- **No function versioning or aliases.**

---

## Observability

- **No metrics.** No CloudWatch equivalent. No CPU graphs, no request rates, no error rates.
- **No distributed tracing.** No X-Ray equivalent. No request IDs propagated across services.
- **No centralized logging.** Each service logs to stdout. Service logs are uploaded to object store every 30 seconds, but there is no query interface.
- **No alerting.** Nothing pages you when things break.
- **No dashboards.**

---

## Operations

- **No rolling deploys.** Deploying a new version of a service means stopping the old one and starting the new one. There is downtime.
- **No health-based traffic shifting.** The load balancer removes dead backends but does not do canary or blue/green deployments.
- **No configuration management.** No Parameter Store, no Secrets Manager equivalent. Config is environment variables set at startup.
- **No backup and restore procedures.**
- **No multi-region.** There is one region: your machine.
- **No high availability.** If the control plane goes down, no new jobs can be scheduled. Running instances continue, but nothing new can start.
- **No rate limiting on any API.**

---

## What this means

tiny-aws is safe to run on a single machine for learning. It is not safe to expose to the internet, not suitable for production workloads, and not a replacement for any AWS service.

The gaps listed above are not bugs — most of them are deliberate scope decisions. Each gap points to a real engineering problem that AWS (and other clouds) solved with significant investment. Understanding what is missing and *why it is hard to add* is as valuable as understanding what is present.
