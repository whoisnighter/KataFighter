# KataFighter

An open-source character editor for **[Ikemen GO](https://github.com/ikemen-engine/Ikemen-GO)** (and M.U.G.E.N-compatible content), built with **Go + Wails v3**.

Sprites, animations, hitboxes, CNS/ZSS code, sounds and testing in one cross-platform app. It stays in sync with the engine by reusing Ikemen GO's own parsing code.

> **Status: early planning.** No release yet. The roadmap is in [PLAN.md](PLAN.md).

## Goals

- **Engine fidelity:** read files exactly as Ikemen GO does.
- **Ikemen-native:** ZSS, Ikemen-only state controllers and triggers, PNG-based SFF v2.
- **Modern UX:** undo/redo everywhere, drag & drop, dark/light themes.
- **Cross-platform:** Windows, macOS, Linux.
- **Open source:** modular Go libraries others can reuse.

## Roadmap

| Phase | Deliverable |
|---|---|
| 0 | Foundations: repo, Go workspace, Wails app skeleton, CI |
| 1 | `katafighter-formats`: read DEF / SFF / AIR / CNS / CMD / ZSS / SND / ACT |
| 2 | `katafighter-lsp` + VS Code extension (first public release) |
| 3 | Editor viewer: sprites, animations, hitboxes, code |
| 4 | Editing & saving → **v0.1.0 "Kata"** |
| 5 | One-click testing in Ikemen GO |

The details are in [PLAN.md](PLAN.md).

## Repository layout

```
KataFighter/
├─ app/                Wails v3 desktop app (Go services + Svelte/TS)
├─ formats/            katafighter-formats: file format readers/writers
├─ lsp/                katafighter-lsp: language server for CNS/ST/CMD/ZSS
├─ vscode-extension/   VS Code client for the LSP
├─ docs/               design notes, upstream audit
├─ upstream/Ikemen-GO/ Ikemen GO source (git submodule, pinned to v1.0.0)
└─ runtime/            local Ikemen GO builds for testing (git-ignored)
```

Each folder has an `AGENTS.md` that explains its purpose and rules, for contributors and AI coding agents.

## Getting started

```sh
git clone --recurse-submodules https://github.com/whoisnighter/KataFighter.git
```

If you already cloned without submodules:

```sh
git submodule update --init --depth 1
```

For testing, download an [Ikemen GO release](https://github.com/ikemen-engine/Ikemen-GO/releases) and unzip it into `runtime/`.

## Credits

KataFighter reuses code from [Ikemen GO](https://github.com/ikemen-engine/Ikemen-GO) (MIT License). It is an independent project and is not affiliated with the Ikemen GO team or Elecbyte.

## License

MIT (license file coming soon).
