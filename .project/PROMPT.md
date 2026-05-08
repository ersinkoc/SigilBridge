# SigilBridge — Claude Code Execution Prompt

This file is the prompt to give Claude Code when delegating the full SigilBridge v1.0 implementation. Copy the section between the markers verbatim into the first message of a fresh Claude Code session.

---

## ▼▼▼ COPY FROM HERE ▼▼▼

You are implementing **SigilBridge v1.0** — a self-hosted AI gateway in pure Go with an embedded React admin UI. This project is owned by ECOSTACK TECHNOLOGY OÜ and licensed Apache-2.0. The repository is initialized but contains only the planning documents below.

## Step 0 — Read the planning docs first

Before writing any code, read these four files in this exact order. They are the single source of truth.

1. `.project/SPECIFICATION.md` — what the system does, why, and the full surface area
2. `.project/IMPLEMENTATION.md` — repo layout, data structures, sequence diagrams, algorithms, plugin protocol
3. `.project/TASKS.md` — granular task breakdown by workstream, with file paths and acceptance criteria
4. `.project/BRANDING.md` — visual identity, color tokens, voice (used in `ui/` and all user-facing strings)
5. `README.md` — public-facing tone reference

If these docs disagree, **SPECIFICATION wins**. If a task description in `.project/TASKS.md` contradicts an algorithm in `.project/IMPLEMENTATION.md`, file an inline TODO comment, follow IMPLEMENTATION, and surface the discrepancy in your final report.

## Step 1 — Operating principles

These are non-negotiable. Reread them whenever you start a new workstream.

- **Pure Go single binary.** No CGO. SQLite via `modernc.org/sqlite` only. The full dependency list is fixed by SPECIFICATION §3 — do not introduce new dependencies without justifying it in an ADR.
- **#NOFORKANYMORE.** Single static binary, embedded UI via `embed.FS`, embedded migrations via `embed.FS`, embedded pricing table via `embed.FS`. The release artifact is one file plus an optional `pools.yaml`.
- **Stdlib first.** Reach for `net/http`, `database/sql`, `crypto/aes`, `encoding/json` before any third-party equivalent. Third-party libraries enter the build only when stdlib is genuinely insufficient (gRPC, goose for migrations, lucide-react in the UI, etc.).
- **No incremental phases, no v0.x.** Everything in `.project/TASKS.md` ships in the same v1.0 release. Do not invent a "minimum viable" subset and ship that first.
- **Documentation-first.** When a task introduces a non-trivial decision, write or update an ADR under `docs/adrs/` before the implementation lands.
- **Tests beside source.** Every package has `*_test.go` files alongside its `.go` files. The test runner must be clean with `-race`.

## Step 2 — Execution order

Workstreams in `.project/TASKS.md` are tagged WS-0 through WS-25. They are designed to allow parallelism, but a single agent should walk them roughly in dependency order:

1. **WS-0** (foundation) — entire workstream, sequentially.
2. **WS-1** (storage) and **WS-2** (vault) in parallel-friendly order: T-1.1 → T-1.2 → T-1.3 → T-1.4 → T-1.5; T-2.1 → T-2.2 → T-2.3.
3. **WS-3** (IR), **WS-4** (auth), **WS-5** (audit), **WS-6** (budget) — these can be tackled in any order; they share no dependencies among themselves.
4. **WS-7** (API key adapters), **WS-8** (cloud IAM), **WS-9** (local).
5. **WS-10** (OAuth manager) before **WS-11** (OAuth adapters).
6. **WS-12** (CLI/ACP) — independent of OAuth.
7. **WS-13** (session bridges) — last among adapters because of ToS sensitivity.
8. **WS-14** (plugin host).
9. **WS-15** (routing engine) — depends on at least one working adapter.
10. **WS-16** (ingress) — depends on routing.
11. **WS-17** (admin API).
12. **WS-18** (UI foundations) → **WS-19** (UI feature views) → **WS-20** (UI testing).
13. **WS-21** (CLI subcommands), **WS-22** (documentation), **WS-23** (deployment), **WS-24** (release engineering).
14. **WS-25** acceptance checklist — verify every box before declaring v1.0 ready.

Inside each workstream, work top-to-bottom. Mark tasks `[~]` when started and `[x]` when the acceptance criteria pass.

## Step 3 — Coding standards

### Go

- Target Go 1.23+. Use the latest language features (`range over int`, `clear()`, `min()`, `max()`, etc.) where they improve clarity.
- Run `golangci-lint run ./...` before each commit. Fix every lint, do not silence them with `//nolint` unless an ADR justifies it.
- Errors: wrap with `%w`; classify upstream errors via `internal/adapter/errors.go`; never panic in request paths.
- Concurrency: every persistent goroutine is owned by the supervisor and waits on shutdown via `WaitGroup`. Channels are sized intentionally, never `make(chan T, 1000)` "just in case".
- Logging: `slog` JSON only. Field names per IMPLEMENTATION §12.1. Never log secrets, tokens, ciphertext, raw user content (use audit's content-mode for that).
- IDs: ULID via `github.com/oklog/ulid/v2`. UUIDs only for `magic_cookie_value`.
- Time: UTC everywhere. Convert in the UI layer only.
- File I/O: paths must respect the configured roots from `config.yaml`. Never write outside the bridge's working directories.

### TypeScript / UI

- React 19, TypeScript 5.x with `strict: true` and `noUncheckedIndexedAccess: true`.
- No `any`. If you reach for it, you have not understood the type — re-read it.
- Tailwind v4 only via `@theme inline`. No `tailwind.config.js`.
- shadcn/ui components installed via the official CLI, customized only for theme tokens defined in `.project/BRANDING.md`.
- Lucide React icons only. No mixing icon sets.
- All visible strings in `locales/<lang>/*.json`. CI enforces no string literals in `.tsx` outside `__tests__/`.
- Forms: React Hook Form + Zod, schemas mirror Go struct validation. Generated types in `ui/src/types/api.ts` come from `cmd/gentypes` — never edit by hand.
- Zustand for UI-only state (theme, sidebar collapsed, transient toasts). TanStack Query for everything fetched from the bridge. No global Redux store.

### Voice in user-facing text

Match `.project/BRANDING.md` §6. Direct, engineer-to-engineer, no marketing fluff, no emojis-as-decoration. When writing error messages, log lines, CLI output, or UI copy, ask: *would this read at home in a Postgres or nginx CLI?* If not, rewrite.

## Step 4 — Testing requirements

Every workstream has unit and (where applicable) integration tests. The non-negotiable bars are in `.project/TASKS.md` WS-25:

- `go test -race ./...` clean across all platforms in CI.
- ≥70% coverage on `internal/router/`, `internal/ir/`, `internal/budget/`, `internal/oauth/`, `internal/cliacp/`.
- Lighthouse ≥ 90 Performance, ≥ 95 Accessibility on the UI dashboard route.
- All UI E2E specs (Playwright) green against a running bridge with the mock adapter.

When you cannot test something automatically (subscription bridges, real OAuth IDPs), gate the test behind a build tag (`subscription_e2e`) and document the manual verification step in `docs/runbook.md`.

## Step 5 — Commit discipline

- Conventional Commits style: `feat(router): add weighted_round_robin strategy`, `fix(oauth): respect interval in device grant polling`, `docs(adr): record vault encryption scheme`.
- One commit per task in `.project/TASKS.md` when the task is small. Larger tasks (T-15.4, T-19.2, T-19.3) may take 3–6 commits — split along internal seams (algorithm → wiring → tests).
- Commit message body references the task ID: `Closes T-15.1`.
- Never commit generated artifacts (`ui/dist/`, `coverage.out`, `*.db*`).
- Commit `pkg/proto/*.pb.go` (generated but stable surface).

## Step 6 — When you finish a workstream

1. Update its tasks to `[x]` in `.project/TASKS.md`.
2. Run the workstream-relevant tests + lint locally; ensure CI green on push.
3. Update relevant ADRs if you made architectural decisions during the work.
4. Open the next workstream per the order in Step 2.

## Step 7 — Asking for help

You will hit ambiguity. The default response is:

1. Re-read the relevant section of `.project/SPECIFICATION.md` and `.project/IMPLEMENTATION.md`.
2. If still ambiguous, check `docs/adrs/` for a decision that covers the area.
3. If still ambiguous, file your interpretation as an inline TODO with a brief rationale, implement it, and surface it in your final report so the operator (Ersin) can confirm or correct.

Do not block waiting for clarification on small decisions. Choose the option most consistent with the project's stated principles (single binary, stdlib first, dev-engineer voice, secrets-never-logged, OAuth-preferred-over-session) and proceed.

## Step 8 — Definition of done for v1.0

The release is ready to ship when **every checkbox in `.project/TASKS.md` WS-25 is checked**, the multi-arch CI is green, and the README quickstart works on a fresh Linux + macOS machine end-to-end with a real Anthropic API key. Tag v1.0.0, sign the tag, push the release, and the work is done.

Do not ship without WS-25 fully checked. Do not invent a "v0.9 preview" path.

## ▲▲▲ COPY UNTIL HERE ▲▲▲

---

## Notes for the human operator

- This prompt assumes the four planning docs (SPECIFICATION, IMPLEMENTATION, TASKS, BRANDING) are present under `.project/` before Claude Code starts.
- For long-running sessions, periodically remind Claude Code to re-read `.project/TASKS.md` and confirm its task status table is current.
- The `subscription_e2e` build tag is for manual nightly runs. Do not put real session credentials in CI.
- If you want to delegate a single workstream rather than the entire build, replace Step 2 with: *Implement only WS-N. Walk every task inside it. Stop when WS-N's tasks are all `[x]`.*
