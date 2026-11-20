package snd

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/whoisnighter/KataFighter/formats/diag"
)

// wav builds a PCM WAV with n bytes of silence.
func wav(rate uint32, ch, bits uint16, n int) []byte {
	var b bytes.Buffer
	w := func(x any) { binary.Write(&b, binary.LittleEndian, x) }
	b.WriteString("RIFF")
	w(uint32(36 + n))
	b.WriteString("WAVEfmt ")
	w(uint32(16))
	w(uint16(1))
	w(ch)
	w(rate)
	w(rate * uint32(ch) * uint32(bits) / 8)
	w(ch * bits / 8)
	w(bits)
	b.WriteString("data")
	w(uint32(n))
	b.Write(make([]byte, n))
	return b.Bytes()
}

type entry struct {
	group, number int32
	data          []byte
}

// build writes an SND with the given sounds, chained in order.
func build(declared int, es ...entry) []byte {
	var b bytes.Buffer
	w := func(x any) { binary.Write(&b, binary.LittleEndian, x) }
	b.WriteString(Magic)
	w(uint16(0))
	w(uint16(1))
	w(uint32(declared))
	w(uint32(512))
	b.Write(make([]byte, 512-b.Len()))
	for _, e := range es {
		next := uint32(b.Len() + 16 + len(e.data))
		w(next)
		w(uint32(len(e.data)))
		w(e.group)
		w(e.number)
		b.Write(e.data)
	}
	return b.Bytes()
}

func messages(ds []diag.Diagnostic) string {
	var s []string
	for _, d := range ds {
		s = append(s, d.Severity.String()+": "+d.Message)
	}
	return strings.Join(s, "\n")
}

func TestLoad(t *testing.T) {
	a := wav(22050, 1, 16, 22050) // 0.5 s
	f, err := Load(bytes.NewReader(build(5,
		entry{0, 0, a},
		entry{0, 1, wav(11025, 2, 8, 1000)},
		entry{0, 0, a},                 // duplicate
		entry{-1, 3, a},                // negative group
		entry{1, 0, make([]byte, 100)}, // too small
	)))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Sounds) != 2 || f.Sound(0, 1) == nil || f.Sound(0, 0).Index != 0 {
		t.Fatalf("sounds: %+v", f.Sounds)
	}
	if d := f.Sound(0, 0).WAV.Duration(); d != 500*time.Millisecond {
		t.Errorf("duration %v", d)
	}
	if got := f.Sound(0, 1).WAV.String(); got != "11025 Hz 8-bit stereo, 0.05s" {
		t.Errorf("wav %q", got)
	}
	got := messages(f.Diagnostics)
	for _, want := range []string{"duplicate sound key 0,0 (index 2 ignored)", "-1,3 has a negative group", "1,0 can't be read: wav size is too small"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
}

func TestLoadBroken(t *testing.T) {
	if _, err := Load(bytes.NewReader([]byte("NotASndFile!xxxx"))); err == nil {
		t.Error("bad magic: no error")
	}
	// More sounds declared than present: an error diagnostic, sounds kept.
	f, err := Load(bytes.NewReader(build(3, entry{2, 0, wav(8000, 1, 8, 200)})))
	if err != nil || len(f.Sounds) != 1 || !diag.HasErrors(f.Diagnostics) {
		t.Fatalf("%v %d sounds\n%s", err, len(f.Sounds), messages(f.Diagnostics))
	}
	// A subheader chain that loops back.
	b := build(1000, entry{2, 0, wav(8000, 1, 8, 200)})
	binary.LittleEndian.PutUint32(b[512:], 512)
	f, err = Load(bytes.NewReader(b))
	if err != nil || len(f.Sounds) != 1 || len(f.Diagnostics) != 1 {
		t.Fatalf("%v %d sounds\n%s", err, len(f.Sounds), messages(f.Diagnostics))
	}
}

func TestCorpus(t *testing.T) {
	var files []string
	filepath.WalkDir("../../ikemen/release", func(p string, d os.DirEntry, err error) error {
		if err == nil && strings.EqualFold(filepath.Ext(p), ".snd") {
			files = append(files, p)
		}
		return nil
	})
	if len(files) == 0 {
		t.Skip("no Ikemen GO release in ikemen/release")
	}
	for _, p := range files {
		f, err := Open(p)
		if err != nil {
			t.Errorf("%s: %v", p, err)
			continue
		}
		if len(f.Sounds) == 0 {
			t.Errorf("%s: no sounds", p)
		}
		for _, d := range f.Diagnostics {
			t.Errorf("%s: %s", p, d)
		}
	}
}

func FuzzLoad(f *testing.F) {
	f.Add(build(2, entry{0, 0, wav(8000, 1, 8, 200)}, entry{1, 1, wav(8000, 1, 16, 400)}))
	f.Add([]byte(Magic))
	f.Fuzz(func(t *testing.T, b []byte) {
		Load(bytes.NewReader(b))
	})
}
