// INI-style section helpers, as Ikemen GO reads DEF files and the constant
// sections of CNS files.
//
// Upstream: ikemen/source/src/common.go @ v1.0.0 (81c6da71d689625e20db79586815b695da00dd6d)
//
//	ReadIniSection   (common.go:947)  -> ReadSection
//	IniSection.Parse (common.go:964)  -> Section.parse
//	ReadI32 / ReadF32 / ReadBool (common.go:995-1047)
//	getText          (common.go:1131) -> Section.Text
//	decodeShiftJIS   (common.go:433)  -> DecodeShiftJIS
//	compareNatural   (common.go:1566) -> NaturalLess
//
// Changes from upstream: a Section keeps every entry in file order with its
// 1-based line and column, including the duplicates that Ikemen ignores, so
// that callers can report them. Lookups still return the first value.
//
// MIT License
// Copyright (c) 2016 Suehiro
// Copyright (c) 2018 K4thos, NeatUnsou, ShinLucho
// Copyright (c) 2019 Daniel Porto, Gacel, Nikolay Griva
// See ikemen/source/LICENCE.txt for the full notice.
package text

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// Entry is one "key = value" line.
type Entry struct {
	Key   string // lower-cased
	Value string // trimmed, comment removed; "" when the line has no '='
	Line  int    // 1-based
	// Col is the 1-based byte column of the key in the original line.
	Col int
	// Dup is true when an earlier entry has the same key. Ikemen ignores it.
	Dup bool
}

// Section is one [section] with its entries.
type Section struct {
	Name string // lower-cased, keeps a trailing space when Sub follows (see SectionName)
	Sub  string // rest of the header, as written
	Line int    // 1-based line of the header
	// Entries in file order, duplicates included.
	Entries []Entry
	first   map[string]int
}

// Get returns the first value for key (lower-case), like IniSection[key].
func (s *Section) Get(key string) (string, bool) {
	i, ok := s.first[key]
	if !ok {
		return "", false
	}
	return s.Entries[i].Value, true
}

// Value returns the first value for key, or "".
func (s *Section) Value(key string) string {
	v, _ := s.Get(key)
	return v
}

// Entry returns the first entry for key, or nil.
func (s *Section) Entry(key string) *Entry {
	i, ok := s.first[key]
	if !ok {
		return nil
	}
	return &s.Entries[i]
}

// Keys returns the distinct keys in file order.
func (s *Section) Keys() []string {
	var ks []string
	for _, e := range s.Entries {
		if !e.Dup {
			ks = append(ks, e.Key)
		}
	}
	return ks
}

// ReadSection skips to the next section header at or after lines[*i] and
// reads it. lines are the file's lines (see SplitAndTrim); *i is a 0-based
// index and is left on the line after the section. It returns nil when no
// section is left. raw holds the untrimmed lines, used for columns; it may
// be nil.
func ReadSection(lines, raw []string, i *int) *Section {
	var name, sub string
	for ; *i < len(lines); (*i)++ {
		name, sub = SectionName(lines[*i])
		if len(name) > 0 {
			(*i)++
			break
		}
	}
	if len(name) == 0 {
		return nil
	}
	s := &Section{Name: name, Sub: sub, Line: *i, first: map[string]int{}}
	s.parse(lines, raw, i)
	return s
}

func (s *Section) parse(lines, raw []string, i *int) {
	for ; *i < len(lines); (*i)++ {
		if len(lines[*i]) > 0 && lines[*i][0] == '[' {
			break
		}
		line := strings.TrimSpace(strings.SplitN(lines[*i], ";", 2)[0])
		ia := strings.IndexAny(line, "= \t")
		if ia <= 0 {
			continue
		}
		e := Entry{Key: strings.ToLower(line[:ia]), Line: *i + 1, Col: 1}
		if *i < len(raw) {
			e.Col = len(raw[*i]) - len(strings.TrimLeft(raw[*i], " \t\r")) + 1
		}
		if ia = strings.Index(line, "="); ia >= 0 {
			e.Value = strings.TrimSpace(line[ia+1:])
		}
		if _, ok := s.first[e.Key]; ok {
			e.Dup = true
		} else {
			s.first[e.Key] = len(s.Entries)
		}
		s.Entries = append(s.Entries, e)
	}
}

// ReadI32 fills out from a comma list. Empty items leave their output
// unchanged. It reports false when the key is missing or empty.
func (s *Section) ReadI32(key string, out ...*int32) bool {
	str := s.Value(key)
	if len(str) == 0 {
		return false
	}
	for i, v := range strings.Split(str, ",") {
		if i >= len(out) {
			break
		}
		if v = strings.TrimSpace(v); len(v) > 0 {
			*out[i], _ = Atoi(v)
		}
	}
	return true
}

// ReadF32 is ReadI32 for floats.
func (s *Section) ReadF32(key string, out ...*float32) bool {
	str := s.Value(key)
	if len(str) == 0 {
		return false
	}
	for i, v := range strings.Split(str, ",") {
		if i >= len(out) {
			break
		}
		if v = strings.TrimSpace(v); len(v) > 0 {
			*out[i] = float32(Atof(v))
		}
	}
	return true
}

// ReadBool is ReadI32 for booleans (non-zero = true).
func (s *Section) ReadBool(key string, out ...*bool) bool {
	str := s.Value(key)
	if len(str) == 0 {
		return false
	}
	for i, v := range strings.Split(str, ",") {
		if i >= len(out) {
			break
		}
		if v = strings.TrimSpace(v); len(v) > 0 {
			n, _ := Atoi(v)
			*out[i] = n != 0
		}
	}
	return true
}

// ErrNotQuoted is returned by Text for a value without surrounding quotes.
var ErrNotQuoted = errors.New(`string not enclosed in "`)

// Text returns a quoted value without its quotes. Like Ikemen, an unquoted
// value is still returned as written, together with ErrNotQuoted.
func (s *Section) Text(key string) (str string, ok bool, err error) {
	str, ok = s.Get(key)
	if !ok {
		return
	}
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	} else {
		err = ErrNotQuoted
	}
	return
}

// DecodeShiftJIS returns input unchanged when it is valid UTF-8, and decodes
// it as Shift_JIS otherwise (old Japanese characters).
func DecodeShiftJIS(input string) string {
	if utf8.ValidString(input) {
		return input
	}
	b, _, err := transform.Bytes(japanese.ShiftJIS.NewDecoder(), []byte(input))
	if err != nil {
		return input
	}
	return string(b)
}

var naturalKey = regexp.MustCompile(`^([a-zA-Z_]+)(\d*)$`)

// NaturalLess orders "st" < "st1" < "st2" < "st10": the letter prefix
// lexically, then the number. Keys of another shape compare lexically.
func NaturalLess(a, b string) bool {
	am, bm := naturalKey.FindStringSubmatch(a), naturalKey.FindStringSubmatch(b)
	if am == nil || bm == nil {
		return a < b
	}
	if am[1] != bm[1] {
		return am[1] < bm[1]
	}
	an, _ := Atoi(am[2])
	bn, _ := Atoi(bm[2])
	return an < bn
}
