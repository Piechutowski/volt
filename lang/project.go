// Package lang implements the project-level semantics of the Volt
// language (docs/spec.md §V): package and import resolution, pipeline and
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
	"github.com/Piechutowski/volt/nao/gen/golang"
)

// ModFile is the project root marker (spec §V1.1): the Go module file.
// A Volt project is a Go module — one root, one module path, no second
// manifest (D62).
const ModFile = "go.mod"

// Project is a loaded Volt project: the Go module path and the packages
// (directories of .volt files) that were loaded — the ones asked for
// and, transitively, the ones they import (§V1.7).
type Project struct {
	Root     string // absolute path of the directory holding go.mod
	Module   string // the Go module path, from go.mod's module directive
	Packages map[string]*Package

	// Diags collects load-time diagnostics (parse errors, go.mod
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

	// Groups, Preds and Selects are the data-query layer (§V9-§V11),
	// resolved by Check.
	Groups map[string]*GroupInfo
	Preds  map[string]*ast.Pred
	// CheckFns is the validator surface (§V12), lowered by Check.
	CheckFns []golang.CheckFn
	Selects  []*SelectInfo

	// schema is the package's checked table model, set by Check.
	schema *check.Info

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

// PackageAt returns the loaded package whose directory is dir, or nil.
func (pr *Project) PackageAt(dir string) *Package {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil
	}
	for _, pkg := range pr.Packages {
		if pkg.Dir == abs {
			return pkg
		}
	}
	return nil
}

// FindRoot walks up from dir to the nearest directory holding go.mod
// (spec §V1.1): the Go module is the Volt project.
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
			return "", fmt.Errorf("no %s found in %s or any parent directory (a Volt project is a Go module, §V1.1)", ModFile, dir)
		}
		abs = parent
	}
}

// Load loads every package under root — the ./... pattern (§V1.7),
// with §V1.6's exclusions. Parse errors are collected, not fatal:
// Check still runs on what parsed, like the rest of the toolchain.
func Load(root string) (*Project, error) {
	return LoadOverlay(root, nil)
}

// LoadOverlay is Load with in-memory contents taking precedence over
// the disk: overlay maps absolute file paths to their current text.
// This is the language server's view of a project — open editor
// buffers override what was last saved.
func LoadOverlay(root string, overlay map[string]string) (*Project, error) {
	dirs, err := PackageDirs(root, root)
	if err != nil {
		return nil, err
	}
	return LoadDirs(root, dirs, overlay)
}

// PackageDirs enumerates the directories under dir that hold .volt
// files — the ./... pattern — applying §V1.6: directories named with a
// leading dot or underscore, testdata and node_modules are skipped, and
// so is any subtree with its own go.mod (a different project).
func PackageDirs(root, dir string) ([]string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	err = filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if path != abs && excludedDir(d.Name()) {
			return filepath.SkipDir
		}
		if path != absRoot {
			if _, err := os.Stat(filepath.Join(path, ModFile)); err == nil {
				return filepath.SkipDir
			}
		}
		if hasVoltFiles(path) {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// excludedDir applies §V1.6's directory exclusions (Go's own rules).
func excludedDir(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") ||
		name == "testdata" || name == "node_modules"
}

func hasVoltFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".volt") {
			return true
		}
	}
	return false
}

// LoadDirs loads the packages in dirs and, transitively, the packages
// they import (§V1.7) — the way Go loads what a build needs and nothing
// more, so stray .volt trees elsewhere in the module never interfere.
// Every dir MUST lie under root; a dir without .volt files is an error.
func LoadDirs(root string, dirs []string, overlay map[string]string) (*Project, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	pr := &Project{Root: abs, Packages: map[string]*Package{}}

	modPath := filepath.Join(abs, ModFile)
	modSrc, err := os.ReadFile(modPath)
	if err != nil {
		return nil, fmt.Errorf("not a Volt project: %w", err)
	}
	pr.Module, pr.Diags = goModModule(modPath, string(modSrc))

	queue := make([]string, 0, len(dirs))
	for _, d := range dirs {
		ad, err := filepath.Abs(d)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(abs, ad)
		if err != nil || strings.HasPrefix(rel, "..") {
			return nil, fmt.Errorf("%s is outside the project rooted at %s", d, abs)
		}
		if !hasVoltFiles(ad) {
			return nil, fmt.Errorf("no Volt package in %s (no .volt files)", d)
		}
		queue = append(queue, ad)
	}

	seen := map[string]bool{}
	for len(queue) > 0 {
		dir := queue[0]
		queue = queue[1:]
		if seen[dir] {
			continue
		}
		seen[dir] = true
		pkg, err := pr.packageLoad(dir, overlay)
		if err != nil {
			return nil, err
		}
		if pkg == nil {
			continue
		}
		// Imports name root-relative package paths (§V2); an existing
		// directory is loaded, a missing one is left for Check to report.
		for _, f := range pkg.Files {
			for _, d := range f.Decls {
				id, ok := d.(*ast.ImportDecl)
				if !ok {
					continue
				}
				for _, spec := range id.Specs {
					target := filepath.Join(abs, filepath.FromSlash(spec.PathString()))
					if why := importExcluded(abs, spec.PathString()); why != "" {
						pr.Diags = append(pr.Diags, diag.Errorf(spec.Pos(), "spec/V1",
							"import %q names %s, which is not part of this project (§V1.6)", spec.PathString(), why))
						continue
					}
					if hasVoltFiles(target) {
						queue = append(queue, target)
					}
				}
			}
		}
	}
	return pr, nil
}

// importExcluded reports why a root-relative import path is outside
// the project (§V1.6): a segment the exclusions cover, or a nested
// module boundary between the root and the target. "" means allowed.
func importExcluded(root, path string) string {
	dir := root
	for _, seg := range strings.Split(path, "/") {
		if seg == "" || seg == "." {
			continue
		}
		if excludedDir(seg) {
			return "an excluded directory (" + seg + ")"
		}
		dir = filepath.Join(dir, seg)
		if _, err := os.Stat(filepath.Join(dir, ModFile)); err == nil {
			return "a nested module (" + seg + " has its own " + ModFile + ")"
		}
	}
	return ""
}

// packageLoad parses one directory's .volt files into a package (nil
// when the directory holds none), files in name order (§V1.5).
func (pr *Project) packageLoad(dir string, overlay map[string]string) (*Package, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(pr.Root, dir)
	if err != nil {
		return nil, err
	}
	key := filepath.ToSlash(rel)
	var pkg *Package
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".volt") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		src := ""
		if text, ok := overlay[path]; ok {
			src = text
		} else {
			b, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			src = string(b)
		}
		if pkg == nil {
			pkg = &Package{Path: key, Dir: dir, Imports: map[string]string{}}
			pr.Packages[key] = pkg
		}
		f, diags := parser.ParseFile(path, src)
		pkg.Files = append(pkg.Files, f)
		pr.Diags = append(pr.Diags, diags...)
	}
	if pkg == nil {
		return nil, nil
	}
	sort.Slice(pkg.Files, func(i, j int) bool { return pkg.Files[i].Name < pkg.Files[j].Name })
	merged := &ast.File{Name: "<package " + pkg.Path + ">"}
	for _, f := range pkg.Files {
		merged.Decls = append(merged.Decls, f.Decls...)
		merged.EOF = f.EOF
	}
	pkg.merged = merged
	return pkg, nil
}

// goModModule reads the module directive of go.mod — the only line Volt
// needs; every other directive is Go's business and ignored. The scan
// honors go.mod's lexical structure: // and /* */ comments are not
// text, and a directive inside a parenthesized block (require (...))
// is not the top-level module directive.
func goModModule(path, src string) (string, []diag.Diagnostic) {
	inBlockComment, depth := false, 0
	for _, line := range strings.Split(src, "\n") {
		var text strings.Builder
		for i := 0; i < len(line); i++ {
			switch {
			case inBlockComment:
				if strings.HasPrefix(line[i:], "*/") {
					inBlockComment = false
					i++
				}
			case strings.HasPrefix(line[i:], "/*"):
				inBlockComment = true
				i++
			case strings.HasPrefix(line[i:], "//"):
				i = len(line)
			default:
				text.WriteByte(line[i])
			}
		}
		fields := strings.Fields(text.String())
		if len(fields) == 0 {
			continue
		}
		if depth == 0 && fields[0] == "module" && len(fields) >= 2 && fields[1] != "(" {
			return strings.Trim(fields[1], "\""), nil
		}
		for _, f := range fields {
			switch f {
			case "(":
				depth++
			case ")":
				if depth > 0 {
					depth--
				}
			}
		}
	}
	return "", []diag.Diagnostic{diag.Errorf(token.Position{Filename: path, Line: 1, Column: 1},
		"spec/V1", "%s must declare 'module <path>' (§V1.1)", ModFile)}
}
