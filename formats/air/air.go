// Package air reads AIR animation files the way Ikemen GO does.
//
// Upstream: ikemen/source/src/anim.go @ v1.0.0 (81c6da71d689625e20db79586815b695da00dd6d)
//
//	ReadAnimFrame                    (anim.go:45)   -> parseFrame
//	ReadAnimation                    (anim.go:274)  -> parseAction (incl. timing)
//	ReadAction                       (anim.go:439)  -> readAction
//	AnimationTable.readAction        (anim.go:1171) -> readTableEntry
//	ReadAnimationTable               (anim.go:1212) -> Load / Parse + Resolve
//	AnimationTable.resolveCopyAction (anim.go:1238) -> Resolve
//	char.go:4285-4342 (character AIR + common AIR merge) -> Merge
//
// Changes from upstream:
//   - Engine-only state (sprite pointers, playback cursors, palettes) is dropped.
//   - Every Action and Frame records its 1-based source line, and LogMessage
//     warnings become Diagnostics with line numbers. All frame errors are kept
//     (upstream keeps only the last one per action).
//   - A warning is added when a Clsn block has fewer boxes than declared
//     (upstream silently leaves the missing boxes as 0,0,0,0, and so do we).
//   - Copy Action keeps the copying action's own Number and Line, and records
//     the source in CopiedFrom.
//
// MIT License
// Copyright (c) 2016 Suehiro
// Copyright (c) 2018 K4thos, NeatUnsou, ShinLucho
// Copyright (c) 2019 Daniel Porto, Gacel, Nikolay Griva
// See ikemen/source/LICENCE.txt for the full notice.
package air

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/whoisnighter/KataFighter/formats/internal/text"
)

// Trans is a frame's blending mode.
type Trans int

const (
	TransNone Trans = iota
	TransAdd
	TransSub
	TransSubAdd
)

func (t Trans) String() string {
	return [...]string{"none", "add", "sub", "subadd"}[t]
}

// Box is a collision box: left, top, right, bottom (normalized so l<=r, t<=b).
type Box [4]float32

// Frame is one animation element.
type Frame struct {
	Line   int
	Group  int32
	Number int32
	// Offset as written in the file.
	Offset [2]int32
	// EngineOffset is what Ikemen uses: each H flag negates x, each V flag negates y.
	EngineOffset [2]int32
	Time         int32 // -1 = infinite
	FlipH, FlipV bool
	Trans        Trans
	SrcAlpha     byte
	DstAlpha     byte
	Scale        [2]float32
	Angle        float32
	// Clsn1 (attack) and Clsn2 (hurt) boxes after applying defaults.
	// nil = no boxes; an empty non-nil slice = explicitly zero boxes.
	Clsn1, Clsn2 []Box
}

// Action is one [Begin Action N] block.
type Action struct {
	Number int32
	Line   int
	Frames []Frame
	// LoopStart is the index of the frame the loop restarts from.
	LoopStart int
	// Frame indices that follow an "Interpolate X" line.
	InterpolateOffset, InterpolateScale, InterpolateAngle, InterpolateBlend []int
	// CopyAction is the target of an unresolved "Copy Action N" line, or -1.
	CopyAction int32
	// CopiedFrom is the action whose data this one took, or -1. It is set by a
	// resolved Copy Action, and when an empty action took the next action's
	// frames (an Ikemen/MUGEN quirk).
	CopiedFrom int32

	// Timing as computed by Ikemen (-1 TotalTime = ends on an infinite frame).
	TotalTime, LoopTime, PreLoopTime int32
}

// Severity of a diagnostic.
type Severity int

const (
	Warning Severity = iota
	Error
)

// Diagnostic is a problem found while parsing. Line is 1-based (0 = whole file).
type Diagnostic struct {
	Line     int
	Severity Severity
	Message  string
}

func (d Diagnostic) String() string {
	sev := "warning"
	if d.Severity == Error {
		sev = "error"
	}
	if d.Line > 0 {
		return fmt.Sprintf("%d: %s: %s", d.Line, sev, d.Message)
	}
	return fmt.Sprintf("%s: %s", sev, d.Message)
}

// File is a parsed AIR file (or several merged).
type File struct {
	Actions     map[int32]*Action
	Diagnostics []Diagnostic
}

// Numbers returns the action numbers in ascending order.
func (f *File) Numbers() []int32 {
	ns := make([]int32, 0, len(f.Actions))
	for n := range f.Actions {
		ns = append(ns, n)
	}
	sort.Slice(ns, func(i, j int) bool { return ns[i] < ns[j] })
	return ns
}

// Load reads, parses and resolves one AIR file (like ReadAnimationTable).
func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f := Parse(string(text.StripBOM(b)))
	f.Resolve()
	return f, nil
}

// Parse parses AIR text without resolving Copy Action, so that other files
// can be merged first (see Merge).
func Parse(src string) *File {
	p := &parser{f: &File{Actions: make(map[int32]*Action)}, lines: text.SplitAndTrim(src, "\n")}
	for i := 0; p.readTableEntry(&i) != nil; {
	}
	return p.f
}

// Merge adds the actions of other that f does not have yet (char.go: common
// AIR files fill in actions missing from the character's AIR).
func (f *File) Merge(other *File) {
	for no, a := range other.Actions {
		if f.Actions[no] == nil {
			f.Actions[no] = a
		}
	}
}

// Resolve applies Copy Action. Chains are followed up to 8 links; actions
// whose copy cannot be resolved are removed, as in Ikemen.
func (f *File) Resolve() {
	var toDelete []int32
	for _, no := range f.Numbers() {
		a := f.Actions[no]
		for loops := 0; a.CopyAction >= 0 && loops < 8; loops++ {
			target := f.Actions[a.CopyAction]
			if target == nil || target == a {
				break
			}
			if target.CopyAction >= 0 {
				a.CopyAction = target.CopyAction
				continue
			}
			number, line, from := a.Number, a.Line, target.Number
			*a = *target
			a.Number, a.Line, a.CopiedFrom = number, line, from
			a.CopyAction = -1
			break
		}
		if a.CopyAction >= 0 {
			f.diag(a.Line, Warning, "action %d: Copy Action %d cannot be resolved; action removed", no, a.CopyAction)
			toDelete = append(toDelete, no)
		}
	}
	for _, no := range toDelete {
		delete(f.Actions, no)
	}
}

func (f *File) diag(line int, sev Severity, format string, a ...any) {
	f.Diagnostics = append(f.Diagnostics, Diagnostic{Line: line, Severity: sev, Message: fmt.Sprintf(format, a...)})
}

type parser struct {
	f     *File
	lines []string
}

func (p *parser) atoi(s string, line int) int32 {
	n, overflow := text.Atoi(s)
	if overflow {
		p.f.diag(line, Warning, "integer outside int32 range: %s", strings.TrimSpace(s))
	}
	return n
}

// readTableEntry reads the next action into the table, mirroring
// AnimationTable.readAction. It returns nil when no action is left.
func (p *parser) readTableEntry(i *int) *Action {
	for *i < len(p.lines) {
		no, a, ok := p.readAction(i)
		if a != nil {
			// Duplicate action numbers: the first one wins, even a Copy Action
			if existing := p.f.Actions[no]; existing != nil {
				p.f.diag(a.Line, Warning, "duplicate action %d (ignored)", no)
				return existing
			}
			// An empty action takes the very next action found in the file
			logged := false
			for len(a.Frames) == 0 && *i < len(p.lines) && a.CopyAction < 0 {
				if !logged {
					p.f.diag(a.Line, Warning, "action %d has no valid frames", no)
					logged = true
				}
				if a2 := p.readTableEntry(i); a2 != nil {
					line := a.Line
					*a = *a2
					a.Number, a.Line, a.CopiedFrom = no, line, a2.Number
					break
				}
				(*i)++
			}
			p.f.Actions[no] = a
			return a
		}
		if !ok {
			(*i)++
		}
	}
	return nil
}

// readAction mirrors ReadAction: it skips to the next section header and
// parses it when it is [Begin Action N].
func (p *parser) readAction(i *int) (no int32, a *Action, ok bool) {
	var name, subname string
	for ; *i < len(p.lines); (*i)++ {
		name, subname = text.SectionName(p.lines[*i])
		if len(name) > 0 {
			break
		}
	}
	if name != "begin " {
		return
	}
	spi := strings.Index(subname, " ")
	if spi < 0 || strings.ToLower(subname[:spi+1]) != "action " {
		return
	}
	header := *i + 1
	(*i)++
	no = p.atoi(subname[spi+1:], header)
	a = p.parseAction(i)
	a.Number, a.Line = no, header
	return no, a, true
}

func (p *parser) parseAction(i *int) *Action {
	a := &Action{CopyAction: -1, CopiedFrom: -1}
	ols := 0
	var clsn1, clsn1d, clsn2, clsn2d []Box
	def1, def2 := true, true

	for ; *i < len(p.lines); (*i)++ {
		if len(p.lines[*i]) > 0 && p.lines[*i][0] == '[' {
			break
		}
		lineNo := *i + 1
		line := strings.ToLower(strings.TrimSpace(strings.SplitN(p.lines[*i], ";", 2)[0]))

		// Copy Action
		if len(line) >= 12 && line[:12] == "copy action " {
			numStr := strings.TrimSpace(line[12:])
			id, err := strconv.ParseInt(numStr, 10, 32)
			if err != nil || numStr == "" {
				a.CopyAction = -1 // invalid number: treated as empty
				p.f.diag(lineNo, Warning, "invalid Copy Action number %q", numStr)
			} else {
				a.CopyAction = int32(id)
			}
			return a
		}

		af, msgs := p.parseFrame(line, lineNo)
		for _, m := range msgs {
			p.f.diag(lineNo, Warning, "%s", m)
		}

		switch {
		case af != nil:
			ols = a.LoopStart
			if def1 {
				clsn1 = clsn1d
			}
			if def2 {
				clsn2 = clsn2d
			}
			af.Clsn1, af.Clsn2 = clsn1, clsn2
			a.Frames = append(a.Frames, *af)
			def1, def2 = true, true
		case len(line) >= 9 && line[:9] == "loopstart":
			a.LoopStart = len(a.Frames)
		case len(line) >= 18 && line[:18] == "interpolate offset":
			a.InterpolateOffset = append(a.InterpolateOffset, len(a.Frames))
		case len(line) >= 17 && line[:17] == "interpolate scale":
			a.InterpolateScale = append(a.InterpolateScale, len(a.Frames))
		case len(line) >= 17 && line[:17] == "interpolate angle":
			a.InterpolateAngle = append(a.InterpolateAngle, len(a.Frames))
		case len(line) >= 17 && line[:17] == "interpolate blend":
			a.InterpolateBlend = append(a.InterpolateBlend, len(a.Frames))
		case len(line) >= 5 && line[:4] == "clsn":
			ii := strings.Index(line, ":")
			if ii < 0 {
				break
			}
			size := p.atoi(line[ii+1:], lineNo) // number of expected boxes
			if size < 0 {
				break
			}
			var clsn []Box
			if line[4] == '1' {
				clsn1 = make([]Box, size)
				clsn = clsn1
				if len(line) >= 12 && line[5:12] == "default" {
					clsn1d = clsn1
				}
				def1 = false
			} else if line[4] == '2' {
				clsn2 = make([]Box, size)
				clsn = clsn2
				if len(line) >= 12 && line[5:12] == "default" {
					clsn2d = clsn2
				}
				def2 = false
			} else {
				break
			}
			if size == 0 {
				break
			}
			(*i)++
			n := int32(0)
			for n < size && *i < len(p.lines) {
				line := strings.ToLower(strings.TrimSpace(strings.SplitN(p.lines[*i], ";", 2)[0]))
				if len(line) == 0 {
					(*i)++
					continue
				}
				if len(line) < 4 || line[:4] != "clsn" {
					break
				}
				ii := strings.Index(line, "=")
				if ii < 0 {
					break
				}
				ary := strings.Split(line[ii+1:], ",")
				if len(ary) < 4 {
					break
				}
				ln := *i + 1
				l, t, r, b := p.atoi(ary[0], ln), p.atoi(ary[1], ln), p.atoi(ary[2], ln), p.atoi(ary[3], ln)
				if l > r {
					l, r = r, l
				}
				if t > b {
					t, b = b, t
				}
				clsn[n] = Box{float32(l), float32(t), float32(r), float32(b)}
				n++
				(*i)++
			}
			if n < size {
				p.f.diag(lineNo, Warning, "Clsn%c declares %d boxes but only %d follow", line[4], size, n)
			}
			(*i)-- // back up so the line after the last box is not skipped
		}
	}

	if a.LoopStart >= len(a.Frames) {
		a.LoopStart = ols
	}
	a.computeTiming()
	return a
}

func (a *Action) computeTiming() {
	if len(a.Frames) == 0 {
		return
	}
	if a.Frames[len(a.Frames)-1].Time == -1 {
		a.TotalTime = -1
		return
	}
	tmp := int32(0)
	for i, f := range a.Frames {
		if f.Time == -1 {
			a.TotalTime = 0
			a.LoopTime = -tmp
			a.PreLoopTime = 0
		}
		a.TotalTime += f.Time
		if i < a.LoopStart {
			a.PreLoopTime += f.Time
			tmp += f.Time
		} else {
			a.LoopTime += f.Time
		}
	}
	if a.TotalTime == -1 {
		a.PreLoopTime = 0
	}
}

// parseFrame mirrors ReadAnimFrame. It returns nil for lines that are not
// frames, plus any problems found on the line.
func (p *parser) parseFrame(line string, lineNo int) (*Frame, []string) {
	if len(line) == 0 || (line[0] < '0' || '9' < line[0]) && line[0] != '-' {
		return nil, nil
	}
	raw := strings.SplitN(line, ",", 10)
	ary := make([]string, len(raw))
	for i, s := range raw {
		ary[i] = strings.TrimSpace(s)
	}
	if len(ary) < 5 {
		return nil, []string{"invalid animation element definition"}
	}
	var msgs []string
	af := &Frame{Line: lineNo, SrcAlpha: 255, Scale: [2]float32{1, 1}}
	af.Group = p.atoi(ary[0], lineNo)
	af.Number = p.atoi(ary[1], lineNo)
	af.Offset = [2]int32{p.atoi(ary[2], lineNo), p.atoi(ary[3], lineNo)}
	af.EngineOffset = af.Offset
	af.Time = p.atoi(ary[4], lineNo)

	// H and V flags
	if len(ary) >= 6 && ary[5] != "" {
		for _, ch := range ary[5] {
			switch ch {
			case 'H', 'h':
				af.FlipH = true
				af.EngineOffset[0] *= -1
			case 'V', 'v':
				af.FlipV = true
				af.EngineOffset[1] *= -1
			default:
				msgs = append(msgs, fmt.Sprintf("invalid animation element flip flag: %c", ch))
			}
		}
	}

	// Transparency
	if len(ary) >= 7 && ary[6] != "" {
		parseAlpha := func(a string, start int, defSrc, defDst byte) (src, dst byte) {
			src, dst = defSrc, defDst
			i, alp := start, 0
			for ; i < len(a) && a[i] >= '0' && a[i] <= '9'; i++ {
				alp = alp*10 + int(a[i]-'0')
			}
			alp &= 0x3fff
			if alp > 255 {
				alp = 255
			}
			src = byte(alp)
			if i < len(a) && a[i] == 'd' {
				i++
				if i < len(a) && a[i] >= '0' && a[i] <= '9' {
					alp = 0
					for ; i < len(a) && a[i] >= '0' && a[i] <= '9'; i++ {
						alp = alp*10 + int(a[i]-'0')
					}
					alp &= 0x3fff
					if alp > 255 {
						alp = 255
					}
					dst = byte(alp)
				}
			}
			return
		}
		transErr := func(s string) {
			msgs = append(msgs, fmt.Sprintf("invalid animation element transparency: %v", s))
		}
		t := ary[6]
		if ia := strings.IndexAny(t, "ASas"); ia >= 0 {
			t = t[ia:]
		}
		a := strings.ToLower(t)
		switch {
		case a == "a1":
			af.Trans, af.SrcAlpha, af.DstAlpha = TransAdd, 255, 128
		case len(a) >= 3 && a[:3] == "sas":
			af.Trans = TransSubAdd
			af.SrcAlpha, af.DstAlpha = parseAlpha(a, 3, 255, 255)
		case len(a) >= 2 && a[:2] == "sa":
			af.Trans, af.SrcAlpha, af.DstAlpha = TransSubAdd, 255, 255
			if len(a) > 2 {
				transErr(a)
			}
		case len(a) >= 2 && a[:2] == "as":
			af.Trans = TransAdd
			af.SrcAlpha, af.DstAlpha = parseAlpha(a, 2, 255, 0) // MUGEN defaults to 0 here
		case len(a) >= 2 && a[:2] == "ss":
			af.Trans = TransSub
			af.SrcAlpha, af.DstAlpha = parseAlpha(a, 2, 255, 255)
		case len(a) >= 1 && a[0] == 'a':
			af.Trans, af.SrcAlpha, af.DstAlpha = TransAdd, 255, 255
			if len(a) > 1 {
				transErr(a)
			}
		case len(a) >= 1 && a[0] == 's':
			af.Trans, af.SrcAlpha, af.DstAlpha = TransSub, 255, 255
			if len(a) > 1 {
				transErr(a)
			}
		default:
			transErr(a)
		}
	}

	// X scale, Y scale, angle: blank means "no change" in Ikemen (0 in MUGEN 1.1)
	for k, dst := range []*float32{&af.Scale[0], &af.Scale[1], &af.Angle} {
		idx := 7 + k
		if len(ary) > idx && ary[idx] != "" {
			if text.IsNumeric(ary[idx]) {
				*dst = float32(text.Atof(ary[idx]))
			} else {
				msgs = append(msgs, fmt.Sprintf("invalid animation element %s: %v",
					[...]string{"x-scale", "y-scale", "angle"}[k], ary[idx]))
			}
		}
	}
	return af, msgs
}
