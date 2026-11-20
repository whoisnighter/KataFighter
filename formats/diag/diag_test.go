package diag

import "testing"

func TestString(t *testing.T) {
	cases := []struct {
		d    Diagnostic
		want string
	}{
		{Diagnostic{File: "kfm.cns", Range: At(12, 5), Severity: Error, Message: "bad"}, "kfm.cns:12:5: error: bad"},
		{Diagnostic{File: "kfm.cns", Range: At(12, 0), Severity: Warning, Message: "w", Code: "dup"}, "kfm.cns:12: warning: w [dup]"},
		{Diagnostic{File: "kfm.sff", Severity: Warning, Message: "w"}, "kfm.sff: warning: w"},
		{Diagnostic{Range: At(3, 0), Severity: Hint, Message: "h"}, "3: hint: h"},
		{Diagnostic{Severity: Info, Message: "i"}, "info: i"},
	}
	for _, c := range cases {
		if got := c.d.String(); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
}

func TestList(t *testing.T) {
	l := List{File: "a.def"}
	l.Warnf(At(1, 0), "", "w %d", 1)
	if HasErrors(l.Items) {
		t.Error("no errors expected")
	}
	l.Errorf(At(2, 3), "x", "e")
	if !HasErrors(l.Items) || l.Items[1].File != "a.def" || l.Items[1].String() != "a.def:2:3: error: e [x]" {
		t.Errorf("items = %v", l.Items)
	}
	if !(Pos{1, 9}).Before(Pos{2, 1}) || (Pos{2, 1}).Before(Pos{2, 1}) {
		t.Error("Before")
	}
}
