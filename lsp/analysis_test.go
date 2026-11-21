package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lsp/protocol"
)

func TestServerAnalyzesInBackground(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module bg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "app", "routes.volt")
	broken := "package app\n\nscope /api {\n\tget /users Users.index\n}\n"
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewServer()
	s.debounce = 0
	var mu sync.Mutex
	var published []protocol.PublishDiagnosticsParams
	ctx := &protocol.Context{Notify: func(method string, params any) {
		if method != protocol.TextDocumentPublishDiagnostics {
			return
		}
		mu.Lock()
		published = append(published, params.(protocol.PublishDiagnosticsParams))
		mu.Unlock()
	}}
	uri := "file://" + path
	waitFor := func(what string, want func(protocol.PublishDiagnosticsParams) bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			mu.Lock()
			for _, p := range published {
				if p.URI == uri && want(p) {
					mu.Unlock()
					return
				}
			}
			mu.Unlock()
			time.Sleep(5 * time.Millisecond)
		}
		mu.Lock()
		defer mu.Unlock()
		t.Fatalf("no publication %s; got %+v", what, published)
	}

	if err := s.didOpen(ctx, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: uri, Text: broken},
	}); err != nil {
		t.Fatal(err)
	}
	waitFor("with the handler error", func(p protocol.PublishDiagnosticsParams) bool {
		return len(p.Diagnostics) == 1 && strings.Contains(p.Diagnostics[0].Message, "spec/handler-references")
	})

	fixed := strings.Replace(broken, "Users.index", "Users.Index", 1)
	if err := s.didChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument:   protocol.VersionedTextDocumentIdentifier{TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri}},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{{Text: fixed}},
	}); err != nil {
		t.Fatal(err)
	}
	waitFor("clean after the fix", func(p protocol.PublishDiagnosticsParams) bool { return len(p.Diagnostics) == 0 })

	doc := s.docGet(uri)
	if doc.vindex == nil || doc.vpkg == nil || doc.vpkg.Path != "app" {
		t.Fatalf("the document did not adopt the analysis: vindex=%v vpkg=%v", doc.vindex != nil, doc.vpkg)
	}
	if len(doc.Diags) != 0 {
		t.Fatalf("adopted diagnostics: %v", doc.Diags)
	}
	if st := s.sessionFor(root).Stats(); st.PackagesChecked < 2 || st.FilesParsed < 2 {
		t.Fatalf("session did not carry the work: %+v", st)
	}
}

func TestProjectVetSpeaksInTheFilesThatCheckClean(t *testing.T) {
	const clean = "package db\n\ntable posts {\n\tid int [pk]\n\tindexes {\n\t}\n}\n"
	const broken = "package db\n\ntable users {\n\tid int [pk]\n\tid int\n\tindexes {\n\t}\n}\n"
	root := voltProject(t, map[string]string{"go.mod": "module v\n", "db/clean.volt": clean, "db/broken.volt": broken})
	res := projectAnalyze(root, nil, nil, &voltIndexMemo{})
	if res == nil {
		t.Fatal("the project does not load")
	}
	vetIn := func(name string) (vets, errs int) {
		for _, dg := range res.docDiags(filepath.Join(root, "db", name)) {
			if strings.HasPrefix(dg.Code, "vet/") {
				vets++
			} else if dg.Severity == diag.Error {
				errs++
			}
		}
		return vets, errs
	}
	if vets, errs := vetIn("broken.volt"); vets != 0 || errs == 0 {
		t.Errorf("the broken file has %d vet warnings and %d errors, want none and some", vets, errs)
	}
	if vets, errs := vetIn("clean.volt"); vets == 0 || errs != 0 {
		t.Errorf("the clean file has %d vet warnings and %d errors, want some and none", vets, errs)
	}
}
