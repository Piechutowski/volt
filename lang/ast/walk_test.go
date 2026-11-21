package ast_test

import (
	"testing"

	"github.com/Piechutowski/volt/lang/ast"
)

func TestInspectSkipsUnsetFields(t *testing.T) {
	bare := []ast.Node{
		&ast.File{},
		&ast.Table{}, &ast.TablePartial{}, &ast.Column{}, &ast.TypeRef{}, &ast.IndexesBlock{},
		&ast.Index{Key: []ast.Node{nil}}, &ast.ValidatorsBlock{}, &ast.Validator{}, &ast.PartialRef{},
		&ast.Ref{}, &ast.RefEndpoint{}, &ast.Records{},
		&ast.RecordRow{Values: []ast.Node{nil}},
		&ast.SettingList{}, &ast.Setting{}, &ast.QualName{}, &ast.NegNumber{},
		&ast.QualIdent{},
		&ast.Migration{}, &ast.TableCreate{}, &ast.TableDrop{}, &ast.TableRename{}, &ast.TableAlter{},
		&ast.AlterAdd{}, &ast.AlterDrop{}, &ast.AlterRename{}, &ast.TableRebuild{}, &ast.IndexCreate{},
		&ast.IndexDrop{}, &ast.Exec{},
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
		&ast.QualIdent{Name: name}, &ast.Setting{Value: name}, &ast.Index{Key: []ast.Node{name}},
		&ast.Migration{Name: name}, &ast.TableCreate{Name: name}, &ast.TableDrop{Name: name},
		&ast.TableRename{To: name}, &ast.TableAlter{Name: name}, &ast.AlterDrop{Name: name},
		&ast.AlterRename{From: name}, &ast.TableRebuild{Name: name}, &ast.IndexCreate{Table: name},
		&ast.IndexDrop{Table: name},
	}
	for _, n := range partial {
		if got := count(n); got != 2 {
			t.Errorf("%T with one child set: %d nodes visited, want 2", n, got)
		}
	}
}
