// Package lang implements the project-level semantics of the Volt
// language (docs/spec.md Part II): package and import resolution, pipeline and
// scope checking, route expansion and conflict detection. The pipeline
// is Load then Check; diagnostic codes cite the spec section they
// enforce, e.g. "spec/imports".
package lang

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Piechutowski/volt/internal/par"
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
	"github.com/Piechutowski/volt/lang/source"
	"github.com/Piechutowski/volt/lang/token"
	"github.com/Piechutowski/volt/nao/naming"
)

// ModFile is the project root marker (spec §projects-and-packages): the Go module file.
// A Volt project is a Go module.
const ModFile = "go.mod"

// Project is a loaded Volt project: the Go module path and the packages
// (directories of .volt files) that were loaded — the ones asked for
// and, transitively, the ones they import (§projects-and-packages).
type Project struct {
	Root     string // absolute path of the directory holding go.mod
	Module   string // the Go module path, from go.mod's module directive
	Packages map[string]*Package
	src      source.Source

	// Diags collects load-time diagnostics (parse errors, go.mod
	// problems). Check appends semantic diagnostics.
	Diags []diag.Diagnostic
}

// Source is where the project was loaded from (spec
// §values-and-effects); Check reads the packages' Go files through it.
func (pr *Project) Source() source.Source { return pr.src }

// Package is one directory of .volt files sharing a namespace (§projects-and-packages).
type Package struct {
	Path  string // root-relative slash path; "." for the root directory
	Dir   string // absolute directory
	Name  string // declared package name (from the package clauses)
	Files []*ast.File

	// Imports maps qualifier -> imported package path, resolved by Check.
	Imports     map[string]string
	importSpecs []*ast.ImportSpec

	// Volt-layer results, populated by Check.
	Pipelines map[string]*ast.Pipeline
	Routes    []*RouteInfo
	// Controllers maps controller name -> its actions, for the generator.
	Controllers map[string]*ControllerInfo

	// Groups, Preds and Selects are the data-query layer (§groups to §selects-over-groups),
	// resolved by Check.
	Groups map[string]*GroupInfo
	Preds  map[string]*ast.Pred
	// Validators is the validator surface (§validators), lowered by Check.
	Validators     []naming.ValidatorFn
	Selects        []*SelectInfo
	selectByMethod map[string]selectMember
	selectByName   map[string]*SelectInfo

	schema *check.Info
	plan   *naming.Plan

	merged *ast.File
}

// HasRouting reports whether the package declares any scope, the one
// declaration that gives it routes (§data-and-network-layers).
func (p *Package) HasRouting() bool {
	for _, d := range p.merged.Decls {
		if _, ok := d.(*ast.Scope); ok {
			return true
		}
	}
	return false
}

// Merged returns the package's declarations as one synthetic file.
func (p *Package) Merged() *ast.File { return p.merged }

// Schema returns the package's checked table model (nil before Check).
func (p *Package) Schema() *check.Info { return p.schema }

// Plan returns the package's naming plan (nil before Check): every
// generated Go and SQL name, decided once.
func (p *Package) Plan() *naming.Plan { return p.plan }

// HasSchema reports whether the package declares data elements.
func (p *Package) HasSchema() bool {
	return p.schema != nil && len(p.schema.Tables) > 0
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
// (spec §projects-and-packages): the Go module is the Volt project.
func FindRoot(src source.Source, dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if src.Exists(filepath.Join(abs, ModFile)) {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("no %s found in %s or any parent directory (a Volt project is a Go module, §projects-and-packages)", ModFile, dir)
		}
		abs = parent
	}
}

// Load loads every package under root — the ./... pattern (§projects-and-packages),
// with §projects-and-packages's exclusions. Parse errors are collected, not fatal.
func Load(src source.Source, root string) (*Project, error) {
	return LoadOverlay(src, root, nil)
}

// LoadOverlay is Load with in-memory contents taking precedence over
// the disk: overlay maps absolute file paths to their current text.
func LoadOverlay(src source.Source, root string, overlay map[string]string) (*Project, error) {
	dirs, err := PackageDirs(src, root, root)
	if err != nil {
		return nil, err
	}
	return LoadDirs(src, root, dirs, overlay)
}

// PackageDirs enumerates the directories under dir that hold .volt
// files — the ./... pattern — applying §projects-and-packages: directories named with a
// leading dot or underscore, testdata and node_modules are skipped, and
// so is any subtree with its own go.mod (a different project).
func PackageDirs(src source.Source, root, dir string) ([]string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	var walk func(path string) ([]string, error)
	walk = func(path string) ([]string, error) {
		if path != absRoot && src.Exists(filepath.Join(path, ModFile)) {
			return nil, nil
		}
		entries, err := src.ReadDir(path)
		if err != nil {
			return nil, err
		}
		var out []string
		volt := false
		for _, e := range entries {
			switch {
			case e.Dir && !excludedDir(e.Name):
				sub, err := walk(filepath.Join(path, e.Name))
				if err != nil {
					return nil, err
				}
				out = append(out, sub...)
			case !e.Dir && strings.HasSuffix(e.Name, ".volt"):
				volt = true
			}
		}
		if volt {
			out = append([]string{path}, out...)
		}
		return out, nil
	}
	return walk(abs)
}

func excludedDir(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") ||
		name == "testdata" || name == "node_modules"
}

func hasVoltFiles(src source.Source, dir string) bool {
	entries, err := src.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.Dir && strings.HasSuffix(e.Name, ".volt") {
			return true
		}
	}
	return false
}

// LoadDirs loads the packages in dirs and, transitively, the packages
// they import (§projects-and-packages). Every dir MUST lie under root; a dir without
// .volt files is an error.
func LoadDirs(src source.Source, root string, dirs []string, overlay map[string]string) (*Project, error) {
	return loadDirs(src, root, dirs, overlay, nil)
}

func loadDirs(src source.Source, root string, dirs []string, overlay map[string]string, s *Session) (*Project, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	pr := &Project{Root: abs, Packages: map[string]*Package{}, src: src}

	modPath := filepath.Join(abs, ModFile)
	modSrc, err := src.ReadFile(modPath)
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
		if !hasVoltFiles(src, ad) {
			return nil, fmt.Errorf("no Volt package in %s (no .volt files)", d)
		}
		queue = append(queue, ad)
	}

	seen := map[string]bool{}
	for wave := queue; len(wave) > 0; {
		var todo []string
		for _, dir := range wave {
			if !seen[dir] {
				seen[dir] = true
				todo = append(todo, dir)
			}
		}
		sort.Strings(todo)
		pkgs, err := pr.packagesParse(todo, overlay, s)
		if err != nil {
			return nil, err
		}
		wave = nil
		for _, pkg := range pkgs {
			if pkg == nil {
				continue
			}
			pr.Packages[pkg.Path] = pkg
			for _, f := range pkg.Files {
				for _, d := range f.Decls {
					id, ok := d.(*ast.ImportDecl)
					if !ok {
						continue
					}
					for _, spec := range id.Specs {
						target := filepath.Join(abs, filepath.FromSlash(spec.PathString()))
						if why := importExcluded(src, abs, spec.PathString()); why != "" {
							pr.Diags = append(pr.Diags, diag.Errorf(spec.Pos(), "spec/projects-and-packages",
								"import %q names %s, which is not part of this project (§projects-and-packages)", spec.PathString(), why))
							continue
						}
						if hasVoltFiles(src, target) {
							wave = append(wave, target)
						}
					}
				}
			}
		}
	}
	return pr, nil
}

func importExcluded(src source.Source, root, path string) string {
	dir := root
	for _, seg := range strings.Split(path, "/") {
		if seg == "" || seg == "." {
			continue
		}
		if excludedDir(seg) {
			return "an excluded directory (" + seg + ")"
		}
		dir = filepath.Join(dir, seg)
		if src.Exists(filepath.Join(dir, ModFile)) {
			return "a nested module (" + seg + " has its own " + ModFile + ")"
		}
	}
	return ""
}

func (pr *Project) packagesParse(dirs []string, overlay map[string]string, s *Session) ([]*Package, error) {
	type fileParse struct {
		pkg   int
		path  string
		file  *ast.File
		diags []diag.Diagnostic
		err   error
	}
	pkgs := make([]*Package, len(dirs))
	var jobs []fileParse
	for i, dir := range dirs {
		entries, err := pr.src.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(pr.Root, dir)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.Dir || !strings.HasSuffix(e.Name, ".volt") {
				continue
			}
			if pkgs[i] == nil {
				pkgs[i] = &Package{Path: filepath.ToSlash(rel), Dir: dir, Imports: map[string]string{}}
			}
			jobs = append(jobs, fileParse{pkg: i, path: filepath.Join(dir, e.Name)})
		}
	}
	par.For(len(jobs), func(j int) {
		job := &jobs[j]
		if s != nil {
			job.file, job.diags, job.err = s.parse(pr.src, job.path, overlay)
			return
		}
		text, overlaid := overlay[job.path]
		if !overlaid {
			b, err := pr.src.ReadFile(job.path)
			if err != nil {
				job.err = err
				return
			}
			text = string(b)
		}
		job.file, job.diags = parser.ParseFile(job.path, text)
	})
	for j := range jobs {
		job := &jobs[j]
		if job.err != nil {
			return nil, job.err
		}
		pkgs[job.pkg].Files = append(pkgs[job.pkg].Files, job.file)
		pr.Diags = append(pr.Diags, job.diags...)
	}
	for _, pkg := range pkgs {
		if pkg == nil {
			continue
		}
		sort.Slice(pkg.Files, func(i, j int) bool { return pkg.Files[i].Name < pkg.Files[j].Name })
		merged := &ast.File{Name: "<package " + pkg.Path + ">"}
		for _, f := range pkg.Files {
			merged.Decls = append(merged.Decls, f.Decls...)
			merged.EOF = f.EOF
		}
		pkg.merged = merged
	}
	return pkgs, nil
}

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
	return "", []diag.Diagnostic{diag.Errorf(token.At(path, 0, 1, 1),
		"spec/projects-and-packages", "%s must declare 'module <path>' (§projects-and-packages)", ModFile)}
}
