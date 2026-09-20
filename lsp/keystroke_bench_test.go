package lsp

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/Piechutowski/volt/internal/corpus"
)

// BenchmarkKeystroke is the editor's whole path in process: one
// keystroke inside one table of a one-file project, from didChange to
// the publish of its diagnostics, with the debounce zeroed. The stdio
// replay of docs/editor.md §8 measures the same path with the
// transport; this one is repeatable enough to compare commits.
func BenchmarkKeystroke(b *testing.B) {
	root := b.TempDir()
	spec := corpus.Spec{Tables: 200, Columns: 12, Single: true}
	if err := corpus.Write(root, spec); err != nil {
		b.Fatal(err)
	}
	path := filepath.Join(root, "schema.volt")
	text := corpus.Files(spec)["schema.volt"]
	anchor := "c002 text [not null]"
	if !strings.Contains(text, anchor) {
		b.Fatal("anchor")
	}
	s := NewServer()
	s.debounce = 0
	published := make(chan struct{}, 8)
	ctx := &glsp.Context{Notify: func(method string, params any) {
		if method == protocol.ServerTextDocumentPublishDiagnostics {
			published <- struct{}{}
		}
	}}
	uri := "file://" + path
	if err := s.didOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: uri, LanguageID: "volt", Version: 1, Text: text}}); err != nil {
		b.Fatal(err)
	}
	<-published
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		edited := strings.Replace(text, anchor, anchor[:len(anchor)-1]+", note: '"+strings.Repeat("x", 1+i%5)+"']", 1)
		if err := s.didChange(ctx, &protocol.DidChangeTextDocumentParams{
			TextDocument:   protocol.VersionedTextDocumentIdentifier{TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri}, Version: protocol.Integer(i + 2)},
			ContentChanges: []any{protocol.TextDocumentContentChangeEventWhole{Text: edited}},
		}); err != nil {
			b.Fatal(err)
		}
		<-published
	}
}

// BenchmarkDocumentUpdateLocal is the document's own front end alone,
// on the same keystroke: what the handler goroutine pays before the
// background run is even kicked.
func BenchmarkDocumentUpdateLocal(b *testing.B) {
	root := b.TempDir()
	spec := corpus.Spec{Tables: 200, Columns: 12, Single: true}
	if err := corpus.Write(root, spec); err != nil {
		b.Fatal(err)
	}
	path := filepath.Join(root, "schema.volt")
	text := corpus.Files(spec)["schema.volt"]
	anchor := "c002 text [not null]"
	d := &Document{URI: "file://" + path}
	d.UpdateLocal(text)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		edited := strings.Replace(text, anchor, anchor[:len(anchor)-1]+", note: '"+strings.Repeat("x", 1+i%5)+"']", 1)
		d.UpdateLocal(edited)
	}
}
