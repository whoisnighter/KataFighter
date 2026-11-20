package expr

import (
	"fmt"
	"strings"
)

// Node is an expression node. Pos and End are byte offsets into the text
// that was parsed.
type Node interface {
	Span() (pos, end int)
	node()
}

type span struct{ P, E int }

func (s span) Span() (int, int) { return s.P, s.E }
func (span) node()              {}

// Num is a number literal. Float is true when the text has a '.' or an
// exponent. A leading '-' directly before the digits is part of the literal,
// as in Ikemen.
type Num struct {
	span
	Text  string
	Float bool
}

// Str is a quoted string ("holdfwd").
type Str struct {
	span
	Value string
}

// Ident is a bare name: a trigger without arguments (Time), a keyword
// argument (S, NA, data.life) or a ZSS local ($x). Name is lower-cased.
type Ident struct {
	span
	Name string
	Raw  string
}

// Call is name(args): a trigger or function with arguments, e.g.
// var(3), ifelse(a, b, c), gethitvar(damage), const(size.xscale).
type Call struct {
	span
	Name *Ident
	Args []Node
}

// Sub is a trigger followed by a bare keyword, e.g. "vel x", "p2bodydist y",
// "screenpos x". Ikemen reads these per trigger; any trigger directly
// followed by a name is parsed this way.
type Sub struct {
	span
	X   Node
	Key *Ident
}

// Redirect is "name, value" or "name(arg), value", e.g. "parent, var(1)",
// "helper(1200), time", "enemynear(1), pos x". Any name or call directly
// followed by ',' where an operand is expected is parsed this way.
type Redirect struct {
	span
	Target Node // *Ident or *Call
	X      Node
}

// Unary is a prefix operator (!, ~, -) or a legacy postfix "!".
type Unary struct {
	span
	Op      string
	X       Node
	Postfix bool
}

// Binary is X Op Y.
type Binary struct {
	span
	Op   string
	X, Y Node
}

// Range is "X = [lo, hi]" or with '(' / ')' for exclusive ends, and != for
// "not in range".
type Range struct {
	span
	X, Lo, Hi        Node
	Op               string // "=" or "!="
	LoOpen, HiClosed string // "[" or "(", "]" or ")"
}

// Compare is a comparison followed by a comma list, the old MUGEN trigger
// forms: "animelem = 2, >= 3", "projhit1200 = 1, < 4", "timemod = 4, 2",
// "hitdefattr = SCA, NA, SA". Base is the "X = Y" part.
type Compare struct {
	span
	Base  *Binary
	Items []Item
}

// Item is one element after the comma of a Compare. Op is "" for a bare value.
type Item struct {
	Op string
	X  Node
}

// Assign is "target := value" (var(1) := 3, $x := 1).
type Assign struct {
	span
	Target, Value Node
}

// BadExpr marks a part that could not be parsed. The parser reports the
// matching error.
type BadExpr struct {
	span
}

// String prints a node in a normalized form, mainly for tests and debugging.
func String(n Node) string {
	var b strings.Builder
	write(&b, n)
	return b.String()
}

func write(b *strings.Builder, n Node) {
	switch n := n.(type) {
	case nil:
		b.WriteString("<nil>")
	case *Num:
		b.WriteString(n.Text)
	case *Str:
		fmt.Fprintf(b, "%q", n.Value)
	case *Ident:
		b.WriteString(n.Name)
	case *Call:
		b.WriteString(n.Name.Name)
		b.WriteString("(")
		for i, a := range n.Args {
			if i > 0 {
				b.WriteString(", ")
			}
			write(b, a)
		}
		b.WriteString(")")
	case *Sub:
		write(b, n.X)
		b.WriteString(" ")
		b.WriteString(n.Key.Name)
	case *Redirect:
		b.WriteString("{")
		write(b, n.Target)
		b.WriteString(", ")
		write(b, n.X)
		b.WriteString("}")
	case *Unary:
		if n.Postfix {
			b.WriteString("(")
			write(b, n.X)
			b.WriteString(n.Op + ")")
			return
		}
		b.WriteString("(" + n.Op)
		write(b, n.X)
		b.WriteString(")")
	case *Binary:
		b.WriteString("(")
		write(b, n.X)
		b.WriteString(" " + n.Op + " ")
		write(b, n.Y)
		b.WriteString(")")
	case *Range:
		b.WriteString("(")
		write(b, n.X)
		b.WriteString(" " + n.Op + " " + n.LoOpen)
		write(b, n.Lo)
		b.WriteString(", ")
		write(b, n.Hi)
		b.WriteString(n.HiClosed + ")")
	case *Compare:
		b.WriteString("(")
		write(b, n.Base)
		for _, it := range n.Items {
			b.WriteString(", ")
			if it.Op != "" {
				b.WriteString(it.Op + " ")
			}
			write(b, it.X)
		}
		b.WriteString(")")
	case *Assign:
		b.WriteString("(")
		write(b, n.Target)
		b.WriteString(" := ")
		write(b, n.Value)
		b.WriteString(")")
	case *BadExpr:
		b.WriteString("<bad>")
	default:
		fmt.Fprintf(b, "<%T>", n)
	}
}

// Walk calls f for n and every node below it, depth first. If f returns
// false the children of that node are skipped.
func Walk(n Node, f func(Node) bool) {
	if n == nil || !f(n) {
		return
	}
	switch n := n.(type) {
	case *Call:
		Walk(n.Name, f)
		for _, a := range n.Args {
			Walk(a, f)
		}
	case *Sub:
		Walk(n.X, f)
		Walk(n.Key, f)
	case *Redirect:
		Walk(n.Target, f)
		Walk(n.X, f)
	case *Unary:
		Walk(n.X, f)
	case *Binary:
		Walk(n.X, f)
		Walk(n.Y, f)
	case *Range:
		Walk(n.X, f)
		Walk(n.Lo, f)
		Walk(n.Hi, f)
	case *Compare:
		Walk(n.Base, f)
		for _, it := range n.Items {
			Walk(it.X, f)
		}
	case *Assign:
		Walk(n.Target, f)
		Walk(n.Value, f)
	}
}
