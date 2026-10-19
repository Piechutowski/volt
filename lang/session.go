package lang

import (
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/Piechutowski/volt/internal/par"
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
	"github.com/Piechutowski/volt/nao/gen/golang"
)

// Session caches parses and check results for one project root. The
// zero value is ready. A Session runs one operation at a time: a
// concurrent Load, Check or Vet waits for the one under way.
type Session struct {
	op    sync.Mutex
	mu    sync.Mutex
	files map[string]*fileEntry
	pkgs  map[string]*pkgEntry
	memos map[string]*declMemo
	scans map[string]*goDir
	gen   int
	stats SessionStats
}

// SessionStats counts the work a session did and the work it skipped.
type SessionStats struct {
	FilesParsed, FilesReused          int
	DeclsParsed, DeclsReused          int // top-level declarations across the files parsed
	PackagesChecked, PackagesReused   int
	PackagesVetted, PackagesVetReused int
	// Within the packages checked: tables whose schema check,
	// model and lowered checks were answered by the memo, and not.
	TablesChecked, TablesReused   int
	ModelsBuilt, ModelsReused     int
	ChecksLowered, ChecksReused   int
	SelectsChecked, SelectsReused int
	RoutesLowered, RoutesReused   int // scope items (routes, resources, datasets) lowered and answered
	// Within the packages vetted: declarations judged afresh by the
	// rules, and answered by the memo.
	DeclsVetted, DeclsVetReused int
}

type fileEntry struct {
	gen   int
	src   string
	file  *ast.File
	diags []diag.Diagnostic
	reuse *parser.Reuse
}

type goDir struct {
	srcs []GoSource
	scan *goScan
}

type pkgEntry struct {
	key   pkgKey
	res   pkgResult
	diags []diag.Diagnostic
	vet   []diag.Diagnostic
	vetOK bool
}

type pkgKey []pkgInput

type pkgInput struct {
	path  string
	files []int
	funcs *goScan
}

func (k pkgKey) equal(o pkgKey) bool {
	if k == nil || o == nil || len(k) != len(o) {
		return false
	}
	for i := range k {
		if k[i].path != o[i].path || k[i].funcs != o[i].funcs || !slices.Equal(k[i].files, o[i].files) {
			return false
		}
	}
	return true
}

type pkgResult struct {
	schema      *check.Info
	plan        *golang.Plan
	groups      map[string]*GroupInfo
	preds       map[string]*ast.Pred
	selects     []*SelectInfo
	validators  []golang.ValidatorFn
	validByKey  map[string][2]bool
	pipelines   map[string]*ast.Pipeline
	routes      []*RouteInfo
	controllers map[string]*ControllerInfo
}

func pkgCapture(pkg *Package) pkgResult {
	return pkgResult{
		schema: pkg.schema, plan: pkg.plan,
		groups: pkg.Groups, preds: pkg.Preds, selects: pkg.Selects, validators: pkg.Validators, validByKey: pkg.ValidByKey,
		pipelines: pkg.Pipelines, routes: pkg.Routes, controllers: pkg.Controllers,
	}
}

func (p *Package) resultsReset() {
	pkgResult{}.restore(p)
}

func (r pkgResult) restore(pkg *Package) {
	pkg.schema, pkg.plan = r.schema, r.plan
	pkg.Groups, pkg.Preds, pkg.Selects, pkg.Validators, pkg.ValidByKey = r.groups, r.preds, r.selects, r.validators, r.validByKey
	pkg.selectIndex()
	pkg.Pipelines, pkg.Routes, pkg.Controllers = r.pipelines, r.routes, r.controllers
}

// Load is LoadOverlay through the session's caches.
func (s *Session) Load(root string, overlay map[string]string) (*Project, error) {
	dirs, err := PackageDirs(root, root)
	if err != nil {
		return nil, err
	}
	return s.LoadDirs(root, dirs, overlay)
}

// LoadDirs is the package-level LoadDirs through the session's caches.
func (s *Session) LoadDirs(root string, dirs []string, overlay map[string]string) (*Project, error) {
	s.op.Lock()
	defer s.op.Unlock()
	return loadDirs(root, dirs, overlay, s)
}

// Check is the package-level Check with the session's memo: a package
// whose files, imports (transitively) and Go files are what they were
// when it was last checked is restored instead of re-run.
func (s *Session) Check(pr *Project) []diag.Diagnostic {
	s.op.Lock()
	defer s.op.Unlock()
	return checkWith(pr, s)
}

// Vet is lang.Vet through the session's memo: a package whose check
// results were reused and whose warnings were computed before answers
// from the memo. Run after Check on the same project.
func (s *Session) Vet(pr *Project) []diag.Diagnostic {
	s.op.Lock()
	defer s.op.Unlock()
	return vetWith(pr, s)
}

func (s *Session) vetRestore(path string, key pkgKey) ([]diag.Diagnostic, bool) {
	if key == nil {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.pkgs[path]
	if e == nil || !e.key.equal(key) || !e.vetOK {
		return nil, false
	}
	s.stats.PackagesVetReused++
	return e.vet, true
}

func (s *Session) vetStore(path string, key pkgKey, diags []diag.Diagnostic) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.PackagesVetted++
	if key == nil {
		return
	}
	if e := s.pkgs[path]; e != nil && e.key.equal(key) {
		e.vet, e.vetOK = diags, true
	}
}

func (s *Session) declMemos(paths []string) map[string]*declMemo {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.memos == nil {
		s.memos = map[string]*declMemo{}
	}
	out := make(map[string]*declMemo, len(paths))
	for _, p := range paths {
		m := s.memos[p]
		if m == nil {
			m = &declMemo{}
			s.memos[p] = m
		}
		out[p] = m
	}
	return out
}

func (s *Session) goFuncsFor(pr *Project, paths []string) *goFuncsCache {
	dirs := make([]string, len(paths))
	for i, path := range paths {
		dirs[i] = pr.Packages[path].Dir
	}
	scans := make([]*goScan, len(dirs))
	par.For(len(dirs), func(i int) {
		srcs := GoSourcesRead(dirs[i])
		s.mu.Lock()
		prev := s.scans[dirs[i]]
		s.mu.Unlock()
		if prev != nil && slices.Equal(prev.srcs, srcs) {
			scans[i] = prev.scan
			return
		}
		funcs, broken := GoFuncsOf(dirs[i], srcs)
		scans[i] = goScanOf(funcs, broken)
		s.mu.Lock()
		if s.scans == nil {
			s.scans = map[string]*goDir{}
		}
		s.scans[dirs[i]] = &goDir{srcs: srcs, scan: scans[i]}
		s.mu.Unlock()
	})
	cache := &goFuncsCache{by: make(map[string]*goScan, len(dirs))}
	for i, dir := range dirs {
		cache.by[dir] = scans[i]
	}
	return cache
}

func (s *Session) vetStats(memos map[string]*declMemo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range memos {
		s.stats.DeclsVetReused += m.vet.Hits
		s.stats.DeclsVetted += m.vet.Misses
	}
}

func (s *Session) declStats(memos map[string]*declMemo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range memos {
		s.stats.TablesReused += m.schema.Hits
		s.stats.TablesChecked += m.schema.Misses
		s.stats.ModelsReused += m.plan.Hits
		s.stats.ModelsBuilt += m.plan.Misses
		s.stats.ChecksReused += m.checks.Hits
		s.stats.ChecksLowered += m.checks.Misses
		s.stats.SelectsReused += m.selects.Hits
		s.stats.SelectsChecked += m.selects.Misses
		s.stats.RoutesReused += m.routes.Hits
		s.stats.RoutesLowered += m.routes.Misses
	}
}

// Stats reports the session's counters so far.
func (s *Session) Stats() SessionStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stats
}

func (s *Session) parse(path string, overlay map[string]string) (*ast.File, []diag.Diagnostic, error) {
	text, overlaid := overlay[path]
	if !overlaid {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}
		text = string(b)
	}
	s.mu.Lock()
	prev := s.files[path]
	s.mu.Unlock()
	if prev != nil && prev.src == text {
		s.reused()
		return prev.file, prev.diags, nil
	}
	var prevReuse *parser.Reuse
	if prev != nil {
		prevReuse = prev.reuse
	}
	file, diags, reuse, st := parser.ParseFileReuse(path, text, prevReuse)
	e := &fileEntry{src: text, file: file, diags: diags, reuse: reuse}
	s.mu.Lock()
	if s.files == nil {
		s.files = map[string]*fileEntry{}
	}
	s.gen++
	e.gen = s.gen
	s.files[path] = e
	s.stats.FilesParsed++
	s.stats.DeclsParsed += st.Parsed
	s.stats.DeclsReused += st.Reused
	s.mu.Unlock()
	return file, diags, nil
}

func (s *Session) reused() {
	s.mu.Lock()
	s.stats.FilesReused++
	s.mu.Unlock()
}

func (s *Session) packageKeys(pr *Project, paths []string, funcs *goFuncsCache) map[string]pkgKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := map[string]pkgKey{}
	state := map[string]int{}
	var visit func(path string) pkgKey
	visit = func(path string) pkgKey {
		switch state[path] {
		case 1:
			return nil
		case 2:
			return keys[path]
		}
		state[path] = 1
		defer func() { state[path] = 2 }()
		pkg := pr.Packages[path]
		if pkg == nil {
			return nil
		}
		own := pkgInput{path: path, funcs: funcs.by[pkg.Dir]}
		if own.funcs == nil {
			return nil
		}
		for _, f := range pkg.Files {
			e := s.files[f.Name]
			if e == nil || e.file != f {
				return nil
			}
			own.files = append(own.files, e.gen)
		}
		inputs := map[string]pkgInput{path: own}
		for _, t := range pkg.Imports {
			k := visit(t)
			if k == nil {
				return nil
			}
			for _, in := range k {
				inputs[in.path] = in
			}
		}
		key := make(pkgKey, 0, len(inputs))
		for _, in := range inputs {
			key = append(key, in)
		}
		slices.SortFunc(key, func(a, b pkgInput) int { return strings.Compare(a.path, b.path) })
		keys[path] = key
		return key
	}
	for _, path := range paths {
		visit(path)
	}
	return keys
}

func (s *Session) restore(path string, key pkgKey, pkg *Package) ([]diag.Diagnostic, bool) {
	if key == nil {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.pkgs[path]
	if e == nil || !e.key.equal(key) {
		return nil, false
	}
	e.res.restore(pkg)
	s.stats.PackagesReused++
	return e.diags, true
}

func (s *Session) store(path string, key pkgKey, pkg *Package, diags []diag.Diagnostic) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.PackagesChecked++
	if key == nil {
		return
	}
	if s.pkgs == nil {
		s.pkgs = map[string]*pkgEntry{}
	}
	s.pkgs[path] = &pkgEntry{key: key, res: pkgCapture(pkg), diags: diags}
}
