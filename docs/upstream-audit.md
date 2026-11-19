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

## Verdict
SFF extraction looks feasible (~1–2 days): decoders are self-contained; only the texture/loader layer needs replacing.
