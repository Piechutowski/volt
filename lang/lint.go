package lang

import (
	"github.com/Piechutowski/volt/internal/par"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/vet"
)

// Vet reports legal-but-suspicious Volt: every lang/vet analyzer
// (docs/lint.md) over each package's declarations as one file, and the
// project-level rule for pipelines no scope pipes through. Run after
// Check on a checked project.
func Vet(pr *Project) []diag.Diagnostic {
	return vetWith(pr, nil)
}

func vetWith(pr *Project, s *Session) []diag.Diagnostic {
	c := &checker{pr: pr}
	paths := c.paths()
	per := make([][]diag.Diagnostic, len(paths))
	var keys map[string]pkgKey
	var memos map[string]*declMemo
	var funcs *goFuncsCache
	if s != nil {
		funcs = s.goFuncsFor(pr, paths)
		keys = s.packageKeys(pr, paths, funcs)
		memos = s.declMemos(paths)
	}
	fresh := make([]bool, len(paths))
	par.For(len(paths), func(i int) {
		pkg := pr.Packages[paths[i]]
		var memo *declMemo
		var srcs []GoSource
		if s != nil {
			if ds, ok := s.vetRestore(paths[i], keys[paths[i]]); ok {
				per[i] = ds
				return
			}
			memo, srcs = memos[paths[i]], funcs.srcs[pkg.Dir]
			fresh[i] = true
		} else {
			srcs = GoSourcesRead(pr.src, pkg.Dir)
		}
		per[i] = vetPackage(pkg, srcs, memo)
		if s != nil {
			s.vetStore(paths[i], keys[paths[i]], per[i])
		}
	})
	if s != nil {
		did := map[string]*declMemo{}
		for i, path := range paths {
			if fresh[i] {
				did[path] = memos[path]
			}
		}
		s.vetStats(did)
	}
	var out []diag.Diagnostic
	for _, ds := range per {
		out = append(out, ds...)
	}
	diag.Sort(out)
	return out
}

func vetPackage(pkg *Package, gosrcs []GoSource, memo *declMemo) []diag.Diagnostic {
	var declVet *vet.Memo
	var goVet *vet.GoMemo
	if memo != nil {
		declVet, goVet = &memo.vet, &memo.goVet
	}
	var out []diag.Diagnostic
	if pkg.HasSchema() {
		out = vet.RunMemo(pkg.merged, pkg.schema, pkg.plan, declVet, vet.All()...)
	}
	var srcs []vet.GoSource
	for _, s := range gosrcs {
		if !s.Head {
			srcs = append(srcs, vet.GoSource{Name: s.Name, Text: s.Text})
		}
	}
	if pkg.HasRouting() {
		out = append(out, vet.RunGoMemo(pkg.Dir, srcs, bodiesOf(pkg), goVet)...)
	} else {
		out = append(out, vet.RunGoMemo(pkg.Dir, srcs, nil, goVet, vet.ByName("packagedoc"))...)
	}
	return append(out, vetUnusedPipelines(pkg)...)
}

// bodiesOf is where each action of pkg receives its request body, as the
// generated interface declares it (spec §handler-references,
// §request-bodies): after the request, the scope loads and the route's
// parameters.
func bodiesOf(pkg *Package) []vet.Body {
	var out []vet.Body
	for _, c := range pkg.Controllers {
		for _, a := range c.Actions {
			if a.Body != nil {
				out = append(out, vet.Body{Action: a.Name, At: 1 + len(a.ScopeLoads) + len(a.Params), Type: a.Body.GoType()})
			}
		}
	}
	return out
}

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
				"pipeline %q is declared and never piped through by any scope", name))
		}
	}
	return out
}
