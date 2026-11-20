package expr

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct{ in, want string }{
		{"1 + 2 * 3", "(1 + (2 * 3))"},
		{"1 - 2 - 3", "((1 - 2) - 3)"},
		{"2 ** 3 ** 2", "((2 ** 3) ** 2)"},
		{"a || b && c | d ^ e & f = g", "(a || (b && (c | (d ^ (e & (f = g))))))"},
		{"Time = 5 && !Ctrl", "((time = 5) && (!ctrl))"},
		{"-5 + -x", "(-5 + (-x))"},
		{"- 5", "(-5)"},
		{"-.5e2", "-.5e2"},
		{"~a ** 2", "((~a) ** 2)"},
		{"stateno = [200, 210]", "(stateno = [200, 210])"},
		{"stateno != (200, 210]", "(stateno != (200, 210])"},
		{"time = (1 + 2)", "(time = (1 + 2))"},
		{"vel x > 0", "(vel x > 0)"},
		{"p2bodydist X < 30", "(p2bodydist x < 30)"},
		{"parent, var(1) = 2", "({parent, var(1)} = 2)"},
		{"helper(1200), pos y", "{helper(1200), pos y}"},
		{"enemynear(var(2)), stateno", "{enemynear(var(2)), stateno}"},
		{"ifelse(a, 1, 2)", "ifelse(a, 1, 2)"},
		{"gethitvar(fall.yvel)", "gethitvar(fall.yvel)"},
		{"const(size.ground.back)", "const(size.ground.back)"},
		{`command = "holdfwd"`, `(command = "holdfwd")`},
		{"animelem = 2, >= 3", "((animelem = 2), >= 3)"},
		{"timemod = 4, 2", "((timemod = 4), 2)"},
		{"hitdefattr = SCA, NA, SA", "((hitdefattr = sca), na, sa)"},
		{"var(1) := var(1) + 1", "(var(1) := (var(1) + 1))"},
		{"var(1) := 1 || 0", "((var(1) := 1) || 0)"},
		{"$x := 2", "($x := 2)"},
		{"time!", "(time!)"},
		{"random % 3 = 0", "((random % 3) = 0)"},
		{"(p2,hitDefAttr != SCA,SA || x)", "((({p2, hitdefattr} != sca), sa) || x)"},
		{"ifelse(time = 1, 2, 3)", "ifelse((time = 1), 2, 3)"},
		{"ifelse(animelem = 2, >= 3, 1, 0)", "ifelse(((animelem = 2), >= 3), 1, 0)"},
		{"a && hitdefattr = SC, NA", "(a && ((hitdefattr = sc), na))"},
		{"12abc", "12"}, // then an error for "abc"
	}
	for _, c := range cases {
		n, errs := Parse(c.in, Options{})
		got := String(n)
		if got != c.want {
			t.Errorf("%q\n got %s\nwant %s", c.in, got, c.want)
		}
		if len(errs) > 0 && c.in != "12abc" {
			t.Errorf("%q: errors %v", c.in, errs)
		}
	}
}

func TestErrors(t *testing.T) {
	cases := []struct{ in, msg string }{
		{"", "empty expression"},
		{"1 +", "missing value"},
		{"(1 + 2", "missing ')'"},
		{"f(1, 2", "missing ')' after arguments"},
		{"stateno = [1, 2", "missing ']' or ')'"},
		{"stateno = [1 2]", "missing ','"},
		{`command = "abc`, "string not closed"},
		{"1 2", "unexpected \"2\""},
		{"a = )", "unexpected \")\""},
	}
	for _, c := range cases {
		_, errs := Parse(c.in, Options{})
		if len(errs) == 0 || !strings.Contains(errs[0].Msg, c.msg) {
			t.Errorf("%q: errors %v, want %q", c.in, errs, c.msg)
		}
	}
}

func TestSpans(t *testing.T) {
	src := "  vel x > ifelse(a, 1, 2)"
	n, _ := Parse(src, Options{})
	b := n.(*Binary)
	if p, e := b.Span(); src[p:e] != "vel x > ifelse(a, 1, 2)" {
		t.Errorf("binary span %q", src[p:e])
	}
	if p, e := b.Y.Span(); src[p:e] != "ifelse(a, 1, 2)" {
		t.Errorf("call span %q", src[p:e])
	}
	var idents []string
	Walk(n, func(n Node) bool {
		if id, ok := n.(*Ident); ok {
			idents = append(idents, id.Name)
		}
		return true
	})
	if strings.Join(idents, " ") != "vel x ifelse a" {
		t.Errorf("walk %v", idents)
	}
}

func TestList(t *testing.T) {
	list, errs := ParseList("-10, , ifelse(a, 1, 2)", Options{})
	if len(errs) > 0 || len(list) != 3 || list[1] != nil || String(list[2]) != "ifelse(a, 1, 2)" {
		t.Errorf("list %v errs %v", list, errs)
	}
	// The legacy comma form is not used in lists.
	list, _ = ParseList("a = 1, 2", Options{})
	if len(list) != 2 {
		t.Errorf("list %v", list)
	}
}

func TestZSS(t *testing.T) {
	n, errs := Parse("time > 3 # comment", Options{ZSS: true})
	if len(errs) > 0 || String(n) != "(time > 3)" {
		t.Errorf("got %s %v", String(n), errs)
	}
	if _, errs := Parse("time!", Options{ZSS: true}); len(errs) == 0 {
		t.Error("postfix ! must be an error in ZSS")
	}
}

func FuzzParse(f *testing.F) {
	for _, s := range []string{"a = [1, 2]", "parent, var(1) := 3", "animelem = 2, >= 3", `"x`, "((("} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		Parse(s, Options{})
		Parse(s, Options{ZSS: true})
		ParseList(s, Options{})
	})
}
