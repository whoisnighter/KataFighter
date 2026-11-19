// Command schemagen reads Ikemen GO's compiler source with go/ast and writes
// the CNS/ZSS schema (state controllers, their parameters, triggers) as JSON.
//
//	go run ./schemagen -src ../ikemen/source -out ../lsp/schema/ikemen.json
//
// It reads, never imports, the engine source: Ikemen is package main with cgo
// dependencies. Re-run it whenever the pinned Ikemen version changes.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	src := flag.String("src", "../ikemen/source", "Ikemen GO source checkout (the repo root)")
	out := flag.String("out", "../lsp/schema/ikemen.json", "output JSON file")
	flag.Parse()

	s, err := Generate(filepath.Join(*src, "src"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "schemagen:", err)
		os.Exit(1)
	}
	s.Source.Version = gitOut(*src, "describe", "--tags", "--always")
	s.Source.Commit = gitOut(*src, "rev-parse", "HEAD")

	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "schemagen:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, append(b, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "schemagen:", err)
		os.Exit(1)
	}
	params, dyn := 0, 0
	for _, c := range s.Controllers {
		params += len(c.Params)
		dyn += len(c.Unresolved)
	}
	kw := 0
	for _, t := range s.Triggers {
		kw += len(t.Keywords)
	}
	fmt.Printf("%s: %d controllers (%d params, %d unresolved names), %d triggers (%d keywords)\n",
		*out, len(s.Controllers), params, dyn, len(s.Triggers), kw)
	for _, w := range s.Warnings {
		fmt.Println("  warning:", w)
	}
}

func gitOut(dir string, args ...string) string {
	b, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
