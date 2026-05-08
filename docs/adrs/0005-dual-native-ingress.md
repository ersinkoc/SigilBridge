# ADR-0005: Dual-Native Ingress

## Status

Accepted

## Context

Clients commonly use either OpenAI-compatible Chat Completions or Anthropic Messages. Forcing all clients through only one shape would lose native features or require client rewrites.

## Decision

Expose both OpenAI-compatible `/v1/chat/completions` and Anthropic-native `/v1/messages`, then normalize both into the internal representation.

## Consequences

- Existing SDKs can point at SigilBridge with minimal changes.
- Anthropic-specific fields such as `mcp_servers` and cache markers can be preserved.
- The IR and translators must be tested carefully to avoid semantic loss.

## Alternatives Considered

- OpenAI-only ingress: broad compatibility but weaker Anthropic feature preservation.
- Anthropic-only ingress: strong native support but less ecosystem coverage.
- A custom SigilBridge API: clean internally, poor adoption.
