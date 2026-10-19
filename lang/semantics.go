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
	"github.com/Piechutowski/volt/lang/vet"
	"github.com/Piechutowski/volt/nao/gen/golang"
)

// Check runs project-level semantic analysis (spec §projects-and-packages–§settings-whitelists) over a
// loaded project: file and package rules, import resolution, the DBML
// layer per package, and routing expansion. It populates each package's
// Imports, Pipelines, Routes and Controllers, and returns all
// diagnostics including the ones collected at load time.
func Check(pr *Project) []diag.Diagnostic {
	return checkWith(pr, nil)
}

func checkWith(pr *Project, s *Session) []diag.Diagnostic {
	c := &checker{pr: pr, diags: append([]diag.Diagnostic{}, pr.Diags...)}

	for _, path := range c.paths() {
		c.fileRules(pr.Packages[path])
	}
	for _, path := range c.paths() {
		c.importsResolve(pr.Packages[path])
	}
	c.cyclesCheck()

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
		pkg.plan = golang.PlanBuildMemo(pkg.merged, info, planMemo)
	})
	for _, path := range paths {
		c.schemas[path] = pr.Packages[path].schema
	}
	phase((*checker).dataQueries)
	phase((*checker).tableValidators)
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

	pkg       *Package
	usedQual  map[string]bool
	conflicts *routeIndex

	gofuncs *goFuncsCache

	memos map[string]*declMemo
	memo  *declMemo
}

type declMemo struct {
	schema  check.Memo
	plan    golang.PlanMemo
	checks  checksMemo
	selects selectsMemo
	routes  routesMemo
	vet     vet.Memo
}

type checksMemo struct {
	prev, next   map[*check.TableInfo]*checksEntry
	Hits, Misses int
}

type checksEntry struct {
	model   any
	funcs   *goScan
	pkgName string
	specs   []golang.ValidatorSpec
	diags   []diag.Diagnostic

	validCreate, validUpdate bool
}

func (e *checksEntry) holds(model any, funcs *goScan, pkgName string) bool {
	return e.model == model && e.funcs == funcs && e.pkgName == pkgName
}

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
					c.errorf(d.Pos(), "projects-and-packages", "the package clause must be the first declaration in the file (§projects-and-packages)")
				}
				if clause != nil {
					c.errorf(d.Pos(), "projects-and-packages", "duplicate package clause (§projects-and-packages)")
					continue
				}
				clause = d
			case *ast.Use:
			}
		}
		if clause == nil {
			c.errorf(f.Pos(), "projects-and-packages", "every .volt file must begin with a package clause (§projects-and-packages)")
			continue
		}
		name := clause.Name.Name()
		if name == "volt" {
			c.errorf(clause.Pos(), "projects-and-packages", "package name %q is reserved for the runtime (§projects-and-packages); generated code could not tell your package from it", name)
			continue
		}
		if pkg.Name == "" {
			pkg.Name = name
		} else if pkg.Name != name {
			c.errorf(clause.Pos(), "projects-and-packages", "package name %q disagrees with %q declared elsewhere in the directory (§projects-and-packages)", name, pkg.Name)
		}
		if pkg.Path != "." && name != base {
			c.errorf(clause.Pos(), "projects-and-packages", "package name %q must match its directory name %q (§projects-and-packages)", name, base)
		}
	}
}

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
						c.errorf(spec.Pos(), "imports", "package %q imported twice with different qualifiers %q and %q (§imports)", path, prev.Qualifier(), spec.Qualifier())
					}
					continue
				}
				byPath[path] = spec

				qual := spec.Qualifier()
				if prev, taken := pkg.Imports[qual]; taken && prev != path {
					c.errorf(spec.Pos(), "imports", "import qualifier %q already used for package %q (§imports); alias one of them", qual, prev)
					continue
				}
				target, exists := c.pr.Packages[path]
				switch {
				case path == pkg.Path:
					c.errorf(spec.Pos(), "imports", "a package cannot import itself (§imports)")
				case qual == pkg.Name:
					c.errorf(spec.Pos(), "imports", "import qualifier %q is this package's own name (§imports); alias the import", qual)
				case !exists:
					c.errorf(spec.Pos(), "imports", "unknown package %q: no directory of .volt files at that path under the project root (§imports)", path)
				case target == pkg:
				default:
					pkg.Imports[qual] = path
				}
			}
		}
	}
}

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
				pos := pkg.merged.Pos()
				for _, s := range pkg.importSpecs {
					if s.PathString() == target {
						pos = s.Pos()
					}
				}
				c.errorf(pos, "imports", "import cycle: %s (§imports)", cycleString(stack, target))
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
				c.errorf(pl.Name.Pos(), "pipelines", "Pipeline name %q must be a plain identifier (§pipelines)", pl.Name.Name())
				continue
			}
			if _, dup := pkg.Pipelines[pl.Name.Name()]; dup {
				c.errorf(pl.Pos(), "pipelines", "duplicate Pipeline name %q (§pipelines)", pl.Name.Name())
				continue
			}
			pkg.Pipelines[pl.Name.Name()] = pl
			for _, plug := range pl.Plugs {
				c.plugCheck(plug)
			}
		}
	}

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

	held := map[string]string{}
	for _, r := range pkg.Routes {
		switch {
		case r.Query != nil && r.Query.Local:
			held[r.Query.Field] = "this package's query routes"
		case r.Query != nil:
			held[r.Query.Field] = "the query routes through package " + r.Query.Qualifier
		case r.Events:
			held["Events"] = "the event routes (§event-routes)"
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
			c.errorf(first.Pos, "scopes-and-routes", "controller %q takes the name of the Controllers field holding %s (§handler-references); rename the controller", name, what)
		}
	}

	for _, spec := range pkg.importSpecs {
		if _, resolved := pkg.Imports[spec.Qualifier()]; resolved && !c.usedQual[spec.Qualifier()] {
			c.errorf(spec.Pos(), "imports", "package %q imported and not used (§imports)", spec.PathString())
		}
	}
}

func (c *checker) plugCheck(p *ast.Plug) {
	for _, part := range p.Ref.Parts {
		if part.Quoted() || !goIdentOK(part.Name()) || goKeywords[part.Name()] {
			c.errorf(p.Pos(), "pipelines", "plug reference %q must be plain, non-keyword Go identifiers (§pipelines)", p.Ref.String())
			return
		}
	}
	switch q := p.Ref.Qualifier(); {
	case q == "volt":
	case q == "" || q == c.pkg.Name:
		name := p.Ref.Base()
		const want = "func %s(next http.Handler) http.Handler"
		sc := c.goFuncs(c.pkg)
		gf, found := sc.funcs[name]
		if !found {
			c.errorf(p.Ref.Pos(), "pipelines", "no function %s in package %s's Go files — declare "+want+" beside the routes (§pipelines)%s", name, c.pkg.Name, name, sc.brokenHint())
			return
		}
		if gf.Generic {
			c.errorf(p.Ref.Pos(), "pipelines", "%s is generic (%s); a plug cannot instantiate it — wrap it in a plain "+want+" (§pipelines)", name, gf.Sig, name)
			return
		}
		if gf.Variadic || len(gf.Params) != 1 || gf.Params[0].Type != "http.Handler" ||
			len(gf.Results) != 1 || gf.Results[0] != "http.Handler" {
			c.errorf(p.Ref.Pos(), "pipelines", "%s is not middleware: found %s, a plug is "+want+" (§pipelines)", name, gf.Sig, name)
		}
	default:
		if _, isImport := c.pkg.Imports[q]; isImport {
			c.usedQual[q] = true
			c.errorf(p.Pos(), "pipelines", "cross-package plug %q is not supported in v0 (§pipelines); wrap it in a function of this package", p.Ref.String())
		} else {
			c.errorf(p.Pos(), "pipelines", "unknown plug qualifier %q (§pipelines): expected volt.X or a function of package %q", q, c.pkg.Name)
		}
	}
}

func (c *checker) scopeWalk(sc *ast.Scope, inh inherited, facts routeFacts, memo *routesMemo, seenShape, seenHelper map[string]*RouteInfo) {
	for _, seg := range sc.Path.Segments {
		if seg.Kind == ast.SegWild {
			c.errorf(seg.Pos(), "scopes-and-routes", "a Scope prefix cannot contain a wildcard (§route-paths)")
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
					c.errorf(s.Pos(), "scopes-and-routes", "pipe: takes a Pipeline name (§scopes)")
					continue
				}
				if _, exists := c.pkg.Pipelines[id.Name()]; !exists {
					c.errorf(s.Pos(), "scopes-and-routes", "unknown Pipeline %q (§scopes)", id.Name())
					continue
				}
				next.pipes = append(next.pipes, id.Name())
			case "name":
				id, ok := s.Value.(*ast.Ident)
				if !ok {
					c.errorf(s.Pos(), "scopes-and-routes", "name: takes an identifier (§scopes)")
					continue
				}
				n, err := golang.GoName(id.Name())
				if err != nil {
					c.errorf(s.Pos(), "scopes-and-routes", "scope name %q: %v (§scopes)", id.Name(), err)
					continue
				}
				next.namePrefix += n
			case "error_handler":
				name, ok := selfFuncRefIn(s.Value, c.pkg.Name)
				if ok {
					const want = "func %s(w http.ResponseWriter, r *volt.Request, err error)"
					sc := c.goFuncs(c.pkg)
					if gf, found := sc.funcs[name]; !found {
						c.errorf(s.Pos(), "scopes-and-routes", "no function %s in package %s's Go files — declare "+want+" beside the routes (§scopes)%s", name, c.pkg.Name, name, sc.brokenHint())
					} else if gf.Generic || gf.Variadic || len(gf.Params) != 3 || gf.Params[0].Type != "http.ResponseWriter" ||
						gf.Params[1].Type != "*volt.Request" || gf.Params[2].Type != "error" || len(gf.Results) != 0 {
						c.errorf(s.Pos(), "scopes-and-routes", "%s is not an error handler: found %s, expected "+want+" (§scopes)", name, gf.Sig, name)
					}
				}
				if !ok {
					c.errorf(s.Pos(), "scopes-and-routes", "error_handler: takes a function of this package, written Name or %s.Name (§reserved)", c.pkg.Name)
					continue
				}
				next.errHandler = name
			default:
				c.errorf(s.Pos(), "settings-whitelists", "setting %q is not valid on a Scope (§settings-whitelists); valid: pipe, name, error_handler", s.Name)
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
	caseMatch := ""
	for _, cand := range info.Tables {
		if name := cand.Decl.Name.Base(); strings.EqualFold(name, base) {
			caseMatch = name
		}
	}
	if caseMatch != "" {
		return nil, fmt.Sprintf("; did you mean %q? names are case-sensitive", caseMatch)
	}
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

type routesMemo struct {
	prev, next   map[ast.Node]*routesEntry
	Hits, Misses int
}

type routesEntry struct {
	inh     inherited
	facts   routeFacts
	lookups []routeLookup
	routes  []*RouteInfo
	used    []string
	diags   []diag.Diagnostic
}

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

func (c *checker) routeAdd(r RouteInfo, seenShape, seenHelper map[string]*RouteInfo) {
	shape := r.shapeKey
	if prev, dup := seenShape[shape]; dup {
		c.errorf(r.Pos, "scopes-and-routes", "route %s %s conflicts with the route at %s: identical method and path shape (§route-conflicts)", methodOrAny(r.Method), r.Spelled, prev.Pos)
		return
	}
	if prev := c.conflicts.ambiguous(&r); prev != nil {
		c.errorf(r.Pos, "scopes-and-routes", "route %s %s is ambiguous with the route at %s (%s %s): both match some requests and neither is more specific (§route-conflicts)",
			methodOrAny(r.Method), r.Spelled, prev.Pos, methodOrAny(prev.Method), prev.Spelled)
		return
	}
	seenShape[shape] = &r

	name := r.HelperName
	if name == "" {
		name = r.ClientName
	}
	if name != "" {
		if prev, dup := seenHelper[name]; dup {
			if r.FromResources && prev.FromResources && r.Pos == prev.Pos {
				c.errorf(r.Pos, "resources", "resources %q: the collection and member reverse-URL helpers are both %q, because singularizing %q does not change it (§table-resolution); set the singular explicitly, e.g. [singular: <name>]",
					prev.Table, r.HelperName, prev.Table)
			} else {
				c.errorf(r.Pos, "scopes-and-routes", "reverse-URL helper or client method %q already produced by the route at %s (§route-names-and-reverse-urls, §generated-client); disambiguate with [name:] or a scope name", name, prev.Pos)
			}
			r.HelperName, r.ClientName = "", ""
		} else {
			seenHelper[name] = &r
		}
	}
	if r.Query == nil && !r.Events && r.HelperName != "" {
		r.ClientName = r.HelperName
	}

	r.ord = len(c.pkg.Routes)
	if r.Query != nil || r.Events {
		c.routeAccept(&r)
		return
	}
	ci := c.pkg.Controllers[r.Controller]
	if ci == nil {
		ci = &ControllerInfo{Name: r.Controller}
		c.pkg.Controllers[r.Controller] = ci
	}
	if a := ci.Action(r.Action); a != nil {
		if !paramsEqual(a.Params, r.Params) {
			c.errorf(r.Pos, "scopes-and-routes", "%s.%s is reached by routes with different parameter signatures (§handler-references): %s vs %s (first at %s)",
				r.Controller, r.Action, paramsString(r.Params), paramsString(a.Params), a.Routes[0].Pos)
			return
		}
		a.Routes = append(a.Routes, &r)
	} else {
		ci.Actions = append(ci.Actions, &ActionInfo{Name: r.Action, Params: r.Params, Routes: []*RouteInfo{&r}})
	}
	c.routeAccept(&r)
}

func (c *checker) routeAccept(r *RouteInfo) {
	c.pkg.Routes = append(c.pkg.Routes, r)
	c.conflicts.add(r)
}

type routeIndex struct {
	root *routeNode
}

type routeNode struct {
	routes   []*RouteInfo
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

func (ix *routeIndex) ambiguous(r *RouteInfo) *RouteInfo {
	var best *RouteInfo
	var walk func(n *routeNode, depth int)
	walk = func(n *routeNode, depth int) {
		for _, prev := range n.routes {
			if best != nil && prev.ord >= best.ord {
				break
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
					walk(child, depth+1)
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

type pathShape struct {
	segs []string
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
		return a.wild == b.wild || (a.wild && b.wild)
	case i == len(a.segs):
		return a.wild
	default:
		return b.wild
	}
}

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
	return !b.wild || a.wild
}

func methodsOverlap(a, b string) bool { return a == "" || b == "" || a == b }
func methodSubset(a, b string) bool   { return b == "" || a == b }

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
// or pkg.Query for a query route (§query-routes).
func (r *RouteInfo) HandlerRef() string {
	if r.Query != nil {
		return r.Query.Ref()
	}
	return r.Controller + "." + r.Action
}
