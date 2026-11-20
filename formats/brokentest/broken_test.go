// Package brokentest runs every reader over the intentionally broken files
// in testdata/broken. Readers must not panic, and their diagnostics are
// snapshot-tested: each sample has a .diag file next to it.
//
// After an intended change to the diagnostics, regenerate with:
//
//	go test ./brokentest -update
package brokentest

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/whoisnighter/KataFighter/formats/air"
	"github.com/whoisnighter/KataFighter/formats/cns"
	"github.com/whoisnighter/KataFighter/formats/def"
	"github.com/whoisnighter/KataFighter/formats/diag"
	"github.com/whoisnighter/KataFighter/formats/sff"
	"github.com/whoisnighter/KataFighter/formats/snd"
	"github.com/whoisnighter/KataFighter/formats/zss"
)

var update = flag.Bool("update", false, "rewrite the .diag snapshots")

const dir = "../testdata/broken"

// load runs the reader for the file's extension. err is a load error
// (a file the reader refuses as a whole); ds are the diagnostics.
func load(path string) (ds []diag.Diagnostic, err error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".sff":
		var f *sff.File
		if f, err = sff.Open(path, sff.Options{Character: true}); f != nil {
			ds = f.Diagnostics
		}
	case ".snd":
		var f *snd.File
		if f, err = snd.Open(path); f != nil {
			ds = f.Diagnostics
		}
	case ".air":
		var f *air.File
		if f, err = air.Load(path); f != nil {
			ds = f.Diagnostics
		}
	case ".def":
		var d *def.Def
		if d, err = def.Load(path, def.Options{}); d != nil {
			ds = d.Diagnostics
		}
	case ".cns", ".st", ".cmd", ".const":
		var f *cns.File
		if f, err = cns.Load(path); f != nil {
			ds = f.Diagnostics
		}
	case ".zss":
		var f *zss.File
		if f, err = zss.Load(path); f != nil {
			ds = f.Diagnostics
		}
	default:
		err = fmt.Errorf("no reader for %s", filepath.Ext(path))
	}
	return
}

// snapshot renders the result without the directory, so it is the same on
// every OS.
func snapshot(ds []diag.Diagnostic, err error) []byte {
	var b bytes.Buffer
	if err != nil {
		fmt.Fprintf(&b, "load error: %v\n", err)
	}
	for _, d := range ds {
		d.File = filepath.Base(d.File)
		fmt.Fprintln(&b, d.String())
	}
	if b.Len() == 0 {
		b.WriteString("(no diagnostics)\n")
	}
	return b.Bytes()
}

func TestBrokenFiles(t *testing.T) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var samples []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) != ".diag" {
			samples = append(samples, e.Name())
		}
	}
	sort.Strings(samples)
	if len(samples) == 0 {
		t.Fatal("no samples in", dir)
	}
	for _, name := range samples {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name)
			var ds []diag.Diagnostic
			var lerr error
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("panic: %v", r)
					}
				}()
				ds, lerr = load(path)
			}()
			if lerr == nil && len(ds) == 0 {
				t.Error("a broken file must give a load error or at least one diagnostic")
			}
			got := snapshot(ds, lerr)
			snap := path + ".diag"
			if *update {
				if err := os.WriteFile(snap, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(snap)
			if err != nil {
				t.Fatalf("no snapshot (run with -update): %v", err)
			}
			want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))
			if !bytes.Equal(got, want) {
				t.Errorf("diagnostics changed\n--- got\n%s--- want\n%s", got, want)
			}
		})
	}
}
