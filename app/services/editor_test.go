package services

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEditorOpenSave(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.cns")
	if err := os.WriteFile(p, []byte("\xEF\xBB\xBF[Statedef 0]\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var e EditorService
	f, err := e.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if f.Text != "[Statedef 0]\r\n" || f.Language != "cns" || f.ReadOnly {
		t.Fatalf("open = %+v", f)
	}

	v, err := e.Save(p, "[Statedef 1]\r\n", f.Version)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "\xEF\xBB\xBF[Statedef 1]\r\n" {
		t.Errorf("saved %q (the BOM and CRLF must be kept)", b)
	}

	// A stale version is refused.
	if _, err := e.Save(p, "x", f.Version); err == nil {
		t.Error("save with a stale version should fail")
	}
	// Someone else changes the file: the current version is refused too.
	later := time.Now().Add(2 * time.Second)
	os.WriteFile(p, []byte("changed elsewhere"), 0o644)
	os.Chtimes(p, later, later)
	if _, err := e.Save(p, "x", v); err == nil {
		t.Error("save over an external change should fail")
	}
	if b, _ := os.ReadFile(p); string(b) != "changed elsewhere" {
		t.Errorf("file overwritten: %q", b)
	}

	// Saving never creates files.
	if _, err := e.Save(filepath.Join(dir, "new.cns"), "x", ""); err == nil {
		t.Error("save to a missing file should fail")
	}
	if m, _ := filepath.Glob(filepath.Join(dir, "*.tmp")); len(m) > 0 {
		t.Errorf("temp files left: %v", m)
	}
}

func TestEditorOpenNotUTF8(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sjis.cmd")
	os.WriteFile(p, []byte("; \x82\xa0\n[Command]\n"), 0o644) // "あ" in Shift_JIS
	f, err := (&EditorService{}).Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if !f.ReadOnly || f.Reason == "" {
		t.Errorf("open = %+v, want read-only", f)
	}
}

func TestEditorCheck(t *testing.T) {
	var e EditorService
	// "é" before the error: byte column 16, UTF-16 column 15.
	src := "[Statedef 0]\n[State 0, é]\ntype = Null\ntrigger1 = Tme\n"
	ms := e.Check("x.cns", src)
	if len(ms) != 1 {
		t.Fatalf("markers = %+v", ms)
	}
	m := ms[0]
	if m.Code != "unknown-trigger" || m.Severity != "error" || m.StartLine != 4 || m.StartCol != 12 || m.EndCol != 15 || m.Line != 4 || m.Col != 12 {
		t.Errorf("marker = %+v", m)
	}

	ms = e.Check("x.cns", "[Statedef 0]\n[State 0, é]\ntype = Nul\ntrigger1 = 1\n")
	if len(ms) != 1 || ms[0].StartCol != 8 || ms[0].EndCol != 11 {
		t.Errorf("markers = %+v", ms)
	}

	if ms := e.Check("x.air", "anything"); len(ms) != 0 {
		t.Errorf("air markers = %+v", ms)
	}
}
