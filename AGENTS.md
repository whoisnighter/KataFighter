# AGENTS.md: KataFighter

KataFighter is an open-source character editor for Ikemen GO, built with Go + Wails v3. Current phase: **Phase 0/1 → 3** (`formats` readers and parsers done; sprite browser and animation player done; Monaco editor with LSP diagnostics done; completion/hover and cross-navigation next). See the [roadmap](docs/PLAN.md#7-roadmap).

This file is the hub. Each fact lives in one file only, and the others link to it. When you work in a folder, read that folder's AGENTS.md too.

## Map

| Path | What | Owns |
|---|---|---|
| [app/](app/AGENTS.md) | Wails v3 desktop app (Go services + Svelte/TS) | UI ↔ core boundary |
| [formats/](formats/AGENTS.md) | `katafighter-formats`: file readers/writers | Extraction rules, parsing rules |
| [lsp/](lsp/AGENTS.md) | `katafighter-lsp`: language server | `schema/` (generated) |
| [tools/](tools/AGENTS.md) | Build-time generators (`schemagen`) | Generating `lsp/schema` from Ikemen source |
| [docs/](docs/AGENTS.md) | Plan, audit, design notes | Vision, roadmap, upstream audit |
| [ikemen/](ikemen/AGENTS.md) | Ikemen GO source (submodule) + release builds | Pinned engine version, test corpus |

## Dependency direction

```
app ──► lsp ──► formats ◄── code is copied from ikemen/source (never imported)
 │               ▲
 └───────────────┘

ikemen/source ◄──reads── tools/schemagen ──writes──► lsp/schema/ikemen.json
```

## Global rules

- **Engine fidelity:** match Ikemen GO at the version pinned in [ikemen/](ikemen/AGENTS.md).
- **Clean-room:** don't copy code, names, icons or UI from Fighter Factory.
- **Never commit** anything under `ikemen/release/`.
- **Line endings:** LF (enforced by `.gitattributes`).

## Commits

- Imperative mood, short subject line, and a body when the change needs explaining.

## Setup

See [CONTRIBUTING.md](CONTRIBUTING.md) (requirements, test corpus, running tests and the app). CI runs the same steps on Windows, macOS and Linux ([.github/workflows/ci.yml](.github/workflows/ci.yml)).
