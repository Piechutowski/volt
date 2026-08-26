package lang

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/token"
	"github.com/Piechutowski/volt/orm/gen/golang"
	"github.com/Piechutowski/volt/orm/inflect"
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
				kw := "use"
				if d.Reuse {
					kw = "reuse"
				}
				c.errorf(d.Pos(), "V2", "%s is not part of the Volt language (§V2.5); replace file imports with the package system — see docs/dbml-imports.md for migration", kw)
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
	case q == "" || q == "volt" || q == c.pkg.Name:
		// existence is the Go compiler's business (§V3.2)
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
		}
	}
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
	if _, isImport := c.pkg.Imports[controller]; isImport {
		c.usedQual[controller] = true
		c.errorf(r.Handler.Pos(), "V4", "handlers live in the routes package; %q names an imported package (§V4.3)", controller)
		return nil
	}
	if r.Handler.Parts[0].Quoted() || r.Handler.Parts[1].Quoted() {
		c.errorf(r.Handler.Pos(), "V4", "handler names are plain (unquoted) identifiers (§V4.1.6), found %q", r.Handler.String())
		return nil
	}
	if !exportedIdentOK(controller) || !exportedIdentOK(action) {
		c.errorf(r.Handler.Pos(), "V4", "handler must be Controller.Action, both exported Go identifiers (§V4.3), found %q", r.Handler.String())
		return nil
	}

	helper := inh.namePrefix + action
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

	return &RouteInfo{
		Method:       method,
		Pattern:      patternOf(segs),
		Spelled:      spelledOf(segs),
		Params:       params,
		Controller:   controller,
		Action:       action,
		HelperName:   helper,
		Pipes:        inh.pipes,
		ErrorHandler: inh.errHandler,
		Pos:          r.Pos(),
	}
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
	keyType := TInt64

	// §V5.1: the declaration names a MODEL. Resolving it gives the
	// table (hence the URL segment) and the key's Go type, and makes
	// the model name authoritative for the member helper — no
	// singularization guess.
	tableName, singular := declared, ""
	if ti, ok := c.resourceModel(res); ok {
		tableName = ti.Decl.Name.Base()
		singular, _ = golang.GoName(declared)
		if t, ok := c.pkParamType(ti, res.Name.Pos()); ok {
			keyType = t
		}
	} else if res.Pkg == nil {
		c.resourceHint(res, declared)
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
			case "only", "except":
				list, ok := s.Value.(*ast.IdentList)
				if !ok {
					c.errorf(s.Pos(), "V5", "%s: takes an action list like (index, show) (§V5.3)", s.Name)
					continue
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
				id, ok := s.Value.(*ast.Ident)
				if !ok || !goIdentOK(id.Name()) || goKeywords[id.Name()] || reservedParamNames[id.Name()] {
					c.errorf(s.Pos(), "V5", "param: takes a valid, non-keyword, non-reserved Go identifier (§V5.3, §V4.1.2)")
					continue
				}
				paramName = id.Name()
			case "model":
				c.errorf(s.Pos(), "V5", "model: is not a setting; name the model in the declaration itself — `resources %s` (§V5.1)", settingRefText(s))
			default:
				c.errorf(s.Pos(), "V6", "setting %q is not valid on resources (§V6); valid: api, only, except, param, singular", s.Name)
			}
		}
	}
	if only != nil && except != nil {
		c.errorf(exceptPos, "V5", "only: and except: cannot be combined (§V5.3)")
		_ = onlyPos
		except = nil
	}

	nameSeg := func(n string) *ast.Segment {
		return &ast.Segment{Kind: ast.SegLit, Name: &ast.Ident{Tok: token.Token{Kind: token.IDENT, Val: n, Pos: res.Pos()}}}
	}
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
		for _, m := range a.Methods {
			r := &RouteInfo{
				Method:       m,
				Pattern:      patternOf(segs),
				Spelled:      spelledOf(segs),
				Params:       params,
				Controller:   controller,
				Action:       a.Name,
				HelperName:   helper,
				Pipes:        inh.pipes,
				ErrorHandler: inh.errHandler,
				Pos:          res.Pos(),

				FromResources: true,
			}
			helper = "" // the helper belongs to the first method of the action
			out = append(out, r)
		}
	}
	return out
}

// resourceHint notes when a bare declaration names a table that has a
// model — the author almost certainly meant the model (§V5.1).
func (c *checker) resourceHint(res *ast.Resources, declared string) {
	info := c.schemas[c.pkg.Path]
	if info == nil {
		return
	}
	for _, ti := range info.Tables {
		if ti.Decl.Name.Base() != declared {
			continue
		}
		if model, err := golang.ModelName(ti.Decl); err == nil && model != declared {
			c.pkg.resourceHints = append(c.pkg.resourceHints,
				resourceHint{pos: res.Name.Pos(), declared: declared, suggest: model})
		}
		return
	}
}

// resourceModel resolves a resources declaration to the table whose
// model name it spells (§V5.1). Not every declaration names a model —
// a resource without a schema is legal — so a miss is not an error
// here; vet suggests the model form when one plainly exists.
func (c *checker) resourceModel(res *ast.Resources) (*check.TableInfo, bool) {
	pkgPath := c.pkg.Path
	if res.Pkg != nil {
		qual := res.Pkg.Name()
		target, ok := c.pkg.Imports[qual]
		if !ok {
			c.errorf(res.Pkg.Pos(), "V5", "unknown package qualifier %q (§V5.1)", qual)
			return nil, false
		}
		c.usedQual[qual] = true
		pkgPath = target
	}
	info := c.schemas[pkgPath]
	if info == nil {
		return nil, false
	}
	want := res.Name.Name()
	for _, ti := range info.Tables {
		if name, err := golang.ModelName(ti.Decl); err == nil && name == want {
			return ti, true
		}
	}
	if res.Pkg != nil {
		// Qualified means the author meant a model; a miss is an error.
		c.errorf(res.Name.Pos(), "V5", "no model %q in package %q (§V5.1)", want, pkgPath)
	}
	return nil, false
}

// settingRefText renders a legacy model: value for the migration hint.
func settingRefText(s *ast.Setting) string {
	switch v := s.Value.(type) {
	case *ast.Ident:
		return v.Name()
	case *ast.EnumConst:
		return v.Enum.Name() + "." + v.Value.Name()
	}
	return "<Model>"
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

	if r.HelperName != "" {
		if prev, dup := seenHelper[r.HelperName]; dup {
			// A resources declaration colliding with itself means its
			// table name survived singularization unchanged (nao's
			// inflector is English), so the collection and member
			// helpers came out identical. Neither [name:] (not a
			// resources setting) nor a scope name helps there — both
			// sides get the same prefix — so name the working fix.
			if r.FromResources && prev.FromResources && r.Pos == prev.Pos {
				c.errorf(r.Pos, "V5", "resources %q: the collection and member reverse-URL helpers are both %q, because singularizing %q does not change it (§V5.4); set the singular explicitly, e.g. [singular: <name>]",
					prev.Controller, r.HelperName, prev.Controller)
			} else {
				c.errorf(r.Pos, "V4", "reverse-URL helper %q already produced by the route at %s (§V4.6); disambiguate with [name:] or a scope name", r.HelperName, prev.Pos)
			}
			r.HelperName = ""
		} else {
			seenHelper[r.HelperName] = r
		}
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
	return fmt.Sprintf("%-7s %-32s %s.%s %s", methodOrAny(r.Method), r.Spelled, r.Controller, r.Action, helper)
}
