package lsp

import (
	"strings"
	"sync"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	glspserver "github.com/tliron/glsp/server"
)

// Version is stamped into the initialize response.
const Version = "0.1.0"

const serverName = "volt-lsp"

// Server owns the open documents and the protocol handler.
type Server struct {
	watchSupported bool // client accepts dynamic didChangeWatchedFiles registration
	mu             sync.Mutex
	docs           map[string]*Document

	handler protocol.Handler
}

// NewServer wires up the LSP handler.
func NewServer() *Server {
	s := &Server{docs: map[string]*Document{}}
	s.handler = protocol.Handler{
		Initialize:  s.initialize,
		Initialized: s.initialized,
		Shutdown:    func(*glsp.Context) error { return nil },
		SetTrace:    func(*glsp.Context, *protocol.SetTraceParams) error { return nil },

		TextDocumentDidOpen:   s.didOpen,
		TextDocumentDidChange: s.didChange,
		TextDocumentDidClose:  s.didClose,

		WorkspaceDidChangeWatchedFiles: s.watchedFilesChanged,

		TextDocumentCompletion:     s.completion,
		TextDocumentHover:          s.hover,
		TextDocumentDefinition:     s.definition,
		TextDocumentReferences:     s.references,
		TextDocumentRename:         s.rename,
		TextDocumentDocumentSymbol: s.documentSymbol,
	}
	return s
}

// RunStdio serves LSP over stdin/stdout until the client disconnects.
func (s *Server) RunStdio() error {
	return glspserver.NewServer(&s.handler, serverName, false).RunStdio()
}

func (s *Server) initialize(_ *glsp.Context, params *protocol.InitializeParams) (any, error) {
	if ws := params.Capabilities.Workspace; ws != nil && ws.DidChangeWatchedFiles != nil &&
		ws.DidChangeWatchedFiles.DynamicRegistration != nil {
		s.watchSupported = *ws.DidChangeWatchedFiles.DynamicRegistration
	}
	capabilities := s.handler.CreateServerCapabilities()

	syncKind := protocol.TextDocumentSyncKindFull
	capabilities.TextDocumentSync = &protocol.TextDocumentSyncOptions{
		OpenClose: boolPtr(true),
		Change:    &syncKind,
	}
	capabilities.CompletionProvider = &protocol.CompletionOptions{
		TriggerCharacters: []string{".", "~", "[", ",", ":", ">", "<", "-"},
	}

	version := Version
	return protocol.InitializeResult{
		Capabilities: capabilities,
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name:    serverName,
			Version: &version,
		},
	}, nil
}

// ---------------------------------------------------------------------------
// document sync

// initialized asks the client to report Go file changes: a Go reference
// (§V3.2, §V12.5) is checked against the package's Go files, so a
// gopls rename or a newly written function changes the truth of a
// .volt file the user is not editing. The request is fire-and-forget:
// a client without dynamic registration simply never notifies, and the
// per-request freshness check (freshen) still catches up on demand.
func (s *Server) initialized(ctx *glsp.Context, _ *protocol.InitializedParams) error {
	if !s.watchSupported {
		return nil
	}
	go ctx.Call(protocol.ServerClientRegisterCapability, protocol.RegistrationParams{
		Registrations: []protocol.Registration{{
			ID:     "volt-go-files",
			Method: string(protocol.MethodWorkspaceDidChangeWatchedFiles),
			RegisterOptions: protocol.DidChangeWatchedFilesRegistrationOptions{
				Watchers: []protocol.FileSystemWatcher{{GlobPattern: "**/*.go"}},
			},
		}},
	}, nil)
	return nil
}

// watchedFilesChanged re-checks every open document when a Go file was
// saved, created or deleted: D63 facts live in those files.
func (s *Server) watchedFilesChanged(ctx *glsp.Context, params *protocol.DidChangeWatchedFilesParams) error {
	goTouched := false
	for _, ev := range params.Changes {
		if strings.HasSuffix(string(ev.URI), ".go") {
			goTouched = true
		}
	}
	if goTouched {
		s.refreshOthers(ctx, "")
	}
	return nil
}

// freshen re-runs a document's analysis before answering a request when
// the Go files it depends on changed since — the fallback for clients
// that do not report file changes, and for the window between a save
// and its notification. The republished diagnostics follow.
func (s *Server) freshen(ctx *glsp.Context, doc *Document) {
	if doc != nil && doc.GoFilesChanged() {
		doc.Update(doc.Text)
		s.diagnosticsPublish(ctx, doc)
	}
}

func (s *Server) didOpen(ctx *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
	doc := &Document{URI: params.TextDocument.URI, Siblings: s.openTexts}
	s.mu.Lock()
	s.docs[params.TextDocument.URI] = doc
	s.mu.Unlock()
	doc.Update(params.TextDocument.Text)
	s.diagnosticsPublish(ctx, doc)
	s.refreshOthers(ctx, params.TextDocument.URI)
	return nil
}

// openTexts snapshots every open buffer keyed by on-disk path — the
// overlay the Volt project pass feeds to lang.LoadOverlay.
func (s *Server) openTexts() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]string, len(s.docs))
	for uri, doc := range s.docs {
		out[pathFromURI(uri)] = doc.Text
	}
	return out
}

func (s *Server) didChange(ctx *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	doc := s.docGet(params.TextDocument.URI)
	if doc == nil {
		return nil
	}
	for _, change := range params.ContentChanges {
		switch c := change.(type) {
		case protocol.TextDocumentContentChangeEventWhole:
			doc.Update(c.Text)
		case protocol.TextDocumentContentChangeEvent:
			if c.Range == nil {
				doc.Update(c.Text)
				continue
			}
			start := doc.FromLSP(c.Range.Start)
			end := doc.FromLSP(c.Range.End)
			doc.Update(doc.Text[:start] + c.Text + doc.Text[end:])
		}
	}
	s.diagnosticsPublish(ctx, doc)
	s.refreshOthers(ctx, params.TextDocument.URI)
	return nil
}

func (s *Server) didClose(ctx *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
	s.mu.Lock()
	delete(s.docs, params.TextDocument.URI)
	s.mu.Unlock()
	// clear stale squiggles
	ctx.Notify(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
		URI:         params.TextDocument.URI,
		Diagnostics: []protocol.Diagnostic{},
	})
	// the closed buffer reverts to its saved content for everyone else
	s.refreshOthers(ctx, params.TextDocument.URI)
	return nil
}

// refreshOthers re-checks and republishes every open document except
// the one that just changed. Open documents of a Volt project see each
// other through the overlay, so an edit in one file changes the truth
// of the diagnostics in all of them — without this, a fixed (or newly
// broken) sibling keeps its stale squiggles until it is touched.
func (s *Server) refreshOthers(ctx *glsp.Context, except string) {
	s.mu.Lock()
	others := make([]*Document, 0, len(s.docs))
	for uri, doc := range s.docs {
		if uri != except {
			others = append(others, doc)
		}
	}
	s.mu.Unlock()
	for _, doc := range others {
		doc.Update(doc.Text)
		s.diagnosticsPublish(ctx, doc)
	}
}

func (s *Server) diagnosticsPublish(ctx *glsp.Context, doc *Document) {
	ctx.Notify(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
		URI:         doc.URI,
		Diagnostics: doc.LSPDiagnostics(),
	})
}

func (s *Server) docGet(uri string) *Document {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.docs[uri]
}

// ---------------------------------------------------------------------------
// language features

func (s *Server) completion(ctx *glsp.Context, params *protocol.CompletionParams) (any, error) {
	doc := s.docGet(params.TextDocument.URI)
	s.freshen(ctx, doc)
	if doc == nil {
		return nil, nil
	}
	items := doc.Complete(params.Position)
	if items == nil {
		return nil, nil
	}
	return items, nil
}

func (s *Server) hover(ctx *glsp.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	doc := s.docGet(params.TextDocument.URI)
	s.freshen(ctx, doc)
	if doc == nil {
		return nil, nil
	}
	return doc.Hover(params.Position), nil
}

func (s *Server) definition(ctx *glsp.Context, params *protocol.DefinitionParams) (any, error) {
	doc := s.docGet(params.TextDocument.URI)
	s.freshen(ctx, doc)
	if doc == nil {
		return nil, nil
	}
	loc := doc.Definition(params.Position)
	if loc == nil {
		return nil, nil
	}
	return *loc, nil
}

func (s *Server) references(ctx *glsp.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	doc := s.docGet(params.TextDocument.URI)
	s.freshen(ctx, doc)
	if doc == nil {
		return nil, nil
	}
	return doc.References(params.Position, params.Context.IncludeDeclaration), nil
}

func (s *Server) rename(ctx *glsp.Context, params *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
	doc := s.docGet(params.TextDocument.URI)
	s.freshen(ctx, doc)
	if doc == nil {
		return nil, nil
	}
	return doc.Rename(params.Position, params.NewName)
}

func (s *Server) documentSymbol(_ *glsp.Context, params *protocol.DocumentSymbolParams) (any, error) {
	doc := s.docGet(params.TextDocument.URI)
	if doc == nil {
		return nil, nil
	}
	return doc.DocumentSymbols(), nil
}

func boolPtr(b bool) *bool { return &b }
