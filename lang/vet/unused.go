// Unused-declaration analyzer: a declared-but-never-injected partial.
package vet

import (
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/diag"
)

type unusedPartialRule struct{ meta }

var unusedPartial = &unusedPartialRule{meta{
	name: "unusedpartial",
	doc:  "reports TablePartials never injected into a table in this file",
}}

func (r *unusedPartialRule) Decl(d ast.Decl, nodes []ast.Node, f Facts) []diag.Diagnostic {
	var out []diag.Diagnostic
	if tp := f.Partial; tp != nil && tp.Uses == 0 {
		out = append(out, r.warnf(tp.Decl.Pos(), "TablePartial %q is declared but never injected", tp.Decl.Name.Name()))
	}
	return out
}
