# Testing Guide

## Running Unit Tests

Each Go service has tests in its own directory. Run from the service folder:

```sh
go test ./...
```

Or run benchmarks:

```sh
go test -bench=. -benchmem
```

## Unit Test Files

### control-plane/registry

| File | Covers |
|------|--------|
| `store_test.go` | NodeStore CRUD — Save, LoadAll, Delete, upsert |
| `store_bench_test.go` | NodeStore.Save benchmark |
| `main_test.go` | HTTP handler wiring, node registration endpoints |
| `handlers_test.go` | HTTP handler response codes, JSON encoding |
| `auth_test.go` | API key authentication middleware |
| `expiry_test.go` | Node expiry / staleness detection |
| `healthcheck_test.go` | /health endpoint |
| `iam_test.go` | IAM policy evaluation |
| `instances_test.go` | Instance lifecycle (launch, terminate, list) |
| `instance_sequence_test.go` | Instance ID sequence generation |
| `picknode_test.go` | Node selection algorithm |
| `services_test.go` | Service registry logic |
| `service_handlers_test.go` | Service HTTP handler endpoints |
| `sns_publish_test.go` | SNS publish on instance/node events |

### control-plane/scheduler

| File | Covers |
|------|--------|
| `store_test.go` | JobStore CRUD — Save, LoadAll, upsert, env vars, exit codes |
| `store_bench_test.go` | JobStore.Save benchmark |
| `main_test.go` | HTTP handler wiring, job submission endpoints |
| `auth_test.go` | API key authentication middleware |
| `picknode_test.go` | Node selection for job scheduling |
| `concurrency_test.go` | Concurrent job operations |
| `timeout_test.go` | Job timeout enforcement |
| `sqs_test.go` | SQS job queue integration |
| `sns_notify_test.go` | SNS notification on job state changes |

### control-plane/controller

| File | Covers |
|------|--------|
| `main_test.go` | Controller HTTP endpoints |
| `reconcile_test.go` | Reconciliation loop logic |

### control-plane/cli

| File | Covers |
|------|--------|
| `config_test.go` | Environment-based URL resolution (registry, scheduler, object store) |
| `httpclient_test.go` | HTTP client auth headers, fetchNodes |
| `object_test.go` | objectURL construction (flat/bucket/gateway), parseObjectArgs |
| `bucket_test.go` | Bucket URL construction (create/list, gateway routing) |
| `instance_test.go` | Instance launch payload/volume parsing |
| `deploy_test.go` | Deploy command logic |
| `lambda_test.go` | Lambda CLI subcommand |
| `lb_test.go` | Load balancer CLI subcommand |
| `queue_test.go` | Queue CLI subcommand |
| `storage_test.go` | Storage CLI subcommand |

### control-plane/api

| File | Covers |
|------|--------|
| `main_test.go` | API gateway HTTP wiring |
| `proxy_test.go` | Reverse proxy routing |

### control-plane/metadata

| File | Covers |
|------|--------|
| `main_test.go` | Metadata service endpoints |
| `resources_test.go` | Resource metadata lookups |

### control-plane/messaging/sqs

| File | Covers |
|------|--------|
| `main_test.go` | SQS queue endpoints |
| `ordering_test.go` | FIFO message ordering |
| `visibility_test.go` | Visibility timeout logic |

### control-plane/messaging/sns

| File | Covers |
|------|--------|
| `main_test.go` | SNS topic endpoints |
| `fanout_test.go` | Fan-out to multiple subscribers |

### control-plane/networking/vpc

| File | Covers |
|------|--------|
| `main_test.go` | VPC service endpoints |
| `route_test.go` | Route table logic |

### data-plane/compute/lambda-runtime

| File | Covers |
|------|--------|
| `main_test.go` | Lambda runtime endpoints |
| `invoke_test.go` | Function invocation flow |

### data-plane/networking/load-balancer

| File | Covers |
|------|--------|
| `main_test.go` | Load balancer endpoints |
| `proxy_test.go` | Backend proxy / routing logic |

## Integration Tests

Located in `tests/integration/`. Require running services.

| File | Platform | Covers |
|------|----------|--------|
| `smoke-test.sh` | Linux/macOS | Full-stack smoke: health, nodes, objects, scheduler, jobs, buckets, instances |
| `smoke-test.ps1` | Windows | Same as above |
| `build-smoke.sh` | Linux/macOS | Build verification |
| `build-smoke.ps1` | Windows | Same as above |
| `controller-smoke.sh` | Linux/macOS | Controller health + reconcile trigger |
| `controller-smoke.ps1` | Windows | Same as above |
| `gateway-smoke.sh` | Linux/macOS | API gateway routing: health, nodes, jobs, objects, instances, direct-vs-gateway |
| `gateway-smoke.ps1` | Windows | Same as above |
| `iam-smoke.sh` | Linux/macOS | IAM policy enforcement |
| `lambda-invoke-smoke.sh` | Linux/macOS | Lambda function invocation |
| `lambda-smoke.ps1` | Windows | Lambda CLI smoke |
| `lb-test.ps1` | Windows | Load balancer routing |
| `queue-smoke.ps1` | Windows | SQS queue operations |
| `replication-test.ps1` | Windows | Object store replication |
| `service-smoke.sh` | Linux/macOS | Service registry smoke |
| `sg-test.ps1` | Windows | Security group rules |
| `sns-smoke.sh` | Linux/macOS | SNS publish/subscribe |

## Other Tests

| File | Covers |
|------|--------|
| `tests/chaos/agent-kill.ps1` | Chaos: kill agent, verify recovery |
| `tests/distributed/README.md` | Distributed test documentation |
