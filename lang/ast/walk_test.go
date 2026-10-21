package ast_test

import (
	"testing"

	"github.com/Piechutowski/volt/lang/ast"
)

func TestInspectSkipsUnsetFields(t *testing.T) {
	bare := []ast.Node{
		&ast.File{}, &ast.Project{}, &ast.ProjectProp{},
		&ast.Table{}, &ast.TablePartial{}, &ast.Column{}, &ast.TypeRef{}, &ast.IndexesBlock{},
		&ast.Index{Key: []ast.Node{nil}}, &ast.ValidatorsBlock{}, &ast.Validator{}, &ast.PartialRef{},
		&ast.Ref{}, &ast.RefEndpoint{}, &ast.Enum{}, &ast.EnumValue{}, &ast.Records{},
		&ast.RecordRow{Values: []ast.Node{nil}},
		&ast.SettingList{}, &ast.Setting{}, &ast.QualName{}, &ast.NegNumber{},
		&ast.EnumConst{},
		&ast.Migration{}, &ast.CreateTable{}, &ast.DropTable{}, &ast.RenameTable{}, &ast.AlterTable{},
		&ast.AlterAdd{}, &ast.AlterDrop{}, &ast.AlterRename{}, &ast.RebuildTable{}, &ast.CreateIndex{},
		&ast.DropIndex{}, &ast.Exec{},
	}
	count := func(n ast.Node) int {
		seen := 0
		ast.Inspect(n, func(c ast.Node) bool {
			if c != n {
				c.Pos()
			}
			seen++
			return true
		})
		return seen
	}
	for _, n := range bare {
		if got := count(n); got != 1 {
			t.Errorf("%T with nothing set: %d nodes visited, want 1", n, got)
		}
	}
	name := &ast.Ident{}
	partial := []ast.Node{
		&ast.Table{Name: name}, &ast.Column{Name: name}, &ast.RefEndpoint{Table: name}, &ast.Validator{Args: []*ast.Ident{name}},
		&ast.EnumConst{Value: name}, &ast.Setting{Value: name}, &ast.Index{Key: []ast.Node{name}},
		&ast.Migration{Name: name}, &ast.CreateTable{Name: name}, &ast.DropTable{Name: name},
		&ast.RenameTable{To: name}, &ast.AlterTable{Name: name}, &ast.AlterDrop{Name: name},
		&ast.AlterRename{From: name}, &ast.RebuildTable{Name: name}, &ast.CreateIndex{Table: name},
		&ast.DropIndex{Table: name},
	}
	for _, n := range partial {
		if got := count(n); got != 2 {
			t.Errorf("%T with one child set: %d nodes visited, want 2", n, got)
		}
	}
}
