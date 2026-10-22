// Redundancy analyzers: constructs that repeat what the language already
// guarantees, or repeat each other.
package vet

import (
	"fmt"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
)

type redundantIndexRule struct{ meta }

var redundantIndex = &redundantIndexRule{meta{
	name: "redundantindex",
	doc:  "reports indexes that duplicate the primary key, a column's unique setting or another index",
}}

func (r *redundantIndexRule) Decl(d ast.Decl, nodes []ast.Node, f Facts) []diag.Diagnostic {
	var out []diag.Diagnostic
	ti := f.Table
	if ti == nil {
		return out
	}
	pk := ti.PK()
	var pkNames []string
	for _, cd := range pk {
		pkNames = append(pkNames, cd.Col.Name.Name())
	}
	if len(pk) == 1 {
		for _, name := range []string{"unique", "index"} {
			if s := pk[0].Col.Settings.Get(name); s != nil {
				out = append(out, r.warnf(s.Pos(), "%s on %q is redundant; the column is the primary key, which is unique and indexed already", name, pkNames[0]))
			}
		}
	}
	seenKeys := map[string]bool{}
	for _, ix := range ti.Indexes {
		key := indexKey(ix)
		if seenKeys[key] {
			out = append(out, r.warnf(ix.Pos(), "duplicate index on %s in table %q", key, ti.Decl.Name.Name()))
		}
		seenKeys[key] = true

		if len(pk) > 1 && ix.Settings.Get("where") == nil && key == "("+strings.Join(pkNames, ", ")+")" {
			out = append(out, r.warnf(ix.Pos(), "index on %s is redundant; it is the key of the primary key, which is unique and indexed already", key))
		}
		if len(ix.Key) != 1 || ix.Settings.Get("unique") == nil {
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
		if cd.Col.Settings.Get("unique") != nil {
			out = append(out, r.warnf(ix.Pos(), "unique index on %q is redundant; the column carries unique, which holds over the whole table", id.Name()))
		}
		if len(pk) == 1 && pk[0] == cd {
			out = append(out, r.warnf(ix.Pos(), "unique index on %q is redundant; the column is the primary key", id.Name()))
		}
	}
	return out
}

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
	key := "(" + strings.Join(parts, ", ") + ")"
	if s := ix.Settings.Get("where"); s != nil {
		if fe, ok := s.Value.(*ast.FuncExpr); ok {
			key += " where `" + fe.Text() + "`"
		}
	}
	return key
}

type emptyBodyRule struct{ meta }

var emptyBody = &emptyBodyRule{meta{
	name: "emptybody",
	doc:  "reports empty Indexes and Validators blocks and a Project with neither properties nor a note",
}}

func (r *emptyBodyRule) Decl(d ast.Decl, nodes []ast.Node, f Facts) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, n := range nodes {
		switch n := n.(type) {
		case *ast.IndexesBlock:
			if len(n.Indexes) == 0 {
				out = append(out, r.warnf(n.Pos(), "empty Indexes block"))
			}
		case *ast.ValidatorsBlock:
			if len(n.Validators) == 0 {
				out = append(out, r.warnf(n.Pos(), "empty Validators block"))
			}
		case *ast.Project:
			if len(n.Props) == 0 && n.Settings.Get("note") == nil {
				out = append(out, r.warnf(n.Pos(), "empty Project: no properties and no note"))
			}
		}
	}
	return out
}

type shadowedColumnRule struct{ meta }

var shadowedColumn = &shadowedColumnRule{meta{
	name: "shadowedcolumn",
	doc:  "reports column definitions silently overridden by spec §tablepartial partial-injection conflict resolution",
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
		if winner.Col == sh.Col {
			out = append(out, r.warnf(winner.Col.Pos(), "column %q comes into table %q twice, since the table injects %s more than once",
				sh.Col.Name.Name(), ti.Decl.Name.Name(), originOf(sh)))
			continue
		}
		out = append(out, r.warnf(winner.Col.Pos(), "column %q in table %q silently overrides the definition from %s",
			sh.Col.Name.Name(), ti.Decl.Name.Name(), originOf(sh)))
	}
	return out
}

func originOf(cd *check.ColumnDef) string {
	if cd.Partial == nil {
		return "the table body"
	}
	return fmt.Sprintf("TablePartial %q", cd.Partial.Name.Name())
}
