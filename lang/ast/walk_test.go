package ast_test

import (
	"testing"

	"github.com/Piechutowski/volt/lang/ast"
)

func TestInspectSkipsUnsetFields(t *testing.T) {
	bare := []ast.Node{
		&ast.File{}, &ast.Use{}, &ast.UseItem{}, &ast.Project{}, &ast.ProjectProp{},
		&ast.Table{}, &ast.TablePartial{}, &ast.Column{}, &ast.TypeRef{}, &ast.IndexesBlock{},
		&ast.Index{Key: []ast.Node{nil}}, &ast.ValidatorsBlock{}, &ast.Validator{}, &ast.PartialRef{}, &ast.Note{},
		&ast.Ref{}, &ast.RefEndpoint{}, &ast.Enum{}, &ast.EnumValue{}, &ast.Records{},
		&ast.RecordRow{Values: []ast.Node{nil}}, &ast.StickyNote{}, &ast.TableGroup{}, &ast.DiagramView{},
		&ast.ViewCategory{}, &ast.SettingList{}, &ast.Setting{}, &ast.QualName{}, &ast.NegNumber{},
		&ast.EnumConst{}, &ast.RefValue{},
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
		&ast.Table{Alias: name}, &ast.Column{Name: name}, &ast.Ref{Name: name}, &ast.Validator{Args: []*ast.Ident{name}},
		&ast.EnumConst{Value: name}, &ast.Setting{Value: name}, &ast.Index{Key: []ast.Node{name}},
	}
	for _, n := range partial {
		if got := count(n); got != 2 {
			t.Errorf("%T with one child set: %d nodes visited, want 2", n, got)
		}
	}
}
