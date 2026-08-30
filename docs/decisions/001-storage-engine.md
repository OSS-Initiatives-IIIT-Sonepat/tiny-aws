# ADR 001: Local C++ block engine instead of HDFS/WebHDFS

**Date:** 2026  
**Status:** Accepted

## Context

tiny-aws needs a storage backend for its object store. The original plan was
to use HDFS or WebHDFS for distributed storage.

## Decision

Use a local C++17 block engine (files on disk) exposed via C FFI to Rust, with
SQLite for object metadata.

## Rationale

| Criterion | HDFS | Local C++ engine |
|-----------|------|------------------|
| Setup complexity | High (JVM, NameNode, DataNodes) | Zero — just compile |
| Dependencies | Java runtime, Hadoop | C++17 compiler, CMake |
| Educational value | Opaque | Fully owned, readable |
| Performance (single node) | Overhead | Direct FS calls |
| Distributed storage | Built-in | Add replication layer (done in Tier C) |

HDFS is designed for multi-petabyte clusters. tiny-aws is an educational
mini-cloud where understanding every layer matters. A hand-rolled C++ engine
makes the storage layer transparent and eliminates the JVM dependency.

## Consequences

- Replication requires explicit implementation (Tier C: fan-out PUT/GET/DELETE).
- No HDFS tooling compatibility (acceptable — tiny-aws has its own CLI).
- Simple disaster recovery: storage is just files, SQLite is just a file.
