package ast

import (
	goast "go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"path/filepath"
	"slices"
	"testing"
)

func TestDocOfCoversEveryDocumentedNode(t *testing.T) {
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := gotoken.NewFileSet()
	var documented, covered []string
	for _, path := range paths {
		f, err := goparser.ParseFile(fset, path, nil, goparser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		goast.Inspect(f, func(n goast.Node) bool {
			switch x := n.(type) {
			case *goast.TypeSpec:
				st, ok := x.Type.(*goast.StructType)
				if !ok {
					return true
				}
				for _, field := range st.Fields.List {
					star, ok := field.Type.(*goast.StarExpr)
					if !ok {
						continue
					}
					if id, ok := star.X.(*goast.Ident); ok && id.Name == "CommentGroup" && len(field.Names) == 1 && field.Names[0].Name == "Doc" {
						documented = append(documented, x.Name.Name)
					}
				}
			case *goast.FuncDecl:
				if x.Name.Name != "DocOf" {
					return true
				}
				goast.Inspect(x.Body, func(n goast.Node) bool {
					if cc, ok := n.(*goast.CaseClause); ok {
						for _, e := range cc.List {
							if star, ok := e.(*goast.StarExpr); ok {
								covered = append(covered, star.X.(*goast.Ident).Name)
							}
						}
					}
					return true
				})
			}
			return true
		})
	}
	slices.Sort(documented)
	slices.Sort(covered)
	if len(documented) == 0 || !slices.Equal(documented, covered) {
		t.Errorf("DocOf covers %v, the nodes with a Doc field are %v (spec §doc-comments): a construct that takes a doc comment must be read by DocOf", covered, documented)
	}
}
