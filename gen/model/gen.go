// Package model renders the data half of a Volt package: nao's model,
// query and DDL output. It is the sibling of gen/router — same shape,
// same contract — so `volt gen` drives both from one project load and
// nothing needs a standalone ORM CLI.
package model

import (
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

	models, err := plan.Models(gopts)
	if err != nil {
		return nil, err
	}
	out := []File{{"nao_models.go", models}}

	if !opts.ModelsOnly {
		queries, err := plan.Queries(gopts)
		if err != nil {
			return nil, err
		}
		dyn, err := plan.Dyn(gopts)
		if err != nil {
			return nil, err
		}
		out = append(out, File{"nao_queries.go", queries}, File{"nao_dyn.go", dyn})
		if fns := pkg.SelectFns(); len(fns) > 0 {
			selects, err := plan.Selects(fns, gopts)
			if err != nil {
				return nil, err
			}
			out = append(out, File{"nao_selects.go", selects})
		}
		if len(pkg.CheckFns) > 0 {
			validators, err := plan.Validators(pkg.CheckFns, gopts)
			if err != nil {
				return nil, err
			}
			out = append(out, File{"nao_validate.go", validators})
		}
	}
	if opts.SQL {
		ddl, err := sqlite.Generate(pkg.Merged(), pkg.Schema(), sqlite.Options{Source: opts.Source})
		if err != nil {
			return nil, err
		}
		out = append(out, File{"nao_schema.sql", ddl})
	}
	return out, nil
}
