# Contributing to KataFighter

Thanks for helping. This page covers setup and the workflow. Project rules live in the `AGENTS.md` files (one per folder, starting with the [root one](AGENTS.md)); they apply to everyone, with or without an AI agent.

## Setup

Requirements:

- **Go 1.27** or newer
- **Node.js 24** with npm (for the app frontend)
- **Wails CLI v3.1.0**: `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0`, then `wails3 doctor`
- On Linux, the WebKitGTK headers: `libgtk-3-dev libwebkit2gtk-4.1-dev`

```sh
git clone --recurse-submodules https://github.com/whoisnighter/KataFighter.git
cd KataFighter
cd tools && go run ./fetchcorpus     # Ikemen GO release used by the tests
```

If you cloned without submodules: `git submodule update --init --depth 1`.

## Running the tests

The repository is a Go workspace (`go.work`). Run the tests in each module:

```sh
cd formats && go vet ./... && go test ./...
cd tools   && go vet ./... && go test ./...
cd app     && wails3 build && go test ./... && (cd frontend && npm run check)
```

Corpus tests skip when `ikemen/release` is empty, so run `fetchcorpus` once. CI runs all of this on Windows, macOS and Linux for every push and pull request ([ci.yml](.github/workflows/ci.yml)).

Useful extras:

```sh
cd formats
go test ./sff -run Golden -update      # after an intended change to sprite loading
go test ./brokentest -update           # after an intended change to diagnostics
go test ./expr -run '^$' -fuzz FuzzParse -fuzztime 30s
```

## Running the app

```sh
cd app
wails3 dev            # desktop window with hot reload
wails3 task browser   # same app in a browser at http://localhost:34115, no window
```

## Making a change

1. Read the `AGENTS.md` of the folder you work in. Each fact lives in one file; link to it instead of repeating it.
2. Keep behavior identical to Ikemen GO at the pinned version ([ikemen/AGENTS.md](ikemen/AGENTS.md)). When you port code from it, follow the extraction rules in [formats/AGENTS.md](formats/AGENTS.md) and update [docs/upstream-audit.md](docs/upstream-audit.md).
3. Add tests:
   - parser rules need a unit test;
   - new readers need a corpus test;
   - a new kind of broken input goes in `formats/testdata/broken/`, together with its `.diag` snapshot.
4. Run `gofmt`, `go vet` and the tests of the modules you touched.
5. Tick finished roadmap items in [docs/PLAN.md](docs/PLAN.md).

Commits: imperative mood, a short subject line, and a body when the change needs explaining. Keep personal or tool-specific settings out of the repository: `CLAUDE.local.md`, `AGENTS.local.md` and `.claude/*.local.json` are git-ignored for that.

## Test corpus

- **Kung Fu Man** and the `data/` folder come from the official Ikemen GO release. `tools/fetchcorpus` downloads them into `ikemen/release/`, pinned by checksum; that folder is never committed.
- **Broken files** in `formats/testdata/broken/` are written by hand for KataFighter.
- **Community characters** will be added only with the author's written permission, and with their license recorded in [CREDITS.md](CREDITS.md). Don't commit anyone else's character without that.

## Clean-room rule

Don't copy code, names, icons or UI from Fighter Factory. If you have worked on it, say so in your pull request.
