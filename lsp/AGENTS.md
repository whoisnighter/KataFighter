# lsp/ — katafighter-lsp

Language server for CNS/ST/CMD/ZSS (diagnostics, completion, hover, go-to-definition). Phase 2 in `PLAN.md`.

Rules:
- Parse via `formats/`; do not duplicate parsers here.
- State controllers, triggers and parameters live as data in `schema/`, not hard-coded.
