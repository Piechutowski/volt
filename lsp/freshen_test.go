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

// TestGoFileChangeRefreshesInBackground: a Go file rewritten on disk
// without a notification (gopls renamed the function) is caught up on
// by the next request, through one background analysis of the project
// rather than a project pass per request on the handler goroutine
// (D79, D103): the requests in the window answer from what they
// adopted, the fresh result is published once and adopted after.
func TestGoFileChangeRefreshesInBackground(t *testing.T) {
	root := goRefProject(t)
	schema := filepath.Join(root, "db", "schema.volt")
	uri := "file://" + schema
	s := NewServer()
	s.debounce = 20 * time.Millisecond
	var mu sync.Mutex
	published := 0
	ctx := &glsp.Context{Notify: func(method string, params any) {
		if method == protocol.ServerTextDocumentPublishDiagnostics && params.(protocol.PublishDiagnosticsParams).URI == uri {
			mu.Lock()
			published++
			mu.Unlock()
		}
	}}
	publishes := func() int {
		mu.Lock()
		defer mu.Unlock()
		return published
	}
	waitPublishes := func(n int) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for publishes() < n {
			if time.Now().After(deadline) {
				t.Fatalf("%d publishes, want %d", publishes(), n)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	if err := s.didOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: uri, Version: 1, Text: goRefSchema}}); err != nil {
		t.Fatal(err)
	}
	waitPublishes(1)
	session := s.sessionFor(root)
	loads := func() int {
		st := session.Stats()
		return st.FilesParsed + st.FilesReused
	}
	unit := loads() // the files one project pass reads
	hover := protocol.Position{Line: 7, Character: 3}
	if h, _ := s.hover(ctx, &protocol.HoverParams{TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: uri}, Position: hover}}); h == nil {
		t.Fatal("precondition: the Go reference hovers before the rename")
	}

	// gopls renames the function; no notification arrives.
	goFile := filepath.Join(root, "db", "checks.go")
	src, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(goFile, []byte(strings.ReplaceAll(string(src), "EmailValid", "AddressValid")), 0o644); err != nil {
		t.Fatal(err)
	}
	before := loads()
	for i := 0; i < 5; i++ {
		s.hover(ctx, &protocol.HoverParams{TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: uri}, Position: hover}})
	}
	if passes := (loads() - before) / unit; passes != 0 {
		t.Fatalf("%d project passes ran on the requests' goroutine", passes)
	}
	waitPublishes(2)
	time.Sleep(3 * s.debounce) // a kick during the run costs one more run at most
	if passes := (loads() - before) / unit; passes < 1 || passes > 2 {
		t.Fatalf("%d project passes after five requests, want one or two", passes)
	}
	doc := s.docGet(uri)
	if doc.GoFilesChanged() {
		t.Fatal("the adopted analysis still predates the Go file change")
	}
	if loc := doc.Definition(posOf(t, goRefSchema, "EmailValid", 0)); loc != nil {
		t.Errorf("definition still resolves to the renamed function: %v", loc)
	}
	if publishes() > 3 {
		t.Errorf("%d publishes for one Go file change", publishes())
	}
}
