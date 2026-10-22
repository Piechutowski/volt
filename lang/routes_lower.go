package lang

import (
	"maps"
	"strings"
	"unicode"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/token"
	"github.com/Piechutowski/volt/nao/gen/golang"
	"github.com/Piechutowski/volt/nao/inflect"
)

type routeFacts struct {
	pkgName, pkgPath, module string
	imports                  map[string]string
}

func (f routeFacts) equal(g routeFacts) bool {
	return f.pkgName == g.pkgName && f.pkgPath == g.pkgPath && f.module == g.module && maps.Equal(f.imports, g.imports)
}

type routeEnv interface {
	dataPackage(path string) (pkgName string, hasSchema, hasSelects bool)
	tableByName(path, base string) (ti *check.TableInfo, hint string)
	modelOf(path, key string) (model any, methods []golang.CRUDMethod, err error)
	crudByName(path, name string) (key string, model any, cm golang.CRUDMethod, ok bool)
	selectByMethod(path, name string) (sel *SelectInfo, member *check.TableInfo, ok bool)
	selectByName(path, name string) (si *SelectInfo, hint string)
	queryHint(path, name string) string
}

type lookupKind uint8

const (
	lkDataPackage lookupKind = iota
	lkTableByName
	lkModelOf
	lkCrudByName
	lkSelectByMethod
	lkSelectByName
	lkQueryHint
)

type routeLookup struct {
	kind       lookupKind
	path, name string

	pkgName, key, hint, err   string
	hasSchema, hasSelects, ok bool
	ti, member                *check.TableInfo
	sel                       *SelectInfo
	model                     any
}

func (l routeLookup) holds(env routeEnv) bool {
	switch l.kind {
	case lkDataPackage:
		n, s, sel := env.dataPackage(l.path)
		return n == l.pkgName && s == l.hasSchema && sel == l.hasSelects
	case lkTableByName:
		ti, hint := env.tableByName(l.path, l.name)
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

type routeLowering struct {
	facts  routeFacts
	env    routeEnv
	record bool

	lookups []routeLookup
	used    []string
	diags   []diag.Diagnostic
}

func (l *routeLowering) errorf(pos token.Position, section, format string, args ...any) {
	l.diags = append(l.diags, diag.Errorf(pos, "spec/"+section, format, args...))
}

func (l *routeLowering) note(lookup routeLookup) {
	if l.record {
		l.lookups = append(l.lookups, lookup)
	}
}

func (l *routeLowering) qualify(qual string) (string, qualKind) {
	path, kind := qualifierResolve(qual, l.facts.pkgName, l.facts.imports)
	if kind == qualImport {
		l.used = append(l.used, qual)
	}
	return path, kind
}

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

func (l *routeLowering) tableByName(path, base string) (*check.TableInfo, string) {
	ti, hint := l.env.tableByName(path, base)
	l.note(routeLookup{kind: lkTableByName, path: path, name: base, ti: ti, hint: hint})
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

func (l *routeLowering) queryHint(path, name string) string {
	hint := l.env.queryHint(path, name)
	l.note(routeLookup{kind: lkQueryHint, path: path, name: name, hint: hint})
	return hint
}

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
		r.shape = shapeParse(r)
		r.shapeKey = r.Method + " " + shapeOf(r)
	}
	return routes, l.lookups, l.used, l.diags
}

func (l *routeLowering) dataset(ds *ast.Dataset, inh inherited) []*RouteInfo {
	qual, target, local := l.facts.pkgName, l.facts.pkgPath, true
	qualPos := ds.Name.Pos()
	if ds.Pkg != nil {
		qual, qualPos, local = ds.Pkg.Name(), ds.Pkg.Pos(), false
		var kind qualKind
		switch target, kind = l.qualify(qual); kind {
		case qualImport:
		case qualSelf:
			l.diags = append(l.diags, selfQualified(ds.Pkg.Pos(), qual, ds.Name.Name()))
			return nil
		default:
			l.errorf(ds.Pkg.Pos(), "datasets", "unknown package qualifier %q (§dataset-declaration)", qual)
			return nil
		}
	}
	_, _, hasSelects := l.dataPackage(target)
	if local && !hasSelects {
		l.errorf(ds.Name.Pos(), "datasets", "package %q declares no select; a dataset names a select of this package, or of an imported data package, qualified (§dataset-declaration)", target)
		return nil
	}
	si, hint := l.selectByName(target, ds.Name.Name())
	if si == nil {
		l.errorf(ds.Name.Pos(), "datasets", "no select %q in package %q%s (§dataset-declaration)", ds.Name.Name(), target, hint)
		return nil
	}

	strip := ""
	var only, except map[string]bool
	var onlyPos, exceptPos token.Position
	members := map[string]bool{}
	for _, m := range si.Members {
		members[m.Decl.Name.Name()] = true
	}
	if ds.Settings != nil {
		for _, s := range ds.Settings.Settings {
			switch s.Name {
			case "strip":
				lit, ok := s.Value.(*ast.BasicLit)
				if !ok || lit.Tok.Kind != token.STRING {
					l.errorf(s.Pos(), "datasets", "strip: takes a string, the table-name prefix to drop from the URL segment (§dataset-settings)")
					continue
				}
				strip = lit.Tok.Val
			case "only", "except":
				list, ok := s.Value.(*ast.IdentList)
				if !ok {
					l.errorf(s.Pos(), "datasets", "%s: takes a table list like (da_a_a, da_b_b) (§dataset-settings)", s.Name)
					continue
				}
				set := map[string]bool{}
				for i, id := range list.Names {
					if list.Mods[i] != nil {
						l.errorf(list.Mods[i].Pos(), "datasets", "%s: entries are table names; %q does not belong after %q (§dataset-settings)", s.Name, list.Mods[i].Name(), id.Name())
					}
					if !members[id.Name()] {
						l.errorf(id.Pos(), "datasets", "%s: %q is not a member of select %q's target (§dataset-settings)", s.Name, id.Name(), si.Decl.Name.Name())
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
				l.errorf(s.Pos(), "settings-whitelists", "setting %q is not valid on a dataset (§settings-whitelists); valid: strip, only, except", s.Name)
			}
		}
	}
	if only != nil && except != nil {
		pos := onlyPos
		if exceptPos.Line() > onlyPos.Line() || (exceptPos.Line() == onlyPos.Line() && exceptPos.Column() > onlyPos.Column()) {
			pos = exceptPos
		}
		l.errorf(pos, "datasets", "only: and except: cannot both be set on a dataset (§dataset-settings)")
		return nil
	}

	var out []*RouteInfo
	for _, m := range si.Members {
		base := m.Decl.Name.Name()
		if only != nil && !only[base] {
			continue
		}
		if except[base] {
			continue
		}
		seg := base
		if strip != "" {
			if !strings.HasPrefix(base, strip) {
				l.errorf(ds.Pos(), "datasets", "strip: %q is not a prefix of member table %q (§dataset-settings)", strip, base)
				continue
			}
			seg = strings.TrimPrefix(base, strip)
			if seg == "" || !goIdentOK(seg) {
				l.errorf(ds.Pos(), "datasets", "strip: %q leaves member table %q with no usable segment (§dataset-settings)", strip, base)
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

func (l *routeLowering) route(r *ast.Route, inh inherited) *RouteInfo {
	method := methodOf(r.Verb())
	if r.Verb() == "put" {
		l.errorf(r.Pos(), "routes", "put is not a route verb: the partial update is patch, so write patch %s %s (§routes)", r.Path.String(), r.Handler.String())
		method = "PATCH"
	}
	l.diags = append(l.diags, stringTypedErrors(r.Path.Segments)...)
	segs := append(append([]*ast.Segment{}, inh.prefix...), r.Path.Segments...)
	params, ok := l.pathParams(segs)
	if !ok {
		return nil
	}

	var query *QueryRef
	events := false
	controller, action := "", r.Handler.Parts[0].Name()
	if len(r.Handler.Parts) == 2 {
		controller, action = action, r.Handler.Parts[1].Name()
	}
	target, kind := "", qualUnknown
	if controller != "" {
		target, kind = l.qualify(controller)
	}
	switch {
	case kind == qualRuntime:
		if action != "Events" {
			l.errorf(r.Handler.Pos(), "scopes-and-routes", "the runtime provides no handler volt.%s; volt.Events is the event stream (§event-routes)", action)
			return nil
		}
		if method != "GET" {
			l.errorf(r.Pos(), "scopes-and-routes", "an event route is read with get (§event-routes)")
			return nil
		}
		events = true
	case controller == "":
		query = l.queryRef(r, method, params, l.facts.pkgName, l.facts.pkgPath, true)
		if query == nil {
			return nil
		}
		controller, action = "", query.Method
	case kind == qualSelf:
		l.diags = append(l.diags, selfQualified(r.Handler.Pos(), controller, action))
		return nil
	default:
		if kind == qualImport {
			query = l.queryRef(r, method, params, controller, target, false)
			if query == nil {
				return nil
			}
			controller, action = "", query.Method
		} else if !exportedIdentOK(controller) || !exportedIdentOK(action) {
			l.errorf(r.Handler.Pos(), "scopes-and-routes", "handler must be Controller.Action, both exported Go identifiers (§handler-references), found %q", r.Handler.String())
			return nil
		}
	}

	helper := inh.namePrefix + action
	client := ""
	if s := settingOf(r.Settings, "name"); s != nil {
		id, ok := s.Value.(*ast.Ident)
		if !ok {
			l.errorf(s.Pos(), "scopes-and-routes", "name: takes an identifier (§route-names-and-reverse-urls)")
		} else if n, err := golang.GoName(id.Name()); err != nil {
			l.errorf(s.Pos(), "scopes-and-routes", "route name %q: %v (§route-names-and-reverse-urls)", id.Name(), err)
		} else {
			helper = inh.namePrefix + n
		}
	}
	if r.Settings != nil {
		for _, s := range r.Settings.Settings {
			if s.Name != "name" {
				l.errorf(s.Pos(), "settings-whitelists", "setting %q is not valid on a route (§settings-whitelists); valid: name", s.Name)
			}
		}
	}

	if query != nil {
		action = ""
		client = helper
		if method != "GET" {
			helper = ""
		}
	}
	if events {
		client = helper
	}
	if r.Verb() == "put" {
		return nil
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

var queryValueTypes = map[string]bool{
	"string": true, "bool": true, "int": true, "float64": true, "time.Time": true,
}

func (l *routeLowering) queryRef(r *ast.Route, method string, params []Param, qual, target string, local bool) *QueryRef {
	last := r.Handler.Parts[len(r.Handler.Parts)-1]
	return l.queryBind(last.Pos(), r.Handler.Parts[0].Pos(), method, params, qual, target, local, last.Name(), paramAt{route: r})
}

func (l *routeLowering) queryBind(pos, qualPos token.Position, method string, params []Param, qual, target string, local bool, name string, segAt paramAt) *QueryRef {
	pkgName, hasSchema, _ := l.dataPackage(target)
	ref := qual + "." + name
	if local {
		ref = name
	}
	if !hasSchema {
		if local {
			l.errorf(pos, "scopes-and-routes", "package %q declares no tables, so it has no query %s; a query of an imported data package is written qualified (§query-routes)", target, name)
		} else {
			l.errorf(pos, "scopes-and-routes", "package %q declares no tables; a query route needs a data package (§query-routes)", target)
		}
		return nil
	}
	field := "Queries"
	if !local {
		var err error
		if field, err = golang.GoName(qual); err != nil {
			l.errorf(qualPos, "scopes-and-routes", "import qualifier %q: %v (§query-routes)", qual, err)
			return nil
		}
	}
	importPath := l.facts.module
	if target != "." {
		importPath += "/" + target
	}
	qr := &QueryRef{Qualifier: qual, Field: field, Package: target, Import: importPath, PkgName: pkgName, Local: local, Method: name, Status: 200}

	type sigParam struct {
		name, goType string
		body, key    bool
	}
	var sig []sigParam
	found, op := false, ""
	var dropped map[string]bool

	if si, m, ok := l.selectByMethod(target, name); ok {
		found, op, dropped = true, "select", si.droppedParams
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
	if !found {
		if _, _, cm, ok := l.crudByName(target, name); ok {
			found, op = true, cm.Op
			for _, k := range cm.Key {
				sig = append(sig, sigParam{name: k.GoName, goType: k.GoType, key: cm.Op == "update"})
			}
			if cm.Body != "" {
				sig = append(sig, sigParam{name: "cs", goType: qual + "." + cm.Body, body: true})
				qr.Changeset = &ChangesetBind{Params: cm.Params, New: cm.New, Get: cm.Get, Allow: cm.Allow}
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
		hint := l.queryHint(target, name)
		l.errorf(pos, "scopes-and-routes", "no generated query %s in package %q%s — a query route names a select method or a default CRUD method (Get, List, Create, Update, Delete) (§query-routes)", ref, target, hint)
		return nil
	}

	byName := map[string]Param{}
	for _, p := range params {
		byName[p.Name] = p
	}
	bound := map[string]bool{}
	ok := true
	if want := queryMethods[op]; method != want {
		at, written := pos, strings.ToLower(methodOrAny(method))
		if segAt.route != nil {
			at, written = segAt.route.Pos(), segAt.route.Verb()
		}
		why := "the generated method a query route binds fixes its verb"
		if method == "HEAD" {
			why += ", and ServeMux answers HEAD on a get route"
		}
		l.errorf(at, "query-routes", "%s is routed with %s, not %s: %s (§query-routes)", ref, strings.ToLower(want), written, why)
		ok = false
	}
	for _, sp := range sig {
		if sp.goType == "" {
			if pp, inPath := byName[sp.name]; inPath {
				bound[sp.name] = true
				if pp.Wild {
					l.errorf(segAt.pos(pp.Name), "scopes-and-routes", "parameter %q of %s cannot be a wildcard (§query-routes)", sp.name, ref)
					ok = false
				}
			} else if sp.key {
				l.errorf(pos, "query-routes", "%s loads the row to update by %q, which must be a path parameter of the route (§query-routes)", ref, sp.name)
				ok = false
			}
			qr.Unresolved = true
			continue
		}
		qp := QueryParam{Name: sp.name, GoType: sp.goType}
		switch {
		case sp.body:
			qp.Source = FromBody
		case strings.HasPrefix(sp.goType, "[]"):
			qp.Source = FromList
			if pp, inPath := byName[sp.name]; inPath {
				l.errorf(segAt.pos(pp.Name), "scopes-and-routes", "list parameter %q of %s cannot be a path parameter; pass it as a repeated query key (§query-routes)", sp.name, ref)
				ok = false
			}
			if !queryValueTypes[strings.TrimPrefix(sp.goType, "[]")] {
				l.errorf(pos, "query-routes", "list parameter %q of %s is %s, which the query string cannot carry (§query-routes)", sp.name, ref, sp.goType)
				ok = false
			}
		default:
			if pp, inPath := byName[sp.name]; inPath {
				qp.Source = FromPath
				bound[sp.name] = true
				if pp.Wild {
					l.errorf(segAt.pos(pp.Name), "scopes-and-routes", "parameter %q of %s cannot be a wildcard (§query-routes)", sp.name, ref)
					ok = false
				} else if pp.Type.GoType() != sp.goType {
					if KnownParamType(sp.goType) {
						l.errorf(segAt.pos(pp.Name), "scopes-and-routes", "path parameter %q is %s but %s takes %s; spell it %s (§query-routes)", sp.name, pp.Type.GoType(), ref, sp.goType, paramSpelled(sp.name, ParamType(sp.goType)))
					} else {
						l.errorf(segAt.pos(pp.Name), "scopes-and-routes", "parameter %q of %s is %s, which a path segment cannot carry (§route-paths); pass it in the query string (§query-routes)", sp.name, ref, sp.goType)
					}
					ok = false
				}
			} else if sp.key {
				spell := ""
				if KnownParamType(sp.goType) {
					spell = "; spell it " + paramSpelled(sp.name, ParamType(sp.goType))
				}
				l.errorf(pos, "query-routes", "%s loads the row to update by %q, which must be a path parameter of the route%s (§query-routes)", ref, sp.name, spell)
				ok = false
			} else {
				qp.Source = FromQuery
				if !queryValueTypes[sp.goType] {
					l.errorf(pos, "query-routes", "parameter %q of %s is %s, which the query string cannot carry (§query-routes)", sp.name, ref, sp.goType)
					ok = false
				}
			}
		}
		qr.Params = append(qr.Params, qp)
	}
	for _, p := range params {
		if !bound[p.Name] {
			if dropped[p.Name] {
				continue
			}
			l.errorf(segAt.pos(p.Name), "scopes-and-routes", "path parameter %q is not a parameter of %s (§query-routes)", p.Name, ref)
			ok = false
		}
	}
	if !ok {
		return nil
	}
	return qr
}

func litSeg(n string, at token.Position) *ast.Segment {
	return &ast.Segment{Kind: ast.SegLit, Name: &ast.Ident{Tok: token.Token{Kind: token.IDENT, Val: n, Pos: at}}}
}

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

func nameStartReported(name string) bool {
	for _, r := range name {
		return unicode.IsDigit(r) || unicode.IsMark(r)
	}
	return false
}

func (l *routeLowering) pathParams(segs []*ast.Segment) ([]Param, bool) {
	var params []Param
	seen := map[string]bool{}
	ok := true
	for i, seg := range segs {
		if !goIdentOK(seg.Name.Name()) {
			if !nameStartReported(seg.Name.Name()) {
				l.errorf(seg.Pos(), "scopes-and-routes", "path segment %q must be a plain identifier of letters, digits and underscores (§route-paths)", seg.Name.Name())
			}
			ok = false
			continue
		}
		switch seg.Kind {
		case ast.SegLit:
			continue
		case ast.SegWild:
			if i != len(segs)-1 {
				l.errorf(seg.Pos(), "scopes-and-routes", "the wildcard ':%s...' must be the last path segment (§route-paths)", seg.Name.Name())
				ok = false
			}
		}
		name := seg.Name.Name()
		if seen[name] {
			l.errorf(seg.Pos(), "scopes-and-routes", "duplicate path parameter %q (§route-paths)", name)
			ok = false
			continue
		}
		seen[name] = true
		if !goIdentOK(name) || goKeywords[name] {
			l.errorf(seg.Pos(), "scopes-and-routes", "path parameter %q must be a valid, non-keyword Go identifier (§route-paths)", name)
			ok = false
			continue
		}
		if reservedParamNames[name] {
			l.errorf(seg.Pos(), "scopes-and-routes", "path parameter %q collides with a name in generated signatures (§route-paths); reserved: w, r, opts, volt, cs", name)
			ok = false
			continue
		}
		pt := TString
		if seg.Type != nil && !stringTyped(seg) {
			if seg.Type.Name() != string(TInt) {
				l.errorf(seg.Type.Pos(), "scopes-and-routes", "unknown parameter type %q (§route-paths); valid: int", seg.Type.Name())
				ok = false
				continue
			}
			pt = TInt
		}
		params = append(params, Param{Name: name, GoName: name, Type: pt, Wild: seg.Kind == ast.SegWild})
	}
	return params, ok
}

func paramSpelled(name string, t ParamType) string {
	if t == TInt {
		return ":" + name + "(int)"
	}
	return ":" + name
}

func stringTyped(seg *ast.Segment) bool {
	return seg.Kind == ast.SegParam && seg.Type != nil && strings.EqualFold(seg.Type.Name(), string(TString))
}

func stringTypedErrors(segs []*ast.Segment) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, seg := range segs {
		if stringTyped(seg) {
			n := seg.Name.Name()
			out = append(out, diag.Errorf(seg.Type.Pos(), "spec/route-paths", "write :%s, not :%s(%s): a bare path parameter is a string, and int is the one type written (§route-paths)", n, n, seg.Type.Name()))
		}
	}
	return out
}

func (l *routeLowering) resources(res *ast.Resources, inh inherited) []*RouteInfo {
	declared := res.Name.Name()
	if !goIdentOK(declared) || (res.Pkg != nil && !goIdentOK(res.Pkg.Name())) {
		l.errorf(res.Name.Pos(), "resources", "resources name %q must be a plain identifier (§route-paths)", res.Ref())
		return nil
	}

	paramName := "id"

	def := false
	if res.Settings != nil {
		if s := res.Settings.Get("default"); s != nil {
			def = true
			if s.Value != nil {
				l.errorf(s.Pos(), "resources", "default is a flag and takes no value (§default-handlers)")
			}
		}
	}

	ti, ok, reported := l.resourceTable(res)
	if reported {
		return nil
	}
	if !ok {
		return nil
	}
	tableName, singular := ti.Decl.Name.Name(), ""
	if model, err := golang.ModelName(ti.Decl); err == nil {
		singular = model
	}
	keyType, ok := l.pkParamType(ti, res.Name.Pos())
	if !ok {
		return nil
	}

	controller, err := golang.GoName(tableName)
	if err != nil {
		l.errorf(res.Name.Pos(), "resources", "resources name %q: %v (§declaration)", declared, err)
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
					l.errorf(s.Pos(), "resources", "api is a flag and takes no value (§settings)")
				}
				api = true
			case "default":
				api = true
			case "only", "except":
				list, ok := s.Value.(*ast.IdentList)
				if !ok {
					l.errorf(s.Pos(), "resources", "%s: takes an action list like (index, show) (§settings)", s.Name)
					continue
				}
				for i, mod := range list.Mods {
					if mod != nil {
						l.errorf(mod.Pos(), "resources", "%s: entries are single action names; %q does not belong after %q (§settings)", s.Name, mod.Name(), list.Names[i].Name())
					}
				}
				set := map[string]bool{}
				for _, id := range list.Names {
					canon, known := actionByLower[id.Name()]
					if !known {
						l.errorf(id.Pos(), "resources", "unknown action %q (§the-action-table); valid: index, new, create, show, edit, update, delete", id.Name())
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
				if !ok || !goIdentOK(id.Name()) {
					l.errorf(s.Pos(), "resources", "singular: takes a plain identifier (§settings)")
					continue
				}
				gn, err := golang.GoName(id.Name())
				if err != nil {
					l.errorf(s.Pos(), "resources", "singular %q: %v (§settings)", id.Name(), err)
					continue
				}
				singular = gn
			case "param":
				if def {
					l.errorf(s.Pos(), "resources", "param: does not apply with [default]; the key parameter is named by the primary-key column, as the generated CRUD spells it (§default-handlers)")
					continue
				}
				id, ok := s.Value.(*ast.Ident)
				if !ok || !goIdentOK(id.Name()) || goKeywords[id.Name()] || reservedParamNames[id.Name()] {
					l.errorf(s.Pos(), "resources", "param: takes a valid, non-keyword, non-reserved Go identifier (§settings, §route-paths)")
					continue
				}
				paramName = id.Name()
			case "model":
				l.errorf(s.Pos(), "resources", "model: is not a setting; name the table in the declaration itself — `resources <table>` (§declaration)")
			default:
				l.errorf(s.Pos(), "settings-whitelists", "setting %q is not valid on resources (§settings-whitelists); valid: api, only, except, param, singular, default", s.Name)
			}
		}
	}
	if only != nil && except != nil {
		l.errorf(exceptPos, "resources", "only: and except: cannot be combined (§settings)")
		_ = onlyPos
		except = nil
	}

	var crud map[string]golang.CRUDMethod
	qual, target, local := l.facts.pkgName, l.facts.pkgPath, true
	qualPos := res.Name.Pos()
	if res.Pkg != nil {
		qual, local = res.Pkg.Name(), false
		target, _ = l.importOf(res.Pkg.Name())
		qualPos = res.Pkg.Pos()
	}
	if def {
		_, methods, err := l.modelOf(target, ti.Key)
		if err != nil {
			l.errorf(res.Name.Pos(), "resources", "resources %s [default]: %v (§default-handlers)", res.Ref(), err)
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

	for _, seg := range inh.prefix {
		if seg.Kind != ast.SegParam || seg.Name.Name() != paramName {
			continue
		}
		if def {
			l.errorf(res.Name.Pos(), "resources", "resources %s [default]: the key parameter %q is already a parameter of the enclosing scope, and the generated CRUD fixes its name; give the scope's parameter another name (§default-handlers, §route-paths)", res.Ref(), paramName)
		} else {
			l.errorf(res.Name.Pos(), "resources", "resources %q: the key parameter %q is already a parameter of the enclosing scope; rename it with [param: <name>] (§settings, §route-paths)", declared, paramName)
		}
		return nil
	}

	nameSeg := func(n string) *ast.Segment { return litSeg(n, res.Pos()) }
	idSeg := &ast.Segment{Kind: ast.SegParam, MarkPos: res.Pos(),
		Name: &ast.Ident{Tok: token.Token{Kind: token.IDENT, Val: paramName, Pos: res.Pos()}}}
	if keyType == TInt {
		idSeg.Type = &ast.Ident{Tok: token.Token{Kind: token.IDENT, Val: string(keyType), Pos: res.Pos()}}
	}

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
		for i := range params {
			if keyType == tUnresolved && a.OnID && params[i].Name == paramName {
				params[i].Type = tUnresolved
			}
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
		var qr *QueryRef
		client := ""
		if def {
			cm, has := crud[a.Op]
			if !has {
				l.errorf(res.Name.Pos(), "resources", "resources %s [default]: the generated CRUD has no %s method for %q (%s); drop the action with except: (%s) (§default-handlers)",
					res.Ref(), a.Op, a.Name, crudAbsent(a.Op), strings.ToLower(a.Name))
				continue
			}
			qr = l.queryBind(res.Name.Pos(), qualPos, a.Method, params, qual, target, local, cm.Name, paramAt{at: res.Pos()})
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
		r := &RouteInfo{
			Method:       a.Method,
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
		out = append(out, r)
	}
	return out
}

func crudAbsent(op string) string {
	if op == "update" {
		return "every column is part of the key, so there is nothing to update"
	}
	return "the table has no primary key"
}

func (l *routeLowering) resourceTable(res *ast.Resources) (ti *check.TableInfo, ok, reported bool) {
	pkgPath := l.facts.pkgPath
	if res.Pkg != nil {
		qual := res.Pkg.Name()
		target, kind := l.qualify(qual)
		switch kind {
		case qualImport:
		case qualSelf:
			l.diags = append(l.diags, selfQualified(res.Pkg.Pos(), qual, res.Name.Name()))
			return nil, false, true
		default:
			l.errorf(res.Pkg.Pos(), "resources", "unknown package qualifier %q (§declaration)", qual)
			return nil, false, true
		}
		pkgPath = target
	}
	if _, hasSchema, _ := l.dataPackage(pkgPath); !hasSchema {
		l.errorf(res.Name.Pos(), "resources", "package %q declares no tables; resources names a table of this package, or of an imported data package, qualified (§declaration)", pkgPath)
		return nil, false, true
	}
	want := res.Name.Name()
	if cand, hint := l.tableByName(pkgPath, want); cand != nil {
		return cand, true, false
	} else {
		l.errorf(res.Name.Pos(), "resources", "no table %q in package %q%s (§declaration)", want, pkgPath, hint)
	}
	return nil, false, true
}

func (l *routeLowering) pkParamType(ti *check.TableInfo, pos token.Position) (ParamType, bool) {
	var pkNames []string
	for _, cd := range ti.PK() {
		pkNames = append(pkNames, cd.Col.Name.Name())
	}
	switch {
	case len(pkNames) == 0:
		l.errorf(pos, "resources", "table %q has no single-column primary key (§table-resolution)", ti.Key)
		return "", false
	case len(pkNames) > 1:
		l.errorf(pos, "resources", "table %q has a composite primary key (%s); composite keys are not routable (§table-resolution)", ti.Key, strings.Join(pkNames, ", "))
		return "", false
	}
	cd := ti.Column(pkNames[0])
	if cd == nil {
		l.errorf(pos, "resources", "table %q primary-key column %q not found (§table-resolution)", ti.Key, pkNames[0])
		return "", false
	}
	pk := cd.Col
	if !cd.Type.Known() {
		return tUnresolved, true
	}
	switch goType := cd.Type.Go; goType {
	case "int", "string":
		return ParamType(goType), true
	default:
		l.errorf(pos, "resources", "primary-key type %q of table %q cannot be a route parameter (§table-resolution); routable: int, string", pk.Type.String(), ti.Key)
		return "", false
	}
}
