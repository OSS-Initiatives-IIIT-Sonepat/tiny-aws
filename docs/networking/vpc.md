# Networking architecture

## Services

| Service | Port | Language |
|---------|------|----------|
| networking (VPC/subnet/SG) | :9005 | Go |
| network-agent | — | Rust |
| metadata aggregator | :9006 | Go |

## VPC model

```
VPC (10.0.0.0/16)
 └─ Subnet (10.0.1.0/24)
     ├─ Route table (0.0.0.0/0 -> igw)
     ├─ Security group
     │   ├─ rule: inbound allow tcp 80 0.0.0.0/0
     │   └─ rule: inbound deny  tcp 22 0.0.0.0/0
     └─ Instance i-1 (assigned via PUT /instances/{id}/subnet)
```

## Endpoints

### Networking service (:9005)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/vpcs` | Create VPC |
| GET | `/vpcs` | List VPCs |
| POST | `/subnets` | Create subnet |
| GET | `/subnets?vpc_id=` | List subnets |
| POST | `/route-tables` | Add route |
| GET | `/route-tables?subnet_id=` | List routes |
| POST | `/security-groups` | Create SG |
| GET | `/security-groups` | List SGs |
| POST | `/security-groups/{id}/rules` | Add rule |
| GET | `/security-groups/{id}/rules` | List rules |
| PUT | `/instances/{id}/subnet` | Assign instance to subnet |

## Network agent

Set `SG_ID=<id>` and `NETWORKING_URL` before starting. The agent polls for
SG rules every 30 s and applies them:

- **Windows**: `netsh advfirewall firewall add rule ...` (best-effort)
- **Linux**: `iptables -A INPUT|OUTPUT ...` (best-effort)

## Metadata aggregator (:9006)

`GET /resources` aggregates nodes, instances, jobs, VPCs, subnets, and
security groups from all services into a single JSON response.
