# SigilBridge — SPECIFICATION

| Field | Value |
|---|---|
| **Project** | SigilBridge |
| **Version** | 1.0.0 (initial release scope, draft) |
| **Author** | ECOSTACK TECHNOLOGY OÜ |
| **License** | Apache-2.0 |
| **Tagline** | *"The sigil that bridges every model."* |
| **Domain** | sigilbridge.com (TBD) |
| **Repository** | github.com/SigilBridge (TBD) |
| **Last Updated** | 2026-05-07 |

---

## 1. Overview

SigilBridge is a self-hosted, single-binary AI gateway written in pure Go. It unifies access to commercial LLM providers — through both **official APIs** and **personal subscription accounts** — behind a single OpenAI-compatible and Anthropic-native HTTP interface, with multi-key routing, automatic fallback, per-key budget enforcement, audit logging, and MCP passthrough.

### 1.1 Problem

Teams operating multiple LLM accounts face four pain points:

- **Fragmentation** — Anthropic API key, OpenAI API key, Bedrock IAM, personal Pro subscription, local Ollama; each a different SDK, schema, and error model.
- **No unified access surface** — internal services either hardcode one provider or implement their own multi-provider abstraction layer per project.
- **No central enforcement** — budgets, rate limits, and model whitelists are per-developer, per-app, or nonexistent.
- **No audit trail** — for compliance, cost attribution, debugging, or post-incident review.

A fifth, optional pain point: **subscription accounts have no API surface**. Claude Max, ChatGPT Plus, GitHub Copilot, Gemini Advanced, Cursor Pro — paid capacity the buyer owns but cannot programmatically compose without a bridge.

Two adjacent realities make "just use the API key" insufficient:

1. **Most paid AI products today are subscriptions, not APIs.** Claude Max, ChatGPT Plus, Copilot, Gemini Advanced, Cursor Pro all sell capacity through OAuth-or-login flows. SigilBridge treats **OAuth-based access as the primary subscription path** — sanctioned by the provider, refresh-token-stable, no ToS gray zone. Cookie-scraping `claude_web`/`chatgpt_web` adapters exist as legacy fallbacks only.

2. **Most developers already have CLI agents installed locally.** Claude Code, Codex CLI, Gemini CLI, Aider — each holding a valid session against an active subscription. A bridge that can spawn these tools as subprocesses over **Agent Client Protocol (ACP)** reuses their authentication and quota without storing yet another credential.

### 1.2 Goals

- **Universal authentication**: API keys, cloud IAM (AWS/GCP/Azure), OAuth flows with refresh tokens, session credentials, and local CLI subprocess delegation — all behind a single bridge with one unified provider interface.
- **OAuth-first subscription bridging**: official OAuth flows (PKCE + Device Grant) preferred over reverse-engineered web sessions wherever the provider exposes one — Claude Max, GitHub Copilot, Gemini Advanced, Cursor Pro.
- **Agent Client Protocol (ACP) integration**: spawn locally installed CLI agents (Claude Code, Codex CLI, Gemini CLI, Aider) as managed subprocesses and route requests through them, reusing their already-authenticated sessions and subscription quotas.
- Single static binary, deploys anywhere (Linux, macOS, Windows; amd64, arm64).
- OpenAI-compatible `/v1/chat/completions` + Anthropic-native `/v1/messages` ingress, side-by-side.
- Pluggable provider adapter system: any LLM (official APIs + OAuth flows + CLI agents + subscription bridges + local Ollama).
- Multi-key pools with weighted routing, automatic failover, circuit breaking.
- Per-bridge-key budgets, rate limits, model whitelists, scope-based access control.
- Append-only audit log + Prometheus metrics.
- Encrypted at-rest credential vault for OAuth tokens and subscription credentials.
- Full streaming (SSE) support end-to-end.
- MCP server passthrough for tool-using requests.
- Embedded React admin UI via `embed.FS`.

### 1.3 Non-goals (v1.0)

- Multi-node clustering, Raft consensus, distributed budget counters, S3-shared audit log — deliberate omission; single-node deployment is the v1.0 model. Operators needing HA front the bridge with active-passive failover at the load balancer / DNS level.
- Fine-tuning, batch, or image-generation routing.
- Built-in vector store, RAG, or embeddings cache.
- LLM-as-judge evaluation harness.
- Token-level billing dashboards (basic cost summary only).
- Public SaaS deployment — SigilBridge is self-hosted only.

### 1.4 Comparison to existing tools

| Tool | Language | Single binary | OAuth flows | CLI agents (ACP) | Session bridge | OAI-compat | Anth-native | OSS |
|---|---|---|---|---|---|---|---|---|
| LiteLLM | Python | ✗ | partial | ✗ | ✗ | ✓ | ✗ | ✓ |
| OpenRouter | SaaS | n/a | ✗ | ✗ | ✗ | ✓ | ✗ | ✗ |
| Portkey | Python/SaaS | ✗ | partial | ✗ | ✗ | ✓ | ✓ | partial |
| Helicone | TS/SaaS | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | partial |
| one-api | Go | ✓ | ✗ | ✗ | partial | ✓ | partial | ✓ |
| **SigilBridge** | **Go** | **✓** | **✓** | **✓** | **✓** | **✓** | **✓** | **✓** |

Differentiation: pure Go single binary, **OAuth-first** subscription support (sanctioned by providers), **CLI agent delegation via ACP** (reuses locally-installed agent sessions), dual-native ingress (OAI + Anthropic), document-first design, embedded admin UI.

---

## 2. Design Principles

### 2.1 #NOFORKANYMORE

A single static Go binary. No runtime dependencies, no separate database server, no Docker requirement, no Node.js for the UI. Drop the binary on a host, point a config file at it, run.

### 2.2 Standard library first

Permitted external dependencies, in priority order:

- `golang.org/x/*` (sys, crypto, sync, net) — quasi-stdlib
- `github.com/refraction-networking/utls` — TLS fingerprint spoofing for subscription bridges (no stdlib equivalent)
- `gopkg.in/yaml.v3` — config parsing
- `modernc.org/sqlite` — SQLite driver, **pure Go** (CGO-free, preserves single-binary builds on every platform)
- `github.com/pressly/goose/v3` — SQL schema migrations (embedded `*.sql` files via `embed.FS`)
- `github.com/sony/gobreaker` — circuit breaker (small, audited, no transitive deps)
- `github.com/pkoukk/tiktoken-go` — OpenAI tokenizer
- `github.com/oklog/ulid/v2` — ULID generation
- `github.com/chromedp/chromedp` — browser automation for subscription bootstrap (CLI subcommand only, not in hot path)

Anything else requires explicit justification in an ADR.

### 2.3 Document-first workflow

Every change starts in **SPECIFICATION** → flows to **IMPLEMENTATION** → tasked in **TASKS** → branded in **BRANDING** → readme'd in **README** → captured for autonomous execution in **PROMPT.md**.

### 2.4 Zero-configuration onboarding

Default config produces a runnable instance with the `mock` provider. First real request requires only a single provider credential — pools, routing, budgets, audit all have sensible defaults.

### 2.5 Streaming-native

Every layer respects backpressure. SSE flows from upstream to ingress without buffering full responses. Partial response cleanup: provider closes mid-stream → bridge closes ingress with proper error frame, never a half-open response.

### 2.6 Observable by default

Every request emits a structured audit record. Every adapter exposes Prometheus metrics. Every internal queue has length and latency gauges. No silent failures.

### 2.7 No telemetry, no phone-home

SigilBridge never makes network calls except to configured upstream providers. No version checks, no usage analytics, no error reporting to any third party. Verifiable via firewall.

---

## 3. Architecture

### 3.1 High-level data flow

```
                        Client (OAI/Anthropic SDK)
                                  │
                                  │ HTTPS, Authorization: Bearer sb_live_…
                                  ▼
                       ┌──────────────────────┐
                       │      Ingress         │
                       │  Auth · RateLimit    │
                       │  Budget · Audit-open │
                       └──────────┬───────────┘
                                  │
                                  ▼
                       ┌──────────────────────┐
                       │     Normalizer       │
                       │  Request → IR        │
                       └──────────┬───────────┘
                                  │
                                  ▼
                       ┌──────────────────────┐
                       │       Router         │
                       │  Pool · Strategy     │
                       │  Health · Cooldown   │
                       └──────────┬───────────┘
                                  │
        ┌─────────────────────────┼─────────────────────────┐
        │                         │                         │
        ▼                         ▼                         ▼
 ┌─────────────┐          ┌──────────────┐         ┌──────────────┐
 │ anthropic   │          │  claude_web  │         │  openai_api  │
 │   _api      │          │  (subscr.)   │         │              │
 └─────────────┘          └──────────────┘         └──────────────┘
        │                         │                         │
        └─────────────────────────┼─────────────────────────┘
                                  │
                                  ▼
                       ┌──────────────────────┐
                       │   Denormalizer       │
                       │  IR → ingress format │
                       └──────────┬───────────┘
                                  │
                                  ▼
                          Client response
```

### 3.2 Side stores

| Store | Backed by | Purpose |
|---|---|---|
| `bridge_keys` | SQLite | Bridge-issued keys, hashed, with budgets and scopes |
| `pools` | YAML config + in-memory snapshot | Model→upstream routing config (not in DB) |
| `sessions` | SQLite (encrypted BLOBs) | Subscription credentials |
| `audit` | JSONL files + SQLite index | Append-only request log + indexed lookups |
| `metrics` | In-memory | Prometheus registry |
| `cooldowns` | In-memory + SQLite (persisted on shutdown) | Upstream health state |
| `budget_counters` | SQLite | Per-key spending counters |
| `ratelimit_buckets` | SQLite | Per-key sliding window counters |
| `events` | SQLite | Admin event log |

### 3.3 Internal Representation (IR)

The IR is the canonical request/response model used internally between the Normalizer, Router, and Adapter layers. Every ingress format normalizes into IR; every provider adapter denormalizes IR into its native call.

**IRRequest**:

```go
type IRRequest struct {
    ID            string         // ULID
    BridgeKeyID   string
    ReceivedAt    time.Time
    ModelAlias    string         // pool name, e.g., "sonnet-4.5"
    System        string         // optional system prompt
    Messages      []IRMessage
    Tools         []IRToolDef
    MCPServers    []IRMCPServer
    MaxTokens     int
    Temperature   *float32
    TopP          *float32
    StopSequences []string
    Stream        bool
    Metadata      map[string]string
    Extras        map[string]any  // provider-specific passthrough
}

type IRMessage struct {
    Role    string         // system|user|assistant|tool
    Content []IRContentBlock
}

type IRContentBlock struct {
    Type     string  // text|image|tool_use|tool_result|document
    Text     string
    ImageURL string
    ImageB64 []byte
    ToolUse  *IRToolUse
    ToolResult *IRToolResult
    Document *IRDocument
}
```

**IRResponse**:

```go
type IRResponse struct {
    ID               string
    UpstreamProvider string
    UpstreamModel    string
    StopReason       string  // end_turn|max_tokens|stop_sequence|tool_use|error
    Content          []IRContentBlock
    Usage            IRUsage
    LatencyMs        int64
    TTFBMs           int64
    CostCents        int
    Error            *IRError
}

type IRUsage struct {
    InputTokens       int
    OutputTokens      int
    CacheReadTokens   int
    CacheWriteTokens  int
}
```

**IREvent** (streaming):

```go
type IREvent struct {
    Type    string  // start|delta|content_block_start|content_block_delta|content_block_stop|stop|error|usage
    Delta   *IRContentBlock
    Index   int
    Usage   *IRUsage
    Error   *IRError
    StopReason string
}
```

IR types are versioned internally (`ir.V1Request`, etc.) to allow non-breaking schema evolution.

---

## 4. Component Specifications

### 4.1 Ingress

HTTP/1.1 + HTTP/2 server on configurable port (default `8787`). TLS via embedded cert path or, behind a reverse proxy (nginx, Caddy), TLS terminated upstream and bridge runs plain HTTP on localhost.

**Endpoints**:

| Method | Path | Purpose |
|---|---|---|
| POST | `/v1/chat/completions` | OpenAI compatible (chat + streaming) |
| POST | `/v1/messages` | Anthropic native (chat + streaming) |
| POST | `/v1/messages/count_tokens` | Anthropic token counter |
| GET | `/v1/models` | List available pools as models |
| GET | `/healthz` | Liveness |
| GET | `/readyz` | Readiness (provider health summary) |
| GET | `/metrics` | Prometheus exposition |
| ANY | `/admin/v1/...` | Admin API (separate auth, see §5.3) |
| ANY | `/admin/ui/...` | Embedded React admin UI |

**Concurrency**: one goroutine per request, bounded by `server.max_concurrent_requests` (default 1024). Beyond limit, returns `503` with `Retry-After`.

**Timeouts**: `server.request_timeout_seconds` (default 600s) covers full request lifetime including streaming. `server.idle_timeout_seconds` (default 120s) for keep-alive.

### 4.2 Auth & Bridge Keys

Bridge keys are the credentials internal clients use.

**Format**:
```
sb_live_<32 hex chars>     # production keys
sb_test_<32 hex chars>     # test keys (mock provider only)
```

**Storage**: only the SHA-256 hash of the key is persisted. The plaintext key is shown exactly once at creation; if lost, a new key must be issued.

**Lookup**: ingress reads `Authorization: Bearer sb_live_…`, computes SHA-256, looks up in KV store, caches the result in an LRU (1000 entries, 5-minute TTL). Cache invalidates on admin update or revocation.

**Bridge key record**:

```yaml
id: 01J7ABC...                            # ULID
hash: sha256:9f86d081884c7d659...
name: "team-backend"
created_at: 2026-05-07T10:00:00Z
created_by: "admin@example.com"
last_used_at: 2026-05-08T14:23:11Z
revoked_at: null

scopes:
  allowed_pools: ["sonnet-4.5", "haiku-4.5"]   # [] = all
  allowed_models: []                            # [] = all in scope (passthrough model param)
  ip_allowlist: []                              # CIDR list, [] = all

budgets:
  daily_cents: 5000                             # $50/day
  monthly_cents: 100000                         # $1000/month
  hard_cap: true                                # reject when exceeded; false = warn-only

rate_limits:
  rpm: 60                                       # requests/minute
  tpm: 100000                                   # tokens/minute (input + output)

metadata:                                       # arbitrary tags, surfaced in audit
  team: backend
  cost_center: eng-platform
```

### 4.3 Router & Pool Manager

#### 4.3.1 Pool definition

A pool is a named bucket of upstream candidates for a model alias. Clients reference the model alias (`sonnet-4.5`); the router resolves to a concrete upstream.

```yaml
pools:
  - name: sonnet-4.5
    description: "Anthropic Sonnet 4.5 with subscription fallback"
    strategy: weighted_round_robin

    upstreams:
      - id: anth-key-a
        provider: anthropic_api
        config:
          api_key: ${ANTHROPIC_KEY_A}
          model: claude-sonnet-4-5
        priority: 1
        weight: 70

      - id: anth-key-b
        provider: anthropic_api
        config:
          api_key: ${ANTHROPIC_KEY_B}
          model: claude-sonnet-4-5
        priority: 1
        weight: 30

      - id: claude-web-personal
        provider: claude_web
        config:
          session_ref: vault://claude-personal
          model: claude-sonnet-4-5
        priority: 2
        weight: 100

      - id: bedrock-fallback
        provider: bedrock
        config:
          region: us-east-1
          access_key_id: ${AWS_ACCESS_KEY_ID}
          secret_access_key: ${AWS_SECRET_ACCESS_KEY}
          model_id: anthropic.claude-sonnet-4-5-20251022-v1:0
        priority: 3
        weight: 100

    cooldown:
      initial_seconds: 5
      max_seconds: 300
      backoff: exponential

    circuit_breaker:
      failure_threshold: 5
      recovery_timeout_seconds: 60

    retry:
      max_attempts: 3
      retryable_status_codes: [408, 429, 500, 502, 503, 504]
```

#### 4.3.2 Routing strategies

| Strategy | Behavior |
|---|---|
| `round_robin` | Cycle through upstreams in order, ignoring weight |
| `weighted_round_robin` | Cycle proportionally to `weight` (default) |
| `least_used` | Pick the upstream with fewest in-flight requests |
| `priority_first` | Always pick lowest-priority-number with healthy upstream |
| `random` | Uniform random pick across healthy upstreams |
| `weighted_random` | Random pick weighted by `weight` |

Strategy applies *within* a priority tier. Cross-tier escalation is always priority-ordered.

#### 4.3.3 Selection algorithm

1. Resolve `model_alias` → pool. If no pool found, return 404.
2. Filter upstreams to **healthy + lowest available priority tier**.
3. If no healthy upstreams in lowest tier, escalate to next priority tier.
4. Apply strategy to select one upstream from the tier.
5. Pre-check: budget allows estimated cost? If `hard_cap=true` and predicted cost exceeds remaining budget, return 402 (Payment Required).
6. Dispatch to adapter. On retryable error (5xx, 429, network), mark upstream unhealthy, retry from step 2 (max `retry.max_attempts` total per request).
7. On non-retryable error or final retry exhausted, return upstream error to client (translated to ingress format).

#### 4.3.4 Health state

Every upstream maintains:

```go
type UpstreamHealth struct {
    State                string    // healthy|degraded|cooldown|circuit_open
    InFlight             int
    LastError            error
    LastErrorAt          time.Time
    LastSuccessAt        time.Time
    ConsecutiveFailures  int
    CooldownUntil        time.Time
    CircuitOpenUntil     time.Time
}
```

State transitions:

| From | To | Trigger |
|---|---|---|
| `healthy` | `degraded` | 2 consecutive 5xx within 60s |
| `degraded` | `cooldown` | 5 consecutive failures |
| `cooldown` | `healthy` | `cooldown_until` passed AND probe succeeds |
| `cooldown` | `circuit_open` | 3 consecutive cooldowns |
| `circuit_open` | `cooldown` (half-open) | After `recovery_timeout_seconds` |
| any | `healthy` | Single successful request from `cooldown` half-open state |

### 4.4 Provider Adapters

#### 4.4.1 Adapter taxonomy

SigilBridge categorizes adapters by **authentication model**. All categories implement the same `Provider` interface (§4.4.2) — the taxonomy is documentation, not a Go type hierarchy.

| Category | Auth model | Examples | ToS posture |
|---|---|---|---|
| **API key** | Provider-issued secret | `anthropic_api`, `openai_api`, `groq`, `gemini_api`, `mistral_api`, `deepseek_api` | Sanctioned |
| **Cloud IAM** | Cloud provider's own auth | `bedrock` (SigV4), `vertex_ai` (Service Account), `azure_openai` (Azure key + resource) | Sanctioned |
| **OAuth** | PKCE / Device Grant + refresh token | `claude_oauth`, `copilot_oauth`, `gemini_oauth`, `cursor_oauth` | Sanctioned |
| **CLI / ACP** | Subprocess delegation; CLI uses its own auth | `claude_code_cli`, `codex_cli`, `gemini_cli`, `aider_cli` | Sanctioned (uses each CLI's own auth) |
| **Session** | Reverse-engineered web cookies | `claude_web`, `chatgpt_web` | **Risky** — disabled by default, ToS gray zone |
| **Local** | None (trusted local endpoint) | `ollama` | n/a |
| **Plugin** | Any (defined by the plugin) | Out-of-tree via `hashicorp/go-plugin` | Defined by plugin author |
| **Mock** | None | `mock` | n/a |

**Authentication preference order** when multiple options exist for the same provider:

1. **API key** — when the operator has an account with API access.
2. **OAuth** — when the operator has only a subscription. Sanctioned, refresh-token-stable.
3. **CLI / ACP** — when the developer has the CLI installed locally and wants to reuse that session.
4. **Session** — last resort. Cookie-scraping web bridges, ToS-risky, fingerprint-detection-prone.

`claude_web` is therefore deprecated in favor of `claude_oauth` for any operator with Claude Max access; it remains in the codebase for users without Max plans or for offline-only environments.

#### 4.4.2 Adapter interface

```go
type Provider interface {
    // ID is a stable identifier (e.g., "anthropic_api", "claude_oauth", "claude_code_cli")
    ID() string

    // Chat handles a non-streaming request
    Chat(ctx context.Context, req IRRequest, cfg ProviderConfig) (IRResponse, error)

    // Stream handles a streaming request, emitting IR events
    Stream(ctx context.Context, req IRRequest, cfg ProviderConfig) (<-chan IREvent, error)

    // CountTokens returns input token count (best effort)
    CountTokens(ctx context.Context, req IRRequest, cfg ProviderConfig) (int, error)

    // HealthCheck performs a minimal liveness probe
    HealthCheck(ctx context.Context, cfg ProviderConfig) error

    // Capabilities reports what this provider supports
    Capabilities() ProviderCapabilities
}

type ProviderCapabilities struct {
    Streaming        bool
    ToolUse          bool
    Vision           bool
    PromptCaching    bool
    MCPServers       bool
    DocumentInput    bool
    MaxContextTokens int
    StabilityClass   string  // stable|experimental|risky
    Category         string  // api_key|cloud_iam|oauth|cli_acp|session|local|plugin|mock
}
```

#### 4.4.3 Built-in adapters (all included in v1.0)

| Adapter | Category | Auth | Stability |
|---|---|---|---|
| `mock` | mock | none | stable |
| `anthropic_api` | api_key | API key | stable |
| `openai_api` | api_key | API key | stable |
| `bedrock` | cloud_iam | AWS SigV4 | stable |
| `vertex_ai` | cloud_iam | Service account | stable |
| `azure_openai` | cloud_iam | API key + resource | stable |
| `groq` | api_key | API key | stable |
| `gemini_api` | api_key | API key | stable |
| `mistral_api` | api_key | API key | stable |
| `deepseek_api` | api_key | API key | stable |
| `ollama` | local | none (local HTTP) | stable |
| `claude_oauth` | oauth | OAuth + refresh (Claude Max) | stable |
| `copilot_oauth` | oauth | OAuth + refresh (GitHub Copilot Chat) | stable |
| `gemini_oauth` | oauth | OAuth + refresh (Google) | stable |
| `cursor_oauth` | oauth | OAuth + refresh (Cursor) | experimental |
| `claude_code_cli` | cli_acp | local subprocess (CLI's own auth) | stable |
| `codex_cli` | cli_acp | local subprocess (CLI's own auth) | stable |
| `gemini_cli` | cli_acp | local subprocess (CLI's own auth) | experimental |
| `aider_cli` | cli_acp | local subprocess (CLI's own auth) | experimental |
| `claude_web` | session | session vault (cookie scrape) | **risky** — legacy fallback |
| `chatgpt_web` | session | session vault (cookie scrape) | **experimental** — legacy fallback |

#### 4.4.4 Plugin adapters

Out-of-tree provider adapters via `hashicorp/go-plugin` (subprocess + gRPC). Plugins implement the same `Provider` interface as built-in adapters but run as separate executables, isolated from the bridge process.

Plugin discovery: directory `~/.sigilbridge/plugins/` is scanned at startup. Each plugin is an executable accompanied by a manifest (`plugin.yaml`) declaring its provider ID, capabilities, and required config schema. The bridge spawns and supervises plugin processes, restarts them on crash, and reports their health in `/admin/v1/health`.

Reference plugin: `provider-example/` in the repository — a minimal adapter demonstrating the protocol. Full plugin protocol covered in ADR-0007.

### 4.5 Internal Representation (IR)

Defined in §3.3.

### 4.6 Budget & Rate Limit

#### 4.6.1 Token counting

| Provider family | Method |
|---|---|
| OpenAI | `tiktoken-go` with model-specific encoding (`cl100k_base`, `o200k_base`) |
| Anthropic | Native `messages/count_tokens` endpoint pre-flight if `precise_counting: true`; else local approximation `len(text)/3.5` |
| Gemini, Mistral, others | Provider-specific where available; fallback to char-based with documented uncertainty |
| Subscription bridges | Approximation only — no cost metering on token basis |

#### 4.6.2 Cost calculation

Pricing stored in embedded `pricing.yaml`, refreshed per release:

```yaml
pricing:
  anthropic_api:
    claude-sonnet-4-5:
      input_per_mtok_cents: 300
      output_per_mtok_cents: 1500
      cache_read_per_mtok_cents: 30
      cache_write_per_mtok_cents: 375
    claude-haiku-4-5:
      input_per_mtok_cents: 100
      output_per_mtok_cents: 500

  openai_api:
    gpt-5:
      input_per_mtok_cents: 200
      output_per_mtok_cents: 1000
    gpt-5-nano:
      input_per_mtok_cents: 5
      output_per_mtok_cents: 40
```

Cost formula:

```
cost_cents = round(
  (input × input_rate
   + output × output_rate
   + cache_read × cache_read_rate
   + cache_write × cache_write_rate
  ) / 1_000_000
)
```

Subscription providers (`claude_web`, `chatgpt_web`) default to `cost_cents: 0` — usage is metered against personal subscription quota, not billed per-token. Configurable via `pricing.<provider>.subscription_metering: tokens|requests|none`.

#### 4.6.3 Budget enforcement

Counters per bridge_key, in the `budget_counters` table. Atomic upsert on every committed request:

```sql
INSERT INTO budget_counters (key_id, period, bucket, cents)
VALUES (?, 'daily',   ?, ?)
ON CONFLICT(key_id, period, bucket)
DO UPDATE SET cents = cents + excluded.cents;

INSERT INTO budget_counters (key_id, period, bucket, cents)
VALUES (?, 'monthly', ?, ?)
ON CONFLICT(key_id, period, bucket)
DO UPDATE SET cents = cents + excluded.cents;
```

Bucket values: `YYYY-MM-DD` for daily, `YYYY-MM` for monthly.

**Pre-flight check** (before dispatching to adapter):
- Estimate cost from input tokens × input_rate + max_tokens × output_rate (worst case).
- If `hard_cap=true` and `estimated_cost + current_spend > budget_limit`, return 402:

```json
{
  "type": "error",
  "error": {
    "type": "budget_exceeded",
    "message": "Monthly budget exceeded for bridge key. Limit: $1000.00, used: $999.85, requested: $0.30"
  }
}
```

**Post-flight commit**: actual cost (computed from real usage) is added to counters atomically after response.

#### 4.6.4 Rate limit

Sliding window with per-minute buckets in `ratelimit_buckets`. Increment + read in one statement:

```sql
INSERT INTO ratelimit_buckets (key_id, metric, bucket, value)
VALUES (?, 'rpm', ?, 1)
ON CONFLICT(key_id, metric, bucket)
DO UPDATE SET value = value + 1
RETURNING value;
```

`bucket` is the minute epoch (`unix_ts / 60`). A background worker prunes buckets older than 10 minutes every minute. On exceed, return 429 with `Retry-After: <seconds>` and `X-RateLimit-Remaining: 0` headers.

### 4.7 Audit & Observability

#### 4.7.1 Audit log format (JSONL)

One record per request, written to `audit/YYYY-MM-DD.jsonl`. Daily rotation, gzip after 7 days, automatic prune after `retention_days` (default 90).

```json
{
  "ts": "2026-05-07T14:23:11.123Z",
  "request_id": "01JX...",
  "bridge_key_id": "01J...",
  "ingress_format": "anthropic",
  "model_alias": "sonnet-4.5",
  "upstream_provider": "anthropic_api",
  "upstream_id": "anth-key-a",
  "upstream_model": "claude-sonnet-4-5",
  "input_tokens": 1284,
  "output_tokens": 412,
  "cache_read_tokens": 0,
  "cache_write_tokens": 0,
  "cost_cents": 11,
  "latency_ms": 3214,
  "ttfb_ms": 412,
  "stream": true,
  "stop_reason": "end_turn",
  "status": "success",
  "error": null,
  "retries": 0,
  "client_ip_hash": "sha256:abc...",
  "user_agent": "anthropic-sdk-python/0.45.0",
  "metadata": {
    "team": "backend",
    "cost_center": "eng-platform"
  }
}
```

**Content modes** (config: `audit.content_mode`):

| Mode | Behavior |
|---|---|
| `none` (default) | No prompt or response content captured |
| `hash` | SHA-256 of prompt + response strings |
| `truncated` | First 500 chars of prompt and response |
| `full` | Complete prompt + response (warning: PII risk, GDPR considerations) |

#### 4.7.2 Prometheus metrics

```
# Counters
sigilbridge_requests_total{ingress, model, provider, upstream, status}
sigilbridge_tokens_total{direction="input|output|cache_read|cache_write", provider, upstream}
sigilbridge_cost_cents_total{provider, upstream}
sigilbridge_errors_total{type, provider, upstream}

# Histograms
sigilbridge_request_duration_seconds{ingress, provider, upstream}
sigilbridge_ttfb_seconds{provider, upstream}

# Gauges
sigilbridge_inflight_requests{provider, upstream}
sigilbridge_upstream_health{provider, upstream}        # 0=down, 1=degraded, 2=healthy
sigilbridge_circuit_breaker_state{provider, upstream}  # 0=closed, 1=open, 2=half_open
sigilbridge_budget_used_cents{key_id, period="daily|monthly"}
sigilbridge_budget_limit_cents{key_id, period="daily|monthly"}
sigilbridge_session_expiry_seconds{session_id}         # for subscription credentials
```

### 4.8 Session Vault (subscription)

Encrypted at-rest store for subscription credentials. Required for any pool that references `vault://...` upstream config.

#### 4.8.1 Encryption

- Master key: 256-bit AES key, sourced from `SIGILBRIDGE_MASTER_KEY` env var (base64-encoded). Required at startup; bridge refuses to start any pool that references the vault if master key is absent.
- Per-record encryption: AES-256-GCM with random 96-bit nonce per record, stored alongside ciphertext.
- AAD (additional authenticated data): record ID + record version, prevents swap attacks.
- Key derivation: master key used directly (no per-record subkey — simplifies rotation; ADR-0003 covers an optional HKDF-based per-session subkey scheme for environments that require it).

#### 4.8.2 Record schema

```yaml
session_id: vault://claude-personal
provider: claude_web
created_at: 2026-05-07T10:00:00Z
last_refreshed_at: 2026-05-07T16:00:00Z
expires_at: 2026-06-06T10:00:00Z
ciphertext_b64: "AES256-GCM ciphertext..."
nonce_b64: "12-byte nonce..."
metadata:
  account_email_hash: "sha256:..."        # never plaintext
  organization_uuid: "..."
```

Decrypted plaintext per provider:

```yaml
# claude_web
session_key: "sk-ant-sid01-..."
organization_uuid: "..."
user_agent: "Mozilla/5.0..."
ja3_fingerprint: "..."
cookies: {...}                            # full cookie jar from bootstrap

# chatgpt_web
session_token: "eyJ..."
access_token: "eyJ..."
user_agent: "..."
ja3_fingerprint: "..."
cookies: {...}
```

#### 4.8.3 Session bootstrap

First-time setup uses an out-of-band CLI subcommand:

```
sigilbridge session add --provider claude_web --name claude-personal
```

This launches a headless-but-visible Chrome instance via `chromedp`, navigates to the provider's login page, waits for the user to log in (fully interactive — supports MFA, magic links, SSO), then captures cookies and TLS fingerprint and writes the encrypted record to the vault.

**Bootstrap flow**:
1. Start chromedp with realistic Chrome flags (no `--headless` to avoid bot detection during login).
2. Navigate to provider's signin URL.
3. Wait for `document.cookie` to contain the session cookie name (timeout: 5 min).
4. Hit a benign authenticated endpoint (e.g., `/api/organizations`) to confirm session is valid.
5. Extract User-Agent from browser, capture full cookie jar.
6. JA3 fingerprint synthesized from Chrome version's known fingerprint database (vendored).
7. Encrypt and persist.
8. Run a smoke test: send a 1-token completion request via the new session.

#### 4.8.4 Session refresh worker

Background goroutine, runs every `subscription_adapters.refresh_interval_seconds` (default 6 hours):

1. For each session: attempt a benign authenticated endpoint call.
2. If 200 and Set-Cookie present: re-encrypt with rotated values.
3. If 401: mark session expired, emit `session_expired` admin event, alert via Prometheus.
4. If Cloudflare/Akamai challenge HTML: mark session degraded, alert (likely fingerprint flag).

Expired sessions do not crash the bridge — affected pools degrade or use fallback upstreams.

### 4.9 Storage

Single embedded **SQLite** database. Driver: `modernc.org/sqlite` (pure Go, CGO-free), so the build remains a single static binary on every supported platform. Schema migrations live as `migrations/*.sql` files, embedded via `embed.FS` and applied at startup by `goose`.

#### 4.9.1 Pragmas applied at open

```sql
PRAGMA journal_mode = WAL;          -- concurrent reader/writer, durable
PRAGMA synchronous = NORMAL;        -- safe with WAL, ~10× faster than FULL
PRAGMA foreign_keys = ON;
PRAGMA busy_timeout = 5000;         -- wait up to 5s on lock contention
PRAGMA temp_store = MEMORY;
PRAGMA cache_size = -20000;         -- 20MB page cache
PRAGMA mmap_size = 268435456;       -- 256MB memory-mapped I/O
```

#### 4.9.2 Schema (v1.0)

```sql
-- Bridge keys (issued credentials)
CREATE TABLE bridge_keys (
    id                TEXT PRIMARY KEY,           -- ULID
    hash              TEXT NOT NULL UNIQUE,       -- SHA-256 hex of the plaintext key
    name              TEXT NOT NULL,
    created_at        DATETIME NOT NULL,
    created_by        TEXT,
    last_used_at      DATETIME,
    revoked_at        DATETIME,
    scopes_json       TEXT NOT NULL,              -- {allowed_pools, allowed_models, ip_allowlist}
    budgets_json      TEXT NOT NULL,              -- {daily_cents, monthly_cents, hard_cap}
    rate_limits_json  TEXT NOT NULL,              -- {rpm, tpm}
    metadata_json     TEXT NOT NULL DEFAULT '{}'  -- arbitrary tags
);
CREATE INDEX idx_bridge_keys_hash_active
  ON bridge_keys(hash) WHERE revoked_at IS NULL;

-- Subscription session vault (encrypted at rest)
CREATE TABLE sessions (
    id                  TEXT PRIMARY KEY,         -- e.g., "claude-personal"
    provider            TEXT NOT NULL,            -- claude_web | chatgpt_web
    created_at          DATETIME NOT NULL,
    last_refreshed_at   DATETIME,
    expires_at          DATETIME,
    nonce               BLOB NOT NULL,            -- 12-byte AES-GCM nonce
    ciphertext          BLOB NOT NULL,            -- AES-256-GCM encrypted credential bundle
    metadata_json       TEXT NOT NULL DEFAULT '{}'
);

-- Spending counters per bridge key, per period
CREATE TABLE budget_counters (
    key_id   TEXT NOT NULL,
    period   TEXT NOT NULL,                       -- 'daily' | 'monthly'
    bucket   TEXT NOT NULL,                       -- '2026-05-07' or '2026-05'
    cents    INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (key_id, period, bucket),
    FOREIGN KEY (key_id) REFERENCES bridge_keys(id) ON DELETE CASCADE
);

-- Per-minute rate limit buckets
CREATE TABLE ratelimit_buckets (
    key_id   TEXT NOT NULL,
    metric   TEXT NOT NULL,                       -- 'rpm' | 'tpm'
    bucket   INTEGER NOT NULL,                    -- minute epoch (unix_ts / 60)
    value    INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (key_id, metric, bucket),
    FOREIGN KEY (key_id) REFERENCES bridge_keys(id) ON DELETE CASCADE
);
CREATE INDEX idx_ratelimit_bucket ON ratelimit_buckets(bucket);

-- Index into JSONL audit files for fast queries
CREATE TABLE audit_index (
    request_id      TEXT PRIMARY KEY,             -- ULID
    ts              DATETIME NOT NULL,
    bridge_key_id   TEXT,
    pool_name       TEXT,
    upstream_id     TEXT,
    status          TEXT NOT NULL,                -- success | error | budget_exceeded | …
    cost_cents      INTEGER NOT NULL DEFAULT 0,
    file_path       TEXT NOT NULL,                -- 'audit/2026-05-07.jsonl'
    file_offset     INTEGER NOT NULL,             -- byte offset for fast seek
    file_length     INTEGER NOT NULL
);
CREATE INDEX idx_audit_ts   ON audit_index(ts);
CREATE INDEX idx_audit_key  ON audit_index(bridge_key_id, ts);
CREATE INDEX idx_audit_pool ON audit_index(pool_name, ts);

-- Upstream health state (in-memory primary; persisted on shutdown for warm start)
CREATE TABLE cooldowns (
    upstream_id            TEXT PRIMARY KEY,
    pool_name              TEXT NOT NULL,
    state                  TEXT NOT NULL,         -- healthy|degraded|cooldown|circuit_open
    consecutive_failures   INTEGER NOT NULL DEFAULT 0,
    last_error             TEXT,
    last_error_at          DATETIME,
    last_success_at        DATETIME,
    cooldown_until         DATETIME,
    circuit_open_until     DATETIME,
    updated_at             DATETIME NOT NULL
);

-- Admin / system events
CREATE TABLE events (
    id            TEXT PRIMARY KEY,               -- ULID
    ts            DATETIME NOT NULL,
    type          TEXT NOT NULL,                  -- session_expired | pool_reload | admin_action | …
    severity      TEXT NOT NULL,                  -- info | warn | error
    payload_json  TEXT NOT NULL,
    actor         TEXT
);
CREATE INDEX idx_events_ts   ON events(ts);
CREATE INDEX idx_events_type ON events(type, ts);
```

#### 4.9.3 What is **not** in SQLite

- **Pool definitions** — live in `pools.yaml`, loaded into memory at startup, hot-reloaded via admin endpoint. SQLite stores state, not config.
- **Audit log content** — JSONL files in `audit/YYYY-MM-DD.jsonl`. SQLite holds only the index for fast filtering and offset-based reads.

#### 4.9.4 Why SQLite

- Familiar to every operator: `sqlite3 data/sigilbridge.db` opens a working REPL with no extra tooling.
- Single file. Easy to ship, inspect, copy, encrypt at the filesystem level.
- WAL mode gives concurrent readers + a single writer with no extra moving parts and no separate server process.
- Schema enforcement, foreign keys, joins, indexes — admin queries and audit lookups stay declarative.
- Backup is `sqlite3 data/sigilbridge.db ".backup data/backup.db"` or `VACUUM INTO`.

#### 4.9.5 Concurrency

`database/sql` connection pool: **1 writer connection** (serialized via a Go mutex around the writer pool) + **N reader connections** (default `runtime.NumCPU()`). SQLite WAL guarantees readers never block writers and vice versa for the durations that matter at this scale.

Hot paths use prepared statements:
- Bridge key lookup by hash (cached via in-process LRU)
- Budget upsert (one statement)
- Rate-limit bucket upsert with `RETURNING` (one statement)
- Audit-index insert (one statement, fire-and-forget on a buffered channel)

#### 4.9.6 Backup

Nightly `VACUUM INTO 'backup/sigilbridge-YYYY-MM-DD.db'`, kept for `storage.backup.retention_days` (default 14). Manual: `sigilbridge backup --output path.db`.

### 4.10 OAuth Flow Manager

The component responsible for OAuth-based provider authentication. **OAuth is the preferred subscription-bridging mechanism** wherever the provider exposes it: sanctioned by the provider, refresh-token-stable, no ToS gray zone.

#### 4.10.1 Supported flow types

| Flow | RFC | When used |
|---|---|---|
| **Authorization Code + PKCE** | RFC 7636 | Primary path on desktop / interactive setups; opens system browser, captures redirect on a localhost listener |
| **Device Authorization Grant** | RFC 8628 | Headless deployments (servers, CI). User enters a verification code on a separate device |
| **Refresh Token Grant** | RFC 6749 §6 | Background refresh of near-expiry access tokens |

#### 4.10.2 Bootstrap (interactive)

CLI subcommand:

```
sigilbridge oauth add --provider claude_oauth --name claude-max-personal
sigilbridge oauth add --provider copilot_oauth --name copilot-work --device   # headless
```

Authorization Code + PKCE flow:

1. Bridge generates `code_verifier` (43–128 bytes) and `code_challenge = BASE64URL(SHA256(code_verifier))`.
2. Spawns a local HTTP listener on `127.0.0.1:<random_port>` for the redirect URI.
3. Opens the system browser to the provider's authorization URL with `code_challenge`, `code_challenge_method=S256`, `state`, and the requested scopes.
4. User authenticates in their normal browser session (cookies, MFA, SSO — whatever).
5. Provider redirects back to the localhost listener with `?code=...&state=...`.
6. Bridge verifies `state`, exchanges `code + code_verifier` at the provider's token endpoint for `access_token + refresh_token + expires_in`.
7. Tokens encrypted (AES-256-GCM) and stored in vault as `oauth://<provider>/<name>`.

Device Grant flow (headless):

1. Bridge POSTs to the provider's device authorization endpoint, receives `device_code`, `user_code`, `verification_uri`, `interval`.
2. Bridge prints `Open <verification_uri> on any device and enter code: <user_code>`.
3. Bridge polls the token endpoint every `interval` seconds with `device_code`.
4. On user approval: receives tokens, stores in vault.

#### 4.10.3 Refresh worker

Runs every `oauth.refresh_check_interval_seconds` (default 300 = 5 min):

1. Iterate every `oauth://*` vault entry.
2. If `expires_at - now < oauth.refresh_lead_time_seconds` (default 300), call the provider's token endpoint with `grant_type=refresh_token`.
3. On success: re-encrypt with new `access_token` (and rotated `refresh_token` if returned), persist, update `expires_at`.
4. On failure (revoked, network error, provider 4xx): mark entry `expired`, emit `oauth_refresh_failed` event, raise Prometheus alert.

Expired OAuth entries do not crash the bridge — affected pools degrade or use fallback upstreams.

#### 4.10.4 Provider OAuth registry

Embedded `oauth_providers.yaml` ships with SigilBridge, defining each provider's OAuth endpoints, scopes, and quirks:

```yaml
oauth_providers:
  claude_oauth:
    authorize_url: "https://claude.ai/oauth/authorize"
    token_url:     "https://api.anthropic.com/oauth/token"
    client_id:     "9d1c250a-e61b-44d9-88ed-5944d1962f5e"   # public client ID for OAuth-flow apps
    scopes:        ["org:read", "messages:write"]
    use_pkce:      true
    refresh_supported: true
    upstream_endpoint: "https://api.anthropic.com/v1/messages"

  copilot_oauth:
    authorize_url: "https://github.com/login/oauth/authorize"
    token_url:     "https://github.com/login/oauth/access_token"
    client_id:     "Iv1.b507a08c87ecfe98"                    # GitHub Copilot's public client ID
    scopes:        ["copilot", "read:user"]
    use_pkce:      true
    refresh_supported: true
    upstream_endpoint: "https://api.githubcopilot.com/chat/completions"

  gemini_oauth:
    authorize_url: "https://accounts.google.com/o/oauth2/v2/auth"
    token_url:     "https://oauth2.googleapis.com/token"
    client_id:     "<configurable>"
    scopes:        ["https://www.googleapis.com/auth/cloud-platform"]
    use_pkce:      true
    refresh_supported: true
    upstream_endpoint: "https://generativelanguage.googleapis.com/v1beta/models"

  cursor_oauth:
    authorize_url: "https://cursor.com/api/auth/authorize"
    token_url:     "https://cursor.com/api/auth/token"
    client_id:     "<configurable>"
    scopes:        ["chat", "models:read"]
    use_pkce:      true
    refresh_supported: true
    upstream_endpoint: "https://api2.cursor.sh/v1/chat/completions"
```

Operators override via `~/.sigilbridge/oauth_providers.yaml` for self-hosted Anthropic instances, custom Cloudflare AI Gateway in front of OpenAI, enterprise SSO in front of Gemini, etc.

#### 4.10.5 Vault schema for OAuth entries

```yaml
session_id: oauth://claude_oauth/claude-max-personal
provider:   claude_oauth
created_at: 2026-05-07T10:00:00Z
last_refreshed_at: 2026-05-07T15:55:00Z
expires_at: 2026-05-07T16:00:00Z
nonce_b64:  "..."
ciphertext_b64: |
  Encrypted JSON:
    access_token:  "..."
    refresh_token: "..."
    token_type:    "Bearer"
    scope:         "org:read messages:write"
    id_token:      "..."          # if OIDC
metadata:
  account_email_hash: "sha256:..."
  organization_uuid:  "..."
```

### 4.11 CLI Agent Spawner (Agent Client Protocol)

CLI-category adapters spawn already-installed local CLI agents (Claude Code, Codex CLI, Gemini CLI, Aider) as managed subprocesses and communicate with them over **Agent Client Protocol (ACP)** — a JSON-RPC 2.0-over-stdio protocol used by Zed and others to embed agents in editors.

#### 4.11.1 Why CLI adapters

- **Zero new credentials**: reuses the developer's already-authenticated CLI session — no OAuth flow, no API keys, no vault entry.
- **Native subscription quota**: most CLI agents already implement Pro/Max/team quota natively (Claude Code uses Claude Max; Codex CLI uses ChatGPT subscription).
- **Always current**: each new release of the upstream CLI is automatically picked up — no adapter maintenance.
- **Composable**: tools normally chained interactively (Claude Code → Codex review) become programmatically composable through bridge pools.

#### 4.11.2 Spawn lifecycle

```
First request → check process pool for upstream `id`
                ├── alive + idle → reuse
                └── absent → spawn:
                              1. exec.Cmd with `acp_args`, `env`, cwd
                              2. wire stdin/stdout to JSON-RPC framer
                              3. send `initialize` request, await capability response
                              4. register in pool

Per request:
  a. translate IRRequest → ACP `agent.message` request
  b. send over stdin, framing each call with Content-Length header
  c. consume stdout: dispatch `agent.message_delta` notifications → IREvent stream
  d. on terminator (`agent.message_complete`): close the IREvent channel

Idle management:
  - per-process timer; reset on every request
  - on timeout (`cli.idle_timeout_seconds`, default 600s):
       send ACP `shutdown`, wait up to 5s, kill if unresponsive
  - bridge shutdown: graceful shutdown all CLI subprocesses

Crash recovery:
  - stderr captured to ringbuffer (last 4KB) for diagnostics
  - mark upstream sick on non-zero exit
  - restart on next request with exponential backoff (5s → 60s)
```

#### 4.11.3 Authentication delegation

CLI adapters do **not** store credentials in the SigilBridge vault. The CLI subprocess uses its own credential store (e.g., `~/.config/claude/credentials.json`, OS keychain, shell env). This is by design: bridge delegates authentication to the CLI tool, inheriting whatever auth state the CLI already has.

Pre-flight check: at startup and via `/admin/v1/health`, bridge runs the CLI's auth status subcommand (per-CLI):

| CLI | Auth check command |
|---|---|
| Claude Code | `claude auth status` |
| Codex CLI | `codex auth status` |
| Gemini CLI | `gemini auth list` |
| Aider | n/a (config-file driven) |

Auth issues surface in `/admin/v1/health` and the embedded UI's Session Manager panel, with operator-actionable hints (e.g., "Run `claude auth login` on the host to refresh").

#### 4.11.4 Adapter configuration

```yaml
upstreams:
  - id: claude-code-local
    provider: claude_code_cli
    config:
      executable: "claude"                # or absolute path; PATH lookup fallback
      acp_args: ["--acp"]                 # CLI's ACP-mode argument(s)
      working_directory: "/var/lib/sigilbridge/cli-sandbox"
      idle_timeout_seconds: 600
      max_concurrent_calls: 1             # most CLI agents serialize internally
      env:
        CLAUDE_LOG_LEVEL: "warn"
        # Inherits parent env unless `env_isolate: true`
      env_isolate: false
      stderr_capture_bytes: 4096
```

#### 4.11.5 Limitations

- **Single concurrency per subprocess** — most CLI agents serialize internally; for higher throughput, configure multiple upstreams pointing to multiple subprocess instances.
- **No mid-tool-call streaming granularity** — CLI may collapse intermediate tool execution into a single result block.
- **Latency overhead** — ~200–500 ms per request (subprocess IPC + JSON-RPC framing + CLI startup if cold).
- **Memory** — each idle subprocess holds ~50–200 MB RAM (the CLI itself).
- **Cross-platform** — Linux + macOS are tier-1; Windows behavior may differ depending on the CLI's platform support.
- **Sandbox** — CLI subprocesses run in a configured working directory; operators are responsible for filesystem isolation. Bridge does not chroot or namespace.

#### 4.11.6 Supported CLIs (v1.0)

| CLI | Adapter | Auth source | ACP version | Notes |
|---|---|---|---|---|
| Claude Code | `claude_code_cli` | Claude Max / API | 1.x | Primary reference implementation; tier-1 |
| Codex CLI | `codex_cli` | ChatGPT subscription / API | 1.x | OpenAI's official CLI; tier-1 |
| Gemini CLI | `gemini_cli` | Google account | 1.x | Experimental — protocol still settling |
| Aider | `aider_cli` | Various (env / config) | community port | Community-supported; needs ACP shim |

Additional CLIs add via the plugin SDK (§4.4.4) — implement the ACP shim, register a `provider_id`, ship as a SigilBridge plugin.

---

## 5. API Surface

### 5.1 OpenAI-compatible

#### 5.1.1 POST `/v1/chat/completions`

Full OpenAI Chat Completions schema:

```json
{
  "model": "sonnet-4.5",
  "messages": [
    {"role": "system", "content": "You are helpful."},
    {"role": "user", "content": "Hello"}
  ],
  "temperature": 0.7,
  "max_tokens": 4096,
  "stream": true,
  "tools": [...],
  "tool_choice": "auto",
  "response_format": {"type": "json_schema", ...},
  "user": "user-123"
}
```

`model` is the **pool name** (model alias), not the upstream model name. Bridge resolves the pool, selects upstream, translates to upstream-native format, translates response back to OAI format.

**Streaming**: SSE with `data: {...}\n\n` chunks, terminating `data: [DONE]\n\n`. Bridge re-frames upstream chunks (e.g., Anthropic event-stream → OAI delta format) as needed.

#### 5.1.2 GET `/v1/models`

Returns pools as model objects:

```json
{
  "object": "list",
  "data": [
    {"id": "sonnet-4.5", "object": "model", "owned_by": "sigilbridge"},
    {"id": "haiku-4.5",  "object": "model", "owned_by": "sigilbridge"},
    {"id": "gpt-5",      "object": "model", "owned_by": "sigilbridge"}
  ]
}
```

### 5.2 Anthropic-native

#### 5.2.1 POST `/v1/messages`

Full Anthropic Messages schema:

```json
{
  "model": "sonnet-4.5",
  "messages": [
    {"role": "user", "content": "Hello"}
  ],
  "system": "You are helpful.",
  "max_tokens": 4096,
  "temperature": 0.7,
  "stream": true,
  "tools": [...],
  "mcp_servers": [
    {"type": "url", "url": "https://mcp.example.com/sse", "name": "example"}
  ],
  "metadata": {"user_id": "user-123"}
}
```

**Streaming**: Anthropic native event stream (`message_start`, `content_block_start`, `content_block_delta`, `content_block_stop`, `message_delta`, `message_stop`).

#### 5.2.2 POST `/v1/messages/count_tokens`

Returns input token count for a request body without dispatching to upstream. Bridge dispatches to upstream's count_tokens endpoint if available, else local estimation.

### 5.3 Admin API

Mounted under `/admin/v1/`. Authenticated via `Authorization: Bearer <admin_token>` (separate token list from bridge keys).

| Method | Path | Purpose |
|---|---|---|
| GET | `/admin/v1/keys` | List bridge keys (no plaintext) |
| POST | `/admin/v1/keys` | Create bridge key (returns plaintext **once**) |
| GET | `/admin/v1/keys/{id}` | Get key metadata |
| PATCH | `/admin/v1/keys/{id}` | Update budgets, scopes |
| DELETE | `/admin/v1/keys/{id}` | Revoke key |
| GET | `/admin/v1/pools` | List pools |
| POST | `/admin/v1/pools` | Create or update pool |
| DELETE | `/admin/v1/pools/{name}` | Delete pool |
| POST | `/admin/v1/pools/{name}/probe` | Force health probe |
| GET | `/admin/v1/sessions` | List sessions (no plaintext) |
| POST | `/admin/v1/sessions` | Add session (browser launch) |
| DELETE | `/admin/v1/sessions/{id}` | Revoke session |
| POST | `/admin/v1/sessions/{id}/refresh` | Force refresh |
| GET | `/admin/v1/audit?from=&to=&key_id=` | Query audit log |
| GET | `/admin/v1/budgets/{key_id}` | Current budget state |
| GET | `/admin/v1/usage?period=daily` | Aggregated usage stats |
| GET | `/admin/v1/health` | Detailed health (per-upstream) |
| POST | `/admin/v1/reload` | Reload config from disk |

### 5.4 Embedded Web UI

A single-page React application served from `/admin/ui` via `embed.FS`. Talks exclusively to the Admin API (§5.3). Ships inside the SigilBridge binary — no separate frontend deploy, no Node.js needed at runtime.

#### 5.4.1 Stack

| Layer | Choice | Notes |
|---|---|---|
| Framework | **React 19** (latest) | Functional components, hooks, `use` for promise unwrap |
| Language | **TypeScript 5.x** | `strict: true`, `noUncheckedIndexedAccess: true` |
| Build | **Vite 6+** | ESM-first, rolldown-vite when stable |
| Package manager | **pnpm** | Workspace-friendly, lockfile-stable |
| Styling | **Tailwind CSS v4** | `@theme inline` directive, no `tailwind.config.js` |
| Components | **shadcn/ui** (latest) | Copy-in components, fully customizable |
| Icons | **Lucide React** | Tree-shakable, ~24px stroke icons |
| Routing | **React Router v7** | SPA mode (no SSR), data-router |
| Data | **TanStack Query v5** | Cache, retries, optimistic updates |
| Tables | **TanStack Table v8** | Headless, virtualized via `@tanstack/react-virtual` |
| State (UI-only) | **Zustand** | Theme, sidebar collapsed, toasts queue |
| Forms | **React Hook Form + Zod** | Schema-driven validation matching backend |
| Toasts | **Sonner** | shadcn/ui-recommended, accessible |
| Charts | **Recharts** | Lightweight, sufficient for budget + latency dashboards |
| Drag-drop | **@dnd-kit/core** | Pool editor (priority + weight ordering) |
| Code editor | **@monaco-editor/react** | YAML editing for `pools.yaml`, `oauth_providers.yaml` |
| Date utils | **date-fns** | Tree-shakable, no moment.js |
| Theming | **next-themes** (port for non-Next) or custom provider | `dark` / `light` / `system` |
| i18n | **i18next + react-i18next** | Locales: `en` (default), `tr` |
| Linting | **ESLint flat config + typescript-eslint** | + `eslint-plugin-react-hooks`, `eslint-plugin-jsx-a11y` |
| Formatting | **Prettier** | Tailwind class sorter plugin |
| Testing | **Vitest + React Testing Library + Playwright** | Unit + component + E2E |

#### 5.4.2 Design system

**Color tokens** (Tailwind v4 `@theme inline` block in `app.css`, defined per mode):

```css
@theme inline {
  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --color-primary: var(--primary);          /* deep indigo, brand */
  --color-accent: var(--accent);            /* ember orange, attention */
  --color-muted: var(--muted);
  --color-success: var(--success);
  --color-warning: var(--warning);
  --color-danger: var(--danger);
  --color-border: var(--border);
  --color-card: var(--card);
  --radius-sm: 0.375rem;
  --radius-md: 0.5rem;
  --radius-lg: 0.75rem;
  --font-sans: "Inter", "Geist", system-ui, sans-serif;
  --font-mono: "JetBrains Mono", "ui-monospace", monospace;
}

:root              { /* light mode CSS variables */ }
.dark              { /* dark mode CSS variables */ }
@media (prefers-reduced-motion: reduce) { * { transition: none !important; } }
```

Concrete values live in `BRANDING.md`. Default palette: deep indigo primary (`#312E81`), ember orange accent (`#F59E0B`), neutral slate for surfaces.

**Typography scale**: 12 / 14 / 16 / 18 / 20 / 24 / 30 / 36 px. Two weights (400 regular, 500 medium). No bold below headings.

**Motion**: 150ms ease-out for most interactive transitions, 250ms for layout shifts. `prefers-reduced-motion` disables non-essential animation globally.

**Shape**: 6px radius default, 8px on cards, 10px on dialogs. No fully-rounded "pill" elements except status badges and toggle switches.

#### 5.4.3 Theme: dark / light / system

Three modes, persisted in `localStorage`:
- `dark` — explicit dark
- `light` — explicit light
- `system` (default) — follows `prefers-color-scheme`

Theme switcher in the header (Sun / Moon / Monitor lucide icons). Theme set as a `class="dark"` on `<html>` to drive CSS variables; no FOUC because the theme script runs in `<head>` before paint.

#### 5.4.4 Authentication

- **Login screen** (`/login`) — single input: admin token (paste or type, masked).
- `POST /admin/v1/auth/login` → returns httpOnly Secure cookie containing a 15-minute JWT (HS256, signed with admin secret derived from master key).
- All admin API calls send the cookie automatically (same-origin).
- Auto-refresh: TanStack Query refetches on `401` after silent re-auth attempt; if re-auth fails, redirect to `/login`.
- **Logout**: clears cookie + Zustand state + TanStack Query cache.

#### 5.4.5 Routes

```
/login                         Login screen
/                              Dashboard (overview cards + sparklines)
/keys                          Bridge keys list
/keys/new                      Create bridge key (one-time secret reveal)
/keys/:id                      Detail view: budget, scopes, recent requests
/pools                         Pool list with health snapshot
/pools/:name                   Visual pool editor (drag-drop priority, weight sliders)
/credentials                   Combined view: OAuth tokens, sessions, CLI agents
/credentials/oauth/new         OAuth bootstrap (browser launch / device code)
/credentials/sessions/new      Session bootstrap (chromedp launch)
/credentials/cli               CLI agent status (process list, restart, logs)
/audit                         Audit query (filterable, exportable to CSV)
/budgets                       Budget dashboard (charts, top spenders)
/health                        Per-upstream health: state, latency p50/p95/p99
/events                        Admin event log (admin actions + system events)
/settings                      Server settings, hot reload, version info
/settings/oauth-providers      Monaco editor for oauth_providers.yaml override
/settings/pools-raw            Monaco editor for pools.yaml
```

Layout: persistent left sidebar (collapsible), top bar with theme switcher / language switcher / user menu. Mobile: sidebar collapses to a hamburger drawer.

#### 5.4.6 Real-time updates

- **SSE endpoint**: `GET /admin/v1/events/stream` — pushes structured events: `request_completed`, `health_changed`, `oauth_refreshed`, `oauth_refresh_failed`, `cli_spawned`, `cli_crashed`, `key_created`, `pool_reloaded`, `audit_appended`.
- Subscribed once at app mount, multiplexed to consumers via Zustand event bus.
- TanStack Query caches updated via `queryClient.setQueryData` so subscribed views refresh instantly.
- "Live" toggle on dashboards switches between SSE-driven updates and 5-second polling.

#### 5.4.7 Forms & validation

- All forms use React Hook Form with Zod schemas.
- Schemas live in `src/schemas/` and **mirror the backend's validation rules** — single source of truth via shared TypeScript types generated from Go structs (build step: `go run ./cmd/gentypes` emits `src/types/api.ts`).
- Backend errors map to field-level errors via `setError`.
- Submission states: `idle | submitting | success | error` — buttons show spinner, success toast on completion, error toast with detail on failure.

#### 5.4.8 Accessibility (WCAG 2.1 AA)

- All interactive elements reachable via keyboard; visible focus ring (`focus-visible`).
- ARIA labels on icon-only buttons, ARIA-live regions for toasts and async status.
- Color contrast ≥ 4.5:1 for body text, ≥ 3:1 for large text and UI components.
- `prefers-reduced-motion` honored (motion section above).
- Skip link to main content.
- Form errors announced; error messages `aria-describedby` connected to inputs.
- Lighthouse Accessibility score target: ≥ 95 on every route.
- `eslint-plugin-jsx-a11y` enforced in CI.

#### 5.4.9 Internationalization

- **Locales**: `en` (default), `tr`.
- All user-visible strings in `locales/<lang>/<namespace>.json`; no inline strings.
- Namespaces: `common`, `keys`, `pools`, `credentials`, `audit`, `budgets`, `health`, `settings`.
- Date / number / relative-time formatting via the `Intl` API (no extra libraries).
- Language switcher in the header, persisted in `localStorage`.
- RTL layout supported architecturally (logical-property CSS), but not shipped with an RTL locale in v1.0.
- Adding a new locale: drop a translated JSON folder, add to language list — no rebuild of the binary required if loaded via fetch (tradeoff: in-binary embedding means rebuild; v1.0 ships `en` + `tr` embedded).

#### 5.4.10 Responsive design

Mobile-first, four breakpoints (Tailwind defaults):

| Breakpoint | Width | Layout |
|---|---|---|
| Mobile | < 640px | Hamburger drawer nav, single-column cards, horizontal-scroll tables |
| `sm` | ≥ 640px | Two-column cards, dropdown filters |
| `md` | ≥ 768px | Collapsible sidebar, side-by-side editor + preview |
| `lg` | ≥ 1024px | Persistent sidebar, full-width tables |
| `xl` | ≥ 1280px | Multi-column dashboard layouts, sticky filter rails |

All routes tested at 360px, 768px, 1280px, and 1920px widths.

#### 5.4.11 Performance targets

- **Initial bundle**: ≤ 200 KB gzipped (excluding lazy-loaded routes).
- **Time to Interactive** (mid-range laptop, no cache): ≤ 2 s.
- **Lighthouse Performance** score: ≥ 90 on the dashboard.
- **Code splitting**: route-based via React Router lazy loading; heavy components (Monaco editor, Recharts) split out separately.
- **Asset compression**: gzip + brotli pre-compressed, served by Go handler with `Accept-Encoding` negotiation.
- **Cache-busting**: filename-hashed assets (`app-[hash].js`); `index.html` served with `Cache-Control: no-cache`.

#### 5.4.12 Empty / loading / error states

Every list view ships with three states:
- **Loading** — Skeleton placeholders matching the final layout shape.
- **Empty** — Illustrated empty state with primary CTA (e.g., "Create your first bridge key").
- **Error** — Error icon + message + "Retry" button + link to `/events` for context.

Error boundaries at the route level prevent a single failure from blanking the whole UI.

#### 5.4.13 Testing

| Layer | Tool | Coverage target |
|---|---|---|
| Unit | Vitest | ≥ 80% on `src/lib/`, `src/schemas/` |
| Component | React Testing Library + Vitest | All shadcn-derived components and custom widgets |
| E2E | Playwright | Critical paths: login, create key, edit pool, OAuth bootstrap dry-run, audit query |

CI runs all three on every PR; coverage report posted as PR comment.

#### 5.4.14 Build & embed pipeline

```
ui/                                  # Frontend source
├── package.json                     # pnpm
├── vite.config.ts
├── tsconfig.json                    # strict
├── tailwind.config (none — v4 uses @theme inline)
├── eslint.config.js                 # flat config
├── src/
│   ├── main.tsx
│   ├── App.tsx
│   ├── routes/                      # one file per route
│   ├── components/
│   │   ├── ui/                      # shadcn/ui primitives
│   │   └── ...                      # custom components
│   ├── lib/                         # API client, formatters, utils
│   ├── schemas/                     # Zod schemas
│   ├── types/api.ts                 # generated from Go structs
│   ├── locales/{en,tr}/*.json
│   └── styles/app.css               # @theme block + base layer
└── dist/                            # output, embedded by Go

internal/admin/ui/
├── embed.go                         # //go:embed dist/*  (relative to ui/dist after build)
└── handler.go                       # Go HTTP handler serving embedded FS
```

Build chain:
1. `pnpm install --frozen-lockfile`
2. `go run ./cmd/gentypes` — emits `ui/src/types/api.ts` from Go structs (admin API DTOs).
3. `pnpm --filter ui run build` — Vite bundle to `ui/dist/`.
4. `go build` — Go binary embeds `ui/dist/*` via `//go:embed`.

Local development: `pnpm --filter ui run dev` runs Vite at `:5173` with proxy to bridge admin API at `:8788`.

Production binary: serves `/admin/ui/*` from the embedded FS with a fallback to `index.html` for client-side routing. Pre-compressed `.gz` and `.br` assets included; Go handler picks the right one based on `Accept-Encoding`.

#### 5.4.15 Visual identity

Theme: deep indigo (primary) + ember orange (accent). Full palette, typography choice, logo specification, and voice guidelines live in **BRANDING.md** (next deliverable after IMPLEMENTATION.md and TASKS.md).

---

## 6. Provider Specifications

### 6.1 anthropic_api

| Field | Value |
|---|---|
| Endpoint | `https://api.anthropic.com/v1/messages` |
| Auth | `x-api-key` header |
| Native format | Anthropic — passthrough on Anthropic ingress |
| Streaming | Native event stream |
| Tools | Native |
| MCP | Native passthrough |
| Vision | Native (image content blocks) |
| Caching | Native `cache_control: ephemeral` |
| Health probe | `POST /v1/messages` with 1-token request to `claude-haiku-4-5` |
| Stability | stable |

### 6.2 openai_api

| Field | Value |
|---|---|
| Endpoint | `https://api.openai.com/v1/chat/completions` |
| Auth | `Authorization: Bearer` |
| Native format | OpenAI — passthrough on OAI ingress |
| Streaming | Native SSE |
| Tools | Native (function calling) |
| MCP | Not supported (translated to function calls if model supports) |
| Vision | Native (image_url content) |
| Caching | Implicit (server-side) |
| Health probe | `POST /v1/chat/completions` 1-token to `gpt-5-nano` |
| Stability | stable |

### 6.3 claude_web (subscription bridge)

**This is the most operationally complex adapter.** It operates against Anthropic's consumer ToS and may break or get the operator's account banned at any time. Read carefully.

#### 6.3.1 Approach

Two-phase architecture:

- **Bootstrap (interactive, CLI subcommand)**: chromedp-driven real Chrome session for first login. User logs in via Anthropic's normal sign-in flow (email magic link, Google, MFA — whatever they normally use). On successful login, the adapter scrapes:
  - `sessionKey` cookie value
  - User's primary `organization_uuid` from `/api/organizations`
  - Browser's User-Agent
  - Synthesized JA3 fingerprint matching the Chrome version
  - Full cookie jar
- **Hot path (programmatic)**: Direct HTTPS calls using `utls` to spoof a Chrome JA3/JA4 fingerprint. Endpoints reverse-engineered from the web app:
  - `POST /api/organizations/{uuid}/chat_conversations` — create conversation
  - `POST /api/organizations/{uuid}/chat_conversations/{conv_uuid}/completion` — send message, receive SSE stream
  - `POST /api/organizations/{uuid}/chat_conversations/{conv_uuid}/title` — title conversation (cosmetic)
  - `DELETE /api/organizations/{uuid}/chat_conversations/{conv_uuid}` — cleanup

#### 6.3.2 Request mapping

IR → claude_web translation:

- Each bridge call creates a fresh conversation (no chat history reuse — keeps semantics aligned with stateless API).
- IR `system` field → web's system context (web schema differs slightly from API).
- IR `messages` → flattened single user message (concat with role separators) OR alternating user/assistant if `claude_web.conversation_reuse: true`.
- IR `tools` → not natively supported by claude_web. Tool-using requests fail with `not_supported_by_subscription` unless `claude_web.tool_emulation: prompt_inject` is set, which injects tool schemas into the prompt and parses model output for tool_use blocks (best-effort, lossy).
- IR `mcp_servers` → not supported. Requests with MCP servers fail with `not_supported_by_subscription`.
- IR `max_tokens`, `temperature`, etc. → mapped where the web endpoint accepts them; otherwise dropped silently with a warning logged.

#### 6.3.3 Streaming

claude_web uses SSE natively. Bridge consumes the stream, parses Anthropic-style events, and re-emits in IR event format. Latency parity with API is acceptable but typically 50–200ms slower TTFB.

#### 6.3.4 Error handling

| Upstream behavior | Adapter response |
|---|---|
| 401 / sessionKey rejected | Mark session expired, emit `session_expired` event, return retryable error |
| 403 / org access denied | Permanent error, escalate to fallback |
| 429 / rate limited | Retryable with exponential backoff |
| 5xx | Retryable |
| Cloudflare challenge (403 with HTML body) | **Critical**: log full response, mark upstream cooldown, alert admin (likely fingerprint detection) |
| Connection reset / timeout | Retryable, increment failure count |

#### 6.3.5 Anti-detection measures

- `utls` Chrome 131+ ClientHello fingerprint (configurable per session).
- HTTP/2 frame ordering matching real Chrome.
- Realistic User-Agent matching the chromedp version captured at bootstrap.
- Request pacing: minimum 1 second between requests per session (configurable).
- No parallel requests on the same session (request queue per session).
- `Accept-Language`, `Sec-Ch-Ua-*` headers matching Chrome.
- Optional session warming: interleave benign API calls (e.g., `/api/organizations`) every N hot requests.

#### 6.3.6 Risk acknowledgment

This adapter operates against Anthropic's Terms of Service for consumer subscriptions. Operators accept that:

- Their account may be banned at any time, with no recourse.
- Endpoints may change without notice, breaking the adapter.
- Anthropic may add bot detection that we cannot circumvent.

The adapter is **disabled by default**. Operator must set:

```yaml
subscription_adapters:
  enabled: true
  acknowledge_tos_risk: true
```

in config to allow startup. Bridge logs a warning at every startup with a TOS reminder.

### 6.4 chatgpt_web (subscription bridge)

Architecture mirrors claude_web. Differences:

- Uses `__Secure-next-auth.session-token` cookie + access token from `/api/auth/session`.
- Cloudflare/Arkose Labs challenges are common during bootstrap; may require manual challenge solve. Adapter cannot solve captchas; on hot-path challenge, marks upstream sick.
- Endpoints: `/backend-api/conversation` (SSE chat), `/api/auth/session` (token refresh).
- More restrictive than claude_web; longer rate limits, more frequent fingerprint flagging.
- Stability: `experimental` — operationally less reliable than claude_web.

### 6.5 Pluggable provider interface

SigilBridge ships with a plugin SDK in v1.0. Plugins are out-of-tree provider adapters that implement the same `Provider` interface (§4.4.1) but run as **separate subprocesses**, communicating with the bridge over gRPC via `hashicorp/go-plugin`. This isolates third-party adapter code from the bridge process and allows independent versioning.

**Plugin lifecycle**:
1. At startup, bridge scans `~/.sigilbridge/plugins/` for executables with a `plugin.yaml` manifest.
2. For each plugin, bridge spawns the executable with handshake env vars; plugin starts a gRPC server on a Unix socket and reports back.
3. Bridge introspects plugin capabilities, registers the provider ID in the adapter table.
4. On request, bridge serializes IR → protobuf → plugin; plugin deserializes, calls upstream, streams response back.
5. On plugin crash, bridge marks all upstreams using that plugin sick, restarts the plugin (exponential backoff up to 5 minutes), restores once healthy.

**Manifest example** (`plugin.yaml`):
```yaml
name: my-custom-provider
version: 1.2.0
provider_id: my_custom
executable: ./my-custom-provider
handshake:
  protocol_version: 1
capabilities:
  streaming: true
  tool_use: false
  vision: false
config_schema_json: |
  {
    "type": "object",
    "properties": {
      "endpoint": {"type": "string"},
      "api_key": {"type": "string"}
    },
    "required": ["endpoint", "api_key"]
  }
```

Reference plugin: `provider-example/` in the SigilBridge repo — a minimal adapter showing the wire protocol, manifest format, and gRPC stubs. Plugin protocol details: ADR-0007.

---

## 7. Routing & Fallback Specification

### 7.1 Selection state machine

```
              ┌──────────────────────────┐
              │   START: request in      │
              └────────────┬─────────────┘
                           │
                           ▼
              ┌──────────────────────────┐
              │ Resolve pool from alias  │── pool_not_found ──→ 404
              └────────────┬─────────────┘
                           │
                           ▼
              ┌──────────────────────────┐
              │ Filter healthy upstreams │── all_unhealthy ──→ 503
              └────────────┬─────────────┘
                           │
                           ▼
              ┌──────────────────────────┐
              │ Apply strategy           │
              └────────────┬─────────────┘
                           │ selected upstream
                           ▼
              ┌──────────────────────────┐
              │ Pre-check budget         │── budget_exceeded ──→ 402 (if hard_cap)
              └────────────┬─────────────┘
                           │
                           ▼
              ┌──────────────────────────┐
              │ Dispatch to adapter      │
              └────────────┬─────────────┘
                           │
                       ┌───┴───┐
                       │       │
                   success   failure
                       │       │
                       ▼       ▼
                    ┌───────────────────────┐
                    │ Classify error        │
                    └────────┬──────────────┘
                             │
                  ┌──────────┼──────────┐
                  │          │          │
              permanent  retryable  retries_exhausted
                  │          │          │
                  ▼          ▼          ▼
                RETURN    Mark         RETURN
                error     unhealthy,   error
                          GOTO Filter
```

### 7.2 Error classification

| HTTP / signal | Class | Retryable | Marks upstream sick |
|---|---|---|---|
| 200 | success | n/a | no |
| 400, 422 | client_error | no | no |
| 401, 403 | auth_error | no | yes (likely bad credential) |
| 404 (model not found) | config_error | no | no |
| 408 | timeout | yes | no |
| 429 | rate_limited | yes | yes (cooldown) |
| 500, 502, 503, 504 | server_error | yes | yes |
| Network timeout | timeout | yes | yes |
| Connection reset | network | yes | yes |
| Context canceled | client_canceled | no | no |
| Cloudflare/Akamai challenge HTML | bot_detected | no | yes (alert admin) |

### 7.3 Cooldown timing (default exponential backoff)

```
attempt 1: 5s
attempt 2: 10s
attempt 3: 20s
attempt 4: 40s
attempt 5: 80s
attempt 6: 160s
attempt 7+: 300s (max)
```

Reset to attempt-1 base on first success after recovery.

---

## 8. Security

### 8.1 Threat model

| Threat | Mitigation |
|---|---|
| Compromised bridge key | Per-key budget caps + rate limits + revocation; audit log surfaces unusual usage |
| Compromised admin token | Separate token storage, rotation, audit log of admin actions, IP allowlist support |
| Compromised session vault key | Env-only key (never on disk), AES-256-GCM, rotation via re-encrypt-all admin op |
| Database file theft | Bridge keys hashed (not recoverable); session ciphertexts unreadable without master key |
| Memory dump | `mlock` on master key + best-effort zeroing on shutdown (Go GC limitations apply) |
| Supply chain | Minimal dependency surface (see §2.2); each dep audited per release |
| Subscription session theft | Each session ciphertext independent; one compromise doesn't expose others |

### 8.2 Hardening checklist (operator)

- [ ] Run as unprivileged user
- [ ] `SIGILBRIDGE_MASTER_KEY` only in environment, never in config files or repos
- [ ] Bind admin API to localhost or trusted network only
- [ ] Front with reverse proxy + WAF for any public deployment
- [ ] Rotate bridge keys on personnel changes
- [ ] Set per-key daily/monthly budgets (defense in depth)
- [ ] Audit log retention ≥ compliance requirement
- [ ] Restrict file permissions on `data/` and `audit/` (0700)
- [ ] Use systemd `ProtectSystem=strict` and `PrivateTmp=true`

### 8.3 Cryptography

- TLS: Go stdlib `crypto/tls`, modern cipher suites only (TLS 1.3 preferred).
- At-rest: AES-256-GCM.
- Hashing: SHA-256.
- Random: `crypto/rand` only.
- No custom crypto, no rolling-your-own.

### 8.4 Subscription bridge legal note

Operators are responsible for compliance with provider Terms of Service. SigilBridge is a tool; usage decisions and consequences are the operator's. README and docs include explicit disclaimers; bridge logs a warning at every startup if subscription adapters are enabled.

---

## 9. Configuration

### 9.1 File layout

```
~/.sigilbridge/
├── config.yaml          # main config
├── pools.yaml           # pool definitions (can be inline in config.yaml)
├── admin_tokens.yaml    # admin tokens
├── pricing.yaml         # cost table (auto-distributed with binary)
├── data/                # KV store files (BadgerDB or CobaltDB)
├── audit/               # JSONL audit logs
└── tls/                 # optional TLS certs
```

### 9.2 Main config (`config.yaml`)

```yaml
server:
  bind: "0.0.0.0:8787"
  tls:
    enabled: false
    cert_file: ""
    key_file: ""
  max_concurrent_requests: 1024
  request_timeout_seconds: 600
  idle_timeout_seconds: 120
  shutdown_grace_seconds: 30

admin:
  bind: "127.0.0.1:8788"
  tokens_file: "admin_tokens.yaml"
  ui_enabled: true

storage:
  path: "data/sigilbridge.db"     # SQLite file
  busy_timeout_ms: 5000
  cache_size_kb: 20000            # 20MB
  mmap_size_mb: 256
  backup:
    enabled: true
    interval_hours: 24
    retention_days: 14
    path: "backup/"

audit:
  enabled: true
  path: "audit/"
  content_mode: none       # none|hash|truncated|full
  retention_days: 90
  rotate_compress_after_days: 7

vault:
  master_key_env: SIGILBRIDGE_MASTER_KEY

oauth:
  refresh_check_interval_seconds: 300    # 5 minutes
  refresh_lead_time_seconds: 300         # refresh tokens expiring within 5 min
  bootstrap_listener_addr: "127.0.0.1:0" # 0 = OS-assigned random port
  providers_file: oauth_providers.yaml

cli_agents:
  enabled: true
  default_idle_timeout_seconds: 600
  default_stderr_capture_bytes: 4096
  health_check_interval_seconds: 60
  spawn_log_level: warn

subscription_adapters:
  enabled: false
  acknowledge_tos_risk: false
  refresh_interval_seconds: 21600        # 6 hours (cookie/session bridges only)

logging:
  level: info              # debug|info|warn|error
  format: json             # json|text
  file: ""                 # empty = stderr

metrics:
  prometheus_enabled: true
  bind: ""                 # empty = mounted on main server at /metrics

pools_file: pools.yaml
```

### 9.3 Hot reload

`POST /admin/v1/reload` re-reads config, pools, admin_tokens, pricing without dropping in-flight requests. Not all fields are hot-reloadable — `server.bind`, `storage.path`, `vault.master_key_env` require restart. Bridge returns 409 with a list of fields requiring restart.

---

## 10. Deployment

### 10.1 Single binary

```
sigilbridge serve --config /etc/sigilbridge/config.yaml
```

Default config search path: `./config.yaml` → `~/.sigilbridge/config.yaml` → `/etc/sigilbridge/config.yaml`.

### 10.2 systemd unit (provided)

```ini
[Unit]
Description=SigilBridge AI Gateway
After=network.target

[Service]
Type=simple
User=sigilbridge
EnvironmentFile=/etc/sigilbridge/sigilbridge.env
ExecStart=/usr/local/bin/sigilbridge serve --config /etc/sigilbridge/config.yaml
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
LimitNOFILE=65535
ProtectSystem=strict
ReadWritePaths=/var/lib/sigilbridge
PrivateTmp=true
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
```

`/etc/sigilbridge/sigilbridge.env`:

```
SIGILBRIDGE_MASTER_KEY=base64-encoded-32-bytes
ANTHROPIC_KEY_A=sk-ant-...
ANTHROPIC_KEY_B=sk-ant-...
```

### 10.3 Docker (optional)

Multi-arch image: `ghcr.io/sigilbridge/sigilbridge:VERSION`. Distroless base. ~25MB compressed.

```
docker run -d \
  -p 8787:8787 \
  -v /etc/sigilbridge:/config:ro \
  -v sigilbridge-data:/data \
  -e SIGILBRIDGE_MASTER_KEY=... \
  ghcr.io/sigilbridge/sigilbridge:0.1.0 \
  serve --config /config/config.yaml
```

### 10.4 Platforms

| OS | Architecture | Tier |
|---|---|---|
| Linux | amd64 | 1 (CI tested, primary) |
| Linux | arm64 | 1 |
| macOS | amd64 | 1 |
| macOS | arm64 | 1 |
| Windows | amd64 | 2 (built, smoke tested) |
| FreeBSD | amd64 | 3 (community) |

---

## 11. Performance Targets

### 11.1 Latency overhead

- Bridge-induced overhead per request (excluding upstream call): **p50 < 2ms, p99 < 10ms**
- Streaming TTFB overhead: **p99 < 5ms** beyond upstream's TTFB

### 11.2 Throughput

- Single instance: **≥ 2000 RPS** for streaming chat on 8-core, 16GB host (limited by upstream, not bridge)
- Concurrent streams: 1024 default cap, configurable

### 11.3 Memory

- Idle: **< 100MB**
- Per in-flight stream: **~200KB**
- 1024 concurrent streams: **~300MB total**

### 11.4 Storage

- Per audit record (JSONL): ~600 bytes uncompressed, ~200 bytes gzipped
- Per audit_index row (SQLite): ~120 bytes
- Per bridge_key record: ~1KB
- 1M requests/day audit: ~600MB/day JSONL + ~120MB/day SQLite index → ~6GB JSONL + ~1.2GB SQLite after 30-day retention with gzip after 7d
- SQLite file growth: linear with audit volume + budget/ratelimit churn; quarterly `VACUUM` recommended (or via `sigilbridge maintenance vacuum`)

---

## 12. v1.0 Release Scope

There are no incremental phases. Everything below ships in the **v1.0** initial release. Post-1.0 versions exist for bug fixes, performance work, security patches, and operator-driven additions — not for finishing core scope. Anything not listed here is either deliberately out of scope (§12.13) or part of a future major version.

### 12.1 Core platform

- Single static Go binary, multi-arch (Linux/macOS/Windows × amd64/arm64; FreeBSD community-built)
- Pure-Go SQLite storage (`modernc.org/sqlite`) — no CGO, no separate database server
- `embed.FS`-bundled SQL migrations applied via `goose` at startup
- systemd unit + Docker image (`ghcr.io/sigilbridge/sigilbridge`)
- Default config produces a runnable instance with the `mock` provider

### 12.2 Ingress

- OpenAI-compatible: `POST /v1/chat/completions`, `GET /v1/models`
- Anthropic-native: `POST /v1/messages`, `POST /v1/messages/count_tokens`
- Full SSE streaming end-to-end on both surfaces
- Health endpoints (`/healthz`, `/readyz`)
- Prometheus metrics (`/metrics`)
- TLS termination (built-in or via reverse proxy)

### 12.3 Provider adapters (all 21 included)

**API-key adapters** (provider-issued secret):

| Adapter | Notes |
|---|---|
| `mock` | Deterministic responses for testing |
| `anthropic_api` | Official Anthropic API |
| `openai_api` | Official OpenAI API |
| `groq` | Fast inference |
| `gemini_api` | Google AI Studio direct |
| `mistral_api` | Mistral cloud |
| `deepseek_api` | DeepSeek cloud |

**Cloud-IAM adapters** (cloud provider's own auth):

| Adapter | Notes |
|---|---|
| `bedrock` | AWS SigV4 |
| `vertex_ai` | Google Service Account |
| `azure_openai` | API key + Azure resource |

**OAuth adapters** (PKCE / Device Grant + refresh token — preferred subscription path):

| Adapter | Notes |
|---|---|
| `claude_oauth` | Claude Max / Pro via OAuth |
| `copilot_oauth` | GitHub Copilot Chat |
| `gemini_oauth` | Google Gemini Advanced |
| `cursor_oauth` | Cursor Pro (experimental) |

**CLI / ACP adapters** (subprocess delegation via Agent Client Protocol):

| Adapter | Notes |
|---|---|
| `claude_code_cli` | Claude Code subprocess; reuses CLI's own auth |
| `codex_cli` | Codex CLI subprocess |
| `gemini_cli` | Gemini CLI subprocess (experimental) |
| `aider_cli` | Aider subprocess (community ACP shim) |

**Local & legacy fallback adapters**:

| Adapter | Notes |
|---|---|
| `ollama` | Local inference; trusted local HTTP |
| `claude_web` | Subscription bridge — disabled by default, requires explicit ToS acknowledgment; **legacy fallback** to `claude_oauth` |
| `chatgpt_web` | Subscription bridge — disabled by default, marked experimental |

Plus the **plugin SDK** (§4.4.4) for out-of-tree adapters via `hashicorp/go-plugin`.

### 12.4 Routing & resilience

- Six routing strategies: `round_robin`, `weighted_round_robin` (default), `least_used`, `priority_first`, `random`, `weighted_random`
- Multi-tier priority fallback with automatic escalation
- Per-upstream circuit breaker (`gobreaker`)
- Exponential cooldown on failure (5s → 300s)
- Configurable retry policy with classified error matrix (§7.2)

### 12.5 Auth & enforcement

- Bridge keys (`sb_live_…`, `sb_test_…`) with SHA-256 storage, LRU cache
- Per-key budgets (daily + monthly, hard or soft cap), pre-flight check + post-flight commit
- Per-key rate limits (RPM + TPM, sliding window with per-minute buckets)
- Per-key scopes: `allowed_pools`, `allowed_models`, `ip_allowlist` (CIDR)
- Admin tokens, separate from bridge keys, with their own audit trail

### 12.6 Subscription, OAuth & CLI integration

Three complementary mechanisms for accessing LLM capacity without traditional API keys:

**OAuth bridges** (preferred — sanctioned by providers):
- Authorization Code + PKCE flow with system browser bootstrap
- Device Authorization Grant flow for headless setups
- Background refresh worker (5-minute lead time before expiry)
- Embedded `oauth_providers.yaml` registry for `claude_oauth`, `copilot_oauth`, `gemini_oauth`, `cursor_oauth`
- Self-hosted overrides supported (custom Cloudflare AI Gateway, enterprise SSO, etc.)
- OAuth tokens encrypted in vault alongside session credentials

**CLI agent spawning (Agent Client Protocol)** — reuse the developer's already-installed CLI:
- Spawn `claude_code_cli`, `codex_cli`, `gemini_cli`, `aider_cli` as managed subprocesses
- ACP / JSON-RPC 2.0-over-stdio protocol (Content-Length-framed)
- Idle subprocess management (default 10-minute idle timeout)
- Authentication fully delegated to the CLI's own credential store — no vault entry, no extra secrets
- Auth status surfaced via per-CLI `auth status` subcommand probes
- Crash recovery + exponential-backoff restart
- Sandboxed working directory per upstream

**Session/cookie bridges (legacy fallback)** — for users without OAuth or CLI access:
- Encrypted session vault (AES-256-GCM, master key from env)
- `chromedp` interactive bootstrap (one-time browser-driven login)
- `utls` Chrome 131+ ClientHello fingerprint spoofing
- Per-session pacing + serialized request queue
- Anti-detection: header parity, request spacing, optional warming
- Disabled by default; explicit ToS acknowledgment required at startup
- **Marked deprecated** for any provider where an OAuth flow exists (e.g., `claude_oauth` supersedes `claude_web` for Claude Max users)

### 12.7 MCP passthrough

- Anthropic-native `mcp_servers` parameter forwarded verbatim to capable adapters (`anthropic_api`)
- Translated-to-tool-calls fallback for OpenAI-family providers where feasible
- Hard rejection (`not_supported_by_subscription`) on subscription bridges and non-capable adapters
- Optional health probe: `POST /admin/v1/mcp/probe` to verify a configured MCP server is reachable

### 12.8 Observability

- Append-only JSONL audit log with daily rotation, gzip after 7 days, prune after `retention_days` (default 90)
- SQLite `audit_index` for fast filtering by key, pool, status, timerange
- Configurable content modes: `none` (default), `hash`, `truncated`, `full`
- Prometheus metrics: counters, histograms, gauges (full list in §4.7.2)
- Structured JSON logs to stderr or file
- Per-request unique ULID, propagated through all logs and audit records

### 12.9 Admin surface

**REST API** (`/admin/v1/...`, full list in §5.3):
- Bridge key CRUD with secret-once flow
- Pool CRUD + force-probe
- OAuth credential CRUD + bootstrap + force-refresh
- Session vault CRUD + bootstrap + force-refresh
- CLI agent process status + restart
- Audit query, budget read, usage aggregation
- Health detail, hot reload, event stream (SSE)

**Embedded React Admin UI** (`/admin/ui`, served from `embed.FS`) — full specification in §5.4. v1.0 deliverables:
- Stack: React 19 + TypeScript 5 strict + Vite + Tailwind CSS v4 + shadcn/ui + Lucide React + TanStack Query/Table + React Router v7 + React Hook Form + Zod
- Theme: dark / light / system, persisted; Tailwind v4 `@theme inline` color tokens
- 16 routes covering bridge keys, pools, OAuth credentials, sessions, CLI agents, audit, budgets, health, events, settings
- Real-time updates via SSE (`/admin/v1/events/stream`)
- WCAG 2.1 AA accessibility (Lighthouse ≥ 95)
- i18n: English + Turkish (`en`, `tr`) embedded; architecture supports adding locales without backend changes
- Responsive: mobile-first, tested at 360 / 768 / 1280 / 1920 px
- Performance: ≤ 200 KB initial gzipped bundle, ≤ 2 s TTI, route-based code splitting
- Built-in Monaco editor for `pools.yaml` and `oauth_providers.yaml` overrides
- Visual pool editor with `dnd-kit` drag-drop priority + weight sliders
- OAuth bootstrap flow inline (browser launch via local listener, or device-code panel for headless)
- CLI agent dashboard: process status, stderr ringbuffer view, restart button
- Audit query: filterable, sortable, exportable to CSV, jump-to-JSONL-line
- Loading / empty / error states for every list view
- Tests: Vitest + React Testing Library + Playwright E2E for critical paths

### 12.10 Operations

- Hot reload via `POST /admin/v1/reload` (config + pools + admin tokens + pricing)
- Backup: nightly `VACUUM INTO`, manual `sigilbridge backup --output path.db`
- Restore: `sigilbridge restore --from path.db`
- SQLite maintenance: `sigilbridge maintenance vacuum` for compaction
- Pricing table updates: `sigilbridge pricing update` pulls from a known release URL (configurable, can be air-gapped to a local file)

### 12.11 Documentation

- README with quickstart, configuration reference, architecture overview
- ADRs 0001–0008 covering key technical decisions
- IMPLEMENTATION.md — package layout, key data structures, sequence diagrams, encryption flow
- TASKS.md — granular work items mapped to file paths and acceptance criteria
- BRANDING.md — visual identity, logo, palette, voice
- Operator runbook in `docs/runbook.md` (deployment, common errors, recovery)
- Plugin development guide in `docs/plugins.md`

### 12.12 Testing & CI

- Unit tests for all internal packages (`internal/...`)
- Integration tests using `mock` adapter end-to-end through ingress
- Subscription adapter tests behind a build tag (`-tags=subscription_e2e`), run only with provided session credentials
- Race detector clean (`go test -race ./...`)
- Multi-arch CI builds on every PR
- Reproducible release artifacts (SHA-256 sums, signed tags)

### 12.13 Explicitly out of scope for v1.0

These are deliberate omissions, not deferred work. PRs adding them to v1.0 scope will be declined.

- Multi-node clustering, Raft consensus, distributed budget counters, S3-shared audit log
- Fine-tuning, batch, or image-generation routing
- Built-in vector store, RAG, embeddings cache
- LLM-as-judge evaluation harness
- Token-level billing dashboards beyond the basic cost summary
- Public SaaS hosted version
- Mobile admin app
- Team / organization multi-tenancy with hierarchical billing (single-tenant only; multi-tenant lives in v2.x if there is demand)

---

## Appendix A — Open Questions

- [ ] Should bridge keys support OAuth-style token refresh, or remain long-lived only?
- [ ] Should the audit log support optional encryption at rest (separate from session vault)?
- [ ] How aggressive should default rate limits be (tradeoff: usability vs. accidental cost spikes)?
- [ ] Should `model_alias` resolution support glob patterns (e.g., `claude-*`) in pool names?
- [ ] MCP server passthrough: pass-through verbatim, or normalize through IR?
- [ ] At what audit volume does SQLite hit its ceiling and warrant a Postgres-backed deployment option? (currently: deferred until someone reports it)
- [ ] Should `claude_oauth` formally deprecate and remove `claude_web` in v2.0, or keep both indefinitely for offline / non-Max users?
- [ ] OAuth `refresh_token` rotation policy: store every rotation in audit log, or just the latest?
- [ ] CLI agent crash policy: should bridge auto-restart on first request, or require admin probe?
- [ ] ACP version compatibility matrix: what happens when an upstream CLI updates its ACP version mid-running? Auto-renegotiate, or hard fail?
- [ ] Should OAuth client_ids be operator-configurable (private apps) or only registry-shipped (public flows)?

## Appendix B — Glossary

| Term | Meaning |
|---|---|
| **Bridge key** | Credential issued by SigilBridge for internal client use (`sb_live_...`) |
| **Pool** | Named collection of upstream candidates for a model alias |
| **Upstream** | A single provider+credential combination |
| **Provider** | The LLM service (Anthropic, OpenAI, Claude.ai web, etc.) |
| **Model alias** | Human-friendly name clients use (e.g., `sonnet-4.5`) |
| **IR** | Internal Representation — canonical request/response model |
| **Subscription bridge** | Adapter using non-API consumer subscription credentials |
| **Vault** | Encrypted at-rest storage for subscription credentials |
| **Cooldown** | Temporary state preventing an upstream from being selected after failures |
| **Circuit breaker** | Mechanism that fully isolates an upstream after persistent failures |
| **TTFB** | Time to first byte (streaming) |

## Appendix C — ADR Index (planned)

- ADR-0001: Choice of Go over Rust/TypeScript
- ADR-0002: SQLite as the storage backend (single file, pure-Go driver, WAL mode)
- ADR-0003: Session vault encryption scheme
- ADR-0004: utls for TLS fingerprint spoofing
- ADR-0005: Dual-native ingress (OAI + Anthropic) instead of unified internal-only schema
- ADR-0006: ULID for all IDs
- ADR-0007: Plugin adapter protocol (`hashicorp/go-plugin` over gRPC)
- ADR-0008: Subscription adapter risk acknowledgment UX
- ADR-0009: OAuth flow design (PKCE for desktop, Device Grant for headless, refresh token policy)
- ADR-0010: Agent Client Protocol integration for local CLI subprocess delegation
- ADR-0011: Adapter taxonomy and authentication preference order (API key → OAuth → CLI → Session)

---

**End of SPECIFICATION v1.0.**

Next document: **IMPLEMENTATION.md** (file/package layout, key data structures, sequence diagrams, encryption flow, plugin protocol details).
