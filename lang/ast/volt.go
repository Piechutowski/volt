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
	// Mods holds an optional one-word modifier per name (parallel to
	// Names, nil entries when absent): order lists use it for asc/desc
	// (§V11.5); other settings reject modifiers at check time.
	Mods   []*Ident
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

/* ===== groups, predicates, selects (spec §V9-§V11) ===== */

// Group is a named set of tables for generation (spec §V9): either a
// block of members or a `+`/`\` expression over tables, groups and
// TableGroups.
type Group struct {
	GroupPos token.Position
	Name     *Ident
	Terms    []*GroupTerm // block form: all Add, one name each; expr form: signed
	EndPos   token.Position
}

func (d *Group) Pos() token.Position { return d.GroupPos }
func (d *Group) End() token.Position { return d.EndPos }
func (d *Group) declNode()           {}

// GroupTerm is one term of a group expression: one name, or a
// parenthesized set of names, added or removed (spec §V9.3).
type GroupTerm struct {
	Neg    bool     // true for '\' terms
	Names  []*Ident // one name, or the members of a parenthesized set
	Lparen token.Position
	Rparen token.Position // End of the closing paren; zero when unparenthesized
}

// Set reports whether the term was written as a parenthesized set.
func (t *GroupTerm) Set() bool { return t.Rparen.Line > 0 }

// End is the term's extent: the closing paren or the last name.
func (t *GroupTerm) End() token.Position {
	if t.Set() {
		return t.Rparen
	}
	return t.Names[len(t.Names)-1].End()
}

// Pred is a named predicate declaration (spec §V10).
type Pred struct {
	PredPos token.Position
	Name    *Ident
	X       PredExpr
	Rbrace  token.Position
}

func (d *Pred) Pos() token.Position { return d.PredPos }
func (d *Pred) End() token.Position { return d.Rbrace }
func (d *Pred) declNode()           {}

// PredExpr is a node of the closed predicate expression language
// (spec §V10). It is deliberately not SQL (D06).
type PredExpr interface {
	Node
	predExpr()
}

// PredBinary is "x and y" / "x or y".
type PredBinary struct {
	Op   string // "and" | "or" (canonical lower case)
	X, Y PredExpr
}

func (x *PredBinary) Pos() token.Position { return x.X.Pos() }
func (x *PredBinary) End() token.Position { return x.Y.End() }
func (x *PredBinary) predExpr()           {}

// PredNot is "not x".
type PredNot struct {
	NotPos token.Position
	X      PredExpr
}

func (x *PredNot) Pos() token.Position { return x.NotPos }
func (x *PredNot) End() token.Position { return x.X.End() }
func (x *PredNot) predExpr()           {}

// PredParen is "(x)"; kept for exact source spans.
type PredParen struct {
	Lparen token.Position
	X      PredExpr
	Rparen token.Position
}

func (x *PredParen) Pos() token.Position { return x.Lparen }
func (x *PredParen) End() token.Position { return x.Rparen }
func (x *PredParen) predExpr()           {}

// PredRef is a bare name referencing another Pred (spec §V10.2).
type PredRef struct {
	Name *Ident
}

func (x *PredRef) Pos() token.Position { return x.Name.Pos() }
func (x *PredRef) End() token.Position { return x.Name.End() }
func (x *PredRef) predExpr()           {}

// PredCompare is "a <op> b" with op one of = != < <= > >=.
type PredCompare struct {
	X  Operand
	Op token.Kind // EQ NEQ LT LE GT GE
	Y  Operand
}

func (x *PredCompare) Pos() token.Position { return x.X.Pos() }
func (x *PredCompare) End() token.Position { return x.Y.End() }
func (x *PredCompare) predExpr()           {}

// PredIn is "col in (lit, ...)".
type PredIn struct {
	Col    *Ident
	Items  []*Lit
	Rparen token.Position
}

func (x *PredIn) Pos() token.Position { return x.Col.Pos() }
func (x *PredIn) End() token.Position { return x.Rparen }
func (x *PredIn) predExpr()           {}

// PredLike is "col like pattern" (pattern: string literal or param).
type PredLike struct {
	Col     *Ident
	Pattern Operand // *Lit (string) or *Param
}

func (x *PredLike) Pos() token.Position { return x.Col.Pos() }
func (x *PredLike) End() token.Position { return x.Pattern.End() }
func (x *PredLike) predExpr()           {}

// PredNull is "col is [not] null".
type PredNull struct {
	Col    *Ident
	Not    bool
	EndPos token.Position
}

func (x *PredNull) Pos() token.Position { return x.Col.Pos() }
func (x *PredNull) End() token.Position { return x.EndPos }
func (x *PredNull) predExpr()           {}

// Operand is a comparison operand: column reference, :param, or literal.
type Operand interface {
	Node
	operand()
}

// ColRef is a column reference inside a predicate.
type ColRef struct {
	Name *Ident
}

func (x *ColRef) Pos() token.Position { return x.Name.Pos() }
func (x *ColRef) End() token.Position { return x.Name.End() }
func (x *ColRef) operand()            {}

// Param is a ":name" query parameter (spec §V10, D15).
type Param struct {
	ColonPos token.Position
	Name     *Ident
}

func (x *Param) Pos() token.Position { return x.ColonPos }
func (x *Param) End() token.Position { return x.Name.End() }
func (x *Param) operand()            {}

// Lit is a number, string or boolean literal operand.
type Lit struct {
	Tok token.Token // NUMBER, STRING, or IDENT true/false
}

func (x *Lit) Pos() token.Position { return x.Tok.Pos }
func (x *Lit) End() token.Position { return x.Tok.End() }
func (x *Lit) operand()            {}

// Select declares a query generated for every member of its target
// (spec §V11).
type Select struct {
	SelectPos token.Position
	Name      *Ident
	Lparen    token.Position // projection parens (§V11.7); zero when absent
	Star      bool           // (* \\ a \\ (b, c)) star form
	Cols      []*Ident       // explicit list, or the star form's exclusions
	Rparen    token.Position
	Target    *Ident   // group or table name (§V11.2)
	Where     PredExpr // nil = all rows
	Settings  *SettingList
	EndPos    token.Position
}

// Projected reports whether the select carries a projection (§V11.7).
func (d *Select) Projected() bool { return len(d.Cols) > 0 }

func (d *Select) Pos() token.Position { return d.SelectPos }
func (d *Select) End() token.Position { return d.EndPos }
func (d *Select) declNode()           {}
