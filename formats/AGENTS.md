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
| `sff/` | SFF v1 + v2 reader: sprites, palettes, linked sprites, export to `image.Image` |
| `cmd/kf/` | Debug CLI: `kf inspect <file.sff>`, `kf dump <file.sff> <outdir>` (`-char` for a character's main SFF, `-pal N` to pick a palette) |

```sh
go test ./...                                     # corpus tests skip if ikemen/release is empty
go test ./sff -run '^$' -fuzz FuzzLoad -fuzztime 30s
go run ./cmd/kf dump -char ../ikemen/release/<build>/chars/kfm/kfm.sff out/
```

## testdata/

Golden files and license-safe samples only. The KFM corpus is read from `ikemen/release/` and is not committed.
