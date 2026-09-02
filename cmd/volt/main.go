// Command volt is the framework's one binary: the CLI front end over
// the lang package and both generators — nao's models and queries for
// data packages, routers for routing packages (D41).
//
//	volt check  [--json] [dir]     load the project, run semantic analysis (docs/spec.md §V)
//	volt vet    [--json] [dir]     check plus warnings for legal-but-suspicious Volt
//	volt gen    [dir]              generate models, queries and routers
//	volt routes [dir]              print the expanded route table
//	volt lsp                       language server on stdin/stdout
//	volt version                   report the tool version
//
// Every command resolves the project root by walking up from dir (or
// the working directory) to the nearest go.mod.
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
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/Piechutowski/volt/gen/model"
	"github.com/Piechutowski/volt/gen/router"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lsp"
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
				Usage:     "load the named packages and run semantic analysis (docs/spec.md §V)",
				ArgsUsage: "[packages]",
				Flags:     []cli.Flag{jsonFlag},
				Action: func(_ context.Context, c *cli.Command) error {
					return run(c, "check")
				},
			},
			{
				Name:      "vet",
				Usage:     "check plus warnings for legal-but-suspicious Volt",
				ArgsUsage: "[packages]",
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
				Usage:     "generate Go for the named packages: models and queries for data packages, routers for routing packages. Packages as in Go: none = the current directory, dir, or dir/... for everything beneath",
				ArgsUsage: "[packages]",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "models-only", Aliases: []string{"m"}, Usage: "emit only nao_models.go; skip the query layers"},
					&cli.BoolFlag{Name: "sql", Usage: "also write nao_schema.sql (SQLite DDL and seed inserts)"},
				},
				Action: func(_ context.Context, c *cli.Command) error {
					return genRun(c)
				},
			},
			{
				Name:      "routes",
				Usage:     "print the expanded route table (verb, path, handler, helper)",
				ArgsUsage: "[packages]",
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

// load resolves the packages named by the command's arguments — Go's
// rules (D39, §V1.7): none means the package in the working directory,
// a directory means that package, and dir/... means every package
// beneath dir. The project root is the nearest go.mod (§V1.1); the
// named packages and their imports are loaded and checked. named lists
// the package paths the arguments selected, in argument order.
func load(c *cli.Command) (pr *lang.Project, named []string, diags []diag.Diagnostic, err error) {
	args := c.Args().Slice()
	if len(args) == 0 {
		args = []string{"."}
	}
	root := ""
	var dirs []string
	for _, arg := range args {
		pattern := filepath.ToSlash(arg)
		base := arg
		recursive := false
		if strings.HasSuffix(pattern, "/...") || pattern == "..." {
			recursive = true
			base = filepath.FromSlash(strings.TrimSuffix(strings.TrimSuffix(pattern, "..."), "/"))
			if base == "" {
				base = "."
			}
		}
		r, err := lang.FindRoot(base)
		if err != nil {
			return nil, nil, nil, cli.Exit(err.Error(), 2)
		}
		if root == "" {
			root = r
		} else if root != r {
			return nil, nil, nil, cli.Exit(fmt.Sprintf("%s belongs to a different project (%s); one project per invocation", arg, r), 2)
		}
		if recursive {
			found, err := lang.PackageDirs(root, base)
			if err != nil {
				return nil, nil, nil, cli.Exit(err.Error(), 2)
			}
			if len(found) == 0 {
				return nil, nil, nil, cli.Exit(fmt.Sprintf("no Volt packages under %s", base), 2)
			}
			dirs = append(dirs, found...)
		} else {
			dirs = append(dirs, base)
		}
	}
	pr, err = lang.LoadDirs(root, dirs, nil)
	if err != nil {
		return nil, nil, nil, cli.Exit(err.Error(), 2)
	}
	seen := map[string]bool{}
	for _, d := range dirs {
		if pkg := pr.PackageAt(d); pkg != nil && !seen[pkg.Path] {
			seen[pkg.Path] = true
			named = append(named, pkg.Path)
		}
	}
	return pr, named, lang.Check(pr), nil
}

func run(c *cli.Command, mode string) error {
	pr, _, diags, err := load(c)
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
// to clobber non-generated files (all or nothing).
func genRun(c *cli.Command) error {
	pr, named, diags, err := load(c)
	if err != nil {
		return err
	}
	if diag.HasErrors(diags) {
		for _, d := range diags {
			fmt.Println(d)
		}
		return cli.Exit("gen: project has errors; fix them first (see 'volt check')", 1)
	}

	// Generate for the named packages only — imports are loaded for
	// checking, never written (Go's rule, D39).
	paths := append([]string(nil), named...)
	sort.Strings(paths)

	// Collect everything first, then refuse every clobber, then write:
	// all or nothing across the whole project.
	type outFile struct {
		path string
		code []byte
	}
	var out []outFile

	for _, path := range paths {
		pkg := pr.Packages[path]
		source := "package " + pkg.Path

		if pkg.HasSchema() {
			files, err := model.Generate(pkg, model.Options{
				Source: source, ModelsOnly: c.Bool("models-only"), SQL: c.Bool("sql"),
			})
			if err != nil {
				return cli.Exit("gen: "+err.Error(), 1)
			}
			for _, f := range files {
				out = append(out, outFile{filepath.Join(pkg.Dir, f.Name), f.Code})
			}
		}

		if pkg.HasRouting() {
			files, err := router.Generate(pkg, router.Options{Source: source})
			if err != nil {
				return cli.Exit("gen: "+err.Error(), 1)
			}
			for _, name := range router.Files {
				out = append(out, outFile{filepath.Join(pkg.Dir, name), files[name]})
			}
		}
	}
	if len(out) == 0 {
		fmt.Println("gen: no package declares data or routing elements; nothing to do")
		return nil
	}

	for _, f := range out {
		// SQL carries the marker in its own comment syntax.
		marker := []byte("// Code generated ")
		if filepath.Ext(f.path) == ".sql" {
			marker = []byte("-- Code generated ")
		}
		if old, err := os.ReadFile(f.path); err == nil && !bytes.HasPrefix(old, marker) {
			return cli.Exit(fmt.Sprintf("gen: refusing to overwrite %s: it lacks the generated-code header", f.path), 2)
		}
	}
	// An optional output that is no longer produced (every select or
	// check removed) must not linger and keep enforcing deleted rules:
	// a generated file — marker present — is ours to remove.
	produced := map[string]bool{}
	for _, f := range out {
		produced[f.path] = true
	}
	for _, path := range paths {
		pkg := pr.Packages[path]
		if !pkg.HasSchema() {
			continue
		}
		for _, name := range []string{"nao_selects.go", "nao_validate.go"} {
			stale := filepath.Join(pkg.Dir, name)
			if produced[stale] {
				continue
			}
			if old, err := os.ReadFile(stale); err == nil && bytes.HasPrefix(old, []byte("// Code generated ")) {
				if err := os.Remove(stale); err != nil {
					return cli.Exit(err.Error(), 2)
				}
				fmt.Println(stale, "(removed: no longer generated)")
			}
		}
	}
	for _, f := range out {
		if err := os.WriteFile(f.path, f.code, 0o644); err != nil {
			return cli.Exit(err.Error(), 2)
		}
		fmt.Println(f.path)
	}
	return nil
}

// routesRun implements 'volt routes': the introspection table.
func routesRun(c *cli.Command) error {
	pr, named, diags, err := load(c)
	if err != nil {
		return err
	}
	if diag.HasErrors(diags) {
		for _, d := range diags {
			fmt.Println(d)
		}
		return cli.Exit("routes: project has errors; fix them first (see 'volt check')", 1)
	}
	// Generate for the named packages only — imports are loaded for
	// checking, never written (Go's rule, D39).
	paths := append([]string(nil), named...)
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
