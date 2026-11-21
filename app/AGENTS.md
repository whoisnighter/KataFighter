# app/: KataFighter desktop app

↑ [Root](../AGENTS.md) · Roadmap: [Phases 3–5](../docs/PLAN.md#7-roadmap) · Architecture: [PLAN §4](../docs/PLAN.md#4-architecture)

Wails v3 application: Go services in `services/`, Svelte 5 + TypeScript frontend in `frontend/`. Go module `github.com/whoisnighter/KataFighter/app`, part of the root `go.work`.

## Rules

- Wails is pinned to **v3.1.0** in `go.mod`, and `@wailsio/runtime` to the same version in `frontend/package.json`. Bump both together, along with the `wails3` CLI.
- `go.mod` replaces `formats` and `lsp` with `../formats` and `../lsp`. The Wails build runs `go mod tidy`, which ignores `go.work`; without the replaces, the app would build against the last versions pushed to GitHub.
- The UI talks to the core only through typed Wails bindings in `services/`. The frontend does no format parsing.
- Format logic comes from [formats/](../formats/AGENTS.md). Code editing features come from [lsp/](../lsp/AGENTS.md): the editor calls its Go packages (`lsp/check`) through `EditorService`, not the LSP protocol. Monaco only draws; it gets no language services of its own.
- `monaco-editor` is pinned to an exact version in `frontend/package.json`. Only the editor core is bundled (`monaco-editor/editor/editor.api` + the editor worker), never the full `monaco-editor` entry, which would pull in the TypeScript/JSON/CSS workers.
- `frontend/bindings/` is generated on every build (`wails3 generate bindings`) and is not committed.
- Styles use the CSS variables in `frontend/public/style.css`. Icons are the line icons in `frontend/src/lib/Icon.svelte`.
- The logo is imported from [docs/assets/logo.svg](../docs/assets/) (single source); don't copy it into `frontend/`. The app icon (`build/appicon.png`) is its PNG export; after changing it, run `wails3 task common:generate:icons`.
- Every feature must work in both modes (below). Native-only calls (file dialogs, window buttons) check `ProjectService.Mode()` and offer a browser fallback.

## Layout

| Path | What |
|---|---|
| `main.go` | App and the frameless main window (the title bar is drawn by the frontend) |
| `services/` | `ProjectService`: `Mode`, `ChooseDef` (native dialog), `Open` (loads a character through `formats`: files + problems). `ViewerService`: `Sprites`, `SpriteImage` (PNG data URL, optionally recolored with an SFF palette or an ACT file), `Actions` (AIR, ready to draw); files are cached by path and mtime. `EditorService`: `Open` (text + version; non-UTF-8 files are read-only), `Save` (atomic, keeps the BOM, refuses if the file changed on disk since `Open`), `Check` (parser + schema diagnostics in Monaco columns) |
| `frontend/src/App.svelte` | Shell: title bar, Explorer, tab area, Problems panel, status bar, shortcuts |
| `frontend/src/lib/` | Shell parts: `TitleBar` (menus, window buttons), `Explorer`, `Welcome` (recent list), `Overview`, `Problems`, `StatusBar`, `QuickOpen` (path input). Viewers: `SpriteBrowser`, `AnimPlayer`, `PaletteSelect`; `sprites.ts` caches sprite images. Code editor: `CodeEditor` (one Monaco instance, checks 250 ms after typing), `monaco.ts` (CNS/ZSS syntax colors, theme read from the CSS tokens, one model per open file with its unsaved state) |
| `build/` | Wails build assets (icons, Windows/macOS/Linux packaging). Metadata lives in `build/config.yml`; run `wails3 task common:update:build-assets` after editing it |

## Build and run

Needs Go, Node.js and the Wails CLI (`go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0`; check with `wails3 doctor`).

```sh
cd app
wails3 dev          # desktop window with hot reload
wails3 build        # bin/katafighter(.exe)
wails3 task browser # browser mode, see below
go test ./...       # service tests; corpus tests skip if ikemen/release is empty
```

## Browser mode (agents, headless testing)

`wails3 task browser` builds with `-tags server` and serves the whole app, Go services included, at **http://localhost:34115** (`BROWSER_PORT=xxxx` to change). No native window opens and nothing takes focus, so an agent or a test runner can drive the real app from any browser and take screenshots.

- `Mode()` returns `"browser"`: the window buttons are hidden and *File → Open Character…* (Ctrl+O) shows a path box instead of the native dialog.
- `/?def=<absolute path to a .def>` opens a character directly; add `&view=sprites` or `&view=anim` to open a viewer tab, or `&file=<absolute path>&line=N` to open a file in the code editor.
- Don't save edits to files under `ikemen/release/`: it is the test corpus. Copy a character elsewhere to test saving.
- The server embeds `frontend/dist` at build time: restart the task after frontend changes.
