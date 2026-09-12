package lang

// An editor's view of a project across edits (D79). A keystroke changes
// one file; everything else — the other files, the packages that do
// not import the changed one — is what it was. A Session keeps the
// parses by content and the per-package check results by the identity
// of their inputs, so Load re-parses the changed file and Check re-runs
// the packages that could see it. Load and Check produce exactly the
// Project and diagnostics of LoadOverlay and Check; a test proves it
// edit by edit.

import (
	"hash/fnv"
	"io/fs"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
	"github.com/Piechutowski/volt/nao/gen/golang"
)

// Session caches parses and check results for one project root. The
// zero value is ready; a Session is safe for concurrent use.
type Session struct {
	mu    sync.Mutex
	files map[string]*fileEntry // by absolute path
	pkgs  map[string]*pkgEntry  // by package path
	gen   int                   // the last file identity handed out
	stats SessionStats
}

// SessionStats counts the work a session did and the work it skipped.
type SessionStats struct {
	FilesParsed, FilesReused        int
	PackagesChecked, PackagesReused int
}

// fileEntry is one file's last parse. gen is its identity: a new parse
// gets a new gen, so a memo keyed by gens can never match a parse that
// no longer exists, whatever the allocator reuses.
type fileEntry struct {
	gen   int
	src   string
	disk  bool // src was read from disk: size and mtime describe that read
	size  int64
	mtime time.Time
	file  *ast.File
	diags []diag.Diagnostic
}

// pkgEntry is one package's last check: the key its inputs hashed to,
// the fields the per-package phases wrote, and the diagnostics they
// produced.
type pkgEntry struct {
	key   string
	res   pkgResult
	diags []diag.Diagnostic
}

// pkgResult is everything the per-package phases of Check write; the
// values are immutable once written, so restoring them shares them.
type pkgResult struct {
	schema      *check.Info
	plan        *golang.Plan
	groups      map[string]*GroupInfo
	preds       map[string]*ast.Pred
	selects     []*SelectInfo
	checkFns    []golang.CheckFn
	pipelines   map[string]*ast.Pipeline
	routes      []*RouteInfo
	controllers map[string]*ControllerInfo
}

func pkgCapture(pkg *Package) pkgResult {
	return pkgResult{
		schema: pkg.schema, plan: pkg.plan,
		groups: pkg.Groups, preds: pkg.Preds, selects: pkg.Selects, checkFns: pkg.CheckFns,
		pipelines: pkg.Pipelines, routes: pkg.Routes, controllers: pkg.Controllers,
	}
}

func (r pkgResult) restore(pkg *Package) {
	pkg.schema, pkg.plan = r.schema, r.plan
	pkg.Groups, pkg.Preds, pkg.Selects, pkg.CheckFns = r.groups, r.preds, r.selects, r.checkFns
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
	return loadDirs(root, dirs, overlay, s)
}

// Check is the package-level Check with the session's memo: a package
// whose files, imports (transitively) and Go files are what they were
// when it was last checked is restored instead of re-run.
func (s *Session) Check(pr *Project) []diag.Diagnostic {
	return checkWith(pr, s)
}

// Stats reports the session's counters so far.
func (s *Session) Stats() SessionStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stats
}

// parse returns the file's parse, reusing the last one when the text
// is what it was: an overlaid file by its text, a disk file by size and
// modification time first — the read itself is then skipped — and by
// text when those moved.
func (s *Session) parse(path string, entry fs.DirEntry, overlay map[string]string) (*ast.File, []diag.Diagnostic, error) {
	text, overlaid := overlay[path]
	s.mu.Lock()
	prev := s.files[path]
	s.mu.Unlock()
	var info fs.FileInfo
	if !overlaid {
		var err error
		if info, err = entry.Info(); err != nil {
			return nil, nil, err
		}
		if prev != nil && prev.disk && prev.size == info.Size() && prev.mtime.Equal(info.ModTime()) {
			s.reused()
			return prev.file, prev.diags, nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}
		text = string(b)
	}
	if prev != nil && prev.src == text {
		if !overlaid {
			s.mu.Lock()
			prev.disk, prev.size, prev.mtime = true, info.Size(), info.ModTime()
			s.mu.Unlock()
		}
		s.reused()
		return prev.file, prev.diags, nil
	}
	file, diags := parser.ParseFile(path, text)
	e := &fileEntry{src: text, file: file, diags: diags}
	if !overlaid {
		e.disk, e.size, e.mtime = true, info.Size(), info.ModTime()
	}
	s.mu.Lock()
	if s.files == nil {
		s.files = map[string]*fileEntry{}
	}
	s.gen++
	e.gen = s.gen
	s.files[path] = e
	s.stats.FilesParsed++
	s.mu.Unlock()
	return file, diags, nil
}

func (s *Session) reused() {
	s.mu.Lock()
	s.stats.FilesReused++
	s.mu.Unlock()
}

// packageKeys hashes each package's inputs: the identities of its
// files, its Go files' stamp, and the keys of the packages it imports,
// so a change anywhere upstream changes the key. A package on an
// import cycle, or holding a file the session did not parse, gets ""
// and is never memoized.
func (s *Session) packageKeys(pr *Project, paths []string) map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := map[string]string{}
	state := map[string]int{} // 1 visiting, 2 done
	var visit func(path string) string
	visit = func(path string) string {
		switch state[path] {
		case 1:
			return "" // a cycle: importsResolve reports it; nothing to memoize
		case 2:
			return keys[path]
		}
		state[path] = 1
		pkg := pr.Packages[path]
		h := fnv.New64a()
		ok := pkg != nil
		if ok {
			for _, f := range pkg.Files {
				e := s.files[f.Name]
				if e == nil || e.file != f {
					ok = false
					break
				}
				h.Write([]byte(strconv.Itoa(e.gen)))
				h.Write([]byte{';'})
			}
			h.Write([]byte(GoDirStamp(pkg.Dir)))
			h.Write([]byte{'|'})
			targets := make([]string, 0, len(pkg.Imports))
			for _, t := range pkg.Imports {
				targets = append(targets, t)
			}
			sort.Strings(targets)
			for _, t := range targets {
				k := visit(t)
				if k == "" {
					ok = false
					break
				}
				h.Write([]byte(k))
				h.Write([]byte{','})
			}
		}
		state[path] = 2
		if !ok {
			keys[path] = ""
			return ""
		}
		keys[path] = strconv.FormatUint(h.Sum64(), 16)
		return keys[path]
	}
	for _, path := range paths {
		visit(path)
	}
	return keys
}

// restore hands a package its memoized results and diagnostics when
// its key matches; false means it must be checked.
func (s *Session) restore(path, key string, pkg *Package) ([]diag.Diagnostic, bool) {
	if key == "" {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.pkgs[path]
	if e == nil || e.key != key {
		return nil, false
	}
	e.res.restore(pkg)
	s.stats.PackagesReused++
	return e.diags, true
}

// store memoizes a freshly checked package under its key.
func (s *Session) store(path, key string, pkg *Package, diags []diag.Diagnostic) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.PackagesChecked++
	if key == "" {
		return
	}
	if s.pkgs == nil {
		s.pkgs = map[string]*pkgEntry{}
	}
	s.pkgs[path] = &pkgEntry{key: key, res: pkgCapture(pkg), diags: diags}
}
