// Package diag is the diagnostics model shared by every katafighter-formats
// reader, the LSP and the editor: a problem with a severity, a message and a
// position (file:line:col).
//
// Positions are 1-based. Columns count bytes in the line, which is what the
// readers see; the LSP converts them to UTF-16 when it talks to a client.
// A zero Line means "the whole file", a zero Col means "the whole line".
package diag

import (
	"fmt"
	"strings"
)

// Severity orders like the LSP: Error is the most severe.
type Severity int

const (
	Error Severity = iota + 1
	Warning
	Info
	Hint
)

func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warning:
		return "warning"
	case Info:
		return "info"
	case Hint:
		return "hint"
	}
	return fmt.Sprintf("severity(%d)", int(s))
}

// Pos is a 1-based line and byte column.
type Pos struct {
	Line, Col int
}

func (p Pos) String() string {
	switch {
	case p.Line <= 0:
		return ""
	case p.Col <= 0:
		return fmt.Sprintf("%d", p.Line)
	}
	return fmt.Sprintf("%d:%d", p.Line, p.Col)
}

// Before reports whether p comes before q.
func (p Pos) Before(q Pos) bool {
	return p.Line < q.Line || p.Line == q.Line && p.Col < q.Col
}

// Range is a half-open span [Start, End). A zero End means "just Start".
type Range struct {
	Start, End Pos
}

// At returns a range for a whole line (col = 0) or a single position.
func At(line, col int) Range { return Range{Start: Pos{line, col}} }

// Diagnostic is one problem found in a file.
type Diagnostic struct {
	File     string // as given to the reader; may be empty for in-memory text
	Range    Range
	Severity Severity
	// Code is a short stable identifier ("duplicate-key", "unknown-trigger")
	// that tools can filter on. It may be empty.
	Code    string
	Message string
}

// String formats the diagnostic as "file:line:col: severity: message", leaving
// out the parts that are unknown.
func (d Diagnostic) String() string {
	var b strings.Builder
	loc := d.File
	if p := d.Range.Start.String(); p != "" {
		if loc != "" {
			loc += ":"
		}
		loc += p
	}
	if loc != "" {
		b.WriteString(loc)
		b.WriteString(": ")
	}
	b.WriteString(d.Severity.String())
	b.WriteString(": ")
	b.WriteString(d.Message)
	if d.Code != "" {
		b.WriteString(" [")
		b.WriteString(d.Code)
		b.WriteString("]")
	}
	return b.String()
}

// List collects diagnostics for one file.
type List struct {
	File  string
	Items []Diagnostic
}

// Add appends a diagnostic, filling in the list's file name if it has none.
func (l *List) Add(d Diagnostic) {
	if d.File == "" {
		d.File = l.File
	}
	l.Items = append(l.Items, d)
}

// Errorf adds an error at r.
func (l *List) Errorf(r Range, code, format string, a ...any) {
	l.Add(Diagnostic{Range: r, Severity: Error, Code: code, Message: fmt.Sprintf(format, a...)})
}

// Warnf adds a warning at r.
func (l *List) Warnf(r Range, code, format string, a ...any) {
	l.Add(Diagnostic{Range: r, Severity: Warning, Code: code, Message: fmt.Sprintf(format, a...)})
}

// HasErrors reports whether any diagnostic is an error.
func HasErrors(ds []Diagnostic) bool {
	for _, d := range ds {
		if d.Severity == Error {
			return true
		}
	}
	return false
}

// SetFile sets File on every diagnostic that has none, e.g. after parsing
// in-memory text that turned out to come from a file.
func SetFile(ds []Diagnostic, file string) {
	for i := range ds {
		if ds[i].File == "" {
			ds[i].File = file
		}
	}
}
