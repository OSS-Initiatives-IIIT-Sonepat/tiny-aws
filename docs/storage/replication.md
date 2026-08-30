# Storage replication design

## Overview

The object-store supports optional replication to peer storage nodes.
Set `REPLICATION_FACTOR=N` (default 1 = no replication) to write to N-1 peers
after every successful local write.

## Peer discovery

Every 15 s the object-store queries the registry for all healthy `role=storage`
nodes except itself. Peer addresses are derived from the node hostname on port
`:7001`.

`ponytail:` all storage nodes are assumed to run on `:7001`. Add a per-node
address field to the registry if storage nodes need different ports.

## Write path (PUT)

```
client -> PUT /objects/{key}
  local write (C++ engine + SQLite)
  fan-out PUT to write_peers (async, best-effort)
```

Failures to replicate are logged but do not fail the primary write.

## Read path (GET)

```
client -> GET /objects/{key}
  local read -> return if found
  local miss -> try each peer in order -> return first hit
  all miss -> 404
```

## Delete path (DELETE)

```
client -> DELETE /objects/{key}
  local delete
  fan-out DELETE to write_peers (async, best-effort)
```

## Configuration

| Variable | Default | Effect |
|----------|---------|--------|
| `REPLICATION_FACTOR` | `1` | Number of total copies (1 = primary only) |

## Running two nodes locally

```powershell
# Node 1 (primary, drives replication)
$env:OBJECT_STORE_ADDR = "127.0.0.1:7001"
$env:STORAGE_ROOT      = "data1"
$env:METADATA_DB       = "metadata1.db"
$env:REPLICATION_FACTOR = "2"
cargo run

# Node 2 (replica)
$env:OBJECT_STORE_ADDR = "127.0.0.1:7002"
$env:STORAGE_ROOT      = "data2"
$env:METADATA_DB       = "metadata2.db"
cargo run
```

Then run: `.\tests\integration\replication-test.ps1`
