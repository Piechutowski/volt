// Package ast declares the types used to represent Volt syntax trees,
// mirroring the role of go/ast. Nodes carry no semantic judgment.
package ast

import "github.com/Piechutowski/volt/lang/token"

// Node is implemented by all AST nodes.
type Node interface {
	Pos() token.Position // position of the first token of the node
	End() token.Position // position immediately after the node
}

// Ident is a plain or quoted identifier (spec §identifiers).
type Ident struct {
	Tok token.Token
}

func (x *Ident) Pos() token.Position { return x.Tok.Pos }
func (x *Ident) End() token.Position { return x.Tok.End() }

// Name returns the identifier's value with escapes applied.
func (x *Ident) Name() string { return x.Tok.Val }

// Quoted reports whether the identifier was written "quoted"; quoted
// identifiers never act as keywords (spec §case-sensitivity-and-terminology).
func (x *Ident) Quoted() bool { return x.Tok.Quoted }

// QualName is a type name, possibly qualified by a Go package: Name or
// pkg.Name (spec §columns, §types).
type QualName struct {
	Parts []*Ident // 1 or 2 elements
}

func (x *QualName) Pos() token.Position { return x.Parts[0].Pos() }
func (x *QualName) End() token.Position { return x.Parts[len(x.Parts)-1].End() }

// Qualifier returns the package qualifier, or "" for a bare name.
func (x *QualName) Qualifier() string {
	if len(x.Parts) == 2 {
		return x.Parts[0].Name()
	}
	return ""
}

// Base returns the unqualified name.
func (x *QualName) Base() string { return x.Parts[len(x.Parts)-1].Name() }

// String returns the spelling, e.g. "time.Time" or "Slug".
func (x *QualName) String() string {
	if len(x.Parts) == 2 {
		return x.Parts[0].Name() + "." + x.Parts[1].Name()
	}
	return x.Parts[0].Name()
}

// BasicLit is a STRING or NUMBER literal.
type BasicLit struct {
	Tok token.Token
}

func (x *BasicLit) Pos() token.Position { return x.Tok.Pos }
func (x *BasicLit) End() token.Position { return x.Tok.End() }

// FuncExpr is a backtick expression literal (spec §expression-literals).
type FuncExpr struct {
	Tok token.Token
}

func (x *FuncExpr) Pos() token.Position { return x.Tok.Pos }
func (x *FuncExpr) End() token.Position { return x.Tok.End() }
func (x *FuncExpr) Text() string        { return x.Tok.Val }

// NegNumber is a minus sign applied to a number literal (spec §default-values, §records-sample-data).
type NegNumber struct {
	MinusPos token.Position
	Num      *BasicLit
}

func (x *NegNumber) Pos() token.Position { return x.MinusPos }
func (x *NegNumber) End() token.Position { return x.Num.End() }

// QualIdent is a dotted setting value, pkg.Name: the function of this
// package a Part II setting names, as error_handler does (spec §scopes).
type QualIdent struct {
	Qual *Ident
	Name *Ident
}

func (x *QualIdent) Pos() token.Position { return x.Qual.Pos() }
func (x *QualIdent) End() token.Position { return x.Name.End() }

// SettingList is a bracketed, comma-separated settings list.
type SettingList struct {
	Lbrack   token.Position
	Settings []*Setting
	Rbrack   token.Position
}

func (x *SettingList) Pos() token.Position { return x.Lbrack }
func (x *SettingList) End() token.Position { return x.Rbrack }

// Get returns the first setting whose canonical name matches, or nil.
func (x *SettingList) Get(name string) *Setting {
	if x == nil {
		return nil
	}
	for _, s := range x.Settings {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// Setting is one entry: a flag (pk) or a key-value pair (note: '...').
// Name is the lowercased form of its one identifier ("no_transaction",
// §settings-lists).
type Setting struct {
	NameTok token.Token // the name as written
	Name    string
	Value   Node // nil for flags; BasicLit, Ident, FuncExpr, NegNumber, QualIdent, IdentList, or RefEndpoint for ref:
	endPos  token.Position
}

func (x *Setting) Pos() token.Position { return x.NameTok.Pos }
func (x *Setting) End() token.Position { return x.endPos }

// SetEnd records the setting's extent.
func (x *Setting) SetEnd(p token.Position) { x.endPos = p }

// File is one parsed Volt source file.
type File struct {
	Name  string // file name as given to the parser
	Decls []Decl // top-level declarations in source order
	EOF   token.Position
}

func (f *File) Pos() token.Position {
	if len(f.Decls) > 0 {
		return f.Decls[0].Pos()
	}
	return f.EOF
}
func (f *File) End() token.Position { return f.EOF }

// Decl is implemented by all top-level declarations.
type Decl interface {
	Node
	declNode()
}

// Project is a Project element (spec §project).
type Project struct {
	ProjectPos token.Position
	Name       *Ident // may be nil
	Settings   *SettingList
	Props      []*ProjectProp
	Rbrace     token.Position
}

func (d *Project) Pos() token.Position { return d.ProjectPos }
func (d *Project) End() token.Position { return d.Rbrace }
func (d *Project) declNode()           {}

// ProjectProp is one key: 'value' line inside a Project body.
type ProjectProp struct {
	Key   *Ident
	Value *BasicLit
}

func (x *ProjectProp) Pos() token.Position { return x.Key.Pos() }
func (x *ProjectProp) End() token.Position { return x.Value.End() }

// Table is a Table element (spec §table).
type Table struct {
	TablePos token.Position
	Name     *Ident
	Settings *SettingList
	Body     []TableItem
	Rbrace   token.Position
}

func (d *Table) Pos() token.Position { return d.TablePos }
func (d *Table) End() token.Position { return d.Rbrace }
func (d *Table) declNode()           {}

// TablePartial is a TablePartial element (spec §tablepartial).
type TablePartial struct {
	PartialPos token.Position
	Name       *Ident
	Body       []TableItem
	Rbrace     token.Position
}

func (d *TablePartial) Pos() token.Position { return d.PartialPos }
func (d *TablePartial) End() token.Position { return d.Rbrace }
func (d *TablePartial) declNode()           {}

// TableItem is implemented by everything that can appear in a Table or
// TablePartial body: *Column, *IndexesBlock, *ValidatorsBlock,
// *PartialRef, *Records.
type TableItem interface {
	Node
	tableItemNode()
}

// Column is one column definition (spec §columns).
type Column struct {
	Name     *Ident
	Type     *TypeRef
	Settings *SettingList
}

func (x *Column) Pos() token.Position { return x.Name.Pos() }
func (x *Column) End() token.Position {
	if x.Settings != nil {
		return x.Settings.End()
	}
	return x.Type.End()
}
func (x *Column) tableItemNode() {}

// TypeRef is a column type: a Go type name, optionally qualified by a
// package alias, with the "[]" of []byte (spec §columns).
type TypeRef struct {
	Name   *QualName
	Slice  bool
	Lbrack token.Position
}

func (x *TypeRef) Pos() token.Position {
	if x.Slice {
		return x.Lbrack
	}
	return x.Name.Pos()
}
func (x *TypeRef) End() token.Position { return x.Name.End() }

// String returns the type as written, e.g. "[]byte" or "time.Time".
func (x *TypeRef) String() string {
	if x.Slice {
		return "[]" + x.Name.String()
	}
	return x.Name.String()
}

// IndexesBlock is one indexes { ... } block (spec §indexes).
type IndexesBlock struct {
	IndexesPos token.Position
	Indexes    []*Index
	Rbrace     token.Position
}

func (x *IndexesBlock) Pos() token.Position { return x.IndexesPos }
func (x *IndexesBlock) End() token.Position { return x.Rbrace }
func (x *IndexesBlock) tableItemNode()      {}

// Index is one index definition. Key atoms are *Ident (columns) or
// *FuncExpr (expressions).
type Index struct {
	Key       []Node
	Composite bool // written with parentheses
	Settings  *SettingList
}

func (x *Index) Pos() token.Position { return x.Key[0].Pos() }
func (x *Index) End() token.Position {
	if x.Settings != nil {
		return x.Settings.End()
	}
	return x.Key[len(x.Key)-1].End()
}

// ValidatorsBlock is a validators block (spec §validators).
type ValidatorsBlock struct {
	ValidatorsPos token.Position
	Validators    []*Validator
	Rbrace        token.Position
}

func (x *ValidatorsBlock) Pos() token.Position { return x.ValidatorsPos }
func (x *ValidatorsBlock) End() token.Position { return x.Rbrace }
func (x *ValidatorsBlock) tableItemNode()      {}

// Validator is one validator line: a Go function of the package applied
// to columns of the table (spec §validators).
type Validator struct {
	Ref    *GoRef
	Args   []*Ident
	EndPos token.Position
}

func (x *Validator) Pos() token.Position { return x.Ref.Pos() }
func (x *Validator) End() token.Position { return x.EndPos }

// PartialRef is a ~partial injection line (spec §tablepartial).
type PartialRef struct {
	TildePos token.Position
	Name     *Ident
}

func (x *PartialRef) Pos() token.Position { return x.TildePos }
func (x *PartialRef) End() token.Position { return x.Name.End() }
func (x *PartialRef) tableItemNode()      {}

// Ref is a top-level relationship element, Ref: left > right, whose
// left endpoint is the foreign key (spec §relationships-ref).
type Ref struct {
	RefPos   token.Position
	Left     *RefEndpoint
	Right    *RefEndpoint
	Settings *SettingList
	endPos   token.Position
}

func (d *Ref) Pos() token.Position     { return d.RefPos }
func (d *Ref) End() token.Position     { return d.endPos }
func (d *Ref) SetEnd(p token.Position) { d.endPos = p }
func (d *Ref) declNode()               {}

// RefEndpoint is one side of a relationship, a table and one or more of
// its columns, and the value of a column's ref: setting (spec
// §relationships-ref).
type RefEndpoint struct {
	Table   *Ident
	Columns []*Ident
	endPos  token.Position
}

func (x *RefEndpoint) Pos() token.Position     { return x.Table.Pos() }
func (x *RefEndpoint) End() token.Position     { return x.endPos }
func (x *RefEndpoint) SetEnd(p token.Position) { x.endPos = p }

// TypeDecl is a Type element: a Go type admitted as a column type, with
// its storage type and import path (spec §types).
type TypeDecl struct {
	TypePos token.Position
	Name    *QualName
	Storage *TypeRef
	From    *BasicLit
	EndPos  token.Position
}

func (d *TypeDecl) Pos() token.Position { return d.TypePos }
func (d *TypeDecl) End() token.Position { return d.EndPos }
func (d *TypeDecl) declNode()           {}

// Records is sample data, top-level or in-table (spec §records-sample-data).
type Records struct {
	RecordsPos token.Position
	Table      *Ident   // nil for in-table records
	Columns    []*Ident // nil when the column list is implicit
	HasColumns bool
	Rows       []*RecordRow
	Rbrace     token.Position
}

func (d *Records) Pos() token.Position { return d.RecordsPos }
func (d *Records) End() token.Position { return d.Rbrace }
func (d *Records) declNode()           {}
func (d *Records) tableItemNode()      {}

// RecordRow is one CSV-style row. Values are *BasicLit, *Ident (true/false/
// null), *FuncExpr or *NegNumber.
type RecordRow struct {
	Values []Node
}

func (x *RecordRow) Pos() token.Position { return x.Values[0].Pos() }
func (x *RecordRow) End() token.Position { return x.Values[len(x.Values)-1].End() }
