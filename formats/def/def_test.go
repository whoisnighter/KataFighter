package def

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/whoisnighter/KataFighter/formats/diag"
)

const release = "../../ikemen/release/Ikemen_GO-v1.0.0-windows"

func codes(ds []diag.Diagnostic) string {
	var cs []string
	for _, d := range ds {
		cs = append(cs, d.Range.Start.String()+" "+d.Code)
	}
	return strings.Join(cs, ", ")
}

func TestParse(t *testing.T) {
	d := Parse(`; comment
[Info]
name = "Kung Fu Man"  ; trailing comment
author = Elecbyte
name = "Second"
localcoord = 640,480
mugenversion = 1.1
ikemenversion = 1.0

[Files]
cmd = kfm.cmd
cns = kfm.cns
st10 = ten.cns
st = kfm.cns
st2 = two.cns
St1 = one.cns
stcommon = common1.cns
sprite = kfm.sff
pal1 = a.act
pal12 = b.act
font1 = f.fnt
font1.height = 12
movelist0 = zero.dat
movelist2 = two.dat

[Palette Keymap]
x = 1
a = 200
z2 = 3

[Map]
Foo = 1.5

[Info]
name = "ignored"
`, Options{})
	in := d.Info
	if in.Name != "Kung Fu Man" || in.DisplayName != "Kung Fu Man" || in.LifebarName != "Kung Fu Man" {
		t.Errorf("names %+v", in)
	}
	if in.Author != "Elecbyte" { // unquoted: kept as written, with a warning
		t.Errorf("author %q", in.Author)
	}
	if in.LocalCoord != [2]int32{640, 480} || in.PortraitScale != 0.5 {
		t.Errorf("localcoord %v scale %v", in.LocalCoord, in.PortraitScale)
	}
	if in.MugenVersion != "1.1" || in.IkemenVersion != "1.0" {
		t.Errorf("versions %q %q", in.MugenVersion, in.IkemenVersion)
	}
	f := d.Files
	if got := strings.Join(f.ST, " "); got != "kfm.cns one.cns two.cns ten.cns" {
		t.Errorf("st order %q", got)
	}
	if f.CMD != "kfm.cmd" || f.StCommon != "common1.cns" || f.Sprite != "kfm.sff" || f.CNS != "kfm.cns" {
		t.Errorf("files %+v", f)
	}
	if f.Pal[1] != "a.act" || f.Pal[12] != "b.act" || len(f.Pal) != 2 {
		t.Errorf("pal %v", f.Pal)
	}
	if f.Fonts[1] != (Font{"f.fnt", 12}) {
		t.Errorf("fonts %v", f.Fonts)
	}
	if f.Movelists[0] != "zero.dat" || f.Movelists[2] != "two.dat" {
		t.Errorf("movelists %v", f.Movelists)
	}
	// x=slot 0, a=slot 3 (out of range -> 1), z2=slot 11
	if d.Keymap[3] != 1 || d.Keymap[0] != 1 || d.Keymap[11] != 3 || d.Keymap[1] != 0 {
		t.Errorf("keymap %v", d.Keymap)
	}
	if d.Map["foo"] != 1.5 {
		t.Errorf("map %v", d.Map)
	}
	want := "4:1 unquoted-string, 5:1 duplicate-key, 28:1 keymap-range, 34 duplicate-section"
	if got := codes(d.Diagnostics); got != want {
		t.Errorf("diagnostics\n got %s\nwant %s", got, want)
	}
}

func TestLanguageSections(t *testing.T) {
	src := `
[Info]
name = "Plain"
mugenversion = 1.0
[ja.Info]
name = "Japanese"
mugenversion = 9.9
[ja.Info]
name = "Second"
`
	if d := Parse(src, Options{}); d.Info.Name != "Plain" {
		t.Errorf("en: %q", d.Info.Name)
	}
	d := Parse(src, Options{Language: "ja"})
	if d.Info.Name != "Japanese" {
		t.Errorf("ja: %q", d.Info.Name)
	}
	if d.Info.MugenVersion != "1.0" { // Compile only reads the plain [Info]
		t.Errorf("mugenversion %q", d.Info.MugenVersion)
	}
	// Localized first: it blocks a later plain section.
	d = Parse("[ja.Info]\nname=\"J\"\n[Info]\nname=\"P\"\n", Options{Language: "ja"})
	if d.Info.Name != "J" {
		t.Errorf("localized first: %q", d.Info.Name)
	}
}

func TestDisplayNameDefaults(t *testing.T) {
	d := Parse("[Info]\nname = \"A\"\ndisplayname = \"B\"\n", Options{})
	if d.Info.DisplayName != "B" || d.Info.LifebarName != "B" {
		t.Errorf("%+v", d.Info)
	}
	d = Parse("", Options{})
	if d.Info.LocalCoord != [2]int32{320, 240} || d.Info.PortraitScale != 1 {
		t.Errorf("defaults %+v", d.Info)
	}
}

func TestShiftJIS(t *testing.T) {
	// "ア" in Shift_JIS is 0x83 0x41.
	d := Parse("[Files]\nsprite = \x83\x41.sff\n", Options{})
	if d.Files.Sprite != "ア.sff" {
		t.Errorf("sprite %q", d.Files.Sprite)
	}
}

// hasSuffixFold ignores case: on Windows and macOS the exact-case lookup
// already succeeds and returns the name as written.
func hasSuffixFold(s, suffix string) bool {
	return len(s) >= len(suffix) && strings.EqualFold(s[len(s)-len(suffix):], suffix)
}

func TestFind(t *testing.T) {
	root := t.TempDir()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(root, "chars", "a", "Sub"), 0o755))
	must(os.MkdirAll(filepath.Join(root, "data"), 0o755))
	for _, p := range []string{"chars/a/a.def", "chars/a/Sub/Kfm.SFF", "data/common1.cns.zss", "data/x.cmd"} {
		must(os.WriteFile(filepath.Join(root, filepath.FromSlash(p)), nil, 0o644))
	}
	d := &Def{Path: filepath.Join(root, "chars", "a", "a.def")}
	if EngineDir(d.Path) == "" {
		t.Fatal("EngineDir found nothing")
	}
	if got := d.Find(`sub\kfm.sff ; comment`, root); !hasSuffixFold(got, "chars/a/Sub/Kfm.SFF") {
		t.Errorf("case-insensitive find: %q", got)
	}
	if got := d.Find(`"sub/KFM.sff"`, root); !hasSuffixFold(got, "chars/a/Sub/Kfm.SFF") {
		t.Errorf("quoted find: %q", got)
	}
	if got := d.Find("x.cmd", root); !strings.HasSuffix(got, "data/x.cmd") {
		t.Errorf("data/ fallback: %q", got)
	}
	if got := d.Find("missing.cns", root); got != "" {
		t.Errorf("missing: %q", got)
	}
	if got := d.FindState("common1.cns", root); !strings.HasSuffix(got, "data/common1.cns.zss") {
		t.Errorf("zss fallback: %q", got)
	}
}

func TestCorpus(t *testing.T) {
	defs, _ := filepath.Glob(release + "/chars/*/*.def")
	n := 0
	for _, p := range defs {
		base := strings.ToLower(filepath.Base(p))
		if base == "intro.def" || base == "ending.def" {
			continue // storyboards, not characters
		}
		n++
		t.Run(filepath.Base(p), func(t *testing.T) {
			d, err := Load(p, Options{})
			if err != nil {
				t.Fatal(err)
			}
			for _, dg := range d.Diagnostics {
				t.Error(dg)
			}
			if !strings.HasPrefix(d.Info.Name, "Kung Fu Man") {
				t.Errorf("name %q", d.Info.Name)
			}
			engine := EngineDir(p)
			files := map[string]string{"cns": d.Files.CNS, "sprite": d.Files.Sprite, "anim": d.Files.Anim,
				"sound": d.Files.Sound, "cmd": d.Files.CMD}
			for k, v := range files {
				if d.Find(v, engine) == "" {
					t.Errorf("%s = %q not found", k, v)
				}
			}
			for _, st := range append(d.Files.ST, d.Files.StCommon) {
				if d.FindState(st, engine) == "" {
					t.Errorf("state file %q not found", st)
				}
			}
			if d.Keymap[0] == 0 {
				t.Error("keymap not read")
			}
		})
	}
	if n == 0 {
		t.Skip("no Ikemen GO release in ikemen/release")
	}
}
