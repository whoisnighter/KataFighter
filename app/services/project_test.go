package services

import (
	"os"
	"testing"
)

const release = "../../ikemen/release/Ikemen_GO-v1.0.0-windows"

func TestOpenKFM(t *testing.T) {
	for _, p := range []string{release + "/chars/kfm/kfm.def", release + "/chars/kfm_zss/kfm_zss.def"} {
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
		for _, pr := range c.Problems {
			t.Errorf("%s: problem %+v", p, pr)
		}
		if len(c.Files) < 6 {
			t.Errorf("%s: only %d files", p, len(c.Files))
		}
	}
}
