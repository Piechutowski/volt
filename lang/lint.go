package lang

import (
	"github.com/Piechutowski/volt/internal/par"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/vet"
)

// Vet reports legal-but-suspicious Volt: every lang/vet analyzer
// (docs/lint.md) over each package's declarations as one file, the way
// the checker sees them, and the project-level rule for pipelines no
// scope pipes through. Run after Check on a checked project. Warnings
// never change what conforming Volt is.
func Vet(pr *Project) []diag.Diagnostic {
	c := &checker{pr: pr}
	paths := c.paths()
	per := make([][]diag.Diagnostic, len(paths))
	par.For(len(paths), func(i int) {
		pkg := pr.Packages[paths[i]]
		if pkg.HasSchema() {
			per[i] = vet.Run(pkg.merged, pkg.schema, vet.All()...)
		}
		per[i] = append(per[i], vetUnusedPipelines(pkg)...)
	})
	var out []diag.Diagnostic
	for _, ds := range per {
		out = append(out, ds...)
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
