package sff

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The oracle test checks our decoders against Ikemen GO's own code, on every
// compressed sprite of the release corpus. It pulls the four decoder methods
// out of ikemen/source/src/image.go with go/ast, compiles them into a small
// throwaway program next to a stub Sprite type, and feeds it the exact bytes
// our loader decoded. Nothing from Ikemen is committed: the program is built
// in a temp dir at test time.
//
// It is skipped when the submodule, the release or the go tool is missing.

const upstreamImageGo = "../../ikemen/source/src/image.go"

var upstreamDecoders = map[string]string{
	"pcx":  "RlePcxDecode",
	"rle8": "Rle8Decode",
	"rle5": "Rle5Decode",
	"lz5":  "Lz5Decode",
}

const oracleMain = `package main

import (
	"bufio"
	"encoding/binary"
	"io"
	"os"
)

// Stub with only the fields the extracted decoders use.
type Sprite struct {
	Size [2]uint16
	rle  int
}

var kinds = map[byte]func(*Sprite, []byte) []byte{
	1: (*Sprite).RlePcxDecode,
	2: (*Sprite).Rle8Decode,
	3: (*Sprite).Rle5Decode,
	4: (*Sprite).Lz5Decode,
}

func run(dec func(*Sprite, []byte) []byte, s *Sprite, in []byte) (out []byte, ok bool) {
	defer func() {
		if recover() != nil {
			out, ok = nil, false
		}
	}()
	return dec(s, in), true
}

func main() {
	r, w := bufio.NewReader(os.Stdin), bufio.NewWriter(os.Stdout)
	defer w.Flush()
	for {
		var hdr struct {
			Kind      byte
			W, H, Bpl int32
			N         uint32
		}
		if err := binary.Read(r, binary.LittleEndian, &hdr); err != nil {
			return
		}
		in := make([]byte, hdr.N)
		if _, err := io.ReadFull(r, in); err != nil {
			panic(err)
		}
		s := &Sprite{Size: [2]uint16{uint16(hdr.W), uint16(hdr.H)}, rle: int(hdr.Bpl)}
		out, ok := run(kinds[hdr.Kind], s, in)
		var okb byte
		if ok {
			okb = 1
		}
		binary.Write(w, binary.LittleEndian, okb)
		binary.Write(w, binary.LittleEndian, uint32(len(out)))
		w.Write(out)
	}
}
`

var oracleKind = map[string]byte{"pcx": 1, "rle8": 2, "rle5": 3, "lz5": 4}

// buildOracle extracts Ikemen's decoders and builds the oracle program.
func buildOracle(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat(upstreamImageGo); err != nil {
		t.Skip("ikemen/source submodule not checked out")
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go tool not found")
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, upstreamImageGo, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var src bytes.Buffer
	src.WriteString("// Extracted at test time from ikemen/source/src/image.go (MIT, see ikemen/source/LICENCE.txt).\n\n")
	src.WriteString(oracleMain)
	found := map[string]bool{}
	for _, d := range file.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Recv == nil {
			continue
		}
		for _, name := range upstreamDecoders {
			if fn.Name.Name == name {
				src.WriteString("\n")
				printer.Fprint(&src, fset, fn)
				src.WriteString("\n")
				found[name] = true
			}
		}
	}
	for _, name := range upstreamDecoders {
		if !found[name] {
			t.Fatalf("%s not found in %s", name, upstreamImageGo)
		}
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module oracle\n\ngo 1.20\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), src.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "oracle.exe")
	cmd := exec.Command(goTool, "build", "-o", bin, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=", "CGO_ENABLED=0", "GOTOOLCHAIN=local")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building oracle: %v\n%s", err, out)
	}
	return bin
}

type decodeCall struct {
	kind      string
	in, out   []byte
	w, h, bpl int
	failed    bool
}

func TestDecodersMatchIkemen(t *testing.T) {
	files := corpus(t)
	bin := buildOracle(t)
	total := map[string]int{}
	for _, p := range files {
		var calls []decodeCall
		onDecode = func(kind string, in []byte, w, h, bpl int, out []byte, err error) {
			calls = append(calls, decodeCall{kind, bytes.Clone(in), bytes.Clone(out), w, h, bpl, err != nil})
		}
		_, err := Open(p, Options{Character: isCharSff(p)})
		onDecode = nil
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if len(calls) == 0 {
			continue
		}
		got, err := runOracle(bin, calls)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		for i, c := range calls {
			total[c.kind]++
			g := got[i]
			if g.failed != c.failed || !bytes.Equal(g.out, c.out) {
				t.Errorf("%s: call %d (%s %dx%d): ours %d bytes (failed=%v), Ikemen %d bytes (failed=%v)",
					filepath.ToSlash(p), i, c.kind, c.w, c.h, len(c.out), c.failed, len(g.out), g.failed)
			}
		}
	}
	t.Logf("compared with Ikemen: %v", total)
	if len(total) == 0 {
		t.Error("no compressed sprites found in the corpus")
	}
}

func runOracle(bin string, calls []decodeCall) ([]decodeCall, error) {
	var in bytes.Buffer
	for _, c := range calls {
		binary.Write(&in, binary.LittleEndian, struct {
			Kind      byte
			W, H, Bpl int32
			N         uint32
		}{oracleKind[c.kind], int32(c.w), int32(c.h), int32(c.bpl), uint32(len(c.in))})
		in.Write(c.in)
	}
	cmd := exec.Command(bin)
	cmd.Stdin = &in
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("oracle: %v\n%s", err, stderr.Bytes())
	}
	r := bufio.NewReader(bytes.NewReader(out))
	res := make([]decodeCall, len(calls))
	for i := range res {
		var ok byte
		var n uint32
		if err := binary.Read(r, binary.LittleEndian, &ok); err != nil {
			return nil, fmt.Errorf("oracle output truncated at %d: %v", i, err)
		}
		binary.Read(r, binary.LittleEndian, &n)
		res[i].out = make([]byte, n)
		if _, err := io.ReadFull(r, res[i].out); err != nil {
			return nil, err
		}
		res[i].failed = ok == 0
		if res[i].failed {
			res[i].out = nil
		}
	}
	return res, nil
}
