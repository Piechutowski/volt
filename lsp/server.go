package lsp

import (
	"strings"
	"sync"
	"time"

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
	// texts mirrors every open document's text by on-disk path, under
	// mu, for the background analysis to snapshot without touching a
	// Document (D79).
	texts map[string]string
	// versions is each open document's version as the client last
	// numbered it, kept beside texts under mu so a publish can say
	// which text it judged (D105).
	versions map[string]int32
	// analyses is the background analysis of each project root.
	analyses map[string]*analysis
	// debounce is how long edits must be quiet before a run.
	debounce time.Duration

	handler protocol.Handler
}

// NewServer wires up the LSP handler.
func NewServer() *Server {
	s := &Server{
		docs:     map[string]*Document{},
		texts:    map[string]string{},
		versions: map[string]int32{},
		analyses: map[string]*analysis{},
		debounce: analysisDebounce,
	}
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

	// Incremental: the client sends the keystroke, not the file, and
	// didChange applies each range to the text as it then stands (D105).
	syncKind := protocol.TextDocumentSyncKindIncremental
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
		s.analysisKickAll(ctx)
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
	doc := &Document{URI: params.TextDocument.URI, Siblings: s.openTexts, Session: s.sessionFor}
	s.mu.Lock()
	s.docs[params.TextDocument.URI] = doc
	s.mu.Unlock()
	s.edited(ctx, doc, params.TextDocument.Text, params.TextDocument.Version)
	return nil
}

// edited records a document's new text and its version: its own front
// end now, and the project's analysis once the edits settle (D79).
// Open documents of a project see each other through the overlay, so
// one edit changes the truth of every one of them — the analysis
// publishes to all. A document outside any project has its verdict
// right away.
func (s *Server) edited(ctx *glsp.Context, doc *Document, text string, version int32) {
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

// openTexts snapshots every open buffer keyed by on-disk path — the
// overlay the Volt project pass feeds to the loader.
func (s *Server) openTexts() map[string]string {
	texts, _ := s.openSnapshot()
	return texts
}

// openSnapshot copies every open buffer's text and version under one
// hold of mu: the overlay a run analyzes and the versions its publish
// stamps are of one and the same moment, so a client that drops a
// publish whose version is not its buffer's drops exactly the stale
// ones (D105).
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

func (s *Server) didChange(ctx *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	doc := s.docGet(params.TextDocument.URI)
	if doc == nil {
		return nil
	}
	text := doc.Text
	for _, change := range params.ContentChanges {
		switch c := change.(type) {
		case protocol.TextDocumentContentChangeEventWhole:
			text = c.Text
		case protocol.TextDocumentContentChangeEvent:
			if c.Range == nil {
				text = c.Text
				continue
			}
			doc.textSet(text)
			start := doc.FromLSP(c.Range.Start)
			end := doc.FromLSP(c.Range.End)
			text = text[:start] + c.Text + text[end:]
		}
	}
	s.edited(ctx, doc, text, params.TextDocument.Version)
	return nil
}

func (s *Server) didClose(ctx *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
	path := pathFromURI(params.TextDocument.URI)
	s.mu.Lock()
	delete(s.docs, params.TextDocument.URI)
	delete(s.texts, path)
	delete(s.versions, path)
	s.mu.Unlock()
	// clear stale squiggles
	ctx.Notify(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
		URI:         params.TextDocument.URI,
		Diagnostics: []protocol.Diagnostic{},
	})
	// the closed buffer reverts to its saved content for everyone else
	if root, ok := projectRootOf(path); ok {
		s.analysisKick(ctx, root)
	}
	return nil
}

// analysisKickAll re-analyzes every project with an open document.
func (s *Server) analysisKickAll(ctx *glsp.Context) {
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

// diagnosticsPublish sends a document its own diagnostics, stamped
// with the version of the text they are positioned in: the current one.
func (s *Server) diagnosticsPublish(ctx *glsp.Context, doc *Document) {
	s.mu.Lock()
	version := s.versions[pathFromURI(doc.URI)]
	s.mu.Unlock()
	ctx.Notify(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
		URI:         doc.URI,
		Version:     versionStamp(version),
		Diagnostics: doc.LSPDiagnostics(),
	})
}

// versionStamp is a document version as a publish carries it.
func versionStamp(v int32) *protocol.UInteger {
	u := protocol.UInteger(v)
	return &u
}

// docGet returns the open document, carrying the newest analysis of
// its project (adopt), so every request reads the current truth.
func (s *Server) docGet(uri string) *Document {
	s.mu.Lock()
	doc := s.docs[uri]
	s.mu.Unlock()
	s.adopt(doc)
	return doc
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
