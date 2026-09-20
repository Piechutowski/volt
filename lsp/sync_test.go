package lsp

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func rangeAt(l1, c1, l2, c2 int) *protocol.Range {
	return &protocol.Range{
		Start: protocol.Position{Line: protocol.UInteger(l1), Character: protocol.UInteger(c1)},
		End:   protocol.Position{Line: protocol.UInteger(l2), Character: protocol.UInteger(c2)},
	}
}

func ranged(r *protocol.Range, text string) any {
	return protocol.TextDocumentContentChangeEvent{Range: r, Text: text}
}

// TestIncrementalSync proves the server applies ranged changes the
// way the protocol defines them (D105): each item of one notification
// against the text the items before it produced, positions in UTF-16
// units, a CRLF newline deleted by a range that spans it, a range past
// the end of the text landing at its end, and the document's text and
// the server's mirror agreeing afterwards.
func TestIncrementalSync(t *testing.T) {
	s := NewServer()
	ctx := &glsp.Context{Notify: func(string, any) {}}
	change := func(uri string, version int32, items ...any) {
		t.Helper()
		if err := s.didChange(ctx, &protocol.DidChangeTextDocumentParams{
			TextDocument:   protocol.VersionedTextDocumentIdentifier{TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri}, Version: version},
			ContentChanges: items,
		}); err != nil {
			t.Fatal(err)
		}
	}
	expect := func(uri, want string, version int32) {
		t.Helper()
		doc := s.docGet(uri)
		if doc.Text != want {
			t.Fatalf("%s: document text\n--- got\n%q\n--- want\n%q", uri, doc.Text, want)
		}
		path := pathFromURI(uri)
		s.mu.Lock()
		mirror, v := s.texts[path], s.versions[path]
		s.mu.Unlock()
		if mirror != want {
			t.Fatalf("%s: the server's mirror differs from the document:\n%q", uri, mirror)
		}
		if v != version {
			t.Fatalf("%s: version %d, want %d", uri, v, version)
		}
	}

	uri := "file:///lone.dbml"
	if err := s.didOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: uri, Version: 1,
		Text: "Table users {\n  id int [pk]\n  name text\n}\n"}}); err != nil {
		t.Fatal(err)
	}
	// Two items: the second is positioned in the text after the first.
	change(uri, 2,
		ranged(rangeAt(1, 5, 1, 8), "integer"),
		ranged(rangeAt(2, 11, 2, 11), " [not null]"))
	expect(uri, "Table users {\n  id integer [pk]\n  name text [not null]\n}\n", 2)
	// A non-BMP character counts two units; the range after it says so.
	change(uri, 3, ranged(rangeAt(2, 7, 2, 11), "\"na😀\""))
	change(uri, 4, ranged(rangeAt(2, 13, 2, 13), "y"))
	expect(uri, "Table users {\n  id integer [pk]\n  name \"na😀\"y [not null]\n}\n", 4)
	// A whole-text item replaces everything, ranged items follow it.
	change(uri, 5, protocol.TextDocumentContentChangeEventWhole{Text: "Table t {\n  id int\n}\n"}, ranged(rangeAt(1, 8, 1, 8), " [pk]"))
	expect(uri, "Table t {\n  id int [pk]\n}\n", 5)

	crlf := "file:///crlf.dbml"
	if err := s.didOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: crlf, Version: 1,
		Text: "Table t {\r\n  id int [pk]\r\n  x int\r\n}\r\n"}}); err != nil {
		t.Fatal(err)
	}
	// The newline of line 1 deleted: from the end of its text to the
	// start of line 2.
	change(crlf, 2, ranged(rangeAt(1, 13, 2, 0), " "))
	expect(crlf, "Table t {\r\n  id int [pk]   x int\r\n}\r\n", 2)
	// A range past the end of the text lands at the end; a character
	// past the end of a line lands at the end of the line.
	change(crlf, 3, ranged(rangeAt(9, 0, 9, 0), "// end"), ranged(rangeAt(0, 40, 0, 40), " // head"))
	expect(crlf, "Table t { // head\r\n  id int [pk]   x int\r\n}\r\n// end", 3)
	// An empty text is a deletion.
	change(crlf, 4, ranged(rangeAt(0, 9, 0, 17), ""))
	expect(crlf, "Table t {\r\n  id int [pk]   x int\r\n}\r\n// end", 4)
}

// TestPublishCarriesVersion proves every publish names the version of
// the text its diagnostics are positioned in: the document's own
// verdict outside a project at once, the background run's from the
// snapshot it analyzed, a close's clearing publish none (D105).
func TestPublishCarriesVersion(t *testing.T) {
	var mu sync.Mutex
	var published []protocol.PublishDiagnosticsParams
	ctx := &glsp.Context{Notify: func(method string, params any) {
		if method != protocol.ServerTextDocumentPublishDiagnostics {
			return
		}
		mu.Lock()
		published = append(published, params.(protocol.PublishDiagnosticsParams))
		mu.Unlock()
	}}
	last := func(uri string) *protocol.PublishDiagnosticsParams {
		mu.Lock()
		defer mu.Unlock()
		for i := len(published) - 1; i >= 0; i-- {
			if published[i].URI == uri {
				return &published[i]
			}
		}
		return nil
	}
	version := func(p *protocol.PublishDiagnosticsParams) int {
		if p == nil || p.Version == nil {
			return -1
		}
		return int(*p.Version)
	}
	s := NewServer()
	s.debounce = 0

	lone := "file:///lone.dbml"
	if err := s.didOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: lone, Version: 7, Text: "Table t {\n  id int [pk]\n}\n"}}); err != nil {
		t.Fatal(err)
	}
	if v := version(last(lone)); v != 7 {
		t.Fatalf("the open's publish carries version %d, want 7", v)
	}
	if err := s.didChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument:   protocol.VersionedTextDocumentIdentifier{TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: lone}, Version: 9},
		ContentChanges: []any{ranged(rangeAt(1, 9, 1, 9), ", null")},
	}); err != nil {
		t.Fatal(err)
	}
	if v := version(last(lone)); v != 9 {
		t.Fatalf("the change's publish carries version %d, want 9", v)
	}

	root := voltProject(t, map[string]string{
		"go.mod":          "module lsptest\n",
		"app/routes.volt": "package app\n\nScope / {\n\tget / Home.Index\n}\n",
	})
	uri := "file://" + filepath.Join(root, "app", "routes.volt")
	waitVersion := func(want int) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if version(last(uri)) == want {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatalf("no publish with version %d; last %+v", want, last(uri))
	}
	if err := s.didOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: uri, Version: 3, Text: "package app\n\nScope / {\n\tget / Home.Index\n}\n"}}); err != nil {
		t.Fatal(err)
	}
	waitVersion(3)
	if err := s.didChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument:   protocol.VersionedTextDocumentIdentifier{TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri}, Version: 4},
		ContentChanges: []any{ranged(rangeAt(3, 17, 3, 17), "\n\tget /x Home.X")},
	}); err != nil {
		t.Fatal(err)
	}
	waitVersion(4)
	if err := s.didClose(ctx, &protocol.DidCloseTextDocumentParams{TextDocument: protocol.TextDocumentIdentifier{URI: uri}}); err != nil {
		t.Fatal(err)
	}
	if p := last(uri); p == nil || p.Version != nil || len(p.Diagnostics) != 0 {
		t.Fatalf("the close's publish: %+v", p)
	}
}
