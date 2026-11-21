package services

import (
	"os"
	"testing"
)

const release = "../../ikemen/release/Ikemen_GO-v1.0.0-windows"

func TestOpenKFM(t *testing.T) {
	// kfm.cns has three real quirks that Ikemen logs too (see lsp/check
	// corpus_test.go): PosFreeze with x / y, and Null with x.
	for p, warnings := range map[string]int{release + "/chars/kfm/kfm.def": 3, release + "/chars/kfm_zss/kfm_zss.def": 0} {
		if _, err := os.Stat(p); err != nil {
			t.Skip("no Ikemen GO release in ikemen/release")
		}
		c, err := (&ProjectService{}).Open(p)
		if err != nil {
			t.Fatal(err)
		}
		if c.Name != "Kung Fu Man" || c.EngineDir == "" {
			t.Errorf("%s: name %q engine %q", p, c.Name, c.EngineDir)
		}
		for _, f := range c.Files {
			if f.Path == "" || len(f.Summary) >= 6 && f.Summary[:6] == "error:" {
				t.Errorf("%s: %+v", p, f)
			}
		}
		w := 0
		for _, pr := range c.Problems {
			if pr.Severity == "warning" {
				w++
				continue
			}
			t.Errorf("%s: problem %+v", p, pr)
		}
		if w != warnings {
			t.Errorf("%s: %d warnings, want %d: %+v", p, w, warnings, c.Problems)
		}
		if len(c.Files) < 6 {
			t.Errorf("%s: only %d files", p, len(c.Files))
		}
	}
}
