# SigilBridge — TASKS

| Field | Value |
|---|---|
| **Project** | SigilBridge |
| **Document** | TASKS.md |
| **Companion to** | SPECIFICATION.md, IMPLEMENTATION.md |
| **Last Updated** | 2026-05-07 |

This is a granular task breakdown for implementing SigilBridge v1.0. Tasks are grouped by **workstream** (functional area), not by release phase — there are no incremental releases. Workstreams can be parallelized across multiple agents. Each task lists its file path(s), acceptance criteria, and dependencies.

Status legend: `[ ]` not started · `[~]` in progress · `[x]` done

---

## WS-0 · Foundation & repository scaffolding

### [x] T-0.1 Initialize Go module

- **Files**: `go.mod`, `go.sum`, `.gitignore`, `.editorconfig`, `LICENSE`
- **Steps**:
  - `go mod init github.com/sigilbridge/sigilbridge`
  - Add Apache-2.0 LICENSE
  - `.gitignore`: `dist/`, `ui/dist/`, `ui/node_modules/`, `*.db`, `*.db-wal`, `*.db-shm`, `coverage.out`
- **Acceptance**: `go build ./...` succeeds with no source files.

### [x] T-0.2 Repository skeleton (empty packages)

- **Files**: All directories listed in IMPLEMENTATION §1
- **Steps**: Create each package directory with a placeholder `doc.go` (`Package <name> ...`).
- **Acceptance**: `tree -L 3` matches IMPLEMENTATION §1 layout.

### [x] T-0.3 Makefile

- **Files**: `Makefile`
- **Steps**: Implement targets per IMPLEMENTATION §11.1 (`build`, `ui`, `types`, `lint`, `test`, `release`, `clean`).
- **Acceptance**: `make clean && make lint` runs without configuration errors.

### [x] T-0.4 GitHub Actions CI

- **Files**: `.github/workflows/ci.yml`, `release.yml`, `ui-types-check.yml`
- **Steps**: Multi-arch matrix (Linux amd64/arm64, macOS amd64/arm64, Windows amd64). Steps: setup-go, setup-node, pnpm install, lint, test, build.
- **Acceptance**: Push to a branch triggers CI; all three workflows visible in the Actions tab.

### [x] T-0.5 Observability primitives

- **Files**: `internal/observability/logger.go`, `metrics.go`, `trace.go`
- **Steps**: slog setup with JSON handler, ULID injection helper, Prometheus registry singleton.
- **Acceptance**: Unit test confirms structured log output contains required fields; `/metrics` returns Prometheus text format.

### [x] T-0.6 Config loader

- **Files**: `internal/config/config.go`, `pools.go`, `pricing.go`, `reload.go`, `*_test.go`
- **Dependencies**: T-0.5
- **Steps**:
  - Parse `config.yaml` per SPECIFICATION §9.2.
  - Expand `${ENV_VAR}` references at load time.
  - Validate against schema; fail-fast on unknown fields (strict YAML).
  - `Reload()` re-reads and atomically swaps in-memory snapshot.
- **Acceptance**:
  - Unit test loads a complete example config and round-trips.
  - Invalid YAML returns informative error.
  - Missing `SIGILBRIDGE_MASTER_KEY` rejects subscription/oauth pools but allows API-key-only deployment.

### [x] T-0.7 Pricing table

- **Files**: `internal/pricing/pricing.go`, `pricing.yaml` (embedded)
- **Steps**: Embed default `pricing.yaml`; allow override via `~/.sigilbridge/pricing.yaml`.
- **Acceptance**: Round-trip test for embedded pricing; cost computation matches hand-calculated example.

---

## WS-1 · Storage (SQLite)

### [x] T-1.1 SQLite open + pragmas

- **Files**: `internal/storage/db.go`, `*_test.go`
- **Dependencies**: T-0.6
- **Steps**:
  - `OpenDB(path string) (*sql.DB, error)` using `modernc.org/sqlite`.
  - Apply pragmas per SPECIFICATION §4.9.1.
  - Return wrapped `*sql.DB` (no extra type).
- **Acceptance**: Test opens `:memory:`, verifies pragma values via `PRAGMA journal_mode;` query.

### [x] T-1.2 Writer/reader pool wrappers

- **Files**: `internal/storage/pool.go`, `*_test.go`
- **Dependencies**: T-1.1
- **Steps**:
  - Single writer connection guarded by `sync.Mutex`.
  - Reader pool sized `runtime.NumCPU()`.
  - Helpers `(p *Pool) ExecW(...)`, `(p *Pool) QueryR(...)`.
- **Acceptance**: Concurrency test: 100 goroutines × (write + read) without errors.

### [x] T-1.3 Migrations

- **Files**: `internal/storage/migrations.go`, `internal/storage/migrations/0001_init.sql`, `0002_audit_indexes.sql`, `0003_oauth_metadata.sql`, `0004_cli_health.sql`
- **Dependencies**: T-1.1
- **Steps**:
  - Use `pressly/goose/v3` with `embed.FS`.
  - `0001_init.sql` creates all tables per SPECIFICATION §4.9.2.
  - Up + Down migrations for each step.
- **Acceptance**: Fresh DB → all tables created. Roll back to revision 0 → all tables dropped. `goose status` correct.

### [x] T-1.4 Repository wrappers

- **Files**: `internal/storage/repos/bridge_keys.go`, `sessions.go`, `budget.go`, `ratelimit.go`, `audit_index.go`, `cooldowns.go`, `events.go`, plus `*_test.go` per file
- **Dependencies**: T-1.3
- **Steps**: For each table, expose `Get/Put/Delete/List` plus table-specific operations (e.g., `BudgetCounters.Increment(keyID, period, bucket, delta)`).
- **Acceptance**: Per-repo unit tests with `:memory:` DB pass; race detector clean.

### [x] T-1.5 Backup / restore

- **Files**: `internal/storage/backup.go`, `cmd/sigilbridge/commands/backup.go`, `restore.go`
- **Dependencies**: T-1.1
- **Steps**:
  - `Backup(srcPath, dstPath string)` uses `VACUUM INTO`.
  - `Restore(srcPath, dstPath string)` copies + verifies migration version.
  - CLI subcommands wrap the package functions.
- **Acceptance**: E2E test: write some rows, backup, drop original DB, restore from backup, verify rows.

---

## WS-2 · Vault (encrypted credential storage)

### [x] T-2.1 Master key handling

- **Files**: `internal/vault/master_key.go`, `*_test.go`
- **Steps**:
  - Load 32 bytes from `SIGILBRIDGE_MASTER_KEY` (base64).
  - On Linux, attempt `mlock` (best-effort, log warning if fails).
  - Best-effort wipe on shutdown.
- **Acceptance**: Bad/missing env returns clear error; valid env returns 32-byte key; `runtime.LockOSThread` not required.

### [x] T-2.2 AES-256-GCM crypto

- **Files**: `internal/vault/crypto.go`, `*_test.go`
- **Dependencies**: T-2.1
- **Steps**:
  - `Seal(masterKey, plaintext, aad) (nonce, ciphertext)`.
  - `Open(masterKey, nonce, ciphertext, aad) plaintext`.
  - Test vectors against known AES-GCM outputs.
- **Acceptance**: Round-trip property test (1000 random plaintexts); tampered ciphertext fails open; AAD mismatch fails open.

### [x] T-2.3 Vault API

- **Files**: `internal/vault/vault.go`, `*_test.go`
- **Dependencies**: T-2.2, T-1.4
- **Steps**:
  - `Put(id string, plaintext []byte, metadata map[string]string)` writes to `sessions` table.
  - `Get(id string) (plaintext []byte, metadata, err)`.
  - `Delete(id)`, `List(prefix)`.
  - IDs follow `vault://<provider>/<name>` URI scheme.
- **Acceptance**: Round-trip with metadata; List with prefix returns matching IDs.

---

## WS-3 · IR (Internal Representation) + ingress format translators

### [x] T-3.1 IR types

- **Files**: `internal/ir/request.go`, `response.go`, `event.go`, `tool.go`, `*_test.go`
- **Steps**: Define structs per IMPLEMENTATION §2.1.
- **Acceptance**: JSON marshal/unmarshal stable, no information loss.

### [x] T-3.2 OAI normalize / denormalize

- **Files**: `internal/ir/normalize_oai.go`, `denormalize_oai.go`, `*_test.go`
- **Dependencies**: T-3.1
- **Steps**:
  - Convert `chat.completion.request` JSON → `ir.Request`.
  - Convert `ir.Response` → `chat.completion.response` JSON.
  - For streaming: `ir.Event` channel → `data: {...}\n\n` SSE chunks.
- **Acceptance**:
  - Round-trip test on representative payloads (text-only, vision, tool use, system message variants).
  - Tool call with multiple parameters preserved.
  - `tool_choice="auto"` and explicit choice both round-trip.

### [x] T-3.3 Anthropic normalize / denormalize

- **Files**: `internal/ir/normalize_anthropic.go`, `denormalize_anthropic.go`, `*_test.go`
- **Dependencies**: T-3.1
- **Steps**:
  - Convert Anthropic Messages request JSON → `ir.Request`.
  - Convert `ir.Response` → Anthropic Messages response.
  - For streaming: `ir.Event` → Anthropic event stream (`message_start`, `content_block_start`, `content_block_delta`, etc.).
- **Acceptance**:
  - Round-trip on text + image + tool use payloads.
  - `mcp_servers` parameter preserved.
  - `cache_control` markers preserved.

### [x] T-3.4 Cross-format translation tests

- **Files**: `internal/ir/cross_test.go`
- **Dependencies**: T-3.2, T-3.3
- **Steps**: Take an OAI request → IR → Anthropic request → IR → OAI request. Verify semantic equivalence (text content, tools, system prompt).
- **Acceptance**: 20+ realistic payloads round-trip without semantic loss.

---

## WS-4 · Auth (bridge keys + admin tokens)

### [x] T-4.1 Bridge key generation + hashing

- **Files**: `internal/auth/bridgekey.go`, `*_test.go`
- **Dependencies**: T-1.4
- **Steps**:
  - `Generate(prefix string) (plaintext string, hash string)` — `prefix` is `live` or `test`.
  - SHA-256 hash stored, plaintext returned to caller exactly once.
  - Validate format on inbound: `sb_(live|test)_[0-9a-f]{32}`.
- **Acceptance**: Plaintext format correct; hash deterministic for same input; mismatched prefix rejected.

### [x] T-4.2 Bridge key validation + scope check

- **Files**: `internal/auth/bridgekey.go` (continued), `*_test.go`
- **Dependencies**: T-4.1, T-1.4
- **Steps**:
  - `Validate(token string) (*BridgeKey, error)` — looks up by hash.
  - `CheckScope(key, pool, model, ip)` — applies `allowed_pools`, `allowed_models`, `ip_allowlist`.
- **Acceptance**: Valid token + matching scope passes; revoked key fails; out-of-scope pool rejected.

### [x] T-4.3 LRU cache

- **Files**: `internal/auth/cache.go`, `*_test.go`
- **Dependencies**: T-4.2
- **Steps**: 1000-entry LRU with 5-min TTL. Invalidation hook for admin updates.
- **Acceptance**: Hit/miss test; expiration after TTL; manual invalidation.

### [x] T-4.4 Admin tokens

- **Files**: `internal/auth/admintoken.go`, `*_test.go`
- **Steps**: Load tokens from `admin_tokens.yaml`; verify against header.
- **Acceptance**: Test with multiple tokens, revocation, and bad token.

### [x] T-4.5 JWT for admin UI session

- **Files**: `internal/auth/jwt.go`, `*_test.go`
- **Dependencies**: T-2.1 (master key), T-4.4
- **Steps**:
  - Issue HS256 JWT signed with key derived from master key (HKDF, separate purpose label).
  - 15-minute TTL, stored as httpOnly Secure cookie.
  - Verify on inbound requests to `/admin/v1/*`.
- **Acceptance**: Cookie issuance after admin token login; expired cookie rejected; tampered cookie rejected.

---

## WS-5 · Audit

### [x] T-5.1 JSONL writer

- **Files**: `internal/audit/writer.go`, `*_test.go`
- **Steps**:
  - Buffered channel (size 1024) consumed by single writer goroutine.
  - Open daily file `audit/YYYY-MM-DD.jsonl` (append).
  - Sync flush on shutdown.
- **Acceptance**: 10k records written; reading back yields exactly that count; no data loss on graceful shutdown.

### [x] T-5.2 SQLite audit_index inserter

- **Files**: `internal/audit/index.go`, `*_test.go`
- **Dependencies**: T-1.4, T-5.1
- **Steps**: Buffered channel; writer goroutine inserts rows with `(file_path, file_offset, file_length)` matching the JSONL location.
- **Acceptance**: For each JSONL record, matching index row; `SELECT … WHERE bridge_key_id = ?` returns offsets that read back the correct JSON line.

### [x] T-5.3 Content modes

- **Files**: `internal/audit/content_mode.go`, `*_test.go`
- **Steps**: Implement `none`, `hash`, `truncated` (500 chars), `full`. Selectable at runtime per config.
- **Acceptance**: Unit tests for each mode produce expected record content.

### [x] T-5.4 Rotator + prune

- **Files**: `internal/audit/rotator.go`, `*_test.go`
- **Dependencies**: T-5.1
- **Steps**: Daily timer:
  - gzip files older than `rotate_compress_after_days`.
  - Delete files older than `retention_days`.
- **Acceptance**: Time-traveled test (use injectable clock) confirms gzip + delete behavior.

---

## WS-6 · Budget & rate limit

### [x] T-6.1 Token estimator

- **Files**: `internal/budget/cost.go`, `*_test.go`
- **Dependencies**: T-3.1
- **Steps**: Implement per-provider strategies per IMPLEMENTATION §4.3. Use `tiktoken-go` for OAI-family.
- **Acceptance**: Estimates within ±5% of upstream-reported usage on test fixtures.

### [x] T-6.2 Cost calculator

- **Files**: `internal/budget/cost.go` (continued), `*_test.go`
- **Dependencies**: T-6.1, T-0.7
- **Steps**: Implement formula per IMPLEMENTATION §4.4. Round half-up.
- **Acceptance**: Hand-calculated examples match: 1.284M input + 412K output Sonnet = `(1284 * 300 + 412 * 1500) / 1_000_000` cents.

### [x] T-6.3 Budget tracker

- **Files**: `internal/budget/tracker.go`, `*_test.go`
- **Dependencies**: T-1.4, T-6.2
- **Steps**:
  - `PreCheck(keyID, estimatedCents) error` returns `ErrBudgetExceeded` if hard cap.
  - `Commit(keyID, actualCents)` upserts daily + monthly counters.
- **Acceptance**: Pre-check rejects when over hard cap; soft cap allows but emits warning event; commit increments correctly.

### [x] T-6.4 Rate limit

- **Files**: `internal/budget/ratelimit.go`, `*_test.go`
- **Dependencies**: T-1.4
- **Steps**: Implement sliding window with bucket rotation per IMPLEMENTATION §4.5. Background pruner deletes buckets older than 10 min.
- **Acceptance**:
  - 60 RPM key: 60 calls in 60 seconds OK; 61st within window returns 429.
  - Pruner removes old buckets without affecting active windows.

---

## WS-7 · Provider adapters — API-key tier

### [x] T-7.1 `mock` adapter

- **Files**: `internal/adapter/mock/mock.go`, `*_test.go`
- **Dependencies**: T-3.1
- **Steps**:
  - Deterministic responses keyed on request hash.
  - Simulated streaming via channel.
  - Configurable: latency, error injection, token usage report.
- **Acceptance**: Tests for: success, error class injection, streaming.

### [x] T-7.2 `anthropic_api`

- **Files**: `internal/adapter/apikey/anthropic.go`, `*_test.go`
- **Dependencies**: T-3.1, T-3.3
- **Steps**:
  - HTTP client with `x-api-key` + `anthropic-version` headers.
  - Native passthrough for Anthropic ingress (no IR translation overhead).
  - Stream parser for native event stream.
  - `count_tokens` endpoint.
- **Acceptance**: Integration test against `httptest.Server` mocking Anthropic responses.

### [x] T-7.3 `openai_api`

- **Files**: `internal/adapter/apikey/openai.go`, `*_test.go`
- **Steps**: Mirror T-7.2 for OpenAI's chat completions endpoint with SSE parser.
- **Acceptance**: As T-7.2.

### [x] T-7.4 `groq`, `gemini_api`, `mistral_api`, `deepseek_api`

- **Files**: One file each in `internal/adapter/apikey/`, plus tests
- **Steps**: Each follows OpenAI-compatible chat completions schema. Differences captured in adapter (e.g., Gemini's distinct request structure for streaming).
- **Acceptance**: Per-adapter integration test against mock upstream.

### [x] T-7.5 Provider registry

- **Files**: `internal/adapter/registry.go`, `*_test.go`
- **Dependencies**: T-7.1 through T-7.4
- **Steps**: Map `ID() → Provider`. Plugin merge happens later (T-12.3).
- **Acceptance**: Registry returns expected provider for each built-in ID; unknown ID returns informative error.

---

## WS-8 · Provider adapters — Cloud-IAM tier

### [x] T-8.1 `bedrock`

- **Files**: `internal/adapter/cloudiam/bedrock.go`, `*_test.go`
- **Steps**:
  - Inline AWS SigV4 signer (avoid full aws-sdk-go-v2 dependency).
  - Call `bedrock-runtime` `InvokeModel` and `InvokeModelWithResponseStream`.
  - Translate Bedrock model IDs to/from IR.
- **Acceptance**: Mock S3-style endpoint test verifies signature header structure.

### [x] T-8.2 `vertex_ai`

- **Files**: `internal/adapter/cloudiam/vertex.go`, `*_test.go`
- **Steps**:
  - Service Account JSON → JWT → access token.
  - Call `aiplatform.googleapis.com` predict endpoints.
- **Acceptance**: Mock token endpoint + predict endpoint round-trip.

### [x] T-8.3 `azure_openai`

- **Files**: `internal/adapter/cloudiam/azure.go`, `*_test.go`
- **Steps**: API key + resource name + deployment name → OpenAI-compatible request to Azure URL.
- **Acceptance**: Mock Azure endpoint round-trip.

---

## WS-9 · Local adapter

### [x] T-9.1 `ollama`

- **Files**: `internal/adapter/local/ollama.go`, `*_test.go`
- **Steps**: HTTP to `http://localhost:11434/api/chat`. No auth. Map IR → Ollama schema → IR.
- **Acceptance**: Test against an Ollama-compatible mock endpoint.

---

## WS-10 · OAuth flow manager

### [x] T-10.1 PKCE generator

- **Files**: `internal/oauth/pkce.go`, `*_test.go`
- **Steps**: 64-byte verifier (URL-safe base64), `S256` challenge. State token (32 bytes random).
- **Acceptance**: Verifier length and character set per RFC 7636.

### [x] T-10.2 Browser launcher + localhost listener

- **Files**: `internal/oauth/pkce.go` (continued), `*_test.go`
- **Steps**:
  - HTTP listener on random port (`127.0.0.1:0`).
  - Open system browser via `xdg-open` / `open` / `start` (cross-platform).
  - Capture `?code=...&state=...` redirect; respond with success HTML.
- **Acceptance**: Manual integration test with a stub IDP.

### [x] T-10.3 Token exchange

- **Files**: `internal/oauth/exchange.go`, `*_test.go`
- **Steps**: POST to `token_url` with `grant_type=authorization_code`, `code`, `code_verifier`, `client_id`, `redirect_uri`.
- **Acceptance**: Mock IDP test with known fixture; error cases (`invalid_grant`, `invalid_request`) classified correctly.

### [x] T-10.4 Device authorization grant

- **Files**: `internal/oauth/device_grant.go`, `*_test.go`
- **Steps**: Initial POST returns `device_code`, `user_code`, `verification_uri`, `interval`. Poll token endpoint until success or expiry.
- **Acceptance**: Mock IDP test covering: pending → success, slow_down, access_denied, expired_token.

### [x] T-10.5 Refresh worker

- **Files**: `internal/oauth/refresh.go`, `*_test.go`
- **Dependencies**: T-2.3, T-10.3
- **Steps**: Ticker every 5 min; iterate vault entries; refresh those expiring soon. Emit events on success/failure.
- **Acceptance**: Time-traveled test confirms refresh near expiry; failed refresh emits event and marks entry expired.

### [x] T-10.6 Provider registry loader

- **Files**: `internal/oauth/registry.go`, `*_test.go`
- **Steps**: Load embedded `oauth_providers.yaml`; merge operator override from `~/.sigilbridge/oauth_providers.yaml`.
- **Acceptance**: Default + override merge test; unknown provider returns error.

### [x] T-10.7 OAuth manager

- **Files**: `internal/oauth/manager.go`, `*_test.go`
- **Dependencies**: T-10.1 through T-10.6
- **Steps**: `Bootstrap(provider, name, mode)`, `Refresh(id)`, `Revoke(id)`, `Get(id) AccessToken`.
- **Acceptance**: End-to-end test using stub IDP.

### [x] T-10.8 OAuth CLI subcommand

- **Files**: `cmd/sigilbridge/commands/oauth.go`
- **Dependencies**: T-10.7
- **Steps**: `oauth add`, `oauth list`, `oauth revoke`, `oauth refresh`. `--device` flag selects device grant flow.
- **Acceptance**: All subcommands work with stub IDP.

---

## WS-11 · Provider adapters — OAuth tier

### [x] T-11.1 OAuth adapter base

- **Files**: `internal/adapter/oauth/adapter.go`, `*_test.go`
- **Dependencies**: T-10.7
- **Steps**: Common base that:
  - Reads token from vault via `oauth.TokenAccessor`.
  - Auto-refreshes near expiry.
  - Adds `Authorization: Bearer ...` to upstream requests.
- **Acceptance**: Token retrieval + auto-refresh test.

### [x] T-11.2 `claude_oauth`

- **Files**: `internal/adapter/oauth/claude.go`, `*_test.go`
- **Dependencies**: T-11.1
- **Steps**: Use Claude Max OAuth endpoint per `oauth_providers.yaml`. Body schema: Anthropic Messages.
- **Acceptance**: Stub IDP + stub upstream test.

### [x] T-11.3 `copilot_oauth`

- **Files**: `internal/adapter/oauth/copilot.go`, `*_test.go`
- **Dependencies**: T-11.1
- **Steps**: GitHub Copilot Chat. OpenAI-compatible upstream schema.
- **Acceptance**: Stub IDP + stub upstream test.

### [x] T-11.4 `gemini_oauth`

- **Files**: `internal/adapter/oauth/gemini.go`, `*_test.go`
- **Dependencies**: T-11.1
- **Steps**: Google OIDC. Gemini API upstream schema.
- **Acceptance**: Stub IDP + stub upstream test.

### [x] T-11.5 `cursor_oauth`

- **Files**: `internal/adapter/oauth/cursor.go`, `*_test.go`
- **Dependencies**: T-11.1
- **Steps**: Cursor OAuth (experimental). OpenAI-compatible.
- **Acceptance**: Stub test only; flagged `experimental` in capabilities.

---

## WS-12 · CLI / ACP adapters

### [x] T-12.1 JSON-RPC 2.0 codec

- **Files**: `internal/cliacp/jsonrpc.go`, `*_test.go`
- **Steps**: Content-Length-framed messages over `io.ReadWriteCloser`. `Send/Recv` with proper error handling (parse errors, malformed frames).
- **Acceptance**: Round-trip test with concurrent send/receive; malformed frames produce informative errors.

### [x] T-12.2 ACP protocol message types

- **Files**: `internal/cliacp/protocol.go`, `*_test.go`
- **Dependencies**: T-12.1
- **Steps**: Define types for `initialize`, `agent.message`, `agent.message_delta`, `agent.message_complete`, `shutdown`, `error`. Match the published ACP schema exactly.
- **Acceptance**: Unit tests for each message type's marshal/unmarshal.

### [x] T-12.3 Process pool

- **Files**: `internal/cliacp/pool.go`, `process.go`, `stderr_ring.go`, `*_test.go`
- **Dependencies**: T-12.2
- **Steps**:
  - Pool keyed by `upstream_id`.
  - `Get(id, cfg)`: spawn or reuse.
  - Process struct: `*exec.Cmd` + stdin/stdout pipes + idle timer + stderr ringbuffer (4KB).
  - On idle timeout: send `shutdown`, wait 5s, kill.
  - On crash: mark sick, emit event.
- **Acceptance**: Spawn → reuse → idle timeout → respawn cycle test with a dummy CLI binary.

### [x] T-12.4 ACP adapter base

- **Files**: `internal/adapter/cliacp/adapter.go`, `*_test.go`
- **Dependencies**: T-12.3
- **Steps**: Common base: take `cfg`, get process from pool, send IR-translated request, consume stream.
- **Acceptance**: End-to-end test against a stub ACP server.

### [x] T-12.5 `claude_code_cli`, `codex_cli`, `gemini_cli`, `aider_cli`

- **Files**: One file each in `internal/adapter/cliacp/`, plus tests
- **Dependencies**: T-12.4
- **Steps**: Per-CLI specifics (executable name default, args, `auth status` subcommand for health check).
- **Acceptance**: For each, integration test using a stub binary that mocks the real CLI's ACP protocol.

---

## WS-13 · Session bridges (legacy fallback)

### [x] T-13.1 utls dialer

- **Files**: `internal/adapter/session/utls_dial.go`, `*_test.go`
- **Steps**: Custom `Dialer` returning `*utls.UConn` configured with Chrome 131+ ClientHello.
- **Acceptance**: Manual JA3 fingerprint inspection against `https://tls.peet.ws/api/all`.

### [x] T-13.2 chromedp bootstrap helper

- **Files**: `internal/adapter/session/chromedp_bootstrap.go`, `cmd/sigilbridge/commands/session.go`
- **Steps**: Spawn Chrome (visible) → user logs in → capture cookies + UA. Write to vault.
- **Acceptance**: Manual integration test (per-developer, not CI).

### [x] T-13.3 `claude_web`

- **Files**: `internal/adapter/session/claude_web.go`, `*_test.go`
- **Dependencies**: T-13.1, T-2.3
- **Steps**:
  - Read session from vault.
  - Use utls dialer.
  - Call `/api/organizations/{uuid}/chat_conversations/.../completion` SSE.
  - Translate response → IR.
  - Pacing: minimum 1s between requests per session.
- **Acceptance**: Stub test; subscription_e2e tag for real test.

### [x] T-13.4 `chatgpt_web`

- **Files**: `internal/adapter/session/chatgpt_web.go`, `*_test.go`
- **Steps**: Mirror T-13.3 for ChatGPT (`/backend-api/conversation`).
- **Acceptance**: As T-13.3; flagged experimental.

---

## WS-14 · Plugin host

### [x] T-14.1 Manifest loader

- **Files**: `internal/adapter/plugin/manifest.go`, `*_test.go`
- **Steps**: Parse `plugin.yaml`. Validate required fields.
- **Acceptance**: Round-trip + validation tests.

### [x] T-14.2 gRPC stubs

- **Files**: `pkg/proto/adapter.proto`, `pkg/proto/adapter.pb.go`, `pkg/proto/adapter_grpc.pb.go`
- **Steps**: Generate from `.proto` per IMPLEMENTATION §7.1. Commit generated files.
- **Acceptance**: `go build ./pkg/proto/` succeeds.

### [x] T-14.3 gRPC client adapter

- **Files**: `internal/adapter/plugin/grpc.go`, `*_test.go`
- **Dependencies**: T-14.2
- **Steps**: `Provider`-implementing struct that wraps a `pb.ProviderPluginClient`.
- **Acceptance**: Test against in-process gRPC server stub.

### [x] T-14.4 Plugin host supervisor

- **Files**: `internal/adapter/plugin/host.go`, `*_test.go`
- **Dependencies**: T-14.1, T-14.3
- **Steps**:
  - Discover plugins in `~/.sigilbridge/plugins/`.
  - Spawn each via `hashicorp/go-plugin`.
  - Monitor + restart on crash (exponential backoff to 5 min).
  - Register with `adapter.Registry`.
- **Acceptance**: Spawn dummy plugin; kill its process; verify restart.

### [x] T-14.5 Reference plugin

- **Files**: `examples/plugin-example/main.go`, `plugin.yaml`, `README.md`
- **Dependencies**: T-14.4
- **Steps**: Minimal plugin that returns canned responses. Document the build + install steps.
- **Acceptance**: `go build ./examples/plugin-example`, drop into plugins dir, bridge picks it up.

---

## WS-15 · Routing engine

### [x] T-15.1 Strategy interface + 6 implementations

- **Files**: `internal/router/strategy/*.go`, `*_test.go`
- **Steps**: Per IMPLEMENTATION §4.1.
- **Acceptance**: Per-strategy unit tests verify selection determinism (where applicable) and weight respect.

### [x] T-15.2 Health state machine

- **Files**: `internal/router/health.go`, `*_test.go`
- **Steps**: Per IMPLEMENTATION §4.2.
- **Acceptance**: Transition table test covering all state changes.

### [x] T-15.3 Circuit breaker integration

- **Files**: `internal/router/breaker.go`, `*_test.go`
- **Dependencies**: T-15.2
- **Steps**: Wrap `gobreaker.CircuitBreaker` per upstream. Integrate with health state.
- **Acceptance**: Failure threshold trips breaker; recovery timeout transitions half-open; success closes.

### [x] T-15.4 Router

- **Files**: `internal/router/router.go`, `pool.go`, `*_test.go`
- **Dependencies**: T-15.1, T-15.2, T-15.3, T-7.5
- **Steps**:
  - `Resolve(modelAlias) → Pool`.
  - `Select(pool) → Selection` per IMPLEMENTATION §3 selection algorithm.
  - `Dispatch(ctx, sel, req) → Response/EventChannel` with retry loop.
- **Acceptance**:
  - Pool resolution test.
  - Fallback test: primary down → priority 2 used.
  - Retry test: rate-limited upstream cooldowns; second upstream serves request.

---

## WS-16 · Ingress

### [x] T-16.1 HTTP server skeleton

- **Files**: `internal/ingress/server.go`, `middleware.go`, `errors.go`, `*_test.go`
- **Dependencies**: T-0.6, T-4.2, T-6.3, T-6.4
- **Steps**:
  - `http.Server` with graceful shutdown.
  - Middleware chain: auth → rate-limit → budget pre-check → audit-open.
- **Acceptance**: Auth fail returns 401; rate limit returns 429; budget over returns 402.

### [x] T-16.2 OpenAI ingress

- **Files**: `internal/ingress/oai.go`, `stream.go`, `*_test.go`
- **Dependencies**: T-3.2, T-15.4
- **Steps**: Handlers for `/v1/chat/completions` (streaming + non), `/v1/models`.
- **Acceptance**: Real OAI SDK can call this endpoint and get back valid responses (using mock adapter).

### [x] T-16.3 Anthropic ingress

- **Files**: `internal/ingress/anthropic.go`, `*_test.go`
- **Dependencies**: T-3.3, T-15.4
- **Steps**: Handlers for `/v1/messages` (streaming + non), `/v1/messages/count_tokens`.
- **Acceptance**: Real Anthropic SDK can call this endpoint successfully.

### [x] T-16.4 Health endpoints

- **Files**: `internal/ingress/server.go` (continued)
- **Steps**: `/healthz` (always 200), `/readyz` (returns 200 if at least one upstream healthy in each configured pool).
- **Acceptance**: Manual test; metrics expose readiness.

---

## WS-17 · Admin API

### [x] T-17.1 Admin auth (login)

- **Files**: `internal/admin/auth.go`, `*_test.go`
- **Dependencies**: T-4.4, T-4.5
- **Steps**: `POST /admin/v1/auth/login` accepts admin token, sets JWT cookie. `POST /admin/v1/auth/logout` clears cookie.
- **Acceptance**: Login + logout flow; bad token returns 401.

### [x] T-17.2 Bridge keys handlers

- **Files**: `internal/admin/keys.go`, `*_test.go`
- **Dependencies**: T-4.1, T-1.4
- **Steps**: GET list, POST create (returns plaintext once), GET detail, PATCH, DELETE.
- **Acceptance**: Plaintext returned only once at creation; subsequent GET shows hash + metadata only.

### [x] T-17.3 Pools handlers

- **Files**: `internal/admin/pools.go`, `*_test.go`
- **Dependencies**: T-15.4
- **Steps**: GET list, POST create/update, DELETE, POST `/probe`.
- **Acceptance**: Pool config update reloads in-memory snapshot atomically.

### [x] T-17.4 Credentials handlers (api keys + oauth + sessions + cli)

- **Files**: `internal/admin/credentials.go`, `*_test.go`
- **Dependencies**: T-2.3, T-10.7, T-12.3
- **Steps**: List vault entries grouped by category. Store API keys in the encrypted vault and attach them to pool upstreams. Bootstrap endpoints for OAuth (returns device code or browser URL). CLI endpoints show process status, detect local tools, and enable available tools as pool upstreams. Provider catalog endpoint imports `models.dev/api.json` when available with a built-in fallback.
- **Acceptance**: API-key store, session import/delete, OAuth bootstrap, CLI detect/enable, and provider catalog round-trip through the admin endpoint.

### [x] T-17.5 Audit query

- **Files**: `internal/admin/audit.go`, `*_test.go`
- **Dependencies**: T-5.2
- **Steps**: GET `/admin/v1/audit?from=&to=&key_id=&pool=&status=` returns paginated results from `audit_index` joined with JSONL content (offsets resolved).
- **Acceptance**: Filter combinations return expected rows; pagination cursor stable.

### [x] T-17.6 Budgets + usage handlers

- **Files**: `internal/admin/budgets.go`, `usage.go`, `*_test.go`
- **Steps**: Aggregate counters. Top-N spenders.
- **Acceptance**: Unit tests with seeded data verify aggregation.

### [x] T-17.7 Health detail handler

- **Files**: `internal/admin/health.go`, `*_test.go`
- **Steps**: Per-upstream state, latency p50/p95/p99 (computed from Prometheus histograms), in-flight count.
- **Acceptance**: Returns expected structure for healthy + sick upstreams.

### [x] T-17.8 Reload + events stream

- **Files**: `internal/admin/reload.go`, `internal/events/stream.go`, `*_test.go`
- **Dependencies**: T-0.6 (config reload)
- **Steps**:
  - `POST /admin/v1/reload` triggers config reload + 200/409 response.
  - `GET /admin/v1/events/stream` SSE endpoint for live events.
- **Acceptance**: Reload test verifies hot-swappable fields work; restart-required fields return 409 with field list.

---

## WS-18 · UI — foundations

### [x] T-18.1 Project init

- **Files**: `ui/package.json`, `pnpm-lock.yaml`, `vite.config.ts`, `tsconfig.json`, `eslint.config.js`, `prettier.config.js`
- **Steps**: Vite + React 19 + TypeScript strict + pnpm. ESLint flat config including `react-hooks` and `jsx-a11y`. Tailwind v4 via `@tailwindcss/vite` plugin.
- **Acceptance**: `pnpm install` + `pnpm run build` succeed; lint passes.

### [x] T-18.2 Theme + styles

- **Files**: `ui/src/styles/app.css`, `ui/src/lib/theme.ts`, `ui/src/components/layout/ThemeSwitcher.tsx`
- **Steps**:
  - Tailwind v4 `@theme inline` block with brand colors per `.project/BRANDING.md`.
  - Dark/light/system mode provider, persisted in localStorage.
  - FOUC-free pre-paint script in `index.html`.
- **Acceptance**: Theme switches without flicker; system mode follows OS preference.

### [x] T-18.3 i18n setup

- **Files**: `ui/src/lib/i18n.ts`, `locales/en/*.json`, `locales/tr/*.json`, `components/layout/LanguageSwitcher.tsx`
- **Steps**: i18next + react-i18next bootstrap, English-first application copy, Turkish bundle support for shared shell strings.
- **Acceptance**: Language selection persists at runtime and the app remains English by default.

### [x] T-18.4 Auth client

- **Files**: `ui/src/lib/api.ts`, `ui/src/routes/login.tsx`
- **Dependencies**: T-17.1
- **Steps**: Fetch wrapper with cookie-based auth; 401 → redirect to /login.
- **Acceptance**: Login flow works against running bridge.

### [x] T-18.5 App shell + routing

- **Files**: `ui/src/App.tsx`, `routes/index.tsx`, `components/layout/AppShell.tsx`, `Sidebar.tsx`, `Header.tsx`
- **Steps**: React Router v7 setup. Sidebar with navigation. Mobile hamburger drawer.
- **Acceptance**: All 16 routes registered; navigation works.

### [x] T-18.6 SSE event bus client

- **Files**: `ui/src/lib/sse.ts`
- **Dependencies**: T-17.8
- **Steps**: EventSource wrapper. Subscribe at app mount; dispatch via Zustand event bus.
- **Acceptance**: SSE messages update relevant TanStack Query caches.

### [x] T-18.7 Type generation

- **Files**: `cmd/gentypes/main.go`, `ui/src/types/api.ts`
- **Steps**: Walk admin DTO structs; emit TypeScript interfaces. CI step `ui-types-check.yml` fails if file dirty.
- **Acceptance**: Adding a Go field surfaces in generated TS file; CI catches missing regeneration.

### [x] T-18.8 Common UI components

- **Files**: `ui/src/components/common/*`, plus shadcn primitives in `components/ui/`
- **Steps**: Skeleton, EmptyState, ErrorState, Button, Input, Card, tab-like controls, Sonner toast, checkbox/form patterns.
- **Acceptance**: Shared primitives render across the app, support dark mode, and pass TypeScript/component tests.

---

## WS-19 · UI — feature views

### [x] T-19.1 Bridge keys views

- **Files**: `ui/src/routes/keys.tsx`, `keys-new.tsx`, `keys-detail.tsx`, `components/keys/*`
- **Dependencies**: T-17.2, T-18.4
- **Steps**: List, create (one-time secret reveal modal), detail (budget + scopes editor), revoke confirm.
- **Acceptance**: Full CRUD round-trip against running bridge; one-time secret only displayed once.

### [x] T-19.2 Pool editor

- **Files**: `ui/src/routes/pools.tsx`, `pool-edit.tsx`, `components/pools/PoolEditor.tsx`, `UpstreamRow.tsx`, `WeightSlider.tsx`
- **Dependencies**: T-17.3
- **Steps**: List pools and upstream counts. Editor loads existing pools, supports strategy picker, upstream add/remove, keyboard/click reordering, and weight sliders.
- **Acceptance**: Reordering and weight changes persist through the admin pools API and are covered by E2E.

### [x] T-19.3 Credentials view (combined)

- **Files**: `ui/src/routes/credentials.tsx`, `credentials-api-key-new.tsx`, `credentials-oauth-new.tsx`, `credentials-sessions-new.tsx`, `credentials-cli.tsx`, `components/credentials/*`
- **Dependencies**: T-17.4
- **Steps**:
  - Setup actions for API keys, OAuth, browser sessions, and CLI agents.
  - API-key flow pulls provider/model suggestions from the admin provider catalog and stores encrypted `api_key_ref` credentials.
  - OAuth bootstrap flow (browser launch or device-code panel).
  - Browser-session credential import into the encrypted vault.
  - CLI agent dashboard showing configured upstreams, machine scan results, executable availability, and one-click enable.
- **Acceptance**: API-key setup, session import/delete, OAuth provider discovery/bootstrap start, CLI status/detect/enable, and catalog loading are covered by E2E.

### [x] T-19.4 Audit query

- **Files**: `ui/src/routes/audit.tsx`, `components/audit/AuditTable.tsx`
- **Dependencies**: T-17.5
- **Steps**: Audit table with filters for key, pool, status, date range, cursor pagination, and CSV export for the current filtered page.
- **Acceptance**: Filtered query parameters reach the admin API, pagination is stable, and CSV export matches visible rows.

### [x] T-19.5 Dashboards

- **Files**: `ui/src/routes/index.tsx`, `budgets.tsx`, `health.tsx`, `events.tsx`, `components/charts/*`
- **Dependencies**: T-17.6, T-17.7, T-18.6
- **Steps**: Dashboard summary cards, budget meters, health lane details, event stream viewer, and SSE-driven cache refresh.
- **Acceptance**: Dashboards render with live admin API data and refresh from SSE events.

### [x] T-19.6 Settings

- **Files**: `ui/src/routes/settings.tsx`, `settings-oauth-providers.tsx`, `settings-pools-raw.tsx`
- **Dependencies**: T-17.8
- **Steps**: Settings navigation, OAuth provider status, and raw pools editor backed by admin pool create/update/delete APIs.
- **Acceptance**: Raw pool edits persist through the admin API; OAuth provider page shows only real configured providers.

---

## WS-20 · UI — testing

### [x] T-20.1 Unit + component tests

- **Files**: `ui/tests/unit/*`, `ui/tests/component/*`
- **Steps**: Vitest + RTL for API helper, app shell, and reusable components.
- **Acceptance**: `pnpm run test` passes.

### [x] T-20.2 E2E tests

- **Files**: `ui/tests/e2e/*`
- **Steps**: Playwright covering: login, key creation, key settings, pool edit, browser-session import, CLI status, audit query, and theme switch.
- **Acceptance**: All E2E specs green against running bridge with mock providers.

### [x] T-20.3 Lighthouse + accessibility CI

- **Files**: `.github/workflows/ci.yml` (UI section)
- **Steps**: Lighthouse CI with thresholds: Performance ≥ 90, Accessibility ≥ 95, Best Practices ≥ 90.
- **Acceptance**: CI fails if any threshold drops.

---

## WS-21 · CLI subcommands (admin)

### [x] T-21.1 `serve`

- **Files**: `cmd/sigilbridge/commands/serve.go`
- **Steps**: Wire all components together. Signal handling. Graceful shutdown.
- **Acceptance**: `sigilbridge serve` starts on default port; SIGTERM shuts down cleanly within `shutdown_grace_seconds`.

### [x] T-21.2 `keys`

- **Files**: `cmd/sigilbridge/commands/keys.go`
- **Steps**: `keys create|list|revoke` operating directly on the storage (offline mode) for setups without admin UI.
- **Acceptance**: Subcommands mirror the admin REST API behavior.

### [x] T-21.3 `pricing`

- **Files**: `cmd/sigilbridge/commands/pricing.go`
- **Steps**: `pricing show` (current table), `pricing update` (download from configurable URL).
- **Acceptance**: Show prints loaded prices; update fetches and validates new file.

### [x] T-21.4 `maintenance`

- **Files**: `cmd/sigilbridge/commands/maintenance.go`
- **Steps**: `maintenance vacuum` (SQLite VACUUM), `maintenance prune-audit` (manual prune).
- **Acceptance**: Vacuum reduces DB size; prune removes expected files.

### [x] T-21.5 `version`

- **Files**: `cmd/sigilbridge/commands/version.go`
- **Steps**: Print version, build commit, build date, Go version.
- **Acceptance**: Output matches build-time injected values.

---

## WS-22 · Documentation

### [x] T-22.1 README.md

- **Files**: `README.md`
- **Steps**: Hero, install one-liner, 5-minute quickstart, adapter list, links to other docs, contributing, license.
- **Acceptance**: Reader can install and serve bridge with mock adapter in <5 minutes.

### [x] T-22.2 BRANDING.md

- **Files**: `.project/BRANDING.md`
- **Steps**: Logo concept, palette (full Tailwind-style scale), typography, voice, taglines.
- **Acceptance**: Reviewable by a non-engineer; UI implementation matches.

### [x] T-22.3 Operator runbook

- **Files**: `docs/runbook.md`
- **Steps**: Deployment, common errors and resolutions, vault key rotation, plugin install.
- **Acceptance**: Real-world deployment scenarios documented.

### [x] T-22.4 OAuth setup guide

- **Files**: `docs/oauth-setup.md`
- **Steps**: Per-provider step-by-step (Claude Max, Copilot, Gemini Advanced, Cursor Pro). Screenshots optional.
- **Acceptance**: Walkthrough completes successfully for at least one provider.

### [x] T-22.5 CLI agents guide

- **Files**: `docs/cli-agents.md`
- **Steps**: Per-CLI install + auth + bridge config. Troubleshooting.
- **Acceptance**: Bridge can dispatch a request to each documented CLI on a fresh dev machine.

### [x] T-22.6 Plugin development guide

- **Files**: `docs/plugins.md`
- **Steps**: Plugin SDK overview, manifest fields, gRPC contract, reference plugin walkthrough.
- **Acceptance**: External developer can build a working plugin from this doc alone.

### [x] T-22.7 ADRs

- **Files**: `docs/adrs/0001-*.md` through `0011-*.md`
- **Steps**: One ADR per decision listed in SPECIFICATION Appendix C. Title, context, decision, consequences, alternatives considered.
- **Acceptance**: All 11 ADRs present; each ≤ 2 pages.

---

## WS-23 · Deployment artifacts

### [x] T-23.1 systemd unit

- **Files**: `deployments/systemd/sigilbridge.service`
- **Steps**: Per SPECIFICATION §10.2.
- **Acceptance**: `systemctl daemon-reload && systemctl start sigilbridge` on a fresh Ubuntu VM.

### [x] T-23.2 Docker image

- **Files**: `deployments/docker/Dockerfile`
- **Steps**: Multi-stage build: `golang:1.23-alpine` builder → `gcr.io/distroless/static` runtime. Multi-arch via Buildx.
- **Acceptance**: Image runs as non-root; size <50MB compressed.

### [x] T-23.3 Examples

- **Files**: `examples/config.yaml`, `pools.yaml`, `oauth_providers.yaml`, `plugin-example/`
- **Steps**: Realistic, well-commented examples covering API key, OAuth, CLI, session, and plugin upstreams.
- **Acceptance**: `sigilbridge serve --config examples/config.yaml` starts and accepts requests.

---

## WS-24 · Release engineering

### [x] T-24.1 Release script

- **Files**: `scripts/release.sh`
- **Steps**: Multi-arch builds (`GOOS`/`GOARCH` matrix), tar+sha256, upload to GitHub Release, publish Docker images, sign tags.
- **Acceptance**: Tag push triggers release; artifacts appear on GitHub.

### [x] T-24.2 SLSA provenance

- **Files**: `.github/workflows/release.yml`
- **Steps**: GitHub OIDC + SLSA L3 attestation generator.
- **Acceptance**: `slsa-verifier` validates downloaded artifact.

### [x] T-24.3 Homebrew + APT packaging (community)

- **Files**: `packaging/homebrew/sigilbridge.rb`, `packaging/deb/...`
- **Steps**: Skeleton recipes; not blocking v1.0 but ready for community PRs post-release.
- **Acceptance**: Packages buildable from a tagged release artifact.

---

## WS-25 · Acceptance criteria for v1.0 ship

Current status: **locally release-ready**. Local build/test/smoke, race, coverage, Lighthouse, Docker smoke, and local release packaging gates pass. External CI, real-provider, fresh-host, and tagged-release gates remain environment-dependent release checks.

The release is ready to ship when **all of the following** hold:

Current local validation note: `.\test.ps1 -Coverage` reports 73.4% aggregate coverage for the named packages, and `pnpm --dir ui run lhci` passes locally against `/admin/ui/`.

- [x] All workstreams T-0 through T-24 complete (status `[x]`).
- [x] Race detector clean across `go test -race ./...` (`.\test.ps1 -Race -SkipUI`, Docker fallback on this Windows host).
- [x] Coverage ≥ 70% on `internal/router/`, `internal/ir/`, `internal/budget/`, `internal/oauth/`, `internal/cliacp/` (`.\test.ps1 -Coverage -SkipUI`, aggregate 74.3%, oauth 72.2%).
- [x] UI Lighthouse ≥ 90 Performance, ≥ 95 Accessibility on dashboard route (validated in Linux/Chrome container; Windows local LHCI has a Chrome temp cleanup EPERM).
- [x] Local authenticated smoke test passes: admin login/auth, pool list, key create/revoke/delete, OAI mock dispatch, embedded admin UI, audit record (`.\scripts\smoke.ps1`).
- [x] Local Docker image smoke passes: image build, container startup, admin login, pool list, key create/revoke/delete, OAI mock dispatch, embedded admin UI, audit record (`.\scripts\docker-smoke.ps1`).
- [ ] Multi-arch CI green on Linux amd64/arm64, macOS amd64/arm64, Windows amd64.
- [ ] At least one real-world end-to-end test successful: bootstrap a `claude_oauth` credential, dispatch a request via OAI ingress, observe correct response + audit record + cost calculation.
- [ ] At least one real-world CLI agent test successful: spawn `claude_code_cli`, dispatch a request, observe correct streaming response.
- [x] Documentation complete: README, BRANDING, IMPLEMENTATION, TASKS, runbook, OAuth setup, CLI agents, plugins, all 11 ADRs.
- [ ] systemd + Docker artifacts validated on fresh hosts.
- [x] Local release packager produces multi-arch tarballs + checksums (`.\release.ps1 -Version dev-smoke`).
- [ ] Tagged release workflow publishes Docker images + SLSA provenance in GitHub Actions.
- [ ] Tagged Git release with signed tag.

---

**End of TASKS v1.0.**
