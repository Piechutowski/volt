// Migration nodes (spec/migrations): the element and the operations
// of its body.
package ast

import "github.com/Piechutowski/volt/lang/token"

// Migration is a Migration element: a named list of structural
// operations, written flat or as up and down blocks (spec/migrations).
type Migration struct {
	Doc          *CommentGroup // nil when the construct has no doc comment (spec/doc-comments)
	MigrationPos token.Position
	Name         *Ident
	Settings     *SettingList
	UpDown       bool           // the body is up { } [ down { } ] rather than flat
	Up           []Operation    // the flat body, or the up block
	Down         []Operation    // the down block
	DownPos      token.Position // position of "down"; zero when no down block was written
	Rbrace       token.Position
}

func (d *Migration) Pos() token.Position { return d.MigrationPos }
func (d *Migration) End() token.Position { return d.Rbrace }
func (d *Migration) declNode()           {}

// HasDown reports whether a down block was written, empty or not (spec/migrations).
func (d *Migration) HasDown() bool { return d.DownPos.IsValid() }

// Operation is implemented by every operation of a migration body
// (spec/migrations): *TableCreate, *TableDrop, *TableRename,
// *TableAlter, *TableRebuild, *IndexCreate, *IndexDrop and *Exec.
type Operation interface {
	Node
	operationNode()
}

// TableCreate is a table_create operation; its body holds columns and
// Indexes blocks (spec/migrations).
type TableCreate struct {
	Doc        *CommentGroup // nil when the construct has no doc comment (spec/doc-comments)
	CreatePos  token.Position
	Name       *Ident
	Body       []TableItem
	Incomplete bool // true if a syntax error dropped an item from Body
	Rbrace     token.Position
}

func (x *TableCreate) Pos() token.Position { return x.CreatePos }
func (x *TableCreate) End() token.Position { return x.Rbrace }
func (x *TableCreate) operationNode()      {}

// TableDrop is a table_drop operation, with the dropped shape as its
// body when the migration is to be reversible (spec/migrations).
type TableDrop struct {
	Doc        *CommentGroup // nil when the construct has no doc comment (spec/doc-comments)
	DropPos    token.Position
	Name       *Ident
	Body       []TableItem
	Incomplete bool           // true if a syntax error dropped an item from Body
	Lbrace     token.Position // zero when no body was written
	Rbrace     token.Position
}

func (x *TableDrop) Pos() token.Position { return x.DropPos }
func (x *TableDrop) End() token.Position {
	if x.HasBody() {
		return x.Rbrace
	}
	return x.Name.End()
}
func (x *TableDrop) operationNode() {}

// HasBody reports whether the dropped shape was written (spec/migrations).
func (x *TableDrop) HasBody() bool { return x.Lbrace.IsValid() }

// TableRename is a table_rename operation (spec/migrations).
type TableRename struct {
	Doc       *CommentGroup // nil when the construct has no doc comment (spec/doc-comments)
	RenamePos token.Position
	From, To  *Ident
}

func (x *TableRename) Pos() token.Position { return x.RenamePos }
func (x *TableRename) End() token.Position { return x.To.End() }
func (x *TableRename) operationNode()      {}

// TableAlter is a table_alter operation: one ALTER TABLE statement per
// item (spec/migrations).
type TableAlter struct {
	Doc      *CommentGroup // nil when the construct has no doc comment (spec/doc-comments)
	AlterPos token.Position
	Name     *Ident
	Items    []AlterItem
	Rbrace   token.Position
}

func (x *TableAlter) Pos() token.Position { return x.AlterPos }
func (x *TableAlter) End() token.Position { return x.Rbrace }
func (x *TableAlter) operationNode()      {}

// AlterItem is implemented by the items of a table_alter body
// (spec/migrations): *AlterAdd, *AlterDrop and *AlterRename.
type AlterItem interface {
	Node
	alterItemNode()
}

// AlterAdd adds a column (spec/migrations).
type AlterAdd struct {
	Doc    *CommentGroup // nil when the construct has no doc comment (spec/doc-comments)
	AddPos token.Position
	Column *Column
}

func (x *AlterAdd) Pos() token.Position { return x.AddPos }
func (x *AlterAdd) End() token.Position { return x.Column.End() }
func (x *AlterAdd) alterItemNode()      {}

// AlterDrop drops a column; the type and settings, when written, are
// the dropped column's shape for reversibility (spec/migrations).
type AlterDrop struct {
	Doc      *CommentGroup // nil when the construct has no doc comment (spec/doc-comments)
	DropPos  token.Position
	Name     *Ident
	Type     *TypeRef // nil when no shape was written
	Settings *SettingList
}

func (x *AlterDrop) Pos() token.Position { return x.DropPos }
func (x *AlterDrop) End() token.Position {
	if x.Settings != nil {
		return x.Settings.End()
	}
	if x.Type != nil {
		return x.Type.End()
	}
	return x.Name.End()
}
func (x *AlterDrop) alterItemNode() {}

// AlterRename renames a column (spec/migrations).
type AlterRename struct {
	Doc       *CommentGroup // nil when the construct has no doc comment (spec/doc-comments)
	RenamePos token.Position
	From, To  *Ident
}

func (x *AlterRename) Pos() token.Position { return x.RenamePos }
func (x *AlterRename) End() token.Position { return x.To.End() }
func (x *AlterRename) alterItemNode()      {}

// TableRebuild is a table_rebuild operation; its body is the new shape
// (spec/migrations).
type TableRebuild struct {
	Doc        *CommentGroup // nil when the construct has no doc comment (spec/doc-comments)
	RebuildPos token.Position
	Name       *Ident
	Body       []TableItem
	Incomplete bool // true if a syntax error dropped an item from Body
	Rbrace     token.Position
}

func (x *TableRebuild) Pos() token.Position { return x.RebuildPos }
func (x *TableRebuild) End() token.Position { return x.Rbrace }
func (x *TableRebuild) operationNode()      {}

// IndexCreate is an index_create operation: a table and an index as a
// schema Indexes block writes it (spec/migrations).
type IndexCreate struct {
	Doc       *CommentGroup // nil when the construct has no doc comment (spec/doc-comments)
	CreatePos token.Position
	Table     *Ident
	Index     *Index
}

func (x *IndexCreate) Pos() token.Position { return x.CreatePos }
func (x *IndexCreate) End() token.Position { return x.Index.End() }
func (x *IndexCreate) operationNode()      {}

// IndexDrop is an index_drop operation (spec/migrations).
type IndexDrop struct {
	Doc     *CommentGroup // nil when the construct has no doc comment (spec/doc-comments)
	DropPos token.Position
	Table   *Ident
	Index   *Index
}

func (x *IndexDrop) Pos() token.Position { return x.DropPos }
func (x *IndexDrop) End() token.Position { return x.Index.End() }
func (x *IndexDrop) operationNode()      {}

// Exec is an exec operation: SQL in an expression literal run
// verbatim, with its inverse in the down: setting (spec/migrations).
type Exec struct {
	Doc      *CommentGroup // nil when the construct has no doc comment (spec/doc-comments)
	ExecPos  token.Position
	SQL      *FuncExpr
	Settings *SettingList
}

func (x *Exec) Pos() token.Position { return x.ExecPos }
func (x *Exec) End() token.Position {
	if x.Settings != nil {
		return x.Settings.End()
	}
	return x.SQL.End()
}
func (x *Exec) operationNode() {}

// DownSQL returns the expression literal of the down: setting, or nil
// when the operation has no inverse (spec/migrations).
func (x *Exec) DownSQL() *FuncExpr {
	s := x.Settings.Get("down")
	if s == nil {
		return nil
	}
	if lit, ok := s.Value.(*FuncExpr); ok {
		return lit
	}
	return nil
}
