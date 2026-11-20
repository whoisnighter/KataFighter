# ikemen/: Ikemen GO reference material

↑ [Root](../AGENTS.md) · Used by: [formats/](../formats/AGENTS.md) · Audit: [upstream-audit.md](../docs/upstream-audit.md)

Read-only reference. Nothing here is built or imported.

## Pinned version (owned here)

**Ikemen GO v1.0.0**, commit `81c6da71d689625e20db79586815b695da00dd6d`. To bump it: check out a new tag in `source/`, update this section, re-run [schemagen](../tools/AGENTS.md), then re-audit.

## source/

Ikemen GO source as a git submodule. Don't edit it.

## release/ (git-ignored)

Official builds from https://github.com/ikemen-engine/Ikemen-GO/releases, unzipped here, e.g. `release/Ikemen_GO-v1.0.0-windows/`. `cd tools && go run ./fetchcorpus` downloads that one (URL and SHA-256 pinned in [tools/fetchcorpus](../tools/AGENTS.md); update them when bumping the version). Used for:
- the KFM test corpus (`chars/kfm*`: main `.sff` files are v2, `intro`/`ending` are v1)
- reference output for golden tests
- the "Test" button ([Phase 5](../docs/PLAN.md#phase-5--test-integration-23-weeks))
