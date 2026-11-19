# formats/ — katafighter-formats

Go readers/writers for DEF, SFF, AIR, CNS/ST, CMD, ZSS, SND, ACT, extracted from Ikemen GO (`ikemen/source`, tag v1.0.0). Phase 1 in `docs/PLAN.md`.

Rules:
- No UI or GPU dependencies. Decoded sprites are kept in memory as `[]byte` / RGBA.
- Extracted files keep the Ikemen GO MIT copyright header and note the upstream file + commit they came from.
- Tolerant parsing: never panic on malformed input; return errors / diagnostics.
- Read `docs/upstream-audit.md` before extracting; update it when you extract something new.
- `testdata/`: golden files and license-safe samples only. The KFM corpus comes from `ikemen/release/` (not committed).
