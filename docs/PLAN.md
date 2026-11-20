# KataFighter — Project Plan

> An open-source character editor for **Ikemen GO** (and M.U.G.E.N-compatible content), built with **Go + Wails v3**.

Status: Draft v0.2 · Last updated: 2026-11-19

---

## 1. Vision

KataFighter aims to be a modern, cross-platform, open-source studio for creating fighting game characters for Ikemen GO. It should feel like a real IDE: sprites, animations, hitboxes, code (CNS/ZSS), sounds and testing in one place, and it should always agree with the engine because it is built on the engine's own code.

### Goals

- **Engine fidelity first.** Parse and interpret files exactly like Ikemen GO does by reusing (extracting) Ikemen GO's own Go code.
- **Ikemen-native.** First-class support for Ikemen GO features (ZSS, new state controllers/triggers, PNG-based SFF v2), not only legacy MUGEN.
- **Modern UX.** Fast, keyboard-friendly, dark/light themes, undo/redo everywhere, drag & drop.
- **Cross-platform.** Windows, macOS, Linux from day one.
- **Open source & community-driven.** MIT license, clear contribution guide, modular code others can reuse.

### Non-goals (for v1)

- Not a game engine — Ikemen GO is the runtime.
- No stage / screenpack / storyboard editor in v1 (planned later).
- No SFF v1 (PCX) *editing* in v1 (read-only import + convert to SFF v2 is acceptable).
- No 3D, no OpenBOR support.
- Not a clone of Fighter Factory: no copied code, UI assets, names or icons (clean-room implementation).

---

## 2. Guiding Principles

1. **Single source of truth with the engine.** Format/parsing logic lives in a shared Go module extracted from Ikemen GO; aim to upstream it so the engine and the editor share the same code.
2. **Lossless round-trip.** Opening and saving a file without edits must produce an equivalent file (preserve comments, ordering and formatting in text files where possible).
3. **Tolerant parsing.** Community characters are often "dirty". Never crash; report diagnostics and keep going.
4. **Ship small, ship early.** Each phase produces something usable on its own (e.g. the LSP is useful before the editor exists).
5. **Separation of concerns.** Core libraries have zero UI dependencies; the UI talks to the core only through a typed service layer.

---

## 3. Tech Stack

| Layer | Choice | Notes |
|---|---|---|
| Language (core) | Go (latest stable) | Same language as Ikemen GO |
| Desktop shell | **Wails v3** | Native OS WebView; **pin an exact version** in `go.mod` |
| Frontend | Svelte 5 + TypeScript + Vite | Lightweight, reactive; React acceptable if contributors prefer |
| Code editor | Monaco Editor | Same core as VS Code; connects to our LSP |
| Sprite / hitbox canvas | HTML Canvas 2D → WebGL (PixiJS) later | Pixel-perfect zoom, nearest-neighbour scaling |
| Styling | CSS variables + a small component kit | Theming via tokens |
| Language server | Go, LSP protocol | Reused by editor and a VS Code extension |
| Build / CI | GitHub Actions | Windows / macOS / Linux matrix |
| Packaging | Wails v3 packaging (NSIS/MSI, .app/.dmg, AppImage/.deb) | Code signing later |
| Testing | Go `testing` + golden files; Vitest + Playwright for frontend | |
| License | MIT | Compatible with Ikemen GO (MIT code) |

---

## 4. Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                     KataFighter (Wails v3 app)               │
│  ┌───────────────────────── Frontend (Svelte/TS) ─────────┐  │
│  │ Project tree │ Sprite editor │ Animation/Clsn editor   │  │
│  │ Code editor (Monaco) │ Palette │ Sound │ Test panel    │  │
│  └───────────────▲──────────────────────────▲─────────────┘  │
│                  │ Wails bindings (typed)    │ LSP over IPC   │
│  ┌───────────────┴──────────┐   ┌────────────┴─────────────┐  │
│  │  app/ services (Go)      │   │  katafighter-lsp (Go)    │  │
│  │  project, undo, files,   │   │  diagnostics, completion │  │
│  │  image import, launcher  │   │  hover, go-to-definition │  │
│  └───────────────┬──────────┘   └────────────┬─────────────┘  │
│                  └──────────────┬─────────────┘               │
│                  ┌──────────────┴──────────────┐              │
│                  │  katafighter-formats (Go)   │ ← extracted   │
│                  │  DEF · SFF · AIR · CNS/ST · │   from        │
│                  │  CMD · ZSS · SND · ACT      │   Ikemen GO   │
│                  └─────────────────────────────┘              │
└──────────────────────────────────────────────────────────────┘
                 │ "Test" button
                 ▼
          Ikemen GO executable (user-configured path)
```

### Repository layout (monorepo, Go workspace)

```
KataFighter/
├─ go.work
├─ README.md
├─ AGENTS.md                    # rules for contributors & AI agents
├─ LICENSE                      # MIT
├─ CREDITS.md                   # third-party credits and license notices (Ikemen GO, icons)
├─ CONTRIBUTING.md
├─ formats/                     # module: katafighter-formats
│  ├─ def/  sff/  air/  cns/  cmd/  zss/  snd/  act/
│  └─ testdata/                 # golden files, sample chars (license-safe)
├─ lsp/                         # module: katafighter-lsp
│  ├─ cmd/katafighter-lsp/
│  └─ schema/                   # sctrl & trigger definitions (data-driven)
├─ app/                         # Wails v3 application
│  ├─ main.go
│  ├─ services/                 # project, sprite, anim, code, sound, test
│  └─ frontend/                 # Svelte + TS
├─ vscode-extension/            # thin client for katafighter-lsp
├─ docs/                        # this plan, upstream audit, design notes
└─ ikemen/                      # Ikemen GO reference material (not built)
   ├─ source/                   # git submodule, pinned tag (v1.0.0)
   └─ release/                  # official builds, git-ignored
```

---

## 5. Reusing Ikemen GO Code (Strategy)

Ikemen GO's source is MIT-licensed, but much of it lives in a single `package main`, so it cannot simply be imported as a library (verify against the current `develop` branch).

Plan:

1. **Audit** Ikemen GO `src/` and list the files needed: SFF loading/decoding (RLE8, RLE5, LZ5, PNG), AIR parsing, CNS/ZSS compiler & expression parser, command parsing, SND loading.
2. **Extract** those parts into `formats/` as clean packages, keeping the original copyright and MIT notice in `CREDITS.md` and file headers.
3. **Add writers** (encoders/serializers) — the engine only needs readers; the editor also needs to save.
4. **Track upstream**: record the Ikemen GO commit each package was extracted from; add a script to diff against newer upstream versions.
5. **Upstream proposal**: open a discussion with the Ikemen GO team about turning the extracted module into a shared library used by both projects.

**Exception: CNS / ZSS / CMD.** Ikemen's compiler parses and emits runtime bytecode in a single pass, with no syntax tree. We write our own tolerant parser instead, and generate the controller, parameter and trigger schema from Ikemen's source with `go/ast`. The reasoning is in [upstream-audit.md](upstream-audit.md#cns--zss--cmd-compiler-compilergo-84k-compiler_functionsgo-71k-bytecodego-156k-lines).

---

## 6. Feature Scope by Format

| Format | Read | Write | v1 editing features |
|---|---|---|---|
| DEF | ✅ | ✅ | Character info, file references, palettes list |
| SFF v2 | ✅ | ✅ | Add/replace/delete sprites, group/index, axis, linked sprites, palette assignment, PNG import/export |
| SFF v1 | ✅ | ❌ | Import and convert to SFF v2 |
| ACT | ✅ | ✅ | Palette editing, swap, generate from image |
| AIR | ✅ | ✅ | Action list, frames, timing, flip, blend, offsets, Clsn1/Clsn2 (default & per-frame), loopstart, scale/angle/interpolation |
| CNS / ST | ✅ | ✅ (text) | Code editor + LSP, state list outline, jump between state ↔ animation |
| ZSS | ✅ | ✅ (text) | Code editor + LSP |
| CMD | ✅ | ✅ (text) | Code editor + command visualizer |
| SND | ✅ | ✅ | Play, add/replace/delete sounds (WAV/OGG import) |

---

## 7. Roadmap

Each phase ends with a tagged release. Time estimates assume one part-time developer and are rough.

### Phase 0 — Foundations (1–2 weeks)
- [ ] Repo, MIT license, README, CONTRIBUTING, code of conduct (code of conduct left)
- [ ] Go workspace (`go.work`) with `formats`, `lsp`, `app` modules (`formats`, `app`, `tools` done; `lsp` not created yet)
- [x] Wails v3 hello-world app with Svelte + TS, pinned versions (v3.1.0; opens a `.def` and lists its files and problems)
- [ ] CI: build + test on Windows/macOS/Linux (`.github/workflows/ci.yml` added; tick once it is green)
- [ ] Collect a **test corpus**: Kung Fu Man + a few license-safe community characters (with permission) + intentionally broken files (KFM via `tools/fetchcorpus` and broken files done; community characters need permission)
- **Exit criteria:** CI green on all three OSes; app window opens.

### Phase 1 — `katafighter-formats` read-only (3–5 weeks)
- [x] Audit & extract Ikemen GO parsers (Section 5)
- [ ] DEF, SFF v2 (all compression modes), ACT, AIR, SND readers (DEF, SFF v1+v2, AIR done; ACT, SND left)
- [x] CNS/ST/CMD/ZSS parsing into an AST with source positions
- [x] Diagnostics model (errors/warnings with file:line:col)
- [x] CLI `kf inspect <char.def>` for debugging
- **Exit criteria:** every character in the corpus loads without panic; sprite decode matches Ikemen output (pixel-compare golden tests).

### Phase 2 — `katafighter-lsp` + VS Code extension (3–4 weeks) → **first public release**
- [ ] Data-driven schema of state controllers, triggers, parameters (incl. Ikemen-only ones)
- [ ] Diagnostics, completion, hover docs, go-to-definition (state numbers, anim numbers, sound IDs)
- [ ] Document outline (states list), find references
- [ ] VS Code extension published to Marketplace / Open VSX
- **Exit criteria:** usable daily for CNS/ZSS editing in VS Code.

### Phase 3 — Editor MVP: viewer (3–4 weeks)
- [ ] Open project from `.def`; project tree
- [ ] Sprite browser (group/index grid, zoom, axis display, palette switch)
- [ ] Animation player (play/pause/step, loop, Clsn overlay, onion skin)
- [ ] Monaco editor wired to the LSP for CNS/ST/CMD/ZSS
- [ ] Cross-navigation: click `anim = 200` → opens action 200
- **Exit criteria:** a creator can browse any corpus character comfortably.

### Phase 4 — Editor MVP: editing & saving (5–7 weeks) → **v0.1.0 "Kata"**
- [ ] Global undo/redo (command pattern in Go services)
- [ ] SFF writer: add/replace/delete sprites, PNG import (auto-index to palette), axis tool
- [ ] AIR editor: frame list, timing, offsets, flip/blend; **Clsn drawing tool** (draw, move, resize, copy to frames)
- [ ] Palette editor (ACT), palette remapping on import
- [ ] Lossless round-trip tests for all writers
- [ ] Autosave + crash recovery
- **Exit criteria:** create a simple playable character from PNGs without leaving KataFighter (except for testing in Ikemen).

### Phase 5 — Test integration (2–3 weeks)
- [ ] Configure Ikemen GO path; "Test" button launches Ikemen with the current character (training/debug mode via command-line options)
- [ ] Choose opponent, stage
- [ ] Capture Ikemen log output in a panel; map errors back to file:line
- **Exit criteria:** edit → test loop under 5 seconds.

### Phase 6 — Productivity features (ongoing)
- [ ] Sprite sheet slicer (grid & auto-detect)
- [ ] Batch operations (re-axis, re-palette, renumber groups)
- [ ] Sound editor (SND) with WAV/OGG import
- [ ] Command (CMD) visual builder
- [ ] State templates / snippets library (basic attack, special, super, throw)
- [ ] Character wizard (new character from template)
- [ ] i18n (English, Vietnamese first)

### Later / Ideas
- Stage editor, screenpack/motif editor, fonts
- Embedded OpenGL preview window reusing Ikemen's renderer (pixel-perfect blend/shader preview)
- Plugin system (Go or JS scripts)
- Lua tooling for Ikemen game modes

---

## 8. Testing Strategy

- **Golden tests:** decoded sprites compared byte-by-byte with Ikemen GO's own decoders, extracted from `ikemen/source` at test time (`formats/sff/oracle_test.go`), plus committed per-sprite hash manifests (`formats/testdata/golden/`) to catch regressions in the loader.
- **Round-trip tests:** read → write → read must produce identical data structures; text files byte-identical when unmodified.
- **Corpus tests:** run all parsers over the corpus in CI; no panics allowed, diagnostics snapshot-tested.
- **Fuzzing:** Go native fuzzing for binary readers (SFF, SND) to harden against malformed files.
- **Frontend:** Vitest for logic, Playwright smoke tests for key flows (open project, draw Clsn, save).

---

## 9. Community & Project Management

- GitHub Issues with labels: `formats`, `lsp`, `ui`, `good first issue`, `help wanted`.
- GitHub Projects board mirroring the phases above.
- Discussions for feature requests; announce on MugenGuild and the Ikemen GO Discord after Phase 2.
- Semantic versioning; CHANGELOG.md with Keep a Changelog format.
- Docs site (later): user guide + format reference.

---

## 10. Risks & Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Ikemen code is tightly coupled (`package main`, globals) | Extraction slower than expected | Extract incrementally, format by format; write adapters; talk to Ikemen devs early |
| Ikemen GO changes formats/features | Editor drifts from engine | Track upstream commits; CI job that diffs against latest Ikemen `develop` |
| Wails v3 is young | Breaking changes / platform bugs | Pin exact version; keep UI ↔ Go boundary thin so the shell is replaceable |
| Linux WebKitGTK differences | Rendering/perf issues | Test on Ubuntu LTS + Fedora in CI; Canvas fallback over WebGL |
| Dirty community files | Crashes, data loss | Tolerant parsers, fuzzing, never overwrite without backup |
| Scope creep | Never ship | Strict phase exit criteria; LSP shipped early as standalone value |
| Legal (assets in test corpus) | Takedown | Use KFM and characters with explicit author permission only |

---

## 11. Open Questions

- Svelte vs React for the frontend (contributor familiarity)?
- How does Ikemen GO want to be referenced (name usage, branding)? Ask the team.
- Should text files be edited via AST (structured) or text-only + LSP in v1? (Proposal: text + LSP in v1, structured editing later.)
- Minimum supported OS versions (Windows 10+? macOS 12+?).
- Name for releases (proposal: belts or kata names — "Shiro", "Kuro", ...).

---

## 12. Immediate Next Steps

1. ~~Create the GitHub repository `KataFighter` (MIT) and push this plan.~~ Done.
2. ~~Add Ikemen GO v1.0.0 as `ikemen/source` and audit SFF (Section 5).~~ Done, see `docs/upstream-audit.md`.
3. ~~SFF spike: extract the SFF reader into `formats/sff`, with a CLI that dumps Kung Fu Man's sprites to PNG.~~ Done: v1 + v2, `kf inspect` / `kf dump`.
4. ~~Build the first golden test: compare decoded sprites with Ikemen output.~~ Done: decoder oracle + hash manifests.
   - ~~AIR reader (`formats/air`) + shared text helpers.~~ Done.
5. ~~Set up `go.work`; scaffold Wails v3 + Svelte app.~~ Done (`app/`, Wails v3.1.0). `lsp` module still to create.
6. ~~DEF reader, shared diagnostics model, CNS/ST/CMD and ZSS parsers.~~ Done (`formats/def`, `diag`, `expr`, `cns`, `zss`).
7. Viewer slice (Phase 3): sprite browser, then animation player with Clsn overlay.
8. ACT and SND readers; CI (Phase 0); the `lsp` module (Phase 2).
