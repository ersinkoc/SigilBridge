# ADR-0002: SQLite Storage

## Status

Accepted

## Context

SigilBridge needs durable bridge keys, budget counters, rate-limit buckets, vault ciphertexts, audit indexes, cooldowns, and event history. v1.0 is explicitly single-node.

## Decision

Use SQLite with WAL mode and the pure-Go `modernc.org/sqlite` driver.

## Consequences

- Operators get one local database file.
- Backups can use `VACUUM INTO`.
- Builds stay CGO-free.
- Multi-node writes and distributed counters remain out of scope for v1.0.

## Alternatives Considered

- Postgres: strong concurrency, but violates the no-external-service goal.
- Badger or Pebble: good KV engines, but SQL queries are better for audit and admin views.
- BoltDB: simple file storage, but less ergonomic for relational queries.
