// Package server is the katafighter-lsp language server: LSP over a byte
// stream (stdio in cmd/katafighter-lsp). It keeps the open documents in
// memory and publishes diagnostics for CNS, ST, CMD and ZSS files on every
// change, using the formats parsers and lsp/check.
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"path"

	"github.com/whoisnighter/KataFighter/formats/diag"
	"github.com/whoisnighter/KataFighter/lsp/check"
	"github.com/whoisnighter/KataFighter/lsp/schema"
)

// Name and Version are reported to the client in initialize.
const (
	Name    = "katafighter-lsp"
	Version = "0.0.1"
)

// Server is one LSP session.
type Server struct {
	c      *conn
	schema *schema.Schema
	log    *log.Logger

	initialized bool
	shutdown    bool
	docs        map[string]*document
}

type document struct {
	uri     string
	version int
	text    string
	kind    check.Kind
}

// New returns a server that reads from r and writes to w. Logs go to logw
// (stderr in the binary); nil discards them.
func New(r io.Reader, w io.Writer, logw io.Writer) *Server {
	if logw == nil {
		logw = io.Discard
	}
	return &Server{
		c:      newConn(r, w),
		schema: schema.Default(),
		log:    log.New(logw, Name+": ", log.LstdFlags),
		docs:   map[string]*document{},
	}
}

// errExit is returned by Run after the exit notification.
var errExit = errors.New("exit")

// Run serves until the client sends exit or closes the stream. It returns
// nil after a clean shutdown + exit, and an error otherwise (the binary
// exits with 1, as the LSP spec asks).
func (s *Server) Run() error {
	for {
		body, err := s.c.read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				if s.shutdown {
					return nil
				}
				return errors.New("stream closed before shutdown")
			}
			return err
		}
		var m message
		if err := json.Unmarshal(body, &m); err != nil {
			s.reply(nil, nil, &rpcError{Code: codeParseError, Message: err.Error()})
			continue
		}
		if err := s.handle(&m); err != nil {
			if errors.Is(err, errExit) {
				if s.shutdown {
					return nil
				}
				return errors.New("exit before shutdown")
			}
			return err
		}
	}
}

func (s *Server) reply(id *json.RawMessage, result any, e *rpcError) {
	if id == nil {
		null := json.RawMessage("null")
		id = &null
	}
	m := &message{ID: id, Error: e}
	if e == nil {
		if result == nil {
			result = json.RawMessage("null")
		}
		m.Result = result
	}
	if err := s.c.write(m); err != nil {
		s.log.Printf("write: %v", err)
	}
}

func (s *Server) notify(method string, params any) {
	b, err := json.Marshal(params)
	if err != nil {
		s.log.Printf("marshal %s: %v", method, err)
		return
	}
	if err := s.c.write(&message{Method: method, Params: b}); err != nil {
		s.log.Printf("write: %v", err)
	}
}

func (s *Server) handle(m *message) error {
	isRequest := m.ID != nil
	if m.Method == "" {
		return nil // a response to a request we never send
	}
	if m.Method == "exit" {
		return errExit
	}
	if !s.initialized && m.Method != "initialize" {
		if isRequest {
			s.reply(m.ID, nil, &rpcError{Code: codeServerNotInitialized, Message: "server not initialized"})
		}
		return nil
	}
	if s.shutdown && isRequest {
		s.reply(m.ID, nil, &rpcError{Code: codeInvalidRequest, Message: "server is shutting down"})
		return nil
	}

	var result any
	var err error
	switch m.Method {
	case "initialize":
		result = s.initialize()
	case "initialized", "$/cancelRequest", "$/setTrace", "textDocument/didSave",
		"workspace/didChangeConfiguration", "workspace/didChangeWatchedFiles":
	case "shutdown":
		s.shutdown = true
	case "textDocument/didOpen":
		err = s.didOpen(m.Params)
	case "textDocument/didChange":
		err = s.didChange(m.Params)
	case "textDocument/didClose":
		err = s.didClose(m.Params)
	default:
		if isRequest {
			s.reply(m.ID, nil, &rpcError{Code: codeMethodNotFound, Message: "method not found: " + m.Method})
		}
		return nil
	}
	if !isRequest {
		if err != nil {
			s.log.Printf("%s: %v", m.Method, err)
		}
		return nil
	}
	if err != nil {
		s.reply(m.ID, nil, &rpcError{Code: codeInvalidParams, Message: err.Error()})
	} else {
		s.reply(m.ID, result, nil)
	}
	return nil
}

func (s *Server) initialize() any {
	s.initialized = true
	return map[string]any{
		"capabilities": map[string]any{
			// Full sync: files are small and the parsers are fast.
			"textDocumentSync": map[string]any{"openClose": true, "change": 1},
		},
		"serverInfo": map[string]any{"name": Name, "version": Version},
	}
}

type textDocumentItem struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
	Text    string `json:"text"`
}

type versionedDoc struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

func (s *Server) didOpen(raw json.RawMessage) error {
	var p struct {
		TextDocument textDocumentItem `json:"textDocument"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	td := p.TextDocument
	d := &document{uri: td.URI, version: td.Version, text: td.Text, kind: check.KindOf(uriPath(td.URI))}
	s.docs[td.URI] = d
	s.publish(d)
	return nil
}

func (s *Server) didChange(raw json.RawMessage) error {
	var p struct {
		TextDocument   versionedDoc `json:"textDocument"`
		ContentChanges []struct {
			Range *Range `json:"range"`
			Text  string `json:"text"`
		} `json:"contentChanges"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	d := s.docs[p.TextDocument.URI]
	if d == nil {
		return fmt.Errorf("document not open: %s", p.TextDocument.URI)
	}
	for _, ch := range p.ContentChanges {
		if ch.Range != nil {
			return errors.New("incremental changes are not supported (server asked for full sync)")
		}
		d.text = ch.Text
	}
	d.version = p.TextDocument.Version
	s.publish(d)
	return nil
}

func (s *Server) didClose(raw json.RawMessage) error {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	delete(s.docs, p.TextDocument.URI)
	s.notify("textDocument/publishDiagnostics", map[string]any{
		"uri": p.TextDocument.URI, "diagnostics": []any{},
	})
	return nil
}

// Diagnostic is the LSP diagnostic shape.
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Code     string `json:"code,omitempty"`
	Source   string `json:"source"`
	Message  string `json:"message"`
}

func (s *Server) publish(d *document) {
	ds := s.diagnostics(d)
	s.notify("textDocument/publishDiagnostics", map[string]any{
		"uri": d.uri, "version": d.version, "diagnostics": ds,
	})
}

func (s *Server) diagnostics(d *document) []Diagnostic {
	out := []Diagnostic{}
	if d.kind == check.KindNone {
		return out
	}
	ls := lines(d.text)
	for _, x := range check.Text(d.kind, d.text, s.schema) {
		out = append(out, Diagnostic{
			Range:    toRange(ls, x.Range),
			Severity: severity(x.Severity),
			Code:     x.Code,
			Source:   "katafighter",
			Message:  x.Message,
		})
	}
	return out
}

// severity maps diag severities to LSP ones; both use 1 = Error .. 4 = Hint.
func severity(s diag.Severity) int {
	if s < diag.Error || s > diag.Hint {
		return int(diag.Error)
	}
	return int(s)
}

// uriPath returns the path part of a URI, which is all KindOf needs.
func uriPath(uri string) string {
	if u, err := url.Parse(uri); err == nil && u.Path != "" {
		return u.Path
	}
	return path.Base(uri)
}
