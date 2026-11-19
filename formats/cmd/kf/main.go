// Command kf is a debugging CLI for katafighter-formats.
//
//	kf inspect [-char] <file.sff|file.air>  print a summary and warnings
//	kf dump [-char] [-pal N] <file.sff> <outdir>   write every sprite as <group>_<number>.png
package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/whoisnighter/KataFighter/formats/air"
	"github.com/whoisnighter/KataFighter/formats/sff"
)

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  kf inspect [-char] <file.sff|file.air>
  kf dump [-char] [-pal N] <file.sff> <outdir>

  -char   treat the file as a character's main SFF (SFF v1 palette rules)
  -pal N  render indexed sprites with palette N (1-based, v2 key 1,N) instead of their own`)
	os.Exit(2)
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	fs := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	char := fs.Bool("char", false, "")
	pal := fs.Int("pal", 0, "")
	fs.Usage = usage
	fs.Parse(os.Args[2:])

	var err error
	switch os.Args[1] {
	case "inspect":
		if fs.NArg() != 1 {
			usage()
		}
		err = inspect(fs.Arg(0), *char)
	case "dump":
		if fs.NArg() != 2 {
			usage()
		}
		err = dump(fs.Arg(0), fs.Arg(1), *char, *pal)
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "kf:", err)
		os.Exit(1)
	}
}

func inspect(path string, char bool) error {
	if strings.EqualFold(filepath.Ext(path), ".air") {
		return inspectAir(path)
	}
	f, err := sff.Open(path, sff.Options{Character: char})
	if err != nil {
		return err
	}
	h := f.Header
	fmt.Printf("file      %s\nversion   %v\nsprites   %d (header says %d)\npalettes  %d\n",
		path, h.Version, len(f.Sprites), h.NumberOfSprites, len(f.Palettes))

	formats := map[string]int{}
	linked, empty := 0, 0
	for _, s := range f.Sprites {
		formats[fmt.Sprintf("%v/%dbpp", s.Format, s.ColorDepth)]++
		if s.LinkedTo >= 0 {
			linked++
		}
		if !s.HasImage() {
			empty++
		}
	}
	keys := make([]string, 0, len(formats))
	for k := range formats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Println("formats:")
	for _, k := range keys {
		fmt.Printf("  %-14s %d\n", k, formats[k])
	}
	fmt.Printf("linked    %d\nno image  %d\n", linked, empty)

	if len(f.Palettes) > 0 && h.Version[0] == 2 {
		fmt.Println("palettes (group,number colors [link]):")
		for i, p := range f.Palettes {
			if i >= 12 {
				fmt.Printf("  ... %d more\n", len(f.Palettes)-i)
				break
			}
			link := ""
			if p.LinkedTo >= 0 {
				link = fmt.Sprintf(" -> %d", p.LinkedTo)
			}
			fmt.Printf("  %3d: %d,%d  %d%s\n", i, p.Group, p.Number, p.NumColors, link)
		}
	}
	fmt.Printf("warnings  %d\n", len(f.Warnings))
	for _, w := range f.Warnings {
		fmt.Println("  " + w)
	}
	return nil
}

func inspectAir(path string) error {
	f, err := air.Load(path)
	if err != nil {
		return err
	}
	frames, copied := 0, 0
	for _, a := range f.Actions {
		frames += len(a.Frames)
		if a.CopiedFrom >= 0 {
			copied++
		}
	}
	fmt.Printf("file      %s\nactions   %d (%d copied)\nframes    %d\n", path, len(f.Actions), copied, frames)
	fmt.Println("actions (number: frames, loopstart, total time):")
	for i, no := range f.Numbers() {
		if i >= 15 {
			fmt.Printf("  ... %d more\n", len(f.Actions)-i)
			break
		}
		a := f.Actions[no]
		fmt.Printf("  %5d: %2d frames, loop %d, time %d\n", no, len(a.Frames), a.LoopStart, a.TotalTime)
	}
	fmt.Printf("diagnostics %d\n", len(f.Diagnostics))
	for _, d := range f.Diagnostics {
		fmt.Println("  " + d.String())
	}
	return nil
}

func dump(path, out string, char bool, palNo int) error {
	f, err := sff.Open(path, sff.Options{Character: char})
	if err != nil {
		return err
	}
	var pal []uint32
	if palNo > 0 {
		for _, p := range f.Palettes {
			if p.Group == 1 && int(p.Number) == palNo {
				pal = p.Colors
			}
		}
		if pal == nil {
			return fmt.Errorf("palette 1,%d not found", palNo)
		}
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	n := 0
	for _, s := range f.Sprites {
		img, err := f.Image(s, pal)
		if err != nil {
			continue
		}
		name := filepath.Join(out, fmt.Sprintf("%d_%d.png", s.Group, s.Number))
		w, err := os.Create(name)
		if err != nil {
			return err
		}
		err = png.Encode(w, img)
		w.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		n++
	}
	fmt.Printf("wrote %d sprites to %s (%d warnings)\n", n, out, len(f.Warnings))
	return nil
}
