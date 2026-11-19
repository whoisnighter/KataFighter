# lsp/: katafighter-lsp

↑ [Root](../AGENTS.md) · Roadmap: [Phase 2](../docs/PLAN.md#phase-2--katafighter-lsp--vs-code-extension-34-weeks--first-public-release)

Language server for CNS/ST/CMD/ZSS: diagnostics, completion, hover, go-to-definition. Clients: [vscode-extension/](../vscode-extension/AGENTS.md) and [app/](../app/AGENTS.md) (Monaco). The Go module is not created yet.

## Rules

- Parse through [formats/](../formats/AGENTS.md). Don't duplicate parsers here.
- State controllers, triggers and parameters come from `schema/`, never from hard-coded lists.

## schema/

| File | What |
|---|---|
| `ikemen.json` | **Generated** by [tools/schemagen](../tools/AGENTS.md) from the pinned Ikemen source. Lists controllers (name, origin, params with type/count/mandatory) and triggers (name, origin, keyword args). Don't edit it by hand. |

Hover documentation will be a separate hand-written file keyed by the same names.
