// Redundancy analyzers: constructs that repeat what the language already
// guarantees, or repeat each other. Redundancy is how two definitions
// drift apart later.
package vet

import (
	"fmt"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
)

func init() {
	register(redundantNull)
	register(redundantIndex)
	register(emptyBody)
	register(legacyFlag)
	register(shadowedColumn)
	register(projectNotes)
	register(unnecessaryQuotes)
}

type redundantNullRule struct{ meta }

var redundantNull = &redundantNullRule{meta{
	name: "redundantnull",
	doc:  "reports explicit [null] settings; columns are nullable by default (spec §6.3.2)",
}}

func (r *redundantNullRule) Decl(d ast.Decl, nodes []ast.Node, f Facts) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, n := range nodes {
		col, ok := n.(*ast.Column)
		if !ok || col.Settings == nil {
			continue
		}
		if s := col.Settings.Get("null"); s != nil {
			out = append(out, r.warnf(s.Pos(), "explicit 'null' is redundant; columns are nullable by default"))
		}
	}
	return out
}

type redundantIndexRule struct{ meta }

var redundantIndex = &redundantIndexRule{meta{
	name: "redundantindex",
	doc:  "reports indexes that duplicate a column setting, another index, or combine pk with unique",
}}

func (r *redundantIndexRule) Decl(d ast.Decl, nodes []ast.Node, f Facts) []diag.Diagnostic {
	var out []diag.Diagnostic
	ti := f.Table
	if ti == nil {
		return out
	}
	seenKeys := map[string]bool{}
	for _, ix := range ti.Indexes {
		if ix.Settings.Get("pk") != nil && ix.Settings.Get("unique") != nil {
			out = append(out, r.warnf(ix.Pos(), "combining 'pk' and 'unique' on one index is redundant; a primary key is already unique"))
		}
		key := indexKey(ix)
		if seenKeys[key] {
			out = append(out, r.warnf(ix.Pos(), "duplicate index on %s in table %q", key, ti.Decl.Name.String()))
		}
		seenKeys[key] = true

		// single-column index repeating a column-level constraint
		if len(ix.Key) != 1 {
			continue
		}
		id, ok := ix.Key[0].(*ast.Ident)
		if !ok {
			continue
		}
		cd := ti.Column(id.Name())
		if cd == nil {
			continue
		}
		if ix.Settings.Get("unique") != nil && columnHas(cd, "unique") {
			out = append(out, r.warnf(ix.Pos(), "unique index on %q duplicates the column's 'unique' setting", id.Name()))
		}
		if ix.Settings.Get("unique") != nil && columnHas(cd, "pk") {
			out = append(out, r.warnf(ix.Pos(), "unique index on %q is redundant; the column is already the primary key", id.Name()))
		}
		if ix.Settings.Get("pk") != nil && columnHas(cd, "pk") {
			out = append(out, r.warnf(ix.Pos(), "pk index on %q duplicates the column's 'pk' setting", id.Name()))
		}
	}
	return out
}

// indexKey canonicalizes an index key for duplicate detection: column order
// is significant for composite indexes, so it is preserved.
func indexKey(ix *ast.Index) string {
	parts := make([]string, 0, len(ix.Key))
	for _, a := range ix.Key {
		switch a := a.(type) {
		case *ast.Ident:
			parts = append(parts, a.Name())
		case *ast.FuncExpr:
			parts = append(parts, "`"+a.Text()+"`")
		}
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// columnHas reports whether the column carries the given canonical setting,
// via the settings list or a legacy flag.
func columnHas(cd *check.ColumnDef, name string) bool {
	if cd.Col.Settings != nil {
		if cd.Col.Settings.Get(name) != nil {
			return true
		}
		if name == "pk" && cd.Col.Settings.Get("primary key") != nil {
			return true
		}
	}
	for _, f := range cd.Col.LegacyFlags {
		if strings.EqualFold(f.Name(), name) {
			return true
		}
	}
	return false
}

type emptyBodyRule struct{ meta }

var emptyBody = &emptyBodyRule{meta{
	name: "emptybody",
	doc:  "reports empty indexes/checks blocks, TableGroups, DiagramView categories and Project bodies",
}}

func (r *emptyBodyRule) Decl(d ast.Decl, nodes []ast.Node, f Facts) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, n := range nodes {
		switch n := n.(type) {
		case *ast.IndexesBlock:
			if len(n.Indexes) == 0 {
				out = append(out, r.warnf(n.Pos(), "empty indexes block"))
			}
		case *ast.ChecksBlock:
			if len(n.Checks) == 0 {
				out = append(out, r.warnf(n.Pos(), "empty checks block"))
			}
		case *ast.TableGroup:
			if len(n.Members) == 0 {
				out = append(out, r.warnf(n.Pos(), "TableGroup %q has no members", n.Name.Name()))
			}
		case *ast.ViewCategory:
			if !n.Wildcard && len(n.Names) == 0 {
				out = append(out, r.warnf(n.Pos(), "empty %s category selects nothing", n.Kind.Name()))
			}
		case *ast.Project:
			if len(n.Props) == 0 && len(n.Notes) == 0 {
				out = append(out, r.warnf(n.Pos(), "empty Project body"))
			}
		}
	}
	return out
}

type legacyFlagRule struct{ meta }

var legacyFlag = &legacyFlagRule{meta{
	name: "legacyflag",
	doc:  "reports legacy bare pk/unique flags between type and settings; prefer the settings list (spec §6.3.7)",
}}

func (r *legacyFlagRule) Decl(d ast.Decl, nodes []ast.Node, f Facts) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, n := range nodes {
		col, ok := n.(*ast.Column)
		if !ok {
			continue
		}
		for _, fl := range col.LegacyFlags {
			out = append(out, r.warnf(fl.Pos(), "legacy flag %q; write it in the settings list: [%s]", fl.Name(), strings.ToLower(fl.Name())))
		}
	}
	return out
}

type shadowedColumnRule struct{ meta }

var shadowedColumn = &shadowedColumnRule{meta{
	name: "shadowedcolumn",
	doc:  "reports column definitions silently overridden by spec §6.9.4 partial-injection conflict resolution",
}}

func (r *shadowedColumnRule) Decl(d ast.Decl, nodes []ast.Node, f Facts) []diag.Diagnostic {
	var out []diag.Diagnostic
	ti := f.Table
	if ti == nil {
		return out
	}
	for _, sh := range ti.Shadowed {
		winner := ti.Column(sh.Col.Name.Name())
		if winner == nil {
			continue
		}
		out = append(out, r.warnf(winner.Col.Pos(), "column %q in table %q silently overrides the definition from %s",
			sh.Col.Name.Name(), ti.Decl.Name.String(), originOf(sh)))
	}
	return out
}

func originOf(cd *check.ColumnDef) string {
	if cd.Partial == nil {
		return "the table body"
	}
	return fmt.Sprintf("TablePartial %q", cd.Partial.Name.Name())
}

type projectNotesRule struct{ meta }

var projectNotes = &projectNotesRule{meta{
	name: "projectnotes",
	doc:  "reports multiple Note definitions in a Project body; only one survives in tools",
}}

func (r *projectNotesRule) Decl(d ast.Decl, nodes []ast.Node, f Facts) []diag.Diagnostic {
	var out []diag.Diagnostic
	if pr, ok := d.(*ast.Project); ok && len(pr.Notes) > 1 {
		out = append(out, r.warnf(pr.Notes[1].Pos(), "Project has %d Note definitions; tools keep only one", len(pr.Notes)))
	}
	return out
}

type unnecessaryQuotesRule struct{ meta }

var unnecessaryQuotes = &unnecessaryQuotesRule{meta{
	name: "unnecessaryquotes",
	doc:  "reports quoted identifiers that are already valid plain identifiers",
}}

func (r *unnecessaryQuotesRule) Decl(d ast.Decl, nodes []ast.Node, f Facts) []diag.Diagnostic {
	var out []diag.Diagnostic
	seen := map[string]bool{} // repeated spellings on one line of this declaration are reported once
	for _, n := range nodes {
		id, ok := n.(*ast.Ident)
		if !ok || !id.Quoted() || !isPlainIdent(id.Name()) {
			continue
		}
		key := fmt.Sprintf("%d:%s", id.Pos().Line(), id.Name())
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, r.warnf(id.Pos(), "identifier %q does not need quotes", id.Name()))
	}
	return out
}

func isPlainIdent(s string) bool {
	if s == "" {
		return false
	}
	hasLetter := false
	for _, r := range s {
		switch {
		case r == '_' || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z'):
			hasLetter = true
		case '0' <= r && r <= '9':
		default:
			return false
		}
	}
	return hasLetter
}
