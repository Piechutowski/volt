package lang

import (
	"github.com/Piechutowski/volt/lang/diag"
)

// Vet reports legal-but-suspicious Volt (the lang-layer analog of
// edbml/vet). Run after Check on a checked project. Warnings never
// change what conforming Volt is.
func Vet(pr *Project) []diag.Diagnostic {
	var out []diag.Diagnostic
	c := &checker{pr: pr}
	for _, path := range c.paths() {
		pkg := pr.Packages[path]
		out = append(out, vetUnusedPipelines(pkg)...)
		out = append(out, vetResourceNamesTable(pkg)...)
	}
	diag.Sort(out)
	return out
}

// vetUnusedPipelines flags pipelines no scope pipes through: dead
// middleware declarations that silently protect nothing.
func vetUnusedPipelines(pkg *Package) []diag.Diagnostic {
	if pkg.Pipelines == nil {
		return nil
	}
	used := map[string]bool{}
	for _, r := range pkg.Routes {
		for _, p := range r.Pipes {
			used[p] = true
		}
	}
	var out []diag.Diagnostic
	for name, pl := range pkg.Pipelines {
		if !used[name] {
			out = append(out, diag.Warningf(pl.Pos(), "vet/pipeline",
				"Pipeline %q is declared and never piped through by any scope", name))
		}
	}
	return out
}

// vetResourceNamesTable flags a declaration that spells a table's model
// name instead of the table's own name: the reference should read as the
// schema does (§V5.1).
func vetResourceNamesTable(pkg *Package) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, h := range pkg.resourceHints {
		out = append(out, diag.Warningf(h.pos, "vet/resourcetable",
			"resources %s spells a model name; name the table as declared — `resources %s` — so the reference matches the schema and the key keeps its type (§V5.1)",
			h.declared, h.suggest))
	}
	return out
}
