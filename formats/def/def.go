// Package def reads character DEF files the way Ikemen GO does.
//
// Upstream: ikemen/source @ v1.0.0 (81c6da71d689625e20db79586815b695da00dd6d)
//
//	Char.load, DEF part      (char.go:3747-3979)  -> Parse
//	loadMovelists            (char.go:3076)       -> Files.Movelists
//	Compiler.Compile, [Info] and [Files] part (compiler.go:8139-8172)
//	                                              -> Info versions, Files.CMD/ST/StCommon
//	SearchFile / FileExist   (common.go:539, :445) -> Find
//
// Changes from upstream:
//   - The result is a plain struct instead of writes into engine state.
//     Paths are returned as written (Shift_JIS decoded); Find resolves them.
//   - Ikemen reads the DEF twice (Char.load and Compile) with slightly
//     different rules; Parse does it once. Compile's [Info] / [Files] ignore
//     language sections, so MugenVersion, IkemenVersion, cmd, st* and
//     stcommon come from the first plain section only, as in Compile.
//   - Problems Ikemen silently ignores (unquoted names, duplicate keys and
//     sections, out-of-range keymap slots) become diagnostics.
//   - Zip paths are not supported by Find.
//
// MIT License
// Copyright (c) 2016 Suehiro
// Copyright (c) 2018 K4thos, NeatUnsou, ShinLucho
// Copyright (c) 2019 Daniel Porto, Gacel, Nikolay Griva
// See ikemen/source/LICENCE.txt for the full notice.
package def

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/whoisnighter/KataFighter/formats/diag"
	"github.com/whoisnighter/KataFighter/formats/internal/text"
)

// PaletteMax is Ikemen's default [Config] PaletteMax.
const PaletteMax = 100

// KeymapButtons are the [Palette Keymap] keys in slot order.
var KeymapButtons = [12]string{"a", "b", "c", "x", "y", "z", "a2", "b2", "c2", "x2", "y2", "z2"}

// Info is the [Info] section.
type Info struct {
	Name        string
	DisplayName string // defaults to Name
	LifebarName string // defaults to DisplayName
	Author      string
	// LocalCoord defaults to 320,240.
	LocalCoord [2]int32
	// PortraitScale defaults to 1, or 320/LocalCoord[0] when localcoord is set.
	PortraitScale float32
	MugenVersion  string // raw value, e.g. "1.1"
	IkemenVersion string // raw value, e.g. "1.0"
}

// Font is a fontN / fontN.height pair from [Files].
type Font struct {
	Path   string
	Height int32
}

// Files is the [Files] section. Paths are as written in the DEF.
type Files struct {
	CNS, Sprite, Anim, Sound string
	CMD, StCommon            string
	// ST holds st, st0, st1 ... stN in natural order.
	ST []string
	// Pal maps a 1-based slot (pal1..pal100) to an ACT file.
	Pal map[int]string
	// Fonts maps N of fontN to the font.
	Fonts map[int]Font
	// Movelists maps N of movelistN (movelist = 0) to a file.
	Movelists map[int]string
}

// Def is a parsed DEF file.
type Def struct {
	Path  string // file the DEF was loaded from; empty for Parse
	Info  Info
	Files Files
	// Keymap holds the 1-based palette slot for each of KeymapButtons, or 0
	// when the button is not set (Ikemen then uses its default).
	Keymap [12]int32
	// Map is the [Map] section (character map defaults).
	Map map[string]float32
	// Shaders maps an alias to a shader file ([Shaders]).
	Shaders map[string]string
	// Sections holds every section in file order, including the ones Ikemen
	// ignores, for tools that need positions.
	Sections    []*text.Section
	Diagnostics []diag.Diagnostic
}

// Options control parsing.
type Options struct {
	// Language is Ikemen's [Config] Language ("en" by default). A section
	// named "<language>.info" etc. overrides the plain one.
	Language string
}

// Load reads and parses a DEF file.
func Load(path string, opt Options) (*Def, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	d := Parse(string(text.StripBOM(b)), opt)
	d.Path = path
	diag.SetFile(d.Diagnostics, path)
	return d, nil
}

var stKey = regexp.MustCompile(`^st[0-9]*$`)

// Parse parses DEF text.
func Parse(src string, opt Options) *Def {
	if opt.Language == "" {
		opt.Language = "en"
	}
	langPrefix := strings.ToLower(opt.Language) + "."
	d := &Def{
		Info:  Info{LocalCoord: [2]int32{320, 240}, PortraitScale: 1},
		Files: Files{Pal: map[int]string{}, Fonts: map[int]Font{}, Movelists: map[int]string{}},
		Map:   map[string]float32{},
	}
	var dl diag.List
	raw := strings.Split(src, "\n")
	lines := text.SplitAndTrim(src, "\n")

	// done[base][isLan] records that a section has been applied. Like
	// Ikemen, a localized section applies once and also blocks the plain
	// one; a plain one applies once and does not block the localized one.
	done := map[string]bool{}
	compileInfo, compileFiles := true, true
	// stray reports text before the first section or between sections that
	// is not a section header: Ikemen ignores it silently.
	stray := func(from, to int) {
		for k := from; k < to && k < len(lines); k++ {
			if l := strings.TrimSpace(strings.SplitN(lines[k], ";", 2)[0]); l != "" {
				if strings.HasPrefix(l, "[") {
					dl.Warnf(diag.At(k+1, 1), "header-not-closed", "section header not closed with ']' (ignored)")
				} else {
					dl.Warnf(diag.At(k+1, 0), "outside-section", "text outside any [section] is ignored")
				}
				return
			}
		}
	}
	for i := 0; i < len(lines); {
		start := i
		s := text.ReadSection(lines, raw, &i)
		if s == nil {
			stray(start, len(lines))
			break
		}
		stray(start, s.Line-1)
		d.Sections = append(d.Sections, s)
		for _, e := range s.Entries {
			if e.Dup {
				dl.Warnf(diag.At(e.Line, e.Col), "duplicate-key", "duplicate key %q in [%s%s] (ignored, the first one wins)", e.Key, s.Name, s.Sub)
			}
		}

		isLan := strings.HasPrefix(s.Name, langPrefix)
		base := s.Name
		if isLan {
			base = s.Name[len(langPrefix):]
		}
		if base == "palette " && !(len(s.Sub) >= 6 && strings.EqualFold(s.Sub[:6], "keymap")) {
			continue
		}
		if base == "palette " {
			base = "palette keymap"
		}
		switch base {
		case "info", "files", "palette keymap", "map", "shaders":
		default:
			continue
		}

		// Compile reads [Info] / [Files] separately: plain sections only.
		if !isLan && base == "info" && compileInfo {
			compileInfo = false
			d.Info.MugenVersion = s.Value("mugenversion")
			d.Info.IkemenVersion = s.Value("ikemenversion")
		}
		if !isLan && base == "files" && compileFiles {
			compileFiles = false
			d.Files.CMD = text.DecodeShiftJIS(s.Value("cmd"))
			d.Files.StCommon = text.DecodeShiftJIS(s.Value("stcommon"))
			var keys []string
			for _, k := range s.Keys() {
				if stKey.MatchString(k) {
					keys = append(keys, k)
				}
			}
			sort.SliceStable(keys, func(a, b int) bool { return text.NaturalLess(keys[a], keys[b]) })
			for _, k := range keys {
				d.Files.ST = append(d.Files.ST, text.DecodeShiftJIS(s.Value(k)))
			}
		}

		if (isLan && done[base+".lan"]) || (!isLan && done[base]) {
			dl.Warnf(diag.At(s.Line, 0), "duplicate-section", "duplicate section [%s%s] (ignored, the first one wins)", s.Name, s.Sub)
			continue
		}
		if isLan {
			done[base+".lan"] = true
		}
		done[base] = true

		switch base {
		case "info":
			d.readInfo(s, &dl)
		case "files":
			d.readFiles(s, &dl)
		case "palette keymap":
			d.Keymap = [12]int32{}
			for k, btn := range KeymapButtons {
				var n int32
				if s.ReadI32(btn, &n) {
					if n < 1 || n > PaletteMax {
						e := s.Entry(btn)
						dl.Warnf(diag.At(e.Line, e.Col), "keymap-range", "palette slot %d for %s is outside 1..%d; Ikemen uses 1", n, btn, PaletteMax)
						n = 1
					}
					d.Keymap[k] = n
				}
			}
		case "map":
			d.Map = map[string]float32{}
			for _, k := range s.Keys() {
				d.Map[k] = float32(text.Atof(s.Value(k)))
			}
		case "shaders":
			d.Shaders = map[string]string{}
			for _, k := range s.Keys() {
				d.Shaders[k] = s.Value(k)
			}
		}
	}
	sort.SliceStable(dl.Items, func(a, b int) bool {
		return dl.Items[a].Range.Start.Before(dl.Items[b].Range.Start)
	})
	d.Diagnostics = dl.Items
	return d
}

func (d *Def) readInfo(s *text.Section, dl *diag.List) {
	text1 := func(key string) (string, bool) {
		v, ok, err := s.Text(key)
		if err != nil {
			e := s.Entry(key)
			dl.Warnf(diag.At(e.Line, e.Col), "unquoted-string", "%s should be in double quotes", key)
		}
		return v, ok
	}
	in := Info{LocalCoord: [2]int32{320, 240}, PortraitScale: 1,
		MugenVersion: d.Info.MugenVersion, IkemenVersion: d.Info.IkemenVersion}
	in.Name, _ = text1("name")
	var ok bool
	if in.DisplayName, ok = text1("displayname"); !ok {
		in.DisplayName = in.Name
	}
	if in.LifebarName, ok = text1("lifebarname"); !ok {
		in.LifebarName = in.DisplayName
	}
	in.Author, _ = text1("author")
	if s.ReadI32("localcoord", &in.LocalCoord[0], &in.LocalCoord[1]) {
		if in.LocalCoord[0] == 0 {
			e := s.Entry("localcoord")
			dl.Warnf(diag.At(e.Line, e.Col), "localcoord-zero", "localcoord width is 0")
		} else {
			in.PortraitScale = 320 / float32(in.LocalCoord[0])
		}
	}
	s.ReadF32("portraitscale", &in.PortraitScale)
	d.Info = in
}

func (d *Def) readFiles(s *text.Section, dl *diag.List) {
	f := &d.Files
	f.CNS = text.DecodeShiftJIS(s.Value("cns"))
	f.Sprite = text.DecodeShiftJIS(s.Value("sprite"))
	f.Anim = text.DecodeShiftJIS(s.Value("anim"))
	f.Sound = text.DecodeShiftJIS(s.Value("sound"))
	f.Pal = map[int]string{}
	for i := 1; i <= PaletteMax; i++ {
		if v := text.DecodeShiftJIS(s.Value("pal" + itoa(i))); v != "" {
			f.Pal[i] = v
		}
	}
	f.Fonts = map[int]Font{}
	for _, k := range s.Keys() {
		if !strings.HasPrefix(k, "font") {
			continue
		}
		rest := k[4:]
		j := 0
		for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
			j++
		}
		if j == 0 {
			continue
		}
		n, _ := text.Atoi(rest[:j])
		ft := f.Fonts[int(n)]
		switch rest[j:] {
		case "":
			ft.Path = s.Value(k)
		case ".height":
			ft.Height, _ = text.Atoi(s.Value(k))
		default:
			continue
		}
		f.Fonts[int(n)] = ft
	}
	// loadMovelists: movelist wins over movelist0; movelistN for N >= 1.
	f.Movelists = map[int]string{}
	if v := text.DecodeShiftJIS(s.Value("movelist")); v != "" {
		f.Movelists[0] = v
	} else if v := text.DecodeShiftJIS(s.Value("movelist0")); v != "" {
		f.Movelists[0] = v
	}
	for _, k := range s.Keys() {
		if k == "movelist" || k == "movelist0" || !strings.HasPrefix(k, "movelist") {
			continue
		}
		idx := k[len("movelist"):]
		if isInt(idx) {
			if v := text.DecodeShiftJIS(s.Value(k)); v != "" {
				n, _ := text.Atoi(idx)
				if n >= 0 {
					f.Movelists[int(n)] = v
				}
			}
		}
	}
}

func isInt(s string) bool {
	if s == "" {
		return false
	}
	if s[0] == '-' || s[0] == '+' {
		s = s[1:]
	}
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	n := len(b)
	for ; i > 0; i /= 10 {
		n--
		b[n] = byte('0' + i%10)
	}
	return string(b[n:])
}

// Find resolves a file named in the DEF the way Ikemen's SearchFile does:
// relative to the DEF's folder, then to engineDir (Ikemen's working
// directory), then to engineDir/data. Backslashes, quotes and trailing
// comments are accepted, and the match is case-insensitive when the exact
// name does not exist. It returns "" when nothing is found.
func (d *Def) Find(name, engineDir string) string {
	name = strings.TrimSpace(name)
	if len(name) >= 2 {
		if q := name[0]; (q == '"' || q == '\'') && name[len(name)-1] == q {
			name = name[1 : len(name)-1]
		}
	}
	if sc := strings.Index(name, ";"); sc >= 0 {
		name = strings.TrimSpace(name[:sc])
	}
	name = strings.ReplaceAll(name, "\\", "/")
	if name == "" {
		return ""
	}
	if filepath.IsAbs(name) {
		return fileExist(name)
	}
	var roots []string
	if d.Path != "" {
		roots = append(roots, filepath.Dir(d.Path))
	}
	roots = append(roots, engineDir, filepath.Join(engineDir, "data"))
	for _, r := range roots {
		if found := fileExist(filepath.Join(r, name)); found != "" {
			return found
		}
	}
	return ""
}

// fileExist mirrors FileExist (without zip support): the exact path, or a
// case-insensitive match of each path element. Ikemen uses a glob for the
// fallback; walking the folders gives the same answer on every OS.
func fileExist(p string) string {
	if info, err := os.Stat(p); err == nil {
		if info.IsDir() {
			return ""
		}
		return filepath.ToSlash(p)
	}
	p = filepath.Clean(p)
	vol := filepath.VolumeName(p)
	rest := p[len(vol):]
	cur := vol
	if strings.HasPrefix(rest, string(filepath.Separator)) {
		cur += string(filepath.Separator)
		rest = rest[1:]
	}
	if cur == "" {
		cur = "."
	}
	parts := strings.Split(rest, string(filepath.Separator))
	for k, part := range parts {
		next := filepath.Join(cur, part)
		if _, err := os.Stat(next); err != nil {
			entries, err := os.ReadDir(cur)
			if err != nil {
				return ""
			}
			next = ""
			for _, e := range entries {
				if strings.EqualFold(e.Name(), part) {
					next = filepath.Join(cur, e.Name())
					break
				}
			}
			if next == "" {
				return ""
			}
		}
		cur = next
		if k == len(parts)-1 {
			if info, err := os.Stat(cur); err != nil || info.IsDir() {
				return ""
			}
		}
	}
	return filepath.ToSlash(cur)
}

// FindState resolves a state file (st*, stcommon) like Find, then, as
// stateCompile does for a missing non-ZSS file, tries name + ".zss" (KFM's
// "common1.cns" is shipped as "common1.cns.zss").
func (d *Def) FindState(name, engineDir string) string {
	if p := d.Find(name, engineDir); p != "" {
		return p
	}
	if !strings.EqualFold(filepath.Ext(strings.TrimSpace(name)), ".zss") {
		return d.Find(strings.TrimSpace(name)+".zss", engineDir)
	}
	return ""
}

// EngineDir guesses Ikemen's working directory for a character DEF: the
// nearest parent folder that contains a "data" folder. It returns "" when
// there is none.
func EngineDir(defPath string) string {
	dir, _ := filepath.Abs(filepath.Dir(defPath))
	for {
		if info, err := os.Stat(filepath.Join(dir, "data")); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
