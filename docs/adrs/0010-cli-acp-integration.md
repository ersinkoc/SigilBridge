# ADR-0010: Agent Client Protocol Integration

## Status

Accepted

## Context

Developers often already have authenticated local AI CLIs. Reusing those tools avoids storing additional provider credentials and gives access to CLI-specific capabilities.

## Decision

Add CLI adapters that spawn configured executables and communicate over an ACP-style JSON-RPC protocol on stdio.

## Consequences

- SigilBridge can route through local agents such as Claude Code, Codex CLI, Gemini CLI, and Aider.
- Credentials stay in each CLI's native auth store.
- Process lifecycle, stderr capture, idle shutdown, and health checks become bridge responsibilities.
- Operators must treat CLI working directories as part of the trust boundary.

## Alternatives Considered

- Reimplement each CLI provider API: duplicates auth and provider behavior.
- Shell out per request without a protocol: inefficient and hard to stream.
- Skip CLI support: misses a major subscription access path.
