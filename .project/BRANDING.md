# SigilBridge Branding

## Positioning

SigilBridge is an operator-grade AI gateway: precise, calm, self-hosted, and trustworthy. It should feel like infrastructure you can run at 2 a.m. without needing a vendor dashboard.

## Tagline Options

- One bridge for every model path.
- Route models, not application code.
- Self-hosted control for AI access.
- The bridge between your keys, accounts, agents, and apps.

## Logo Concept

The logo should combine two ideas:

- A compact mark suggesting a sealed credential or vault.
- A bridge shape suggesting routing between independent model providers.

Avoid wizard-like or fantasy styling. The product name can carry the "sigil" idea; the visual system should stay modern, technical, and durable.

## Palette

Primary brand colors are balanced for a quiet operations UI. They should not produce a one-note purple, beige, or dark-blue app.

| Token | 50 | 100 | 200 | 300 | 400 | 500 | 600 | 700 | 800 | 900 | 950 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Bridge | `#f0fdfa` | `#ccfbf1` | `#99f6e4` | `#5eead4` | `#2dd4bf` | `#14b8a6` | `#0d9488` | `#0f766e` | `#115e59` | `#134e4a` | `#042f2e` |
| Signal | `#fff7ed` | `#ffedd5` | `#fed7aa` | `#fdba74` | `#fb923c` | `#f97316` | `#ea580c` | `#c2410c` | `#9a3412` | `#7c2d12` | `#431407` |
| Ink | `#f8fafc` | `#f1f5f9` | `#e2e8f0` | `#cbd5e1` | `#94a3b8` | `#64748b` | `#475569` | `#334155` | `#1e293b` | `#0f172a` | `#020617` |
| Status | `#f0fdf4` | `#dcfce7` | `#bbf7d0` | `#86efac` | `#4ade80` | `#22c55e` | `#16a34a` | `#15803d` | `#166534` | `#14532d` | `#052e16` |
| Danger | `#fef2f2` | `#fee2e2` | `#fecaca` | `#fca5a5` | `#f87171` | `#ef4444` | `#dc2626` | `#b91c1c` | `#991b1b` | `#7f1d1d` | `#450a0a` |

Recommended UI usage:

- Primary actions: Bridge 600 on light, Bridge 400 on dark.
- Destructive actions: Danger 600 on light, Danger 400 on dark.
- Warnings and cost emphasis: Signal 600.
- Text and surfaces: Ink scale.

## Typography

- Interface: Inter, system-ui, sans-serif.
- Code and tokens: JetBrains Mono, ui-monospace, SFMono-Regular, Consolas.
- Keep headings compact in dashboards and tool surfaces. Reserve large display type for marketing or documentation covers only.

## Voice

Write like a senior operator explaining a system clearly:

- Direct, specific, and calm.
- Prefer verbs over marketing adjectives.
- Say what changed, what failed, and what to do next.
- Avoid jokes in UI states, errors, and security copy.
- Avoid magic language in product surfaces; use "route", "seal", "vault", "pool", and "upstream" consistently.

Examples:

- Good: "This key can call 2 pools and has spent $12.48 today."
- Good: "OAuth refresh failed. Reconnect the credential or route traffic to a fallback upstream."
- Avoid: "Unlock infinite AI power."

## Product Copy Rules

- Default app copy is English.
- Turkish may exist as an optional i18n locale, but it must not become the default visible language.
- Keep button labels short: "Create key", "Probe", "Reload", "Revoke".
- Prefer concrete empty states: "No bridge keys yet" with a single primary action.
- Do not describe keyboard shortcuts or UI mechanics in visible app text unless needed for accessibility or recovery.
