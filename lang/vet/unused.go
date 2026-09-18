// Unused-declaration analyzers — DBML's version of Go's "imported and not
// used". A declared-but-unreferenced enum, partial or alias is usually a
// leftover from a refactor.
package vet

import (
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/diag"
)

// unusedEnumFold reads the enum keys every table's columns name from
// the declarations' summaries.
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
			p.Reportf(e.Decl.Pos(), "enum %q is declared but never used as a column type", e.Decl.Name.String())
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

// unusedAliasFold reads the unqualified table spellings every
// declaration uses from the summaries.
type unusedAliasFold struct{ meta }

var unusedAlias = &unusedAliasFold{meta{
	name: "unusedalias",
	doc:  "reports table aliases that nothing references",
}}

func (fo *unusedAliasFold) File(p *FilePass) {
	used := map[string]bool{}
	for _, s := range p.Summaries {
		for _, name := range s.NamesUsed {
			used[name] = true
		}
	}
	for _, ti := range p.Info.Tables {
		if ti.Alias != "" && !used[ti.Alias] && !strings.EqualFold(ti.Alias, ti.Decl.Name.Base()) {
			p.Reportf(ti.Decl.Alias.Pos(), "alias %q of table %q is never used", ti.Alias, ti.Decl.Name.String())
		}
	}
}
