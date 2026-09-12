package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// TestServerAnalyzesInBackground drives the server the way an editor
// does: open a project file with an error, get its diagnostic from the
// background analysis, fix it, get none — and find the document
// carrying the project's index when a request comes (D79).
func TestServerAnalyzesInBackground(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module bg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "app", "routes.volt")
	broken := "package app\n\nScope /api {\n\tget /users Users.index\n}\n"
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}

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
		return len(p.Diagnostics) == 1 && strings.Contains(p.Diagnostics[0].Message, "§V4.3")
	})

	fixed := strings.Replace(broken, "Users.index", "Users.Index", 1)
	if err := s.didChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument:   protocol.VersionedTextDocumentIdentifier{TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri}},
		ContentChanges: []any{protocol.TextDocumentContentChangeEventWhole{Text: fixed}},
	}); err != nil {
		t.Fatal(err)
	}
	waitFor("clean after the fix", func(p protocol.PublishDiagnosticsParams) bool { return len(p.Diagnostics) == 0 })

	// A request adopts the result: the document navigates the project.
	doc := s.docGet(uri)
	if doc.vindex == nil || doc.vpkg == nil || doc.vpkg.Path != "app" {
		t.Fatalf("the document did not adopt the analysis: vindex=%v vpkg=%v", doc.vindex != nil, doc.vpkg)
	}
	if len(doc.Diags) != 0 {
		t.Fatalf("adopted diagnostics: %v", doc.Diags)
	}
	// Two edits, two analyses of one package, through one session.
	if st := s.sessionFor(root).Stats(); st.PackagesChecked < 2 || st.FilesParsed < 2 {
		t.Fatalf("session did not carry the work: %+v", st)
	}
}
