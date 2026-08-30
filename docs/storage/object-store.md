# Object store design

## Architecture

```
Client (CLI / other services)
        |
        v
   Rust (axum :7001)
        |
   +---------+---------+
   |                   |
   v                   v
C++ block engine   SQLite metadata
(files on disk)    (size, etag, content-type)
```

## Storage layout

```
$STORAGE_ROOT/
  <key>          flat object
  <bucket>/<key> bucket-scoped object (nested path)
```

## Replication

After every PUT, the object-store fans out the write to `REPLICATION_FACTOR - 1`
peer nodes discovered from the registry. GET falls back to peers on local miss.
DELETE is fanned out too.

Peers are discovered every 15 s from `GET /nodes?role=storage`.

## Authentication

Set `TINYAWS_API_KEY` on the object-store process to require bearer tokens.
The same key must be set on the CLI. The key is checked by an axum middleware
on every request (no exemptions).

## API

| Method | Path | Description |
|--------|------|-------------|
| PUT | `/objects/{key}` | Write object |
| GET | `/objects/{key}` | Read object |
| DELETE | `/objects/{key}` | Delete object |
| GET | `/objects/{key}/meta` | Object metadata |
| GET | `/objects` | List objects |
| PUT | `/buckets/{name}` | Create bucket |
| GET | `/buckets` | List buckets |
| PUT | `/buckets/{b}/objects/{key}` | Write to bucket |
| GET | `/buckets/{b}/objects/{key}` | Read from bucket |
| DELETE | `/buckets/{b}/objects/{key}` | Delete from bucket |
| GET | `/buckets/{b}/objects` | List bucket objects |
