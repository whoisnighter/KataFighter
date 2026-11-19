// Sprite decoders extracted from Ikemen GO.
//
// Upstream: ikemen/source/src/image.go @ v1.0.0 (81c6da71d689625e20db79586815b695da00dd6d)
//   RlePcxDecode  (image.go:964)
//   Rle8Decode    (image.go:1162)
//   Rle5Decode    (image.go:1190)
//   Lz5Decode     (image.go:1234)
//
// Changes from upstream: methods on *Sprite turned into functions that take
// the sprite size; the logic itself is unchanged. Callers go through
// decodeSafe, which turns out-of-range panics on malformed data into errors.
//
// MIT License
// Copyright (c) 2016 Suehiro
// Copyright (c) 2018 K4thos, NeatUnsou, ShinLucho
// Copyright (c) 2019 Daniel Porto, Gacel, Nikolay Griva
// See ikemen/source/LICENCE.txt for the full notice.

package sff

import "fmt"

// decodeSafe runs a decoder and converts a panic (index out of range on
// corrupt input) into an error, so one bad sprite never crashes the editor.
func decodeSafe(name string, f func() []byte) (p []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			p, err = nil, fmt.Errorf("%s: corrupt data: %v", name, r)
		}
	}()
	return f(), nil
}

// rlePcxDecode decodes PCX RLE data. bpl is the PCX bytes-per-line value;
// bpl <= 0 means the data is not compressed and is returned as is.
func rlePcxDecode(rle []byte, w, h, bpl int) (p []byte) {
	if len(rle) == 0 || bpl <= 0 {
		return rle
	}
	p = make([]byte, w*h)
	i, j, k := 0, 0, 0
	for j < len(p) {
		n, d := 1, rle[i]
		if i < len(rle)-1 {
			i++
		}
		if d >= 0xc0 {
			n = int(d & 0x3f)
			d = rle[i]
			if i < len(rle)-1 {
				i++
			}
		}
		for ; n > 0; n-- {
			if k < w && j < len(p) {
				p[j] = d
				j++
			}
			k++
			if k == bpl {
				k = 0
				n = 1
			}
		}
	}
	return
}

func rle8Decode(rle []byte, w, h int) (p []byte) {
	if len(rle) == 0 {
		return rle
	}
	p = make([]byte, w*h)
	i, j := 0, 0
	for j < len(p) {
		n, d := 1, rle[i]
		if i < len(rle)-1 {
			i++
		}
		if d&0xc0 == 0x40 {
			n = int(d & 0x3f)
			d = rle[i]
			if i < len(rle)-1 {
				i++
			}
		}
		for ; n > 0; n-- {
			if j < len(p) {
				p[j] = d
				j++
			}
		}
	}
	return
}

func rle5Decode(rle []byte, w, h int) (p []byte) {
	if len(rle) == 0 {
		return rle
	}
	p = make([]byte, w*h)
	i, j := 0, 0
	for j < len(p) {
		rl := int(rle[i])
		if i < len(rle)-1 {
			i++
		}
		dl := int(rle[i] & 0x7f)
		c := byte(0)
		if rle[i]>>7 != 0 {
			if i < len(rle)-1 {
				i++
			}
			c = rle[i]
		}
		if i < len(rle)-1 {
			i++
		}
		for {
			if j < len(p) {
				p[j] = c
				j++
			}
			rl--
			if rl < 0 {
				dl--
				if dl < 0 {
					break
				}
				c = rle[i] & 0x1f
				rl = int(rle[i] >> 5)
				if i < len(rle)-1 {
					i++
				}
			}
		}
	}
	return
}

func lz5Decode(rle []byte, w, h int) (p []byte) {
	if len(rle) == 0 {
		return rle
	}
	p = make([]byte, w*h)
	i, j, n := 0, 0, 0
	ct, cts, rb, rbc := rle[i], uint(0), byte(0), uint(0)
	if i < len(rle)-1 {
		i++
	}
	for j < len(p) {
		d := int(rle[i])
		if i < len(rle)-1 {
			i++
		}
		if ct&byte(1<<cts) != 0 {
			if d&0x3f == 0 {
				d = (d<<2 | int(rle[i])) + 1
				if i < len(rle)-1 {
					i++
				}
				n = int(rle[i]) + 2
				if i < len(rle)-1 {
					i++
				}
			} else {
				rb |= byte(d & 0xc0 >> rbc)
				rbc += 2
				n = int(d & 0x3f)
				if rbc < 8 {
					d = int(rle[i]) + 1
					if i < len(rle)-1 {
						i++
					}
				} else {
					d = int(rb) + 1
					rb, rbc = 0, 0
				}
			}
			for {
				if j < len(p) {
					p[j] = p[j-d]
					j++
				}
				n--
				if n < 0 {
					break
				}
			}
		} else {
			if d&0xe0 == 0 {
				n = int(rle[i]) + 8
				if i < len(rle)-1 {
					i++
				}
			} else {
				n = d >> 5
				d &= 0x1f
			}
			for ; n > 0; n-- {
				if j < len(p) {
					p[j] = byte(d)
					j++
				}
			}
		}
		cts++
		if cts >= 8 {
			ct, cts = rle[i], 0
			if i < len(rle)-1 {
				i++
			}
		}
	}
	return
}
