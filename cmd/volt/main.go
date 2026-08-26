// Command volt is the one binary of the Volt language: the CLI front
// end over the lang package plus the router generator, built on
// urfave/cli in the style of nao (D41):
//
//	volt check  [--json] [dir]     load the project, run semantic analysis (SPEC.md §V)
//	volt vet    [--json] [dir]     check plus warnings for legal-but-suspicious Volt
//	volt gen    [dir]              generate volt_*.go for every routing package
//	volt routes [dir]              print the expanded route table
//	volt lsp                       language server on stdin/stdout
//	volt version                   report the tool version
//
// Every command resolves the project root by walking up from dir (or
// the working directory) to the nearest volt.mod.
//
// Exit status: 0 clean (warnings do not fail), 1 errors found, 2 usage
// or I/O problems.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/urfave/cli/v3"

	"github.com/Piechutowski/volt/gen/router"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lsp"
	"github.com/Piechutowski/volt/nao/edbml/diag"
)

const version = "0.1.0-dev"

func main() {
	jsonFlag := &cli.BoolFlag{Name: "json", Usage: "emit diagnostics as JSON"}

	app := &cli.Command{
		EnableShellCompletion: true,
		Name:                  "volt",
		Usage:                 "the Volt language: check, lint and generate routers from .volt packages",
		Commands: []*cli.Command{
			{
				Name:      "check",
				Usage:     "load the project and run semantic analysis (SPEC.md §V)",
				ArgsUsage: "[dir]",
				Flags:     []cli.Flag{jsonFlag},
				Action: func(_ context.Context, c *cli.Command) error {
					return run(c, "check")
				},
			},
			{
				Name:      "vet",
				Usage:     "check plus warnings for legal-but-suspicious Volt",
				ArgsUsage: "[dir]",
				Flags: []cli.Flag{
					jsonFlag,
					&cli.BoolFlag{Name: "werror", Usage: "treat warnings as errors in the exit status"},
				},
				Action: func(_ context.Context, c *cli.Command) error {
					return run(c, "vet")
				},
			},
			{
				Name:      "gen",
				Usage:     "generate volt_*.go router files for every package with routing elements",
				ArgsUsage: "[dir]",
				Action: func(_ context.Context, c *cli.Command) error {
					return genRun(c)
				},
			},
			{
				Name:      "routes",
				Usage:     "print the expanded route table (verb, path, handler, helper)",
				ArgsUsage: "[dir]",
				Action: func(_ context.Context, c *cli.Command) error {
					return routesRun(c)
				},
			},
			{
				Name:  "lsp",
				Usage: "run the Volt language server (LSP over stdin/stdout)",
				Action: func(context.Context, *cli.Command) error {
					return lsp.NewServer().RunStdio()
				},
			},
			{
				Name:  "version",
				Usage: "report the tool version",
				Action: func(_ context.Context, _ *cli.Command) error {
					fmt.Println("volt", version)
					return nil
				},
			},
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

// load resolves the project root from the command's optional dir
// argument and loads + checks the project.
func load(c *cli.Command) (*lang.Project, []diag.Diagnostic, error) {
	dir := "."
	switch c.Args().Len() {
	case 0:
	case 1:
		dir = c.Args().First()
	default:
		return nil, nil, cli.Exit("at most one project directory", 2)
	}
	root, err := lang.FindRoot(dir)
	if err != nil {
		return nil, nil, cli.Exit(err.Error(), 2)
	}
	pr, err := lang.Load(root)
	if err != nil {
		return nil, nil, cli.Exit(err.Error(), 2)
	}
	return pr, lang.Check(pr), nil
}

func run(c *cli.Command, mode string) error {
	pr, diags, err := load(c)
	if err != nil {
		return err
	}
	if mode == "vet" {
		diags = append(diags, lang.Vet(pr)...)
		diag.Sort(diags)
	}
	if c.Bool("json") {
		if err := diagsPrintJSON(diags); err != nil {
			return cli.Exit(err.Error(), 2)
		}
	} else {
		for _, d := range diags {
			fmt.Println(d)
		}
	}
	if diag.HasErrors(diags) || (mode == "vet" && c.Bool("werror") && len(diags) > 0) {
		return cli.Exit("", 1)
	}
	return nil
}

func diagsPrintJSON(all []diag.Diagnostic) error {
	type jsonDiag struct {
		File     string `json:"file"`
		Line     int    `json:"line"`
		Column   int    `json:"column"`
		Severity string `json:"severity"`
		Code     string `json:"code"`
		Message  string `json:"message"`
	}
	out := make([]jsonDiag, 0, len(all))
	for _, d := range all {
		out = append(out, jsonDiag{
			File: d.Pos.Filename, Line: d.Pos.Line, Column: d.Pos.Column,
			Severity: d.Severity.String(), Code: d.Code, Message: d.Msg,
		})
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// genRun implements 'volt gen': check the whole project, then write the
// four router files into every package with routing elements, refusing
// to clobber non-generated files (all or nothing, like nao gen).
func genRun(c *cli.Command) error {
	pr, diags, err := load(c)
	if err != nil {
		return err
	}
	if diag.HasErrors(diags) {
		for _, d := range diags {
			fmt.Println(d)
		}
		return cli.Exit("gen: project has errors; fix them first (see 'volt check')", 1)
	}

	paths := make([]string, 0, len(pr.Packages))
	for p := range pr.Packages {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	generated := false
	marker := []byte("// Code generated ")
	for _, path := range paths {
		pkg := pr.Packages[path]
		if !pkg.HasRouting() {
			continue
		}
		generated = true
		files, err := router.Generate(pkg, router.Options{Source: "package " + pkg.Path})
		if err != nil {
			return cli.Exit("gen: "+err.Error(), 1)
		}
		// Refuse every clobber before writing anything: all or nothing.
		for _, name := range router.Files {
			target := filepath.Join(pkg.Dir, name)
			if old, err := os.ReadFile(target); err == nil && !bytes.HasPrefix(old, marker) {
				return cli.Exit(fmt.Sprintf("gen: refusing to overwrite %s: it lacks the generated-code header", target), 2)
			}
		}
		for _, name := range router.Files {
			target := filepath.Join(pkg.Dir, name)
			if err := os.WriteFile(target, files[name], 0o644); err != nil {
				return cli.Exit(err.Error(), 2)
			}
			fmt.Println(target)
		}
	}
	if !generated {
		fmt.Println("gen: no package declares routing elements; nothing to do")
	}
	return nil
}

// routesRun implements 'volt routes': the introspection table.
func routesRun(c *cli.Command) error {
	pr, diags, err := load(c)
	if err != nil {
		return err
	}
	if diag.HasErrors(diags) {
		for _, d := range diags {
			fmt.Println(d)
		}
		return cli.Exit("routes: project has errors; fix them first (see 'volt check')", 1)
	}
	paths := make([]string, 0, len(pr.Packages))
	for p := range pr.Packages {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, path := range paths {
		pkg := pr.Packages[path]
		if len(pkg.Routes) == 0 {
			continue
		}
		fmt.Printf("package %s\n", pkg.Path)
		for _, r := range pkg.Routes {
			fmt.Printf("  %s\n", r)
		}
	}
	return nil
}
