// Package cns parses the INI-style MUGEN text formats used by characters:
// CNS (constants and states), ST (states) and CMD (commands plus the -1
// state). It builds a syntax tree with positions and reports diagnostics; it
// does not compile anything.
//
// Upstream: ikemen/source/src/compiler.go @ v1.0.0 (81c6da71d689625e20db79586815b695da00dd6d)
//
//	stateCompileCNS (compiler.go:6786) -> Parse (statedef / state grouping)
//	parseSection    (compiler.go:5847) -> parseStateEntries
//	scanStateDef    (compiler.go:7332) -> parseStatedefNumber
//	parseTriggerNumber (compiler.go:5832)
//	ReadIniSection  (common.go:947)    -> other sections (via internal/text rules)
//
// Changes from upstream: nothing is compiled. Every error Ikemen reports
// becomes a diagnostic and parsing continues, so one file shows all its
// problems. Trigger values are parsed with package expr. Controller types
// and trigger names are not checked here (see lsp/schema).
//
// MIT License
// Copyright (c) 2016 Suehiro
// Copyright (c) 2018 K4thos, NeatUnsou, ShinLucho
// Copyright (c) 2019 Daniel Porto, Gacel, Nikolay Griva
// See ikemen/source/LICENCE.txt for the full notice.
package cns

import (
	"os"
	"sort"
	"strings"

	"github.com/whoisnighter/KataFighter/formats/diag"
	"github.com/whoisnighter/KataFighter/formats/expr"
	"github.com/whoisnighter/KataFighter/formats/internal/text"
)

// Kind classifies a section.
type Kind int

const (
	KindOther    Kind = iota // [Data], [Size], [Remap], [Defaults], ...
	KindStatedef             // [Statedef N]
	KindState                // [State N, label]
	KindCommand              // [Command] (CMD)
)

// Entry is one "key = value" line.
type Entry struct {
	Key    string // lower-cased: "trigger1", "value", "var(3)"
	KeyRaw string // as written
	Value  string // trimmed, comment removed
	Line   int    // 1-based
	// KeyCol and ValueCol are 1-based byte columns in the line.
	KeyCol, ValueCol int
	// Expr is the parsed value of a trigger (trigger1, triggerall, ...).
	// It is nil for other keys; use ParseValue for those.
	Expr expr.Node
	// Dup is true when an earlier entry in the section has the same key
	// (triggers excepted). Ikemen ignores it.
	Dup bool
}

// Pos converts a byte offset inside Value to a file position.
func (e *Entry) Pos(off int) diag.Pos {
	return diag.Pos{Line: e.Line, Col: e.ValueCol + off}
}

// Range converts a byte span inside Value to a file range.
func (e *Entry) Range(pos, end int) diag.Range {
	return diag.Range{Start: e.Pos(pos), End: e.Pos(end)}
}

// ParseValue parses the value as a comma-separated list of expressions,
// the shape of most controller parameters. Values that are not expressions
// (hitflag = MAF, attr = S, NA) may parse with errors; callers that know the
// parameter type decide whether that matters.
func (e *Entry) ParseValue() ([]expr.Node, []expr.Error) {
	return expr.ParseList(e.Value, expr.Options{})
}

// IsTrigger reports whether the key is triggerall or triggerN (1..65536).
func (e *Entry) IsTrigger() bool {
	_, _, ok := TriggerNumber(e.Key)
	return ok
}

// Section is one [header] and its entries.
type Section struct {
	Kind Kind
	Name string // lower-cased first word of the header, with its trailing space ("statedef ")
	Sub  string // rest of the header as written ("200, Stand Light Punch")
	// Line is the 1-based header line; EndLine is the last line before the
	// next section (or the end of the file).
	Line, EndLine int
	Entries       []*Entry
}

// Get returns the first entry for key (lower-case), or nil.
func (s *Section) Get(key string) *Entry {
	for _, e := range s.Entries {
		if e.Key == key {
			return e
		}
	}
	return nil
}

// Value returns the first value for key, or "".
func (s *Section) Value(key string) string {
	if e := s.Get(key); e != nil {
		return e.Value
	}
	return ""
}

// Statedef is a [Statedef N] section with the controllers that follow it.
type Statedef struct {
	*Section
	// NumberText is the state number as written ("200", "-1", "+1",
	// "const(StateNo)").
	NumberText string
	// Number is the state number, -10 for "+1" (Ikemen's encoding), or 0
	// when it comes from a constant (see Const).
	Number int32
	Const  string // the constant name of "const(name)", lower-cased
	// Controllers in file order.
	Controllers []*Controller
}

// Controller is a [State N, label] section.
type Controller struct {
	*Section
	// Label is what follows the comma in the header (may be empty).
	Label string
	// Type is the lower-cased controller type ("changestate"), "" if missing.
	Type string
	// TriggerAll holds the triggerall entries.
	TriggerAll []*Entry
	// Triggers[i] holds the entries of trigger(i+1).
	Triggers [][]*Entry
	// Params holds the other entries (type, persistent and ignorehitpause
	// included), in file order.
	Params []*Entry
}

// File is a parsed CNS, ST or CMD file.
type File struct {
	Path     string
	Sections []*Section
	// States are the [Statedef] blocks in file order (duplicates included;
	// Ikemen keeps the first).
	States []*Statedef
	// Commands are the [Command] sections (CMD files).
	Commands    []*Section
	Diagnostics []diag.Diagnostic
}

// Section returns the first section with the given lower-case name
// ("data", "size", "remap"), or nil.
func (f *File) Section(name string) *Section {
	for _, s := range f.Sections {
		if strings.TrimSpace(s.Name) == name {
			return s
		}
	}
	return nil
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

// TriggerNumber parses "triggerall" (all = true) or "triggerN" with N in
// 1..65536, like parseTriggerNumber.
func TriggerNumber(key string) (n int32, all, ok bool) {
	if !strings.HasPrefix(key, "trigger") {
		return 0, false, false
	}
	suffix := key[7:]
	if suffix == "all" {
		return 0, true, true
	}
	if suffix == "" {
		return 0, false, false
	}
	var v int64
	for i := 0; i < len(suffix); i++ {
		if suffix[i] < '0' || suffix[i] > '9' {
			return 0, false, false
		}
		v = v*10 + int64(suffix[i]-'0')
		if v > 65536 {
			return 0, false, false
		}
	}
	if v < 1 {
		return 0, false, false
	}
	return int32(v), false, true
}

type parser struct {
	f     *File
	dl    diag.List
	lines []string // trimmed
	raw   []string
}

// Parse parses CNS / ST / CMD text.
func Parse(src string) *File {
	p := &parser{f: &File{}, raw: strings.Split(src, "\n")}
	p.lines = text.SplitAndTrim(src, "\n")
	p.parse()
	sort.SliceStable(p.dl.Items, func(a, b int) bool {
		return p.dl.Items[a].Range.Start.Before(p.dl.Items[b].Range.Start)
	})
	p.f.Diagnostics = p.dl.Items
	return p.f
}

func stripComment(s string) string {
	return strings.TrimSpace(strings.SplitN(s, ";", 2)[0])
}

func (p *parser) col(line int, s string) int {
	// 1-based byte column of s inside raw line (line is 0-based).
	if line < len(p.raw) {
		if i := strings.Index(p.raw[line], s); i >= 0 {
			return i + 1
		}
	}
	return 1
}

func (p *parser) parse() {
	var cur *Statedef
	seen := map[string]int{} // statedef number text -> header line
	stray := false           // inside a run of text outside any section
	for i := 0; i < len(p.lines); {
		name, sub := text.SectionName(p.lines[i])
		if name == "" {
			l := stripComment(p.lines[i])
			switch {
			case strings.HasPrefix(l, "["):
				p.dl.Warnf(diag.At(i+1, p.col(i, "[")), "header-not-closed", "section header not closed with ']'")
				stray = false
			case l != "" && !stray:
				// Ikemen ignores it silently; report each run once.
				p.dl.Warnf(diag.At(i+1, 0), "outside-section", "text outside any [section] is ignored")
				stray = true
			}
			i++
			continue
		}
		stray = false
		s := &Section{Name: name, Sub: strings.TrimSpace(sub), Line: i + 1}
		i++
		start := i
		for i < len(p.lines) && !(len(p.lines[i]) > 0 && p.lines[i][0] == '[') {
			i++
		}
		s.EndLine = i
		p.f.Sections = append(p.f.Sections, s)

		switch strings.TrimSpace(name) {
		case "statedef":
			s.Kind = KindStatedef
			p.parseStateEntries(s, start, i)
			sd := &Statedef{Section: s}
			p.parseStatedefNumber(sd)
			if prev, ok := seen[strings.ToLower(sd.NumberText)]; ok {
				p.dl.Warnf(diag.At(s.Line, 0), "duplicate-statedef", "state %s is already defined on line %d (ignored by Ikemen)", sd.NumberText, prev)
			} else {
				seen[strings.ToLower(sd.NumberText)] = s.Line
			}
			p.f.States = append(p.f.States, sd)
			cur = sd
		case "state":
			s.Kind = KindState
			p.parseStateEntries(s, start, i)
			c := p.controller(s)
			if cur == nil {
				p.dl.Warnf(diag.At(s.Line, 0), "orphan-state", "[State] outside of a [Statedef] (ignored)")
			} else {
				cur.Controllers = append(cur.Controllers, c)
			}
		case "command":
			s.Kind = KindCommand
			p.parseIniEntries(s, start, i)
			p.f.Commands = append(p.f.Commands, s)
			cur = nil
		default:
			p.parseIniEntries(s, start, i)
			cur = nil
		}
	}
}

// parseIniEntries reads a plain section with the IniSection rules: the key
// ends at '=' or white space, and the first key wins.
func (p *parser) parseIniEntries(s *Section, from, to int) {
	seen := map[string]bool{}
	for i := from; i < to; i++ {
		line := stripComment(p.lines[i])
		ia := strings.IndexAny(line, "= \t")
		if ia <= 0 {
			continue
		}
		e := &Entry{KeyRaw: line[:ia], Key: strings.ToLower(line[:ia]), Line: i + 1}
		e.KeyCol = p.col(i, e.KeyRaw)
		if j := strings.Index(line, "="); j >= 0 {
			e.Value = strings.TrimSpace(line[j+1:])
			e.ValueCol = e.KeyCol + j + 1 + (len(line[j+1:]) - len(strings.TrimLeft(line[j+1:], " \t")))
		}
		if seen[e.Key] {
			e.Dup = true
			p.dl.Warnf(diag.At(e.Line, e.KeyCol), "duplicate-key", "duplicate key %q (ignored, the first one wins)", e.Key)
		}
		seen[e.Key] = true
		s.Entries = append(s.Entries, e)
	}
}

// topLevelEqual finds the first '=' outside parentheses.
func topLevelEqual(s string) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case '=':
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// specialParamName accepts "var(3)", "fvar (2)", "map(name)" as keys.
func specialParamName(lhs string) (string, bool) {
	i := strings.Index(lhs, "(")
	if i <= 0 {
		return "", false
	}
	switch strings.ToLower(strings.TrimSpace(lhs[:i])) {
	case "var", "fvar", "sysvar", "sysfvar", "map":
	default:
		return "", false
	}
	depth, end := 0, -1
	for j := i; j < len(lhs) && end < 0; j++ {
		switch lhs[j] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				end = j
			}
		}
	}
	if end < 0 {
		return "", false
	}
	return strings.ToLower(strings.TrimSpace(lhs[:end+1])), true
}

// parseStateEntries mirrors parseSection with ignoreMostErrors = true.
func (p *parser) parseStateEntries(s *Section, from, to int) {
	seen := map[string]bool{}
	for i := from; i < to; i++ {
		line := stripComment(p.lines[i])
		ia := topLevelEqual(line)
		if ia <= 0 {
			if line != "" {
				p.dl.Warnf(diag.At(i+1, p.col(i, line)), "invalid-line", "line ignored: no '='")
			}
			continue
		}
		lhs := strings.TrimSpace(line[:ia])
		e := &Entry{KeyRaw: lhs, Line: i + 1}
		e.KeyCol = p.col(i, lhs)
		rest := line[ia+1:]
		e.Value = strings.TrimSpace(rest)
		e.ValueCol = e.KeyCol + ia + 1 + (len(rest) - len(strings.TrimLeft(rest, " \t")))
		if name, ok := specialParamName(lhs); ok {
			e.Key = name
		} else if strings.Contains(lhs, "(") {
			p.dl.Warnf(diag.At(e.Line, e.KeyCol), "invalid-param", "invalid parameter syntax %q (ignored)", lhs)
			continue
		} else {
			if j := strings.IndexAny(lhs, " \t"); j >= 0 {
				p.dl.Warnf(diag.At(e.Line, e.KeyCol+j), "param-space", "parameter name %q contains a space; Ikemen reads it as %q", lhs, lhs[:j])
				lhs = lhs[:j]
			}
			e.Key = strings.ToLower(lhs)
		}
		if strings.HasPrefix(e.Key, "trigger") {
			if _, _, ok := TriggerNumber(e.Key); !ok {
				p.dl.Warnf(diag.At(e.Line, e.KeyCol), "invalid-trigger-name", "invalid trigger name %q (ignored)", e.KeyRaw)
				continue
			}
			if e.Value == "" {
				p.dl.Errorf(diag.At(e.Line, e.KeyCol), "empty-trigger", "%s cannot be empty", e.Key)
			} else {
				var errs []expr.Error
				e.Expr, errs = expr.Parse(e.Value, expr.Options{})
				for _, er := range errs {
					p.dl.Add(diag.Diagnostic{Range: e.Range(er.Pos, er.End), Severity: diag.Error, Code: "syntax", Message: er.Msg})
				}
			}
		} else {
			if seen[e.Key] {
				e.Dup = true
				p.dl.Warnf(diag.At(e.Line, e.KeyCol), "duplicate-key", "duplicate parameter %q (ignored, the first one wins)", e.Key)
			}
			seen[e.Key] = true
		}
		s.Entries = append(s.Entries, e)
	}
}

// parseStatedefNumber mirrors scanStateDef.
func (p *parser) parseStatedefNumber(sd *Statedef) {
	t := strings.TrimSpace(sd.Sub)
	sd.NumberText = t
	low := strings.ToLower(strings.ReplaceAll(t, " ", ""))
	switch {
	case t == "":
		p.dl.Errorf(diag.At(sd.Line, 0), "statedef-number", "missing state number")
	case strings.HasPrefix(low, "const(") && strings.HasSuffix(low, ")"):
		sd.Const = low[len("const(") : len(low)-1]
	case low == "+1":
		sd.Number = -10
	default:
		n, _ := text.Atoi(t)
		sd.Number = n
		if !text.IsNumeric(strings.Fields(t)[0]) {
			p.dl.Warnf(diag.At(sd.Line, 0), "statedef-number", "state number %q is not a number; Ikemen reads %d", t, n)
		}
	}
}

func (p *parser) controller(s *Section) *Controller {
	c := &Controller{Section: s}
	if i := strings.Index(s.Sub, ","); i >= 0 {
		c.Label = strings.TrimSpace(s.Sub[i+1:])
	}
	missingWarned := false
	for _, e := range s.Entries {
		n, all, ok := TriggerNumber(e.Key)
		switch {
		case ok && all:
			c.TriggerAll = append(c.TriggerAll, e)
		case ok:
			for int(n) > len(c.Triggers) {
				c.Triggers = append(c.Triggers, nil)
			}
			if !missingWarned {
				for k := 0; k < int(n)-1; k++ {
					if len(c.Triggers[k]) == 0 {
						p.dl.Warnf(diag.At(e.Line, e.KeyCol), "missing-trigger", "trigger%d declared without trigger%d", n, k+1)
						missingWarned = true
						break
					}
				}
			}
			c.Triggers[n-1] = append(c.Triggers[n-1], e)
		default:
			c.Params = append(c.Params, e)
			if e.Key == "type" && c.Type == "" && !e.Dup {
				c.Type = strings.ToLower(e.Value)
			}
		}
	}
	if c.Type == "" {
		p.dl.Errorf(diag.At(s.Line, 0), "missing-type", "state controller type not specified")
	}
	if len(c.Triggers) == 0 || len(c.Triggers[0]) == 0 {
		p.dl.Errorf(diag.At(s.Line, 0), "missing-trigger1", "missing trigger1")
	}
	return c
}
