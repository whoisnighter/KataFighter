package sff

import (
	"bytes"
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The golden test pins what Load returns for every .sff of the release corpus:
// one line per sprite and palette, with a hash of the decoded pixels. The
// pixels themselves are not committed (the corpus is not ours to ship).
//
// After an intended change, regenerate with:
//
//	go test ./sff -run Golden -update
var update = flag.Bool("update", false, "rewrite golden files in testdata/golden")

const goldenDir = "../testdata/golden/sff"

func hash(b []byte) string {
	if len(b) == 0 {
		return "-"
	}
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:8])
}

// manifest renders a loaded file as stable text.
func manifest(f *File) []byte {
	var b bytes.Buffer
	h := f.Header
	fmt.Fprintf(&b, "version %v sprites %d palettes %d\n", h.Version, len(f.Sprites), len(f.Palettes))
	fmt.Fprintln(&b, "# palette: index group,number colors link hash")
	for i, p := range f.Palettes {
		cols := make([]byte, 0, len(p.Colors)*4)
		for _, c := range p.Colors {
			cols = append(cols, byte(c), byte(c>>8), byte(c>>16), byte(c>>24))
		}
		fmt.Fprintf(&b, "pal %d %d,%d %d %d %s\n", i, p.Group, p.Number, len(p.Colors), p.LinkedTo, hash(cols))
	}
	fmt.Fprintln(&b, "# sprite: index group,number WxH axis format depth pal link pixels")
	for _, s := range f.Sprites {
		px := s.Pix
		if len(px) == 0 {
			px = s.RGBA
		}
		fmt.Fprintf(&b, "spr %d %d,%d %dx%d %d,%d %v %d %d %d %s\n", s.Index, s.Group, s.Number,
			s.Size[0], s.Size[1], s.Offset[0], s.Offset[1], s.Format, s.ColorDepth, s.PalIndex, s.LinkedTo, hash(px))
	}
	for _, d := range f.Diagnostics {
		fmt.Fprintf(&b, "%v %s\n", d.Severity, d.Message)
	}
	return b.Bytes()
}

// goldenName maps ".../Ikemen_GO-v1.0.0-windows/chars/kfm/kfm.sff" to "chars/kfm/kfm.sff.txt".
func goldenName(p string) string {
	p = filepath.ToSlash(p)
	if i := strings.Index(p, "/release/"); i >= 0 {
		p = p[i+len("/release/"):]
		if j := strings.Index(p, "/"); j >= 0 {
			p = p[j+1:] // drop the build folder name
		}
	}
	return p + ".txt"
}

func TestGolden(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range corpus(t) {
		name := goldenName(p)
		seen[name] = true
		t.Run(name, func(t *testing.T) {
			f, err := Open(p, Options{Character: isCharSff(p)})
			if err != nil {
				t.Fatal(err)
			}
			got := manifest(f)
			gp := filepath.Join(goldenDir, filepath.FromSlash(name))
			if *update {
				os.MkdirAll(filepath.Dir(gp), 0o755)
				if err := os.WriteFile(gp, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("no golden file (run with -update): %v", err)
			}
			want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))
			if !bytes.Equal(got, want) {
				gl, wl := strings.Split(string(got), "\n"), strings.Split(string(want), "\n")
				for i := 0; i < max(len(gl), len(wl)); i++ {
					var g, w string
					if i < len(gl) {
						g = gl[i]
					}
					if i < len(wl) {
						w = wl[i]
					}
					if g != w {
						t.Fatalf("line %d differs\n got: %s\nwant: %s", i+1, g, w)
					}
				}
			}
		})
	}
	if !*update {
		filepath.WalkDir(goldenDir, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				rel, _ := filepath.Rel(goldenDir, p)
				if !seen[filepath.ToSlash(rel)] {
					t.Errorf("golden file %s has no matching .sff in the corpus", rel)
				}
			}
			return nil
		})
	}
}
