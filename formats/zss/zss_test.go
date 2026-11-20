package zss

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

const sample = `# sample
[Function EndState(type) ret]
if animTime = 0 {
	changeState{value: 11 * $type; ctrl: roundState > 1}
}

[StateDef 200; type: S; movetype: A; anim: 200;]
ignorehitpause persistent(0) if time = 0 {
	playSnd{value: S0, 0}
} else if vel x > 0 {
	velSet{x: 0}
} else {
	call EndState(0);
}
let a, b = 1, 2;
let r = call EndState(1);
var(1) := $a + $b;
switch stateNo {
case 200; 210:
	null{}
default:
	null{}
}
for i = 0; 10; 2 {
	if $i = 4 { break; }
}
while time < 5 {
	continue;
}
ignorehitpause {
	helper(1), var(2) := 1;
}
`

func TestParseStructure(t *testing.T) {
	f := Parse(sample)
	for _, d := range f.Diagnostics {
		t.Error(d)
	}
	if len(f.Functions) != 1 || len(f.States) != 1 {
		t.Fatalf("functions %d states %d", len(f.Functions), len(f.States))
	}
	fn := f.Functions[0]
	if fn.Name != "endstate" || strings.Join(fn.Args, ",") != "type" || strings.Join(fn.Returns, ",") != "ret" || len(fn.Body) != 1 {
		t.Errorf("function %+v", fn)
	}
	cs := fn.Body[0].(*If).Then[0].(*Controller)
	if cs.Name != "changestate" || len(cs.Params) != 2 || cs.Params[1].Value != "roundState > 1" {
		t.Errorf("controller %+v", cs)
	}

	s := f.States[0]
	if s.Number != 200 || len(s.Params) != 3 || s.Params[1].Key != "movetype" || s.Params[1].Value != "A" {
		t.Errorf("state %+v", s)
	}
	if len(s.Body) != 8 {
		t.Fatalf("%d statements", len(s.Body))
	}
	iff := s.Body[0].(*If)
	if !iff.Attrs.IgnoreHitPause || iff.Attrs.Persistent != 1<<31-1 || expr.String(iff.Cond.Node) != "(time = 0)" {
		t.Errorf("if attrs %+v cond %s", iff.Attrs, expr.String(iff.Cond.Node))
	}
	elseIf := iff.Else[0].(*If)
	if expr.String(elseIf.Cond.Node) != "(vel x > 0)" {
		t.Errorf("else if cond %s", expr.String(elseIf.Cond.Node))
	}
	if c := elseIf.Else[0].(*Call); c.Name != "endstate" || len(c.Args) != 1 || c.Args[0].Text != "0" {
		t.Errorf("call %+v", c)
	}
	if l := s.Body[1].(*Let); strings.Join(l.Names, ",") != "a,b" || len(l.Values) != 2 || expr.String(l.Values[1].Node) != "2" {
		t.Errorf("let %+v", l)
	}
	if l := s.Body[2].(*Let); l.Call == nil || l.Call.Name != "endstate" {
		t.Errorf("let call %+v", l)
	}
	if e := s.Body[3].(*ExprStmt); expr.String(e.X.Node) != "(var(1) := ($a + $b))" {
		t.Errorf("assign %s", expr.String(e.X.Node))
	}
	sw := s.Body[4].(*Switch)
	if len(sw.Cases) != 2 || len(sw.Cases[0].Values) != 2 || !sw.Cases[1].Default {
		t.Errorf("switch %+v", sw)
	}
	fr := s.Body[5].(*For)
	if fr.Var != "i" || len(fr.Exprs) != 3 || fr.Exprs[2].Text != "2" {
		t.Errorf("for %+v", fr)
	}
	if _, ok := fr.Body[0].(*If).Then[0].(*Break); !ok {
		t.Error("break")
	}
	if b := s.Body[7].(*Block); !b.Attrs.IgnoreHitPause || len(b.Body) != 1 {
		t.Errorf("block %+v", b)
	}

	// Positions: "velSet" on line 11, column 2.
	vs := elseIf.Then[0].(*Controller)
	if p := f.Position(vs.Pos); p != (diag.Pos{Line: 11, Col: 2}) {
		t.Errorf("velSet at %v", p)
	}
}

func TestDiagnostics(t *testing.T) {
	src := `[StateDef 10; type: S]
if time = 0 {
	null{}
}
[StateDef 11;]
var(1) + 1;
break;
null{value: 1 value: 2}
if (time {
}
[StateDef 11;]
persistent(2) let x = 1;
[Function f(a, a)]
persistent(1) if 1 { }
`
	f := Parse(src)
	want := "1:22 missing-semicolon, " +
		"6:1 unused-value, " +
		"7:1 syntax, " + // break outside a loop
		"8:20 syntax, " + // expected ';' or '}'
		"9:9 syntax, " + // missing ')'
		"11:1 duplicate-statedef, " +
		"12:15 syntax, " + // let after an attribute
		"13:16 duplicate-name, " +
		"14:1 syntax" // persistent in a function
	if got := codes(f.Diagnostics); got != want {
		t.Errorf("diagnostics\n got %s\nwant %s", got, want)
		for _, d := range f.Diagnostics {
			t.Log(d)
		}
	}
}

func TestCorpus(t *testing.T) {
	var files []string
	for _, pat := range []string{"data/*.zss", "chars/*/*.zss"} {
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
	}
	f, err := Load(release + "/chars/kfm_zss/kfm.zss")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.States) != 55 || len(f.Functions) != 4 {
		t.Errorf("kfm.zss: %d states, %d functions", len(f.States), len(f.Functions))
	}
}

func FuzzParse(f *testing.F) {
	f.Add(sample)
	f.Add("[StateDef 1;]\nif {")
	f.Add("[Function f(")
	f.Fuzz(func(t *testing.T, s string) {
		Parse(s) // must not panic or loop
	})
}
