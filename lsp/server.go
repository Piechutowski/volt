package lsp

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Piechutowski/volt/lsp/protocol"
)

// Version is stamped into the initialize response.
const Version = "0.1.0"

const serverName = "volt-lsp"

// Server owns the open documents and answers the editor's methods.
type Server struct {
	watchSupported bool
	mu             sync.Mutex
	docs           map[string]*Document
	texts          map[string]string
	versions       map[string]int32
	analyses       map[string]*analysis
	debounce       time.Duration
}

// NewServer makes a server with no open documents.
func NewServer() *Server {
	return &Server{
		docs:     map[string]*Document{},
		texts:    map[string]string{},
		versions: map[string]int32{},
		analyses: map[string]*analysis{},
		debounce: analysisDebounce,
	}
}

func (s *Server) methods() map[string]protocol.Handler {
	return map[string]protocol.Handler{
		"initialize":                      protocol.Request(s.initialize),
		"initialized":                     protocol.Notification(s.initialized),
		"textDocument/didOpen":            protocol.Notification(s.didOpen),
		"textDocument/didChange":          protocol.Notification(s.didChange),
		"textDocument/didClose":           protocol.Notification(s.didClose),
		"workspace/didChangeWatchedFiles": protocol.Notification(s.watchedFilesChanged),
		"textDocument/completion":         protocol.Request(s.completion),
		"textDocument/hover":              protocol.Request(s.hover),
		"textDocument/definition":         protocol.Request(s.definition),
		"textDocument/references":         protocol.Request(s.references),
		"textDocument/rename":             protocol.Request(s.rename),
		"textDocument/documentSymbol":     protocol.Request(s.documentSymbol),
	}
}

// RunStdio serves LSP over stdin and stdout until the editor sends exit or
// closes stdin, then exits the process with code 1 when no shutdown came
// first (LSP Exit Notification).
func (s *Server) RunStdio() error {
	code, err := protocol.Serve(os.Stdin, os.Stdout, s.methods())
	if err != nil {
		return err
	}
	if code != 0 {
		os.Exit(code)
	}
	return nil
}

func (s *Server) initialize(_ *protocol.Context, params *protocol.InitializeParams) (protocol.InitializeResult, error) {
	s.watchSupported = params.Capabilities.Workspace.DidChangeWatchedFiles.DynamicRegistration
	return protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: &protocol.TextDocumentSyncOptions{
				OpenClose: true,
				Change:    protocol.TextDocumentSyncKindIncremental,
			},
			CompletionProvider: &protocol.CompletionOptions{
				TriggerCharacters: []string{".", "~", "[", ",", ":", ">", "<", "-"},
			},
			HoverProvider:          true,
			DefinitionProvider:     true,
			ReferencesProvider:     true,
			RenameProvider:         true,
			DocumentSymbolProvider: true,
		},
		ServerInfo: &protocol.ServerInfo{Name: serverName, Version: Version},
	}, nil
}

func (s *Server) initialized(ctx *protocol.Context, _ *protocol.InitializedParams) error {
	if !s.watchSupported {
		return nil
	}
	ctx.Request("client/registerCapability", protocol.RegistrationParams{
		Registrations: []protocol.Registration{{
			ID:     "volt-go-files",
			Method: "workspace/didChangeWatchedFiles",
			RegisterOptions: protocol.DidChangeWatchedFilesRegistrationOptions{
				Watchers: []protocol.FileSystemWatcher{{GlobPattern: "**/*.go"}},
			},
		}},
	})
	return nil
}

func (s *Server) watchedFilesChanged(ctx *protocol.Context, params *protocol.DidChangeWatchedFilesParams) error {
	goTouched := false
	for _, ev := range params.Changes {
		if strings.HasSuffix(string(ev.URI), ".go") {
			goTouched = true
		}
	}
	if goTouched {
		s.analysisKickAll(ctx)
	}
	return nil
}

func (s *Server) freshen(ctx *protocol.Context, doc *Document) {
	if doc == nil || !doc.GoFilesChanged() {
		return
	}
	if root, ok := projectRootOf(pathFromURI(doc.URI)); ok {
		s.analysisKick(ctx, root)
	}
}

func (s *Server) didOpen(ctx *protocol.Context, params *protocol.DidOpenTextDocumentParams) error {
	doc := &Document{URI: params.TextDocument.URI, Siblings: s.openTexts, Session: s.sessionFor}
	s.mu.Lock()
	s.docs[params.TextDocument.URI] = doc
	s.mu.Unlock()
	s.edited(ctx, doc, params.TextDocument.Text, params.TextDocument.Version)
	return nil
}

func (s *Server) edited(ctx *protocol.Context, doc *Document, text string, version int32) {
	path := pathFromURI(doc.URI)
	s.mu.Lock()
	s.texts[path] = text
	s.versions[path] = version
	s.mu.Unlock()
	doc.UpdateLocal(text)
	if root, ok := projectRootOf(path); ok {
		s.analysisKick(ctx, root)
		return
	}
	doc.vetLocal()
	s.diagnosticsPublish(ctx, doc)
}

func (s *Server) openTexts() map[string]string {
	texts, _ := s.openSnapshot()
	return texts
}

func (s *Server) openSnapshot() (map[string]string, map[string]int32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	texts := make(map[string]string, len(s.texts))
	for path, text := range s.texts {
		texts[path] = text
	}
	versions := make(map[string]int32, len(s.versions))
	for path, v := range s.versions {
		versions[path] = v
	}
	return texts, versions
}

func (s *Server) didChange(ctx *protocol.Context, params *protocol.DidChangeTextDocumentParams) error {
	doc := s.docGet(params.TextDocument.URI)
	if doc == nil {
		return nil
	}
	text := doc.Text
	for _, change := range params.ContentChanges {
		if change.Range == nil {
			text = change.Text
			continue
		}
		doc.textSet(text)
		start := doc.FromLSP(change.Range.Start)
		end := doc.FromLSP(change.Range.End)
		text = text[:start] + change.Text + text[end:]
	}
	s.edited(ctx, doc, text, params.TextDocument.Version)
	return nil
}

func (s *Server) didClose(ctx *protocol.Context, params *protocol.DidCloseTextDocumentParams) error {
	path := pathFromURI(params.TextDocument.URI)
	s.mu.Lock()
	delete(s.docs, params.TextDocument.URI)
	delete(s.texts, path)
	delete(s.versions, path)
	s.mu.Unlock()
	ctx.Notify(protocol.TextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
		URI:         params.TextDocument.URI,
		Diagnostics: []protocol.Diagnostic{},
	})
	if root, ok := projectRootOf(path); ok {
		s.analysisKick(ctx, root)
	}
	return nil
}

func (s *Server) analysisKickAll(ctx *protocol.Context) {
	s.mu.Lock()
	roots := map[string]bool{}
	for uri := range s.docs {
		if root, ok := projectRootOf(pathFromURI(uri)); ok {
			roots[root] = true
		}
	}
	s.mu.Unlock()
	for root := range roots {
		s.analysisKick(ctx, root)
	}
}

func (s *Server) diagnosticsPublish(ctx *protocol.Context, doc *Document) {
	s.mu.Lock()
	version := s.versions[pathFromURI(doc.URI)]
	s.mu.Unlock()
	ctx.Notify(protocol.TextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
		URI:         doc.URI,
		Version:     versionStamp(version),
		Diagnostics: doc.LSPDiagnostics(),
	})
}

func versionStamp(v int32) *protocol.Integer {
	return &v
}

func (s *Server) docGet(uri string) *Document {
	s.mu.Lock()
	doc := s.docs[uri]
	s.mu.Unlock()
	s.adopt(doc)
	return doc
}

func (s *Server) completion(ctx *protocol.Context, params *protocol.CompletionParams) ([]protocol.CompletionItem, error) {
	doc := s.docGet(params.TextDocument.URI)
	s.freshen(ctx, doc)
	if doc == nil {
		return nil, nil
	}
	return doc.Complete(params.Position), nil
}

func (s *Server) hover(ctx *protocol.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	doc := s.docGet(params.TextDocument.URI)
	s.freshen(ctx, doc)
	if doc == nil {
		return nil, nil
	}
	return doc.Hover(params.Position), nil
}

func (s *Server) definition(ctx *protocol.Context, params *protocol.DefinitionParams) (*protocol.Location, error) {
	doc := s.docGet(params.TextDocument.URI)
	s.freshen(ctx, doc)
	if doc == nil {
		return nil, nil
	}
	return doc.Definition(params.Position), nil
}

func (s *Server) references(ctx *protocol.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	doc := s.docGet(params.TextDocument.URI)
	s.freshen(ctx, doc)
	if doc == nil {
		return nil, nil
	}
	return doc.References(params.Position, params.Context.IncludeDeclaration), nil
}

func (s *Server) rename(ctx *protocol.Context, params *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
	doc := s.docGet(params.TextDocument.URI)
	s.freshen(ctx, doc)
	if doc == nil {
		return nil, nil
	}
	return doc.Rename(params.Position, params.NewName)
}

func (s *Server) documentSymbol(_ *protocol.Context, params *protocol.DocumentSymbolParams) ([]protocol.DocumentSymbol, error) {
	doc := s.docGet(params.TextDocument.URI)
	if doc == nil {
		return nil, nil
	}
	return doc.DocumentSymbols(), nil
}
