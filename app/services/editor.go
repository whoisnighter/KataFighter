package services

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"unicode/utf8"

	"github.com/whoisnighter/KataFighter/lsp/check"
	"github.com/whoisnighter/KataFighter/lsp/schema"
	"github.com/whoisnighter/KataFighter/lsp/textpos"
)

// EditorService backs the code editor: it reads and saves text files and
// checks them with katafighter-lsp (parser + schema diagnostics).
type EditorService struct{}

// TextFile is a file opened in the code editor.
type TextFile struct {
	Path string `json:"path"`
	Text string `json:"text"`
	// Language is "cns" (CNS, ST, CMD, const), "zss" or "plaintext".
	Language string `json:"language"`
	// Version identifies the content on disk (size and modification time).
	// Save refuses to overwrite a file whose version changed since.
	Version string `json:"version"`
	// ReadOnly is set for files that are not valid UTF-8 (Shift_JIS
	// comments in older characters): saving them would corrupt the text.
	ReadOnly bool   `json:"readOnly"`
	Reason   string `json:"reason"` // why the file is read-only
}

// Marker is a diagnostic in editor coordinates: 1-based lines and 1-based
// UTF-16 columns, as Monaco expects.
type Marker struct {
	StartLine int    `json:"startLine"`
	StartCol  int    `json:"startCol"`
	EndLine   int    `json:"endLine"`
	EndCol    int    `json:"endCol"`
	Severity  string `json:"severity"` // "error", "warning", "info", "hint"
	Code      string `json:"code"`
	Message   string `json:"message"`
	// Line and Col are the diagnostic's own 1-based line and byte column,
	// for the Problems panel.
	Line int `json:"line"`
	Col  int `json:"col"`
}

var bom = []byte{0xEF, 0xBB, 0xBF}

// Open reads a text file for the editor.
func (e *EditorService) Open(path string) (*TextFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	b = bytes.TrimPrefix(b, bom)
	f := &TextFile{Path: path, Language: language(path), Version: version(st)}
	if utf8.Valid(b) {
		f.Text = string(b)
	} else {
		f.Text = string(bytes.ToValidUTF8(b, []byte(string(utf8.RuneError))))
		f.ReadOnly = true
		f.Reason = "The file is not UTF-8 (probably Shift_JIS), so it is opened read-only."
	}
	return f, nil
}

// Save writes text to an existing file and returns its new version. It
// fails if the file changed on disk since version was read, keeps a UTF-8
// BOM if the file had one, and replaces the file atomically.
func (e *EditorService) Save(path, text, ver string) (string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !st.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file", path)
	}
	if version(st) != ver {
		return "", errors.New("the file changed on disk since it was opened; reopen it before saving")
	}
	old, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	data := []byte(text)
	if bytes.HasPrefix(old, bom) {
		data = append(append([]byte{}, bom...), data...)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(tmp.Name(), st.Mode().Perm()); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return "", err
	}
	st, err = os.Stat(path)
	if err != nil {
		return "", err
	}
	return version(st), nil
}

// Check returns the diagnostics for text as if it were the file at path
// (the extension picks the language). Other files have none.
func (e *EditorService) Check(path, text string) []Marker {
	out := []Marker{}
	ls := textpos.Lines(text)
	for _, d := range check.Text(check.KindOf(path), text, schema.Default()) {
		r := textpos.ToRange(ls, d.Range)
		if d.Range.Start.Line <= 0 { // whole file: mark the first line
			r.End.Line, r.End.Character = 0, 1
		}
		out = append(out, Marker{
			StartLine: r.Start.Line + 1, StartCol: r.Start.Character + 1,
			EndLine: r.End.Line + 1, EndCol: r.End.Character + 1,
			Severity: d.Severity.String(), Code: d.Code, Message: d.Message,
			Line: d.Range.Start.Line, Col: d.Range.Start.Col,
		})
	}
	return out
}

func language(path string) string {
	switch check.KindOf(path) {
	case check.KindCNS:
		return "cns"
	case check.KindZSS:
		return "zss"
	}
	return "plaintext"
}

func version(st os.FileInfo) string {
	return strconv.FormatInt(st.Size(), 10) + "-" + strconv.FormatInt(st.ModTime().UnixNano(), 10)
}
