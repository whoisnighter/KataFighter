// Package act reads ACT palette files (Photoshop color tables) the way
// Ikemen GO does for a character's pal1..palN.
//
// Upstream: ikemen/source/src/image.go @ v1.0.0 (81c6da71d689625e20db79586815b695da00dd6d)
//
//	readActPalette (image.go:613) -> Read / Load
//
// Changes from upstream:
//   - Reads from an io.Reader; Load opens the file (upstream uses OpenFile).
//   - A file shorter than 768 bytes is still accepted, as upstream does, and
//     is reported in Short so tools can warn about it.
//
// MIT License
// Copyright (c) 2016 Suehiro
// Copyright (c) 2018 K4thos, NeatUnsou, ShinLucho
// Copyright (c) 2019 Daniel Porto, Gacel, Nikolay Griva
// See ikemen/source/LICENCE.txt for the full notice.
package act

import (
	"errors"
	"io"
	"os"
)

// Size is the size of a full ACT file: 256 colors × RGB.
const Size = 768

// Palette is a decoded ACT file.
type Palette struct {
	// Colors are packed like SFF palettes (A<<24 | B<<16 | G<<8 | R), so
	// they can replace one (see sff.File.Image). ACT stores colors in
	// reverse order: the file's first color is index 255. Index 0 is
	// transparent, the rest opaque.
	Colors []uint32
	// Read is the number of colors found in the file (256 for a full file).
	// Missing colors are left at 0 (transparent black), as in Ikemen.
	Read int
}

// Short reports whether the file had fewer than 256 colors.
func (p *Palette) Short() bool { return p.Read < 256 }

// Read decodes an ACT palette. Bytes after the first 768 are ignored.
func Read(r io.Reader) (*Palette, error) {
	data := make([]byte, Size)
	n, err := io.ReadFull(r, data)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, err
	}
	p := &Palette{Colors: make([]uint32, 256), Read: n / 3}
	for i := 0; i < p.Read; i++ {
		r, g, b := data[i*3], data[i*3+1], data[i*3+2]
		dst := 255 - i
		var alpha uint32 = 255
		if dst == 0 {
			alpha = 0 // index 0 is transparent
		}
		p.Colors[dst] = alpha<<24 | uint32(b)<<16 | uint32(g)<<8 | uint32(r)
	}
	return p, nil
}

// Load reads an ACT file from disk.
func Load(path string) (*Palette, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Read(f)
}
