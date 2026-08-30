# Job lifecycle

## Sequence diagram

```
CLI                 Scheduler           EC2 Agent
 |                      |                    |
 |-- POST /jobs -------> |                    |
 |                       | pick healthy node  |
 |                       | save pending       |
 |<-- job_id ----------- |                    |
 |                       |                    |
 |                       |<-- GET /jobs?node= |  (every 3s)
 |                       |-- pending jobs --> |
 |                       |                    |
 |                       |<-- PATCH running-- |
 |                       |                    | run command
 |                       |                    | (in workspace if instance_id)
 |                       |<-- PATCH done ----- |
 |                       |                    |
 |-- GET /jobs/{id} ---> |                    |
 |<-- status=done ------ |                    |
```

## Statuses

| Status | Meaning |
|--------|---------|
| `pending` | Assigned to node, waiting for agent to pick up |
| `running` | Agent executing the command |
| `done` | Exit code 0 |
| `failed` | Non-zero exit, timeout (60s), or retries exhausted |

## Retry

Failed jobs are requeued once (`retry_count=1`). Second failure is final.

## Timeout

Scheduler marks `running` jobs as `failed` after 60 s (`jobTimeout`).

## Concurrency

`MAX_JOBS_PER_NODE` (default 1) limits how many jobs run in parallel per node.
Agent poll returns empty list while the node is at capacity.

## Deploy jobs

`deploy_url` field replaces `command` for deploy jobs:
1. Agent downloads zip from `deploy_url`
2. Extracts to instance workspace (or temp dir)
3. Runs `start.ps1` (Windows) or `start.sh` (Linux)

## Lambda jobs

Lambda service submits a shell `command` that downloads the function zip,
loads the handler module, and runs it. Output is captured as `stdout`.
