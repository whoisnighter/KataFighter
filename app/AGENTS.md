# app/ — KataFighter desktop app

Wails v3 application: Go services in `services/`, Svelte 5 + TypeScript frontend in `frontend/`. Not scaffolded yet (Phase 0 / Phase 3+ in `docs/PLAN.md`).

Rules:
- Pin the exact Wails v3 version in `go.mod`.
- The UI talks to the core only through typed Wails bindings in `services/`; no format parsing in the frontend.
- Import format logic from `formats/`, never from `ikemen/`.
