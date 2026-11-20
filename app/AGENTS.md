# app/: KataFighter desktop app

↑ [Root](../AGENTS.md) · Roadmap: [Phases 3–5](../docs/PLAN.md#7-roadmap) · Architecture: [PLAN §4](../docs/PLAN.md#4-architecture)

Wails v3 application: Go services in `services/`, Svelte 5 + TypeScript frontend in `frontend/`. Go module `github.com/whoisnighter/KataFighter/app`, part of the root `go.work`.

## Rules

- Wails is pinned to **v3.1.0** in `go.mod`, and `@wailsio/runtime` to the same version in `frontend/package.json`. Bump both together, along with the `wails3` CLI.
- The UI talks to the core only through typed Wails bindings in `services/`. The frontend does no format parsing.
- Format logic comes from [formats/](../formats/AGENTS.md). Code editing features come from [lsp/](../lsp/AGENTS.md).
- `frontend/bindings/` is generated on every build (`wails3 generate bindings`) and is not committed.
- Styles use the CSS variables in `frontend/public/style.css`.

## Layout

| Path | What |
|---|---|
| `main.go` | App and main window |
| `services/` | `ProjectService`: open dialog for a `.def`, `Open` loads the character through `formats` and returns files + problems |
| `frontend/src/App.svelte` | Shell: open a character, file list, problems |
| `build/` | Wails build assets (icons, Windows/macOS/Linux packaging). Metadata lives in `build/config.yml`; run `wails3 task common:update:build-assets` after editing it |

## Build and run

Needs Go, Node.js and the Wails CLI (`go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0`; check with `wails3 doctor`).

```sh
cd app
wails3 dev          # hot-reload development
wails3 build        # bin/katafighter(.exe)
go test ./...       # service tests; corpus tests skip if ikemen/release is empty
```
