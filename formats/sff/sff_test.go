package sff

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// corpus returns every .sff in the local Ikemen GO release (ikemen/release).
// Tests that need it are skipped when no release is unzipped there.
func corpus(t testing.TB) []string {
	t.Helper()
	var files []string
	filepath.WalkDir("../../ikemen/release", func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".sff") {
			files = append(files, p)
		}
		return nil
	})
	if len(files) == 0 {
		t.Skip("no Ikemen GO release in ikemen/release")
	}
	return files
}

func isCharSff(p string) bool {
	return strings.Contains(filepath.ToSlash(p), "/chars/") &&
		!strings.Contains(p, "intro") && !strings.Contains(p, "ending")
}

func TestCorpusLoads(t *testing.T) {
	for _, p := range corpus(t) {
		t.Run(filepath.ToSlash(p), func(t *testing.T) {
			f, err := Open(p, Options{Character: isCharSff(p)})
			if err != nil {
				t.Fatal(err)
			}
			if len(f.Sprites) != int(f.Header.NumberOfSprites) {
				t.Errorf("loaded %d sprites, header says %d", len(f.Sprites), f.Header.NumberOfSprites)
			}
			for _, w := range f.Diagnostics {
				t.Log(w)
			}
			for _, s := range f.Sprites {
				if len(s.Pix) > 0 && len(s.Pix) != int(s.Size[0])*int(s.Size[1]) {
					t.Errorf("sprite %d,%d: %d pixels for %dx%d", s.Group, s.Number, len(s.Pix), s.Size[0], s.Size[1])
				}
				if s.HasImage() {
					if _, err := f.Image(s, nil); err != nil {
						t.Errorf("sprite %d,%d: %v", s.Group, s.Number, err)
					}
				}
			}
		})
	}
}

func TestKFMStandSprite(t *testing.T) {
	p := "../../ikemen/release/Ikemen_GO-v1.0.0-windows/chars/kfm/kfm.sff"
	if _, err := os.Stat(p); err != nil {
		t.Skip("KFM not found in ikemen/release")
	}
	f, err := Open(p, Options{Character: true})
	if err != nil {
		t.Fatal(err)
	}
	if v := f.Header.Version; v[0] != 2 {
		t.Fatalf("kfm.sff version %v, want 2.x", v)
	}
	s := f.Sprite(0, 0)
	if s == nil || !s.HasImage() {
		t.Fatal("sprite 0,0 missing or empty")
	}
	if f.Sprite(9000, 0) == nil {
		t.Error("portrait sprite 9000,0 missing")
	}
}

func TestDecodersOnTinyInput(t *testing.T) {
	// RLE8: 0x43 = run of 3, next byte is the value; then a literal 7.
	got := rle8Decode([]byte{0x43, 5, 7}, 4, 1)
	if !bytes.Equal(got, []byte{5, 5, 5, 7}) {
		t.Errorf("rle8 = %v", got)
	}
	// Truncated / garbage input must not panic through decodeSafe.
	for _, dec := range []func([]byte, int, int) []byte{rle8Decode, rle5Decode, lz5Decode} {
		if _, err := decodeSafe("x", func() []byte { return dec([]byte{0xff, 0xff}, 64, 64) }); err != nil {
			t.Log("handled:", err)
		}
	}
}

func FuzzLoad(f *testing.F) {
	for _, p := range []string{
		"../../ikemen/release/Ikemen_GO-v1.0.0-windows/chars/kfm/kfm.sff",
		"../../ikemen/release/Ikemen_GO-v1.0.0-windows/chars/kfm/intro.sff",
	} {
		if b, err := os.ReadFile(p); err == nil && len(b) < 1<<20 {
			f.Add(b)
		}
	}
	f.Add([]byte("ElecbyteSpr\x00\x00\x01\x00\x02"))
	f.Fuzz(func(t *testing.T, data []byte) {
		Load(bytes.NewReader(data), Options{Character: true}) // must not panic
	})
}
