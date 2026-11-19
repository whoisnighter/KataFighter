package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Schema is the generated description of what Ikemen's CNS/ZSS compiler accepts.
type Schema struct {
	Source struct {
		Engine  string `json:"engine"`
		Version string `json:"version"`
		Commit  string `json:"commit"`
	} `json:"source"`
	Generator string `json:"generator"`
	// CNSKeywords are accepted in every CNS state controller section.
	CNSKeywords []string     `json:"cnsKeywords"`
	Controllers []Controller `json:"controllers"`
	Triggers    []Trigger    `json:"triggers"`
	Warnings    []string     `json:"warnings,omitempty"`
}

// Controller is one state controller type (scmap entry).
type Controller struct {
	Name   string  `json:"name"`
	Origin string  `json:"origin"` // "mugen" or "ikemen"
	Source string  `json:"source"` // compile function, file:line in Ikemen
	Params []Param `json:"params"`
	// Unresolved lists parameter-name expressions the generator could not
	// evaluate statically (e.g. built with fmt.Sprintf).
	Unresolved []string `json:"unresolved,omitempty"`
}

// Param is one state controller parameter.
type Param struct {
	Name      string `json:"name"`
	Type      string `json:"type,omitempty"`  // Int, Float, Bool, ... (from VT_*), empty if custom
	Count     int    `json:"count,omitempty"` // max number of comma-separated values, 0 if unknown
	Mandatory bool   `json:"mandatory,omitempty"`
}

// Trigger is one name accepted in expressions.
type Trigger struct {
	Name   string `json:"name"`
	Origin string `json:"origin"` // "redirection", "mugen", "ikemen", or "other" (accepted but not in triggerMap)
	// Keywords are the literal arguments the compiler recognizes, e.g. gethitvar(damage).
	Keywords []string `json:"keywords,omitempty"`
}

type source struct {
	fset    *token.FileSet
	files   []*ast.File
	methods map[string]*ast.FuncDecl // CharCompiler methods
}

// Generate parses the compiler sources in srcDir (ikemen/source/src).
func Generate(srcDir string) (*Schema, error) {
	src := &source{fset: token.NewFileSet(), methods: map[string]*ast.FuncDecl{}}
	matches, _ := filepath.Glob(filepath.Join(srcDir, "compiler*.go"))
	if len(matches) == 0 {
		return nil, fmt.Errorf("no compiler*.go in %s", srcDir)
	}
	for _, p := range matches {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		f, err := parser.ParseFile(src.fset, filepath.Base(p), b, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		src.files = append(src.files, f)
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && isCharCompilerRecv(fd) {
				src.methods[fd.Name.Name] = fd
			}
		}
	}

	s := &Schema{Generator: "tools/schemagen", CNSKeywords: []string{"type", "persistent", "ignorehitpause", "triggerall", "trigger<N>"}}
	s.Source.Engine = "Ikemen GO"

	scmap, err := src.mapLiteral("scmap", map[string]string{"mugen state controllers": "mugen", "ikemen state controllers": "ikemen"})
	if err != nil {
		return nil, err
	}
	for _, e := range scmap {
		c := Controller{Name: e.key, Origin: e.group}
		fd := src.methods[e.value]
		if fd == nil {
			s.Warnings = append(s.Warnings, fmt.Sprintf("controller %s: compile function %s not found", e.key, e.value))
		} else {
			pos := src.fset.Position(fd.Pos())
			c.Source = fmt.Sprintf("%s:%d (%s)", pos.Filename, pos.Line, e.value)
			w := &walker{src: src, ctrl: &c, seen: map[string]bool{}, index: map[string]int{}}
			w.walkFunc(fd, env{})
		}
		if c.Params == nil {
			c.Params = []Param{}
		}
		s.Controllers = append(s.Controllers, c)
	}
	sort.Slice(s.Controllers, func(i, j int) bool { return s.Controllers[i].Name < s.Controllers[j].Name })

	tmap, err := src.mapLiteral("triggerMap", map[string]string{"redirections": "redirection", "mugen triggers": "mugen", "ikemen triggers": "ikemen"})
	if err != nil {
		return nil, err
	}
	keywords, err := src.triggerKeywords()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, e := range tmap {
		seen[e.key] = true
		s.Triggers = append(s.Triggers, Trigger{Name: e.key, Origin: e.group, Keywords: keywords[e.key]})
		if _, ok := keywords[e.key]; !ok && e.group != "redirection" {
			s.Warnings = append(s.Warnings, fmt.Sprintf("trigger %s is in triggerMap but has no case in expValue", e.key))
		}
	}
	for name, kw := range keywords {
		if !seen[name] {
			s.Triggers = append(s.Triggers, Trigger{Name: name, Origin: "other", Keywords: kw})
		}
	}
	sort.Slice(s.Triggers, func(i, j int) bool { return s.Triggers[i].Name < s.Triggers[j].Name })
	return s, nil
}

func isCharCompilerRecv(fd *ast.FuncDecl) bool {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return false
	}
	st, ok := fd.Recv.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	id, ok := st.X.(*ast.Ident)
	return ok && id.Name == "CharCompiler"
}

type mapEntry struct{ key, value, group string }

// mapLiteral finds `name = map[...]...{...}` (assignment or var) and returns
// its string keys in source order, with the group taken from the most recent
// comment line inside the literal.
func (src *source) mapLiteral(name string, groups map[string]string) ([]mapEntry, error) {
	for _, f := range src.files {
		var lit *ast.CompositeLit
		ast.Inspect(f, func(n ast.Node) bool {
			if lit != nil {
				return false
			}
			switch n := n.(type) {
			case *ast.AssignStmt:
				if len(n.Lhs) == 1 && len(n.Rhs) == 1 {
					if sel, ok := n.Lhs[0].(*ast.SelectorExpr); ok && sel.Sel.Name == name {
						lit, _ = n.Rhs[0].(*ast.CompositeLit)
					}
				}
			case *ast.ValueSpec:
				if len(n.Names) == 1 && n.Names[0].Name == name && len(n.Values) == 1 {
					lit, _ = n.Values[0].(*ast.CompositeLit)
				}
			}
			return true
		})
		if lit == nil {
			continue
		}
		var comments []*ast.Comment
		for _, cg := range f.Comments {
			for _, c := range cg.List {
				if c.Pos() > lit.Lbrace && c.Pos() < lit.Rbrace {
					comments = append(comments, c)
				}
			}
		}
		var out []mapEntry
		for _, el := range lit.Elts {
			kv, ok := el.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := stringLit(kv.Key)
			if !ok {
				continue
			}
			e := mapEntry{key: key}
			switch v := kv.Value.(type) {
			case *ast.SelectorExpr:
				e.value = v.Sel.Name
			case *ast.BasicLit:
				e.value = v.Value
			}
			for _, c := range comments {
				if c.Pos() < kv.Pos() {
					if g, ok := groups[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(c.Text, "//")))]; ok {
						e.group = g
					}
				}
			}
			out = append(out, e)
		}
		return out, nil
	}
	return nil, fmt.Errorf("map literal %s not found", name)
}

var keywordRe = regexp.MustCompile(`^[a-z_][a-z0-9_.]*$`)

// triggerKeywords reads expValue's main `switch c.token` and returns, for each
// trigger case, the literal keywords matched by nested `switch c.token` blocks.
func (src *source) triggerKeywords() (map[string][]string, error) {
	fd := src.methods["expValue"]
	if fd == nil {
		return nil, fmt.Errorf("expValue not found")
	}
	var main *ast.SwitchStmt
	for _, st := range fd.Body.List {
		if sw, ok := st.(*ast.SwitchStmt); ok && isCToken(sw.Tag) && len(sw.Body.List) > 100 {
			main = sw
			break
		}
	}
	if main == nil {
		return nil, fmt.Errorf("main switch in expValue not found")
	}
	out := map[string][]string{}
	for _, st := range main.Body.List {
		cc := st.(*ast.CaseClause)
		var kws []string
		seen := map[string]bool{}
		for _, b := range cc.Body {
			ast.Inspect(b, func(n ast.Node) bool {
				sw, ok := n.(*ast.SwitchStmt)
				if !ok || !isCToken(sw.Tag) {
					return true
				}
				for _, st := range sw.Body.List {
					for _, e := range st.(*ast.CaseClause).List {
						if s, ok := stringLit(e); ok && keywordRe.MatchString(s) && !seen[s] {
							seen[s] = true
							kws = append(kws, s)
						}
					}
				}
				return true
			})
		}
		for _, e := range cc.List {
			// Skip operator cases ("!", "-", "(" ...) that share the switch
			if name, ok := stringLit(e); ok && keywordRe.MatchString(name) {
				out[name] = kws
			}
		}
	}
	return out, nil
}

func isCToken(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "token" {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "c"
}

func stringLit(e ast.Expr) (string, bool) {
	bl, ok := e.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(bl.Value)
	return s, err == nil
}

// env binds string variables to constant values while walking.
type env map[string]string

func (e env) with(k, v string) env {
	n := env{}
	for a, b := range e {
		n[a] = b
	}
	n[k] = v
	return n
}

// eval resolves a constant string expression: literals, bound identifiers and +.
func (e env) eval(x ast.Expr) (string, bool) {
	switch x := x.(type) {
	case *ast.BasicLit:
		return stringLit(x)
	case *ast.Ident:
		v, ok := e[x.Name]
		return v, ok
	case *ast.ParenExpr:
		return e.eval(x.X)
	case *ast.BinaryExpr:
		if x.Op == token.ADD {
			a, ok1 := e.eval(x.X)
			b, ok2 := e.eval(x.Y)
			return a + b, ok1 && ok2
		}
	}
	return "", false
}

type walker struct {
	src   *source
	ctrl  *Controller
	seen  map[string]bool // method+env already walked
	index map[string]int  // param name -> index in ctrl.Params
}

func (w *walker) add(p Param) {
	if i, ok := w.index[p.Name]; ok {
		q := &w.ctrl.Params[i]
		if q.Type == "" {
			q.Type, q.Count = p.Type, p.Count
		}
		q.Mandatory = q.Mandatory || p.Mandatory
		return
	}
	w.index[p.Name] = len(w.ctrl.Params)
	w.ctrl.Params = append(w.ctrl.Params, p)
}

func (w *walker) unresolved(x ast.Expr) {
	var sb strings.Builder
	sb.WriteString(exprString(x))
	pos := w.src.fset.Position(x.Pos())
	s := fmt.Sprintf("%s (%s:%d)", sb.String(), pos.Filename, pos.Line)
	for _, u := range w.ctrl.Unresolved {
		if u == s {
			return
		}
	}
	w.ctrl.Unresolved = append(w.ctrl.Unresolved, s)
}

func exprString(x ast.Expr) string {
	switch x := x.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.BasicLit:
		return x.Value
	case *ast.BinaryExpr:
		return exprString(x.X) + " " + x.Op.String() + " " + exprString(x.Y)
	case *ast.CallExpr:
		return exprString(x.Fun) + "(...)"
	case *ast.SelectorExpr:
		return exprString(x.X) + "." + x.Sel.Name
	case *ast.ParenExpr:
		return "(" + exprString(x.X) + ")"
	}
	return fmt.Sprintf("%T", x)
}

func (w *walker) walkFunc(fd *ast.FuncDecl, e env) {
	key := fd.Name.Name + fmt.Sprint(e)
	if w.seen[key] || fd.Body == nil {
		return
	}
	w.seen[key] = true
	w.walk(fd.Body, e)
}

// Methods that are infrastructure, not parameter sources.
var skipMethods = map[string]bool{"scAdd": true, "exprs": true, "stateSec": true, "charWarn": true}

func (w *walker) walk(n ast.Node, e env) {
	ast.Inspect(n, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.RangeStmt:
			// for _, s := range []string{"a", "b"} { ... }
			if lit, ok := n.X.(*ast.CompositeLit); ok {
				if v, ok := n.Value.(*ast.Ident); ok {
					var vals []string
					for _, el := range lit.Elts {
						if s, ok := stringLit(el); ok {
							vals = append(vals, s)
						}
					}
					if len(vals) == len(lit.Elts) && len(vals) > 0 {
						for _, s := range vals {
							w.walk(n.Body, e.with(v.Name, s))
						}
						return false
					}
				}
			}
		case *ast.AssignStmt:
			if n.Tok == token.DEFINE && len(n.Lhs) == 1 && len(n.Rhs) == 1 {
				if id, ok := n.Lhs[0].(*ast.Ident); ok {
					if v, ok := e.eval(n.Rhs[0]); ok {
						e[id.Name] = v
					}
				}
			}
		case *ast.IndexExpr:
			if id, ok := n.X.(*ast.Ident); ok && id.Name == "is" {
				if name, ok := e.eval(n.Index); ok {
					w.add(Param{Name: name})
				} else {
					w.unresolved(n.Index)
				}
			}
		case *ast.CallExpr:
			return w.call(n, e)
		}
		return true
	})
}

func (w *walker) call(n *ast.CallExpr, e env) bool {
	sel, ok := n.Fun.(*ast.SelectorExpr)
	if !ok {
		return true
	}
	recv, ok := sel.X.(*ast.Ident)
	if !ok {
		return true
	}
	m := sel.Sel.Name
	args := n.Args

	if recv.Name == "is" { // is.ReadI32("name", ...) and friends
		switch m {
		case "ReadI32", "ReadF32", "ReadBool", "getText", "LoadFile":
			if len(args) > 0 {
				if name, ok := e.eval(args[0]); ok {
					w.add(Param{Name: name, Type: strings.TrimPrefix(m, "Read")})
				} else {
					w.unresolved(args[0])
				}
			}
		}
		return true
	}
	if recv.Name != "c" {
		return true
	}
	switch m {
	case "paramValue": // (is, sc, name, id, vt, numArg, mandatory)
		if len(args) == 7 {
			if name, ok := e.eval(args[2]); ok {
				w.add(Param{Name: name, Type: vtName(args[4]), Count: intLit(args[5]), Mandatory: boolLit(args[6])})
			} else {
				w.unresolved(args[2])
			}
		}
		return true
	case "stateParam": // (is, name, mandatory, func(data string) error)
		if len(args) == 4 {
			if name, ok := e.eval(args[1]); ok {
				p := Param{Name: name, Mandatory: boolLit(args[2])}
				p.Type, p.Count = scAddType(args[3])
				w.add(p)
			} else {
				w.unresolved(args[1])
			}
			// The closure may read more parameters
			w.walk(args[3], e)
		}
		return false
	}
	if skipMethods[m] {
		return true
	}
	fd := w.src.methods[m]
	if fd == nil || !takesIniSection(fd) {
		return true
	}
	// Bind the callee's string parameters to constant arguments
	sub := env{}
	i := 0
	for _, field := range fd.Type.Params.List {
		isString := false
		if id, ok := field.Type.(*ast.Ident); ok && id.Name == "string" {
			isString = true
		}
		names := field.Names
		if len(names) == 0 {
			i++
			continue
		}
		for _, nm := range names {
			if isString && i < len(args) {
				if v, ok := e.eval(args[i]); ok {
					sub[nm.Name] = v
				}
			}
			i++
		}
	}
	w.walkFunc(fd, sub)
	return true
}

func takesIniSection(fd *ast.FuncDecl) bool {
	for _, f := range fd.Type.Params.List {
		if id, ok := f.Type.(*ast.Ident); ok && id.Name == "IniSection" {
			return true
		}
	}
	return false
}

// scAddType finds the first c.scAdd(sc, id, data, VT_x, n) inside a closure.
func scAddType(x ast.Expr) (vt string, n int) {
	ast.Inspect(x, func(node ast.Node) bool {
		if vt != "" {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "scAdd" && len(call.Args) >= 5 {
			vt, n = vtName(call.Args[3]), intLit(call.Args[4])
		}
		return true
	})
	return
}

func vtName(x ast.Expr) string {
	if id, ok := x.(*ast.Ident); ok && strings.HasPrefix(id.Name, "VT_") {
		return strings.TrimPrefix(id.Name, "VT_")
	}
	return ""
}

func intLit(x ast.Expr) int {
	if bl, ok := x.(*ast.BasicLit); ok && bl.Kind == token.INT {
		n, _ := strconv.Atoi(bl.Value)
		return n
	}
	return 0
}

func boolLit(x ast.Expr) bool {
	id, ok := x.(*ast.Ident)
	return ok && id.Name == "true"
}
