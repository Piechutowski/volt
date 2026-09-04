package lang

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/token"
	"github.com/Piechutowski/volt/nao/gen/golang"
	"github.com/Piechutowski/volt/nao/inflect"
)

// Check runs project-level semantic analysis (spec §V1–§V6) over a
// loaded project: file and package rules, import resolution, the DBML
// layer per package, and routing expansion. It populates each package's
// Imports, Pipelines, Routes and Controllers, and returns all
// diagnostics including the ones collected at load time.
func Check(pr *Project) []diag.Diagnostic {
	c := &checker{pr: pr, diags: append([]diag.Diagnostic{}, pr.Diags...)}

	for _, path := range c.paths() {
		c.fileRules(pr.Packages[path])
	}
	for _, path := range c.paths() {
		c.importsResolve(pr.Packages[path])
	}
	c.cyclesCheck()
	for _, path := range c.paths() {
		pkg := pr.Packages[path]
		info, schemaDiags := check.File(pkg.merged)
		c.diags = append(c.diags, schemaDiags...)
		c.schemas[path] = info
		pkg.schema = info
	}
	for _, path := range c.paths() {
		c.dataQueries(pr.Packages[path])
	}
	for _, path := range c.paths() {
		c.tableChecks(pr.Packages[path])
	}
	for _, path := range c.paths() {
		c.routing(pr.Packages[path])
	}

	diag.Sort(c.diags)
	return c.diags
}

type checker struct {
	pr      *Project
	diags   []diag.Diagnostic
	schemas map[string]*check.Info

	// per-package state during routing()
	pkg      *Package
	usedQual map[string]bool

	// gofuncs caches each package directory's Go functions (§V3.2,
	// §V12.5), scanned once per run.
	gofuncs map[string]*goScan
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

	seenShape := map[string]*RouteInfo{}
	seenHelper := map[string]*RouteInfo{}
	for _, d := range pkg.merged.Decls {
		if sc, ok := d.(*ast.Scope); ok {
			c.scopeWalk(sc, inherited{}, seenShape, seenHelper)
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

func (c *checker) scopeWalk(sc *ast.Scope, inh inherited, seenShape, seenHelper map[string]*RouteInfo) {
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
				name, ok := c.selfFuncRef(s.Value)
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
		switch item := item.(type) {
		case *ast.Scope:
			c.scopeWalk(item, next, seenShape, seenHelper)
		case *ast.Route:
			if r := c.routeBuild(item, next); r != nil {
				c.routeAdd(r, seenShape, seenHelper)
			}
		case *ast.Resources:
			for _, r := range c.resourcesExpand(item, next) {
				c.routeAdd(r, seenShape, seenHelper)
			}
		case *ast.Dataset:
			for _, r := range c.datasetExpand(item, next) {
				c.routeAdd(r, seenShape, seenHelper)
			}
		}
	}
}

/* ===== datasets (§V13) ===== */

// datasetExpand turns `dataset db.browse [strip: 'da_']` into one GET
// query route per member of the select's target: the segment is the
// member's table name with the strip prefix removed, the handler the
// member's select method, bound like any query route (§V4.8).
func (c *checker) datasetExpand(ds *ast.Dataset, inh inherited) []*RouteInfo {
	if ds.Pkg == nil {
		c.errorf(ds.Name.Pos(), "V13", "dataset names a select of an imported data package, qualified: dataset db.%s (§V13.1)", ds.Name.Name())
		return nil
	}
	qual := ds.Pkg.Name()
	target, known := c.pkg.Imports[qual]
	if !known {
		c.errorf(ds.Pkg.Pos(), "V13", "unknown package qualifier %q (§V13.1)", qual)
		return nil
	}
	c.usedQual[qual] = true
	pkg := c.pr.Packages[target]
	var si *SelectInfo
	for _, cand := range pkg.Selects {
		if cand.Decl.Name.Name() == ds.Name.Name() {
			si = cand
		}
	}
	if si == nil {
		hint := ""
		for _, cand := range pkg.Selects {
			if strings.EqualFold(cand.Decl.Name.Name(), ds.Name.Name()) {
				hint = fmt.Sprintf("; did you mean %q?", cand.Decl.Name.Name())
			}
		}
		c.errorf(ds.Name.Pos(), "V13", "no select %q in package %q%s (§V13.1)", ds.Name.Name(), target, hint)
		return nil
	}

	strip := ""
	var only, except map[string]bool
	var onlyPos, exceptPos token.Position
	members := map[string]bool{}
	for _, m := range si.Members {
		members[m.Decl.Name.Base()] = true
	}
	if ds.Settings != nil {
		for _, s := range ds.Settings.Settings {
			switch s.Name {
			case "strip":
				lit, ok := s.Value.(*ast.BasicLit)
				if !ok || lit.Tok.Kind != token.STRING {
					c.errorf(s.Pos(), "V13", "strip: takes a string, the table-name prefix to drop from the URL segment (§V13.2)")
					continue
				}
				strip = lit.Tok.Val
			case "only", "except":
				list, ok := s.Value.(*ast.IdentList)
				if !ok {
					c.errorf(s.Pos(), "V13", "%s: takes a table list like (da_a_a, da_b_b) (§V13.2)", s.Name)
					continue
				}
				set := map[string]bool{}
				for i, id := range list.Names {
					if list.Mods[i] != nil {
						c.errorf(list.Mods[i].Pos(), "V13", "%s: entries are table names; %q does not belong after %q (§V13.2)", s.Name, list.Mods[i].Name(), id.Name())
					}
					if !members[id.Name()] {
						c.errorf(id.Pos(), "V13", "%s: %q is not a member of select %q's target (§V13.2)", s.Name, id.Name(), si.Decl.Name.Name())
						continue
					}
					set[id.Name()] = true
				}
				if s.Name == "only" {
					only, onlyPos = set, s.Pos()
				} else {
					except, exceptPos = set, s.Pos()
				}
			default:
				c.errorf(s.Pos(), "V6", "setting %q is not valid on a dataset (§V6); valid: strip, only, except", s.Name)
			}
		}
	}
	if only != nil && except != nil {
		pos := onlyPos
		if exceptPos.Line > onlyPos.Line || (exceptPos.Line == onlyPos.Line && exceptPos.Column > onlyPos.Column) {
			pos = exceptPos
		}
		c.errorf(pos, "V13", "only: and except: cannot both be set on a dataset (§V13.2)")
		return nil
	}

	var out []*RouteInfo
	for _, m := range si.Members {
		base := m.Decl.Name.Base()
		if only != nil && !only[base] {
			continue
		}
		if except[base] {
			continue
		}
		seg := base
		if strip != "" {
			if !strings.HasPrefix(base, strip) {
				c.errorf(ds.Pos(), "V13", "strip: %q is not a prefix of member table %q (§V13.2)", strip, base)
				continue
			}
			seg = strings.TrimPrefix(base, strip)
			if seg == "" || !goIdentOK(seg) {
				c.errorf(ds.Pos(), "V13", "strip: %q leaves member table %q with no usable segment (§V13.2)", strip, base)
				continue
			}
		}
		segs := append(append([]*ast.Segment{}, inh.prefix...), litSeg(seg, ds.Pos()))
		params, ok := c.pathParams(segs)
		if !ok {
			continue
		}
		method := modelOrBase(m) + si.MethodSuffix
		qr := c.queryBind(ds.Name.Pos(), ds.Pkg.Pos(), "GET", params, qual, target, method, func(string) token.Position { return ds.Pos() })
		if qr == nil {
			continue
		}
		helper := inh.namePrefix + method
		out = append(out, &RouteInfo{
			Method:       "GET",
			Pattern:      patternOf(segs),
			Spelled:      spelledOf(segs),
			Params:       params,
			Query:        qr,
			HelperName:   helper,
			ClientName:   helper,
			Pipes:        inh.pipes,
			ErrorHandler: inh.errHandler,
			Pos:          ds.Pos(),
			FromDataset:  true,
		})
	}
	return out
}

// selfFuncRef accepts Name or <thispackage>.Name and returns the bare
// function name.
func (c *checker) selfFuncRef(v ast.Node) (string, bool) {
	usable := func(id *ast.Ident) bool {
		return !id.Quoted() && goIdentOK(id.Name()) && !goKeywords[id.Name()]
	}
	switch v := v.(type) {
	case *ast.Ident:
		if usable(v) {
			return v.Name(), true
		}
	case *ast.EnumConst:
		if v.Enum.Name() == c.pkg.Name && usable(v.Value) {
			return v.Value.Name(), true
		}
	}
	return "", false
}

/* ===== routes (§V4) ===== */

func (c *checker) routeBuild(r *ast.Route, inh inherited) *RouteInfo {
	method := methodOf(r.Verb())
	segs := append(append([]*ast.Segment{}, inh.prefix...), r.Path.Segments...)
	params, ok := c.pathParams(segs)
	if !ok {
		return nil
	}

	controller, action := r.Handler.Parts[0].Name(), r.Handler.Parts[1].Name()
	if r.Handler.Parts[0].Quoted() || r.Handler.Parts[1].Quoted() {
		c.errorf(r.Handler.Pos(), "V4", "handler names are plain (unquoted) identifiers (§V4.1.6), found %q", r.Handler.String())
		return nil
	}
	// A qualified handler names a generated query of an imported data
	// package — a query route (§V4.8) — rather than a controller.
	var query *QueryRef
	events := false
	if controller == "volt" {
		// The runtime's own handlers: today exactly one, the event stream.
		if action != "Events" {
			c.errorf(r.Handler.Pos(), "V4", "the runtime provides no handler volt.%s; volt.Events is the event stream (§V4.11)", action)
			return nil
		}
		if method != "GET" {
			c.errorf(r.Pos(), "V4", "an event route is read with get (§V4.11)")
			return nil
		}
		events = true
	} else if target, isImport := c.pkg.Imports[controller]; isImport {
		c.usedQual[controller] = true
		query = c.queryRef(r, method, params, controller, target)
		if query == nil {
			return nil
		}
		controller, action = "", query.Method
	} else if !exportedIdentOK(controller) || !exportedIdentOK(action) {
		c.errorf(r.Handler.Pos(), "V4", "handler must be Controller.Action, both exported Go identifiers (§V4.3), found %q", r.Handler.String())
		return nil
	}

	helper := inh.namePrefix + action
	client := ""
	if s := settingOf(r.Settings, "name"); s != nil {
		id, ok := s.Value.(*ast.Ident)
		if !ok {
			c.errorf(s.Pos(), "V4", "name: takes an identifier (§V4.6)")
		} else if n, err := golang.GoName(id.Name()); err != nil {
			c.errorf(s.Pos(), "V4", "route name %q: %v (§V4.6)", id.Name(), err)
		} else {
			helper = inh.namePrefix + n
		}
	}
	if r.Settings != nil {
		for _, s := range r.Settings.Settings {
			if s.Name != "name" {
				c.errorf(s.Pos(), "V6", "setting %q is not valid on a route (§V6); valid: name", s.Name)
			}
		}
	}

	if query != nil {
		action = ""
		// The client method carries the name for every query route; the
		// reverse-URL helper only for reads, as with resources (§V4.8).
		client = helper
		if method != "GET" && method != "HEAD" {
			helper = ""
		}
	}
	if events {
		client = helper // the client's Events method (§V4.11)
	}
	return &RouteInfo{
		Method:       method,
		Pattern:      patternOf(segs),
		Spelled:      spelledOf(segs),
		Params:       params,
		Controller:   controller,
		Action:       action,
		Query:        query,
		Events:       events,
		HelperName:   helper,
		ClientName:   client,
		Pipes:        inh.pipes,
		ErrorHandler: inh.errHandler,
		Pos:          r.Pos(),
	}
}

/* ===== query routes (§V4.8) ===== */

// queryValueTypes are the Go types a query-string parameter can carry:
// the scalar column types of Appendix A, matched by the runtime's
// volt.QueryParam.
var queryValueTypes = map[string]bool{
	"string": true, "bool": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"float32": true, "float64": true, "time.Time": true,
}

// queryRef resolves `pkg.Method` to a generated query of the imported
// package — a select method (§V11.6) or a default CRUD method — and
// binds the route's parameters to the method's (§V4.8).
func (c *checker) queryRef(r *ast.Route, method string, params []Param, qual, target string) *QueryRef {
	return c.queryBind(r.Handler.Parts[1].Pos(), r.Handler.Parts[0].Pos(), method, params, qual, target, r.Handler.Parts[1].Name(),
		func(name string) token.Position { return segPos(r, name) })
}

// queryBind resolves a query method by name in the imported package and
// binds the route's parameters; segAt locates a path parameter for
// diagnostics. Shared by query routes and datasets.
func (c *checker) queryBind(pos, qualPos token.Position, method string, params []Param, qual, target, name string, segAt func(string) token.Position) *QueryRef {
	pkg := c.pr.Packages[target]
	info := c.schemas[target]
	if pkg == nil || info == nil || !pkg.HasSchema() {
		c.errorf(pos, "V4", "package %q declares no tables; a query route needs a data package (§V4.8)", target)
		return nil
	}
	field, err := golang.GoName(qual)
	if err != nil {
		c.errorf(qualPos, "V4", "import qualifier %q: %v (§V4.8)", qual, err)
		return nil
	}
	importPath := c.pr.Module
	if target != "." {
		importPath += "/" + target
	}
	qr := &QueryRef{Qualifier: qual, Field: field, Package: target, Import: importPath, PkgName: pkg.Name, Method: name, Status: 200}

	// Signature parameters of the named method, in order.
	type sigParam struct {
		name, goType string
		body         bool
		validates    bool
	}
	var sig []sigParam
	found := false
	var candidates []string

	// Selects: <Model><SelectName> per member (§V11.6).
	for _, si := range pkg.Selects {
		for _, m := range si.Members {
			mn := modelOrBase(m) + si.MethodSuffix
			candidates = append(candidates, mn)
			if mn != name || found {
				continue
			}
			found = true
			for _, p := range si.Params {
				sig = append(sig, sigParam{name: p.GoName, goType: p.GoType})
			}
			switch {
			case si.Shared != "":
				qr.Result = si.Shared
			case len(si.Excluded) > 0:
				qr.Result = modelOrBase(m) + si.MethodSuffix
			default:
				qr.Result = modelOrBase(m)
			}
			qr.Many = true
		}
	}
	// Default CRUD (CRUD-1 to CRUD-7).
	if !found {
		for _, ti := range info.Tables {
			_, methods, err := golang.CRUDMethods(pkg.Merged(), info, ti.Key)
			if err != nil {
				continue
			}
			for _, cm := range methods {
				candidates = append(candidates, cm.Name)
				if cm.Name != name || found {
					continue
				}
				found = true
				for _, k := range cm.Key {
					sig = append(sig, sigParam{name: k.GoName, goType: k.GoType})
				}
				if cm.Body != "" {
					// The params struct validates when it carries the
					// columns of at least one check (§V12.6).
					create, update, _ := golang.ParamsValidators(pkg.Merged(), info, ti.Key, tableChecksOf(pkg, ti.Key))
					sig = append(sig, sigParam{name: "arg", goType: qual + "." + cm.Body, body: true,
						validates: (cm.Op == "create" && create) || (cm.Op == "update" && update)})
				}
				qr.Result, qr.Many = cm.Result, cm.Many
				switch cm.Op {
				case "create":
					qr.Status = 201
				case "delete":
					qr.Status = 204
				}
			}
		}
	}
	if !found {
		hint := ""
		for _, cand := range candidates {
			if strings.EqualFold(cand, name) {
				hint = fmt.Sprintf("; did you mean %q?", cand)
				break
			}
		}
		c.errorf(pos, "V4", "no generated query %s.%s in package %q%s — a query route names a select method or a default CRUD method (Get, List, Create, Update, Delete) (§V4.8)", qual, name, target, hint)
		return nil
	}

	// Bind: path parameters by name (type spelled to match), a params
	// struct from the body, everything else from the query string.
	byName := map[string]Param{}
	for _, p := range params {
		byName[p.Name] = p
	}
	bound := map[string]bool{}
	ok := true
	for _, sp := range sig {
		qp := QueryParam{Name: sp.name, GoType: sp.goType}
		switch {
		case sp.body:
			qp.Source = FromBody
			qp.Validates = sp.validates
			if method != "POST" && method != "PUT" && method != "PATCH" {
				c.errorf(pos, "V4", "%s.%s takes a request body (%s); route it with post, put or patch (§V4.8)", qual, name, sp.goType)
				ok = false
			}
		case strings.HasPrefix(sp.goType, "[]"):
			qp.Source = FromList
			if pp, inPath := byName[sp.name]; inPath {
				c.errorf(segAt(pp.Name), "V4", "list parameter %q of %s.%s cannot be a path parameter; pass it as a repeated query key (§V4.8)", sp.name, qual, name)
				ok = false
			}
		default:
			if pp, inPath := byName[sp.name]; inPath {
				qp.Source = FromPath
				bound[sp.name] = true
				if pp.Wild {
					c.errorf(segAt(pp.Name), "V4", "parameter %q of %s.%s cannot be a wildcard (§V4.8)", sp.name, qual, name)
					ok = false
				} else if pp.Type.GoType() != sp.goType {
					if KnownParamType(sp.goType) {
						c.errorf(segAt(pp.Name), "V4", "path parameter %q is %s but %s.%s takes %s; spell it :%s(%s) (§V4.8)", sp.name, pp.Type.GoType(), qual, name, sp.goType, sp.name, sp.goType)
					} else {
						c.errorf(segAt(pp.Name), "V4", "parameter %q of %s.%s is %s, which a path segment cannot carry (§V4.1.3); pass it in the query string (§V4.8)", sp.name, qual, name, sp.goType)
					}
					ok = false
				}
			} else {
				qp.Source = FromQuery
				if !queryValueTypes[sp.goType] {
					c.errorf(pos, "V4", "parameter %q of %s.%s is %s, which the query string cannot carry (§V4.8)", sp.name, qual, name, sp.goType)
					ok = false
				}
			}
		}
		qr.Params = append(qr.Params, qp)
	}
	for _, p := range params {
		if !bound[p.Name] {
			c.errorf(segAt(p.Name), "V4", "path parameter %q is not a parameter of %s.%s (§V4.8)", p.Name, qual, name)
			ok = false
		}
	}
	if !ok {
		return nil
	}
	return qr
}

// tableChecksOf returns the lowered checks of one table of a package.
func tableChecksOf(pkg *Package, tableKey string) []golang.CheckSpec {
	for _, fn := range pkg.CheckFns {
		if fn.TableKey == tableKey {
			return fn.Checks
		}
	}
	return nil
}

// litSeg builds a synthetic literal segment for an expanded route.
func litSeg(n string, at token.Position) *ast.Segment {
	return &ast.Segment{Kind: ast.SegLit, Name: &ast.Ident{Tok: token.Token{Kind: token.IDENT, Val: n, Pos: at}}}
}

// segPos finds the position of a named parameter segment in the route's
// own path, falling back to the route.
func segPos(r *ast.Route, name string) token.Position {
	for _, seg := range r.Path.Segments {
		if seg.Kind != ast.SegLit && seg.Name.Name() == name {
			return seg.Pos()
		}
	}
	return r.Pos()
}

// pathParams validates the full segment list and derives the parameter
// signature (§V4.1).
func (c *checker) pathParams(segs []*ast.Segment) ([]Param, bool) {
	var params []Param
	seen := map[string]bool{}
	ok := true
	for i, seg := range segs {
		// §V4.1.6: every segment name — literal, parameter or wildcard —
		// is a plain, unquoted identifier; anything else would flow
		// unescaped into registration patterns and generated Go.
		if seg.Name.Quoted() || !goIdentOK(seg.Name.Name()) {
			c.errorf(seg.Pos(), "V4", "path segment %q must be a plain identifier of letters, digits and underscores (§V4.1.6)", seg.Name.Name())
			ok = false
			continue
		}
		switch seg.Kind {
		case ast.SegLit:
			continue
		case ast.SegWild:
			if i != len(segs)-1 {
				c.errorf(seg.Pos(), "V4", "the wildcard ':%s...' must be the last path segment (§V4.1.4)", seg.Name.Name())
				ok = false
			}
		}
		name := seg.Name.Name()
		if seen[name] {
			c.errorf(seg.Pos(), "V4", "duplicate path parameter %q (§V4.1.2)", name)
			ok = false
			continue
		}
		seen[name] = true
		if !goIdentOK(name) || goKeywords[name] {
			c.errorf(seg.Pos(), "V4", "path parameter %q must be a valid, non-keyword Go identifier (§V4.1.2)", name)
			ok = false
			continue
		}
		if reservedParamNames[name] {
			c.errorf(seg.Pos(), "V4", "path parameter %q collides with a name in generated signatures (§V4.1.2); reserved: w, r, opts, volt", name)
			ok = false
			continue
		}
		pt := TString
		if seg.Type != nil {
			if !KnownParamType(seg.Type.Name()) {
				c.errorf(seg.Type.Pos(), "V4", "unknown parameter type %q (§V4.1.3); valid: int, int32, int64, string", seg.Type.Name())
				ok = false
				continue
			}
			pt = ParamType(seg.Type.Name())
		}
		params = append(params, Param{Name: name, GoName: name, Type: pt, Wild: seg.Kind == ast.SegWild})
	}
	return params, ok
}

/* ===== resources (§V5) ===== */

func (c *checker) resourcesExpand(res *ast.Resources, inh inherited) []*RouteInfo {
	declared := res.Name.Name()
	if res.Name.Quoted() || !goIdentOK(declared) || (res.Pkg != nil && (res.Pkg.Quoted() || !goIdentOK(res.Pkg.Name()))) {
		c.errorf(res.Name.Pos(), "V5", "resources name %q must be a plain identifier (§V4.1.6)", res.Ref())
		return nil
	}

	paramName := "id"

	// §V5.5: [default] generates the handlers from the table's default
	// CRUD, which lives in an imported data package — so the reference
	// must be qualified, as a query route's is (§V4.8).
	def := false
	if res.Settings != nil {
		if s := res.Settings.Get("default"); s != nil {
			def = true
			if s.Value != nil {
				c.errorf(s.Pos(), "V5", "default is a flag and takes no value (§V5.5)")
			}
			if res.Pkg == nil {
				c.errorf(res.Name.Pos(), "V5", "resources [default] binds the table's generated CRUD, which lives in an imported data package; qualify the table: resources db.%s [default] (§V5.5)", declared)
				return nil
			}
		}
	}

	// §V5.1: the declaration MUST name a declared table — qualified or
	// not. Resolution gives the URL segment, the member helper (the
	// table's model name) and the key's Go type from the primary key;
	// there is no schemaless fallback to guess any of it from spelling.
	ti, ok, reported := c.resourceTable(res)
	if reported {
		return nil // the reference is wrong; expanding it would only add noise
	}
	if !ok {
		return nil // resourceTable reported the miss
	}
	tableName, singular := ti.Decl.Name.Base(), ""
	if model, err := golang.ModelName(ti.Decl); err == nil {
		singular = model
	}
	keyType, ok := c.pkParamType(ti, res.Name.Pos())
	if !ok {
		return nil
	}

	controller, err := golang.GoName(tableName)
	if err != nil {
		c.errorf(res.Name.Pos(), "V5", "resources name %q: %v (§V5.1)", declared, err)
		return nil
	}
	if singular == "" {
		singularRaw, _ := inflect.SingularLast(tableName)
		singular, err = golang.GoName(singularRaw)
		if err != nil {
			singular = controller
		}
	}
	api := false
	var only, except map[string]bool
	var onlyPos, exceptPos token.Position

	if res.Settings != nil {
		for _, s := range res.Settings.Settings {
			switch s.Name {
			case "api":
				if s.Value != nil {
					c.errorf(s.Pos(), "V5", "api is a flag and takes no value (§V5.3)")
				}
				api = true
			case "default":
				// validated above; implies api (§V5.5)
				api = true
			case "only", "except":
				list, ok := s.Value.(*ast.IdentList)
				if !ok {
					c.errorf(s.Pos(), "V5", "%s: takes an action list like (index, show) (§V5.3)", s.Name)
					continue
				}
				for i, mod := range list.Mods {
					if mod != nil {
						c.errorf(mod.Pos(), "V5", "%s: entries are single action names; %q does not belong after %q (§V5.3)", s.Name, mod.Name(), list.Names[i].Name())
					}
				}
				set := map[string]bool{}
				for _, id := range list.Names {
					canon, known := actionByLower[strings.ToLower(id.Name())]
					if !known {
						c.errorf(id.Pos(), "V5", "unknown action %q (§V5.2); valid: index, new, create, show, edit, update, delete", id.Name())
						continue
					}
					set[canon] = true
				}
				if s.Name == "only" {
					only, onlyPos = set, s.Pos()
				} else {
					except, exceptPos = set, s.Pos()
				}
			case "singular":
				id, ok := s.Value.(*ast.Ident)
				if !ok || id.Quoted() || !goIdentOK(id.Name()) {
					c.errorf(s.Pos(), "V5", "singular: takes a plain identifier (§V5.3)")
					continue
				}
				gn, err := golang.GoName(id.Name())
				if err != nil {
					c.errorf(s.Pos(), "V5", "singular %q: %v (§V5.3)", id.Name(), err)
					continue
				}
				singular = gn
			case "param":
				if def {
					c.errorf(s.Pos(), "V5", "param: does not apply with [default]; the key parameter is named by the primary-key column, as the generated CRUD spells it (§V5.5)")
					continue
				}
				id, ok := s.Value.(*ast.Ident)
				if !ok || !goIdentOK(id.Name()) || goKeywords[id.Name()] || reservedParamNames[id.Name()] {
					c.errorf(s.Pos(), "V5", "param: takes a valid, non-keyword, non-reserved Go identifier (§V5.3, §V4.1.2)")
					continue
				}
				paramName = id.Name()
			case "model":
				c.errorf(s.Pos(), "V5", "model: is not a setting; name the table in the declaration itself — `resources <table>` (§V5.1)")
			default:
				c.errorf(s.Pos(), "V6", "setting %q is not valid on resources (§V6); valid: api, only, except, param, singular, default", s.Name)
			}
		}
	}
	if only != nil && except != nil {
		c.errorf(exceptPos, "V5", "only: and except: cannot be combined (§V5.3)")
		_ = onlyPos
		except = nil
	}

	// §V5.5: the CRUD methods the default handlers call, by operation.
	var crud map[string]golang.CRUDMethod
	qual, target := "", ""
	if def {
		qual = res.Pkg.Name()
		target = c.pkg.Imports[qual]
		dp := c.pr.Packages[target]
		if dp == nil || !dp.HasSchema() {
			c.errorf(res.Pkg.Pos(), "V5", "package %q declares no tables; [default] needs a data package (§V5.5)", target)
			return nil
		}
		_, methods, err := golang.CRUDMethods(dp.Merged(), c.schemas[target], ti.Key)
		if err != nil {
			c.errorf(res.Name.Pos(), "V5", "resources %s.%s [default]: %v (§V5.5)", qual, declared, err)
			return nil
		}
		crud = map[string]golang.CRUDMethod{}
		for _, cm := range methods {
			crud[cm.Op] = cm
			if len(cm.Key) == 1 {
				paramName = cm.Key[0].GoName
			}
		}
	}

	// The key parameter must not repeat a parameter of the enclosing
	// scope (§V4.1.2); say so once, at the declaration, with the fix —
	// rather than once per member action from pathParams.
	for _, seg := range inh.prefix {
		if seg.Kind != ast.SegParam || seg.Name.Name() != paramName {
			continue
		}
		if def {
			c.errorf(res.Name.Pos(), "V5", "resources %s.%s [default]: the key parameter %q is already a parameter of the enclosing scope, and the generated CRUD fixes its name; give the scope's parameter another name (§V5.5, §V4.1.2)", qual, declared, paramName)
		} else {
			c.errorf(res.Name.Pos(), "V5", "resources %q: the key parameter %q is already a parameter of the enclosing scope; rename it with [param: <name>] (§V5.3, §V4.1.2)", declared, paramName)
		}
		return nil
	}

	nameSeg := func(n string) *ast.Segment { return litSeg(n, res.Pos()) }
	idSeg := &ast.Segment{Kind: ast.SegParam, MarkPos: res.Pos(),
		Name: &ast.Ident{Tok: token.Token{Kind: token.IDENT, Val: paramName, Pos: res.Pos()}},
		Type: &ast.Ident{Tok: token.Token{Kind: token.IDENT, Val: string(keyType), Pos: res.Pos()}}}

	var out []*RouteInfo
	for _, a := range resourceActions {
		if api && !a.API {
			continue
		}
		if only != nil && !only[a.Name] {
			continue
		}
		if except != nil && except[a.Name] {
			continue
		}
		segs := append(append([]*ast.Segment{}, inh.prefix...), nameSeg(tableName))
		if a.OnID {
			segs = append(segs, idSeg)
		}
		if a.Suffix != "" {
			segs = append(segs, nameSeg(strings.TrimPrefix(a.Suffix, "/")))
		}
		params, ok := c.pathParams(segs)
		if !ok {
			continue
		}
		helper := ""
		switch a.Name {
		case "Index":
			helper = inh.namePrefix + controller
		case "Show":
			helper = inh.namePrefix + singular
		case "New":
			helper = inh.namePrefix + "New" + singular
		case "Edit":
			helper = inh.namePrefix + "Edit" + singular
		}
		// §V5.5: with [default] the action is a query route over the
		// table's CRUD method — no controller. Writes have a client
		// method named like the form pages would be (CreateUser,
		// UpdateUser, DeleteUser) and no reverse-URL helper (§V4.8).
		var qr *QueryRef
		client := ""
		if def {
			cm, has := crud[a.Op]
			if !has {
				c.errorf(res.Name.Pos(), "V5", "resources %s.%s [default]: the generated CRUD has no %s method for %q (%s); drop the action with except: (%s) (§V5.5)",
					qual, declared, a.Op, a.Name, crudAbsent(a.Op), strings.ToLower(a.Name))
				continue
			}
			qr = c.queryBind(res.Name.Pos(), res.Pkg.Pos(), a.Methods[0], params, qual, target, cm.Name, func(string) token.Position { return res.Pos() })
			if qr == nil {
				continue
			}
			switch a.Name {
			case "Index", "Show":
				client = helper
			default:
				client = inh.namePrefix + a.Name + singular
			}
		}
		for _, m := range a.Methods {
			r := &RouteInfo{
				Method:       m,
				Pattern:      patternOf(segs),
				Spelled:      spelledOf(segs),
				Params:       params,
				Controller:   controller,
				Action:       a.Name,
				Query:        qr,
				HelperName:   helper,
				ClientName:   client,
				Pipes:        inh.pipes,
				ErrorHandler: inh.errHandler,
				Pos:          res.Pos(),

				FromResources: true,
				Table:         tableName,
			}
			if qr != nil {
				r.Controller, r.Action = "", ""
			}
			helper, client = "", "" // the names belong to the first method of the action
			out = append(out, r)
		}
	}
	return out
}

// crudAbsent says why a default CRUD method can be missing (CRUD-1 to
// CRUD-7): Update needs a non-key column to set; the others exist for
// every table with a primary key, which §V5.4 already requires.
func crudAbsent(op string) string {
	if op == "update" {
		return "every column is part of the key, so there is nothing to update"
	}
	return "the table has no primary key"
}

// resourceTable resolves a resources declaration to the table it names
// (§V5.1). Names are matched exactly: `Table posts` is `posts`, never
// `Posts` or `Post`, so what is written in routes.volt is what stands
// in the schema. Every declaration must resolve; a miss is an error
// that names the closest thing it can (a case variant, or the table
// whose model name was written).
func (c *checker) resourceTable(res *ast.Resources) (ti *check.TableInfo, ok, reported bool) {
	pkgPath := c.pkg.Path
	if res.Pkg != nil {
		qual := res.Pkg.Name()
		target, known := c.pkg.Imports[qual]
		if !known {
			c.errorf(res.Pkg.Pos(), "V5", "unknown package qualifier %q (§V5.1)", qual)
			return nil, false, true
		}
		c.usedQual[qual] = true
		pkgPath = target
	}
	info := c.schemas[pkgPath]
	if info == nil {
		return nil, false, false
	}
	want := res.Name.Name()
	var caseMatch string
	for _, cand := range info.Tables {
		name := cand.Decl.Name.Base()
		if name == want {
			return cand, true, false
		}
		if strings.EqualFold(name, want) {
			caseMatch = name
		}
	}
	// A name that differs only in case is a typo worth naming.
	if caseMatch != "" {
		c.errorf(res.Name.Pos(), "V5", "no table %q in package %q; did you mean %q? names are case-sensitive (§V5.1)", want, pkgPath, caseMatch)
		return nil, false, true
	}
	// A name that spells a table's MODEL name gets pointed at the table.
	for _, cand := range info.Tables {
		if model, err := golang.ModelName(cand.Decl); err == nil && model == want {
			c.errorf(res.Name.Pos(), "V5", "no table %q in package %q; %q is the model of table %q — name the table (§V5.1)", want, pkgPath, want, cand.Decl.Name.Base())
			return nil, false, true
		}
	}
	c.errorf(res.Name.Pos(), "V5", "no table %q in package %q (§V5.1)", want, pkgPath)
	return nil, false, true
}

// pkParamType maps a table's primary key to a route parameter type.
// Both declaration forms count (nao SPEC §6.3.3, §6.5.3): column-level
// pk flags/settings and pk-marked indexes.
func (c *checker) pkParamType(ti *check.TableInfo, pos token.Position) (ParamType, bool) {
	var pkNames []string
	seen := map[string]bool{}
	addPK := func(name string) {
		if !seen[name] {
			seen[name] = true
			pkNames = append(pkNames, name)
		}
	}
	for _, cd := range ti.Columns {
		col := cd.Col
		isPK := false
		for _, f := range col.LegacyFlags {
			if strings.EqualFold(f.Name(), "pk") {
				isPK = true
			}
		}
		if col.Settings.Get("pk") != nil || col.Settings.Get("primary key") != nil {
			isPK = true
		}
		if isPK {
			addPK(col.Name.Name())
		}
	}
	for _, ix := range ti.Indexes {
		if ix.Settings.Get("pk") == nil {
			continue
		}
		for _, atom := range ix.Key {
			if id, ok := atom.(*ast.Ident); ok {
				addPK(id.Name())
			}
		}
	}
	switch {
	case len(pkNames) == 0:
		c.errorf(pos, "V5", "table %q has no single-column primary key (§V5.4)", ti.Key)
		return "", false
	case len(pkNames) > 1:
		c.errorf(pos, "V5", "table %q has a composite primary key (%s); composite keys are not routable (§V5.4)", ti.Key, strings.Join(pkNames, ", "))
		return "", false
	}
	cd := ti.Column(pkNames[0])
	if cd == nil {
		c.errorf(pos, "V5", "table %q primary-key column %q not found (§V5.4)", ti.Key, pkNames[0])
		return "", false
	}
	pk := cd.Col
	goType, known := golang.GoTypeName(strings.ToLower(pk.Type.Name.String()))
	if !known {
		c.errorf(pos, "V5", "primary-key type %q of table %q is not a routable type (§V5.4)", pk.Type.String(), ti.Key)
		return "", false
	}
	switch goType {
	case "int", "int32", "int64", "string":
		return ParamType(goType), true
	default:
		c.errorf(pos, "V5", "primary-key Go type %q of table %q cannot be a route parameter (§V5.4); routable: int, int32, int64, string", goType, ti.Key)
		return "", false
	}
}

/* ===== accumulation: conflicts, helpers, controllers ===== */

func (c *checker) routeAdd(r *RouteInfo, seenShape, seenHelper map[string]*RouteInfo) {
	shape := r.Method + " " + shapeOf(r)
	if prev, dup := seenShape[shape]; dup {
		c.errorf(r.Pos, "V4", "route %s %s conflicts with the route at %s: identical method and path shape (§V4.7)", methodOrAny(r.Method), r.Spelled, prev.Pos)
		return
	}
	// §V4.7.2: beyond exact duplicates, reject ServeMux's ambiguity
	// relation — two routes whose request sets overlap with neither more
	// specific would panic at registration, and the checker's promise
	// (§V4.7.3) is that a checked project never does.
	for _, prev := range c.pkg.Routes {
		if routesAmbiguous(prev, r) {
			c.errorf(r.Pos, "V4", "route %s %s is ambiguous with the route at %s (%s %s): both match some requests and neither is more specific (§V4.7.2)",
				methodOrAny(r.Method), r.Spelled, prev.Pos, methodOrAny(prev.Method), prev.Spelled)
			return
		}
	}
	seenShape[shape] = r

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
			seenHelper[name] = r
		}
	}
	if r.Query == nil && !r.Events && r.HelperName != "" {
		r.ClientName = r.HelperName // a named controller route gets a raw client method (§V4.10)
	}

	if r.Query != nil || r.Events {
		c.pkg.Routes = append(c.pkg.Routes, r) // no controller: the handler is generated (§V4.8) or the runtime's (§V4.11)
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
		a.Routes = append(a.Routes, r)
	} else {
		ci.Actions = append(ci.Actions, &ActionInfo{Name: r.Action, Params: r.Params, Routes: []*RouteInfo{r}})
	}
	c.pkg.Routes = append(c.pkg.Routes, r)
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
	sa, sb := shapeParse(a), shapeParse(b)
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
