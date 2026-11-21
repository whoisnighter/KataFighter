# lsp/: katafighter-lsp

↑ [Root](../AGENTS.md) · Roadmap: [Phase 2](../docs/PLAN.md#phase-2--katafighter-lsp--vs-code-extension-34-weeks--first-public-release)

Go module `github.com/whoisnighter/KataFighter/lsp`. Language server for CNS/ST/CMD/ZSS. Clients: [vscode-extension/](../vscode-extension/AGENTS.md) and [app/](../app/AGENTS.md) (Monaco). Done so far: diagnostics. Next: completion, hover, go-to-definition.

## Rules

- Parse through [formats/](../formats/AGENTS.md). Don't duplicate parsers here.
- State controllers, triggers and parameters come from `schema/`, never from hard-coded lists.
- Severities follow Ikemen: an error means Ikemen refuses to load the character, and a warning means Ikemen logs the problem and carries on.
- No false positives on the corpus. `check/corpus_test.go` runs every state file in the release, and the warnings it expects are real KFM quirks. If a new check fires on the corpus, the check is wrong, unless Ikemen reports the same problem.

## Packages

| Path | What |
|---|---|
| `schema/` | Embeds `ikemen.json` and looks up controllers, parameters and triggers (`projhit1200` resolves to `projhit`) |
| `check/` | Checks names that the parsers leave to the LSP: unknown controller type (error), unknown parameter (warning; skipped for controllers with `unresolved` keys), unknown trigger (error). `Text` = parser + schema diagnostics for one file, by extension |
| `server/` | LSP over a byte stream: initialize/shutdown/exit, full text sync, `publishDiagnostics`. Converts byte columns to UTF-16 |
| `cmd/katafighter-lsp/` | The binary: stdio transport, logs to stderr, `-version` |

Trigger names that are deliberately not checked, because Ikemen reads them per trigger and the schema does not say how:
- bare names used as arguments: `map(combo)`, `const(StateDizzy)`, `inputtime(B)`;
- bare names after `=` / `!=` or in a comparison list: `statetype = S`, `hitdefattr = SC, NA`;
- the keyword after a trigger: `vel x`;
- the name at the very start of a parameter value, which may be a data prefix (`F5`, `S $bank`, `GO $anim`, `F(60 + n)`; see `getDataPrefix`).

Mandatory parameters are not checked yet: the `mandatory` flag doesn't record conditions (Width needs `value` only when `edge` and `player` are both absent).

```sh
go test ./...                            # corpus test skips if ikemen/release is empty
go run ./cmd/katafighter-lsp -version
```

## schema/

| File | What |
|---|---|
| `ikemen.json` | **Generated** by [tools/schemagen](../tools/AGENTS.md) from the pinned Ikemen source. Lists controllers (name, origin, params with type/count/mandatory) and triggers (name, origin, keyword args). Don't edit it by hand. |

Hover documentation will be a separate hand-written file keyed by the same names.
