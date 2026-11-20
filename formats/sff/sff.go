// Package sff reads Elecbyte SFF sprite files (v1 and v2) the way Ikemen GO does.
//
// Upstream: ikemen/source/src/image.go @ v1.0.0 (81c6da71d689625e20db79586815b695da00dd6d)
//
//	SffHeader.Read      (image.go:1482)  -> readHeader
//	loadSff             (image.go:1618)  -> Load
//	Sprite.readHeader   (image.go:897)   -> readSubheaderV1
//	Sprite.readPcxHeader(image.go:922)   -> readPcxHeader
//	Sprite.read         (image.go:998)   -> readV1
//	Sprite.readHeaderV2 (image.go:1112)  -> readSubheaderV2
//	Sprite.readV2       (image.go:1311)  -> readV2
//	Sprite.shareCopy    (image.go:811)   -> linked sprites in Load
//	Sff.loadPalettes    (image.go:2069)  -> loadPalettes
//	Sff.ReadPalette     (image.go:2145)  -> readPaletteV2
//
// Changes from upstream:
//   - Decoded pixels are kept in memory (Sprite.Pix / Sprite.RGBA) instead of
//     being uploaded to GPU textures.
//   - Engine globals (sys, loading cancel, file cache) are removed.
//   - Errors while decoding a single sprite are recorded in File.Diagnostics and
//     loading continues (upstream aborts the whole file, or panics on a short
//     read in readV2). Header errors are still fatal.
//   - The isActPal option of loadSff (runtime ACT palette override) is not ported.
//
// MIT License
// Copyright (c) 2016 Suehiro
// Copyright (c) 2018 K4thos, NeatUnsou, ShinLucho
// Copyright (c) 2019 Daniel Porto, Gacel, Nikolay Griva
// See ikemen/source/LICENCE.txt for the full notice.
package sff

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"os"

	"github.com/whoisnighter/KataFighter/formats/diag"
)

// Version holds the SFF version as {hi, lo1, lo2, lo3}, e.g. {2,0,1,0} = 2.0.1.0.
type Version [4]byte

func (v Version) String() string { return fmt.Sprintf("%d.%d.%d.%d", v[0], v[1], v[2], v[3]) }

// Format is the sprite data format stored in the file.
type Format byte

const (
	FormatRaw   Format = 0    // v2: uncompressed
	FormatRLE8  Format = 2    // v2
	FormatRLE5  Format = 3    // v2
	FormatLZ5   Format = 4    // v2
	FormatPNG8  Format = 10   // v2: paletted PNG
	FormatPNG24 Format = 11   // v2
	FormatPNG32 Format = 12   // v2
	FormatPCX   Format = 0xF0 // v1: PCX (not a value stored in the file)
)

func (f Format) String() string {
	switch f {
	case FormatRaw:
		return "raw"
	case FormatRLE8:
		return "rle8"
	case FormatRLE5:
		return "rle5"
	case FormatLZ5:
		return "lz5"
	case FormatPNG8:
		return "png8"
	case FormatPNG24:
		return "png24"
	case FormatPNG32:
		return "png32"
	case FormatPCX:
		return "pcx"
	}
	return fmt.Sprintf("unknown(%d)", byte(f))
}

// Header is the parsed file header.
type Header struct {
	Version                  Version
	FirstSpriteHeaderOffset  uint32
	FirstPaletteHeaderOffset uint32
	NumberOfSprites          uint32
	NumberOfPalettes         uint32
	LiteralDataOffset        uint32 // v2 only ("lofs")
	TranslatedDataOffset     uint32 // v2 only ("tofs")
}

// Sprite is one sprite in file order.
type Sprite struct {
	Index      int       // position in the file
	Group      uint16    // valid range 0–65535
	Number     uint16    // valid range 0–65535
	Size       [2]uint16 // width, height
	Offset     [2]int16  // axis x, y
	Format     Format
	ColorDepth byte // v2 header value; 0 for v1 (always 8-bit indexed)
	PalIndex   int  // index into File.Palettes for indexed sprites; -1 if none
	LinkedTo   int  // index of the sprite whose image this one shares; -1 if none

	// Indexed pixels (len = width*height), for sprites with ColorDepth <= 8.
	Pix []byte
	// True-color pixels for ColorDepth 24/32, laid out as RGBADepth/8 bytes per pixel.
	RGBA      []byte
	RGBADepth int // 24 or 32
	// Premultiplied is true when RGBA holds premultiplied alpha (PNG24/PNG32,
	// which Ikemen converts through image.RGBA).
	Premultiplied bool
}

// HasImage reports whether pixel data was decoded for this sprite.
func (s *Sprite) HasImage() bool { return len(s.Pix) > 0 || len(s.RGBA) > 0 }

// Palette is one palette. Colors are packed as A<<24 | B<<16 | G<<8 | R.
type Palette struct {
	Group, Number uint16 // v2 palette key; zero for v1 palettes
	NumColors     uint16 // as declared in the v2 palette header
	LinkedTo      int    // index of the palette whose data this one reuses; -1 if none
	Colors        []uint32
}

// File is a loaded SFF.
type File struct {
	Header   Header
	Sprites  []*Sprite
	Palettes []*Palette
	// Diagnostics are per-sprite problems; loading continues past them.
	// They have no line positions (SFF is binary).
	Diagnostics []diag.Diagnostic

	byKey map[[2]uint16]*Sprite
}

// Sprite returns the sprite for group/number, or nil. Like Ikemen, the first
// sprite wins when a key is duplicated.
func (f *File) Sprite(group, number uint16) *Sprite {
	if group == 0xFFFF {
		return nil
	}
	return f.byKey[[2]uint16{group, number}]
}

// Palette returns the palette used by an indexed sprite, mirroring
// PaletteList.Get: an out-of-range index falls back to palette 0.
func (f *File) Palette(s *Sprite) *Palette {
	if len(f.Palettes) == 0 {
		return nil
	}
	i := s.PalIndex
	if i < 0 || i >= len(f.Palettes) {
		i = 0
	}
	return f.Palettes[i]
}

// Options control how a file is interpreted.
type Options struct {
	// Character must be true for a character's main SFF. It enables Ikemen's
	// legacy SFF v1 palette handling for the first sprite and sprite 0,0.
	Character bool
}

// Open loads an SFF file from disk.
func Open(path string, opt Options) (*File, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	f, err := Load(fh, opt)
	if f != nil {
		diag.SetFile(f.Diagnostics, path)
	}
	return f, err
}

func (f *File) warnf(format string, a ...any) {
	f.Diagnostics = append(f.Diagnostics, diag.Diagnostic{Severity: diag.Warning, Message: fmt.Sprintf(format, a...)})
}

// Load reads a whole SFF file.
func Load(r io.ReadSeeker, opt Options) (*File, error) {
	f := &File{byKey: make(map[[2]uint16]*Sprite)}
	if err := readHeader(r, &f.Header); err != nil {
		return nil, err
	}
	h := &f.Header
	if err := f.loadPalettes(r); err != nil {
		return nil, err
	}

	f.Sprites = make([]*Sprite, 0, min(int(h.NumberOfSprites), 1<<16))
	var prev *Sprite
	shofs := int64(h.FirstSpriteHeaderOffset)
	for i := 0; i < int(h.NumberOfSprites); i++ {
		if _, err := r.Seek(shofs, io.SeekStart); err != nil {
			return f, fmt.Errorf("sprite %d: %w", i, err)
		}
		s := &Sprite{Index: i, PalIndex: -1, LinkedTo: -1}
		var xofs, size uint32
		var link uint16
		var err error
		if h.Version[0] == 1 {
			err = s.readSubheaderV1(r, &xofs, &size, &link)
		} else {
			err = s.readSubheaderV2(r, &xofs, &size, h.LiteralDataOffset, h.TranslatedDataOffset, &link)
		}
		if err != nil {
			// The sprite table is truncated: keep what we have.
			f.warnf("sprite %d: truncated sprite header: %v", i, err)
			break
		}
		f.Sprites = append(f.Sprites, s)

		if size == 0 {
			if int(link) < i {
				src := f.Sprites[link]
				s.LinkedTo = int(link)
				// shareCopy
				s.Pix, s.RGBA, s.RGBADepth, s.Premultiplied = src.Pix, src.RGBA, src.RGBADepth, src.Premultiplied
				s.Size = src.Size
				if s.PalIndex < 0 {
					s.PalIndex = src.PalIndex
				}
				s.ColorDepth = src.ColorDepth
				if h.Version[0] == 1 {
					s.Format = FormatPCX
				}
			} else {
				s.PalIndex = 0 // index out of range
				f.warnf("sprite %d (%d,%d): invalid link to sprite %d", i, s.Group, s.Number, link)
			}
		} else {
			if h.Version[0] == 1 {
				isCharFirst := opt.Character && (prev == nil || s.Group == 0 && s.Number == 0)
				err = f.readV1(r, s, shofs+32, size, xofs, prev, isCharFirst)
			} else {
				err = f.readV2(r, s, int64(xofs), size)
			}
			if err != nil {
				f.warnf("sprite %d (%d,%d): %v", i, s.Group, s.Number, err)
			}
			prev = s
		}

		key := [2]uint16{s.Group, s.Number}
		if f.byKey[key] != nil {
			f.warnf("duplicate sprite key %d,%d (index %d ignored)", s.Group, s.Number, i)
		} else {
			f.byKey[key] = s
		}

		if h.Version[0] == 1 {
			shofs = int64(xofs)
		} else {
			shofs += 28
		}
	}
	return f, nil
}

func readHeader(r io.Reader, sh *Header) error {
	sig := make([]byte, 12)
	if _, err := io.ReadFull(r, sig); err != nil {
		return fmt.Errorf("sff header: %w", err)
	}
	if string(sig) != "ElecbyteSpr\x00" {
		return errors.New("unrecognized SFF file, invalid header")
	}
	read := func(x any) error { return binary.Read(r, binary.LittleEndian, x) }
	// On disk: verlo3, verlo2, verlo1, verhi
	for _, i := range []int{3, 2, 1, 0} {
		if err := read(&sh.Version[i]); err != nil {
			return err
		}
	}
	var dummy uint32
	if err := read(&dummy); err != nil {
		return err
	}
	switch sh.Version[0] {
	case 1:
		for _, x := range []any{&sh.NumberOfSprites, &sh.FirstSpriteHeaderOffset, &dummy} {
			if err := read(x); err != nil {
				return err
			}
		}
	case 2:
		for i := 0; i < 4; i++ {
			if err := read(&dummy); err != nil {
				return err
			}
		}
		for _, x := range []any{&sh.FirstSpriteHeaderOffset, &sh.NumberOfSprites,
			&sh.FirstPaletteHeaderOffset, &sh.NumberOfPalettes,
			&sh.LiteralDataOffset, &dummy, &sh.TranslatedDataOffset} {
			if err := read(x); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unrecognized SFF version %v", sh.Version)
	}
	return nil
}

func (s *Sprite) readSubheaderV1(r io.Reader, ofs, size *uint32, link *uint16) error {
	for _, x := range []any{ofs, size, s.Offset[:], &s.Group, &s.Number, link} {
		if err := binary.Read(r, binary.LittleEndian, x); err != nil {
			return err
		}
	}
	return nil
}

func (s *Sprite) readSubheaderV2(r io.Reader, ofs, size *uint32, lofs, tofs uint32, link *uint16) error {
	var format byte
	var palidx, flags uint16
	for _, x := range []any{&s.Group, &s.Number, s.Size[:], s.Offset[:], link,
		&format, &s.ColorDepth, ofs, size, &palidx, &flags} {
		if err := binary.Read(r, binary.LittleEndian, x); err != nil {
			return err
		}
	}
	s.Format = Format(format)
	s.PalIndex = int(palidx)
	if flags&1 == 0 {
		*ofs += lofs
	} else {
		*ofs += tofs
	}
	return nil
}

// readPcxHeader sets the sprite size and returns the RLE bytes-per-line
// (0 when the PCX is not RLE encoded).
func (s *Sprite) readPcxHeader(r io.ReadSeeker, offset int64) (bpl int, err error) {
	if _, err := r.Seek(offset, io.SeekStart); err != nil {
		return 0, err
	}
	read := func(x any) error { return binary.Read(r, binary.LittleEndian, x) }
	var dummy uint16
	var encoding, bpp byte
	var rect [4]uint16
	for _, x := range []any{&dummy, &encoding, &bpp} {
		if err := read(x); err != nil {
			return 0, err
		}
	}
	if bpp != 8 {
		return 0, fmt.Errorf("invalid PCX color depth: expected 8-bit, got %v", bpp)
	}
	if err := read(rect[:]); err != nil {
		return 0, err
	}
	if _, err := r.Seek(offset+66, io.SeekStart); err != nil {
		return 0, err
	}
	var b uint16
	if err := read(&b); err != nil {
		return 0, err
	}
	s.Size[0] = rect[2] - rect[0] + 1
	s.Size[1] = rect[3] - rect[1] + 1
	if encoding == 1 {
		return int(b), nil
	}
	return 0, nil
}

func (f *File) newPalette() int {
	f.Palettes = append(f.Palettes, &Palette{LinkedTo: -1, NumColors: 256, Colors: make([]uint32, 256)})
	return len(f.Palettes) - 1
}

func (f *File) readV1(r io.ReadSeeker, s *Sprite, offset int64, datasize uint32,
	nextSubheader uint32, prev *Sprite, isCharFirstSprite bool) error {
	s.Format = FormatPCX
	read := func(x any) error { return binary.Read(r, binary.LittleEndian, x) }
	var ps byte
	if err := read(&ps); err != nil {
		return err
	}
	paletteSame := ps != 0 && prev != nil
	bpl, err := s.readPcxHeader(r, offset)
	if err != nil {
		return err
	}
	pcxDataStart := offset + 128

	var px []byte
	var paletteOffset int64

	if isCharFirstSprite {
		// Legacy SFFv1 handling for the character's 0,0 sprite
		if int64(nextSubheader) > offset {
			datasize = nextSubheader - uint32(offset)
		}
		rleSize := max(int64(datasize)-128, 0)
		px = make([]byte, rleSize)
		r.Seek(pcxDataStart, io.SeekStart)
		if err := read(px); err != nil {
			return err
		}
		paletteOffset = offset + int64(datasize) - 768
	} else {
		// Padding-safe handling for regular SFFv1 sprites
		var blockEnd int64
		if int64(nextSubheader) > offset {
			blockEnd = int64(nextSubheader)
		} else {
			blockEnd = offset + int64(datasize)
		}
		if paletteSame {
			paletteOffset = blockEnd
		} else {
			var b [1]byte
			paletteOffset = -1
			for pos := blockEnd - 769; pos >= pcxDataStart; pos-- {
				r.Seek(pos, io.SeekStart)
				if _, err := r.Read(b[:]); err != nil {
					return err
				}
				if b[0] == 0x0C {
					paletteOffset = pos
					break
				}
			}
			if paletteOffset < 0 {
				paletteOffset = blockEnd - 769
			}
		}
		rleSize := max(paletteOffset-pcxDataStart, 0)
		px = make([]byte, rleSize)
		r.Seek(pcxDataStart, io.SeekStart)
		if err := read(px); err != nil {
			return err
		}
	}

	if paletteSame {
		s.PalIndex = prev.PalIndex
		if s.PalIndex < 0 {
			s.PalIndex = f.newPalette()
		}
	} else {
		s.PalIndex = f.newPalette()
		pal := f.Palettes[s.PalIndex].Colors
		r.Seek(paletteOffset, io.SeekStart)
		// The regular path finds the 0x0C marker, while the legacy
		// character path points directly at the palette
		if !isCharFirstSprite {
			r.Seek(1, io.SeekCurrent)
		}
		var rgb [3]byte
		for i := range pal {
			if err := read(rgb[:]); err != nil {
				return err
			}
			var alpha byte = 255
			if i == 0 {
				alpha = 0
			}
			pal[i] = uint32(alpha)<<24 | uint32(rgb[2])<<16 | uint32(rgb[1])<<8 | uint32(rgb[0])
		}
	}
	w, h := int(s.Size[0]), int(s.Size[1])
	p, err := decodeSafe("pcx", func() []byte { return rlePcxDecode(px, w, h, bpl) })
	if err != nil {
		return err
	}
	return s.setPix(p)
}

// setPix mirrors Sprite.SetPxl: data whose length does not match the sprite
// size is dropped (Ikemen creates no texture for it).
func (s *Sprite) setPix(px []byte) error {
	if len(px) == 0 {
		return nil
	}
	if len(px) != int(s.Size[0])*int(s.Size[1]) {
		return fmt.Errorf("decoded %d pixels, expected %dx%d; image dropped", len(px), s.Size[0], s.Size[1])
	}
	s.Pix = px
	return nil
}

// setRaw mirrors Sprite.SetRaw (minus the GPU upload).
func (s *Sprite) setRaw(data []byte, depth int, premultiplied bool) {
	if depth == 32 {
		// Normalize fully transparent RGBA pixels to prevent their RGB values from bleeding into visible pixels
		for i := 0; i+3 < len(data); i += 4 {
			if data[i+3] == 0 {
				data[i], data[i+1], data[i+2] = 0, 0, 0
			}
		}
	}
	s.RGBA, s.RGBADepth, s.Premultiplied = data, depth, premultiplied
}

func (f *File) readV2(r io.ReadSeeker, s *Sprite, offset int64, datasize uint32) error {
	w, h := int(s.Size[0]), int(s.Size[1])
	if s.Format == FormatRaw {
		r.Seek(offset, io.SeekStart)
		px := make([]byte, datasize)
		if _, err := io.ReadFull(r, px); err != nil {
			return fmt.Errorf("raw: %w", err)
		}
		switch s.ColorDepth {
		case 8:
			return s.setPix(px)
		case 24, 32:
			s.setRaw(px, int(s.ColorDepth), false)
			return nil
		default:
			return fmt.Errorf("unknown color depth %d", s.ColorDepth)
		}
	}

	r.Seek(offset+4, io.SeekStart) // skip the 4-byte uncompressed length
	var px []byte
	if FormatRLE8 <= s.Format && s.Format <= FormatLZ5 {
		if datasize < 4 {
			datasize = 4
		}
		px = make([]byte, datasize-4)
		if _, err := io.ReadFull(r, px); err != nil {
			// upstream panics here
			return fmt.Errorf("%v: %w", s.Format, err)
		}
	}

	var err error
	switch s.Format {
	case FormatRLE8:
		px, err = decodeSafe("rle8", func() []byte { return rle8Decode(px, w, h) })
	case FormatRLE5:
		px, err = decodeSafe("rle5", func() []byte { return rle5Decode(px, w, h) })
	case FormatLZ5:
		px, err = decodeSafe("lz5", func() []byte { return lz5Decode(px, w, h) })
	case FormatPNG8:
		img, err := png.Decode(r)
		if err != nil {
			return fmt.Errorf("png8: %w", err)
		}
		pi, ok := img.(*image.Paletted)
		if !ok {
			return fmt.Errorf("png8: image is %T, not paletted; image dropped", img)
		}
		px = pi.Pix
	case FormatPNG24, FormatPNG32:
		img, err := png.Decode(r)
		if err != nil {
			return fmt.Errorf("%v: %w", s.Format, err)
		}
		rect := img.Bounds()
		rgba, ok := img.(*image.RGBA)
		if !ok {
			rgba = image.NewRGBA(rect)
			draw.Draw(rgba, rect, img, rect.Min, draw.Src)
		}
		if rect.Dx() != w || rect.Dy() != h {
			f.warnf("sprite %d (%d,%d): PNG is %dx%d but header says %dx%d; using PNG size",
				s.Index, s.Group, s.Number, rect.Dx(), rect.Dy(), w, h)
			s.Size = [2]uint16{uint16(rect.Dx()), uint16(rect.Dy())}
		}
		s.setRaw(rgba.Pix, 32, true)
		return nil
	default:
		return fmt.Errorf("unknown format %v", s.Format)
	}
	if err != nil {
		return err
	}
	return s.setPix(px)
}

func (f *File) loadPalettes(r io.ReadSeeker) error {
	h := &f.Header
	if h.Version[0] == 1 {
		return nil
	}
	read := func(x any) error { return binary.Read(r, binary.LittleEndian, x) }
	unique := make(map[[2]uint16]int)
	for i := 0; i < int(h.NumberOfPalettes); i++ {
		r.Seek(int64(h.FirstPaletteHeaderOffset)+int64(i*16), io.SeekStart)
		var gn [3]uint16
		var link uint16
		var ofs, plSize uint32
		for _, x := range []any{gn[:], &link, &ofs, &plSize} {
			if err := read(x); err != nil {
				return fmt.Errorf("palette %d header: %w", i, err)
			}
		}
		p := &Palette{Group: gn[0], Number: gn[1], NumColors: gn[2], LinkedTo: -1}
		key := [2]uint16{gn[0], gn[1]}
		if old, ok := unique[key]; ok {
			// Duplicate key: reuse the first palette with this key
			p.LinkedTo = old
			p.Colors = f.Palettes[old].Colors
			f.warnf("duplicate palette key %d,%d (%d/%d)", gn[0], gn[1], i+1, h.NumberOfPalettes)
		} else if plSize == 0 {
			// Linked palette
			p.LinkedTo = int(link)
			if int(link) < len(f.Palettes) {
				p.Colors = f.Palettes[link].Colors
			} else {
				f.warnf("palette %d (%d,%d): invalid link to palette %d", i, gn[0], gn[1], link)
			}
			unique[key] = int(link)
		} else {
			pal, err := f.readPaletteV2(r, int64(h.LiteralDataOffset+ofs), plSize)
			if err != nil {
				return fmt.Errorf("palette %d: %w", i, err)
			}
			p.Colors = pal
			unique[key] = i
		}
		f.Palettes = append(f.Palettes, p)
	}
	return nil
}

func (f *File) readPaletteV2(r io.ReadSeeker, offset int64, size uint32) ([]uint32, error) {
	if _, err := r.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	rawCount := int(size) / 4
	depth := 1
	for depth < rawCount {
		depth *= 2
	}
	depth = min(max(depth, 16), 256)
	pal := make([]uint32, depth)
	for i := range pal {
		var rgba [4]byte
		if i < rawCount {
			if err := binary.Read(r, binary.LittleEndian, rgba[:]); err != nil {
				return nil, err
			}
		}
		if f.Header.Version[2] == 0 { // 2.0.0.0: index 0 transparent, others opaque
			if i == 0 {
				rgba[3] = 0
			} else {
				rgba[3] = 255
			}
		}
		pal[i] = uint32(rgba[3])<<24 | uint32(rgba[2])<<16 | uint32(rgba[1])<<8 | uint32(rgba[0])
	}
	return pal, nil
}
