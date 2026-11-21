// Command katafighter-lsp is the KataFighter language server for Ikemen GO
// character files (CNS, ST, CMD, ZSS). It speaks LSP over stdin/stdout and
// logs to stderr.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/whoisnighter/KataFighter/lsp/schema"
	"github.com/whoisnighter/KataFighter/lsp/server"
)

func main() {
	version := flag.Bool("version", false, "print the version and exit")
	flag.Bool("stdio", true, "use stdin/stdout (the only transport; accepted for clients that pass it)")
	flag.Parse()
	if *version {
		src := schema.Default().Source
		fmt.Printf("%s %s (schema: %s %s)\n", server.Name, server.Version, src.Engine, src.Version)
		return
	}
	if err := server.New(os.Stdin, os.Stdout, os.Stderr).Run(); err != nil {
		fmt.Fprintln(os.Stderr, server.Name+":", err)
		os.Exit(1)
	}
}
