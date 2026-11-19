# app/: KataFighter desktop app

↑ [Root](../AGENTS.md) · Roadmap: [Phases 3–5](../docs/PLAN.md#7-roadmap) · Architecture: [PLAN §4](../docs/PLAN.md#4-architecture)

Wails v3 application: Go services in `services/`, Svelte 5 + TypeScript frontend in `frontend/`. Not scaffolded yet.

## Rules

- Pin the exact Wails v3 version in `go.mod`.
- The UI talks to the core only through typed Wails bindings in `services/`. The frontend does no format parsing.
- Format logic comes from [formats/](../formats/AGENTS.md). Code editing features come from [lsp/](../lsp/AGENTS.md).
