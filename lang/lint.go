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
	return vetWith(pr, nil)
}

// vetWith is Vet through a session's memo when one is given: a
// package's warnings are a function of the same inputs as its check
// results, so they are kept under the same key (D81).
func vetWith(pr *Project, s *Session) []diag.Diagnostic {
	c := &checker{pr: pr}
	paths := c.paths()
	per := make([][]diag.Diagnostic, len(paths))
	var keys map[string]pkgKey
	if s != nil {
		keys = s.packageKeys(pr, paths, s.goFuncsFor(pr, paths))
	}
	par.For(len(paths), func(i int) {
		pkg := pr.Packages[paths[i]]
		if s != nil {
			if ds, ok := s.vetRestore(paths[i], keys[paths[i]]); ok {
				per[i] = ds
				return
			}
		}
		per[i] = vetPackage(pkg)
		if s != nil {
			s.vetStore(paths[i], keys[paths[i]], per[i])
		}
	})
	var out []diag.Diagnostic
	for _, ds := range per {
		out = append(out, ds...)
	}
	diag.Sort(out)
	return out
}

// vetPackage runs every analyzer over one checked package.
func vetPackage(pkg *Package) []diag.Diagnostic {
	var out []diag.Diagnostic
	if pkg.HasSchema() {
		out = vet.RunWithPlan(pkg.merged, pkg.schema, pkg.plan, vet.All()...)
	}
	return append(out, vetUnusedPipelines(pkg)...)
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
