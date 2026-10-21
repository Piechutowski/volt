// Unused-declaration analyzers: a declared-but-unreferenced enum or
// partial.
package vet

import (
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/diag"
)

type unusedEnumFold struct{ meta }

var unusedEnum = &unusedEnumFold{meta{
	name: "unusedenum",
	doc:  "reports enums never used as a column type in this file (other files may still use exported enums)",
}}

func (fo *unusedEnumFold) File(p *FilePass) {
	used := map[string]bool{}
	for _, s := range p.Summaries {
		for _, key := range s.EnumsUsed {
			used[key] = true
		}
	}
	for _, e := range p.Info.Enums {
		if !used[e.Key] {
			p.Reportf(e.Decl.Pos(), "enum %q is declared but never used as a column type", e.Decl.Name.Name())
		}
	}
}

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
