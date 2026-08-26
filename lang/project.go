// Package lang implements the project-level semantics of the Volt
// language (SPEC.md §V): package and import resolution, pipeline and
// scope checking, route expansion and conflict detection. It plays the
// role lang/check plays for the schema layer, one level up — whole
// project instead of single file.
//
// The pipeline is Load (files → packages) then Check (packages →
// routes), mirroring parse-then-check. Diagnostic codes cite the spec
// section they enforce, e.g. "spec/V2".
package lang

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
	"github.com/Piechutowski/volt/lang/token"
)

// ModFile is the project root marker (spec §V1.1).
const ModFile = "volt.mod"

// Project is a loaded Volt project: the volt.mod module name and every
// package (directory of .volt files) beneath the root.
type Project struct {
	Root     string // absolute path of the directory holding volt.mod
	Module   string // module name from volt.mod
	Packages map[string]*Package

	// Diags collects load-time diagnostics (parse errors, volt.mod
	// problems). Check appends semantic diagnostics.
	Diags []diag.Diagnostic
}

// Package is one directory of .volt files sharing a namespace (§V1).
type Package struct {
	Path  string // root-relative slash path; "." for the root directory
	Dir   string // absolute directory
	Name  string // declared package name (from the package clauses)
	Files []*ast.File

	// Imports maps qualifier -> imported package path, resolved by Check.
	Imports map[string]string
	// importSpecs is every import spec across the package's files.
	importSpecs []*ast.ImportSpec

	// Volt-layer results, populated by Check.
	Pipelines map[string]*ast.Pipeline
	Routes    []*RouteInfo
	// Controllers maps controller name -> its actions, for the generator.
	Controllers map[string]*ControllerInfo

	// schema is the package's checked table model, set by Check.
	schema *check.Info

	// resourceHints records `resources <table>` declarations that could
	// have named a model instead; Vet turns them into advice (§V5.1).
	resourceHints []resourceHint

	// merged is the synthetic single file of all declarations, in file
	// order (sorted by name for determinism), fed to the DBML-layer
	// checker for table semantics.
	merged *ast.File
}

// HasRouting reports whether the package declares any routing elements
// (pipelines or scopes) — such packages get generated router files.
func (p *Package) HasRouting() bool {
	for _, d := range p.merged.Decls {
		switch d.(type) {
		case *ast.Pipeline, *ast.Scope:
			return true
		}
	}
	return false
}

// Merged returns the package's declarations as one synthetic file.
func (p *Package) Merged() *ast.File { return p.merged }

// Schema returns the package's checked table model (nil before Check).
func (p *Package) Schema() *check.Info { return p.schema }

// HasSchema reports whether the package declares data elements — such
// packages get generated model, query and DDL files.
func (p *Package) HasSchema() bool {
	return p.schema != nil && (len(p.schema.Tables) > 0 || len(p.schema.Enums) > 0)
}

// FindRoot walks up from dir to the nearest directory containing
// volt.mod (§V1.1).
func FindRoot(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, ModFile)); err == nil {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("no %s found in %s or any parent directory", ModFile, dir)
		}
		abs = parent
	}
}

// Load reads volt.mod and parses every .volt file under root into
// packages. Parse errors are collected, not fatal: Check still runs on
// what parsed, like the rest of the toolchain.
func Load(root string) (*Project, error) {
	return LoadOverlay(root, nil)
}

// LoadOverlay is Load with in-memory contents taking precedence over
// the disk: overlay maps absolute file paths to their current text.
// This is the language server's view of a project — open editor
// buffers override what was last saved.
func LoadOverlay(root string, overlay map[string]string) (*Project, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	pr := &Project{Root: abs, Packages: map[string]*Package{}}

	modSrc, err := os.ReadFile(filepath.Join(abs, ModFile))
	if err != nil {
		return nil, fmt.Errorf("not a Volt project: %w", err)
	}
	pr.Module, pr.Diags = parseModFile(filepath.Join(abs, ModFile), string(modSrc))

	err = filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != abs && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "node_modules") {
				return filepath.SkipDir
			}
			// A subdirectory with its own volt.mod is a different project
			// (§V1.1): its packages belong to it, not to this module.
			if path != abs {
				if _, err := os.Stat(filepath.Join(path, ModFile)); err == nil {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".volt") {
			return nil
		}
		src := ""
		if text, ok := overlay[path]; ok {
			src = text
		} else {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			src = string(b)
		}
		rel, err := filepath.Rel(abs, filepath.Dir(path))
		if err != nil {
			return err
		}
		key := filepath.ToSlash(rel)
		pkg := pr.Packages[key]
		if pkg == nil {
			pkg = &Package{Path: key, Dir: filepath.Dir(path), Imports: map[string]string{}}
			pr.Packages[key] = pkg
		}
		f, diags := parser.ParseFile(path, src)
		pkg.Files = append(pkg.Files, f)
		pr.Diags = append(pr.Diags, diags...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	for _, pkg := range pr.Packages {
		sort.Slice(pkg.Files, func(i, j int) bool { return pkg.Files[i].Name < pkg.Files[j].Name })
		merged := &ast.File{Name: "<package " + pkg.Path + ">"}
		for _, f := range pkg.Files {
			merged.Decls = append(merged.Decls, f.Decls...)
			merged.EOF = f.EOF
		}
		pkg.merged = merged
	}
	return pr, nil
}

// parseModFile reads volt.mod: comments, blank lines, and exactly one
// "module <name>" directive (§V1.1).
func parseModFile(path, src string) (string, []diag.Diagnostic) {
	var diags []diag.Diagnostic
	module := ""
	for i, line := range strings.Split(src, "\n") {
		pos := token.Position{Filename: path, Line: i + 1, Column: 1}
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		fields := strings.Fields(line)
		switch {
		case fields[0] == "module" && len(fields) == 2:
			if module != "" {
				diags = append(diags, diag.Errorf(pos, "spec/V1", "duplicate module directive in %s", ModFile))
				continue
			}
			module = fields[1]
		default:
			diags = append(diags, diag.Errorf(pos, "spec/V1", "unsupported %s directive %q (only 'module <name>')", ModFile, fields[0]))
		}
	}
	if module == "" {
		diags = append(diags, diag.Errorf(token.Position{Filename: path, Line: 1, Column: 1},
			"spec/V1", "%s must declare 'module <name>'", ModFile))
	}
	return module, diags
}

// resourceHint is one 'name the model instead' suggestion.
type resourceHint struct {
	pos               token.Position
	declared, suggest string
}
