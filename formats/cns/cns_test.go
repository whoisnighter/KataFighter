package cns

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/whoisnighter/KataFighter/formats/diag"
	"github.com/whoisnighter/KataFighter/formats/expr"
)

const release = "../../ikemen/release/Ikemen_GO-v1.0.0-windows"

func codes(ds []diag.Diagnostic) string {
	var cs []string
	for _, d := range ds {
		cs = append(cs, d.Range.Start.String()+" "+d.Code)
	}
	return strings.Join(cs, ", ")
}

const sample = `; KFM-like sample
[Data]
life = 1000
Life = 5          ; duplicate, ignored
attack = 100

[Statedef 200]
type    = S
movetype= A
anim = 200

[State 200, Hit]
type = HitDef
trigger1 = AnimElem = 3
attr = S, NA
damage = 23, 0
var(3) = 1

[State 200, End]
type = ChangeState
triggerall = Time > 0
trigger1 = AnimTime = 0
trigger1 = vel x < 1
trigger2 = command = "holdfwd"
value = 0
ctrl = 1

[Statedef +1]

[State -1, broken]
type = Null
trigger2 = (1 +
trigger3 = 1
foo bar = 1

[Statedef 200]

[Command]
name = "QCF_x"
command = ~D, DF, F, x
`

func TestParseStructure(t *testing.T) {
	f := Parse(sample)
	if len(f.Sections) != 8 {
		t.Fatalf("%d sections", len(f.Sections))
	}
	data := f.Section("data")
	if data == nil || data.Value("life") != "1000" || !data.Entries[1].Dup {
		t.Errorf("data %+v", data)
	}
	if len(f.States) != 3 {
		t.Fatalf("%d states", len(f.States))
	}
	sd := f.States[0]
	if sd.Number != 200 || sd.Value("type") != "S" || sd.Value("movetype") != "A" || len(sd.Controllers) != 2 {
		t.Errorf("statedef %+v", sd)
	}
	if f.States[1].Number != -10 || f.States[1].NumberText != "+1" {
		t.Errorf("+1 statedef %+v", f.States[1])
	}
	hit := sd.Controllers[0]
	if hit.Type != "hitdef" || hit.Label != "Hit" || len(hit.Triggers) != 1 {
		t.Errorf("hitdef %+v", hit)
	}
	if e := hit.Get("var(3)"); e == nil || e.Value != "1" {
		t.Errorf("var(3) param: %+v", e)
	}
	attr := hit.Get("attr")
	if vals, _ := attr.ParseValue(); len(vals) != 2 || expr.String(vals[1]) != "na" {
		t.Errorf("attr values %v", vals)
	}

	cs := sd.Controllers[1]
	if len(cs.TriggerAll) != 1 || len(cs.Triggers) != 2 || len(cs.Triggers[0]) != 2 {
		t.Fatalf("triggers all=%d groups=%d", len(cs.TriggerAll), len(cs.Triggers))
	}
	if got := expr.String(cs.Triggers[0][1].Expr); got != "(vel x < 1)" {
		t.Errorf("trigger1 #2 = %s", got)
	}
	if got := expr.String(cs.Triggers[1][0].Expr); got != `(command = "holdfwd")` {
		t.Errorf("trigger2 = %s", got)
	}
	// Columns: "trigger1 = vel x < 1" -> value starts at column 12.
	e := cs.Triggers[0][1]
	if e.KeyCol != 1 || e.ValueCol != 12 || e.Line != 23 {
		t.Errorf("positions line %d key %d value %d", e.Line, e.KeyCol, e.ValueCol)
	}
	if len(f.Commands) != 1 || f.Commands[0].Value("command") != "~D, DF, F, x" {
		t.Errorf("commands %+v", f.Commands)
	}
}

func TestDiagnostics(t *testing.T) {
	f := Parse(sample)
	want := "4:1 duplicate-key, " +
		"30 missing-trigger1, " + // [State -1, broken] has no trigger1
		"32:1 missing-trigger, " + // trigger2 without trigger1
		"32:16 syntax, " + // "(1 +" -> missing value at the end
		"34:4 param-space, " +
		"36 duplicate-statedef"
	if got := codes(f.Diagnostics); got != want {
		t.Errorf("diagnostics\n got %s\nwant %s", got, want)
		for _, d := range f.Diagnostics {
			t.Log(d)
		}
	}
}

func TestTriggerNumber(t *testing.T) {
	for key, want := range map[string]bool{
		"trigger1": true, "triggerall": true, "trigger65536": true,
		"trigger0": false, "trigger65537": false, "trigger": false, "trigger1a": false, "type": false,
	} {
		if _, _, ok := TriggerNumber(key); ok != want {
			t.Errorf("%s: %v", key, ok)
		}
	}
}

func TestOrphanAndBrokenHeaders(t *testing.T) {
	f := Parse("[State 0, x]\ntype = Null\ntrigger1 = 1\n[Statedef 5\n")
	if got := codes(f.Diagnostics); got != "1 orphan-state, 4:1 header-not-closed" {
		t.Errorf("got %s", got)
	}
}

func TestCorpus(t *testing.T) {
	var files []string
	for _, pat := range []string{"chars/*/*.cns", "chars/*/*.cmd", "chars/*/*.const", "data/*.cmd", "data/*.const"} {
		m, _ := filepath.Glob(release + "/" + pat)
		files = append(files, m...)
	}
	if len(files) == 0 {
		t.Skip("no Ikemen GO release in ikemen/release")
	}
	for _, p := range files {
		f, err := Load(p)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Diagnostics {
			t.Error(d)
		}
		for _, s := range f.States {
			for _, c := range s.Controllers {
				for _, g := range append([][]*Entry{c.TriggerAll}, c.Triggers...) {
					for _, e := range g {
						if e.Expr == nil {
							t.Errorf("%s:%d: trigger without a tree", p, e.Line)
						}
					}
				}
			}
		}
	}
	f, err := Load(release + "/chars/kfm/kfm.cns")
	if err != nil {
		t.Fatal(err)
	}
	ctrls := 0
	for _, s := range f.States {
		ctrls += len(s.Controllers)
	}
	if len(f.States) != 58 || ctrls != 318 {
		t.Errorf("kfm.cns: %d states, %d controllers", len(f.States), ctrls)
	}
}
