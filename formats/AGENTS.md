# formats/: katafighter-formats

↑ [Root](../AGENTS.md) · Roadmap: [Phase 1](../docs/PLAN.md#phase-1--katafighter-formats-read-only-35-weeks)

Go readers/writers for DEF, SFF, AIR, CNS/ST, CMD, ZSS, SND, ACT. Used by [app/](../app/AGENTS.md) and [lsp/](../lsp/AGENTS.md).

## Extraction rules (owned here)

- Copy code from `ikemen/source` at the pinned version (see [ikemen/](../ikemen/AGENTS.md)). Never import it.
- Every extracted file keeps the Ikemen GO MIT copyright header and notes the upstream file and commit it came from.
- Before extracting, read [upstream-audit.md](../docs/upstream-audit.md). After extracting, update it.

## Parsing rules (owned here)

- No UI or GPU dependencies. Decoded sprites stay in memory as `[]byte` / RGBA.
- Tolerant parsing: never panic on malformed community files. Return errors or diagnostics with file:line:col.
- Writers must round-trip losslessly. See [Testing Strategy](../docs/PLAN.md#8-testing-strategy).

## Packages

| Path | What |
|---|---|
| `diag/` | Shared diagnostics model: severity, code, message, `file:line:col` range. **Every reader reports through it** |
| `def/` | DEF reader: `[Info]`, `[Files]` (st order, pal/font/movelist), `[Palette Keymap]`, `[Map]`, language sections; `Find` / `FindState` resolve paths like Ikemen |
| `sff/` | SFF v1 + v2 reader: sprites, palettes, linked sprites, export to `image.Image` |
| `air/` | AIR reader: actions, frames, Clsn boxes, Copy Action, timing; `Parse` → `Merge` (common AIR) → `Resolve` |
| `expr/` | Trigger/parameter expression lexer + parser → syntax tree with byte offsets. Generic: knows grammar keywords (redirections, attack attribute codes), never trigger names |
| `cns/` | CNS / ST / CMD parser: sections, `[Statedef]` → controllers → triggers (parsed with `expr`), `[Command]`; diagnostics for what Ikemen rejects or ignores |
| `zss/` | ZSS parser: `[StateDef]` / `[Function]`, controllers, if/else, switch, for, while, let, call, block attributes |
| `internal/text/` | MUGEN-style `Atoi`/`Atof`/`SectionName` and INI sections (first key wins, Shift_JIS, natural sort) from Ikemen. **Every text parser must use these**, never `strconv`, so numbers parse like the engine |
| `brokentest/` | Runs every reader over `testdata/broken/` and snapshot-tests the diagnostics (`.diag` files) |
| `cmd/kf/` | Debug CLI: `kf inspect <file.def\|.sff\|.air\|.cns\|.cmd\|.zss>` (a DEF loads and checks the whole character), `kf dump <file.sff> <outdir>` (`-char` for a character's main SFF, `-pal N` to pick a palette) |

Parsers check syntax and engine rules only. Names of controllers and triggers are checked against [lsp/schema](../lsp/AGENTS.md) by the LSP, not here.

```sh
go test ./...                                     # corpus tests skip if ikemen/release is empty
go test ./sff -run Golden -update                 # after an intended change to what sff.Load returns
go test ./sff -run '^$' -fuzz FuzzLoad -fuzztime 30s   # also FuzzParse in ./expr and ./zss
go run ./cmd/kf dump -char ../ikemen/release/<build>/chars/kfm/kfm.sff out/
```

## testdata/

Golden files and license-safe samples only. The KFM corpus is read from `ikemen/release/` and is not committed.

- `broken/`: hand-written malformed files (DEF, AIR, CNS, ZSS, SFF, random bytes). Each has a `.diag` snapshot; readers must never panic on them. Regenerate with `go test ./brokentest -update`.
- `golden/sff/`: one manifest per release `.sff` (sprite keys, sizes, axes, formats, palette links and a hash of the decoded pixels). Hashes only, never pixels.
- The decoder oracle (`sff/oracle_test.go`) builds Ikemen's own decoders from `ikemen/source` in a temp dir; it needs the submodule and the `go` tool.
