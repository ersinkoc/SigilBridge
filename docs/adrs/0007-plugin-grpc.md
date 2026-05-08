# ADR-0007: Plugin Adapter Protocol

## Status

Accepted

## Context

Operators may need private or experimental adapters without rebuilding the core bridge. The plugin boundary must support streaming, capabilities, and provider error classification.

## Decision

Use a supervised external process model with a gRPC provider contract and a `plugin.yaml` manifest.

## Consequences

- Custom adapters can ship out of tree.
- Plugin crashes are isolated from the core process.
- Streaming support stays explicit in the protobuf contract.
- Plugin authors must track protocol compatibility.

## Alternatives Considered

- Go build tags: simple for developers, poor for distribution.
- In-process dynamic loading: brittle and platform-specific.
- HTTP-only plugins: easy but less typed and harder to stream consistently.
