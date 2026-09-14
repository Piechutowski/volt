package lang

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Piechutowski/volt/internal/par"
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/token"
	"github.com/Piechutowski/volt/nao/gen/golang"
)

// Check runs project-level semantic analysis (spec §V1–§V6) over a
// loaded project: file and package rules, import resolution, the DBML
// layer per package, and routing expansion. It populates each package's
// Imports, Pipelines, Routes and Controllers, and returns all
// diagnostics including the ones collected at load time.
func Check(pr *Project) []diag.Diagnostic {
	return checkWith(pr, nil)
}

// checkWith is Check with an optional Session memo (D79): a package
// whose inputs are what they were last time is restored, not re-run.
func checkWith(pr *Project, s *Session) []diag.Diagnostic {
	c := &checker{pr: pr, diags: append([]diag.Diagnostic{}, pr.Diags...)}

	for _, path := range c.paths() {
		c.fileRules(pr.Packages[path])
	}
	for _, path := range c.paths() {
		c.importsResolve(pr.Packages[path])
	}
	c.cyclesCheck()

	// The heavy phases are per package and independent within a phase:
	// each runs on every CPU (PERF-7), package by package. Their
	// diagnostics are kept per package, then appended in package order.
	paths := c.paths()
	pkgDiags := make(map[string][]diag.Diagnostic, len(paths))
	fresh := paths
	var keys map[string]pkgKey
	if s != nil {
		c.gofuncs = s.goFuncsFor(pr, paths)
		keys = s.packageKeys(pr, paths, c.gofuncs)
		fresh = make([]string, 0, len(paths))
		for _, path := range paths {
			if ds, ok := s.restore(path, keys[path], pr.Packages[path]); ok {
				pkgDiags[path] = ds
			} else {
				fresh = append(fresh, path)
			}
		}
	}
	// A package checked before answers again from nothing: Check is
	// idempotent on a Project, whatever ran on it earlier (D81).
	for _, path := range fresh {
		pr.Packages[path].resultsReset()
	}
	phase := func(fn func(*checker, *Package)) {
		for i, ds := range c.perPackage(fresh, fn) {
			pkgDiags[fresh[i]] = append(pkgDiags[fresh[i]], ds...)
		}
	}
	if s != nil {
		c.memos = s.declMemos(fresh)
	}
	phase(func(cc *checker, pkg *Package) {
		var schemaMemo *check.Memo
		var planMemo *golang.PlanMemo
		if cc.memo != nil {
			schemaMemo, planMemo = &cc.memo.schema, &cc.memo.plan
		}
		info, schemaDiags := check.FileMemo(pkg.merged, schemaMemo)
		cc.diags = append(cc.diags, schemaDiags...)
		pkg.schema = info
		// The naming plan, once (D74): every later phase asks it for
		// generated names instead of re-deriving them from the AST.
		pkg.plan = golang.PlanBuildMemo(pkg.merged, info, planMemo)
	})
	for _, path := range paths {
		c.schemas[path] = pr.Packages[path].schema
	}
	phase((*checker).dataQueries)
	phase((*checker).tableChecks)
	phase((*checker).routing)
	if s != nil {
		for _, path := range fresh {
			s.store(path, keys[path], pr.Packages[path], pkgDiags[path])
		}
		s.declStats(c.memos)
	}
	for _, path := range paths {
		c.diags = append(c.diags, pkgDiags[path]...)
	}

	diag.Sort(c.diags)
	return c.diags
}

type checker struct {
	pr      *Project
	diags   []diag.Diagnostic
	schemas map[string]*check.Info

	// per-package state during routing()
	pkg       *Package
	usedQual  map[string]bool
	conflicts *routeIndex // accepted routes, bucketed for the §V4.7.2 scan

	// gofuncs caches each package directory's Go functions (§V3.2,
	// §V12.5), scanned once per run and shared across the per-package
	// checkers of a phase.
	gofuncs *goFuncsCache

	// memos are the per-declaration memos of the session's packages
	// (D84), by package path; memo is the current package's, nil
	// outside a session.
	memos map[string]*declMemo
	memo  *declMemo
}

// declMemo holds one package's per-declaration memos across checks
// (D84): the schema checker's tables, the plan's models, the lowered
// checks, the selects, and the routes of each scope item (D99).
type declMemo struct {
	schema  check.Memo
	plan    golang.PlanMemo
	checks  checksMemo
	selects selectsMemo
	routes  routesMemo
}

// checksMemo remembers a table's lowered checks: good while the
// checked table, its model and the directory's Go files are what
// they were.
type checksMemo struct {
	prev, next   map[*check.TableInfo]*checksEntry
	Hits, Misses int
}

// checksEntry is one call of tableSpecs: its inputs (the checked
// table is the map key) and its outputs.
type checksEntry struct {
	model   any
	funcs   *goScan
	pkgName string
	preds   map[string]*ast.Pred
	specs   []golang.CheckSpec
	sqls    map[*ast.Check]string
	diags   []diag.Diagnostic

	validCreate, validUpdate bool // the params structs validate: decided with the specs (§V12.6)
}

// holds reports whether the entry's inputs are the given ones.
func (e *checksEntry) holds(model any, funcs *goScan, pkgName string, preds map[string]*ast.Pred) bool {
	if e.model != model || e.funcs != funcs || e.pkgName != pkgName || len(e.preds) != len(preds) {
		return false
	}
	for name, d := range preds {
		if old, ok := e.preds[name]; !ok || old != d {
			return false
		}
	}
	return true
}

// perPackage runs one phase over every package, on every CPU (PERF-7).
// Each package gets its own checker — the per-package fields are its
// own; the project, the schemas and the Go-function cache are shared,
// read-only or locked — and its diagnostics are appended in package
// order, so the output never depends on the schedule. A phase writes
// only its own package, so the phases are barriers between them.
func (c *checker) perPackage(paths []string, phase func(*checker, *Package)) [][]diag.Diagnostic {
	if c.gofuncs == nil {
		c.gofuncs = &goFuncsCache{}
	}
	out := make([][]diag.Diagnostic, len(paths))
	par.For(len(paths), func(i int) {
		cc := &checker{pr: c.pr, schemas: c.schemas, gofuncs: c.gofuncs, memo: c.memos[paths[i]]}
		phase(cc, c.pr.Packages[paths[i]])
		out[i] = cc.diags
	})
	return out
}

func (c *checker) paths() []string {
	if c.schemas == nil {
		c.schemas = map[string]*check.Info{}
	}
	out := make([]string, 0, len(c.pr.Packages))
	for p := range c.pr.Packages {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func (c *checker) errorf(pos token.Position, section, format string, args ...any) {
	c.diags = append(c.diags, diag.Errorf(pos, "spec/"+section, format, args...))
}

/* ===== §V1: files and packages ===== */

func (c *checker) fileRules(pkg *Package) {
	base := pkg.Path
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	for _, f := range pkg.Files {
		var clause *ast.PackageClause
		for i, d := range f.Decls {
			switch d := d.(type) {
			case *ast.PackageClause:
				if i != 0 {
					c.errorf(d.Pos(), "V1", "the package clause must be the first declaration in the file (§V1.2)")
				}
				if clause != nil {
					c.errorf(d.Pos(), "V1", "duplicate package clause (§V1.2)")
					continue
				}
				clause = d
			case *ast.Use:
				// Rejected by the schema checker (§7) — one error, not two.
			}
		}
		if clause == nil {
			c.errorf(f.Pos(), "V1", "every .volt file must begin with a package clause (§V1.2)")
			continue
		}
		name := clause.Name.Name()
		if name == "volt" {
			c.errorf(clause.Pos(), "V1", "package name %q is reserved for the runtime (§V1.2); generated code could not tell your package from it", name)
			continue
		}
		if pkg.Name == "" {
			pkg.Name = name
		} else if pkg.Name != name {
			c.errorf(clause.Pos(), "V1", "package name %q disagrees with %q declared elsewhere in the directory (§V1.3)", name, pkg.Name)
		}
		if pkg.Path != "." && name != base {
			c.errorf(clause.Pos(), "V1", "package name %q must match its directory name %q (§V1.4)", name, base)
		}
	}
}

/* ===== §V2: imports ===== */

func (c *checker) importsResolve(pkg *Package) {
	byPath := map[string]*ast.ImportSpec{}
	for _, f := range pkg.Files {
		for _, d := range f.Decls {
			imp, ok := d.(*ast.ImportDecl)
			if !ok {
				continue
			}
			for _, spec := range imp.Specs {
				pkg.importSpecs = append(pkg.importSpecs, spec)
				path := spec.PathString()
				if prev, dup := byPath[path]; dup {
					if prev.Qualifier() != spec.Qualifier() {
						c.errorf(spec.Pos(), "V2", "package %q imported twice with different qualifiers %q and %q (§V2.4)", path, prev.Qualifier(), spec.Qualifier())
					}
					continue // idempotent across files (§V2.4)
				}
				byPath[path] = spec

				qual := spec.Qualifier()
				if prev, taken := pkg.Imports[qual]; taken && prev != path {
					c.errorf(spec.Pos(), "V2", "import qualifier %q already used for package %q (§V2.4); alias one of them", qual, prev)
					continue
				}
				target, exists := c.pr.Packages[path]
				switch {
				case path == pkg.Path:
					c.errorf(spec.Pos(), "V2", "a package cannot import itself (§V2.4)")
				case qual == pkg.Name:
					// The package's own name qualifies its own tables,
					// queries and functions (§V3.2, §V4.3, §V5.1, §V13.1).
					c.errorf(spec.Pos(), "V2", "import qualifier %q is this package's own name (§V2.4); alias the import", qual)
				case !exists:
					c.errorf(spec.Pos(), "V2", "unknown package %q: no directory of .volt files at that path under the project root (§V2.2)", path)
				case target == pkg:
					// unreachable; kept for clarity
				default:
					pkg.Imports[qual] = path
				}
			}
		}
	}
}

// cyclesCheck rejects import cycles between packages (§V2.6).
func (c *checker) cyclesCheck() {
	const (
		white = 0
		grey  = 1
		black = 2
	)
	color := map[string]int{}
	var stack []string
	var visit func(path string)
	visit = func(path string) {
		color[path] = grey
		stack = append(stack, path)
		pkg := c.pr.Packages[path]
		targets := make([]string, 0, len(pkg.Imports))
		for _, t := range pkg.Imports {
			targets = append(targets, t)
		}
		sort.Strings(targets)
		for _, target := range targets {
			switch color[target] {
			case white:
				visit(target)
			case grey:
				// report at the spec that closes the cycle
				pos := pkg.merged.Pos()
				for _, s := range pkg.importSpecs {
					if s.PathString() == target {
						pos = s.Pos()
					}
				}
				c.errorf(pos, "V2", "import cycle: %s (§V2.6)", cycleString(stack, target))
			}
		}
		color[path] = black
		stack = stack[:len(stack)-1]
	}
	for _, path := range c.paths() {
		if color[path] == white {
			visit(path)
		}
	}
}

func cycleString(stack []string, target string) string {
	start := 0
	for i, p := range stack {
		if p == target {
			start = i
		}
	}
	return strings.Join(append(append([]string{}, stack[start:]...), target), " -> ")
}

/* ===== §V3–§V5: routing ===== */

// inherited is the scope context flowing down the tree (§V4.4).
type inherited struct {
	prefix     []*ast.Segment
	pipes      []string
	errHandler string
	namePrefix string
}

func (c *checker) routing(pkg *Package) {
	c.pkg = pkg
	c.usedQual = map[string]bool{}
	c.conflicts = &routeIndex{root: &routeNode{}}
	pkg.Pipelines = map[string]*ast.Pipeline{}
	pkg.Controllers = map[string]*ControllerInfo{}

	for _, d := range pkg.merged.Decls {
		if pl, ok := d.(*ast.Pipeline); ok {
			if pl.Name.Quoted() || !goIdentOK(pl.Name.Name()) {
				c.errorf(pl.Name.Pos(), "V3", "Pipeline name %q must be a plain identifier (§V3.1)", pl.Name.Name())
				continue
			}
			if _, dup := pkg.Pipelines[pl.Name.Name()]; dup {
				c.errorf(pl.Pos(), "V3", "duplicate Pipeline name %q (§V3.1)", pl.Name.Name())
				continue
			}
			pkg.Pipelines[pl.Name.Name()] = pl
			for _, plug := range pl.Plugs {
				c.plugCheck(plug)
			}
		}
	}

	// Every scope item is lowered by itemLower, a pure function of the
	// inputs resolved here (D86, D99); the memo answers an item whose
	// inputs and the project's answers are what they were.
	facts := routeFacts{pkgName: pkg.Name, pkgPath: pkg.Path, module: c.pr.Module, imports: pkg.Imports}
	var memo *routesMemo
	if c.memo != nil {
		memo = &c.memo.routes
		memo.Hits, memo.Misses = 0, 0
		memo.next = map[ast.Node]*routesEntry{}
	}
	seenShape := map[string]*RouteInfo{}
	seenHelper := map[string]*RouteInfo{}
	for _, d := range pkg.merged.Decls {
		if sc, ok := d.(*ast.Scope); ok {
			c.scopeWalk(sc, inherited{}, facts, memo, seenShape, seenHelper)
		}
	}
	if memo != nil {
		memo.prev, memo.next = memo.next, nil
	}

	// §V4.3.4: the generated Controllers manifest is one namespace —
	// controller names, the Queries field of each data package (§V4.8.5)
	// and Events (§V4.11) — so a controller cannot take a field's name.
	held := map[string]string{}
	for _, r := range pkg.Routes {
		switch {
		case r.Query != nil && r.Query.Local:
			held[r.Query.Field] = "this package's query routes"
		case r.Query != nil:
			held[r.Query.Field] = "the query routes through package " + r.Query.Qualifier
		case r.Events:
			held["Events"] = "the event routes (§V4.11)"
		}
	}
	names := make([]string, 0, len(pkg.Controllers))
	for name := range pkg.Controllers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if what, taken := held[name]; taken {
			first := pkg.Controllers[name].Actions[0].Routes[0]
			c.errorf(first.Pos, "V4", "controller %q takes the name of the Controllers field holding %s (§V4.3); rename the controller", name, what)
		}
	}

	// §V2.4: every import must be used.
	for _, spec := range pkg.importSpecs {
		if _, resolved := pkg.Imports[spec.Qualifier()]; resolved && !c.usedQual[spec.Qualifier()] {
			c.errorf(spec.Pos(), "V2", "package %q imported and not used (§V2.4)", spec.PathString())
		}
	}
}

// plugCheck validates one pipeline entry (§V3.2): volt.X (runtime),
// X or <pkg>.X (this package). Cross-package plugs are outside v0.
func (c *checker) plugCheck(p *ast.Plug) {
	for _, part := range p.Ref.Parts {
		if part.Quoted() || !goIdentOK(part.Name()) || goKeywords[part.Name()] {
			c.errorf(p.Pos(), "V3", "plug reference %q must be plain, non-keyword Go identifiers (§V3.2)", p.Ref.String())
			return
		}
	}
	switch q := p.Ref.Qualifier(); {
	case q == "volt":
		// the runtime's middleware: not in this package's files
	case q == "" || q == c.pkg.Name:
		// A function of this package: it must exist with the middleware
		// signature, spelled exactly (§V3.2, D63).
		name := p.Ref.Base()
		const want = "func %s(next http.Handler) http.Handler"
		sc := c.goFuncs(c.pkg)
		gf, found := sc.funcs[name]
		if !found {
			c.errorf(p.Ref.Pos(), "V3", "no function %s in package %s's Go files — declare "+want+" beside the routes (§V3.2)%s", name, c.pkg.Name, name, sc.brokenHint())
			return
		}
		if gf.Generic {
			c.errorf(p.Ref.Pos(), "V3", "%s is generic (%s); a plug cannot instantiate it — wrap it in a plain "+want+" (§V3.2)", name, gf.Sig, name)
			return
		}
		if gf.Variadic || len(gf.Params) != 1 || gf.Params[0].Type != "http.Handler" ||
			len(gf.Results) != 1 || gf.Results[0] != "http.Handler" {
			c.errorf(p.Ref.Pos(), "V3", "%s is not middleware: found %s, a plug is "+want+" (§V3.2)", name, gf.Sig, name)
		}
	default:
		if _, isImport := c.pkg.Imports[q]; isImport {
			c.usedQual[q] = true
			c.errorf(p.Pos(), "V3", "cross-package plug %q is not supported in v0 (§V3.2); wrap it in a function of this package", p.Ref.String())
		} else {
			c.errorf(p.Pos(), "V3", "unknown plug qualifier %q (§V3.2): expected volt.X or a function of package %q", q, c.pkg.Name)
		}
	}
}

func (c *checker) scopeWalk(sc *ast.Scope, inh inherited, facts routeFacts, memo *routesMemo, seenShape, seenHelper map[string]*RouteInfo) {
	for _, seg := range sc.Path.Segments {
		if seg.Kind == ast.SegWild {
			c.errorf(seg.Pos(), "V4", "a Scope prefix cannot contain a wildcard (§V4.1.4)")
		}
	}
	next := inherited{
		prefix:     append(append([]*ast.Segment{}, inh.prefix...), sc.Path.Segments...),
		pipes:      append([]string{}, inh.pipes...),
		errHandler: inh.errHandler,
		namePrefix: inh.namePrefix,
	}
	if sc.Settings != nil {
		for _, s := range sc.Settings.Settings {
			switch s.Name {
			case "pipe":
				id, ok := s.Value.(*ast.Ident)
				if !ok {
					c.errorf(s.Pos(), "V4", "pipe: takes a Pipeline name (§V4.4)")
					continue
				}
				if _, exists := c.pkg.Pipelines[id.Name()]; !exists {
					c.errorf(s.Pos(), "V4", "unknown Pipeline %q (§V4.4)", id.Name())
					continue
				}
				next.pipes = append(next.pipes, id.Name())
			case "name":
				id, ok := s.Value.(*ast.Ident)
				if !ok {
					c.errorf(s.Pos(), "V4", "name: takes an identifier (§V4.4)")
					continue
				}
				n, err := golang.GoName(id.Name())
				if err != nil {
					c.errorf(s.Pos(), "V4", "scope name %q: %v (§V4.4)", id.Name(), err)
					continue
				}
				next.namePrefix += n
			case "error_handler":
				name, ok := selfFuncRefIn(s.Value, c.pkg.Name)
				if ok {
					// Held like every Go reference (D63): the function must exist
					// with the runtime's ErrorHandler shape, spelled exactly.
					const want = "func %s(w http.ResponseWriter, r *volt.Request, err error)"
					sc := c.goFuncs(c.pkg)
					if gf, found := sc.funcs[name]; !found {
						c.errorf(s.Pos(), "V4", "no function %s in package %s's Go files — declare "+want+" beside the routes (§V4.4)%s", name, c.pkg.Name, name, sc.brokenHint())
					} else if gf.Generic || gf.Variadic || len(gf.Params) != 3 || gf.Params[0].Type != "http.ResponseWriter" ||
						gf.Params[1].Type != "*volt.Request" || gf.Params[2].Type != "error" || len(gf.Results) != 0 {
						c.errorf(s.Pos(), "V4", "%s is not an error handler: found %s, expected "+want+" (§V4.4)", name, gf.Sig, name)
					}
				}
				if !ok {
					c.errorf(s.Pos(), "V4", "error_handler: takes a function of this package, written Name or %s.Name (§V4.5)", c.pkg.Name)
					continue
				}
				next.errHandler = name
			default:
				c.errorf(s.Pos(), "V6", "setting %q is not valid on a Scope (§V6); valid: pipe, name, error_handler", s.Name)
			}
		}
	}
	for _, item := range sc.Items {
		if nested, ok := item.(*ast.Scope); ok {
			c.scopeWalk(nested, next, facts, memo, seenShape, seenHelper)
			continue
		}
		for _, r := range c.itemLowerMemo(item, next, facts, memo) {
			c.routeAdd(*r, seenShape, seenHelper)
		}
	}
}

// routeOracle answers a scope item's lowering about the project (D99):
// reads only, so the gate walks it as the lowering's callee (D93).
type routeOracle struct{ c *checker }

func (o routeOracle) dataPackage(path string) (string, bool, bool) {
	pkg := o.c.pr.Packages[path]
	if pkg == nil {
		return "", false, false
	}
	return pkg.Name, o.c.schemas[path] != nil && pkg.HasSchema(), len(pkg.Selects) > 0
}

func (o routeOracle) tableByBase(path, base string) (*check.TableInfo, string) {
	info := o.c.schemas[path]
	if info == nil {
		return nil, ""
	}
	if ti := info.TableByBase(base); ti != nil {
		return ti, ""
	}
	// A name that differs only in case is a typo worth naming.
	caseMatch := ""
	for _, cand := range info.Tables {
		if name := cand.Decl.Name.Base(); strings.EqualFold(name, base) {
			caseMatch = name
		}
	}
	if caseMatch != "" {
		return nil, fmt.Sprintf("; did you mean %q? names are case-sensitive", caseMatch)
	}
	// A name that spells a table's MODEL name gets pointed at the table.
	for _, cand := range info.Tables {
		if model, err := golang.ModelName(cand.Decl); err == nil && model == base {
			return nil, fmt.Sprintf("; %q is the model of table %q — name the table", base, cand.Decl.Name.Base())
		}
	}
	return nil, ""
}

func (o routeOracle) modelOf(path, key string) (any, []golang.CRUDMethod, error) {
	pkg := o.c.pr.Packages[path]
	if pkg == nil || pkg.plan == nil {
		return nil, nil, fmt.Errorf("no package %q", path)
	}
	_, methods, err := pkg.plan.CRUDMethods(key)
	return pkg.plan.ModelRef(key), methods, err
}

func (o routeOracle) crudByName(path, name string) (string, any, golang.CRUDMethod, bool) {
	pkg := o.c.pr.Packages[path]
	if pkg == nil || pkg.plan == nil {
		return "", nil, golang.CRUDMethod{}, false
	}
	key, cm, ok := pkg.plan.CRUDMethod(name)
	if !ok {
		return "", nil, golang.CRUDMethod{}, false
	}
	return key, pkg.plan.ModelRef(key), cm, true
}

func (o routeOracle) selectByMethod(path, name string) (*SelectInfo, *check.TableInfo, bool) {
	pkg := o.c.pr.Packages[path]
	if pkg == nil {
		return nil, nil, false
	}
	sm, ok := pkg.selectByMethod[name]
	return sm.sel, sm.member, ok
}

func (o routeOracle) selectByName(path, name string) (*SelectInfo, string) {
	pkg := o.c.pr.Packages[path]
	if pkg == nil {
		return nil, ""
	}
	if si := pkg.selectByName[name]; si != nil {
		return si, ""
	}
	hint := ""
	for _, cand := range pkg.Selects {
		if strings.EqualFold(cand.Decl.Name.Name(), name) {
			hint = fmt.Sprintf("; did you mean %q?", cand.Decl.Name.Name())
		}
	}
	return nil, hint
}

func (o routeOracle) validators(path, key string) (bool, bool) {
	pkg := o.c.pr.Packages[path]
	if pkg == nil {
		return false, false
	}
	v := pkg.ValidByKey[key]
	return v[0], v[1]
}

func (o routeOracle) queryHint(path, name string) string {
	pkg := o.c.pr.Packages[path]
	if pkg == nil || pkg.plan == nil {
		return ""
	}
	var candidates []string
	for _, si := range pkg.Selects {
		for _, m := range si.Members {
			candidates = append(candidates, modelOrBase(m)+si.MethodSuffix)
		}
	}
	candidates = append(candidates, pkg.plan.CRUDMethodNames()...)
	for _, cand := range candidates {
		if strings.EqualFold(cand, name) {
			return fmt.Sprintf("; did you mean %q?", cand)
		}
	}
	return ""
}

// routesMemo remembers a package's lowered scope items (D99): a route,
// resources or dataset declaration whose inherited context, package
// facts and every answer the project gave it are what they were is
// the same routes and diagnostics.
type routesMemo struct {
	prev, next   map[ast.Node]*routesEntry
	Hits, Misses int
}

// routesEntry is one call of itemLower: inputs and outputs.
type routesEntry struct {
	inh     inherited
	facts   routeFacts
	lookups []routeLookup
	routes  []*RouteInfo
	used    []string
	diags   []diag.Diagnostic
}

// holds reports whether the entry's inputs are the given ones and the
// oracle still answers its questions as recorded.
func (e *routesEntry) holds(inh inherited, facts routeFacts, env routeEnv) bool {
	if !inheritedEqual(e.inh, inh) || !e.facts.equal(facts) {
		return false
	}
	for _, l := range e.lookups {
		if !l.holds(env) {
			return false
		}
	}
	return true
}

func inheritedEqual(a, b inherited) bool {
	return slices.Equal(a.prefix, b.prefix) && slices.Equal(a.pipes, b.pipes) && a.errHandler == b.errHandler && a.namePrefix == b.namePrefix
}

// itemLowerMemo lowers one scope item through the memo when there is
// one: a hit replays the item's diagnostics and used imports and
// answers its routes; a miss runs itemLower and keeps the call.
func (c *checker) itemLowerMemo(item ast.Node, inh inherited, facts routeFacts, memo *routesMemo) []*RouteInfo {
	env := routeOracle{c}
	if memo != nil {
		if e := memo.prev[item]; e != nil && e.holds(inh, facts, env) {
			memo.next[item] = e
			memo.Hits++
			c.diags = append(c.diags, e.diags...)
			for _, q := range e.used {
				c.usedQual[q] = true
			}
			return e.routes
		}
	}
	routes, lookups, used, diags := itemLower(item, inh, facts, env, memo != nil)
	c.diags = append(c.diags, diags...)
	for _, q := range used {
		c.usedQual[q] = true
	}
	if memo != nil {
		memo.next[item] = &routesEntry{inh: inh, facts: facts, lookups: lookups, routes: routes, used: used, diags: diags}
		memo.Misses++
	}
	return routes
}

/* ===== accumulation: conflicts, helpers, controllers ===== */

// routeAdd judges one lowered route against the package's accepted
// ones and accepts or rejects it. It takes the route by value: the
// lowering's route is the memo's, written by nobody (D92), and what
// this check names, orders and accepts is its own copy.
func (c *checker) routeAdd(r RouteInfo, seenShape, seenHelper map[string]*RouteInfo) {
	shape := r.shapeKey
	if prev, dup := seenShape[shape]; dup {
		c.errorf(r.Pos, "V4", "route %s %s conflicts with the route at %s: identical method and path shape (§V4.7)", methodOrAny(r.Method), r.Spelled, prev.Pos)
		return
	}
	// §V4.7.2: beyond exact duplicates, reject ServeMux's ambiguity
	// relation — two routes whose request sets overlap with neither more
	// specific would panic at registration, and the checker's promise
	// (§V4.7.4) is that a checked project never does.
	if prev := c.conflicts.ambiguous(&r); prev != nil {
		c.errorf(r.Pos, "V4", "route %s %s is ambiguous with the route at %s (%s %s): both match some requests and neither is more specific (§V4.7.2)",
			methodOrAny(r.Method), r.Spelled, prev.Pos, methodOrAny(prev.Method), prev.Spelled)
		return
	}
	seenShape[shape] = &r

	// One namespace for reverse-URL helpers and client methods (§V4.6,
	// §V4.10): a write query route has a client method and no helper,
	// and it must not collide with either.
	name := r.HelperName
	if name == "" {
		name = r.ClientName
	}
	if name != "" {
		if prev, dup := seenHelper[name]; dup {
			// A resources declaration colliding with itself means its
			// table name survived singularization unchanged (nao's
			// inflector is English), so the collection and member
			// helpers came out identical. Neither [name:] (not a
			// resources setting) nor a scope name helps there — both
			// sides get the same prefix — so name the working fix.
			if r.FromResources && prev.FromResources && r.Pos == prev.Pos {
				c.errorf(r.Pos, "V5", "resources %q: the collection and member reverse-URL helpers are both %q, because singularizing %q does not change it (§V5.4); set the singular explicitly, e.g. [singular: <name>]",
					prev.Table, r.HelperName, prev.Table)
			} else {
				c.errorf(r.Pos, "V4", "reverse-URL helper or client method %q already produced by the route at %s (§V4.6, §V4.10); disambiguate with [name:] or a scope name", name, prev.Pos)
			}
			r.HelperName, r.ClientName = "", ""
		} else {
			seenHelper[name] = &r
		}
	}
	if r.Query == nil && !r.Events && r.HelperName != "" {
		r.ClientName = r.HelperName // a named controller route gets a raw client method (§V4.10)
	}

	r.ord = len(c.pkg.Routes)
	if r.Query != nil || r.Events {
		c.routeAccept(&r) // no controller: the handler is generated (§V4.8) or the runtime's (§V4.11)
		return
	}
	ci := c.pkg.Controllers[r.Controller]
	if ci == nil {
		ci = &ControllerInfo{Name: r.Controller}
		c.pkg.Controllers[r.Controller] = ci
	}
	if a := ci.Action(r.Action); a != nil {
		if !paramsEqual(a.Params, r.Params) {
			c.errorf(r.Pos, "V4", "%s.%s is reached by routes with different parameter signatures (§V4.3): %s vs %s (first at %s)",
				r.Controller, r.Action, paramsString(r.Params), paramsString(a.Params), a.Routes[0].Pos)
			return
		}
		a.Routes = append(a.Routes, &r)
	} else {
		ci.Actions = append(ci.Actions, &ActionInfo{Name: r.Action, Params: r.Params, Routes: []*RouteInfo{&r}})
	}
	c.routeAccept(&r)
}

// routeAccept appends an accepted route to the package's route table and
// to the conflict index, in declaration order; its ord is already the
// table's length.
func (c *checker) routeAccept(r *RouteInfo) {
	c.pkg.Routes = append(c.pkg.Routes, r)
	c.conflicts.add(r)
}

// routeIndex holds one routing package's accepted routes bucketed for
// the §V4.7.2 ambiguity scan. An accepted route can be ambiguous with
// a new one only if its literal prefix lies on the new route's walk
// (§V4.7.3, proven against the pairwise scan by enumeration, D94), so
// the relation itself is unchanged (routesAmbiguous) and only the
// candidate set shrinks, from every accepted route to the walk's.
type routeIndex struct {
	root *routeNode
}

// routeNode is one literal path prefix. A route is filed at the node
// of its longest literal prefix — the segments before its first
// parameter, wildcard or end — so every route that could overlap a
// path is found by walking that path's own segments down the trie,
// through every child where the path has a parameter, and no further
// than the path's own length unless it ends in a wildcard. Keying on
// the first literal segment alone put every route of a scope in one
// list and made the check quadratic in a scope that routes every table
// (D81).
type routeNode struct {
	routes   []*RouteInfo // literal prefix ends here, declaration order
	children map[string]*routeNode
}

func (ix *routeIndex) add(r *RouteInfo) {
	n := ix.root
	for _, seg := range r.shape.segs {
		if seg == "" {
			break
		}
		child := n.children[seg]
		if child == nil {
			if n.children == nil {
				n.children = map[string]*routeNode{}
			}
			child = &routeNode{}
			n.children[seg] = child
		}
		n = child
	}
	n.routes = append(n.routes, r)
}

// ambiguous returns the earliest accepted route ambiguous with r, in
// declaration order — the one the pairwise scan would have found — or
// nil.
func (ix *routeIndex) ambiguous(r *RouteInfo) *RouteInfo {
	var best *RouteInfo
	var walk func(n *routeNode, depth int)
	walk = func(n *routeNode, depth int) {
		for _, prev := range n.routes {
			if best != nil && prev.ord >= best.ord {
				break // lists are in declaration order
			}
			if routesAmbiguous(prev, r) {
				best = prev
				break
			}
		}
		switch {
		case depth < len(r.shape.segs):
			if seg := r.shape.segs[depth]; seg != "" {
				if child := n.children[seg]; child != nil {
					walk(child, depth+1)
				}
			} else {
				for _, child := range n.children {
					walk(child, depth+1) // map order is irrelevant: the minimum wins
				}
			}
		case r.shape.wild:
			for _, child := range n.children {
				walk(child, depth+1)
			}
		}
	}
	walk(ix.root, 0)
	return best
}

/* ===== helpers ===== */

func methodOf(verb string) string {
	if verb == "any" {
		return ""
	}
	return strings.ToUpper(verb)
}

func methodOrAny(m string) string {
	if m == "" {
		return "any"
	}
	return m
}

// patternOf renders the ServeMux registration pattern (§V4.2): exact
// matching everywhere, so the root route uses the {$} end marker.
func patternOf(segs []*ast.Segment) string {
	if len(segs) == 0 {
		return "/{$}"
	}
	var b strings.Builder
	for _, seg := range segs {
		b.WriteByte('/')
		switch seg.Kind {
		case ast.SegParam:
			b.WriteString("{" + seg.Name.Name() + "}")
		case ast.SegWild:
			b.WriteString("{" + seg.Name.Name() + "...}")
		default:
			b.WriteString(seg.Name.Name())
		}
	}
	return b.String()
}

func spelledOf(segs []*ast.Segment) string {
	if len(segs) == 0 {
		return "/"
	}
	var b strings.Builder
	for _, seg := range segs {
		b.WriteByte('/')
		b.WriteString(seg.String())
	}
	return b.String()
}

// shapeOf is the conflict key: literals by spelling, every parameter as
// {} and every wildcard as {...} — brace characters cannot appear in a
// literal segment (§V4.1.6), so the markers cannot collide with
// literals named "P" or "W" or anything else (§V4.7).
func shapeOf(r *RouteInfo) string {
	var b strings.Builder
	rest := r.Pattern
	if rest == "/{$}" {
		return "/"
	}
	for _, part := range strings.Split(strings.TrimPrefix(rest, "/"), "/") {
		b.WriteByte('/')
		switch {
		case strings.HasSuffix(part, "...}"):
			b.WriteString("{...}")
		case strings.HasPrefix(part, "{"):
			b.WriteString("{}")
		default:
			b.WriteString(part)
		}
	}
	return b.String()
}

/* ===== ServeMux ambiguity (§V4.7.2) ===== */

// pathShape is a pattern as literal/param segments plus an optional
// rest-of-path wildcard tail, for the ambiguity relation.
type pathShape struct {
	segs []string // literal spelling, or "" for a parameter
	wild bool
}

func shapeParse(r *RouteInfo) pathShape {
	if r.Pattern == "/{$}" {
		return pathShape{}
	}
	var s pathShape
	for _, part := range strings.Split(strings.TrimPrefix(r.Pattern, "/"), "/") {
		switch {
		case strings.HasSuffix(part, "...}"):
			s.wild = true
		case strings.HasPrefix(part, "{"):
			s.segs = append(s.segs, "")
		default:
			s.segs = append(s.segs, part)
		}
	}
	return s
}

// pathsOverlap reports whether some request path matches both shapes.
func pathsOverlap(a, b pathShape) bool {
	i := 0
	for ; i < len(a.segs) && i < len(b.segs); i++ {
		sa, sb := a.segs[i], b.segs[i]
		if sa != "" && sb != "" && sa != sb {
			return false
		}
	}
	switch {
	case i == len(a.segs) && i == len(b.segs):
		// Both fixed parts consumed. A wildcard needs at least the
		// trailing slash, which a fixed-length pattern cannot supply.
		return a.wild == b.wild || (a.wild && b.wild)
	case i == len(a.segs):
		return a.wild // b has more segments; only a wildcard absorbs them
	default:
		return b.wild
	}
}

// pathSubset reports whether every path matched by a is matched by b.
func pathSubset(a, b pathShape) bool {
	if len(a.segs) < len(b.segs) {
		return false
	}
	for i, sb := range b.segs {
		if i >= len(a.segs) {
			return false
		}
		if sb != "" && a.segs[i] != sb {
			return false
		}
	}
	if len(a.segs) > len(b.segs) || a.wild {
		return b.wild
	}
	// equal length, a not wild: b must also end without demanding more
	return !b.wild || a.wild
}

// methodsOverlap / methodSubset treat "" (any) as the method wildcard.
func methodsOverlap(a, b string) bool { return a == "" || b == "" || a == b }
func methodSubset(a, b string) bool   { return b == "" || a == b }

// routesAmbiguous is Go's ServeMux rule: two patterns conflict when
// their request sets overlap and neither matches a subset of the other.
func routesAmbiguous(a, b *RouteInfo) bool {
	if !methodsOverlap(a.Method, b.Method) {
		return false
	}
	sa, sb := a.shape, b.shape
	if !pathsOverlap(sa, sb) {
		return false
	}
	aSub := methodSubset(a.Method, b.Method) && pathSubset(sa, sb)
	bSub := methodSubset(b.Method, a.Method) && pathSubset(sb, sa)
	return !aSub && !bSub
}

func paramsEqual(a, b []Param) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Type != b[i].Type || a[i].Wild != b[i].Wild || a[i].Name != b[i].Name {
			return false
		}
	}
	return true
}

func paramsString(ps []Param) string {
	parts := make([]string, len(ps))
	for i, p := range ps {
		parts[i] = p.Name + " " + p.Type.GoType()
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func settingOf(sl *ast.SettingList, name string) *ast.Setting {
	if sl == nil {
		return nil
	}
	return sl.Get(name)
}

// reservedParamNames are identifiers that appear in generated
// signatures and bodies: a parameter with one of these names would
// shadow them and break the generated code (§V4.1.2).
var reservedParamNames = map[string]bool{
	"w": true, "r": true, "opts": true, "volt": true,
}

var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
}

// goIdentOK reports whether s is a plain ASCII Go identifier.
func goIdentOK(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_', 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z':
		case '0' <= r && r <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// exportedIdentOK additionally requires an uppercase first letter.
func exportedIdentOK(s string) bool {
	return goIdentOK(s) && s[0] >= 'A' && s[0] <= 'Z'
}

// String renders a route table line for CLI display.
func (r *RouteInfo) String() string {
	helper := r.HelperName
	if helper != "" {
		helper = "Path" + helper
	}
	return fmt.Sprintf("%-7s %-32s %s %s", methodOrAny(r.Method), r.Spelled, r.HandlerRef(), helper)
}

// HandlerRef renders the route's handler as written: Controller.Action,
// or pkg.Query for a query route (§V4.8).
func (r *RouteInfo) HandlerRef() string {
	if r.Query != nil {
		return r.Query.Ref()
	}
	return r.Controller + "." + r.Action
}
