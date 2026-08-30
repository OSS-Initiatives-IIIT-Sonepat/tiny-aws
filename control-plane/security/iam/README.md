# IAM (minimal)

Optional bearer-token auth via `TINYAWS_API_KEY`.

When set on registry and scheduler, client APIs require:

```
Authorization: Bearer <key>
```

Agents (register, heartbeat, job poll, job patch) are exempt so the data plane keeps working without a key.

Implementation: `control-plane/registry/auth.go` and `control-plane/scheduler/auth.go`.
