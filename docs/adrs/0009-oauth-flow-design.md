# ADR-0009: OAuth Flow Design

## Status

Accepted

## Context

Subscription-backed providers often use OAuth. SigilBridge must support both developer workstations with browsers and headless servers.

## Decision

Support Authorization Code with PKCE for browser bootstrap, Device Authorization Grant for headless bootstrap, encrypted refresh-token storage, and a background refresh worker.

## Consequences

- Operators can add credentials without copying passwords.
- Headless hosts can still complete bootstrap.
- Refresh failures can be surfaced before traffic breaks.
- Provider client IDs, scopes, and endpoints need a registry with local overrides.

## Alternatives Considered

- Password grants: obsolete and unsafe.
- Manual token paste only: brittle and poor for refresh.
- Browser-only PKCE: insufficient for servers.
