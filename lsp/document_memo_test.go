package lsp

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/Piechutowski/volt/internal/corpus"
	"github.com/Piechutowski/volt/lang/diag"
)

func occsRender(ix *Index) string {
	var b strings.Builder
	for _, o := range ix.Occs {
		fmt.Fprintf(&b, "%v %s %s %s decl=%v\n", o.ID.Kind, o.ID.Container, o.ID.Name, o.Ident.Pos(), o.IsDecl)
	}
	return b.String()
}

func TestDocumentReusesItsFrontEnd(t *testing.T) {
	text := corpus.Files(corpus.Spec{Tables: 12, Columns: 6, Single: true})["schema.volt"]
	d := &Document{URI: "file:///schema.volt"}
	round := func(name, text string, parsed, checked, indexed int) {
		t.Helper()
		d.UpdateLocal(text)
		fresh := &Document{URI: d.URI}
		fresh.UpdateLocal(text)
		if a, b := diagsText(d.local), diagsText(fresh.local); a != b {
			t.Fatalf("%s: memoized verdict differs from a fresh one\n--- memo\n%s--- fresh\n%s", name, a, b)
		}
		if a, b := occsRender(d.Index), occsRender(fresh.Index); a != b {
			t.Fatalf("%s: memoized index differs from a fresh one\n--- memo\n%s--- fresh\n%s", name, a, b)
		}
		if d.parseStats.Parsed != parsed || d.schema.Misses != checked || d.index.Misses != indexed {
			t.Errorf("%s: parsed %d declarations, checked %d tables, indexed %d; want %d, %d, %d (reused %d, %d, %d)",
				name, d.parseStats.Parsed, d.schema.Misses, d.index.Misses, parsed, checked, indexed,
				d.parseStats.Reused, d.schema.Hits, d.index.Hits)
		}
	}
	decls := strings.Count(text, "\nTable ") + strings.Count(text, "\nType ") + strings.Count(text, "\nTablePartial ") +
		strings.Count(text, "\nScope ") + strings.Count(text, "\nPipeline ") + strings.Count(text, "\nSelect ") +
		strings.Count(text, "\nGroup ") + strings.Count(text, "\nPred ") + strings.Count(text, "\nView ") + 1
	round("first", text, decls, 12, 12)
	round("no change", text, 0, 0, 0)
	round("edit one table", strings.Replace(text, "c002 string\n", "c002 text [note: \"edited\"]\n", 1), 1, 1, 2)
	round("break the table", strings.Replace(text, "c002 string\n", "c002 text [null, null]\n", 1), 1, 1, 2)
	round("fix it", text, 1, 1, 2)
}

func diagsText(ds []diag.Diagnostic) string {
	var b strings.Builder
	for _, dg := range ds {
		b.WriteString(dg.String())
		b.WriteByte('\n')
	}
	return b.String()
}

func TestServerForeignFileVerdict(t *testing.T) {
	root := voltProject(t, map[string]string{
		"go.mod":          "module lsptest\n",
		"app/routes.volt": "package app\n\nScope / {\n\tget / Home.Index\n}\n",
	})
	path := filepath.Join(root, "app", "notes.txt")
	s := NewServer()
	s.debounce = 0
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
	uri := "file://" + path
	if err := s.didOpen(ctx, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: uri, Text: "Table t {\n  id int [pk]\n  Indexes {\n  }\n}\n"},
	}); err != nil {
		t.Fatal(err)
	}
	hasVet := func(ds []protocol.Diagnostic) bool {
		for _, dg := range ds {
			if dg.Code != nil && dg.Code.Value == "vet/emptybody" {
				return true
			}
		}
		return false
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		var found bool
		for _, p := range published {
			if p.URI == uri && hasVet(p.Diagnostics) {
				found = true
			}
		}
		mu.Unlock()
		if found {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no publication with the vet advice; got %+v", published)
		}
		time.Sleep(5 * time.Millisecond)
	}
	doc := s.docGet(uri)
	if doc.vindex != nil || doc.vpkg != nil {
		t.Fatal("a file the loader never read adopted a project package")
	}
	if !hasVet(doc.LSPDiagnostics()) {
		t.Fatalf("the adopted document lacks the vet advice: %v", doc.Diags)
	}
}
