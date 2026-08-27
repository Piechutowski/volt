// Volt-layer AST nodes (spec §V, docs/spec.md).
//
// The Volt language is a strict superset of the DBML core: these declarations may
// appear alongside tables and enums in any .volt file. The DBML-layer
// checker (package check) ignores them entirely; their semantics belong to
// the project-level checker in the lang package.
package ast

import "github.com/Piechutowski/volt/lang/token"

// PackageClause is a "package name" clause (spec §V1). At most one per
// file, and it must be the first declaration; both constraints are
// enforced by the lang checker.
type PackageClause struct {
	PackagePos token.Position
	Name       *Ident
}

func (d *PackageClause) Pos() token.Position { return d.PackagePos }
func (d *PackageClause) End() token.Position { return d.Name.End() }
func (d *PackageClause) declNode()           {}

// ImportDecl is an "import ( ... )" block (spec §V2).
type ImportDecl struct {
	ImportPos token.Position
	Specs     []*ImportSpec
	Rparen    token.Position
}

func (d *ImportDecl) Pos() token.Position { return d.ImportPos }
func (d *ImportDecl) End() token.Position { return d.Rparen }
func (d *ImportDecl) declNode()           {}

// ImportSpec is one import line: [alias] path, the path written as
// slash-separated identifiers rooted at the project root (spec §V2).
type ImportSpec struct {
	Alias *Ident   // nil when the default qualifier applies
	Path  []*Ident // 1+ segments
}

func (x *ImportSpec) Pos() token.Position {
	if x.Alias != nil {
		return x.Alias.Pos()
	}
	return x.Path[0].Pos()
}
func (x *ImportSpec) End() token.Position { return x.Path[len(x.Path)-1].End() }

// PathString returns the canonical slash-joined import path.
func (x *ImportSpec) PathString() string {
	s := x.Path[0].Name()
	for _, p := range x.Path[1:] {
		s += "/" + p.Name()
	}
	return s
}

// Qualifier returns the name the import is referenced by: the alias when
// present, else the last path segment (spec §V2.3).
func (x *ImportSpec) Qualifier() string {
	if x.Alias != nil {
		return x.Alias.Name()
	}
	return x.Path[len(x.Path)-1].Name()
}

// GoRef is a reference to Go code or an imported symbol: one identifier,
// or qualifier.identifier (volt.RequestID, app.Session, db.User).
type GoRef struct {
	Parts []*Ident // 1 or 2 elements
}

func (x *GoRef) Pos() token.Position { return x.Parts[0].Pos() }
func (x *GoRef) End() token.Position { return x.Parts[len(x.Parts)-1].End() }

// Qualifier returns the leading qualifier, or "" if unqualified.
func (x *GoRef) Qualifier() string {
	if len(x.Parts) == 2 {
		return x.Parts[0].Name()
	}
	return ""
}

// Base returns the unqualified name.
func (x *GoRef) Base() string { return x.Parts[len(x.Parts)-1].Name() }

func (x *GoRef) String() string {
	if len(x.Parts) == 2 {
		return x.Parts[0].Name() + "." + x.Parts[1].Name()
	}
	return x.Parts[0].Name()
}

// Pipeline is a named middleware list (spec §V3).
type Pipeline struct {
	PipelinePos token.Position
	Name        *Ident
	Plugs       []*Plug
	Rbrace      token.Position
}

func (d *Pipeline) Pos() token.Position { return d.PipelinePos }
func (d *Pipeline) End() token.Position { return d.Rbrace }
func (d *Pipeline) declNode()           {}

// Plug is one "use ref" line inside a Pipeline body.
type Plug struct {
	UsePos token.Position
	Ref    *GoRef
}

func (x *Plug) Pos() token.Position { return x.UsePos }
func (x *Plug) End() token.Position { return x.Ref.End() }

// Scope is a routing scope (spec §V4): a path prefix, optional settings,
// and a body of routes, resources and nested scopes.
type Scope struct {
	ScopePos token.Position
	Path     *RoutePath
	Settings *SettingList
	Items    []ScopeItem
	Rbrace   token.Position
}

func (d *Scope) Pos() token.Position { return d.ScopePos }
func (d *Scope) End() token.Position { return d.Rbrace }
func (d *Scope) declNode()           {}
func (d *Scope) scopeItemNode()      {}

// ScopeItem is implemented by everything legal in a Scope body:
// *Route, *Resources, *Scope.
type ScopeItem interface {
	Node
	scopeItemNode()
}

// Route is one verb route line (spec §V4.2).
type Route struct {
	VerbTok  token.Token // IDENT: get, post, put, patch, delete, options, head, any
	Path     *RoutePath
	Handler  *GoRef // Controller.Action — exactly two parts
	Settings *SettingList
}

func (x *Route) Pos() token.Position { return x.VerbTok.Pos }
func (x *Route) End() token.Position {
	if x.Settings != nil {
		return x.Settings.End()
	}
	return x.Handler.End()
}
func (x *Route) scopeItemNode() {}

// Verb returns the lowercased verb.
func (x *Route) Verb() string { return lower(x.VerbTok.Val) }

// Resources is a resources declaration (spec §V5).
type Resources struct {
	ResourcesPos token.Position
	// Pkg is the package qualifier when the declaration names a model
	// of an imported package (`resources db.Post`); nil otherwise.
	Pkg      *Ident
	Name     *Ident
	Settings *SettingList
}

// Ref renders the declaration's name as written.
func (x *Resources) Ref() string {
	if x.Pkg != nil {
		return x.Pkg.Name() + "." + x.Name.Name()
	}
	return x.Name.Name()
}

func (x *Resources) Pos() token.Position { return x.ResourcesPos }
func (x *Resources) End() token.Position {
	if x.Settings != nil {
		return x.Settings.End()
	}
	return x.Name.End()
}
func (x *Resources) scopeItemNode() {}

// RoutePath is a route or scope path: "/" or "/seg/:param(type)/*rest"
// (spec §V4.1). Tokens within a path are contiguous — the parser rejects
// interior whitespace.
type RoutePath struct {
	SlashPos token.Position
	Segments []*Segment
	endPos   token.Position
}

func (x *RoutePath) Pos() token.Position     { return x.SlashPos }
func (x *RoutePath) End() token.Position     { return x.endPos }
func (x *RoutePath) SetEnd(p token.Position) { x.endPos = p }

// String returns the canonical spelling, e.g. "/users/:id(int64)".
func (x *RoutePath) String() string {
	if len(x.Segments) == 0 {
		return "/"
	}
	s := ""
	for _, seg := range x.Segments {
		s += "/" + seg.String()
	}
	return s
}

// SegKind distinguishes the three segment forms.
type SegKind int

const (
	SegLit   SegKind = iota // users
	SegParam                // :id or :id(int64)
	SegWild                 // :path... — rest-of-path capture, last segment only
)

// Segment is one path segment.
//
// The wildcard is spelled ":name..." rather than "*name": after a slash,
// "/*" necessarily opens a DBML block comment (§3.3), so a star-marked
// wildcard is lexically unreachable in a superset of DBML. The ellipsis
// form keeps one capture sigil and echoes ServeMux's "{name...}".
type Segment struct {
	Kind    SegKind
	MarkPos token.Position // position of ':'; equals Name.Pos() for literals
	Name    *Ident
	Type    *Ident // param type, nil when defaulted; only for SegParam
	Rparen  token.Position
	endPos  token.Position
}

func (x *Segment) Pos() token.Position {
	if x.Kind == SegLit {
		return x.Name.Pos()
	}
	return x.MarkPos
}
func (x *Segment) End() token.Position {
	if x.endPos.IsValid() {
		return x.endPos
	}
	if x.Type != nil {
		return x.Rparen
	}
	return x.Name.End()
}

// SetEnd records the segment's extent (used for the wildcard ellipsis).
func (x *Segment) SetEnd(p token.Position) { x.endPos = p }

func (x *Segment) String() string {
	switch x.Kind {
	case SegParam:
		if x.Type != nil {
			return ":" + x.Name.Name() + "(" + x.Type.Name() + ")"
		}
		return ":" + x.Name.Name()
	case SegWild:
		return ":" + x.Name.Name() + "..."
	default:
		return x.Name.Name()
	}
}

// IdentList is a parenthesized identifier list setting value, e.g.
// only: (index, show) (spec §V6).
type IdentList struct {
	Lparen token.Position
	Names  []*Ident
	Rparen token.Position
}

func (x *IdentList) Pos() token.Position { return x.Lparen }
func (x *IdentList) End() token.Position { return x.Rparen }

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}
