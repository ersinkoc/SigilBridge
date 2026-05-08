# ADR-0011: Adapter Taxonomy And Auth Preference

## Status

Accepted

## Context

SigilBridge supports many ways to reach a model. Operators need a clear preference order for reliability, compliance, and supportability.

## Decision

Classify adapters by authentication path and prefer them in this order when multiple options exist: API key or cloud IAM, OAuth, CLI agent, local runtime, plugin, then legacy session fallback.

## Consequences

- Official provider APIs and cloud IAM stay the default production path.
- OAuth is the primary subscription path.
- CLI agents are useful for local and developer-machine capacity.
- Session adapters are a last resort and remain disabled by default.
- Plugins can implement any category, but should document their auth model.

## Alternatives Considered

- Treat all adapters equally: simpler config, but hides risk differences.
- Prefer subscriptions first: cost-attractive, but not always compliant or reliable.
- Require plugins for all non-API paths: more extensible, but worse out-of-box usability.
