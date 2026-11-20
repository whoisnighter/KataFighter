package services

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"
	"os"
	"sync"
	"time"

	"github.com/whoisnighter/KataFighter/formats/air"
	"github.com/whoisnighter/KataFighter/formats/sff"
)

// ViewerService serves a character's sprites and animations to the
// sprite browser and the animation player. Loaded files are cached by
// path and reloaded when they change on disk.
type ViewerService struct {
	mu   sync.Mutex
	sffs map[string]cached[*sff.File]
	airs map[string]cached[*air.File]
}

type cached[T any] struct {
	mod  time.Time
	size int64
	file T
}

// SpriteInfo is one sprite's header, without pixels.
type SpriteInfo struct {
	Index    int    `json:"index"`
	Group    int    `json:"group"`
	Number   int    `json:"number"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	AxisX    int    `json:"axisX"`
	AxisY    int    `json:"axisY"`
	Format   string `json:"format"`
	Depth    int    `json:"depth"`    // bits per pixel: 8 for indexed, 24/32 for true color
	Palette  int    `json:"palette"`  // index into SpriteSheet.Palettes, -1 for true color
	LinkedTo int    `json:"linkedTo"` // index of the sprite whose image this one shares, -1 if none
}

// PaletteInfo is one palette of an SFF.
type PaletteInfo struct {
	Index    int `json:"index"`
	Group    int `json:"group"`
	Number   int `json:"number"`
	Colors   int `json:"colors"`
	LinkedTo int `json:"linkedTo"`
}

// SpriteSheet lists the sprites and palettes of an SFF.
type SpriteSheet struct {
	Path     string        `json:"path"`
	Version  string        `json:"version"`
	Sprites  []SpriteInfo  `json:"sprites"`
	Palettes []PaletteInfo `json:"palettes"`
}

// Sprites lists the sprites of a character's main SFF.
func (v *ViewerService) Sprites(path string) (*SpriteSheet, error) {
	f, err := v.sff(path)
	if err != nil {
		return nil, err
	}
	sh := &SpriteSheet{Path: path, Version: f.Header.Version.String(),
		Sprites: make([]SpriteInfo, 0, len(f.Sprites)), Palettes: make([]PaletteInfo, 0, len(f.Palettes))}
	for _, s := range f.Sprites {
		si := SpriteInfo{Index: s.Index, Group: int(s.Group), Number: int(s.Number),
			Width: int(s.Size[0]), Height: int(s.Size[1]), AxisX: int(s.Offset[0]), AxisY: int(s.Offset[1]),
			Format: s.Format.String(), Depth: 8, Palette: -1, LinkedTo: s.LinkedTo}
		if len(s.Pix) > 0 {
			si.Palette = paletteIndex(f, s)
		} else if len(s.RGBA) > 0 {
			si.Depth = s.RGBADepth
		}
		sh.Sprites = append(sh.Sprites, si)
	}
	for i, p := range f.Palettes {
		sh.Palettes = append(sh.Palettes, PaletteInfo{Index: i, Group: int(p.Group), Number: int(p.Number),
			Colors: len(p.Colors), LinkedTo: p.LinkedTo})
	}
	return sh, nil
}

// SpriteImage returns sprite group,number as a PNG data URL. pal is a
// palette index to use instead of the character's main palette, or -1.
// Like a palette change in Ikemen, it only recolors sprites that use the
// main palette (the first one); sprites with their own palette keep it.
func (v *ViewerService) SpriteImage(path string, group, number, pal int) (string, error) {
	f, err := v.sff(path)
	if err != nil {
		return "", err
	}
	if group < 0 || group > 0xFFFF || number < 0 || number > 0xFFFF {
		return "", fmt.Errorf("sprite %d,%d out of range", group, number)
	}
	s := f.Sprite(uint16(group), uint16(number))
	if s == nil {
		return "", fmt.Errorf("sprite %d,%d not found", group, number)
	}
	var colors []uint32
	if len(s.Pix) > 0 && pal >= 0 && pal < len(f.Palettes) && paletteIndex(f, s) == 0 {
		colors = f.Palettes[pal].Colors
	}
	img, err := f.Image(s, colors)
	if err != nil {
		return "", fmt.Errorf("sprite %d,%d: %w", group, number, err)
	}
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// paletteIndex is the palette an indexed sprite draws with, following
// palette links so that palettes sharing data compare equal.
func paletteIndex(f *sff.File, s *sff.Sprite) int {
	i := s.PalIndex
	if i < 0 || i >= len(f.Palettes) {
		i = 0
	}
	for n := 0; n < 8 && f.Palettes[i].LinkedTo >= 0 && f.Palettes[i].LinkedTo < len(f.Palettes); n++ {
		i = f.Palettes[i].LinkedTo
	}
	return i
}

// AnimFrame is one animation element, ready to draw.
type AnimFrame struct {
	Line   int `json:"line"`
	Group  int `json:"group"`
	Number int `json:"number"`
	// Offset as written: where the sprite axis sits relative to the
	// character's position. Flips mirror the sprite around its axis and
	// leave the offset as is (Ikemen negates both, anim.go:1067).
	OffsetX  int          `json:"offsetX"`
	OffsetY  int          `json:"offsetY"`
	Time     int          `json:"time"` // ticks; -1 = forever
	FlipH    bool         `json:"flipH"`
	FlipV    bool         `json:"flipV"`
	Trans    string       `json:"trans"` // none, add, sub, subadd
	SrcAlpha int          `json:"srcAlpha"`
	DstAlpha int          `json:"dstAlpha"`
	ScaleX   float32      `json:"scaleX"`
	ScaleY   float32      `json:"scaleY"`
	Angle    float32      `json:"angle"`
	Clsn1    [][4]float32 `json:"clsn1"` // attack boxes: left, top, right, bottom
	Clsn2    [][4]float32 `json:"clsn2"` // hurt boxes
}

// AnimAction is one [Begin Action N].
type AnimAction struct {
	Number     int         `json:"number"`
	Line       int         `json:"line"`
	Frames     []AnimFrame `json:"frames"`
	LoopStart  int         `json:"loopStart"`
	TotalTime  int         `json:"totalTime"` // -1 = ends on a frame that lasts forever
	CopiedFrom int         `json:"copiedFrom"`
}

// Actions lists the actions of an AIR file in ascending order.
func (v *ViewerService) Actions(path string) ([]AnimAction, error) {
	f, err := v.air(path)
	if err != nil {
		return nil, err
	}
	out := make([]AnimAction, 0, len(f.Actions))
	for _, no := range f.Numbers() {
		a := f.Actions[no]
		aa := AnimAction{Number: int(a.Number), Line: a.Line, LoopStart: a.LoopStart,
			TotalTime: int(a.TotalTime), CopiedFrom: int(a.CopiedFrom), Frames: make([]AnimFrame, 0, len(a.Frames))}
		for _, fr := range a.Frames {
			aa.Frames = append(aa.Frames, AnimFrame{
				Line: fr.Line, Group: int(fr.Group), Number: int(fr.Number),
				OffsetX: int(fr.Offset[0]), OffsetY: int(fr.Offset[1]), Time: int(fr.Time),
				FlipH: fr.FlipH, FlipV: fr.FlipV, Trans: fr.Trans.String(),
				SrcAlpha: int(fr.SrcAlpha), DstAlpha: int(fr.DstAlpha),
				ScaleX: fr.Scale[0], ScaleY: fr.Scale[1], Angle: fr.Angle,
				Clsn1: boxes(fr.Clsn1), Clsn2: boxes(fr.Clsn2),
			})
		}
		out = append(out, aa)
	}
	return out, nil
}

func boxes(bs []air.Box) [][4]float32 {
	out := make([][4]float32, len(bs))
	for i, b := range bs {
		out[i] = b
	}
	return out
}

func (v *ViewerService) sff(path string) (*sff.File, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.sffs == nil {
		v.sffs = map[string]cached[*sff.File]{}
	}
	return load(v.sffs, path, func(p string) (*sff.File, error) {
		return sff.Open(p, sff.Options{Character: true})
	})
}

func (v *ViewerService) air(path string) (*air.File, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.airs == nil {
		v.airs = map[string]cached[*air.File]{}
	}
	return load(v.airs, path, air.Load)
}

func load[T any](cache map[string]cached[T], path string, open func(string) (T, error)) (T, error) {
	var zero T
	st, err := os.Stat(path)
	if err != nil {
		return zero, err
	}
	if c, ok := cache[path]; ok && c.mod.Equal(st.ModTime()) && c.size == st.Size() {
		return c.file, nil
	}
	f, err := open(path)
	if err != nil {
		return zero, err
	}
	cache[path] = cached[T]{mod: st.ModTime(), size: st.Size(), file: f}
	return f, nil
}
