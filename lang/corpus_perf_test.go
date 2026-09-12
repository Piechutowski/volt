package lang_test

// The corpus (internal/corpus) at scale: linear growth per phase in
// every dimension, an allocation budget that pins the absolute cost
// of a fixed input, and benchmarks with a committed baseline
// (testdata/bench_baseline.txt) for humans to compare against — wall
// clock is never asserted (PERF-2).

import (
	"testing"

	"github.com/Piechutowski/volt/gen/model"
	"github.com/Piechutowski/volt/gen/router"
	"github.com/Piechutowski/volt/internal/corpus"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
)

func corpusRoot(t testing.TB, spec corpus.Spec) string {
	t.Helper()
	root := t.TempDir()
	if err := corpus.Write(root, spec); err != nil {
		t.Fatal(err)
	}
	return root
}

// corpusPhases measures the bytes each phase allocates on a project.
func corpusPhases(t testing.TB, root string) (load, check, gen uint64) {
	t.Helper()
	var pr *lang.Project
	load = allocated(func() {
		var err error
		if pr, err = lang.Load(root); err != nil {
			t.Fatal(err)
		}
	})
	check = allocated(func() {
		if diags := lang.Check(pr); diag.HasErrors(diags) {
			t.Fatal(diags)
		}
	})
	gen = allocated(func() { corpusGenerate(t, pr) })
	return load, check, gen
}

func corpusGenerate(t testing.TB, pr *lang.Project) {
	t.Helper()
	for path, pkg := range pr.Packages {
		if pkg.HasSchema() {
			if _, err := model.Generate(pkg, model.Options{Source: "package " + path, SQL: true}); err != nil {
				t.Fatal(err)
			}
		}
		if pkg.HasRouting() {
			if _, err := router.Generate(pkg, router.Options{Source: "package " + path}); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestCorpusScalesLinearly(t *testing.T) {
	base := corpus.Spec{Packages: 2, Tables: 8, Columns: 8}
	for _, dim := range []struct {
		name string
		grow func(corpus.Spec) corpus.Spec
	}{
		{"tables", func(s corpus.Spec) corpus.Spec { s.Tables *= 2; return s }},
		{"packages", func(s corpus.Spec) corpus.Spec { s.Packages *= 2; return s }},
		{"columns", func(s corpus.Spec) corpus.Spec { s.Columns *= 2; return s }},
	} {
		t.Run(dim.name, func(t *testing.T) {
			l1, c1, g1 := corpusPhases(t, corpusRoot(t, base))
			l2, c2, g2 := corpusPhases(t, corpusRoot(t, dim.grow(base)))
			linear(t, "load", l1, l2)
			linear(t, "check", c1, c2)
			linear(t, "generate", g1, g2)
		})
	}
}

// The allocation budget of a fixed corpus (3 packages, 12 tables, 12
// columns): measured on go1.27 when the numbers were last ratcheted,
// asserted within ten percent. A phase that grows past its budget is
// a regression to explain, or a budget to raise in the same commit
// with the reason; a phase that shrinks well below is a budget to
// lower.
var allocBudget = struct {
	load, check, gen uint64
}{
	load:  1491896,
	check: 2342672,
	gen:   8463680,
}

func TestAllocationBudget(t *testing.T) {
	load, check, gen := corpusPhases(t, corpusRoot(t, corpus.Spec{Packages: 3, Tables: 12, Columns: 12}))
	for _, ph := range []struct {
		name        string
		got, budget uint64
	}{{"load", load, allocBudget.load}, {"check", check, allocBudget.check}, {"generate", gen, allocBudget.gen}} {
		t.Logf("%-8s %10d B (budget %d B)", ph.name, ph.got, ph.budget)
		if ph.budget == 0 {
			t.Errorf("%s: no budget set; set allocBudget.%s from the measurement above", ph.name, ph.name)
			continue
		}
		if ph.got > ph.budget+ph.budget/10 {
			t.Errorf("%s allocates %d B, over its budget of %d B by more than ten percent", ph.name, ph.got, ph.budget)
		}
	}
}

var benchSpec = corpus.Spec{Packages: 4, Tables: 25, Columns: 12} // 100 tables

func BenchmarkLoad(b *testing.B) {
	root := corpusRoot(b, benchSpec)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := lang.Load(root); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCheck(b *testing.B) {
	root := corpusRoot(b, benchSpec)
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		pr, err := lang.Load(root)
		if err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		if diags := lang.Check(pr); diag.HasErrors(diags) {
			b.Fatal(diags)
		}
	}
}

func BenchmarkGenerate(b *testing.B) {
	root := corpusRoot(b, benchSpec)
	pr, err := lang.Load(root)
	if err != nil {
		b.Fatal(err)
	}
	if diags := lang.Check(pr); diag.HasErrors(diags) {
		b.Fatal(diags)
	}
	b.ReportAllocs()
	for b.Loop() {
		corpusGenerate(b, pr)
	}
}

// BenchmarkSessionEdit is the editor's cycle: one file changes, the
// session re-parses it and re-checks the packages that see it.
func BenchmarkSessionEdit(b *testing.B) {
	root := corpusRoot(b, benchSpec)
	var s lang.Session
	pr, err := s.Load(root, nil)
	if err != nil {
		b.Fatal(err)
	}
	s.Check(pr)
	path := root + "/d02/schema.volt"
	text := corpus.Files(benchSpec)["d02/schema.volt"]
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		edited := text + "\n// keystroke " + string(rune('a'+i%26)) + "\n"
		pr, err := s.Load(root, map[string]string{path: edited})
		if err != nil {
			b.Fatal(err)
		}
		if diags := s.Check(pr); diag.HasErrors(diags) {
			b.Fatal(diags)
		}
	}
}
