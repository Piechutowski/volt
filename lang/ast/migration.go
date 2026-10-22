// Migration nodes (spec §migrations): the element and the operations
// of its body.
package ast

import "github.com/Piechutowski/volt/lang/token"

// Migration is a Migration element: a named list of structural
// operations, written flat or as up and down blocks (spec §migrations).
type Migration struct {
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

// HasDown reports whether a down block was written, empty or not (spec §migrations).
func (d *Migration) HasDown() bool { return d.DownPos.IsValid() }

// Operation is implemented by every operation of a migration body
// (spec §migrations): *CreateTable, *DropTable, *RenameTable,
// *AlterTable, *RebuildTable, *CreateIndex, *DropIndex and *Exec.
type Operation interface {
	Node
	operationNode()
}

// CreateTable is a create table operation; its body holds columns and
// indexes blocks (spec §migrations).
type CreateTable struct {
	CreatePos  token.Position
	Name       *Ident
	Body       []TableItem
	Incomplete bool // true if a syntax error dropped an item from Body
	Rbrace     token.Position
}

func (x *CreateTable) Pos() token.Position { return x.CreatePos }
func (x *CreateTable) End() token.Position { return x.Rbrace }
func (x *CreateTable) operationNode()      {}

// DropTable is a drop table operation, with the dropped shape as its
// body when the migration is to be reversible (spec §migrations).
type DropTable struct {
	DropPos    token.Position
	Name       *Ident
	Body       []TableItem
	Incomplete bool           // true if a syntax error dropped an item from Body
	Lbrace     token.Position // zero when no body was written
	Rbrace     token.Position
}

func (x *DropTable) Pos() token.Position { return x.DropPos }
func (x *DropTable) End() token.Position {
	if x.HasBody() {
		return x.Rbrace
	}
	return x.Name.End()
}
func (x *DropTable) operationNode() {}

// HasBody reports whether the dropped shape was written (spec §migrations).
func (x *DropTable) HasBody() bool { return x.Lbrace.IsValid() }

// RenameTable is a rename table operation (spec §migrations).
type RenameTable struct {
	RenamePos token.Position
	From, To  *Ident
}

func (x *RenameTable) Pos() token.Position { return x.RenamePos }
func (x *RenameTable) End() token.Position { return x.To.End() }
func (x *RenameTable) operationNode()      {}

// AlterTable is an alter table operation: one ALTER TABLE statement per
// item (spec §migrations).
type AlterTable struct {
	AlterPos token.Position
	Name     *Ident
	Items    []AlterItem
	Rbrace   token.Position
}

func (x *AlterTable) Pos() token.Position { return x.AlterPos }
func (x *AlterTable) End() token.Position { return x.Rbrace }
func (x *AlterTable) operationNode()      {}

// AlterItem is implemented by the items of an alter table body (spec
// §migrations): *AlterAdd, *AlterDrop and *AlterRename.
type AlterItem interface {
	Node
	alterItemNode()
}

// AlterAdd adds a column (spec §migrations).
type AlterAdd struct {
	AddPos token.Position
	Column *Column
}

func (x *AlterAdd) Pos() token.Position { return x.AddPos }
func (x *AlterAdd) End() token.Position { return x.Column.End() }
func (x *AlterAdd) alterItemNode()      {}

// AlterDrop drops a column; the type and settings, when written, are
// the dropped column's shape for reversibility (spec §migrations).
type AlterDrop struct {
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

// AlterRename renames a column (spec §migrations).
type AlterRename struct {
	RenamePos token.Position
	From, To  *Ident
}

func (x *AlterRename) Pos() token.Position { return x.RenamePos }
func (x *AlterRename) End() token.Position { return x.To.End() }
func (x *AlterRename) alterItemNode()      {}

// RebuildTable is a rebuild table operation; its body is the new shape
// (spec §migrations).
type RebuildTable struct {
	RebuildPos token.Position
	Name       *Ident
	Body       []TableItem
	Incomplete bool // true if a syntax error dropped an item from Body
	Rbrace     token.Position
}

func (x *RebuildTable) Pos() token.Position { return x.RebuildPos }
func (x *RebuildTable) End() token.Position { return x.Rbrace }
func (x *RebuildTable) operationNode()      {}

// CreateIndex is a create index operation: a table and an index as a
// schema indexes block writes it (spec §migrations).
type CreateIndex struct {
	CreatePos token.Position
	Table     *Ident
	Index     *Index
}

func (x *CreateIndex) Pos() token.Position { return x.CreatePos }
func (x *CreateIndex) End() token.Position { return x.Index.End() }
func (x *CreateIndex) operationNode()      {}

// DropIndex is a drop index operation (spec §migrations).
type DropIndex struct {
	DropPos token.Position
	Table   *Ident
	Index   *Index
}

func (x *DropIndex) Pos() token.Position { return x.DropPos }
func (x *DropIndex) End() token.Position { return x.Index.End() }
func (x *DropIndex) operationNode()      {}

// Exec is an exec operation: SQL run verbatim, with its inverse in the
// down: setting (spec §migrations).
type Exec struct {
	ExecPos  token.Position
	SQL      *BasicLit
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

// DownSQL returns the string of the down: setting, or nil when the
// operation has no inverse (spec §migrations).
func (x *Exec) DownSQL() *BasicLit {
	s := x.Settings.Get("down")
	if s == nil {
		return nil
	}
	if lit, ok := s.Value.(*BasicLit); ok && lit.Tok.Kind == token.STRING {
		return lit
	}
	return nil
}
