package expr

import "fmt"

// Error is a syntax error. Pos and End are byte offsets into the source.
type Error struct {
	Pos, End int
	Msg      string
}

func (e Error) Error() string { return fmt.Sprintf("%d: %s", e.Pos, e.Msg) }

// Options select the dialect.
type Options struct {
	// ZSS enables '#' comments and disables the legacy CNS postfix '!'.
	ZSS bool
}

// redirects are the trigger redirection keywords (the redirection cases at
// the top of expValue). They are grammar, not triggers: "parent, time".
var redirects = map[string]bool{
	"root": true, "parent": true, "p2": true, "stateowner": true,
	"partner": true, "enemy": true, "enemynear": true, "playerid": true,
	"player": true, "playerindex": true, "helperindex": true,
	"helper": true, "target": true,
}

// precedence mirrors isOperator: higher binds tighter, 0 = not a binary operator.
func precedence(op string) int {
	switch op {
	case "||":
		return 1
	case "^^":
		return 2
	case "&&":
		return 3
	case "|":
		return 4
	case "^":
		return 5
	case "&":
		return 6
	case "=", "!=":
		return 7
	case ">", ">=", "<", "<=":
		return 8
	case "+", "-":
		return 9
	case "*", "/", "%":
		return 10
	case "**":
		return 11
	}
	return 0
}

const eqLevel, relLevel = 7, 8

type parser struct {
	src   string
	toks  []Token
	i     int
	opt   Options
	errs  []Error
	depth int // > 0 inside call arguments, parentheses and ranges
	// list is true for ParseList: a top-level ',' separates values there.
	list bool
}

func newParser(src string, opt Options) *parser {
	return &parser{src: src, toks: Lex(src, opt.ZSS), opt: opt}
}

func (p *parser) tok() Token  { return p.toks[p.i] }
func (p *parser) peek() Token { return p.toks[min(p.i+1, len(p.toks)-1)] }
func (p *parser) next() Token {
	t := p.toks[p.i]
	if p.i < len(p.toks)-1 {
		p.i++
	}
	return t
}
func (p *parser) is(text string) bool {
	t := p.tok()
	return t.Kind == TokPunct && t.Text == text
}
func (p *parser) errorf(t Token, format string, a ...any) {
	// One error per position: the first one is the useful one.
	if n := len(p.errs); n > 0 && p.errs[n-1].Pos == t.Pos {
		return
	}
	p.errs = append(p.errs, Error{Pos: t.Pos, End: max(t.End, t.Pos+1), Msg: fmt.Sprintf(format, a...)})
}

// Parse parses one complete expression, such as a trigger value. The legacy
// comma forms (Compare) are allowed at the top level.
func Parse(src string, opt Options) (Node, []Error) {
	p := newParser(src, opt)
	if p.tok().Kind == TokEOF {
		p.errorf(p.tok(), "empty expression")
		return &BadExpr{span{0, len(src)}}, p.errs
	}
	n := p.expr(1)
	if t := p.tok(); t.Kind != TokEOF {
		p.errorf(t, "unexpected %q after expression", t.Text)
	}
	return n, p.errs
}

// ParseList parses a comma-separated list of expressions, such as a
// controller parameter ("value = 1, 2" or "pos = -10, ifelse(a, 1, 2)").
// An empty item yields nil, as an empty item leaves the default in Ikemen.
func ParseList(src string, opt Options) ([]Node, []Error) {
	p := newParser(src, opt)
	p.list = true
	var list []Node
	for {
		if p.is(",") || p.tok().Kind == TokEOF {
			list = append(list, nil)
		} else {
			list = append(list, p.expr(1))
		}
		if !p.is(",") {
			break
		}
		p.next()
	}
	if t := p.tok(); t.Kind != TokEOF {
		p.errorf(t, "unexpected %q", t.Text)
	}
	return list, p.errs
}

func spanOf(a, b Node) span {
	ap, _ := a.Span()
	_, be := b.Span()
	return span{ap, be}
}

// expr is precedence climbing over the isOperator levels; all operators are
// left-associative, as in the expXxx loops.
func (p *parser) expr(minPrec int) Node {
	x := p.unary()
	for {
		t := p.tok()
		prec := 0
		if t.Kind == TokPunct {
			prec = precedence(t.Text)
		}
		if prec == 0 || prec < minPrec {
			return x
		}
		p.next()
		if prec == eqLevel {
			x = p.equality(x, t.Text)
			continue
		}
		y := p.expr(prec + 1)
		x = &Binary{spanOf(x, y), t.Text, x, y}
	}
}

// equality parses what follows "=" or "!=": a range, a plain operand, and
// the legacy comma list (Compare). Ikemen reads that list inside the
// trigger, so it can appear anywhere; inside parentheses or arguments, where
// ',' also separates arguments, an item is only taken when it starts with a
// comparison operator ("animelem = 2, >= 3") or is an attack attribute code
// ("hitdefattr = SC, NA, SA"), which is what trgAttr and evaluateComparison
// accept there.
func (p *parser) equality(x Node, op string) Node {
	if p.is("[") || p.is("(") {
		if r := p.tryRange(x, op); r != nil {
			return r
		}
	}
	y := p.expr(eqLevel + 1)
	b := &Binary{spanOf(x, y), op, x, y}
	var c *Compare
	for p.is(",") && !(p.list && p.depth == 0) {
		nt := p.peek()
		isOp := nt.Kind == TokPunct && (precedence(nt.Text) == eqLevel || precedence(nt.Text) == relLevel)
		if p.depth > 0 && !isOp && !isAttrCode(nt) {
			break
		}
		p.next()
		var it Item
		if isOp {
			it.Op = nt.Text
			p.next()
		}
		it.X = p.expr(relLevel)
		if c == nil {
			c = &Compare{span: b.span, Base: b}
		}
		c.Items = append(c.Items, it)
		_, c.E = it.X.Span()
	}
	if c != nil {
		return c
	}
	return b
}

// isAttrCode reports whether t is a two-letter attack attribute (NA, SA,
// HP, AT, ...), the items trgAttr reads after a comma.
func isAttrCode(t Token) bool {
	if t.Kind != TokIdent || len(t.Text) != 2 {
		return false
	}
	l := t.Lower()
	return (l[0] == 'n' || l[0] == 's' || l[0] == 'h' || l[0] == 'a') &&
		(l[1] == 'a' || l[1] == 't' || l[1] == 'p')
}

// tryRange parses "[lo, hi]" style ranges. A '(' that turns out to be a
// plain parenthesized operand (no comma) is left for the caller, like
// expRange does.
func (p *parser) tryRange(x Node, op string) Node {
	save, saveErrs := p.i, len(p.errs)
	open := p.next()
	p.depth++
	lo := p.expr(1)
	if !p.is(",") {
		p.depth--
		if open.Text == "(" {
			p.i, p.errs = save, p.errs[:saveErrs]
			return nil
		}
		p.errorf(p.tok(), "missing ',' in range")
		return &Range{span: spanOf(x, lo), X: x, Lo: lo, Hi: &BadExpr{}, Op: op, LoOpen: open.Text, HiClosed: "]"}
	}
	p.next()
	hi := p.expr(1)
	p.depth--
	r := &Range{X: x, Lo: lo, Hi: hi, Op: op, LoOpen: open.Text}
	if t := p.tok(); p.is("]") || p.is(")") {
		r.HiClosed = t.Text
		p.next()
		xp, _ := x.Span()
		r.span = span{xp, t.End}
	} else {
		p.errorf(t, "missing ']' or ')' in range")
		r.HiClosed = "]"
		r.span = spanOf(x, hi)
	}
	return r
}

func (p *parser) unary() Node {
	t := p.tok()
	if t.Kind == TokPunct {
		switch t.Text {
		case "-":
			p.next()
			// "-5" with no space is a negative literal (expValue case "-").
			if n := p.tok(); n.Kind == TokNumber && n.Pos == t.End {
				p.next()
				return p.postfix(&Num{span{t.Pos, n.End}, "-" + n.Text, isFloat(n.Text)})
			}
			x := p.unary()
			_, e := x.Span()
			return &Unary{span: span{t.Pos, e}, Op: "-", X: x}
		case "!", "~":
			p.next()
			x := p.unary()
			_, e := x.Span()
			return &Unary{span: span{t.Pos, e}, Op: t.Text, X: x}
		}
	}
	return p.postfix(p.primary())
}

// postfix handles the legacy CNS "x!" (expPostNot with ignoreMostErrors).
func (p *parser) postfix(x Node) Node {
	for !p.opt.ZSS && p.is("!") {
		t := p.next()
		xp, _ := x.Span()
		x = &Unary{span: span{xp, t.End}, Op: "!", X: x, Postfix: true}
	}
	return x
}

func isFloat(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' || s[i] == 'e' || s[i] == 'E' {
			return true
		}
	}
	return false
}

func (p *parser) primary() Node {
	t := p.tok()
	switch t.Kind {
	case TokNumber:
		p.next()
		return &Num{span{t.Pos, t.End}, t.Text, isFloat(t.Text)}
	case TokString:
		p.next()
		return &Str{span{t.Pos, t.End}, t.Text}
	case TokBad:
		p.next()
		p.errorf(t, "string not closed with \"")
		return &Str{span{t.Pos, t.End}, t.Text}
	case TokIdent:
		return p.name()
	case TokEOF:
		p.errorf(t, "missing value")
		return &BadExpr{span{t.Pos, t.Pos}}
	}
	if t.Text == "(" {
		p.next()
		p.depth++
		x := p.expr(1)
		p.depth--
		if p.is(")") {
			p.next()
		} else {
			p.errorf(p.tok(), "missing ')'")
		}
		return x
	}
	p.errorf(t, "unexpected %q", t.Text)
	// Skip the token unless it can close an enclosing construct.
	switch t.Text {
	case ")", "]", ",", "{", "}", ";", ":":
	default:
		p.next()
	}
	return &BadExpr{span{t.Pos, t.End}}
}

func (p *parser) name() Node {
	t := p.next()
	id := &Ident{span{t.Pos, t.End}, t.Lower(), t.Text}
	var x Node = id
	if p.is("(") {
		c := &Call{Name: id}
		p.next()
		p.depth++
		if !p.is(")") {
			for {
				c.Args = append(c.Args, p.expr(1))
				if !p.is(",") {
					break
				}
				p.next()
			}
		}
		p.depth--
		end := p.tok()
		if p.is(")") {
			p.next()
		} else {
			p.errorf(end, "missing ')' after arguments of %s", t.Text)
		}
		c.span = span{t.Pos, end.End}
		x = c
	}
	switch n := p.tok(); {
	case redirects[id.Name] && n.Kind == TokPunct && n.Text == ",":
		p.next()
		v := p.unary()
		return &Redirect{spanOf(x, v), x, v}
	case n.Kind == TokPunct && n.Text == ":=":
		p.next()
		v := p.expr(eqLevel)
		return &Assign{spanOf(x, v), x, v}
	case n.Kind == TokIdent:
		p.next()
		key := &Ident{span{n.Pos, n.End}, n.Lower(), n.Text}
		return &Sub{spanOf(x, key), x, key}
	}
	return x
}
