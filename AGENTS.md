# AGENTS.md — KataFighter

KataFighter is an open-source character editor for Ikemen GO, built with Go + Wails v3. Plan and roadmap: `docs/PLAN.md`. Current phase: **Phase 0/1** (foundations + SFF extraction spike).

## Layout

```
KataFighter/
├─ app/                Wails v3 desktop app (Go services + Svelte/TS frontend)
├─ formats/            katafighter-formats: DEF/SFF/AIR/CNS/CMD/ZSS/SND/ACT readers & writers
├─ lsp/                katafighter-lsp: language server for CNS/ST/CMD/ZSS
├─ vscode-extension/   VS Code client for the LSP
├─ docs/               PLAN.md, upstream-audit.md, design notes
└─ ikemen/             everything from Ikemen GO (reference only, not built)
   ├─ source/          Ikemen GO source, git submodule pinned to v1.0.0 (81c6da7)
   └─ release/         official release builds, git-ignored (test corpus + test runner)
```

Every folder has its own `AGENTS.md` with local rules. Read it before you work in that folder.

## Rules

- **Dependency direction:** `app` and `lsp` → `formats`. Nothing imports from `ikemen/`. Code is *copied* from `ikemen/source` into `formats/`, never referenced.
- **Engine fidelity:** match Ikemen GO behaviour at the pinned commit. Compare with `ikemen/source`, not with Ikemen's `develop` branch.
- **Extracted code** keeps the Ikemen GO MIT copyright notice and records the upstream file and commit. Update `docs/upstream-audit.md` when you extract something.
- **Tolerant parsing:** never panic on malformed community files. Return errors or diagnostics.
- **Clean-room:** don't copy code, names, icons or UI from Fighter Factory.
- **Never commit** anything under `ikemen/release/` (large binaries and third-party assets).
- Line endings are LF (`.gitattributes`). Use Go `testing` with golden files for format code.
- Commit messages: imperative, short subject line.

## Setup

```sh
git submodule update --init --depth 1
```

Unzip an Ikemen GO release into `ikemen/release/`, for example `ikemen/release/Ikemen_GO-v1.0.0-windows/`.
