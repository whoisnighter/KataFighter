package sff

import (
	"errors"
	"image"
	"image/color"
)

// ColorOf unpacks an SFF palette entry (A<<24 | B<<16 | G<<8 | R).
func ColorOf(c uint32) color.NRGBA {
	return color.NRGBA{R: uint8(c), G: uint8(c >> 8), B: uint8(c >> 16), A: uint8(c >> 24)}
}

// Image returns the sprite as an image.Image. Indexed sprites use pal if it
// is non-nil, otherwise the sprite's own palette from the file.
func (f *File) Image(s *Sprite, pal []uint32) (image.Image, error) {
	w, h := int(s.Size[0]), int(s.Size[1])
	rect := image.Rect(0, 0, w, h)
	switch {
	case len(s.Pix) > 0:
		if pal == nil {
			if p := f.Palette(s); p != nil {
				pal = p.Colors
			}
		}
		cp := make(color.Palette, 256)
		for i := range cp {
			cp[i] = color.NRGBA{}
			if i < len(pal) {
				cp[i] = ColorOf(pal[i])
			}
		}
		return &image.Paletted{Pix: s.Pix, Stride: w, Rect: rect, Palette: cp}, nil
	case len(s.RGBA) > 0 && s.RGBADepth == 24:
		img := image.NewNRGBA(rect)
		for i, j := 0, 0; i+2 < len(s.RGBA) && j+3 < len(img.Pix); i, j = i+3, j+4 {
			img.Pix[j], img.Pix[j+1], img.Pix[j+2], img.Pix[j+3] = s.RGBA[i], s.RGBA[i+1], s.RGBA[i+2], 255
		}
		return img, nil
	case len(s.RGBA) > 0 && s.Premultiplied:
		return &image.RGBA{Pix: s.RGBA, Stride: w * 4, Rect: rect}, nil
	case len(s.RGBA) > 0:
		return &image.NRGBA{Pix: s.RGBA, Stride: w * 4, Rect: rect}, nil
	}
	return nil, errors.New("sprite has no image data")
}
