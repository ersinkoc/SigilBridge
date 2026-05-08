# ADR-0008: Subscription Adapter Risk Acknowledgment

## Status

Accepted

## Context

OAuth-backed subscriptions and CLI agents are preferred because they use provider-sanctioned auth surfaces or local tools. Cookie/session replay adapters are different: they may violate provider Terms of Service.

## Decision

Keep session adapters disabled by default. Require explicit operator configuration acknowledging ToS risk before enabling `claude_web` or `chatgpt_web`.

## Consequences

- Operators must make an intentional choice.
- The admin UI and docs can clearly mark session adapters as legacy fallback.
- OAuth and CLI adapters remain the recommended subscription paths.

## Alternatives Considered

- Remove session adapters entirely: cleaner, but less useful for providers without OAuth or CLI options.
- Enable by default: rejected because it hides legal and operational risk.
- Warn only in logs: not strong enough for a high-risk feature.
