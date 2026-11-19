# tools/: developer tools

↑ [Root](../AGENTS.md)

Go module `github.com/whoisnighter/KataFighter/tools`. These are build-time generators, not shipped in the app.

## schemagen/

This tool reads Ikemen's compiler source in `ikemen/source` with `go/ast` and writes [lsp/schema/ikemen.json](../lsp/AGENTS.md). The output lists the state controllers (with origin and parameters) and the triggers (with keyword arguments). It reads the engine source and never imports it.

```sh
cd tools
go run ./schemagen          # regenerate after bumping ikemen/ (see ../ikemen/AGENTS.md)
go test ./...               # fails if the committed JSON is stale
```

Rules:
- Never edit `lsp/schema/ikemen.json` by hand. Hand-written docs go in a separate file that is layered on top.
- When Ikemen's compiler changes shape and extraction breaks, fix the generator. Don't patch the output.
- Known gaps and how to read them are listed in [upstream-audit.md](../docs/upstream-audit.md#status-schema-generated-2026-11-19).
