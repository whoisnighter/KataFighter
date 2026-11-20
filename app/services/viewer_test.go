package services

import (
	"os"
	"strings"
	"testing"
)

func TestViewerKFM(t *testing.T) {
	dir := release + "/chars/kfm/"
	if _, err := os.Stat(dir + "kfm.sff"); err != nil {
		t.Skip("no Ikemen GO release in ikemen/release")
	}
	v := &ViewerService{}
	sh, err := v.Sprites(dir + "kfm.sff")
	if err != nil {
		t.Fatal(err)
	}
	if len(sh.Sprites) != 281 || len(sh.Palettes) == 0 {
		t.Fatalf("%d sprites, %d palettes", len(sh.Sprites), len(sh.Palettes))
	}
	a, err := v.SpriteImage(dir+"kfm.sff", 0, 0, -1)
	if err != nil || !strings.HasPrefix(a, "data:image/png;base64,") {
		t.Fatalf("sprite 0,0: %v", err)
	}
	b, err := v.SpriteImage(dir+"kfm.sff", 0, 0, 1)
	if err != nil || a == b {
		t.Errorf("palette 1 should recolor sprite 0,0 (err %v)", err)
	}
	if _, err := v.SpriteImage(dir+"kfm.sff", 9999, 0, -1); err == nil {
		t.Error("missing sprite: no error")
	}

	acts, err := v.Actions(dir + "kfm.air")
	if err != nil {
		t.Fatal(err)
	}
	var stand *AnimAction
	for i := range acts {
		if i > 0 && acts[i].Number <= acts[i-1].Number {
			t.Fatal("actions not sorted")
		}
		if acts[i].Number == 0 {
			stand = &acts[i]
		}
	}
	if stand == nil || len(stand.Frames) == 0 || len(stand.Frames[0].Clsn2) == 0 {
		t.Fatalf("action 0: %+v", stand)
	}
}
