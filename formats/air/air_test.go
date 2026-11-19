package air

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/whoisnighter/KataFighter/formats/sff"
)

const release = "../../ikemen/release/Ikemen_GO-v1.0.0-windows"

func parse(t *testing.T, src string) *File {
	t.Helper()
	f := Parse(src)
	f.Resolve()
	return f
}

func TestFrameFields(t *testing.T) {
	f := parse(t, `
[Begin Action 200] ; comment
200,0, 10,-5, 4, H, AS128D64, 1.5, , 45
200,1, 3,2, -1, VH, A1
200,2, 0,0, 2, , S
200,3, 0,0, 2, , xx
`)
	a := f.Actions[200]
	if a == nil || len(a.Frames) != 4 {
		t.Fatalf("got %+v", a)
	}
	fr := a.Frames[0]
	if fr.Line != 3 || fr.Group != 200 || fr.Time != 4 || !fr.FlipH || fr.FlipV {
		t.Errorf("frame 0 = %+v", fr)
	}
	if fr.Offset != [2]int32{10, -5} || fr.EngineOffset != [2]int32{-10, -5} {
		t.Errorf("offsets %v %v", fr.Offset, fr.EngineOffset)
	}
	if fr.Trans != TransAdd || fr.SrcAlpha != 128 || fr.DstAlpha != 64 {
		t.Errorf("trans %v %d %d", fr.Trans, fr.SrcAlpha, fr.DstAlpha)
	}
	if fr.Scale != [2]float32{1.5, 1} || fr.Angle != 45 { // blank y-scale = unchanged in Ikemen
		t.Errorf("scale %v angle %v", fr.Scale, fr.Angle)
	}
	if fr := a.Frames[1]; fr.Trans != TransAdd || fr.DstAlpha != 128 || fr.EngineOffset != [2]int32{-3, -2} {
		t.Errorf("frame 1 = %+v", fr)
	}
	if fr := a.Frames[2]; fr.Trans != TransSub {
		t.Errorf("frame 2 trans = %v", fr.Trans)
	}
	// A -1 frame in the middle resets the counters (Ikemen quirk): 4,-1,2,2 -> 0-1+2+2
	if a.TotalTime != 3 || a.LoopTime != 3 || a.PreLoopTime != 0 {
		t.Errorf("timing total=%d loop=%d pre=%d", a.TotalTime, a.LoopTime, a.PreLoopTime)
	}
	if len(f.Diagnostics) != 1 || f.Diagnostics[0].Line != 6 {
		t.Errorf("diagnostics = %v", f.Diagnostics)
	}
}

func TestClsnDefaultsAndLoopstart(t *testing.T) {
	f := parse(t, `
[Begin Action 0]
Clsn2Default: 1
 Clsn2[0] = -10, 0, 10, -80
0,0, 0,0, 5
Clsn1: 1
 Clsn1[0] = 30,-50, 5,-60
Clsn2: 0
0,1, 0,0, 5
Loopstart
0,2, 0,0, 5
Clsn1: 2
 Clsn1[0] = 0,0,1,1
0,3, 0,0, 5
`)
	a := f.Actions[0]
	if len(a.Frames) != 4 || a.LoopStart != 2 {
		t.Fatalf("frames=%d loopstart=%d", len(a.Frames), a.LoopStart)
	}
	want2 := Box{-10, -80, 10, 0} // normalized
	if c := a.Frames[0].Clsn2; len(c) != 1 || c[0] != want2 {
		t.Errorf("frame 0 clsn2 = %v", c)
	}
	if c := a.Frames[1].Clsn1; len(c) != 1 || c[0] != (Box{5, -60, 30, -50}) {
		t.Errorf("frame 1 clsn1 = %v", c)
	}
	if c := a.Frames[1].Clsn2; c == nil || len(c) != 0 {
		t.Errorf("frame 1 clsn2 should be explicitly empty, got %v", c)
	}
	if c := a.Frames[2]; c.Clsn1 != nil || len(c.Clsn2) != 1 {
		t.Errorf("frame 2 should fall back to defaults, got %v %v", c.Clsn1, c.Clsn2)
	}
	if c := a.Frames[3].Clsn1; len(c) != 2 || c[1] != (Box{}) {
		t.Errorf("frame 3 clsn1 = %v (missing box stays zero)", c)
	}
	if a.TotalTime != 20 || a.PreLoopTime != 10 || a.LoopTime != 10 {
		t.Errorf("timing %d %d %d", a.TotalTime, a.PreLoopTime, a.LoopTime)
	}
	if len(f.Diagnostics) != 1 || !strings.Contains(f.Diagnostics[0].Message, "declares 2 boxes") {
		t.Errorf("diagnostics = %v", f.Diagnostics)
	}
}

func TestCopyEmptyAndDuplicate(t *testing.T) {
	f := parse(t, `
[Begin Action 1]
1,0, 0,0, 3
[Begin Action 2]
Copy Action 3
[Begin Action 3]
Copy Action 1
[Begin Action 4]
Copy Action 99
[Begin Action 5]
; empty: takes the next action
[Begin Action 6]
6,0, 0,0, 1
[Begin Action 1]
9,9, 0,0, 9
`)
	if a := f.Actions[2]; a == nil || a.Number != 2 || a.CopiedFrom != 1 || a.Frames[0].Group != 1 {
		t.Errorf("action 2 (chain 2->3->1) = %+v", a)
	}
	if f.Actions[4] != nil {
		t.Error("unresolvable copy action 4 should be removed")
	}
	if a := f.Actions[5]; a == nil || a.CopiedFrom != 6 || a.Frames[0].Group != 6 || a.Line != 10 {
		t.Errorf("empty action 5 = %+v", a)
	}
	if f.Actions[6] == nil {
		t.Error("action 6 should also be stored")
	}
	if a := f.Actions[1]; a.Frames[0].Group != 1 {
		t.Error("first duplicate must win")
	}
	if len(f.Diagnostics) != 3 {
		t.Errorf("diagnostics = %v", f.Diagnostics)
	}
}

func TestCorpus(t *testing.T) {
	var files []string
	filepath.WalkDir(release, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".air") {
			files = append(files, p)
		}
		return nil
	})
	if len(files) == 0 {
		t.Skip("no Ikemen GO release in ikemen/release")
	}
	for _, p := range files {
		t.Run(filepath.ToSlash(p), func(t *testing.T) {
			f, err := Load(p)
			if err != nil {
				t.Fatal(err)
			}
			if len(f.Actions) == 0 {
				t.Error("no actions")
			}
			for _, d := range f.Diagnostics {
				t.Log(d)
			}
		})
	}
}

// Every frame of KFM must point at a sprite that exists in kfm.sff
// (Ikemen's debugMissing check), after merging data/common.air like char.go.
func TestKFMSpritesExist(t *testing.T) {
	dir := filepath.Join(release, "chars", "kfm")
	b, err := os.ReadFile(filepath.Join(dir, "kfm.air"))
	if err != nil {
		t.Skip("KFM not found")
	}
	f := Parse(string(b))
	common, err := os.ReadFile(filepath.Join(release, "data", "common.air"))
	if err != nil {
		t.Fatal(err)
	}
	before := len(f.Actions)
	f.Merge(Parse(string(common)))
	f.Resolve()
	t.Logf("kfm.air %d actions, %d after merging common.air", before, len(f.Actions))

	s, err := sff.Open(filepath.Join(dir, "kfm.sff"), sff.Options{Character: true})
	if err != nil {
		t.Fatal(err)
	}
	missing := map[[2]int32][]int32{}
	for _, no := range f.Numbers() {
		for _, fr := range f.Actions[no].Frames {
			if fr.Group < 0 || fr.Group > 0xFFFF || fr.Number < 0 || fr.Number > 0xFFFF {
				continue // -1 = blank frame
			}
			if s.Sprite(uint16(fr.Group), uint16(fr.Number)) == nil {
				k := [2]int32{fr.Group, fr.Number}
				missing[k] = append(missing[k], no)
			}
		}
	}
	for k, acts := range missing {
		t.Logf("sprite %d,%d missing (actions %v)", k[0], k[1], acts)
	}
	if len(missing) > 0 {
		t.Errorf("%d sprites referenced by kfm.air are missing from kfm.sff", len(missing))
	}
}
