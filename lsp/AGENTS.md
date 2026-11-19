# lsp/: katafighter-lsp

↑ [Root](../AGENTS.md) · Roadmap: [Phase 2](../docs/PLAN.md#phase-2--katafighter-lsp--vs-code-extension-34-weeks--first-public-release)

Language server for CNS/ST/CMD/ZSS: diagnostics, completion, hover, go-to-definition. Clients: [vscode-extension/](../vscode-extension/AGENTS.md) and [app/](../app/AGENTS.md) (Monaco).

## Rules

- Parse through [formats/](../formats/AGENTS.md). Don't duplicate parsers here.
- State controllers, triggers and parameters live as data in `schema/`, not in code.
