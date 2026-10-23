package naming

import (
	"fmt"

	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/token"
)

// NameCollision is one Go package-scope name that generation would mint
// twice. First and Second describe the colliding origins in declaration
// order; Pos locates the later of the two. Type is the bare declared
// type among them, which the checker refuses (spec §types), and With
// describes the other origin; Type is nil when generated names collide.
type NameCollision struct {
	Name          string
	First, Second string
	Pos           token.Position
	Type          *check.TypeInfo
	With          string
}

type dynOrigin struct {
	kind dynOriginKind
	pos  token.Position
	tm   *Table
	f    *Field
	td   *check.TypeInfo
	sfx  string
}

type dynOriginKind uint8

const (
	dynQueries dynOriginKind = iota
	dynType
	dynModel
	dynChangeset
	dynColumn
	dynParams
	dynNew
	dynHandle
	dynWrapper
)

func (o dynOrigin) name() string {
	switch o.kind {
	case dynModel:
		return o.tm.Model
	case dynChangeset:
		return o.tm.Names.Changeset
	case dynColumn:
		return o.tm.Names.Column
	case dynParams:
		return o.tm.Names.Params
	case dynNew:
		return o.tm.Names.New
	case dynHandle:
		return o.tm.Handle(o.f)
	case dynWrapper:
		return o.tm.Model + o.sfx
	}
	return "Queries"
}

func (o dynOrigin) describe() string {
	switch o.kind {
	case dynType:
		return "Type " + o.td.Key
	case dynModel:
		return fmt.Sprintf("table %s (model %s)", o.tm.Info.Decl.Name.Name(), o.tm.Model)
	case dynChangeset:
		return fmt.Sprintf("table %s (changeset %s)", o.tm.Info.Decl.Name.Name(), o.tm.Names.Changeset)
	case dynColumn:
		return fmt.Sprintf("table %s (column handle type %s)", o.tm.Info.Decl.Name.Name(), o.tm.Names.Column)
	case dynParams:
		return fmt.Sprintf("table %s (write body %s)", o.tm.Info.Decl.Name.Name(), o.tm.Names.Params)
	case dynNew:
		return fmt.Sprintf("table %s (constructor %s)", o.tm.Info.Decl.Name.Name(), o.tm.Names.New)
	case dynHandle:
		return fmt.Sprintf("table %s column %q (handle %s)", o.tm.Info.Decl.Name.Name(), o.f.ColName, o.tm.Handle(o.f))
	case dynWrapper:
		return fmt.Sprintf("table %s (option wrapper %s%s)", o.tm.Info.Decl.Name.Name(), o.tm.Model, o.sfx)
	}
	return "the generated Queries type"
}
