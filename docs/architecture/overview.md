# Architecture overview

## System diagram

```
                        +------------------+
                        |   tinyaws CLI    |
                        +--------+---------+
                                 |
              +------------------+------------------+
              |                  |                  |
              v                  v                  v
       +-------------+   +--------------+   +----------------+
       |  API GW     |   |  SQS :9003   |   |  SNS :9004     |
       |  Go :8000   |   |  Go SQLite   |   |  Go SQLite     |
       +------+------+   +--------------+   +----------------+
              |
   +----------+----------+----------+
   |          |          |          |
   v          v          v          v
+-------+ +-------+ +--------+ +----------+
|Registry| |Sched. | | Lambda | | Network  |
|Go:9000| |Go:9001| |Go:9007 | | Go:9005  |
|SQLite | |SQLite | |SQLite  | | SQLite   |
+---+---+ +---+---+ +--------+ +----------+
    |          |
    |  register|  poll jobs
    |  heartbeat poll instances
    v          v
+-------------+    +----------------+
|  EC2 Agent  |    | Object Store   |
|  Rust :8080 |    | Rust+C++ :7001 |
|  workspaces |    | C++ engine     |
+-------------+    | SQLite meta    |
                   +----------------+

+-------------+    +----------------+    +-----------+
| Controller  |    | Network Agent  |    | Load Bal  |
| Go :9002    |    | Rust           |    | Go :8088  |
| workspace   |    | netsh/iptables |    | round-rbn |
| cleanup     |    +----------------+    +-----------+
+-------------+

+-------------+
| Metadata    |
| Go :9006    |
| aggregates  |
+-------------+
```

## Layer summary

| Layer | Components |
|-------|-----------|
| Interface | CLI, API gateway (:8000) |
| Control | Registry, Scheduler, Lambda, Networking, IAM, SQS, SNS, Metadata, Controller |
| Data plane | EC2 agent, Object store, Network agent, Load balancer |

## Start order

```
registry -> ec2-agent -> object-store -> scheduler
(optional) api-gateway, controller, networking, sqs, sns, metadata, lb, lambda, network-agent
```

## Port map

| Service | Port | Language |
|---------|------|----------|
| Registry | 9000 | Go |
| Scheduler | 9001 | Go |
| Controller | 9002 | Go |
| SQS | 9003 | Go |
| SNS | 9004 | Go |
| Networking | 9005 | Go |
| Metadata | 9006 | Go |
| Lambda | 9007 | Go |
| API Gateway | 8000 | Go |
| EC2 Agent | 8080 | Rust |
| Object Store | 7001 | Rust+C++ |
| Load Balancer | 8088 | Go |
