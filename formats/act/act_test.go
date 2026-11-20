package act

import (
	"bytes"
	"testing"
)

func TestReadFull(t *testing.T) {
	data := make([]byte, Size+10) // extra bytes are ignored
	for i := 0; i < 256; i++ {
		data[i*3], data[i*3+1], data[i*3+2] = byte(i), byte(i+1), byte(i+2)
	}
	p, err := Read(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if p.Read != 256 || p.Short() {
		t.Fatalf("read %d colors", p.Read)
	}
	// The file's first color is index 255, opaque.
	if want := uint32(0xFF<<24 | 2<<16 | 1<<8 | 0); p.Colors[255] != want {
		t.Errorf("color 255 = %08x, want %08x", p.Colors[255], want)
	}
	// The file's last color is index 0, transparent.
	if want := uint32(0<<24 | 257%256<<16 | 256%256<<8 | 255); p.Colors[0] != want {
		t.Errorf("color 0 = %08x, want %08x", p.Colors[0], want)
	}
}

func TestReadShort(t *testing.T) {
	p, err := Read(bytes.NewReader([]byte{10, 20, 30, 40, 50, 60, 70}))
	if err != nil {
		t.Fatal(err)
	}
	if p.Read != 2 || !p.Short() || len(p.Colors) != 256 {
		t.Fatalf("read %d colors, %d entries", p.Read, len(p.Colors))
	}
	if p.Colors[255] != 0xFF1E140A || p.Colors[254] != 0xFF3C3228 || p.Colors[253] != 0 {
		t.Errorf("colors %08x %08x %08x", p.Colors[255], p.Colors[254], p.Colors[253])
	}
}

func TestReadEmpty(t *testing.T) {
	p, err := Read(bytes.NewReader(nil))
	if err != nil || p.Read != 0 || len(p.Colors) != 256 {
		t.Fatalf("%v %+v", err, p)
	}
}
