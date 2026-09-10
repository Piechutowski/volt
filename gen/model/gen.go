// Package model renders the data half of a Volt package: nao's model,
// query and DDL output. It is the sibling of gen/router — same shape,
// same contract — so `volt gen` drives both from one project load and
// nothing needs a standalone ORM CLI.
package model

import (
	"github.com/Piechutowski/volt/internal/par"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/nao/gen/golang"
	"github.com/Piechutowski/volt/nao/gen/sqlite"
)

// Options configures generation.
type Options struct {
	// Source names the input, recorded in the generated headers.
	Source string
	// Package overrides the package clause; empty means the Volt
	// package's name. `volt gen -o` sets it from the directory the
	// files land in (§V1.7).
	Package string
	// ModelsOnly emits just nao_models.go: the structs, params structs
	// and enums — every type the wire carries (D77).
	ModelsOnly bool
	// SQL additionally emits the SQLite DDL and seed inserts.
	SQL bool
}

// File is one generated file: its base name and its contents.
type File struct {
	Name string
	Code []byte
}

// Generate renders the data files for one checked package. The package
// must be free of check errors and declare data elements.
func Generate(pkg *lang.Package, opts Options) ([]File, error) {
	gopts := golang.Options{Package: pkg.Name, Source: opts.Source}
	if opts.Package != "" {
		gopts.Package = opts.Package
	}
	// One plan per package (D74): the checker built it; every file below
	// reads it.
	plan := pkg.Plan()
	if plan == nil {
		plan = golang.PlanBuild(pkg.Merged(), pkg.Schema())
	}

	// Every file reads the plan and writes its own buffer, so the files
	// are emitted on every CPU (PERF-7) and gathered in this order.
	type emit struct {
		name string
		fn   func() ([]byte, error)
	}
	emits := []emit{{"nao_models.go", func() ([]byte, error) { return plan.Models(gopts) }}}
	if !opts.ModelsOnly {
		emits = append(emits,
			emit{"nao_queries.go", func() ([]byte, error) { return plan.Queries(gopts) }},
			emit{"nao_dyn.go", func() ([]byte, error) { return plan.Dyn(gopts) }})
		if fns := pkg.SelectFns(); len(fns) > 0 {
			emits = append(emits, emit{"nao_selects.go", func() ([]byte, error) { return plan.Selects(fns, gopts) }})
		}
		if len(pkg.CheckFns) > 0 {
			emits = append(emits, emit{"nao_validate.go", func() ([]byte, error) { return plan.Validators(pkg.CheckFns, gopts) }})
		}
	}
	if opts.SQL {
		emits = append(emits, emit{"nao_schema.sql", func() ([]byte, error) {
			return sqlite.Generate(pkg.Merged(), pkg.Schema(), sqlite.Options{Source: opts.Source})
		}})
	}
	out := make([]File, len(emits))
	errs := make([]error, len(emits))
	par.For(len(emits), func(i int) {
		code, err := emits[i].fn()
		out[i], errs[i] = File{emits[i].name, code}, err
	})
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
