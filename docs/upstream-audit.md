# Upstream audit — Ikemen GO

↑ [docs](AGENTS.md) · Pinned version: [ikemen/](../ikemen/AGENTS.md) · Extraction rules: [formats/](../formats/AGENTS.md)

Source: `ikemen/source` (submodule), tag **v1.0.0**, commit `81c6da71d689625e20db79586815b695da00dd6d`. License: MIT (`LICENCE.txt`). Go 1.20, everything in `package main`.

## SFF (`src/image.go`, ~2300 lines)

### Can be copied almost verbatim (pure functions, only use `Sprite` fields)
| Function | Lines | Notes |
|---|---|---|
| `SffHeader.Read` | 1482 | v1 + v2 header |
| `Sprite.readHeader` / `readHeaderV2` | 897 / 1112 | v2 sets `rle = -format`; `ofs += lofs` or `tofs` by flag bit 0 |
| `Rle8Decode`, `Rle5Decode`, `Lz5Decode` | 1162–1309 | Lenient: indices clamped, never panic on short input |
| `RlePcxDecode`, `readPcxHeader` | 964 / 922 | SFF v1 |
| `Sff.ReadPalette` | 2145 | v2.0.0.0 forces idx0 transparent, rest opaque |
| `readActPalette` | 613 | ACT files |

### Coupled to the engine (must be rewritten)
- `Sprite.SetPxl` / `SetRaw` send decoded pixels to the GPU via `sys.mainThreadTask` + `gfx.newTexture`; pixels are **not kept in memory**. → Extracted version stores `[]byte` / RGBA in the struct.
- `Sprite.shareCopy` (linked sprites) uses `sys.mainThreadTask`.
- `loadSff` uses `findActiveSff`, `loadingCanceled` (`sys`), `OpenFile` (common.go:1673), `chk` (main.go:28), `LogMessage` (common.go:867).
- `loadPalettes` writes into `PaletteList` (PalTable, numcols, duplicatePals) — keep the data, drop the runtime remap logic.
- `readV2` line ~1346: `panic(err)` on short read → must return an error (tolerant parsing).

### SFF v1 caveats
`Sprite.read` uses heuristics (scan backwards for `0x0C` palette marker; special case for the character's first / 0,0 sprite). Copy as-is and cover with golden tests.

## Corpus available in the v1.0.0 release (`ikemen/release/`)
`chars/kfm`, `kfm720`, `kfm_zss`, `kfm_zaxis`: main `.sff` are **v2.0.1**, `intro.sff`/`ending.sff` are **v1.0.1**. ZSS samples in `kfm_zss`, `data/*.zss`.

## Status: SFF extracted (spike done, 2026-11-19)

Ported to `formats/sff`:
- `decode.go`: the four decoders, logic unchanged.
- `sff.go`: header, sprite and palette loading for v1 and v2, plus linked sprites.
- `image.go`: export to `image.Image`.

Deliberate differences from upstream are listed in the header of `sff.go`:
- Pixels are kept in memory.
- A bad sprite becomes a warning instead of aborting the load.
- No panic on a short read.
- Decoder panics are recovered.
- `isActPal` is not ported.

Verified:
- All 49 `.sff` files in the v1.0.0 release load with 0 warnings: chars, stages, fonts and `data/`, both v1 and v2.
- KFM sprites, palette swaps, the PNG8 portrait and the v1 PCX intro export to PNG and look correct.
- 30 s of fuzzing `Load` with no crash.

Not yet verified: pixel-exact comparison against Ikemen's own rendering (golden tests, [PLAN §8](PLAN.md#8-testing-strategy)).

## AIR (`src/anim.go`, ~2400 lines)

Only lines 1–475 and 1160–1277 handle parsing. The rest is playback and drawing, which the editor will reimplement for preview.

| Function | Lines | Coupling |
|---|---|---|
| `ReadAnimFrame` | 45 | Pure |
| `ReadAnimation` | 274 | Pure except for the `*Sff` / `*PaletteList` pointers, which the parser doesn't need |
| `ReadAction` | 439 | Pure |
| `AnimationTable.readAction` | 1171 | Writes warnings through `LogMessage` |
| `resolveCopyAction` | 1238 | Pure |
| Character AIR + common AIR merge | `char.go:4285–4342` | Uses `sys.cfg.Common.Air` and file search paths |
| `Atoi`, `Atof`, `IsNumeric`, `SectionName`, `SplitAndTrim` | `common.go` | `Atoi` logs overflow through `sys` |

Quirks we preserve (each one has a test in `formats/air/air_test.go`):
- An empty action takes the frames of the next action in the file.
- When an action number appears twice, the first one wins.
- A `Copy Action` chain is followed for at most 8 links. If it cannot be resolved, the action is removed.
- A `-1` frame in the middle of an action resets the timing counters.
- A blank scale or angle means "unchanged" (MUGEN 1.1 reads it as 0).
- `H` and `V` flags negate the engine offset.
- When a Clsn block has fewer boxes than it declares, the missing boxes are 0,0,0,0.
- Clsn defaults apply to every frame that has no explicit Clsn of its own.

`data/common.air` references actions that live in the character's AIR (`Copy Action 20`, `Copy Action 43`). It only resolves after it is merged into a character's AIR, so loading it on its own reports two warnings. This is expected.

## Status: AIR extracted (2026-11-19)

Ported to:
- `formats/internal/text`: the shared MUGEN number and section helpers, which CNS, CMD and DEF will reuse.
- `formats/air`: `Parse`, `Merge`, `Resolve` and `Load`.

Every action and frame records its source line. Warnings become `Diagnostics`. All frame errors are kept, where upstream keeps only the last one.

Verified:
- All 8 `.air` files in the v1.0.0 release parse.
- `kfm.air` has 117 actions and 655 frames, with 0 diagnostics.
- After merging `common.air`, every frame of KFM points at a sprite that exists in `kfm.sff`.

Not yet: an AIR writer (lossless round-trip, Phase 4).

Next to audit: DEF / CNS / ZSS (`char.go`, `compiler.go`).
