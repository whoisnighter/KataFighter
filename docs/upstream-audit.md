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

### Golden tests (2026-11-20)

- **Decoder oracle** (`formats/sff/oracle_test.go`): `RlePcxDecode`, `Rle8Decode`, `Rle5Decode` and `Lz5Decode` are pulled out of `ikemen/source/src/image.go` with `go/ast` and built in a temp dir next to a stub `Sprite`. Every compressed sprite of the release (76 PCX, 631 RLE8, 992 LZ5) is decoded by both and compared byte for byte: all match. Breaking one of our decoders on purpose makes the test fail. Nothing from Ikemen is committed.
- **Manifests** (`formats/testdata/golden/sff/`): for each of the 49 release `.sff`, sprite keys, sizes, axes, formats, palette links and a hash of the decoded pixels. They catch regressions in the loader logic around the decoders (v1 palette heuristics, linked sprites, PNG paths), which the oracle does not cover.

Not covered: Ikemen's GPU-side palette application. RLE5 does not occur in the corpus.

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

## ACT and SND (`src/image.go`, `src/sound.go`, `src/char.go`)

| Function | Where | Notes |
|---|---|---|
| `readActPalette` | `image.go:613` | Pure. Reads up to 768 bytes and accepts short files. Colors are stored **in reverse** (the first color is index 255). Index 0 is transparent |
| `Char.loadPalettes` | `char.go:4385` | Engine state. SFF v1: the ACT files are the palettes. SFF v2: `palN` replaces SFF palette 1,N when the ACT loads, and falls back to the SFF palette when it does not |
| `LoadSnd` / `LoadSndFiltered` | `sound.go:793`, `:810` | `ElecbyteSnd\0` header, then a chain of 16-byte subheaders (next, length, group, number) followed by the WAV. Skips negative keys and duplicate keys (logged). A subheader that cannot be read fails the whole file |
| `readSound` | `sound.go:706` | Rejects sizes under 128 bytes, then decodes the WAV to the end with `beep`. A sound that fails to play is disabled and logged in the console |

### Status: ACT and SND extracted (2026-11-20)

- `formats/act`: `readActPalette` ported as is. `Short()` reports files with fewer than 256 colors, which Ikemen accepts without a warning.
- `formats/snd`: header, subheader chain, the negative and duplicate key rules, and the size check. The WAV data is kept as bytes and its RIFF header is parsed (`ParseWAV`). **Not ported:** the full `beep` decode test, so a sound whose header is valid may still be disabled by Ikemen. An unreadable subheader is an error diagnostic, and the sounds read before it are kept. A subheader chain that loops back is cut short with a warning.
- `Char.loadPalettes` is mirrored in the app's palette list (`paletteOptions` in `app/frontend/src/lib/sprites.ts`): slot N is the DEF's ACT file when one loads, otherwise SFF palette 1,N.
- Verified: all 9 `.snd` files in the v1.0.0 release load with 0 diagnostics (KFM: 12 sounds). The release has no ACT files, so ACT is covered by unit tests only.

## DEF and the CNS constant sections (`char.go:3747–4385`, `common.go`)

`Char.load` reads the DEF, then the `[Data]`, `[Size]`, `[Velocity]`, `[Movement]`, `[Quotes]`, `[Constants]` and `[RemapPreset X]` sections of the CNS file, writing straight into engine state (`sys.cgi`, `Char`). The parsing mechanics underneath are small and pure:

| Function | Where | Notes |
|---|---|---|
| `ReadIniSection`, `IniSection.Parse` | `common.go:947`, `:964` | Keys are lower-cased. **The first key wins** inside a section. `key value` with no `=` gives an empty value. `;` starts a comment. |
| `IniSection.ReadI32` / `ReadF32` / `ReadBool` / `getText` | `common.go:995–1142` | Comma lists; an empty item leaves the default unchanged; `getText` requires quotes |
| `SearchFile`, `LoadFile` | `common.go:539`, `:651` | Search order: the DEF's directory, then `""`, then `data/`. Accepts `\` and quotes. Supports zip paths. Case-insensitive fallback |
| `decodeShiftJIS` | `common.go:433` | Values that are not valid UTF-8 are decoded as Shift_JIS (needs `golang.org/x/text`) |

DEF section rules:
- **The first section of each kind wins.** A `<language>.info`-style section overrides the default once.
- In `[Files]`, `st`, `st1`…`stN` are sorted naturally. `cmd` and `stcommon` are read by `Compile` (`compiler.go:8129`), not by `Char.load`.
- `pal1..palN`, `fontN` and `fontN.height` live in `[Files]`. `[Palette Keymap]` maps `a`..`z2` to palette slots.
- Common files (`common.air`, `.cmd`, `.const`, `stcommon`) come from `sys.cfg.Common.*`, i.e. Ikemen's config, not from the character.

**Verdict: easy.** Port the INI helpers into `internal/text` and build a `def` package that returns a plain struct (info, files, palettes, keymap) plus the CNS constant sections. ~1 day.

### Status: DEF extracted (2026-11-20)

- `formats/internal/text/ini.go`: `ReadIniSection`, `IniSection.Parse`, `ReadI32/F32/Bool`, `getText`, `decodeShiftJIS`, `compareNatural`. Sections keep every entry with its line and column, duplicates included, so they can be reported.
- `formats/def`: one pass that merges what `Char.load` and `Compile` read. Language sections follow `Char.load` (a localized section applies once and blocks the plain one); `MugenVersion`, `IkemenVersion`, `cmd`, `st*` and `stcommon` come from the first plain section, as in `Compile`. `Find` mirrors `SearchFile` (DEF folder, engine folder, `data/`, case-insensitive fallback, no zip). `FindState` adds the `.zss` fallback of `stateCompile` (KFM's `common1.cns` is `data/common1.cns.zss`).
- Diagnostics for what Ikemen silently accepts: unquoted names, duplicate keys and sections, keymap slots outside 1..100.
- The CNS constant sections are read by `formats/cns` as plain sections; nothing interprets them yet.
- Verified: the 4 KFM characters load with 0 diagnostics and every referenced file resolves.

## CNS / ZSS / CMD compiler (`compiler.go` 8.4k, `compiler_functions.go` 7.1k, `bytecode.go` 15.6k lines)

### Structure
- **Parsing and code generation are a single pass.** The compiler never builds a syntax tree. It reads text and emits bytecode opcodes into `BytecodeExp` straight away: 938 distinct `OC_*` opcodes and roughly 800 `append` calls.
- `expValue` (`compiler.go:1272–5323`) is one big `switch c.token` with **about 280 trigger cases**. Inside them are nested `switch`es for keyword arguments such as `gethitvar(damage)`, about 1000 `case`s in total. Each case emits opcodes.
- `scmap` (`compiler.go:36`) maps **159 state controller names** to their compile functions in `compiler_functions.go`: 91 MUGEN and 68 Ikemen.
- `triggerMap` (`compiler.go:203`) lists 276 trigger names: 13 redirections, 142 MUGEN and 121 Ikemen.

*Correction (2026-11-19): an earlier version of this audit said 435 controllers and 1047 trigger cases. Those counts mixed `scmap` with `triggerMap`, and top-level cases with nested ones. The numbers above come from `tools/schemagen`.*
- The output types (`StateBytecode`, `StateControllerBase`, `BytecodeExp`) are defined in `bytecode.go`, and their `Run` methods operate on `*Char`. So the output only means something inside the running engine.
- Coupling to engine globals:
  - `compiler.go` has 102 `sys.*` references, mostly `sys.ignoreMostErrors` (44) and `sys.cgi` (26).
  - `compiler_functions.go` has 24.
  - `Compile` also creates the player's `CommandList` (CMD), so it is tied to `input.go`.
- CNS and ZSS share the expression compiler and the controller table. ZSS adds a block parser (`stateBlock`, `subBlock`, `switchBlock`, `loopBlock`, `callFunc`, `letAssign`: `compiler.go:7366–7965`) and is strict: `ignoreMostErrors` is forced to false.

### Expression grammar (precedence, loosest first)
`||` → `^^` → `&&` → `|` → `^` → `&` → `=` `!=` (and `= [a,b]` / `(a,b)` ranges) → `<` `<=` `>` `>=` → `+` `-` → `*` `/` `%` → `**` → unary `!` `~` `-` → values and triggers (`expValue`). Tokenizer: `compiler.go:485–675`.

### What the editor needs
The editor needs these things:
- A syntax tree with source positions.
- Diagnostics.
- Completion and hover.
- Go-to-definition.
- Lossless round-trip.

It never needs bytecode. The compiler offers none of the first four: no tree, no positions beyond a line index, and errors are free-form strings.

### Options considered
| Option | Verdict |
|---|---|
| A. Copy the compiler (with `bytecode.go`) | ❌ 31k lines. The output is runtime structs bound to `*Char`. We would carry the engine and still have no syntax tree. |
| B. Build Ikemen as a library | ❌ It is all `package main` and needs cgo (SDL2, OpenGL/Vulkan, FFmpeg). |
| **C. Own parser + generated schema + oracle** | ✅ Recommended (see below) |

### Recommended strategy (C)
1. **Own parser** (`formats/cns`, `formats/zss`): a tolerant parser that builds a syntax tree and keeps positions, comments and trivia, so files round-trip.
   - Use `internal/text` for numbers and INI rules.
   - Port the tokenizer and the precedence chain above as they are, since they are small and pure, but build tree nodes instead of opcodes.
2. **Generated schema** (`tools/schemagen`, a `go/ast` program run against `ikemen/source`):
   - State controller names come from `scmap`.
   - Their parameter names come from the string literals in `paramValue`, `stateParam` and `param*` calls. There are 33 + 158 + 28 call sites, and most give the mandatory flag. `paramValue` also gives type and count.
   - Trigger names come from the `case` labels of `expValue`.
   - Output: `lsp/schema/*.json`. The generator is re-run when the pinned version changes, so the editor follows the engine without hand-maintained lists. Hover docs are written by hand on top of this.
3. **Fidelity oracle** (later, optional): a test harness that copies `ikemen/source/src` into a temp dir, adds a `_test.go` that calls `newCharCompiler().Compile`, and checks that our parser accepts exactly what Ikemen accepts on the corpus and on fuzzed snippets. This needs the cgo toolchain, so it runs in CI on Linux only.

### Status: schema generated (2026-11-19)

`tools/schemagen` writes `lsp/schema/ikemen.json` (~250 KB):
- 159 controllers with 2065 parameters. Parameters come with name, type and count from `VT_*` when known, and the mandatory flag.
- 284 triggers with 571 keywords.

The generator follows helper calls such as `paramTrans(prefix)` and `hitDefSub`, with string arguments bound, and expands `range []string{…}` loops.

Known gaps, which are listed in the JSON:
- 18 parameter names are dynamic by design: `varset var(N)=`, `mapset` keys, `helper` size keys, `dialogue textN`.
- `projcontact`, `projguarded` and `projhit` have no `case`. They are matched by prefix (`projcontact1234`, the legacy ID suffix) in `expValue`'s default branch.
- 8 triggers are accepted by `expValue` but are not in `triggerMap` (`statetime`, `hitdefvar`, `debugmode`…). They are marked `origin: "other"`.

**Effort:** DEF ~1 day. schemagen ~2–3 days. CNS parser + expressions ~1–2 weeks. ZSS blocks +1 week. CMD (`input.go` `ReadCommandSymbols`) has not been audited yet.

### Status: parsers written (2026-11-20), strategy C step 1

- `formats/expr`: the tokenizer (`tokenizerCS`) and the `isOperator` precedence chain, building nodes. Trigger-specific shapes are parsed by generic rules so the parser needs no trigger list:
  - a name directly followed by a name is `Sub` (`vel x`, `p2bodydist y`);
  - a redirection keyword followed by `,` is `Redirect` (`parent, time`, `helper(1), pos x`);
  - `= [a, b]` / `(a, b)` is `Range`, and `(` without a comma falls back to parentheses, as in `expRange`;
  - `x = y, ...` is `Compare` (`animelem = 2, >= 3`, `hitdefattr = SC, NA`). Inside parentheses or arguments an item is only taken when it starts with a comparison operator or is an attack attribute code, which is what `evaluateComparison` and `trgAttr` accept there;
  - `:=` is `Assign`, with its value parsed at the `=` level like `_var`.
- `formats/cns`: CNS / ST / CMD. Section grouping follows `stateCompileCNS`, entries follow `parseSection` with `ignoreMostErrors` (top-level `=`, `var(N) =` keys, names cut at the first space). Fatal Ikemen errors (missing `type`, missing `trigger1`, empty or unparsable trigger) are error diagnostics; ignored problems are warnings.
- `formats/zss`: the whole file is tokenized once with `#` comments. Sections, params, `if/else`, `switch`, `for`, `while`, `let`, `call`, `break/continue` and the `ignorehitpause` / `persistent(N)` attributes follow the functions listed in the package header. Any name followed by `{` is a controller.
- Not done: the rule that a top-level ZSS statement must end its line, the MUGEN "reverse order" quirks of `expPostNot`, `[StateDef const(...)]` resolution, and checking names against the schema (that is the LSP's job). The fidelity oracle (step 3) is still open.
- Verified on the release: `kfm.cns` (58 states, 318 controllers), `kfm.cmd`, the `.const` files and all 16 `.zss` files (KFM ZSS + `data/`) parse with 0 diagnostics; every trigger name in `kfm.cns`/`kfm.cmd` is in `lsp/schema/ikemen.json`. `expr` and `zss` survive 20–30 s of fuzzing.
