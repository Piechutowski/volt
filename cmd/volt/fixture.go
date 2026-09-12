package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/Piechutowski/volt/internal/corpus"
)

// fixtureCommand is `volt fixture`: it writes the synthetic project the
// scaling tests and the benchmarks run on (internal/corpus, PERF-2) to
// a directory, at any size, so check, gen and the language server can
// be timed by hand on a project no repository should carry (D80). The
// defaults are the reproduction that started the performance work: a
// thousand tables of a hundred and fifty columns over twenty packages.
func fixtureCommand() *cli.Command {
	return &cli.Command{
		Name:      "fixture",
		Usage:     "write a synthetic project that uses every language feature, at the given size, to time check, gen and the language server by hand",
		ArgsUsage: "DIR",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "packages", Value: 20, Usage: "`N` data packages routed by one routing package"},
			&cli.IntFlag{Name: "tables", Value: 50, Usage: "`N` tables per package, chained by a relation"},
			&cli.IntFlag{Name: "columns", Value: 150, Usage: "`N` data columns per table"},
			&cli.BoolFlag{Name: "single", Usage: "one package declares the schema and routes it (D76); -packages is ignored"},
		},
		Action: func(_ context.Context, c *cli.Command) error {
			return fixtureRun(c)
		},
	}
}

// fixtureRun writes the project. The directory must be new or empty:
// the command never writes over anything.
func fixtureRun(c *cli.Command) error {
	if c.Args().Len() != 1 {
		return cli.Exit("fixture: one argument, the directory to write", 2)
	}
	dir := c.Args().First()
	spec := corpus.Spec{
		Packages: c.Int("packages"),
		Tables:   c.Int("tables"),
		Columns:  c.Int("columns"),
		Single:   c.Bool("single"),
	}
	if spec.Single {
		spec.Packages = 1
	}
	if spec.Packages < 1 || spec.Tables < 1 || spec.Columns < 1 {
		return cli.Exit("fixture: -packages, -tables and -columns are at least 1", 2)
	}
	entries, err := os.ReadDir(dir)
	switch {
	case err == nil && len(entries) > 0:
		return cli.Exit(fmt.Sprintf("fixture: %s is not empty; the fixture goes into a new or empty directory", dir), 2)
	case err != nil && !os.IsNotExist(err):
		return cli.Exit("fixture: "+err.Error(), 2)
	}
	if err := corpus.Write(dir, spec); err != nil {
		return cli.Exit("fixture: "+err.Error(), 2)
	}
	layout := fmt.Sprintf("%d data packages and a routing package", spec.Packages)
	if spec.Single {
		layout = "one package holding schema and routes"
	}
	fmt.Printf("fixture: %d tables of %d columns, %s, in %s\n", spec.Packages*spec.Tables, spec.Columns, layout, dir)
	fmt.Printf("  volt check %s/...\n  volt gen --sql %s/...\n  open %s in the editor\n", dir, dir, dir)
	return nil
}
