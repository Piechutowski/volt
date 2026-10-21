package lsp

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/Piechutowski/volt/internal/corpus"
)

func keystrokeSpec() corpus.Spec {
	spec := corpus.Spec{Tables: 200, Columns: 12, Single: true}
	if n, err := strconv.Atoi(os.Getenv("VOLT_KEYSTROKE_TABLES")); err == nil && n > 0 {
		spec.Tables = n
	}
	if n, err := strconv.Atoi(os.Getenv("VOLT_KEYSTROKE_COLUMNS")); err == nil && n >= 3 {
		spec.Columns = n
	}
	return spec
}

func BenchmarkKeystroke(b *testing.B) {
	root := b.TempDir()
	spec := keystrokeSpec()
	if err := corpus.Write(root, spec); err != nil {
		b.Fatal(err)
	}
	path := filepath.Join(root, "schema.volt")
	text := corpus.Files(spec)["schema.volt"]
	anchor := "c002 string\n"
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
		edited := strings.Replace(text, anchor, "c002 string [note: '"+strings.Repeat("x", 1+i%5)+"']\n", 1)
		if err := s.didChange(ctx, &protocol.DidChangeTextDocumentParams{
			TextDocument:   protocol.VersionedTextDocumentIdentifier{TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri}, Version: protocol.Integer(i + 2)},
			ContentChanges: []any{protocol.TextDocumentContentChangeEventWhole{Text: edited}},
		}); err != nil {
			b.Fatal(err)
		}
		<-published
	}
}

func BenchmarkDocumentUpdateLocal(b *testing.B) {
	root := b.TempDir()
	spec := keystrokeSpec()
	if err := corpus.Write(root, spec); err != nil {
		b.Fatal(err)
	}
	path := filepath.Join(root, "schema.volt")
	text := corpus.Files(spec)["schema.volt"]
	anchor := "c002 string\n"
	d := &Document{URI: "file://" + path}
	d.UpdateLocal(text)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		edited := strings.Replace(text, anchor, "c002 string [note: '"+strings.Repeat("x", 1+i%5)+"']\n", 1)
		d.UpdateLocal(edited)
	}
}
