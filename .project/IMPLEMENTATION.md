# SigilBridge — IMPLEMENTATION

| Field | Value |
|---|---|
| **Project** | SigilBridge |
| **Document** | IMPLEMENTATION.md (companion to SPECIFICATION.md) |
| **Version** | 1.0.0 (initial release scope, draft) |
| **Author** | ECOSTACK TECHNOLOGY OÜ |
| **License** | Apache-2.0 |
| **Last Updated** | 2026-05-07 |

This document describes **how** SigilBridge is built. SPECIFICATION.md describes **what** is built and **why**. Read SPECIFICATION first.

---

## 1. Repository layout

```
sigilbridge/
├── cmd/
│   ├── sigilbridge/                  # main binary
│   │   ├── main.go                   # entry point, CLI router (cobra)
│   │   ├── commands/
│   │   │   ├── serve.go              # `sigilbridge serve --config ...`
│   │   │   ├── oauth.go              # `sigilbridge oauth add|list|revoke`
│   │   │   ├── session.go            # `sigilbridge session add|list|revoke`
│   │   │   ├── keys.go               # `sigilbridge keys create|list|revoke`
│   │   │   ├── backup.go             # `sigilbridge backup --output ...`
│   │   │   ├── restore.go            # `sigilbridge restore --from ...`
│   │   │   ├── pricing.go            # `sigilbridge pricing update|show`
│   │   │   ├── maintenance.go        # `sigilbridge maintenance vacuum`
│   │   │   └── version.go
│   │   └── doc.go
│   └── gentypes/                     # Go-to-TypeScript type generator
│       └── main.go
├── internal/
│   ├── ingress/
│   │   ├── server.go                 # http.Server wiring, graceful shutdown
│   │   ├── oai.go                    # POST /v1/chat/completions, GET /v1/models
│   │   ├── anthropic.go              # POST /v1/messages, /count_tokens
│   │   ├── admin.go                  # mounted /admin/v1/*
│   │   ├── ui.go                     # /admin/ui/* served from embed.FS
│   │   ├── stream.go                 # SSE encoder reused by both formats
│   │   ├── middleware.go             # auth + rate-limit + budget + audit-open
│   │   ├── errors.go                 # error → HTTP status mapping
│   │   └── *_test.go
│   ├── ir/
│   │   ├── request.go                # IRRequest, Message, ContentBlock
│   │   ├── response.go               # IRResponse, Usage, Error
│   │   ├── event.go                  # IREvent (streaming)
│   │   ├── tool.go                   # ToolDef, ToolUse, ToolResult
│   │   ├── normalize_oai.go          # OAI request → IRRequest
│   │   ├── normalize_anthropic.go    # Anthropic request → IRRequest
│   │   ├── denormalize_oai.go        # IRResponse/IREvent → OAI format
│   │   ├── denormalize_anthropic.go  # IRResponse/IREvent → Anthropic events
│   │   └── *_test.go
│   ├── router/
│   │   ├── router.go                 # Resolve(alias) → Upstream, Dispatch, retry loop
│   │   ├── pool.go                   # Pool snapshot
│   │   ├── strategy/
│   │   │   ├── strategy.go           # Strategy interface
│   │   │   ├── round_robin.go
│   │   │   ├── weighted_round_robin.go
│   │   │   ├── least_used.go
│   │   │   ├── priority_first.go
│   │   │   ├── random.go
│   │   │   └── weighted_random.go
│   │   ├── health.go                 # state machine
│   │   ├── breaker.go                # gobreaker wrapper
│   │   └── *_test.go
│   ├── adapter/
│   │   ├── adapter.go                # Provider interface (mirrors SPEC §4.4.2)
│   │   ├── registry.go               # ID → Provider map, plugin merge
│   │   ├── errors.go                 # adapter error → ErrorClass mapping
│   │   ├── apikey/
│   │   │   ├── anthropic.go
│   │   │   ├── openai.go
│   │   │   ├── groq.go
│   │   │   ├── gemini.go
│   │   │   ├── mistral.go
│   │   │   └── deepseek.go
│   │   ├── cloudiam/
│   │   │   ├── bedrock.go            # AWS SigV4 inline signer
│   │   │   ├── vertex.go
│   │   │   └── azure.go
│   │   ├── oauth/
│   │   │   ├── adapter.go            # Common OAuth-adapter base
│   │   │   ├── claude.go
│   │   │   ├── copilot.go
│   │   │   ├── gemini.go
│   │   │   └── cursor.go
│   │   ├── cliacp/
│   │   │   ├── adapter.go            # Common CLI/ACP base
│   │   │   ├── claude_code.go
│   │   │   ├── codex.go
│   │   │   ├── gemini_cli.go
│   │   │   └── aider.go
│   │   ├── session/                  # Legacy fallback
│   │   │   ├── claude_web.go
│   │   │   ├── chatgpt_web.go
│   │   │   ├── utls_dial.go          # Custom Dialer with utls Chrome ClientHello
│   │   │   └── chromedp_bootstrap.go
│   │   ├── local/
│   │   │   └── ollama.go
│   │   ├── plugin/
│   │   │   ├── host.go               # plugin discovery + supervisor
│   │   │   ├── grpc.go               # gRPC client wrapping go-plugin
│   │   │   └── manifest.go
│   │   └── mock/
│   │       └── mock.go
│   ├── auth/
│   │   ├── bridgekey.go              # generate, hash, validate, scope check
│   │   ├── admintoken.go             # admin token verify
│   │   ├── jwt.go                    # admin UI session cookie (HS256)
│   │   ├── cache.go                  # LRU for hot bridge_key lookups
│   │   └── *_test.go
│   ├── budget/
│   │   ├── tracker.go                # pre-check + post-commit
│   │   ├── ratelimit.go              # sliding window
│   │   ├── cost.go                   # token → cents
│   │   └── *_test.go
│   ├── audit/
│   │   ├── writer.go                 # JSONL append, daily rotation, gzip
│   │   ├── index.go                  # SQLite audit_index inserts
│   │   ├── content_mode.go           # none | hash | truncated | full
│   │   ├── rotator.go                # background rotation + prune
│   │   └── *_test.go
│   ├── vault/
│   │   ├── vault.go                  # Get/Put/Delete, vault://<provider>/<name>
│   │   ├── crypto.go                 # AES-256-GCM with AAD
│   │   ├── master_key.go             # env source, mlock-on-Linux best effort
│   │   └── *_test.go
│   ├── oauth/
│   │   ├── manager.go                # holds providers + vault + refresh worker
│   │   ├── pkce.go                   # verifier/challenge, browser launch, listener
│   │   ├── device_grant.go
│   │   ├── refresh.go
│   │   ├── exchange.go               # token endpoint POSTs
│   │   ├── registry.go               # oauth_providers.yaml loader + override merge
│   │   ├── bootstrap.go              # backend for `sigilbridge oauth add`
│   │   └── *_test.go
│   ├── cliacp/
│   │   ├── pool.go                   # process pool keyed by upstream_id
│   │   ├── process.go                # exec.Cmd wrap, stdin/stdout pipes, idle timer
│   │   ├── jsonrpc.go                # Content-Length-framed JSON-RPC 2.0 codec
│   │   ├── protocol.go               # ACP message types
│   │   ├── stderr_ring.go            # last 4KB stderr ringbuffer
│   │   └── *_test.go
│   ├── storage/
│   │   ├── db.go                     # OpenDB(path) → *sql.DB with pragmas
│   │   ├── pool.go                   # writer-mutex + reader-pool wrappers
│   │   ├── migrations.go             # goose driver wired to embed.FS
│   │   ├── migrations/
│   │   │   ├── 0001_init.sql
│   │   │   ├── 0002_audit_indexes.sql
│   │   │   ├── 0003_oauth_metadata.sql
│   │   │   └── 0004_cli_health.sql
│   │   ├── repos/
│   │   │   ├── bridge_keys.go
│   │   │   ├── sessions.go
│   │   │   ├── budget.go
│   │   │   ├── ratelimit.go
│   │   │   ├── audit_index.go
│   │   │   ├── cooldowns.go
│   │   │   └── events.go
│   │   ├── backup.go                 # VACUUM INTO logic
│   │   └── *_test.go
│   ├── config/
│   │   ├── config.go                 # main config struct + YAML parser
│   │   ├── pools.go                  # pools.yaml loader + validator
│   │   ├── pricing.go                # pricing.yaml loader (embedded default)
│   │   ├── reload.go                 # hot reload coordinator
│   │   └── *_test.go
│   ├── pricing/
│   │   ├── pricing.go                # default pricing table (embed)
│   │   └── *_test.go
│   ├── observability/
│   │   ├── metrics.go                # Prometheus collectors
│   │   ├── logger.go                 # slog setup
│   │   ├── trace.go                  # request ULID propagation
│   │   └── *_test.go
│   ├── events/
│   │   ├── bus.go                    # publish-subscribe in-memory
│   │   ├── stream.go                 # /admin/v1/events/stream handler
│   │   └── types.go
│   └── admin/
│       ├── server.go                 # mounts /admin/v1/*
│       ├── keys.go                   # bridge key CRUD
│       ├── pools.go                  # pool CRUD + probe
│       ├── credentials.go            # vault entries (oauth + sessions + cli)
│       ├── audit.go
│       ├── budgets.go
│       ├── usage.go
│       ├── health.go
│       ├── reload.go
│       ├── auth.go                   # POST /admin/v1/auth/login
│       ├── ui/
│       │   ├── embed.go              # //go:embed dist/*
│       │   └── handler.go            # SPA fallback + gzip/brotli negotiation
│       └── *_test.go
├── pkg/
│   └── proto/
│       ├── adapter.proto             # plugin gRPC contract
│       ├── adapter.pb.go             # generated
│       └── adapter_grpc.pb.go        # generated
├── ui/                               # React frontend (built into binary)
│   ├── package.json                  # pnpm
│   ├── pnpm-lock.yaml
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── eslint.config.js
│   ├── prettier.config.js
│   ├── playwright.config.ts
│   ├── vitest.config.ts
│   ├── src/
│   │   ├── main.tsx
│   │   ├── App.tsx
│   │   ├── routes/                   # one file per route (SPEC §5.4.5)
│   │   ├── components/
│   │   │   ├── ui/                   # shadcn/ui primitives
│   │   │   ├── layout/               # AppShell, Sidebar, Header, ThemeSwitcher
│   │   │   ├── keys/
│   │   │   ├── pools/                # PoolEditor (dnd-kit)
│   │   │   ├── credentials/
│   │   │   ├── audit/                # AuditTable (TanStack Table virtual)
│   │   │   ├── charts/
│   │   │   └── common/
│   │   ├── lib/
│   │   │   ├── api.ts                # fetch wrapper with auth + retry
│   │   │   ├── sse.ts                # EventSource wrapper
│   │   │   ├── format.ts             # Intl-based formatters
│   │   │   ├── theme.ts
│   │   │   └── i18n.ts
│   │   ├── schemas/                  # Zod schemas
│   │   ├── types/api.ts              # generated by cmd/gentypes — DO NOT edit
│   │   ├── locales/
│   │   │   ├── en/
│   │   │   └── tr/
│   │   └── styles/
│   │       └── app.css               # @theme inline + base + utilities
│   ├── tests/
│   │   ├── unit/                     # Vitest
│   │   ├── component/                # RTL + Vitest
│   │   └── e2e/                      # Playwright
│   └── dist/                         # output, embedded by Go (gitignored)
├── docs/
│   ├── runbook.md
│   ├── plugins.md
│   ├── oauth-setup.md
│   ├── cli-agents.md
│   └── adrs/
│       ├── 0001-go-language-choice.md
│       ├── 0002-sqlite-storage.md
│       ├── 0003-vault-encryption-scheme.md
│       ├── 0004-utls-fingerprint.md
│       ├── 0005-dual-native-ingress.md
│       ├── 0006-ulid-ids.md
│       ├── 0007-plugin-protocol.md
│       ├── 0008-subscription-tos-ack.md
│       ├── 0009-oauth-flow-design.md
│       ├── 0010-acp-cli-integration.md
│       └── 0011-adapter-taxonomy.md
├── examples/
│   ├── pools.yaml
│   ├── oauth_providers.yaml
│   ├── config.yaml
│   └── plugin-example/
│       ├── main.go
│       ├── plugin.yaml
│       └── README.md
├── scripts/
│   ├── build.sh
│   ├── release.sh
│   ├── lint.sh
│   └── dev.sh
├── deployments/
│   ├── systemd/sigilbridge.service
│   └── docker/Dockerfile
├── .github/
│   └── workflows/
│       ├── ci.yml
│       ├── release.yml
│       └── ui-types-check.yml
├── go.mod
├── go.sum
├── Makefile
├── .gitignore
├── .editorconfig
├── .project/
│   ├── SPECIFICATION.md
│   ├── IMPLEMENTATION.md
│   ├── TASKS.md
│   ├── BRANDING.md
│   └── PROMPT.md
├── LICENSE
└── README.md
```

---

## 2. Key data structures

### 2.1 IR (Internal Representation)

```go
package ir

import "time"

type Request struct {
    ID            string
    BridgeKeyID   string
    ReceivedAt    time.Time
    IngressFormat string                  // "openai" | "anthropic"
    ModelAlias    string                  // pool name
    System        string
    Messages      []Message
    Tools         []ToolDef
    MCPServers    []MCPServer
    MaxTokens     int
    Temperature   *float32
    TopP          *float32
    StopSequences []string
    Stream        bool
    Metadata      map[string]string
    Extras        map[string]any          // provider-specific passthrough
}

type Message struct {
    Role    string                        // "system" | "user" | "assistant" | "tool"
    Content []ContentBlock
}

type ContentBlock struct {
    Type       string                     // "text" | "image" | "tool_use" | "tool_result" | "document"
    Text       string                     `json:",omitempty"`
    ImageURL   string                     `json:",omitempty"`
    ImageB64   []byte                     `json:",omitempty"`
    MediaType  string                     `json:",omitempty"`
    ToolUse    *ToolUse                   `json:",omitempty"`
    ToolResult *ToolResult                `json:",omitempty"`
    Document   *Document                  `json:",omitempty"`
}

type Response struct {
    ID               string
    UpstreamProvider string
    UpstreamModel    string
    StopReason       string                // end_turn | max_tokens | stop_sequence | tool_use | error
    Content          []ContentBlock
    Usage            Usage
    LatencyMs        int64
    TTFBMs           int64
    CostCents        int
    Error            *Error
}

type Usage struct {
    InputTokens      int
    OutputTokens     int
    CacheReadTokens  int
    CacheWriteTokens int
}

type Event struct {
    Type       string                     // start | content_block_start | content_block_delta | content_block_stop | message_delta | stop | usage | error
    Index      int
    Delta      *ContentBlock              `json:",omitempty"`
    StopReason string                     `json:",omitempty"`
    Usage      *Usage                     `json:",omitempty"`
    Error      *Error                     `json:",omitempty"`
}

type Error struct {
    Type       string                     // budget_exceeded | rate_limited | upstream_error | bot_detected | …
    Message    string
    Retryable  bool
    UpstreamID string                     `json:",omitempty"`
    Class      ErrorClass
}
```

### 2.2 Provider interface

```go
package adapter

type Provider interface {
    ID() string
    Chat(ctx context.Context, req ir.Request, cfg ProviderConfig) (ir.Response, error)
    Stream(ctx context.Context, req ir.Request, cfg ProviderConfig) (<-chan ir.Event, error)
    CountTokens(ctx context.Context, req ir.Request, cfg ProviderConfig) (int, error)
    HealthCheck(ctx context.Context, cfg ProviderConfig) error
    Capabilities() Capabilities
}

type Capabilities struct {
    Streaming        bool
    ToolUse          bool
    Vision           bool
    PromptCaching    bool
    MCPServers       bool
    DocumentInput    bool
    MaxContextTokens int
    StabilityClass   string                // "stable" | "experimental" | "risky"
    Category         string                // "api_key" | "cloud_iam" | "oauth" | "cli_acp" | "session" | "local" | "plugin" | "mock"
}

type ProviderConfig struct {
    UpstreamID string
    Raw        map[string]any
    Vault      vault.Reader                // for adapters needing vault
    OAuth      *oauth.TokenAccessor        // for OAuth adapters; nil otherwise
    CLI        *cliacp.ProcessHandle       // for CLI adapters; nil otherwise
}
```

### 2.3 Routing types

```go
package router

type Pool struct {
    Name           string
    Description    string
    Strategy       string
    Upstreams      []*Upstream
    Cooldown       CooldownConfig
    CircuitBreaker CircuitBreakerConfig
    Retry          RetryConfig
}

type Upstream struct {
    ID       string
    Provider string                       // adapter ID
    Config   map[string]any
    Priority int
    Weight   int
    Health   *Health                      // updated atomically via mutex
}

type Health struct {
    mu                  sync.RWMutex
    State               State              // healthy | degraded | cooldown | circuit_open
    InFlight            int
    ConsecutiveFailures int
    LastError           error
    LastErrorAt         time.Time
    LastSuccessAt       time.Time
    CooldownUntil       time.Time
    CircuitOpenUntil    time.Time
    Breaker             *gobreaker.CircuitBreaker
}

type Selection struct {
    Pool     *Pool
    Upstream *Upstream
    Provider adapter.Provider
    Reason   string                       // "primary" | "fallback_priority_2" | "retry_after_429" | …
}
```

---

## 3. Sequence diagrams

### 3.1 Inbound chat completion (non-streaming)

```
Client          Ingress      Middleware       IR        Router       Adapter      Upstream
  |                |              |             |          |             |             |
  |- POST /v1/* -->|              |             |          |             |             |
  |                |- AuthKey --->|             |          |             |             |
  |                |<-- ok -------|             |          |             |             |
  |                |- RateLim --->|             |          |             |             |
  |                |- BudgetCk -->|             |          |             |             |
  |                |- AuditOpen ->|             |          |             |             |
  |                |- Normalize ----------> IRRequest      |             |             |
  |                |                                       |- Resolve -->|             |
  |                |                                       |- Pick ----->|             |
  |                |                                       |- Dispatch ->|             |
  |                |                                       |             |- Translate->|
  |                |                                       |             |- HTTP ----->|
  |                |                                       |             |<-- 200 -----|
  |                |                                       |<-- IRResp --|             |
  |                |<-- Denormalize ---------- IRResponse  |             |             |
  |<-- 200 OK -----|                                       |             |             |
  |                |- AuditCommit ---------------- (cost, latency)       |             |
  |                |- BudgetCommit -------------- (cents)                |             |
```

### 3.2 Streaming variant

```
Client            Ingress         Adapter        Upstream
  |                  |                |              |
  |- POST stream --->|                |              |
  |                  |- Stream ------>|              |
  |                  |                |- POST ------>|
  |                  |                |<-- 200 SSE --|
  |                  |<==== <-chan ir.Event =========|
  |<-- SSE chunk ----|                |              |
  |  (event 1)       |                |              |
  |<-- SSE chunk ----|                |              |
  |  (event 2)       |                |              |
  |     …            |                |              |
  |<-- SSE done -----|                |              |
  |                  |- AuditCommit (final usage from last IREvent.Usage)|
```

Invariants:
- Adapter owns the upstream HTTP body; closes on success, error, or context cancellation.
- Ingress closes the SSE response when the IR event channel closes.
- Client disconnect → ctx cancel → adapter cancel → upstream body close.

### 3.3 OAuth Authorization Code + PKCE bootstrap

```
Operator      CLI subcmd      OAuthMgr      Browser        Provider IDP
  |               |              |              |                |
  |- oauth add -->|              |              |                |
  |               |- start ----->|              |                |
  |               |              |- gen verifier              ↓ |
  |               |              |- gen challenge             ↓ |
  |               |              |- listen :rand                |
  |               |              |- open browser ->|              |
  |               |              |                 |- /authorize ->|
  |               |              |                 |<-- login UI -|
  |               |              |                 |- user logs in->|
  |               |              |                 |<-- 302 localhost?code=…&state=…
  |               |              |<--- GET /callback?code=…&state=…
  |               |              |- POST /token (code, verifier, redirect) ----->|
  |               |              |<-- access_token, refresh_token, expires_in --|
  |               |              |- Vault.Put(oauth://provider/name)             |
  |               |<-- success --|                                               |
  |<-- "added" ---|              |                                               |
```

### 3.4 OAuth refresh worker (background)

```
Refresh Loop (every 5 min)
  for each oauth://* in vault:
    decrypt
    if expires_at - now < 5min:
      POST <token_url> {grant_type=refresh_token, refresh_token=…}
      on 200:
        re-encrypt (rotate ciphertext + nonce)
        update last_refreshed_at, expires_at
      on 4xx/5xx:
        emit event oauth_refresh_failed
        mark vault entry expired
        SSE → admin UI alert
        Prometheus counter++
```

### 3.5 CLI agent spawn + ACP request

```
Router (selects claude_code_cli upstream)
  ↓
adapter.cliacp.Stream()
  ↓
pool.Get(upstream_id)
  ├── if absent or dead:
  │     spawn(executable, acp_args, env, cwd)
  │     pipe = stdin/stdout
  │     codec = jsonrpc.NewCodec(pipe)
  │     codec.Send("initialize", {clientInfo, capabilities})
  │     wait for {result: {serverInfo, capabilities}}
  │     register in pool, start idle timer
  └── else: reuse, reset idle timer
  ↓
codec.Send("agent.message", {role:"user", content: …})
  ↓
loop on codec.Recv():
  case "agent.message_delta": → emit ir.Event{content_block_delta, …}
  case "agent.message_complete": → emit ir.Event{stop, …}; break
  case "error": → emit ir.Event{error, …}; break
  ↓
close output channel
```

### 3.6 Vault encryption flow

```
Put(id, plaintext, metadata):
  nonce  := crypto/rand 12 bytes
  aad    := id || version_byte
  aead   := aes.NewGCM(masterKey)
  cipher := aead.Seal(nil, nonce, plaintext, aad)
  storage.WriteSession{id, nonce, cipher, metadata}

Get(id) → plaintext:
  row    := storage.ReadSession(id)
  aad    := row.id || version_byte
  aead   := aes.NewGCM(masterKey)
  pt, err := aead.Open(nil, row.nonce, row.cipher, aad)
  return pt, err
```

### 3.7 Plugin gRPC call

```
adapter.plugin.Stream(ctx, req, cfg)
  ↓
find process for cfg.Raw["plugin_id"]; spawn if needed (subprocess + handshake env)
  ↓
grpcClient.Stream(ctx, &pb.StreamRequest{IR: marshal(req), Config: marshal(cfg.Raw)})
  ↓
loop on stream.Recv():
  case *pb.IREvent → emit ir.Event
  case io.EOF      → close output channel
  case error       → emit ir.Event{error}
```

---

## 4. Critical algorithms

### 4.1 Routing strategies (sketches)

```go
// weighted_round_robin (smooth, Nginx-style)
func (s *WRRState) Pick(upstreams []*Upstream) *Upstream {
    var best *Upstream
    var totalWeight int
    for _, u := range upstreams {
        u.currentWeight += u.weight
        totalWeight += u.weight
        if best == nil || u.currentWeight > best.currentWeight {
            best = u
        }
    }
    best.currentWeight -= totalWeight
    return best
}

// least_used
func leastUsed(upstreams []*Upstream) *Upstream {
    sort.Slice(upstreams, func(i, j int) bool {
        return upstreams[i].Health.InFlight < upstreams[j].Health.InFlight
    })
    return upstreams[0]
}

// priority_first: upstreams already filtered to lowest healthy priority tier
func priorityFirst(upstreams []*Upstream) *Upstream { return upstreams[0] }
```

### 4.2 Health state transitions

```go
func (h *Health) RecordFailure(err error, class ErrorClass) {
    h.mu.Lock(); defer h.mu.Unlock()
    h.LastError = err
    h.LastErrorAt = time.Now()
    h.ConsecutiveFailures++

    switch {
    case class == AuthError, class == BotDetected:
        h.State = Cooldown
        h.CooldownUntil = time.Now().Add(h.cooldownDuration())
    case h.ConsecutiveFailures >= 5:
        h.State = Cooldown
        h.CooldownUntil = time.Now().Add(h.cooldownDuration())
    case h.ConsecutiveFailures >= 2 && h.State == Healthy:
        h.State = Degraded
    }

    if h.cooldownCount() >= 3 {
        h.State = CircuitOpen
        h.CircuitOpenUntil = time.Now().Add(h.recoveryTimeout)
    }
}

func (h *Health) cooldownDuration() time.Duration {
    base := 5 * time.Second
    n := h.ConsecutiveFailures
    d := base * time.Duration(1<<min(n, 6))
    if d > 300*time.Second { d = 300 * time.Second }
    return d
}
```

### 4.3 Token estimation

```go
func EstimateInputTokens(req ir.Request, provider, model string) (int, error) {
    switch provider {
    case "openai_api", "azure_openai", "groq", "deepseek_api":
        enc := tiktoken.For(model)            // cl100k_base / o200k_base
        return tiktokenCount(enc, req)
    case "anthropic_api", "claude_oauth", "bedrock":
        if cfg.PreciseCounting {
            return callAnthropicCountTokens(req)
        }
        return approxAnthropic(req), nil
    case "claude_code_cli", "codex_cli", "claude_web", "chatgpt_web":
        return approxGeneric(req), nil
    default:
        return approxGeneric(req), nil
    }
}

func approxGeneric(req ir.Request) int {
    chars := len(req.System)
    for _, m := range req.Messages {
        for _, b := range m.Content {
            chars += len(b.Text)
        }
    }
    return int(float64(chars) / 3.5)
}
```

### 4.4 Cost calculation

```go
type ModelPricing struct {
    InputPerMTok      int64               // cents per 1M tokens
    OutputPerMTok     int64
    CacheReadPerMTok  int64
    CacheWritePerMTok int64
}

func CalculateCost(usage ir.Usage, p ModelPricing) int64 {
    cents := int64(usage.InputTokens)*p.InputPerMTok +
             int64(usage.OutputTokens)*p.OutputPerMTok +
             int64(usage.CacheReadTokens)*p.CacheReadPerMTok +
             int64(usage.CacheWriteTokens)*p.CacheWritePerMTok
    return (cents + 500_000) / 1_000_000   // round half-up
}
```

### 4.5 Rate limit (sliding window)

```go
bucket := time.Now().Unix() / 60

db.QueryRow(`
    INSERT INTO ratelimit_buckets (key_id, metric, bucket, value)
    VALUES (?, 'rpm', ?, 1)
    ON CONFLICT(key_id, metric, bucket) DO UPDATE
      SET value = value + 1
    RETURNING value`, keyID, bucket)

var total int64
db.QueryRow(`
    SELECT COALESCE(SUM(value), 0)
    FROM ratelimit_buckets
    WHERE key_id = ? AND metric = 'rpm' AND bucket >= ?`,
    keyID, bucket-0,                       // 1-minute window
).Scan(&total)

if total > rpmLimit {
    return ErrRateLimited{RetryAfter: 60 - (time.Now().Unix() % 60)}
}
```

Background prune every minute: `DELETE FROM ratelimit_buckets WHERE bucket < ? - 10`.

---

## 5. Error handling

### 5.1 Error class taxonomy

```go
package adapter

type ErrorClass int

const (
    Success         ErrorClass = iota
    ClientError                // 400, 422 — return as-is
    AuthError                  // 401, 403 — mark upstream sick
    ConfigError                // 404 model not found — non-retryable
    RateLimited                // 408, 429 — retryable + cooldown
    ServerError                // 500-504 — retryable
    Timeout
    Network
    ClientCanceled
    BotDetected                // CF/Akamai HTML — sick + alert
    BudgetExceeded
)

type Error struct {
    Class      ErrorClass
    UpstreamID string
    Provider   string
    HTTPStatus int                       // 0 if not HTTP
    Message    string
    Retryable  bool
    Wrapped    error
}
```

Each adapter provides `classify(err) ErrorClass`. The router uses the class to decide retry. The ingress maps `ir.Error` → ingress-format-specific error JSON (OAI vs Anthropic shape).

---

## 6. Streaming pipeline

### 6.1 Backpressure

The IR event channel is unbuffered. Adapter blocks on send if ingress consumer is slow; this propagates pressure backward to the upstream HTTP body read.

### 6.2 Cancellation

```
Client closes connection
  → ctx.Done() fires
  → ingress goroutine returns
  → adapter goroutine sees ctx.Done in select, closes upstream body
  → IR event channel closes
  → audit committer writes status="client_canceled"
```

### 6.3 Mid-stream error frames

If upstream emits an error mid-stream, adapter emits a final `ir.Event{Type: "error", Error: …}` and closes the channel. Ingress translates:
- OAI: `data: {"error": {…}}\n\ndata: [DONE]\n\n`
- Anthropic: `event: error\ndata: {…}\n\n`

---

## 7. Plugin gRPC protocol

### 7.1 `pkg/proto/adapter.proto`

```proto
syntax = "proto3";

package sigilbridge.plugin.v1;
option go_package = "github.com/sigilbridge/sigilbridge/pkg/proto;proto";

service ProviderPlugin {
    rpc Health(HealthRequest) returns (HealthResponse);
    rpc Capabilities(CapabilitiesRequest) returns (CapabilitiesResponse);
    rpc Chat(ChatRequest) returns (ChatResponse);
    rpc Stream(StreamRequest) returns (stream IREvent);
    rpc CountTokens(CountTokensRequest) returns (CountTokensResponse);
}

message HealthRequest { bytes config_json = 1; }
message HealthResponse { bool healthy = 1; string detail = 2; }

message CapabilitiesRequest {}
message CapabilitiesResponse {
    bool streaming = 1;
    bool tool_use = 2;
    bool vision = 3;
    bool prompt_caching = 4;
    bool mcp_servers = 5;
    bool document_input = 6;
    int32 max_context_tokens = 7;
    string stability_class = 8;
    string category = 9;
}

message ChatRequest  { bytes ir_request_json = 1; bytes config_json = 2; }
message ChatResponse { bytes ir_response_json = 1; string error_message = 2; string error_class = 3; }
message StreamRequest { bytes ir_request_json = 1; bytes config_json = 2; }
message IREvent       { bytes event_json = 1; }
message CountTokensRequest  { bytes ir_request_json = 1; bytes config_json = 2; }
message CountTokensResponse { int32 input_tokens = 1; }
```

### 7.2 Plugin manifest

```yaml
name: my-custom-provider
version: 1.2.0
provider_id: my_custom
executable: ./my-custom-provider
handshake:
  protocol_version: 1
  magic_cookie_key: SIGILBRIDGE_PLUGIN
  magic_cookie_value: 7c4f9b1e-…
capabilities:
  streaming: true
  tool_use: false
  vision: false
config_schema_json: |
  {"type":"object","properties":{"endpoint":{"type":"string"},"api_key":{"type":"string"}},"required":["endpoint","api_key"]}
```

### 7.3 Host supervisor

```go
type Plugin struct {
    Manifest Manifest
    Process  *exec.Cmd
    Client   *plugin.Client                  // hashicorp/go-plugin
    Adapter  adapter.Provider                // gRPC client wrapper
    State    PluginState                     // healthy | starting | crashed | stopping
}

func (h *Host) Discover(dir string) error {
    entries, _ := os.ReadDir(dir)
    for _, e := range entries {
        if e.IsDir() { continue }
        m, err := loadManifest(filepath.Join(dir, e.Name(), "plugin.yaml"))
        if err != nil { continue }
        h.spawn(m)
    }
    return nil
}
```

---

## 8. Testing strategy

### 8.1 Unit

Each `internal/*` package has `*_test.go` co-located. Pure logic (IR translation, routing strategies, cost calc, rate-limit math) tested without I/O. Race detector clean (`go test -race ./...`).

### 8.2 Integration

`mock` adapter wired into a real bridge process with in-memory SQLite (`:memory:`). Files in `internal/integration/*_test.go`. Coverage: ingress → router → adapter → ingress round-trip on both schemas, streaming + non-streaming, tool use, error classes.

### 8.3 Subscription E2E

Behind build tag `subscription_e2e`. Requires `SIGILBRIDGE_E2E_*` env vars with real session credentials. Smoke tests: 1-token completion against each subscription adapter. Run nightly; fail-soft.

### 8.4 Plugin E2E

`examples/plugin-example/` builds a plugin binary that the integration test spawns. Verifies handshake, Chat, Stream, error paths, crash + restart.

### 8.5 UI tests

| Layer | Tool | Scope |
|---|---|---|
| Unit | Vitest | `lib/`, `schemas/` |
| Component | RTL + Vitest | shadcn-derived + custom widgets |
| E2E | Playwright | login, key creation, pool edit, OAuth bootstrap dry-run, audit query, language switch, theme switch |

### 8.6 Load / soak

`scripts/loadtest/` with `vegeta`: 500 RPS sustained for 10 min, mixed streaming/non-streaming. Memory leak check via `pprof` heap snapshots at minute intervals.

---

## 9. UI implementation details

### 9.1 Type generation

```go
// cmd/gentypes/main.go (sketch)
func main() {
    target := "ui/src/types/api.ts"
    types := []reflect.Type{
        reflect.TypeOf(admin.BridgeKeyDTO{}),
        reflect.TypeOf(admin.PoolDTO{}),
        reflect.TypeOf(admin.UpstreamDTO{}),
        reflect.TypeOf(admin.AuditQueryRequest{}),
        // … all admin DTOs
    }
    out := emitTypeScript(types)
    os.WriteFile(target, []byte(out), 0o644)
}
```

CI step `ui-types-check.yml` runs `gentypes` and fails the build if `ui/src/types/api.ts` is dirty.

### 9.2 Embed handler

```go
//go:embed dist
var distFS embed.FS

func Handler(prefix string) http.Handler {
    sub, _ := fs.Sub(distFS, "dist")
    fileSrv := http.FileServer(http.FS(sub))
    return http.StripPrefix(prefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // SPA fallback
        if _, err := fs.Stat(sub, strings.TrimPrefix(r.URL.Path, "/")); err != nil {
            r.URL.Path = "/"
        }
        // Compression negotiation: try .br, then .gz, then raw
        if strings.Contains(r.Header.Get("Accept-Encoding"), "br") {
            if data, err := fs.ReadFile(sub, r.URL.Path+".br"); err == nil {
                w.Header().Set("Content-Encoding", "br"); w.Write(data); return
            }
        }
        if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
            if data, err := fs.ReadFile(sub, r.URL.Path+".gz"); err == nil {
                w.Header().Set("Content-Encoding", "gzip"); w.Write(data); return
            }
        }
        fileSrv.ServeHTTP(w, r)
    }))
}
```

### 9.3 Dev mode

`dev.ps1` starts the Go backend and Vite UI together. Vite proxies `/admin/v1`, `/v1`, `/healthz`, and `/readyz` to the backend via `VITE_SIGILBRIDGE_TARGET`.

```powershell
.\dev.ps1 -CreateKey
```

For backend-only work:

```powershell
.\dev.ps1 -NoUI
```

---

## 10. Concurrency budget

| Goroutine | Lifecycle | Notes |
|---|---|---|
| HTTP request handler | per request | bounded by `server.max_concurrent_requests` |
| Adapter Stream consumer | per streaming request | child of request handler |
| OAuth refresh worker | persistent | one goroutine, ticker |
| Session refresh worker | persistent | one goroutine, ticker |
| CLI process supervisor | per CLI subprocess | stdin/stdout pumps |
| Audit writer | persistent | buffered channel, sync flush on shutdown |
| Audit rotator | persistent | daily timer |
| Rate-limit pruner | persistent | per-minute timer |
| Cooldown persister | persistent | shutdown-only |
| SSE event bus | persistent | one publisher, N subscribers per UI session |
| Plugin supervisors | per plugin | crash-restart |

All persistent goroutines owned by a `Supervisor` in `cmd/sigilbridge/main.go` that signals shutdown via `context.WithCancel` and waits via `sync.WaitGroup` with timeout.

---

## 11. Build & release

### 11.1 Makefile targets

```makefile
.PHONY: build ui types lint test release clean

build: ui
	go build -ldflags="-X main.version=$(VERSION) -s -w" -o dist/sigilbridge ./cmd/sigilbridge

ui:
	cd ui && pnpm install --frozen-lockfile && pnpm run build
	find ui/dist -type f \( -name '*.js' -o -name '*.css' -o -name '*.html' \) -exec gzip -9k {} \;
	find ui/dist -type f \( -name '*.js' -o -name '*.css' -o -name '*.html' \) -exec brotli -q 11 -k {} \;

types:
	go run ./cmd/gentypes

lint:
	golangci-lint run ./...
	cd ui && pnpm run lint

test:
	go test -race -coverprofile=coverage.out ./...
	cd ui && pnpm run test

release:
	bash scripts/release.sh

clean:
	rm -rf dist/ ui/dist/
```

PowerShell helpers mirror the common local workflows on Windows:

```powershell
.\build.ps1 -Version dev
.\test.ps1 -Coverage
.\clean.ps1
.\release.ps1 -Version v1.0.0
```

### 11.2 CI matrix

`.github/workflows/ci.yml` builds and tests on: Linux amd64/arm64, macOS amd64/arm64, Windows amd64. Each PR: lint → unit tests → integration → UI tests → build.

### 11.3 Release artifacts

- Multi-arch binaries (`.tar.gz` with sha256 sums)
- Distroless Docker image, multi-arch
- Signed Git tags (PGP)
- SLSA L3 provenance via GitHub OIDC

---

## 12. Observability schema

### 12.1 Log fields (slog)

Every log line: `request_id` (ULID, when applicable), `bridge_key_id`, `pool`, `upstream_id`, `provider`, `event`, `latency_ms`.

Levels:
- `debug` — IR transitions, retry decisions
- `info` — request completed, OAuth refreshed, CLI spawned
- `warn` — rate limit hit, budget approaching, refresh failed (recoverable)
- `error` — unrecoverable adapter error, plugin crash, vault decryption failure

### 12.2 Metrics

Full list in SPECIFICATION §4.7.2. Use `prometheus.NewCounterVec` with bounded label cardinality (`provider × upstream` is fine; per-key labels are NOT included to prevent explosion — per-key data lives in audit log + admin UI dashboards).

---

## 13. Cross-cutting concerns

### 13.1 Time

All timestamps UTC; rendered in user locale only in UI layer.

### 13.2 IDs

ULID via `github.com/oklog/ulid/v2` — naturally sortable, URL-safe.

### 13.3 Secret handling

Bridge never logs config values. `pricing.yaml` non-secret. `pools.yaml` may contain `${ENV_VAR}` references, expanded at load time, never logged.

### 13.4 Shutdown order

```
SIGTERM
  → stop accepting new HTTP connections (server.Shutdown with grace period)
  → drain in-flight requests (up to shutdown_grace_seconds)
  → close OAuth refresh, session refresh, rotator, pruner workers
  → flush audit buffer
  → shut down all CLI subprocesses (graceful → kill after 5s)
  → close all plugin clients
  → flush metrics
  → close storage
  → exit 0
```

### 13.5 Recovery on startup

- Reload `cooldowns` table → in-memory state (warm start, avoid re-probing healthy upstreams).
- Run pending migrations.
- Validate `pools.yaml` against loaded adapters; fail-fast on unknown provider.
- Validate every `vault://…` reference resolves; emit warning if expired.
- Run startup health probe (parallel, 5s timeout each), seed health table.

---

**End of IMPLEMENTATION v1.0.**

Next document: **TASKS.md** (granular work items mapped to file paths and acceptance criteria).
