package check

import (
	"strings"
	"testing"

	"github.com/whoisnighter/KataFighter/lsp/schema"
)

func run(kind Kind, src string) []string {
	var out []string
	for _, d := range Text(kind, src, schema.Default()) {
		out = append(out, d.String())
	}
	return out
}

func TestKindOf(t *testing.T) {
	for name, want := range map[string]Kind{
		"kfm.cns": KindCNS, "A.ST": KindCNS, "x/kfm.cmd": KindCNS, "kfm.const": KindCNS,
		"kfm.zss": KindZSS, "kfm.air": KindNone, "README": KindNone,
	} {
		if got := KindOf(name); got != want {
			t.Errorf("KindOf(%q) = %d, want %d", name, got, want)
		}
	}
}

func TestCNS(t *testing.T) {
	src := strings.Join([]string{
		"[Statedef 200]",     // 1
		"type = S",           // 2
		"",                   // 3
		"[State 200, ok]",    // 4
		"type = ChangeState", // 5
		"trigger1 = AnimTime = 0 && statetype = S && vel x > 0", // 6
		"trigger2 = map(combo) > 0 && const(size.xscale) > 0",   // 7
		"value = ifelse(ctrl, 0, 20)",                           // 8
		"ctrl = 1",                                              // 9
		"",                                                      // 10
		"[State 200, bad]",                                      // 11
		"type = ChangeStat",                                     // 12
		"trigger1 = Tme > 3",                                    // 13
		"",                                                      // 14
		"[State 200, params]",                                   // 15
		"type = VelSet",                                         // 16
		"trigger1 = 1",                                          // 17
		"x = 1 + Velx",                                          // 18
		"foo = 3",                                               // 19
		"",                                                      // 20
		"[State 200, prefix]",                                   // 21
		"type = PlaySnd",                                        // 22
		"trigger1 = ProjHit1200 = 1",                            // 23
		"value = F5, 2",                                         // 24
	}, "\n")
	got := run(KindCNS, src)
	want := []string{
		`12:8: error: unknown state controller type "ChangeStat" [unknown-controller]`,
		`13:12: error: unknown trigger "Tme" [unknown-trigger]`,
		`18:9: error: unknown trigger "Velx" [unknown-trigger]`,
		`19:1: warning: unknown parameter "foo" for velset; Ikemen ignores it [unknown-param]`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestCNSUnresolvedParams(t *testing.T) {
	// VarSet keys are built at run time (var(N) = ...), so any key is allowed.
	src := "[Statedef 0]\n[State 0, v]\ntype = VarSet\ntrigger1 = 1\nvar(3) = 1\nanything = 2\n"
	if got := run(KindCNS, src); len(got) != 0 {
		t.Errorf("got %q", got)
	}
}

func TestZSS(t *testing.T) {
	src := strings.Join([]string{
		"[StateDef 200; type: S;]",         // 1
		"let n = 1;",                       // 2
		"if Tme > 3 && map(x) = 1 {",       // 3
		"\tchangeState{value: 0; ctrl: 1}", // 4
		"\tchangeStat{value: 0}",           // 5
		"\tvelSet{x: $n + Velx; bogus: 1}", // 6
		"\tplaySnd{value: S $n, 1}",        // 7
		"}",                                // 8
	}, "\n")
	got := run(KindZSS, src)
	want := []string{
		`3:4: error: unknown trigger "Tme" [unknown-trigger]`,
		`5:2: error: unknown state controller type "changeStat" [unknown-controller]`,
		`6:17: error: unknown trigger "Velx" [unknown-trigger]`,
		`6:23: warning: unknown parameter "bogus" for velset; Ikemen ignores it [unknown-param]`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestParserDiagnosticsIncluded(t *testing.T) {
	// A controller without trigger1 is a parser error; Text must keep it.
	got := run(KindCNS, "[Statedef 0]\n[State 0, x]\ntype = Null\n")
	if len(got) == 0 {
		t.Fatal("expected the parser's diagnostic")
	}
}
