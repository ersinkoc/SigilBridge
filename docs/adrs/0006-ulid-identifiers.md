# ADR-0006: ULID Identifiers

## Status

Accepted

## Context

Requests, bridge keys, audit records, and events need identifiers that are unique, sortable, and readable in logs.

## Decision

Use ULIDs for generated internal IDs.

## Consequences

- IDs sort by time, which helps audit and event inspection.
- IDs are URL-safe and compact.
- Clock behavior matters; generation should use monotonic entropy when possible.

## Alternatives Considered

- UUIDv4: simple but not time-sortable.
- Database autoincrement IDs: local only and less useful across logs.
- Snowflake-style IDs: more operational complexity than needed.
