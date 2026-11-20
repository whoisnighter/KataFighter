# app/: KataFighter desktop app

↑ [Root](../AGENTS.md) · Roadmap: [Phases 3–5](../docs/PLAN.md#7-roadmap) · Architecture: [PLAN §4](../docs/PLAN.md#4-architecture)

Wails v3 application: Go services in `services/`, Svelte 5 + TypeScript frontend in `frontend/`. Go module `github.com/whoisnighter/KataFighter/app`, part of the root `go.work`.

## Rules

- Wails is pinned to **v3.1.0** in `go.mod`, and `@wailsio/runtime` to the same version in `frontend/package.json`. Bump both together, along with the `wails3` CLI.
- The UI talks to the core only through typed Wails bindings in `services/`. The frontend does no format parsing.
- Format logic comes from [formats/](../formats/AGENTS.md). Code editing features come from [lsp/](../lsp/AGENTS.md).
- `frontend/bindings/` is generated on every build (`wails3 generate bindings`) and is not committed.
- Styles use the CSS variables in `frontend/public/style.css`. Icons are the line icons in `frontend/src/lib/Icon.svelte`.
- The logo is imported from [docs/assets/logo.svg](../docs/assets/) (single source); don't copy it into `frontend/`. The app icon (`build/appicon.png`) is its PNG export; after changing it, run `wails3 task common:generate:icons`.
- Every feature must work in both modes (below). Native-only calls (file dialogs, window buttons) check `ProjectService.Mode()` and offer a browser fallback.

## Layout

| Path | What |
|---|---|
| `main.go` | App and the frameless main window (the title bar is drawn by the frontend) |
| `services/` | `ProjectService`: `Mode`, `ChooseDef` (native dialog), `Open` (loads a character through `formats`: files + problems) |
| `frontend/src/App.svelte` | Shell: title bar, Explorer, tab area, Problems panel, status bar, shortcuts |
| `frontend/src/lib/` | Shell parts: `TitleBar` (menus, window buttons), `Explorer`, `Welcome` (recent list), `Overview`, `Problems`, `StatusBar`, `QuickOpen` (path input) |
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
- `/?def=<absolute path to a .def>` opens a character directly.
- The server embeds `frontend/dist` at build time: restart the task after frontend changes.
