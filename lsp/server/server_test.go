package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

// client drives a Server over in-memory pipes.
type client struct {
	t    *testing.T
	in   *io.PipeWriter
	out  *conn
	done chan error
	id   int
}

func start(t *testing.T) *client {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	c := &client{t: t, in: inW, out: newConn(outR, io.Discard), done: make(chan error, 1)}
	go func() {
		err := New(inR, outW, nil).Run()
		outW.Close()
		c.done <- err
	}()
	return c
}

func (c *client) send(method string, id *int, params any) {
	c.t.Helper()
	m := map[string]any{"jsonrpc": "2.0", "method": method}
	if id != nil {
		m["id"] = *id
	}
	if params != nil {
		m["params"] = params
	}
	b, _ := json.Marshal(m)
	if _, err := fmt.Fprintf(c.in, "Content-Length: %d\r\n\r\n%s", len(b), b); err != nil {
		c.t.Fatal(err)
	}
}

func (c *client) request(method string, params any) map[string]any {
	c.t.Helper()
	c.id++
	id := c.id
	c.send(method, &id, params)
	for {
		m := c.next()
		if m["id"] != nil && int(m["id"].(float64)) == id {
			return m
		}
	}
}

func (c *client) next() map[string]any {
	c.t.Helper()
	type res struct {
		b   []byte
		err error
	}
	ch := make(chan res, 1)
	go func() { b, err := c.out.read(); ch <- res{b, err} }()
	select {
	case r := <-ch:
		if r.err != nil {
			c.t.Fatal(r.err)
		}
		var m map[string]any
		if err := json.Unmarshal(r.b, &m); err != nil {
			c.t.Fatal(err)
		}
		return m
	case <-time.After(5 * time.Second):
		c.t.Fatal("timeout waiting for the server")
	}
	return nil
}

func (c *client) diagnostics() (uri string, ds []any) {
	c.t.Helper()
	for {
		m := c.next()
		if m["method"] == "textDocument/publishDiagnostics" {
			p := m["params"].(map[string]any)
			return p["uri"].(string), p["diagnostics"].([]any)
		}
	}
}

func TestSession(t *testing.T) {
	c := start(t)

	// Requests before initialize are rejected.
	if r := c.request("shutdown", nil); r["error"] == nil {
		t.Error("shutdown before initialize should fail")
	}

	r := c.request("initialize", map[string]any{"processId": nil, "capabilities": map[string]any{}})
	caps := r["result"].(map[string]any)["capabilities"].(map[string]any)
	if caps["textDocumentSync"].(map[string]any)["change"].(float64) != 1 {
		t.Errorf("capabilities = %v", caps)
	}
	c.send("initialized", nil, map[string]any{})

	uri := "file:///c%3A/chars/test/test.cns"
	c.send("textDocument/didOpen", nil, map[string]any{"textDocument": map[string]any{
		"uri": uri, "languageId": "cns", "version": 1,
		"text": "[Statedef 0]\n[State 0, x]\ntype = Nul\ntrigger1 = Tme\n",
	}})
	got, ds := c.diagnostics()
	if got != uri || len(ds) != 2 {
		t.Fatalf("uri %s, diagnostics %v", got, ds)
	}
	d := ds[0].(map[string]any)
	if d["code"] != "unknown-controller" || d["severity"].(float64) != 1 {
		t.Errorf("first diagnostic = %v", d)
	}
	rng := d["range"].(map[string]any)
	if s, e := rng["start"].(map[string]any), rng["end"].(map[string]any); s["line"].(float64) != 2 ||
		s["character"].(float64) != 7 || e["character"].(float64) != 10 {
		t.Errorf("range = %v", rng)
	}

	c.send("textDocument/didChange", nil, map[string]any{
		"textDocument":   map[string]any{"uri": uri, "version": 2},
		"contentChanges": []any{map[string]any{"text": "[Statedef 0]\n[State 0, x]\ntype = Null\ntrigger1 = Time\n"}},
	})
	if _, ds := c.diagnostics(); len(ds) != 0 {
		t.Errorf("after fix: %v", ds)
	}

	// Files of other kinds get empty diagnostics.
	c.send("textDocument/didOpen", nil, map[string]any{"textDocument": map[string]any{
		"uri": "file:///x/readme.txt", "languageId": "plaintext", "version": 1, "text": "trigger1 = Tme",
	}})
	if _, ds := c.diagnostics(); len(ds) != 0 {
		t.Errorf("txt: %v", ds)
	}

	c.send("textDocument/didClose", nil, map[string]any{"textDocument": map[string]any{"uri": uri}})
	if got, ds := c.diagnostics(); got != uri || len(ds) != 0 {
		t.Errorf("close: %s %v", got, ds)
	}

	if r := c.request("textDocument/hover", map[string]any{}); r["error"].(map[string]any)["code"].(float64) != codeMethodNotFound {
		t.Errorf("hover = %v", r)
	}
	if r := c.request("shutdown", nil); r["error"] != nil {
		t.Errorf("shutdown = %v", r)
	}
	c.send("exit", nil, nil)
	if err := <-c.done; err != nil {
		t.Errorf("Run = %v", err)
	}
}

func TestExitWithoutShutdown(t *testing.T) {
	c := start(t)
	c.send("exit", nil, nil)
	if err := <-c.done; err == nil {
		t.Error("exit without shutdown should be an error")
	}
}

func TestReadFrame(t *testing.T) {
	c := newConn(bufio.NewReader(strings.NewReader("Content-Length: 2\r\nContent-Type: application/vscode-jsonrpc; charset=utf-8\r\n\r\n{}")), io.Discard)
	b, err := c.read()
	if err != nil || string(b) != "{}" {
		t.Errorf("read = %q, %v", b, err)
	}
	c = newConn(strings.NewReader("Content-Length: x\r\n\r\n"), io.Discard)
	if _, err := c.read(); err == nil {
		t.Error("bad length should fail")
	}
}
