package vet

import (
	goast "go/ast"
	"strings"

	"github.com/Piechutowski/volt/lang/diag"
)

type loadSpelledLongRule struct{ meta }

var loadSpelledLong = &loadSpelledLongRule{meta{
	name: "loadspelledlong",
	doc:  "reports a handler that returns a continuation over a query whose every argument is a route parameter or a literal, which a load of the route spells shorter",
}}

func (r *loadSpelledLongRule) Go(f *GoFile) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, d := range f.File.Decls {
		fd, ok := d.(*goast.FuncDecl)
		if !ok || !handlerShaped(fd) || fd.Body == nil {
			continue
		}
		params := map[string]bool{}
		for i, field := range fd.Type.Params.List {
			for _, id := range field.Names {
				if i > 0 && id.Name != "body" {
					params[id.Name] = true
				}
			}
		}
		runtime := runtimeNames(f.File)
		for _, stmt := range fd.Body.List {
			ret, ok := stmt.(*goast.ReturnStmt)
			if !ok || len(ret.Results) != 1 {
				continue
			}
			call, ok := ret.Results[0].(*goast.CallExpr)
			if !ok || spelled(call.Fun) != "volt.Then" || len(call.Args) != 2 {
				continue
			}
			query, ok := call.Args[0].(*goast.CallExpr)
			if !ok || runtime[qualifier(query.Fun)] || deleteSpelled(spelled(query.Fun)) || !routeBound(query.Args, params) {
				continue
			}
			out = append(out, r.warnf(f.Pos(query), "%s runs on the route's parameters alone; a load of the route hands the handler its result and spells it once (§route-loads)", spelled(query.Fun)))
		}
	}
	return out
}

// deleteSpelled tells a delete of the generated surface by its name,
// since its key arguments may all be route parameters; a create or an
// update takes a changeset, the handler's body or a value it computed,
// which no load binds (spec §request-bodies).
func deleteSpelled(name string) bool {
	return strings.HasSuffix(name, "Delete") || strings.HasSuffix(name, "DeleteWhere")
}

// runtimeNames is the names the file imports the runtime packages
// under, whose functions no load names (spec §route-loads).
func runtimeNames(file *goast.File) map[string]bool {
	out := map[string]bool{}
	for _, im := range file.Imports {
		path := strings.Trim(im.Path.Value, `"`)
		if path != "github.com/Piechutowski/volt" && path != "github.com/Piechutowski/volt/nao/rt" {
			continue
		}
		name := path[strings.LastIndex(path, "/")+1:]
		if im.Name != nil {
			name = im.Name.Name
		}
		out[name] = true
	}
	return out
}

func qualifier(fun goast.Expr) string {
	if sel, ok := fun.(*goast.SelectorExpr); ok {
		if id, ok := sel.X.(*goast.Ident); ok {
			return id.Name
		}
	}
	return ""
}

func routeBound(args []goast.Expr, params map[string]bool) bool {
	for _, a := range args {
		switch x := a.(type) {
		case *goast.Ident:
			if !params[x.Name] {
				return false
			}
		case *goast.BasicLit:
		default:
			return false
		}
	}
	return true
}
