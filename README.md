# SigilBridge

**One self-hosted bridge for OpenAI-compatible, Anthropic-native, OAuth, CLI-agent, local, and plugin-backed model access.**

SigilBridge is a single Go binary that exposes OpenAI Chat Completions and Anthropic Messages endpoints, then routes each request through the pool you configure. It gives operators one place for bridge keys, budgets, rate limits, audit logs, health checks, fallback routing, and encrypted credential storage.

## Why Use It

- Keep application code pointed at one endpoint while changing providers behind the scenes.
- Mix API-key providers, cloud IAM providers, OAuth-backed subscriptions, local CLI agents, Ollama, and custom plugins.
- Add API keys from the admin UI: the secret is encrypted in the local vault and the selected provider/model is attached to a pool.
- Scan the local machine for Claude Code, Codex, Gemini, and Aider CLI tools and enable available agents from the Credentials page.
- Issue scoped bridge keys with per-key budgets, rate limits, and IP allowlists.
- Keep audit JSONL and SQLite indexes locally, without a hosted control plane.
- Run as one deployable binary with SQLite and an embedded admin UI.

## Install

From source:

```bash
git clone https://github.com/sigilbridge/sigilbridge
cd sigilbridge
bash scripts/release.sh
tar -xzf dist/sigilbridge_dev_linux_amd64.tar.gz -C dist
./dist/sigilbridge_dev_linux_amd64/sigilbridge version
```

On Windows:

```powershell
.\build.ps1 -Version dev
.\dist\sigilbridge.exe version
```

Raw `go build ./cmd/sigilbridge` is useful for compile checks. Production binaries should use `build.ps1`, `release.ps1`, or `scripts/release.sh` so the admin UI is built and embedded first.

UI development requires pnpm:

```bash
pnpm --dir ui install
pnpm --dir ui run build
```

## Five-Minute Quickstart

This quickstart uses the `mock` adapter, so it does not need a provider account.

1. Generate a vault master key:

```bash
export SIGILBRIDGE_MASTER_KEY="$(openssl rand -base64 32)"
```

On PowerShell:

```powershell
$env:SIGILBRIDGE_MASTER_KEY = [Convert]::ToBase64String((1..32 | ForEach-Object { Get-Random -Maximum 256 }))
```

2. Create local config files:

```bash
sigilbridge init --dir .sigilbridge
```

`init` writes `config.yaml`, `pools.yaml`, `oauth_providers.yaml`, and `admin_tokens.yaml` with a local `mock` pool and a generated admin token.

3. Create a stored bridge key:

```bash
sigilbridge keys create test --config .sigilbridge/config.yaml --name local-dev
```

The first line is the plaintext key. It is shown once:

```text
sb_test_0123456789abcdef0123456789abcdef
01HX...
sha256:...
```

4. Start the bridge:

```bash
sigilbridge serve --config .sigilbridge/config.yaml
```

If you run plain `sigilbridge serve` in a directory with no config, SigilBridge bootstraps the same local mock config in that directory and starts from it.

5. Call the OpenAI-compatible ingress:

```bash
curl http://127.0.0.1:8787/v1/chat/completions \
  -H "Authorization: Bearer sb_test_0123456789abcdef0123456789abcdef" \
  -H "Content-Type: application/json" \
  -d '{"model":"mock","messages":[{"role":"user","content":"Hello from SigilBridge"}]}'
```

Or list exposed model aliases:

```bash
curl http://127.0.0.1:8787/v1/models \
  -H "Authorization: Bearer sb_test_0123456789abcdef0123456789abcdef"
```

## Adapter List

| Tier | Adapters |
| --- | --- |
| API key | `anthropic_api`, `openai_api`, `groq`, `gemini_api`, `mistral_api`, `deepseek_api` |
| Cloud IAM | `bedrock`, `vertex_ai`, `azure_openai` |
| OAuth | `claude_oauth`, `copilot_oauth`, `gemini_oauth`, `cursor_oauth` |
| CLI / ACP | `claude_code_cli`, `codex_cli`, `gemini_cli`, `aider_cli` |
| Local | `ollama` |
| Legacy session | `claude_web`, `chatgpt_web` |
| Plugin | gRPC provider plugins discovered from the plugin directory |
| Test | `mock` |

OAuth is the preferred subscription path. Session adapters are disabled by default and require an explicit Terms-of-Service risk acknowledgment.

## Pool Example

```yaml
pools:
  - name: sonnet
    description: Sonnet with subscription first and API-key fallback
    strategy: priority_first
    cooldown:
      initial_seconds: 5
      max_seconds: 300
      backoff: exponential
    upstreams:
      - id: claude-max-personal
        provider: claude_oauth
        priority: 1
        weight: 1
        config:
          credential: vault://oauth/claude_max/personal
          model: claude-sonnet-4-5
      - id: anthropic-prod
        provider: anthropic_api
        priority: 2
        weight: 1
        config:
          api_key: ${ANTHROPIC_API_KEY}
          model: claude-sonnet-4-5-20250929
```

## Documentation

- [.project/SPECIFICATION.md](.project/SPECIFICATION.md): product scope and behavior.
- [.project/IMPLEMENTATION.md](.project/IMPLEMENTATION.md): implementation architecture and data flow.
- [.project/TASKS.md](.project/TASKS.md): implementation workstream checklist.
- [.project/BRANDING.md](.project/BRANDING.md): visual identity, voice, palette, and UI copy guidance.
- [docs/runbook.md](docs/runbook.md): deployment and operations.
- [docs/oauth-setup.md](docs/oauth-setup.md): OAuth provider setup.
- [docs/cli-agents.md](docs/cli-agents.md): CLI agent integration.
- [docs/plugins.md](docs/plugins.md): plugin development guide.
- [docs/release-validation.md](docs/release-validation.md): release smoke checks.
- [docs/adrs](docs/adrs): architecture decision records.

## Development

```bash
go test ./...
go build ./...
go vet ./...
pnpm --dir ui run lint
pnpm --dir ui run test
pnpm --dir ui run build
```

On Windows or PowerShell-first shells, use the helper scripts:

```powershell
.\dev.ps1 -CreateKey          # start backend + Vite UI for local development
.\build.ps1 -Version dev      # test, build UI, embed UI, build binary
.\test.ps1 -Coverage          # run backend/UI tests and named-package coverage
.\test.ps1 -Race -SkipUI      # run Go race tests, using Docker fallback on Windows
.\test.ps1 -Smoke             # run tests plus live local auth/API/UI smoke
.\test.ps1 -DockerSmoke       # build the Docker image and validate API/admin/UI smoke
pnpm --dir ui run test:e2e    # run admin UI browser workflow tests
pnpm --dir ui run lhci        # run Lighthouse thresholds against /admin/ui/
.\clean.ps1                   # remove generated artifacts and restore embed placeholder
.\clean.ps1 -RuntimeState     # zip local state to artifacts/local-state-backups, then remove data/backup/audit
.\scripts\check-sensitive-files.ps1 # fail if local configs, state, or secret-looking values are visible to git
.\release.ps1 -Version v1.0.0 # build multi-arch release tarballs and checksums
```

Run UI e2e/Lighthouse sequentially with build and release scripts because they all read or rewrite `ui/dist`.

The UI is English-first. Turkish can be shipped as an optional i18n locale, but default visible product copy should remain English.

## Contributing

Use the existing package boundaries and add tests with the same risk profile as the change. Any decision that changes storage, security, protocol shape, provider semantics, or deployment posture should get an ADR in `docs/adrs`.

## License

Apache-2.0. See [LICENSE](LICENSE).
