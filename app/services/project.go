// Package services holds the Go side of the app. The frontend reaches it
// only through the generated Wails bindings; all format logic comes from
// katafighter-formats.
package services

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/whoisnighter/KataFighter/formats/air"
	"github.com/whoisnighter/KataFighter/formats/cns"
	"github.com/whoisnighter/KataFighter/formats/def"
	"github.com/whoisnighter/KataFighter/formats/diag"
	"github.com/whoisnighter/KataFighter/formats/sff"
	"github.com/whoisnighter/KataFighter/formats/zss"
)

// ProjectService opens a character from its DEF file.
type ProjectService struct{}

// FileInfo is one file referenced by the DEF.
type FileInfo struct {
	Role    string `json:"role"`    // "cns", "st[0]", "sprite", ...
	Name    string `json:"name"`    // as written in the DEF
	Path    string `json:"path"`    // resolved path, "" when not found
	Summary string `json:"summary"` // what the file holds, or a load error
}

// Problem is a diagnostic flattened for the UI.
type Problem struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// Character is what the UI shows after opening a DEF.
type Character struct {
	DefPath     string     `json:"defPath"`
	EngineDir   string     `json:"engineDir"`
	Name        string     `json:"name"`
	DisplayName string     `json:"displayName"`
	Author      string     `json:"author"`
	LocalCoord  [2]int32   `json:"localCoord"`
	Files       []FileInfo `json:"files"`
	Problems    []Problem  `json:"problems"`
}

// Mode reports how the app runs: "desktop" (native window) or "browser"
// (server mode, see mode_server.go), so the UI can adapt (no native
// dialogs or window buttons in the browser).
func (s *ProjectService) Mode() string { return mode }

// ChooseDef shows the native open dialog and returns the chosen DEF path,
// or "" when the user cancels.
func (s *ProjectService) ChooseDef() (string, error) {
	if mode != "desktop" {
		return "", fmt.Errorf("native dialogs are not available in %s mode; type the path instead", mode)
	}
	p, err := application.Get().Dialog.OpenFile().
		SetTitle("Open character").
		CanChooseFiles(true).
		AddFilter("Character definition (*.def)", "*.def").
		PromptForSingleSelection()
	if err != nil && strings.Contains(err.Error(), "cancelled") {
		return "", nil // the user closed the dialog
	}
	return p, err
}

// Open loads a character's DEF and every file it references.
func (s *ProjectService) Open(defPath string) (*Character, error) {
	d, err := def.Load(defPath, def.Options{})
	if err != nil {
		return nil, err
	}
	engine := def.EngineDir(defPath)
	c := &Character{
		DefPath:     defPath,
		EngineDir:   engine,
		Name:        d.Info.Name,
		DisplayName: d.Info.DisplayName,
		Author:      d.Info.Author,
		LocalCoord:  d.Info.LocalCoord,
		Files:       []FileInfo{},
		Problems:    []Problem{},
	}
	c.addProblems(d.Diagnostics)

	add := func(role, name string, find func(string, string) string, load func(string) string) {
		if name == "" {
			return
		}
		fi := FileInfo{Role: role, Name: name, Path: find(name, engine)}
		if fi.Path == "" {
			fi.Summary = "not found"
			c.Problems = append(c.Problems, Problem{File: defPath, Severity: "error",
				Message: fmt.Sprintf("%s file %q not found", role, name)})
		} else if load != nil {
			fi.Summary = load(fi.Path)
		}
		c.Files = append(c.Files, fi)
	}
	f := d.Files
	add("cns", f.CNS, d.Find, c.loadText)
	add("cmd", f.CMD, d.Find, c.loadText)
	for i, st := range f.ST {
		load := c.loadText
		if st == f.CNS {
			load = func(string) string { return "same file as cns" }
		}
		add(fmt.Sprintf("st[%d]", i), st, d.FindState, load)
	}
	add("stcommon", f.StCommon, d.FindState, c.loadText)
	add("sprite", f.Sprite, d.Find, c.loadSprite)
	add("anim", f.Anim, d.Find, c.loadAnim)
	add("sound", f.Sound, d.Find, nil)
	for i := 1; i <= def.PaletteMax; i++ {
		add(fmt.Sprintf("pal%d", i), f.Pal[i], d.Find, nil)
	}
	return c, nil
}

func (c *Character) addProblems(ds []diag.Diagnostic) {
	for _, d := range ds {
		c.Problems = append(c.Problems, Problem{
			File: d.File, Line: d.Range.Start.Line, Col: d.Range.Start.Col,
			Severity: d.Severity.String(), Message: d.Message,
		})
	}
}

func (c *Character) loadSprite(p string) string {
	f, err := sff.Open(p, sff.Options{Character: true})
	if err != nil {
		return "error: " + err.Error()
	}
	c.addProblems(f.Diagnostics)
	return fmt.Sprintf("SFF %v, %d sprites, %d palettes", f.Header.Version, len(f.Sprites), len(f.Palettes))
}

func (c *Character) loadAnim(p string) string {
	f, err := air.Load(p)
	if err != nil {
		return "error: " + err.Error()
	}
	c.addProblems(f.Diagnostics)
	return fmt.Sprintf("%d actions", len(f.Actions))
}

func (c *Character) loadText(p string) string {
	if strings.EqualFold(filepath.Ext(p), ".zss") {
		f, err := zss.Load(p)
		if err != nil {
			return "error: " + err.Error()
		}
		c.addProblems(f.Diagnostics)
		return fmt.Sprintf("ZSS, %d states, %d functions", len(f.States), len(f.Functions))
	}
	f, err := cns.Load(p)
	if err != nil {
		return "error: " + err.Error()
	}
	c.addProblems(f.Diagnostics)
	ctrls := 0
	for _, s := range f.States {
		ctrls += len(s.Controllers)
	}
	switch {
	case len(f.Commands) > 0:
		return fmt.Sprintf("%d commands, %d states", len(f.Commands), len(f.States))
	case len(f.States) > 0:
		return fmt.Sprintf("%d states, %d controllers", len(f.States), ctrls)
	}
	return fmt.Sprintf("%d sections", len(f.Sections))
}
