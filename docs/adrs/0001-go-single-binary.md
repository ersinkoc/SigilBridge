# ADR-0001: Go Single Binary

## Status

Accepted

## Context

SigilBridge must run on small operator-managed hosts across Linux, macOS, and Windows. The runtime should be easy to install, inspect, and recover without coordinating several services.

## Decision

Build SigilBridge in Go as a single deployable binary, with the React UI embedded at build time.

## Consequences

- Cross-platform builds are straightforward.
- Operators can deploy without Node.js, Python, Redis, or Postgres at runtime.
- Go's standard HTTP, crypto, and concurrency libraries cover most hot-path needs.
- Some ecosystems have richer SDKs in TypeScript or Python, so provider adapters may need direct HTTP implementations.

## Alternatives Considered

- Rust: excellent performance and safety, but a slower iteration path for this project.
- TypeScript/Node.js: fast UI-adjacent development, but a larger runtime footprint.
- Python: strong LLM ecosystem, but weaker single-binary deployment.
