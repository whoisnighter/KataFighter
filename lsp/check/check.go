// Package check reports what the formats parsers leave to the LSP: names
// that must exist in the engine. It checks controller types, controller
// parameters and trigger names against lsp/schema.
//
// Severities follow Ikemen GO (compiler.go): an unknown controller type
// ("Invalid state controller") and an unknown trigger ("Invalid data") stop
// the character from loading, so they are errors; unknown parameters are
// only logged when ignoreMostErrors is on (the default), so they are
// warnings.
//
// Not checked yet: mandatory parameters. The schema's mandatory flag does
// not record when a parameter is only required in some cases (Width needs
// value only without edge/player), so checking it would report false errors.
package check

import (
	"sort"
	"strings"

	"github.com/whoisnighter/KataFighter/formats/cns"
	"github.com/whoisnighter/KataFighter/formats/diag"
	"github.com/whoisnighter/KataFighter/formats/expr"
	"github.com/whoisnighter/KataFighter/formats/zss"
	"github.com/whoisnighter/KataFighter/lsp/schema"
)

// Diagnostic codes.
const (
	CodeUnknownController = "unknown-controller"
	CodeUnknownParam      = "unknown-param"
	CodeUnknownTrigger    = "unknown-trigger"
)

// Kind is the language of a file, chosen by extension.
type Kind int

const (
	KindNone Kind = iota
	KindCNS       // .cns, .st, .cmd, .const
	KindZSS       // .zss
)

// KindOf returns the language of a file name.
func KindOf(name string) Kind {
	i := strings.LastIndexByte(name, '.')
	if i < 0 {
		return KindNone
	}
	switch strings.ToLower(name[i:]) {
	case ".cns", ".st", ".cmd", ".const":
		return KindCNS
	case ".zss":
		return KindZSS
	}
	return KindNone
}

// Text parses src as the language of kind and returns the parser's
// diagnostics followed by the schema ones, sorted by position. File names
// are left empty.
func Text(kind Kind, src string, s *schema.Schema) []diag.Diagnostic {
	src = strings.TrimPrefix(src, string(rune(0xFEFF)))
	var ds []diag.Diagnostic
	switch kind {
	case KindCNS:
		f := cns.Parse(src)
		ds = append(f.Diagnostics, CNS(f, s)...)
	case KindZSS:
		f := zss.Parse(src)
		ds = append(f.Diagnostics, ZSS(f, s)...)
	default:
		return nil
	}
	sort.SliceStable(ds, func(a, b int) bool { return ds[a].Range.Start.Before(ds[b].Range.Start) })
	return ds
}

// CNS checks a parsed CNS, ST or CMD file. It returns only the schema
// diagnostics; the parser's own are in f.Diagnostics.
func CNS(f *cns.File, s *schema.Schema) []diag.Diagnostic {
	c := &checker{s: s, dl: diag.List{File: f.Path}}
	for _, sd := range f.States {
		for _, ctrl := range sd.Controllers {
			c.cnsController(ctrl)
		}
	}
	return c.done()
}

// ZSS checks a parsed ZSS file. It returns only the schema diagnostics; the
// parser's own are in f.Diagnostics.
func ZSS(f *zss.File, s *schema.Schema) []diag.Diagnostic {
	c := &checker{s: s, dl: diag.List{File: f.Path}}
	z := &zssChecker{checker: c, f: f}
	for _, st := range f.States {
		z.stmts(st.Body)
	}
	for _, fn := range f.Functions {
		z.stmts(fn.Body)
	}
	return c.done()
}

type checker struct {
	s  *schema.Schema
	dl diag.List
	// inParam is set while checking a parameter value, where a name at the
	// very start may be a data prefix (see param).
	inParam bool
}

func (c *checker) done() []diag.Diagnostic {
	sort.SliceStable(c.dl.Items, func(a, b int) bool {
		return c.dl.Items[a].Range.Start.Before(c.dl.Items[b].Range.Start)
	})
	return c.dl.Items
}

// rangeFunc maps a byte span of the text an expression was parsed from to
// a file range.
type rangeFunc func(pos, end int) diag.Range

func (c *checker) cnsController(ctrl *cns.Controller) {
	for _, e := range ctrl.TriggerAll {
		c.trigger(e)
	}
	for _, group := range ctrl.Triggers {
		for _, e := range group {
			c.trigger(e)
		}
	}
	if ctrl.Type == "" {
		return // the parser reports the missing type
	}
	sc := c.s.Controller(ctrl.Type)
	if sc == nil {
		te := ctrl.Get("type")
		c.dl.Errorf(te.Range(0, len(te.Value)), CodeUnknownController,
			"unknown state controller type %q", te.Value)
		return
	}
	for _, e := range ctrl.Params {
		if c.s.IsCNSKeyword(e.Key) {
			continue
		}
		c.param(sc, e.Key, e.KeyRaw,
			diag.Range{Start: diag.Pos{Line: e.Line, Col: e.KeyCol}, End: diag.Pos{Line: e.Line, Col: e.KeyCol + len(e.KeyRaw)}},
			e.ParseValue, e.Range)
	}
}

func (c *checker) trigger(e *cns.Entry) {
	if e.Expr != nil {
		c.expr(e.Expr, e.Range)
	}
}

// param checks one controller parameter: that the controller has it and,
// for expression parameters, the trigger names in its value.
func (c *checker) param(sc *schema.Controller, key, keyRaw string, keyRange diag.Range,
	parse func() ([]expr.Node, []expr.Error), rng rangeFunc) {
	p := sc.Param(key)
	if p == nil {
		if len(sc.Unresolved) == 0 {
			c.dl.Warnf(keyRange, CodeUnknownParam,
				"unknown parameter %q for %s; Ikemen ignores it", keyRaw, sc.Name)
		}
		return
	}
	if p.Type == "" {
		return // not a plain expression (alpha, trans, attr, ...)
	}
	nodes, errs := parse()
	if len(errs) > 0 {
		return // a value the expression grammar does not cover; nothing reliable to check
	}
	// Ikemen strips a data prefix from the start of anim and sound values
	// (getDataPrefix): F, S, M and the common FX prefixes loaded at run time
	// (GO for gofx.def, ...). So "F5", "S $bank" and "F(60 + n)" are valid,
	// and since the FX prefixes are not known statically, the name at
	// offset 0 is never reported.
	c.inParam = true
	for _, n := range nodes {
		c.expr(n, rng)
	}
	c.inParam = false
}

// expr reports unknown trigger names in an expression. Names that a trigger
// reads itself are skipped: the keyword after a trigger (vel x), bare names
// passed as arguments (map(combo), const(StateDizzy), inputtime(B),
// gethitvar(damage)), and bare names compared with = or != (statetype = S,
// hitdefattr = SC, NA). Their meaning depends on the trigger, and the schema
// does not say which triggers read a raw name, so checking them would report
// false errors.
func (c *checker) expr(n expr.Node, rng rangeFunc) {
	switch n := n.(type) {
	case *expr.Ident:
		c.name(n, rng)
	case *expr.Call:
		c.name(n.Name, rng)
		for _, a := range n.Args {
			if _, ok := a.(*expr.Ident); ok {
				continue
			}
			c.expr(a, rng)
		}
	case *expr.Sub:
		c.expr(n.X, rng)
	case *expr.Redirect:
		c.expr(n.Target, rng)
		c.expr(n.X, rng)
	case *expr.Unary:
		c.expr(n.X, rng)
	case *expr.Binary:
		c.expr(n.X, rng)
		if _, ok := n.Y.(*expr.Ident); ok && (n.Op == "=" || n.Op == "!=") {
			return
		}
		c.expr(n.Y, rng)
	case *expr.Range:
		c.expr(n.X, rng)
		c.expr(n.Lo, rng)
		c.expr(n.Hi, rng)
	case *expr.Compare:
		c.expr(n.Base, rng)
		for _, it := range n.Items {
			if _, ok := it.X.(*expr.Ident); ok {
				continue
			}
			c.expr(it.X, rng)
		}
	case *expr.Assign:
		c.expr(n.Target, rng)
		c.expr(n.Value, rng)
	}
}

// name checks a trigger name and returns its schema entry (nil if unknown).
func (c *checker) name(id *expr.Ident, rng rangeFunc) *schema.Trigger {
	if strings.HasPrefix(id.Raw, "$") {
		return nil // ZSS local variable; the parser checks declarations
	}
	t := c.s.Trigger(id.Name)
	if pos, end := id.Span(); t == nil && !(c.inParam && pos == 0) {
		c.dl.Errorf(rng(pos, end), CodeUnknownTrigger, "unknown trigger %q", id.Raw)
	}
	return t
}

type zssChecker struct {
	*checker
	f *zss.File
}

func (z *zssChecker) rng(base int) rangeFunc {
	return func(pos, end int) diag.Range {
		return diag.Range{Start: z.f.Position(base + pos), End: z.f.Position(base + end)}
	}
}

func (z *zssChecker) x(e *zss.Expr) {
	if e != nil && e.Node != nil {
		z.expr(e.Node, z.rng(e.Pos))
	}
}

func (z *zssChecker) stmts(list []zss.Stmt) {
	for _, s := range list {
		z.stmt(s)
	}
}

func (z *zssChecker) stmt(s zss.Stmt) {
	switch s := s.(type) {
	case *zss.Controller:
		z.controller(s)
	case *zss.Block:
		z.stmts(s.Body)
	case *zss.If:
		z.x(s.Cond)
		z.stmts(s.Then)
		z.stmts(s.Else)
	case *zss.Switch:
		z.x(s.Subject)
		for _, cs := range s.Cases {
			for _, v := range cs.Values {
				z.x(v)
			}
			z.stmts(cs.Body)
		}
	case *zss.For:
		for _, e := range s.Exprs {
			z.x(e)
		}
		z.stmts(s.Body)
	case *zss.While:
		z.x(s.Cond)
		z.stmts(s.Body)
	case *zss.Let:
		for _, e := range s.Values {
			z.x(e)
		}
		if s.Call != nil {
			for _, e := range s.Call.Args {
				z.x(e)
			}
		}
	case *zss.Call:
		for _, e := range s.Args {
			z.x(e)
		}
	case *zss.ExprStmt:
		z.x(s.X)
	}
}

func (z *zssChecker) controller(ctrl *zss.Controller) {
	sc := z.s.Controller(ctrl.Name)
	if sc == nil {
		z.dl.Errorf(z.rng(ctrl.Pos)(0, len(ctrl.NameRaw)), CodeUnknownController,
			"unknown state controller type %q", ctrl.NameRaw)
		return
	}
	for _, p := range ctrl.Params {
		z.param(sc, p.Key, p.KeyRaw, z.rng(p.Pos)(0, len(p.KeyRaw)), p.ParseValue, z.rng(p.ValuePos))
	}
}
