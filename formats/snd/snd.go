// Package snd reads Elecbyte SND sound files the way Ikemen GO does.
//
// Upstream: ikemen/source/src/sound.go @ v1.0.0 (81c6da71d689625e20db79586815b695da00dd6d)
//
//	LoadSnd / LoadSndFiltered (sound.go:793, 810) -> Load / Open
//	readSound                 (sound.go:706)      -> readSound (size check only)
//
// Changes from upstream:
//   - The WAV data of each sound is kept as bytes and its header is parsed
//     (WAV) instead of being decoded with beep. Upstream decodes every sound
//     to the end and disables the ones that fail; that check is not ported, so
//     a sound with a valid header may still be rejected by Ikemen.
//   - LogMessage warnings become Diagnostics. A subheader that cannot be read
//     makes Ikemen reject the whole file; here it is an error diagnostic and
//     the sounds read so far are kept.
//   - Sounds with a negative group or number are skipped like in LoadSnd, and
//     reported.
//   - Engine globals (sys, file cache, sound channels) are removed.
//
// MIT License
// Copyright (c) 2016 Suehiro
// Copyright (c) 2018 K4thos, NeatUnsou, ShinLucho
// Copyright (c) 2019 Daniel Porto, Gacel, Nikolay Griva
// See ikemen/source/LICENCE.txt for the full notice.
package snd

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/whoisnighter/KataFighter/formats/diag"
)

// Magic is the 12-byte signature at the start of every SND file.
const Magic = "ElecbyteSnd\x00"

// Sound is one sound of the file.
type Sound struct {
	Index  int // position in the file
	Group  int32
	Number int32
	Offset int64  // file offset of the WAV data
	Data   []byte // the WAV file
	WAV    *WAV   // parsed WAV header, nil when it is not a readable WAV
}

// File is a loaded SND.
type File struct {
	Version  [2]uint16 // ver, ver2 as stored in the header
	Declared uint32    // number of sounds the header declares
	Sounds   []*Sound  // sounds Ikemen keeps, in file order
	// Diagnostics have no line positions (SND is binary).
	Diagnostics []diag.Diagnostic

	byKey map[[2]int32]*Sound
}

// Sound returns the sound for group,number, or nil.
func (f *File) Sound(group, number int32) *Sound { return f.byKey[[2]int32{group, number}] }

// Keys returns every group,number in ascending order.
func (f *File) Keys() [][2]int32 {
	ks := make([][2]int32, 0, len(f.byKey))
	for k := range f.byKey {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool { return ks[i][0] < ks[j][0] || ks[i][0] == ks[j][0] && ks[i][1] < ks[j][1] })
	return ks
}

// Open loads an SND file from disk.
func Open(path string) (*File, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	f, err := Load(fh)
	if f != nil {
		diag.SetFile(f.Diagnostics, path)
	}
	return f, err
}

func (f *File) report(sev diag.Severity, format string, a ...any) {
	f.Diagnostics = append(f.Diagnostics, diag.Diagnostic{Severity: sev, Message: fmt.Sprintf(format, a...)})
}

// Load reads a whole SND file. Header errors are returned as an error; per-sound
// problems are recorded in Diagnostics.
func Load(r io.ReadSeeker) (*File, error) {
	magic := make([]byte, len(Magic))
	if _, err := io.ReadFull(r, magic); err != nil || string(magic) != Magic {
		return nil, errors.New("unrecognized SND file, invalid header")
	}
	f := &File{byKey: map[[2]int32]*Sound{}}
	var subHeader uint32
	for _, x := range []any{&f.Version[0], &f.Version[1], &f.Declared, &subHeader} {
		if err := binary.Read(r, binary.LittleEndian, x); err != nil {
			return nil, fmt.Errorf("SND header: %w", err)
		}
	}
	size, _ := r.Seek(0, io.SeekEnd)

	seen := map[uint32]bool{}
	for i := uint32(0); i < f.Declared; i++ {
		// A chain that comes back to a subheader would repeat the same
		// sounds up to Declared times (Ikemen logs a duplicate each time).
		if seen[subHeader] {
			f.report(diag.Warning, "sound %d: subheader offset %d was already read; the remaining %d declared sounds are repeats", i, subHeader, f.Declared-i)
			break
		}
		seen[subHeader] = true
		if _, err := r.Seek(int64(subHeader), io.SeekStart); err != nil {
			f.report(diag.Error, "sound %d: bad subheader offset %d; Ikemen rejects the file", i, subHeader)
			break
		}
		var h struct {
			Next, Length  uint32
			Group, Number int32
		}
		if err := binary.Read(r, binary.LittleEndian, &h); err != nil {
			f.report(diag.Error, "sound %d: cannot read subheader at offset %d (header declares %d sounds); Ikemen rejects the file", i, subHeader, f.Declared)
			break
		}
		key := [2]int32{h.Group, h.Number}
		switch {
		case h.Group < 0 || h.Number < 0:
			f.report(diag.Warning, "sound %d: %d,%d has a negative group or number and is ignored", i, h.Group, h.Number)
		case f.byKey[key] != nil:
			f.report(diag.Warning, "duplicate sound key %d,%d (index %d ignored)", h.Group, h.Number, i)
		default:
			f.readSound(r, int(i), key, subHeader+16, h.Length, size)
		}
		subHeader = h.Next
	}
	return f, nil
}

func (f *File) readSound(r io.Reader, index int, key [2]int32, offset, length uint32, fileSize int64) {
	// readSound: "wav size is too small"
	if length < 128 {
		f.report(diag.Warning, "sound %d,%d can't be read: wav size is too small (%d bytes)", key[0], key[1], length)
		return
	}
	if int64(offset)+int64(length) > fileSize {
		f.report(diag.Warning, "sound %d,%d can't be read: %d bytes declared but the file ends after %d", key[0], key[1], length, fileSize-int64(offset))
		return
	}
	s := &Sound{Index: index, Group: key[0], Number: key[1], Offset: int64(offset), Data: make([]byte, length)}
	if _, err := io.ReadFull(r, s.Data); err != nil {
		f.report(diag.Warning, "sound %d,%d can't be read: %v", key[0], key[1], err)
		return
	}
	w, err := ParseWAV(s.Data)
	if err != nil {
		f.report(diag.Warning, "sound %d,%d can't be read: %v", key[0], key[1], err)
		return
	}
	s.WAV = w
	f.Sounds = append(f.Sounds, s)
	f.byKey[key] = s
}
