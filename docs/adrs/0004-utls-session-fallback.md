# ADR-0004: uTLS For Session Fallback

## Status

Accepted

## Context

Legacy session adapters replay authenticated browser-like traffic. Some providers use TLS client fingerprinting. Standard Go TLS does not look like Chrome.

## Decision

Use `github.com/refraction-networking/utls` for session fallback adapters that need Chrome-like TLS fingerprints.

## Consequences

- Session fallback reliability improves.
- The dependency is isolated to session adapters.
- This does not make session adapters risk-free or provider-approved.
- Operators must explicitly enable session adapters and accept ToS risk.

## Alternatives Considered

- Standard `crypto/tls`: simpler, but fingerprint mismatch is likely.
- Always drive a full browser: heavier and poor for request hot paths.
- Remove session adapters: cleaner legally, but v1.0 keeps them as disabled legacy fallback.
