// Package zss parses ZSS (Zantei State Script), Ikemen GO's block-based
// state language, into a syntax tree with positions. It does not compile.
//
// Upstream: ikemen/source/src/compiler.go @ v1.0.0 (81c6da71d689625e20db79586815b695da00dd6d)
//
//	stateCompileZSS (compiler.go:7966) -> Parse (sections)
//	scanStateDef    (compiler.go:7332) -> statedef numbers
//	readKeyValue    (compiler.go:7235) -> params
//	readSentence    (compiler.go:7206) -> sentence
//	blockAttribSet  (compiler.go:7382) -> attrs
//	subBlock, switchBlock, loopBlock, callFunc, letAssign, stateBlock
//	                (compiler.go:7421-7965) -> statements
//
// Changes from upstream: the whole file is tokenized once (package expr's
// lexer with '#' comments) instead of line by line, and errors become
// diagnostics with recovery to the next statement, so one file shows all its
// problems. Ikemen's rule that a top-level statement must end its line is
// not enforced. Controller names are not checked: any name followed by '{'
// is a controller (see lsp/schema for the real list).
//
// MIT License
// Copyright (c) 2016 Suehiro
// Copyright (c) 2018 K4thos, NeatUnsou, ShinLucho
// Copyright (c) 2019 Daniel Porto, Gacel, Nikolay Griva
// See ikemen/source/LICENCE.txt for the full notice.
package zss

import (
	"os"
	"sort"
	"strings"

	"github.com/whoisnighter/KataFighter/formats/diag"
	"github.com/whoisnighter/KataFighter/formats/expr"
	"github.com/whoisnighter/KataFighter/formats/internal/text"
)

// Span is a half-open byte range in the source.
type Span struct{ Pos, End int }

// Param is "key: value" in a [StateDef] header or a controller.
type Param struct {
	Span
	Key, KeyRaw string // Key is lower-cased
	Value       string // the sentence after ':', trimmed
	ValuePos    int    // offset of Value in the source
}

// ParseValue parses the value as a comma list of expressions. Values that
// are not expressions (value: S 10, 0) may parse with errors.
func (p *Param) ParseValue() ([]expr.Node, []expr.Error) {
	return expr.ParseList(p.Value, expr.Options{ZSS: true})
}

// Expr is an expression with its location. Node offsets are relative to
// Pos, i.e. Node spans index into File.Source[Pos:].
type Expr struct {
	Pos  int
	Text string
	Node expr.Node
}

// Attrs are the block prefixes "ignorehitpause" and "persistent(N)".
type Attrs struct {
	IgnoreHitPause bool
	// Persistent is the N of persistent(N), or -1 when absent.
	Persistent int32
}

// Stmt is a statement.
type Stmt interface{ stmt() }

// Controller is "name{key: value; ...}".
type Controller struct {
	Span
	Name    string // lower-cased
	NameRaw string
	Params  []*Param
}

// Block is "{ ... }" with attributes (ignorehitpause { ... }).
type Block struct {
	Span
	Attrs Attrs
	Body  []Stmt
}

// If is "if cond { ... } else ...". Else holds a single *If for "else if".
type If struct {
	Span
	Attrs Attrs
	Cond  *Expr
	Then  []Stmt
	Else  []Stmt
}

// Switch is "switch x { case a; b: ... default: ... }".
type Switch struct {
	Span
	Attrs   Attrs
	Subject *Expr
	Cases   []*Case
}

// Case is one case (Default = true for "default").
type Case struct {
	Span
	Default bool
	Values  []*Expr
	Attrs   Attrs
	Body    []Stmt
}

// For is "for [i =] start; end [; step] { ... }".
type For struct {
	Span
	Attrs Attrs
	Var   string // "" when the loop has no variable
	Exprs []*Expr
	Body  []Stmt
}

// While is "while cond { ... }".
type While struct {
	Span
	Attrs Attrs
	Cond  *Expr
	Body  []Stmt
}

// Let is "let a, b = e1, e2;" or "let a, b = call f(x);".
type Let struct {
	Span
	Names  []string
	Values []*Expr // nil when Call is set
	Call   *Call
}

// Call is "call name(args);".
type Call struct {
	Span
	Name string
	Args []*Expr
}

// ExprStmt is an assignment expression ("var(1) := 2;").
type ExprStmt struct {
	Span
	X *Expr
}

// Break and Continue are loop jumps.
type Break struct{ Span }
type Continue struct{ Span }

func (*Controller) stmt() {}
func (*Block) stmt()      {}
func (*If) stmt()         {}
func (*Switch) stmt()     {}
func (*For) stmt()        {}
func (*While) stmt()      {}
func (*Let) stmt()        {}
func (*Call) stmt()       {}
func (*ExprStmt) stmt()   {}
func (*Break) stmt()      {}
func (*Continue) stmt()   {}

// State is a [StateDef N; params] section with its body.
type State struct {
	Span
	NumberText string
	Number     int32  // -10 for "+1"; 0 when Const is set
	Const      string // name of "const(name)"
	Params     []*Param
	Body       []Stmt
}

// Function is a [Function name(args) rets] section with its body.
type Function struct {
	Span
	Name    string
	Args    []string
	Returns []string
	Body    []Stmt
}

// File is a parsed ZSS file.
type File struct {
	Path        string
	Source      string
	States      []*State
	Functions   []*Function
	Diagnostics []diag.Diagnostic
	lineStarts  []int
}

// Position converts a byte offset to a 1-based line and column.
func (f *File) Position(off int) diag.Pos {
	i := sort.Search(len(f.lineStarts), func(i int) bool { return f.lineStarts[i] > off }) - 1
	if i < 0 {
		i = 0
	}
	return diag.Pos{Line: i + 1, Col: off - f.lineStarts[i] + 1}
}

// Load reads and parses a file.
func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f := Parse(string(text.StripBOM(b)))
	f.Path = path
	diag.SetFile(f.Diagnostics, path)
	return f, nil
}

type parser struct {
	f    *File
	toks []expr.Token
	i    int
	dl   diag.List
	// loop > 0 inside for/while bodies, for break/continue checks.
	loop int
	// inFunc is true inside a [Function], where persistent is not allowed.
	inFunc bool
}

// Parse parses ZSS text.
func Parse(src string) *File {
	f := &File{Source: src, lineStarts: []int{0}}
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			f.lineStarts = append(f.lineStarts, i+1)
		}
	}
	p := &parser{f: f, toks: expr.Lex(src, true)}
	p.file()
	sort.SliceStable(p.dl.Items, func(a, b int) bool {
		return p.dl.Items[a].Range.Start.Before(p.dl.Items[b].Range.Start)
	})
	f.Diagnostics = p.dl.Items
	return f
}

func (p *parser) tok() expr.Token { return p.toks[p.i] }
func (p *parser) next() expr.Token {
	t := p.toks[p.i]
	if p.i < len(p.toks)-1 {
		p.i++
	}
	return t
}
func (p *parser) eof() bool { return p.tok().Kind == expr.TokEOF }

// is reports whether the current token is the punctuation or keyword s
// (keywords compare case-insensitively).
func (p *parser) is(s string) bool {
	t := p.tok()
	if t.Kind == expr.TokPunct {
		return t.Text == s
	}
	return t.Kind == expr.TokIdent && strings.EqualFold(t.Text, s)
}

func (p *parser) rng(pos, end int) diag.Range {
	return diag.Range{Start: p.f.Position(pos), End: p.f.Position(max(end, pos+1))}
}

func (p *parser) errAt(t expr.Token, code, format string, a ...any) {
	// One error per token: the first one is the useful one.
	if n := len(p.dl.Items); n > 0 && p.dl.Items[n-1].Range.Start == p.f.Position(t.Pos) {
		return
	}
	p.dl.Errorf(p.rng(t.Pos, t.End), code, format, a...)
}

func (p *parser) expect(s string) bool {
	if p.is(s) {
		p.next()
		return true
	}
	t := p.tok()
	if t.Kind == expr.TokEOF {
		p.errAt(t, "syntax", "missing %q at end of file", s)
	} else {
		p.errAt(t, "syntax", "expected %q, found %q", s, t.Text)
	}
	return false
}

// sentence collects tokens up to the next ':', ';', '{' or '}' (and ']'
// when inHeader), like readSentence. It returns the trimmed text and its
// offset; the terminator is left as the current token.
func (p *parser) sentence(inHeader bool) (string, int) {
	start := p.tok().Pos
	end := start
	for !p.eof() {
		t := p.tok()
		if t.Kind == expr.TokPunct && (t.Text == ":" || t.Text == ";" || t.Text == "{" || t.Text == "}" || inHeader && t.Text == "]") {
			break
		}
		end = t.End
		p.next()
	}
	return p.f.Source[start:end], start
}

// expression parses a sentence as an expression and reports its errors.
func (p *parser) expression(inHeader bool) *Expr {
	txt, pos := p.sentence(inHeader)
	e := &Expr{Pos: pos, Text: txt}
	if strings.TrimSpace(txt) == "" {
		p.errAt(p.tok(), "syntax", "missing expression")
		return e
	}
	var errs []expr.Error
	e.Node, errs = expr.Parse(txt, expr.Options{ZSS: true})
	for _, er := range errs {
		p.dl.Add(diag.Diagnostic{Range: p.rng(pos+er.Pos, pos+er.End), Severity: diag.Error, Code: "syntax", Message: er.Msg})
	}
	return e
}

// recover skips to the token after the next ';' or to the next '}' / '[',
// whichever comes first, so parsing can continue after an error.
func (p *parser) recover() {
	for !p.eof() {
		switch {
		case p.is(";"):
			p.next()
			return
		case p.is("}"), p.is("["):
			return
		}
		p.next()
	}
}

func (p *parser) file() {
	seenState := map[string]int{}
	seenFunc := map[string]int{}
	for !p.eof() {
		if !p.is("[") {
			t := p.tok()
			p.errAt(t, "syntax", "expected a [StateDef] or [Function] section, found %q", t.Text)
			for !p.eof() && !p.is("[") {
				p.next()
			}
			continue
		}
		open := p.next()
		kw := p.next()
		switch strings.ToLower(kw.Text) {
		case "statedef":
			s := p.stateDef(open)
			key := strings.ToLower(strings.ReplaceAll(s.NumberText, " ", ""))
			if line, ok := seenState[key]; ok {
				p.dl.Errorf(p.rng(open.Pos, open.End), "duplicate-statedef", "state %s is already defined on line %d", s.NumberText, line)
			} else {
				seenState[key] = p.f.Position(open.Pos).Line
			}
			p.f.States = append(p.f.States, s)
		case "function":
			fn := p.function(open)
			if fn.Name != "" {
				if line, ok := seenFunc[fn.Name]; ok {
					p.dl.Errorf(p.rng(open.Pos, open.End), "duplicate-function", "function %s is already defined on line %d", fn.Name, line)
				} else {
					seenFunc[fn.Name] = p.f.Position(open.Pos).Line
				}
			}
			p.f.Functions = append(p.f.Functions, fn)
		default:
			p.errAt(kw, "syntax", "unknown section %q (expected StateDef or Function)", kw.Text)
			for !p.eof() && !p.is("[") {
				p.next()
			}
		}
	}
}

func (p *parser) stateDef(open expr.Token) *State {
	s := &State{Span: Span{Pos: open.Pos}}
	p.inFunc = false
	txt, pos := p.sentence(true)
	s.NumberText = strings.TrimSpace(txt)
	low := strings.ToLower(strings.ReplaceAll(s.NumberText, " ", ""))
	switch {
	case s.NumberText == "":
		p.errAt(p.tok(), "statedef-number", "missing state number")
	case strings.HasPrefix(low, "const(") && strings.HasSuffix(low, ")"):
		s.Const = low[len("const(") : len(low)-1]
	case low == "+1":
		s.Number = -10
	default:
		n, _ := text.Atoi(low)
		s.Number = n
		if !text.IsNumeric(low) {
			p.dl.Errorf(p.rng(pos, pos+len(txt)), "statedef-number", "invalid state number %q", s.NumberText)
		}
	}
	seen := map[string]bool{}
	for p.is(";") {
		p.next()
		if p.is("]") {
			break
		}
		if prm := p.param(true); prm != nil {
			if seen[prm.Key] {
				p.errAt(p.toks[p.i-1], "duplicate-key", "duplicate key %q", prm.Key)
			}
			seen[prm.Key] = true
			s.Params = append(s.Params, prm)
		}
	}
	if !p.is("]") {
		t := p.tok()
		p.errAt(t, "syntax", "expected ';' or ']' in the [StateDef] header, found %q", t.Text)
		for !p.eof() && !p.is("]") && !p.is("[") {
			p.next()
		}
	}
	if p.is("]") {
		p.next()
	}
	s.Body = p.stmts(true)
	s.End = p.tok().Pos
	return s
}

// param reads "key: value". The value ends at ';', '}' (and ']' in headers).
func (p *parser) param(inHeader bool) *Param {
	k := p.tok()
	if k.Kind != expr.TokIdent {
		p.errAt(k, "syntax", "expected a parameter name, found %q", k.Text)
		return nil
	}
	p.next()
	prm := &Param{Span: Span{Pos: k.Pos}, Key: strings.ToLower(k.Text), KeyRaw: k.Text}
	if !p.expect(":") {
		return nil
	}
	txt, pos := p.sentence(inHeader)
	prm.Value, prm.ValuePos = strings.TrimSpace(txt), pos
	prm.End = pos + len(txt)
	if inHeader && p.is("]") {
		// Ikemen keeps reading past ']' until the next ';' and swallows code.
		p.errAt(p.tok(), "missing-semicolon", "missing ';' before ']'")
	}
	return prm
}

func (p *parser) function(open expr.Token) *Function {
	fn := &Function{Span: Span{Pos: open.Pos}}
	p.inFunc = true
	if name := p.tok(); name.Kind == expr.TokIdent {
		fn.Name = name.Lower()
		p.next()
		p.checkVarName(name)
	} else {
		p.errAt(name, "syntax", "missing function name")
	}
	if p.expect("(") {
		fn.Args = p.varNames(")")
	}
	fn.Returns = p.varNames("]")
	for _, r := range fn.Returns {
		if r == "_" {
			p.errAt(p.toks[p.i-1], "syntax", "a return value cannot be named _")
		}
	}
	fn.Body = p.stmts(true)
	fn.End = p.tok().Pos
	return fn
}

// checkVarName mirrors varNameCheck: [a-z_][0-9a-z_]* (after lower-casing).
func (p *parser) checkVarName(t expr.Token) bool {
	nm := t.Lower()
	ok := nm != "" && (nm[0] >= 'a' && nm[0] <= 'z' || nm[0] == '_')
	for i := 1; ok && i < len(nm); i++ {
		c := nm[i]
		ok = c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_'
	}
	if !ok {
		p.errAt(t, "invalid-name", "invalid name %q", t.Text)
	}
	return ok
}

// varNames reads "a, b, c" up to end (consumed), like varNames.
func (p *parser) varNames(end string) []string {
	var names []string
	if p.is(end) {
		p.next()
		return names
	}
	for {
		t := p.tok()
		if t.Kind != expr.TokIdent {
			p.errAt(t, "syntax", "expected a name, found %q", t.Text)
			for !p.eof() && !p.is(end) && !p.is("[") && !p.is("{") && !p.is(";") {
				p.next()
			}
			if p.is(end) {
				p.next()
			}
			return names
		}
		p.next()
		p.checkVarName(t)
		nm := t.Lower()
		if nm != "_" {
			for _, n := range names {
				if n == nm {
					p.errAt(t, "duplicate-name", "duplicate name %q", t.Text)
				}
			}
		}
		names = append(names, nm)
		if p.is(",") {
			p.next()
			continue
		}
		p.expect(end)
		return names
	}
}

// stmts reads statements until '}', case/default (inside blocks), '[' or
// EOF (at the root). The terminator is not consumed.
func (p *parser) stmts(root bool) []Stmt {
	var out []Stmt
	for {
		switch {
		case p.eof(), p.is("["):
			if !root {
				p.errAt(p.tok(), "syntax", "missing '}'")
			}
			return out
		case p.is("}"):
			if root {
				p.errAt(p.tok(), "syntax", "unexpected '}'")
				p.next()
				continue
			}
			return out
		case !root && (p.is("case") || p.is("default")):
			return out
		}
		before := p.i
		if s := p.stmt(); s != nil {
			out = append(out, s)
		}
		if p.i == before { // no progress: skip the token
			p.next()
		}
	}
}

func (p *parser) attrs() (Attrs, bool) {
	a := Attrs{Persistent: -1}
	any := false
	for {
		switch {
		case p.is("ignorehitpause"):
			t := p.next()
			if a.IgnoreHitPause {
				p.errAt(t, "syntax", "ignorehitpause given twice")
			}
			a.IgnoreHitPause, any = true, true
		case p.is("persistent"):
			t := p.next()
			if p.inFunc {
				p.errAt(t, "syntax", "persistent cannot be used in a function")
			}
			if a.Persistent >= 0 {
				p.errAt(t, "syntax", "persistent given twice")
			}
			any = true
			if !p.expect("(") {
				continue
			}
			neg := false
			if p.is("-") {
				p.next()
				neg = true
			}
			n := p.tok()
			if n.Kind != expr.TokNumber {
				p.errAt(n, "syntax", "persistent needs an integer")
			} else {
				p.next()
				v, _ := text.Atoi(n.Text)
				if neg {
					v = -v
				}
				if v <= 0 {
					v = 1<<31 - 1 // Ikemen: <= 0 means "always"
				}
				a.Persistent = v
			}
			p.expect(")")
		default:
			return a, any
		}
	}
}

func (p *parser) stmt() Stmt {
	start := p.tok()
	attrs, hasAttrs := p.attrs()
	t := p.tok()
	switch {
	case p.is("{"):
		p.next()
		b := &Block{Span: Span{Pos: start.Pos}, Attrs: attrs, Body: p.stmts(false)}
		b.End = p.tok().End
		p.expect("}")
		return b
	case p.is("if"):
		return p.ifStmt(start, attrs)
	case p.is("switch"):
		return p.switchStmt(start, attrs)
	case p.is("for"):
		return p.forStmt(start, attrs)
	case p.is("while"):
		p.next()
		w := &While{Span: Span{Pos: start.Pos}, Attrs: attrs, Cond: p.expression(false)}
		w.Body, w.End = p.body(true)
		return w
	case hasAttrs:
		p.errAt(t, "syntax", "expected if, switch, for, while or '{' after a block attribute, found %q", t.Text)
		p.recover()
		return nil
	case p.is("call"):
		p.next()
		c := p.call(start)
		p.expect(";")
		return c
	case p.is("let"):
		return p.let(start)
	case p.is("break"), p.is("continue"):
		p.next()
		if p.loop == 0 {
			p.errAt(t, "syntax", "%s can only be used inside a loop", t.Lower())
		}
		end := p.tok().End
		p.expect(";")
		if strings.EqualFold(t.Text, "break") {
			return &Break{Span{t.Pos, end}}
		}
		return &Continue{Span{t.Pos, end}}
	case t.Kind == expr.TokIdent && p.toks[min(p.i+1, len(p.toks)-1)].Kind == expr.TokPunct && p.toks[min(p.i+1, len(p.toks)-1)].Text == "{":
		return p.controller()
	}
	// Expression statement: must be an assignment.
	x := p.expression(false)
	s := &ExprStmt{Span: Span{start.Pos, x.Pos + len(x.Text)}, X: x}
	if x.Node != nil && !hasAssign(x.Node) {
		p.dl.Errorf(p.rng(x.Pos, x.Pos+len(x.Text)), "unused-value", "expression with unused value (expected an assignment with :=)")
	}
	if !p.is(";") {
		p.errAt(p.tok(), "syntax", "missing ';' after expression")
		p.recover()
		return s
	}
	p.next()
	return s
}

func hasAssign(n expr.Node) bool {
	found := false
	expr.Walk(n, func(n expr.Node) bool {
		if _, ok := n.(*expr.Assign); ok {
			found = true
		}
		return !found
	})
	return found
}

// body reads "{ stmts }" and returns the statements and the end offset.
func (p *parser) body(loop bool) ([]Stmt, int) {
	if !p.expect("{") {
		p.recover()
		return nil, p.tok().Pos
	}
	if loop {
		p.loop++
		defer func() { p.loop-- }()
	}
	b := p.stmts(false)
	end := p.tok().End
	p.expect("}")
	return b, end
}

func (p *parser) ifStmt(start expr.Token, attrs Attrs) *If {
	p.next() // if
	s := &If{Span: Span{Pos: start.Pos}, Attrs: attrs, Cond: p.expression(false)}
	s.Then, s.End = p.body(false)
	if p.is("else") {
		p.next()
		st := p.tok()
		a, _ := p.attrs()
		switch {
		case p.is("if"):
			e := p.ifStmt(st, a)
			s.Else = []Stmt{e}
			s.End = e.End
		case p.is("{"):
			if a != (Attrs{Persistent: -1}) {
				b := &Block{Span: Span{Pos: st.Pos}, Attrs: a}
				b.Body, b.End = p.body(false)
				s.Else, s.End = []Stmt{b}, b.End
			} else {
				s.Else, s.End = p.body(false)
			}
		default:
			p.errAt(p.tok(), "syntax", "expected if or '{' after else, found %q", p.tok().Text)
			p.recover()
		}
	}
	return s
}

func (p *parser) switchStmt(start expr.Token, attrs Attrs) *Switch {
	p.next() // switch
	s := &Switch{Span: Span{Pos: start.Pos}, Attrs: attrs, Subject: p.expression(false)}
	if !p.expect("{") {
		p.recover()
		return s
	}
	hasDefault := false
	for !p.is("}") && !p.eof() && !p.is("[") {
		ct := p.tok()
		c := &Case{Span: Span{Pos: ct.Pos}}
		switch {
		case p.is("case"):
			p.next()
			for {
				c.Values = append(c.Values, p.expression(false))
				if p.is(";") {
					p.next()
					continue
				}
				break
			}
			p.expect(":")
		case p.is("default"):
			p.next()
			if hasDefault {
				p.errAt(ct, "syntax", "default already defined")
			}
			hasDefault = true
			c.Default = true
			if p.is(":") {
				p.next()
			}
		default:
			p.errAt(ct, "syntax", "expected case or default, found %q", ct.Text)
			p.recover()
			continue
		}
		c.Attrs, _ = p.attrs()
		c.Body = p.stmts(false)
		c.End = p.tok().Pos
		s.Cases = append(s.Cases, c)
	}
	s.End = p.tok().End
	p.expect("}")
	return s
}

func (p *parser) forStmt(start expr.Token, attrs Attrs) *For {
	p.next() // for
	s := &For{Span: Span{Pos: start.Pos}, Attrs: attrs}
	// "for i = a; b; c": a name followed by '='.
	if t := p.tok(); t.Kind == expr.TokIdent && p.toks[min(p.i+1, len(p.toks)-1)].Text == "=" {
		p.next()
		p.next()
		p.checkVarName(t)
		s.Var = t.Lower()
	}
	for len(s.Exprs) < 3 {
		s.Exprs = append(s.Exprs, p.expression(false))
		if !p.is(";") {
			break
		}
		p.next()
	}
	if len(s.Exprs) < 2 {
		p.errAt(p.tok(), "syntax", "a for loop needs at least two expressions")
	}
	s.Body, s.End = p.body(true)
	return s
}

func (p *parser) call(start expr.Token) *Call {
	c := &Call{Span: Span{Pos: start.Pos}}
	name := p.tok()
	if name.Kind != expr.TokIdent {
		p.errAt(name, "syntax", "missing function name after call")
		p.recover()
		return c
	}
	p.next()
	c.Name = name.Lower()
	if !p.expect("(") {
		p.recover()
		return c
	}
	// Arguments: expressions separated by top-level commas, up to the
	// matching ')'.
	if p.is(")") {
		c.End = p.next().End
		return c
	}
	for {
		argStart := p.tok().Pos
		depth := 0
		for !p.eof() {
			t := p.tok()
			if t.Kind == expr.TokPunct {
				if t.Text == "(" || t.Text == "[" {
					depth++
				} else if (t.Text == ")" || t.Text == "]") && depth > 0 {
					depth--
				} else if depth == 0 && (t.Text == "," || t.Text == ")" || t.Text == ";" || t.Text == "{" || t.Text == "}") {
					break
				}
			}
			p.next()
		}
		txt := p.f.Source[argStart:p.tok().Pos]
		e := &Expr{Pos: argStart, Text: strings.TrimRight(txt, " \t\r\n")}
		var errs []expr.Error
		e.Node, errs = expr.Parse(e.Text, expr.Options{ZSS: true})
		for _, er := range errs {
			p.dl.Add(diag.Diagnostic{Range: p.rng(argStart+er.Pos, argStart+er.End), Severity: diag.Error, Code: "syntax", Message: er.Msg})
		}
		c.Args = append(c.Args, e)
		if p.is(",") {
			p.next()
			continue
		}
		break
	}
	c.End = p.tok().End
	p.expect(")")
	return c
}

func (p *parser) let(start expr.Token) *Let {
	p.next() // let
	l := &Let{Span: Span{Pos: start.Pos}}
	l.Names = p.varNames("=")
	if len(l.Names) == 0 {
		p.recover()
		return l
	}
	if p.is("call") {
		ct := p.next()
		l.Call = p.call(ct)
	} else {
		if len(l.Names) == 1 {
			l.Values = []*Expr{p.expression(false)}
		} else {
			// One expression per name, separated by top-level commas.
			txt, pos := p.sentence(false)
			list, errs := expr.ParseList(txt, expr.Options{ZSS: true})
			for _, er := range errs {
				p.dl.Add(diag.Diagnostic{Range: p.rng(pos+er.Pos, pos+er.End), Severity: diag.Error, Code: "syntax", Message: er.Msg})
			}
			if len(list) != len(l.Names) {
				p.dl.Errorf(p.rng(pos, pos+len(txt)), "let-count", "%d names but %d values", len(l.Names), len(list))
			}
			for _, n := range list {
				// Node offsets stay relative to pos, as for every Expr.
				l.Values = append(l.Values, &Expr{Pos: pos, Text: txt, Node: n})
			}
		}
	}
	l.End = p.tok().End
	if !p.expect(";") {
		p.recover()
	}
	return l
}

func (p *parser) controller() *Controller {
	name := p.next()
	c := &Controller{Span: Span{Pos: name.Pos}, Name: name.Lower(), NameRaw: name.Text}
	p.next() // {
	seen := map[string]bool{}
	for !p.is("}") && !p.eof() && !p.is("[") {
		if prm := p.param(false); prm != nil {
			if seen[prm.Key] {
				p.dl.Errorf(p.rng(prm.Pos, prm.Pos+len(prm.KeyRaw)), "duplicate-key", "duplicate key %q", prm.Key)
			}
			seen[prm.Key] = true
			c.Params = append(c.Params, prm)
		}
		if p.is(";") {
			p.next()
			continue
		}
		if !p.is("}") {
			t := p.tok()
			p.errAt(t, "syntax", "expected ';' or '}' in %s, found %q", c.NameRaw, t.Text)
			p.recover()
		}
	}
	c.End = p.tok().End
	p.expect("}")
	return c
}
