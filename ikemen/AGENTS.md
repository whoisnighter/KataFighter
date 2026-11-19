# ikemen/ — Ikemen GO reference material

Read-only reference. Nothing in this folder is built or imported by KataFighter.

- `source/`: Ikemen GO source as a git submodule, pinned to tag **v1.0.0** (commit `81c6da7`). Copy code from here into `formats/`, and record what you took in `docs/upstream-audit.md`. Don't edit it. To bump the version, check out a new tag in the submodule, then re-audit.
- `release/`: official release builds from https://github.com/ikemen-engine/Ikemen-GO/releases, unzipped here (git-ignored). Used for the KFM test corpus (`chars/kfm*`), for reference output in golden tests, and for the "Test" button.
