package lang

// The lowering of one scope item (§V4, §V5, §V13) as a pure function
// of explicit inputs (D86, D99): a route, a resources declaration or a
// dataset, with the context its scopes inherited, the package's own
// facts, and an oracle it questions about the project and whose
// answers it returns, so the session's memo can verify on the next
// check that every answer stands and replay the item's routes and
// diagnostics. The side effects the lowering used to have, marking an
// import used and accepting routes, are the caller's, applied from
// what it returns.

import (
	"maps"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/token"
	"github.com/Piechutowski/volt/nao/gen/golang"
	"github.com/Piechutowski/volt/nao/inflect"
)

// routeFacts are the package's own facts a scope item's lowering
// reads, compared as values by the memo.
type routeFacts struct {
	pkgName, pkgPath, module string
	imports                  map[string]string // qualifier -> package path
}

func (f routeFacts) equal(g routeFacts) bool {
	return f.pkgName == g.pkgName && f.pkgPath == g.pkgPath && f.module == g.module && maps.Equal(f.imports, g.imports)
}

// routeEnv is what a scope item's lowering may ask of the project
// beyond its own package's facts. Read only: every answer is an
// identity or a value the lowering records and returns.
type routeEnv interface {
	// dataPackage describes a package as a query target: its Go name,
	// whether it is loaded with a schema, whether it declares selects.
	dataPackage(path string) (pkgName string, hasSchema, hasSelects bool)
	// tableByBase finds a table of a package's schema by base name; the
	// hint describes the nearest miss when there is none.
	tableByBase(path, base string) (ti *check.TableInfo, hint string)
	// modelOf is the identity of a table's model in its package's plan
	// with the CRUD methods the plan mints for it, or the plan's error.
	modelOf(path, key string) (model any, methods []golang.CRUDMethod, err error)
	// crudByName finds the table and method a package's plan mints
	// under a CRUD method name.
	crudByName(path, name string) (key string, model any, cm golang.CRUDMethod, ok bool)
	// selectByMethod finds the select and member minting a method name.
	selectByMethod(path, name string) (sel *SelectInfo, member *check.TableInfo, ok bool)
	// selectByName finds a package's select by declared name; the hint
	// describes a case-insensitive match when there is none.
	selectByName(path, name string) (si *SelectInfo, hint string)
	// validators reports whether a table's params structs validate.
	validators(path, key string) (create, update bool)
	// queryHint is the did-you-mean for a query method a package lacks.
	queryHint(path, name string) string
}

type lookupKind uint8

const (
	lkDataPackage lookupKind = iota
	lkTableByBase
	lkModelOf
	lkCrudByName
	lkSelectByMethod
	lkSelectByName
	lkValidators
	lkQueryHint
)

// routeLookup is one question a lowering asked the oracle and the
// answer it got, by identity or by value.
type routeLookup struct {
	kind       lookupKind
	path, name string

	pkgName, key, hint, err                string
	hasSchema, hasSelects, ok, create, ok2 bool
	ti, member                             *check.TableInfo
	sel                                    *SelectInfo
	model                                  any
}

// holds reports whether the oracle still gives the recorded answer.
func (l routeLookup) holds(env routeEnv) bool {
	switch l.kind {
	case lkDataPackage:
		n, s, sel := env.dataPackage(l.path)
		return n == l.pkgName && s == l.hasSchema && sel == l.hasSelects
	case lkTableByBase:
		ti, hint := env.tableByBase(l.path, l.name)
		return ti == l.ti && hint == l.hint
	case lkModelOf:
		model, _, err := env.modelOf(l.path, l.key)
		return model == l.model && errString(err) == l.err
	case lkCrudByName:
		key, model, _, ok := env.crudByName(l.path, l.name)
		return key == l.key && model == l.model && ok == l.ok
	case lkSelectByMethod:
		sel, m, ok := env.selectByMethod(l.path, l.name)
		return sel == l.sel && m == l.member && ok == l.ok
	case lkSelectByName:
		si, hint := env.selectByName(l.path, l.name)
		return si == l.sel && hint == l.hint
	case lkValidators:
		c, u := env.validators(l.path, l.key)
		return c == l.create && u == l.ok2
	case lkQueryHint:
		return env.queryHint(l.path, l.name) == l.hint
	}
	return false
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// routeLowering is one scope item's lowering: its inputs, and what it
// accumulates to return, the questions asked, the import qualifiers
// used, and the diagnostics.
type routeLowering struct {
	facts  routeFacts
	env    routeEnv
	record bool // keep the questions asked: a memo will verify them

	lookups []routeLookup
	used    []string
	diags   []diag.Diagnostic
}

func (l *routeLowering) errorf(pos token.Position, section, format string, args ...any) {
	l.diags = append(l.diags, diag.Errorf(pos, "spec/"+section, format, args...))
}

// note keeps a question and its answer for the memo, when one asked.
func (l *routeLowering) note(lookup routeLookup) {
	if l.record {
		l.lookups = append(l.lookups, lookup)
	}
}

// importOf resolves an import qualifier, noting its use.
func (l *routeLowering) importOf(qual string) (string, bool) {
	path, ok := l.facts.imports[qual]
	if ok {
		l.used = append(l.used, qual)
	}
	return path, ok
}

func (l *routeLowering) dataPackage(path string) (string, bool, bool) {
	name, hasSchema, hasSelects := l.env.dataPackage(path)
	l.note(routeLookup{kind: lkDataPackage, path: path, pkgName: name, hasSchema: hasSchema, hasSelects: hasSelects})
	return name, hasSchema, hasSelects
}

func (l *routeLowering) tableByBase(path, base string) (*check.TableInfo, string) {
	ti, hint := l.env.tableByBase(path, base)
	l.note(routeLookup{kind: lkTableByBase, path: path, name: base, ti: ti, hint: hint})
	return ti, hint
}

func (l *routeLowering) modelOf(path, key string) (any, []golang.CRUDMethod, error) {
	model, methods, err := l.env.modelOf(path, key)
	l.note(routeLookup{kind: lkModelOf, path: path, key: key, model: model, err: errString(err)})
	return model, methods, err
}

func (l *routeLowering) crudByName(path, name string) (string, any, golang.CRUDMethod, bool) {
	key, model, cm, ok := l.env.crudByName(path, name)
	l.note(routeLookup{kind: lkCrudByName, path: path, name: name, key: key, model: model, ok: ok})
	return key, model, cm, ok
}

func (l *routeLowering) selectByMethod(path, name string) (*SelectInfo, *check.TableInfo, bool) {
	sel, m, ok := l.env.selectByMethod(path, name)
	l.note(routeLookup{kind: lkSelectByMethod, path: path, name: name, sel: sel, member: m, ok: ok})
	return sel, m, ok
}

func (l *routeLowering) selectByName(path, name string) (*SelectInfo, string) {
	si, hint := l.env.selectByName(path, name)
	l.note(routeLookup{kind: lkSelectByName, path: path, name: name, sel: si, hint: hint})
	return si, hint
}

func (l *routeLowering) validators(path, key string) (bool, bool) {
	c, u := l.env.validators(path, key)
	l.note(routeLookup{kind: lkValidators, path: path, key: key, create: c, ok2: u})
	return c, u
}

func (l *routeLowering) queryHint(path, name string) string {
	hint := l.env.queryHint(path, name)
	l.note(routeLookup{kind: lkQueryHint, path: path, name: name, hint: hint})
	return hint
}

// itemLower lowers one scope item to its routes from exactly the
// inputs named on routeLowering, reading nothing else and writing
// nothing it did not create. Beside the routes it returns what it
// asked the oracle and was told, the import qualifiers it used, and
// its diagnostics; the questions are kept only when record asks,
// for a memo to verify.
func itemLower(item ast.Node, inh inherited, facts routeFacts, env routeEnv, record bool) (routes []*RouteInfo, lookups []routeLookup, used []string, diags []diag.Diagnostic) {
	l := &routeLowering{facts: facts, env: env, record: record}
	switch item := item.(type) {
	case *ast.Route:
		if r := l.route(item, inh); r != nil {
			routes = []*RouteInfo{r}
		}
	case *ast.Resources:
		routes = l.resources(item, inh)
	case *ast.Dataset:
		routes = l.dataset(item, inh)
	}
	for _, r := range routes {
		r.shape = shapeParse(r) // a function of the pattern: parsed here, once (D99)
		r.shapeKey = r.Method + " " + shapeOf(r)
	}
	return routes, l.lookups, l.used, l.diags
}

/* ===== datasets (§V13) ===== */

// dataset turns `dataset db.browse [strip: 'da_']` into one GET query
// route per member of the select's target: the segment is the
// member's table name with the strip prefix removed, the handler the
// member's select method, bound like any query route (§V4.8).
func (l *routeLowering) dataset(ds *ast.Dataset, inh inherited) []*RouteInfo {
	// §V13.1: the select is this package's own — bare or self-qualified,
	// as a plug is (§V3.2) — or an imported package's, qualified.
	qual, target, local := l.facts.pkgName, l.facts.pkgPath, true
	qualPos := ds.Name.Pos()
	if ds.Pkg != nil && ds.Pkg.Name() != l.facts.pkgName {
		qual, qualPos, local = ds.Pkg.Name(), ds.Pkg.Pos(), false
		known := false
		if target, known = l.importOf(qual); !known {
			l.errorf(ds.Pkg.Pos(), "V13", "unknown package qualifier %q (§V13.1)", qual)
			return nil
		}
	}
	_, _, hasSelects := l.dataPackage(target)
	if local && !hasSelects {
		l.errorf(ds.Name.Pos(), "V13", "package %q declares no select; a dataset names a select of this package, or of an imported data package, qualified (§V13.1)", target)
		return nil
	}
	si, hint := l.selectByName(target, ds.Name.Name())
	if si == nil {
		l.errorf(ds.Name.Pos(), "V13", "no select %q in package %q%s (§V13.1)", ds.Name.Name(), target, hint)
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
					l.errorf(s.Pos(), "V13", "strip: takes a string, the table-name prefix to drop from the URL segment (§V13.2)")
					continue
				}
				strip = lit.Tok.Val
			case "only", "except":
				list, ok := s.Value.(*ast.IdentList)
				if !ok {
					l.errorf(s.Pos(), "V13", "%s: takes a table list like (da_a_a, da_b_b) (§V13.2)", s.Name)
					continue
				}
				set := map[string]bool{}
				for i, id := range list.Names {
					if list.Mods[i] != nil {
						l.errorf(list.Mods[i].Pos(), "V13", "%s: entries are table names; %q does not belong after %q (§V13.2)", s.Name, list.Mods[i].Name(), id.Name())
					}
					if !members[id.Name()] {
						l.errorf(id.Pos(), "V13", "%s: %q is not a member of select %q's target (§V13.2)", s.Name, id.Name(), si.Decl.Name.Name())
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
				l.errorf(s.Pos(), "V6", "setting %q is not valid on a dataset (§V6); valid: strip, only, except", s.Name)
			}
		}
	}
	if only != nil && except != nil {
		pos := onlyPos
		if exceptPos.Line() > onlyPos.Line() || (exceptPos.Line() == onlyPos.Line() && exceptPos.Column() > onlyPos.Column()) {
			pos = exceptPos
		}
		l.errorf(pos, "V13", "only: and except: cannot both be set on a dataset (§V13.2)")
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
				l.errorf(ds.Pos(), "V13", "strip: %q is not a prefix of member table %q (§V13.2)", strip, base)
				continue
			}
			seg = strings.TrimPrefix(base, strip)
			if seg == "" || !goIdentOK(seg) {
				l.errorf(ds.Pos(), "V13", "strip: %q leaves member table %q with no usable segment (§V13.2)", strip, base)
				continue
			}
		}
		segs := append(append([]*ast.Segment{}, inh.prefix...), litSeg(seg, ds.Pos()))
		params, ok := l.pathParams(segs)
		if !ok {
			continue
		}
		method := modelOrBase(m) + si.MethodSuffix
		qr := l.queryBind(ds.Name.Pos(), qualPos, "GET", params, qual, target, local, method, paramAt{at: ds.Pos()})
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
func (l *routeLowering) selfFuncRef(v ast.Node) (string, bool) {
	return selfFuncRefIn(v, l.facts.pkgName)
}

func selfFuncRefIn(v ast.Node, pkgName string) (string, bool) {
	usable := func(id *ast.Ident) bool {
		return !id.Quoted() && goIdentOK(id.Name()) && !goKeywords[id.Name()]
	}
	switch v := v.(type) {
	case *ast.Ident:
		if usable(v) {
			return v.Name(), true
		}
	case *ast.EnumConst:
		if v.Enum.Name() == pkgName && usable(v.Value) {
			return v.Value.Name(), true
		}
	}
	return "", false
}

/* ===== routes (§V4) ===== */

func (l *routeLowering) route(r *ast.Route, inh inherited) *RouteInfo {
	method := methodOf(r.Verb())
	segs := append(append([]*ast.Segment{}, inh.prefix...), r.Path.Segments...)
	params, ok := l.pathParams(segs)
	if !ok {
		return nil
	}

	for _, part := range r.Handler.Parts {
		if part.Quoted() {
			l.errorf(r.Handler.Pos(), "V4", "handler names are plain (unquoted) identifiers (§V4.1.6), found %q", r.Handler.String())
			return nil
		}
	}
	// §V4.3: `Query` or `<package name>.Query` names a generated query
	// of this package and `pkg.Query` one of an imported package — a
	// query route (§V4.8); `volt.Events` is the runtime's; anything
	// else is Controller.Action.
	var query *QueryRef
	events := false
	controller, action := "", r.Handler.Parts[0].Name()
	if len(r.Handler.Parts) == 2 {
		controller, action = action, r.Handler.Parts[1].Name()
	}
	switch {
	case controller == "volt":
		// The runtime's own handlers: today exactly one, the event stream.
		if action != "Events" {
			l.errorf(r.Handler.Pos(), "V4", "the runtime provides no handler volt.%s; volt.Events is the event stream (§V4.11)", action)
			return nil
		}
		if method != "GET" {
			l.errorf(r.Pos(), "V4", "an event route is read with get (§V4.11)")
			return nil
		}
		events = true
	case controller == "" || controller == l.facts.pkgName:
		query = l.queryRef(r, method, params, l.facts.pkgName, l.facts.pkgPath, true)
		if query == nil {
			return nil
		}
		controller, action = "", query.Method
	default:
		if target, isImport := l.importOf(controller); isImport {
			query = l.queryRef(r, method, params, controller, target, false)
			if query == nil {
				return nil
			}
			controller, action = "", query.Method
		} else if !exportedIdentOK(controller) || !exportedIdentOK(action) {
			l.errorf(r.Handler.Pos(), "V4", "handler must be Controller.Action, both exported Go identifiers (§V4.3), found %q", r.Handler.String())
			return nil
		}
	}

	helper := inh.namePrefix + action
	client := ""
	if s := settingOf(r.Settings, "name"); s != nil {
		id, ok := s.Value.(*ast.Ident)
		if !ok {
			l.errorf(s.Pos(), "V4", "name: takes an identifier (§V4.6)")
		} else if n, err := golang.GoName(id.Name()); err != nil {
			l.errorf(s.Pos(), "V4", "route name %q: %v (§V4.6)", id.Name(), err)
		} else {
			helper = inh.namePrefix + n
		}
	}
	if r.Settings != nil {
		for _, s := range r.Settings.Settings {
			if s.Name != "name" {
				l.errorf(s.Pos(), "V6", "setting %q is not valid on a route (§V6); valid: name", s.Name)
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

// queryRef resolves `pkg.Method`, `Method` or `<this package>.Method`
// to a generated query of the named package — a select method (§V11.6)
// or a default CRUD method — and binds the route's parameters to the
// method's (§V4.8).
func (l *routeLowering) queryRef(r *ast.Route, method string, params []Param, qual, target string, local bool) *QueryRef {
	last := r.Handler.Parts[len(r.Handler.Parts)-1]
	return l.queryBind(last.Pos(), r.Handler.Parts[0].Pos(), method, params, qual, target, local, last.Name(), paramAt{route: r})
}

// queryBind resolves a query method by name in the data package — an
// imported one, or this package when local — and binds the route's
// parameters; segAt locates a path parameter for diagnostics. Shared by
// query routes, resources [default] and datasets.
func (l *routeLowering) queryBind(pos, qualPos token.Position, method string, params []Param, qual, target string, local bool, name string, segAt paramAt) *QueryRef {
	pkgName, hasSchema, _ := l.dataPackage(target)
	ref := qual + "." + name
	if local {
		ref = name
	}
	if !hasSchema {
		if local {
			l.errorf(pos, "V4", "package %q declares no tables, so it has no query %s; a query of an imported data package is written qualified (§V4.8)", target, name)
		} else {
			l.errorf(pos, "V4", "package %q declares no tables; a query route needs a data package (§V4.8)", target)
		}
		return nil
	}
	// The Controllers field: the qualifier as a Go name, or Queries for
	// the package's own handle (§V4.8.5).
	field := "Queries"
	if !local {
		var err error
		if field, err = golang.GoName(qual); err != nil {
			l.errorf(qualPos, "V4", "import qualifier %q: %v (§V4.8)", qual, err)
			return nil
		}
	}
	importPath := l.facts.module
	if target != "." {
		importPath += "/" + target
	}
	qr := &QueryRef{Qualifier: qual, Field: field, Package: target, Import: importPath, PkgName: pkgName, Local: local, Method: name, Status: 200}

	// Signature parameters of the named method, in order.
	type sigParam struct {
		name, goType string
		body         bool
		validates    bool
	}
	var sig []sigParam
	found := false

	// Selects: <Model><SelectName> per member (§V11.6); the first member
	// minting the name owns it.
	if si, m, ok := l.selectByMethod(target, name); ok {
		found = true
		for _, p := range si.Params {
			sig = append(sig, sigParam{name: p.GoName, goType: p.GoType})
		}
		switch {
		case si.Shared != "":
			qr.Result = si.Shared
		case len(si.Excluded) > 0:
			qr.Result = name
		default:
			qr.Result = modelOrBase(m)
		}
		qr.Many = true
	}
	// Default CRUD (CRUD-1 to CRUD-7), from the package's plan (D74).
	if !found {
		if key, _, cm, ok := l.crudByName(target, name); ok {
			found = true
			for _, k := range cm.Key {
				sig = append(sig, sigParam{name: k.GoName, goType: k.GoType})
			}
			if cm.Body != "" {
				// The params struct validates when it carries the
				// columns of at least one check (§V12.6).
				create, update := l.validators(target, key)
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
	if !found {
		// Did you mean: every select method, then every CRUD method, in
		// declaration order — only worth listing on the way to an error.
		hint := l.queryHint(target, name)
		l.errorf(pos, "V4", "no generated query %s in package %q%s — a query route names a select method or a default CRUD method (Get, List, Create, Update, Delete) (§V4.8)", ref, target, hint)
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
				l.errorf(pos, "V4", "%s takes a request body (%s); route it with post, put or patch (§V4.8)", ref, sp.goType)
				ok = false
			}
		case strings.HasPrefix(sp.goType, "[]"):
			qp.Source = FromList
			if pp, inPath := byName[sp.name]; inPath {
				l.errorf(segAt.pos(pp.Name), "V4", "list parameter %q of %s cannot be a path parameter; pass it as a repeated query key (§V4.8)", sp.name, ref)
				ok = false
			}
		default:
			if pp, inPath := byName[sp.name]; inPath {
				qp.Source = FromPath
				bound[sp.name] = true
				if pp.Wild {
					l.errorf(segAt.pos(pp.Name), "V4", "parameter %q of %s cannot be a wildcard (§V4.8)", sp.name, ref)
					ok = false
				} else if pp.Type.GoType() != sp.goType {
					if KnownParamType(sp.goType) {
						l.errorf(segAt.pos(pp.Name), "V4", "path parameter %q is %s but %s takes %s; spell it :%s(%s) (§V4.8)", sp.name, pp.Type.GoType(), ref, sp.goType, sp.name, sp.goType)
					} else {
						l.errorf(segAt.pos(pp.Name), "V4", "parameter %q of %s is %s, which a path segment cannot carry (§V4.1.3); pass it in the query string (§V4.8)", sp.name, ref, sp.goType)
					}
					ok = false
				}
			} else {
				qp.Source = FromQuery
				if !queryValueTypes[sp.goType] {
					l.errorf(pos, "V4", "parameter %q of %s is %s, which the query string cannot carry (§V4.8)", sp.name, ref, sp.goType)
					ok = false
				}
			}
		}
		qr.Params = append(qr.Params, qp)
	}
	for _, p := range params {
		if !bound[p.Name] {
			l.errorf(segAt.pos(p.Name), "V4", "path parameter %q is not a parameter of %s (§V4.8)", p.Name, ref)
			ok = false
		}
	}
	if !ok {
		return nil
	}
	return qr
}

// litSeg builds a synthetic literal segment for an expanded route.
func litSeg(n string, at token.Position) *ast.Segment {
	return &ast.Segment{Kind: ast.SegLit, Name: &ast.Ident{Tok: token.Token{Kind: token.IDENT, Val: n, Pos: at}}}
}

// segPos finds the position of a named parameter segment in the route's
// own path, falling back to the route.
// paramAt locates a path parameter for a diagnostic: in a route, the
// segment naming it; for a dataset or a resources, the item itself.
type paramAt struct {
	route *ast.Route
	at    token.Position
}

func (p paramAt) pos(name string) token.Position {
	if p.route != nil {
		return segPos(p.route, name)
	}
	return p.at
}

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
func (l *routeLowering) pathParams(segs []*ast.Segment) ([]Param, bool) {
	var params []Param
	seen := map[string]bool{}
	ok := true
	for i, seg := range segs {
		// §V4.1.6: every segment name — literal, parameter or wildcard —
		// is a plain, unquoted identifier; anything else would flow
		// unescaped into registration patterns and generated Go.
		if seg.Name.Quoted() || !goIdentOK(seg.Name.Name()) {
			l.errorf(seg.Pos(), "V4", "path segment %q must be a plain identifier of letters, digits and underscores (§V4.1.6)", seg.Name.Name())
			ok = false
			continue
		}
		switch seg.Kind {
		case ast.SegLit:
			continue
		case ast.SegWild:
			if i != len(segs)-1 {
				l.errorf(seg.Pos(), "V4", "the wildcard ':%s...' must be the last path segment (§V4.1.4)", seg.Name.Name())
				ok = false
			}
		}
		name := seg.Name.Name()
		if seen[name] {
			l.errorf(seg.Pos(), "V4", "duplicate path parameter %q (§V4.1.2)", name)
			ok = false
			continue
		}
		seen[name] = true
		if !goIdentOK(name) || goKeywords[name] {
			l.errorf(seg.Pos(), "V4", "path parameter %q must be a valid, non-keyword Go identifier (§V4.1.2)", name)
			ok = false
			continue
		}
		if reservedParamNames[name] {
			l.errorf(seg.Pos(), "V4", "path parameter %q collides with a name in generated signatures (§V4.1.2); reserved: w, r, opts, volt", name)
			ok = false
			continue
		}
		pt := TString
		if seg.Type != nil {
			if !KnownParamType(seg.Type.Name()) {
				l.errorf(seg.Type.Pos(), "V4", "unknown parameter type %q (§V4.1.3); valid: int, int32, int64, string", seg.Type.Name())
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

func (l *routeLowering) resources(res *ast.Resources, inh inherited) []*RouteInfo {
	declared := res.Name.Name()
	if res.Name.Quoted() || !goIdentOK(declared) || (res.Pkg != nil && (res.Pkg.Quoted() || !goIdentOK(res.Pkg.Name()))) {
		l.errorf(res.Name.Pos(), "V5", "resources name %q must be a plain identifier (§V4.1.6)", res.Ref())
		return nil
	}

	paramName := "id"

	// §V5.5: [default] generates the handlers from the table's default
	// CRUD: this package's own for a bare or self-qualified table, an
	// imported package's for a qualified one, as for a query route (§V4.8).
	def := false
	if res.Settings != nil {
		if s := res.Settings.Get("default"); s != nil {
			def = true
			if s.Value != nil {
				l.errorf(s.Pos(), "V5", "default is a flag and takes no value (§V5.5)")
			}
		}
	}

	// §V5.1: the declaration MUST name a declared table — qualified or
	// not. Resolution gives the URL segment, the member helper (the
	// table's model name) and the key's Go type from the primary key;
	// there is no schemaless fallback to guess any of it from spelling.
	ti, ok, reported := l.resourceTable(res)
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
	keyType, ok := l.pkParamType(ti, res.Name.Pos())
	if !ok {
		return nil
	}

	controller, err := golang.GoName(tableName)
	if err != nil {
		l.errorf(res.Name.Pos(), "V5", "resources name %q: %v (§V5.1)", declared, err)
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
					l.errorf(s.Pos(), "V5", "api is a flag and takes no value (§V5.3)")
				}
				api = true
			case "default":
				// validated above; implies api (§V5.5)
				api = true
			case "only", "except":
				list, ok := s.Value.(*ast.IdentList)
				if !ok {
					l.errorf(s.Pos(), "V5", "%s: takes an action list like (index, show) (§V5.3)", s.Name)
					continue
				}
				for i, mod := range list.Mods {
					if mod != nil {
						l.errorf(mod.Pos(), "V5", "%s: entries are single action names; %q does not belong after %q (§V5.3)", s.Name, mod.Name(), list.Names[i].Name())
					}
				}
				set := map[string]bool{}
				for _, id := range list.Names {
					canon, known := actionByLower[strings.ToLower(id.Name())]
					if !known {
						l.errorf(id.Pos(), "V5", "unknown action %q (§V5.2); valid: index, new, create, show, edit, update, delete", id.Name())
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
					l.errorf(s.Pos(), "V5", "singular: takes a plain identifier (§V5.3)")
					continue
				}
				gn, err := golang.GoName(id.Name())
				if err != nil {
					l.errorf(s.Pos(), "V5", "singular %q: %v (§V5.3)", id.Name(), err)
					continue
				}
				singular = gn
			case "param":
				if def {
					l.errorf(s.Pos(), "V5", "param: does not apply with [default]; the key parameter is named by the primary-key column, as the generated CRUD spells it (§V5.5)")
					continue
				}
				id, ok := s.Value.(*ast.Ident)
				if !ok || !goIdentOK(id.Name()) || goKeywords[id.Name()] || reservedParamNames[id.Name()] {
					l.errorf(s.Pos(), "V5", "param: takes a valid, non-keyword, non-reserved Go identifier (§V5.3, §V4.1.2)")
					continue
				}
				paramName = id.Name()
			case "model":
				l.errorf(s.Pos(), "V5", "model: is not a setting; name the table in the declaration itself — `resources <table>` (§V5.1)")
			default:
				l.errorf(s.Pos(), "V6", "setting %q is not valid on resources (§V6); valid: api, only, except, param, singular, default", s.Name)
			}
		}
	}
	if only != nil && except != nil {
		l.errorf(exceptPos, "V5", "only: and except: cannot be combined (§V5.3)")
		_ = onlyPos
		except = nil
	}

	// §V5.5: the CRUD methods the default handlers call, by operation.
	var crud map[string]golang.CRUDMethod
	qual, target, local := l.facts.pkgName, l.facts.pkgPath, true
	qualPos := res.Name.Pos()
	if res.Pkg != nil && res.Pkg.Name() != l.facts.pkgName {
		qual, local = res.Pkg.Name(), false
		target, _ = l.importOf(res.Pkg.Name())
		qualPos = res.Pkg.Pos()
	}
	if def {
		// resourceTable resolved the table, so the package has a schema
		// and, with it, a plan (D74).
		_, methods, err := l.modelOf(target, ti.Key)
		if err != nil {
			l.errorf(res.Name.Pos(), "V5", "resources %s [default]: %v (§V5.5)", res.Ref(), err)
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
			l.errorf(res.Name.Pos(), "V5", "resources %s [default]: the key parameter %q is already a parameter of the enclosing scope, and the generated CRUD fixes its name; give the scope's parameter another name (§V5.5, §V4.1.2)", res.Ref(), paramName)
		} else {
			l.errorf(res.Name.Pos(), "V5", "resources %q: the key parameter %q is already a parameter of the enclosing scope; rename it with [param: <name>] (§V5.3, §V4.1.2)", declared, paramName)
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
		params, ok := l.pathParams(segs)
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
				l.errorf(res.Name.Pos(), "V5", "resources %s [default]: the generated CRUD has no %s method for %q (%s); drop the action with except: (%s) (§V5.5)",
					res.Ref(), a.Op, a.Name, crudAbsent(a.Op), strings.ToLower(a.Name))
				continue
			}
			qr = l.queryBind(res.Name.Pos(), qualPos, a.Methods[0], params, qual, target, local, cm.Name, paramAt{at: res.Pos()})
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
func (l *routeLowering) resourceTable(res *ast.Resources) (ti *check.TableInfo, ok, reported bool) {
	pkgPath := l.facts.pkgPath
	if res.Pkg != nil && res.Pkg.Name() != l.facts.pkgName {
		qual := res.Pkg.Name()
		target, known := l.importOf(qual)
		if !known {
			l.errorf(res.Pkg.Pos(), "V5", "unknown package qualifier %q (§V5.1)", qual)
			return nil, false, true
		}
		pkgPath = target
	}
	if _, hasSchema, _ := l.dataPackage(pkgPath); !hasSchema {
		l.errorf(res.Name.Pos(), "V5", "package %q declares no tables; resources names a table of this package, or of an imported data package, qualified (§V5.1)", pkgPath)
		return nil, false, true
	}
	want := res.Name.Name()
	if cand, hint := l.tableByBase(pkgPath, want); cand != nil {
		return cand, true, false
	} else {
		l.errorf(res.Name.Pos(), "V5", "no table %q in package %q%s (§V5.1)", want, pkgPath, hint)
	}
	return nil, false, true
}

// pkParamType maps a table's primary key to a route parameter type.
// Both declaration forms count (nao SPEC §6.3.3, §6.5.3): column-level
// pk flags/settings and pk-marked indexes.
func (l *routeLowering) pkParamType(ti *check.TableInfo, pos token.Position) (ParamType, bool) {
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
		l.errorf(pos, "V5", "table %q has no single-column primary key (§V5.4)", ti.Key)
		return "", false
	case len(pkNames) > 1:
		l.errorf(pos, "V5", "table %q has a composite primary key (%s); composite keys are not routable (§V5.4)", ti.Key, strings.Join(pkNames, ", "))
		return "", false
	}
	cd := ti.Column(pkNames[0])
	if cd == nil {
		l.errorf(pos, "V5", "table %q primary-key column %q not found (§V5.4)", ti.Key, pkNames[0])
		return "", false
	}
	pk := cd.Col
	goType, known := golang.GoTypeName(strings.ToLower(pk.Type.Name.String()))
	if !known {
		l.errorf(pos, "V5", "primary-key type %q of table %q is not a routable type (§V5.4)", pk.Type.String(), ti.Key)
		return "", false
	}
	switch goType {
	case "int", "int32", "int64", "string":
		return ParamType(goType), true
	default:
		l.errorf(pos, "V5", "primary-key Go type %q of table %q cannot be a route parameter (§V5.4); routable: int, int32, int64, string", goType, ti.Key)
		return "", false
	}
}
