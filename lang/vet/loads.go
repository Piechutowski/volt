package vet

import (
	goast "go/ast"

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
				if i > 0 {
					params[id.Name] = true
				}
			}
		}
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
			if !ok || !routeBound(query.Args, params) {
				continue
			}
			out = append(out, r.warnf(f.Pos(query), "%s runs on the route's parameters alone; a load of the route hands the handler its result and spells it once (§route-loads)", spelled(query.Fun)))
		}
	}
	return out
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
