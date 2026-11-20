package text

import (
	"sort"
	"strings"
	"testing"
)

func TestReadSection(t *testing.T) {
	src := "; head\n[Info]\n  Name = \"KFM\" ; c\nname = dup\nflag on\nlonely\nlocal = 1, ,3\n[Files]\nx=1"
	lines, raw := SplitAndTrim(src, "\n"), strings.Split(src, "\n")
	i := 0
	s := ReadSection(lines, raw, &i)
	if s == nil || s.Name != "info" || s.Line != 2 {
		t.Fatalf("section %+v", s)
	}
	if v, _ := s.Get("name"); v != `"KFM"` {
		t.Errorf("name %q", v)
	}
	e := s.Entry("name")
	if e.Line != 3 || e.Col != 3 {
		t.Errorf("name at %d:%d", e.Line, e.Col)
	}
	if !s.Entries[1].Dup {
		t.Error("second name must be a duplicate")
	}
	if v, ok := s.Get("flag"); !ok || v != "" {
		t.Errorf("flag %q %v", v, ok)
	}
	if txt, _, err := s.Text("name"); txt != "KFM" || err != nil {
		t.Errorf("text %q %v", txt, err)
	}
	a, b, c := int32(7), int32(7), int32(7)
	s.ReadI32("local", &a, &b, &c)
	if a != 1 || b != 7 || c != 3 {
		t.Errorf("ReadI32 %d %d %d (empty item keeps the default)", a, b, c)
	}
	if got := strings.Join(s.Keys(), ","); got != "name,flag,local" {
		t.Errorf("keys %s", got)
	}
	s2 := ReadSection(lines, raw, &i)
	if s2 == nil || s2.Name != "files" || s2.Value("x") != "1" {
		t.Errorf("second section %+v", s2)
	}
	if ReadSection(lines, raw, &i) != nil {
		t.Error("expected no more sections")
	}
}

func TestNaturalLess(t *testing.T) {
	keys := []string{"st10", "st2", "st", "st1", "cmd"}
	sort.SliceStable(keys, func(a, b int) bool { return NaturalLess(keys[a], keys[b]) })
	if got := strings.Join(keys, " "); got != "cmd st st1 st2 st10" {
		t.Errorf("got %s", got)
	}
}

func TestAtoiAtof(t *testing.T) {
	if n, _ := Atoi("12abc"); n != 12 {
		t.Errorf("Atoi = %d", n)
	}
	if _, ov := Atoi("99999999999"); !ov {
		t.Error("overflow not reported")
	}
	if f := Atof("-1.5e2x"); f != -150 {
		t.Errorf("Atof = %v", f)
	}
}
