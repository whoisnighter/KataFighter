package check

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/whoisnighter/KataFighter/lsp/schema"
)

const release = "../../ikemen/release/Ikemen_GO-v1.0.0-windows"

// TestCorpus checks every state file in the release. Ikemen loads them all,
// so there must be no errors. The only warnings are real quirks of KFM that
// Ikemen logs too: PosFreeze has no x / y, and "type = null;PosAdd" is a
// Null controller whose x is ignored.
func TestCorpus(t *testing.T) {
	var files []string
	for _, pat := range []string{"chars/*/*.cns", "chars/*/*.st", "chars/*/*.cmd", "chars/*/*.const",
		"chars/*/*.zss", "data/*.cns", "data/*.st", "data/*.cmd", "data/*.const", "data/*.zss"} {
		m, _ := filepath.Glob(release + "/" + pat)
		files = append(files, m...)
	}
	if len(files) == 0 {
		t.Skip("no Ikemen GO release in ikemen/release")
	}
	want := []string{
		"kfm.cns: 1474:1: warning: unknown parameter \"x\" for posfreeze; Ikemen ignores it [unknown-param]",
		"kfm.cns: 1475:1: warning: unknown parameter \"y\" for posfreeze; Ikemen ignores it [unknown-param]",
		"kfm.cns: 1604:1: warning: unknown parameter \"x\" for null; Ikemen ignores it [unknown-param]",
		"kfm720.cns: 1474:1: warning: unknown parameter \"x\" for posfreeze; Ikemen ignores it [unknown-param]",
		"kfm720.cns: 1475:1: warning: unknown parameter \"y\" for posfreeze; Ikemen ignores it [unknown-param]",
	}
	var got []string
	s := schema.Default()
	for _, p := range files {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range Text(KindOf(p), string(b), s) {
			got = append(got, filepath.Base(p)+": "+d.String())
		}
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("got %d diagnostics:\n%s\nwant:\n%s", len(got), join(got), join(want))
	}
	t.Logf("%d files", len(files))
}

func join(ss []string) string {
	out := ""
	for _, s := range ss {
		out += "  " + s + "\n"
	}
	return out
}
