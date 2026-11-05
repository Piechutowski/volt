// Package protocol is the language server's LSP 3.16 layer over stdio:
// JSON-RPC 2.0 framing and dispatch, the lifecycle, and the protocol types
// Volt reads or writes, written from the two specifications alone.
package protocol

import "encoding/json"

// TextDocumentPublishDiagnostics is the method of the notification that
// carries a document's diagnostics (LSP PublishDiagnostics Notification).
const TextDocumentPublishDiagnostics = "textDocument/publishDiagnostics"

// Integer is a signed 32-bit number (LSP Numbers).
type Integer = int32

// UInteger is an unsigned number below 2^31 (LSP Numbers).
type UInteger = uint32

// DocumentUri is a document's URI as the editor sends it (LSP URI).
type DocumentUri = string

// Position is a zero-based line and a zero-based UTF-16 offset in it (LSP
// Position).
type Position struct {
	Line      UInteger `json:"line"`
	Character UInteger `json:"character"`
}

// Range runs from Start up to, not including, End (LSP Range).
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location is a range in a document (LSP Location).
type Location struct {
	URI   DocumentUri `json:"uri"`
	Range Range       `json:"range"`
}

// DiagnosticSeverity grades a diagnostic (LSP Diagnostic).
type DiagnosticSeverity Integer

// The severities Volt publishes (LSP Diagnostic).
const (
	DiagnosticSeverityError   DiagnosticSeverity = 1
	DiagnosticSeverityWarning DiagnosticSeverity = 2
)

// Diagnostic is one problem at a range; Volt's codes are always strings (LSP
// Diagnostic).
type Diagnostic struct {
	Range    Range               `json:"range"`
	Severity *DiagnosticSeverity `json:"severity,omitempty"`
	Code     string              `json:"code,omitempty"`
	Source   *string             `json:"source,omitempty"`
	Message  string              `json:"message"`
}

// PublishDiagnosticsParams replaces every diagnostic of one document (LSP
// PublishDiagnostics Notification).
type PublishDiagnosticsParams struct {
	URI         DocumentUri  `json:"uri"`
	Version     *Integer     `json:"version,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// MarshalJSON writes no diagnostics as an empty list, since the field is
// required and an empty list is how a document's diagnostics are cleared
// (LSP PublishDiagnostics Notification).
func (p PublishDiagnosticsParams) MarshalJSON() ([]byte, error) {
	type plain PublishDiagnosticsParams
	if p.Diagnostics == nil {
		p.Diagnostics = []Diagnostic{}
	}
	return json.Marshal(plain(p))
}

// TextEdit replaces a range with new text (LSP TextEdit).
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// WorkspaceEdit holds the text edits of each document it changes (LSP
// WorkspaceEdit).
type WorkspaceEdit struct {
	Changes map[DocumentUri][]TextEdit `json:"changes,omitempty"`
}

// TextDocumentIdentifier names a document (LSP TextDocumentIdentifier).
type TextDocumentIdentifier struct {
	URI DocumentUri `json:"uri"`
}

// TextDocumentItem is a document as opened, text and all (LSP
// TextDocumentItem).
type TextDocumentItem struct {
	URI        DocumentUri `json:"uri"`
	LanguageID string      `json:"languageId"`
	Version    Integer     `json:"version"`
	Text       string      `json:"text"`
}

// VersionedTextDocumentIdentifier names a document at a version (LSP
// VersionedTextDocumentIdentifier).
type VersionedTextDocumentIdentifier struct {
	TextDocumentIdentifier
	Version Integer `json:"version"`
}

// TextDocumentPositionParams is a position in a document (LSP
// TextDocumentPositionParams).
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// MarkupKind is the format of a MarkupContent (LSP MarkupContent).
type MarkupKind string

// MarkupKindMarkdown is the one markup kind Volt writes (LSP MarkupContent).
const MarkupKindMarkdown MarkupKind = "markdown"

// MarkupContent is text in a markup kind (LSP MarkupContent).
type MarkupContent struct {
	Kind  MarkupKind `json:"kind"`
	Value string     `json:"value"`
}

// ClientCapabilities is the part of the editor's capabilities Volt reads
// (LSP Initialize Request).
type ClientCapabilities struct {
	Workspace struct {
		DidChangeWatchedFiles struct {
			DynamicRegistration bool `json:"dynamicRegistration"`
		} `json:"didChangeWatchedFiles"`
	} `json:"workspace"`
}

// InitializeParams is the editor's first request (LSP Initialize Request).
type InitializeParams struct {
	Capabilities ClientCapabilities `json:"capabilities"`
}

// TextDocumentSyncKind says how the editor sends changes (LSP Text Document
// Synchronization).
type TextDocumentSyncKind Integer

// TextDocumentSyncKindIncremental asks for each change as a range and its
// replacement (LSP Text Document Synchronization).
const TextDocumentSyncKindIncremental TextDocumentSyncKind = 2

// TextDocumentSyncOptions says which document notifications the server
// takes (LSP Text Document Synchronization).
type TextDocumentSyncOptions struct {
	OpenClose bool                 `json:"openClose,omitempty"`
	Change    TextDocumentSyncKind `json:"change,omitempty"`
}

// CompletionOptions is the server's completion capability (LSP Completion
// Request).
type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

// ServerCapabilities is what the server announces in its initialize reply
// (LSP Initialize Request).
type ServerCapabilities struct {
	TextDocumentSync       *TextDocumentSyncOptions `json:"textDocumentSync,omitempty"`
	CompletionProvider     *CompletionOptions       `json:"completionProvider,omitempty"`
	HoverProvider          bool                     `json:"hoverProvider,omitempty"`
	DefinitionProvider     bool                     `json:"definitionProvider,omitempty"`
	ReferencesProvider     bool                     `json:"referencesProvider,omitempty"`
	RenameProvider         bool                     `json:"renameProvider,omitempty"`
	DocumentSymbolProvider bool                     `json:"documentSymbolProvider,omitempty"`
}

// ServerInfo names the server in its initialize reply (LSP Initialize
// Request).
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// InitializeResult is the reply to initialize (LSP Initialize Request).
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   *ServerInfo        `json:"serverInfo,omitempty"`
}

// InitializedParams carries nothing (LSP Initialized Notification).
type InitializedParams struct{}

// DidOpenTextDocumentParams is a document the editor opened (LSP
// DidOpenTextDocument Notification).
type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// TextDocumentContentChangeEvent is one change: Text replaces Range, or the
// whole document when Range is nil (LSP DidChangeTextDocument Notification).
type TextDocumentContentChangeEvent struct {
	Range *Range `json:"range,omitempty"`
	Text  string `json:"text"`
}

// DidChangeTextDocumentParams is a document's changes, applied in order
// (LSP DidChangeTextDocument Notification).
type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

// DidCloseTextDocumentParams is a document the editor closed (LSP
// DidCloseTextDocument Notification).
type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// FileEvent is one watched file that changed (LSP DidChangeWatchedFiles
// Notification).
type FileEvent struct {
	URI DocumentUri `json:"uri"`
}

// DidChangeWatchedFilesParams is the watched files that changed (LSP
// DidChangeWatchedFiles Notification).
type DidChangeWatchedFilesParams struct {
	Changes []FileEvent `json:"changes"`
}

// FileSystemWatcher is a glob of files to watch (LSP DidChangeWatchedFiles
// Notification).
type FileSystemWatcher struct {
	GlobPattern string `json:"globPattern"`
}

// DidChangeWatchedFilesRegistrationOptions is the files a watch registers
// (LSP DidChangeWatchedFiles Notification).
type DidChangeWatchedFilesRegistrationOptions struct {
	Watchers []FileSystemWatcher `json:"watchers"`
}

// Registration is one capability the server registers (LSP Register
// Capability).
type Registration struct {
	ID              string `json:"id"`
	Method          string `json:"method"`
	RegisterOptions any    `json:"registerOptions,omitempty"`
}

// RegistrationParams is the capabilities registered at once (LSP Register
// Capability).
type RegistrationParams struct {
	Registrations []Registration `json:"registrations"`
}

// CompletionParams is where completion is asked for (LSP Completion
// Request).
type CompletionParams struct {
	TextDocumentPositionParams
}

// CompletionItemKind is the icon an editor shows for a completion (LSP
// Completion Request).
type CompletionItemKind Integer

// The completion kinds Volt writes (LSP Completion Request).
const (
	CompletionItemKindFunction  CompletionItemKind = 3
	CompletionItemKindField     CompletionItemKind = 5
	CompletionItemKindClass     CompletionItemKind = 7
	CompletionItemKindInterface CompletionItemKind = 8
	CompletionItemKindModule    CompletionItemKind = 9
	CompletionItemKindProperty  CompletionItemKind = 10
	CompletionItemKindValue     CompletionItemKind = 12
	CompletionItemKindKeyword   CompletionItemKind = 14
	CompletionItemKindStruct    CompletionItemKind = 22
)

// CompletionItem is one completion (LSP Completion Request).
type CompletionItem struct {
	Label  string              `json:"label"`
	Kind   *CompletionItemKind `json:"kind,omitempty"`
	Detail *string             `json:"detail,omitempty"`
}

// HoverParams is where a hover is asked for (LSP Hover Request).
type HoverParams struct {
	TextDocumentPositionParams
}

// Hover is the text shown at a position, and the range it covers (LSP Hover
// Request).
type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

// DefinitionParams is where a definition is asked for (LSP Goto Definition
// Request).
type DefinitionParams struct {
	TextDocumentPositionParams
}

// ReferenceContext says whether the declaration counts as a reference (LSP
// Find References Request).
type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

// ReferenceParams is where references are asked for (LSP Find References
// Request).
type ReferenceParams struct {
	TextDocumentPositionParams
	Context ReferenceContext `json:"context"`
}

// RenameParams is where a rename is asked for and the new name (LSP Rename
// Request).
type RenameParams struct {
	TextDocumentPositionParams
	NewName string `json:"newName"`
}

// DocumentSymbolParams names the document whose outline is asked for (LSP
// Document Symbols Request).
type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// SymbolKind is the icon an editor shows for a symbol (LSP Document Symbols
// Request).
type SymbolKind Integer

// The symbol kinds Volt writes (LSP Document Symbols Request).
const (
	SymbolKindModule    SymbolKind = 2
	SymbolKindNamespace SymbolKind = 3
	SymbolKindClass     SymbolKind = 5
	SymbolKindProperty  SymbolKind = 7
	SymbolKindField     SymbolKind = 8
	SymbolKindInterface SymbolKind = 11
	SymbolKindArray     SymbolKind = 18
	SymbolKindKey       SymbolKind = 20
	SymbolKindStruct    SymbolKind = 23
)

// DocumentSymbol is one node of a document's outline (LSP Document Symbols
// Request).
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         *string          `json:"detail,omitempty"`
	Kind           SymbolKind       `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}
