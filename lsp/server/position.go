package server

import (
	"strings"
	"unicode/utf8"

	"github.com/whoisnighter/KataFighter/formats/diag"
)

// Position and Range are the LSP shapes: 0-based line, UTF-16 character.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// lines splits text the way the readers count lines (on '\n').
func lines(text string) []string { return strings.Split(text, "\n") }

// utf16Len returns the length of s in UTF-16 code units.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// toPosition converts a 1-based line and 1-based byte column to an LSP
// position. Columns past the end of the line clamp to it.
func toPosition(ls []string, line, col int) Position {
	if line < 1 || line > len(ls) {
		return Position{Line: max(line-1, 0)}
	}
	s := strings.TrimSuffix(ls[line-1], "\r")
	b := min(max(col-1, 0), len(s))
	for b > 0 && b < len(s) && !utf8.RuneStart(s[b]) {
		b--
	}
	return Position{Line: line - 1, Character: utf16Len(s[:b])}
}

// toRange converts a diagnostic range to an LSP range. A whole-line range
// (Col 0) covers the line without its indentation; a single position (no
// End) covers the word that starts there.
func toRange(ls []string, r diag.Range) Range {
	if r.Start.Line <= 0 {
		return Range{}
	}
	if r.Start.Line > len(ls) {
		p := Position{Line: len(ls) - 1}
		return Range{p, p}
	}
	s := strings.TrimSuffix(ls[r.Start.Line-1], "\r")
	if r.Start.Col <= 0 {
		start := len(s) - len(strings.TrimLeft(s, " \t"))
		end := len(strings.TrimRight(s, " \t"))
		return Range{toPosition(ls, r.Start.Line, start+1), toPosition(ls, r.Start.Line, end+1)}
	}
	start := toPosition(ls, r.Start.Line, r.Start.Col)
	if r.End.Line > 0 {
		return Range{start, toPosition(ls, r.End.Line, r.End.Col)}
	}
	b := min(r.Start.Col-1, len(s))
	e := b
	for e < len(s) && s[e] != ' ' && s[e] != '\t' {
		e++
	}
	if e == b && e < len(s) {
		e++
	}
	return Range{start, toPosition(ls, r.Start.Line, e+1)}
}
