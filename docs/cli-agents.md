# CLI Agents Guide

## Overview

CLI adapters let SigilBridge route requests through locally installed AI agent CLIs that already hold their own authenticated sessions. SigilBridge manages the subprocess lifecycle and communicates over Agent Client Protocol style JSON-RPC over stdio.

The bridge does not store the CLI's account credentials. Authentication remains in the CLI's own credential store.

## Global Config

```yaml
cli_agents:
  enabled: true
  default_idle_timeout_seconds: 600
  default_stderr_capture_bytes: 4096
  health_check_interval_seconds: 60
  spawn_log_level: warn
```

## Pool Example

```yaml
pools:
  - name: local-agent
    strategy: priority_first
    upstreams:
      - id: claude-code-local
        provider: claude_code_cli
        priority: 1
        weight: 1
        config:
          executable: /usr/local/bin/claude
          working_directory: /srv/workspaces/default
          idle_timeout_seconds: 600
```

## Supported Adapters

| Adapter | Expected setup |
| --- | --- |
| `claude_code_cli` | Install Claude Code, run its native login/auth command, then configure executable path. |
| `codex_cli` | Install Codex CLI, authenticate with its native flow, then configure executable path. |
| `gemini_cli` | Install Gemini CLI, authenticate locally, then configure executable path. |
| `aider_cli` | Install Aider plus an ACP-compatible headless shim if required by your version. |

## Install Pattern

1. Install the CLI on the SigilBridge host.
2. Run the CLI's native auth command as the same OS user that runs SigilBridge.
3. Confirm a manual prompt works outside SigilBridge.
4. Add a pool upstream using the matching adapter.
5. Probe from the admin UI or health endpoint.
6. Send a request through a bridge key scoped to that pool.

## Working Directory

Set `working_directory` to a directory the CLI can safely inspect. For production, prefer a dedicated workspace with minimal files. Do not point a CLI agent at sensitive home directories unless that is intentional.

## Lifecycle

- The process starts on first request or admin probe.
- JSON-RPC messages are framed over stdin/stdout.
- Stderr is captured in a ring buffer for admin diagnostics.
- Idle processes are stopped after `idle_timeout_seconds`.
- Crashes are surfaced as health events and can be restarted by the host supervisor.

## Troubleshooting

| Symptom | Likely cause | Resolution |
| --- | --- | --- |
| Executable not found | Bad path or missing install | Use an absolute executable path and verify permissions. |
| Auth failure | CLI was authenticated as a different OS user | Log in as the SigilBridge service user. |
| Protocol error | CLI version does not support expected ACP shape | Upgrade the CLI or configure a compatible shim. |
| Request hangs | CLI waiting for interactive input | Disable interactive prompts or complete auth manually first. |
| Empty stderr | Process exits before writing diagnostics | Check OS service logs and executable permissions. |

## Security Notes

- CLI agents can read files in their working directory.
- Treat the service user's CLI auth store as sensitive.
- Scope bridge keys tightly when routing to local agents.
- Use audit content mode carefully if prompts may include source code or secrets.
