// Package expr parses CNS/ZSS trigger and parameter expressions into a
// syntax tree with byte offsets.
//
// Upstream: ikemen/source/src/compiler.go @ v1.0.0 (81c6da71d689625e20db79586815b695da00dd6d)
//
//	specialSymbols (compiler.go:11), tokenizerCS (compiler.go:490) -> Lex
//	isOperator (compiler.go:616)                                   -> precedence
//	expBoolOr ... expPow, expRange, expValue (compiler.go:1272-5776) -> Parser
//
// Changes from upstream: Ikemen compiles straight to bytecode and never
// builds a tree. This package keeps the tokenizer and the precedence chain
// but builds nodes, and it does not know trigger names: trigger-specific
// shapes ("vel x", "animelem = 2, >= 3", "hitdefattr = SCA, NA") are parsed
// by generic rules described on each node type. Checking names and argument
// counts against lsp/schema is left to the caller.
//
// MIT License
// Copyright (c) 2016 Suehiro
// Copyright (c) 2018 K4thos, NeatUnsou, ShinLucho
// Copyright (c) 2019 Daniel Porto, Gacel, Nikolay Griva
// See ikemen/source/LICENCE.txt for the full notice.
package expr

import "strings"

// specialSymbols end an identifier token (compiler.go:11).
const specialSymbols = " !=<>()|&+-*/%,[]^:;{}#\"\t\r\n"

// Kind classifies a token.
type Kind int

const (
	TokEOF Kind = iota
	TokNumber
	TokIdent
	TokString // the text between quotes; Pos is the opening quote
	TokPunct  // operators and punctuation
	TokBad    // an unterminated string
)

// Token is one lexical token. Pos and End are byte offsets into the source.
type Token struct {
	Kind     Kind
	Text     string // as written (case kept)
	Pos, End int
}

// Lower returns the token text in lower case, as Ikemen compares it.
func (t Token) Lower() string { return strings.ToLower(t.Text) }

var twoChar = map[string]bool{
	":=": true, "!=": true, ">=": true, "<=": true,
	"&&": true, "^^": true, "||": true, "**": true,
}

const oneChar = "=:;!><~&^|+-*/%,()[]{}"

// Lex splits src into tokens like tokenizerCS. A '#' outside a string ends
// the line (ZSS comment) when hashComments is true.
func Lex(src string, hashComments bool) []Token {
	var toks []Token
	i := 0
	for {
		for i < len(src) && (src[i] == ' ' || src[i] == '\t' || src[i] == '\r' || src[i] == '\n' || src[i] == '\v' || src[i] == '\f') {
			i++
		}
		if i >= len(src) {
			toks = append(toks, Token{Kind: TokEOF, Pos: len(src), End: len(src)})
			return toks
		}
		c := src[i]
		switch {
		case c == '#' && hashComments:
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		case c == '"':
			j := strings.IndexByte(src[i+1:], '"')
			if j < 0 {
				toks = append(toks, Token{Kind: TokBad, Text: src[i+1:], Pos: i, End: len(src)})
				i = len(src)
				continue
			}
			toks = append(toks, Token{Kind: TokString, Text: src[i+1 : i+1+j], Pos: i, End: i + j + 2})
			i += j + 2
			continue
		case i+1 < len(src) && twoChar[src[i:i+2]]:
			toks = append(toks, Token{Kind: TokPunct, Text: src[i : i+2], Pos: i, End: i + 2})
			i += 2
			continue
		case strings.IndexByte(oneChar, c) >= 0:
			toks = append(toks, Token{Kind: TokPunct, Text: src[i : i+1], Pos: i, End: i + 1})
			i++
			continue
		}
		// Number: digits with at most one '.', then an optional exponent.
		j, dot := i, false
		for ; j < len(src); j++ {
			if src[j] == '.' {
				if dot {
					break
				}
				dot = true
			} else if src[j] < '0' || src[j] > '9' {
				break
			}
		}
		if j > i && j < len(src) && (src[j] == 'e' || src[j] == 'E') {
			k := j + 1
			for j++; j < len(src); j++ {
				if (src[j] < '0' || src[j] > '9') && (j != k || (src[j] != '-' && src[j] != '+')) {
					break
				}
			}
		}
		kind := TokNumber
		if j == i {
			kind = TokIdent
			j = strings.IndexAny(src[i:], specialSymbols)
			if j < 0 {
				j = len(src)
			} else {
				j += i
			}
			if j == i { // a lone special symbol we do not know: skip it
				toks = append(toks, Token{Kind: TokPunct, Text: src[i : i+1], Pos: i, End: i + 1})
				i++
				continue
			}
		}
		// "12abc" lexes as Number "12" then Ident "abc", as in Ikemen.
		toks = append(toks, Token{Kind: kind, Text: src[i:j], Pos: i, End: j})
		i = j
	}
}
