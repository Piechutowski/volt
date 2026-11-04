package lang

import (
	"strconv"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/token"
	"github.com/Piechutowski/volt/nao/naming"
)

// loads lowers the load block of a controller route (spec §route-loads):
// every load names a read of a data package and binds its arguments from
// the route's parameters, the query string or literals. It reports every
// fault and answers ok only when all of them lowered.
func scopeLoadNamed(loads []ScopeLoad, name string) bool {
	for _, ld := range loads {
		if ld.Name == name {
			return true
		}
	}
	return false
}

func (l *routeLowering) loads(r *ast.Route, params []Param, inh inherited) (loads []LoadInfo, ok bool) {
	ok = true
	byName := map[string]Param{}
	for _, p := range params {
		byName[p.Name] = p
	}
	seen := map[string]token.Position{}
	keys := map[string]keyBinding{}
	values := map[string]string{}
	for _, ld := range r.Loads {
		values[ld.Name.Name()] = "a load of the route"
	}
	for _, sl := range inh.loads {
		values[sl.Name] = "a scope load above the route"
	}
	for _, ld := range r.Loads {
		name := ld.Name.Name()
		switch {
		case !loadNameOK(name):
			l.errorf(ld.Name.Pos(), "route-loads", "load %q is not a name a handler parameter can take: a lower-case Go identifier and no Go keyword, that is not reserved, not err or ok, not a name the route files import a package under (context, http, rt, run, time, volt), not a predeclared Go identifier (string, int, len, nil and the rest), and not shaped like a generated local (§route-loads, §route-paths)", name)
			ok = false
			continue
		case loadNameQualifier(name, l.facts.pkgName, l.facts.imports):
			l.errorf(ld.Name.Pos(), "route-loads", "load %q takes the name of a package the generated code refers to (§route-loads, §imports)", name)
			ok = false
			continue
		case byName[name].Name != "":
			l.errorf(ld.Name.Pos(), "route-loads", "load %q takes the name of a path parameter of the route (§route-loads)", name)
			ok = false
			continue
		case scopeLoadNamed(inh.loads, name):
			l.errorf(ld.Name.Pos(), "route-loads", "load %q takes the name of a scope load above the route (§route-loads, §scope-loads)", name)
			ok = false
			continue
		}
		if prev, dup := seen[name]; dup {
			l.errorf(ld.Name.Pos(), "route-loads", "load %q is declared twice (first at %s) (§route-loads)", name, prev)
			ok = false
			continue
		}
		seen[name] = ld.Name.Pos()
		q := l.callQuery(name, ld.Call, byName, values)
		if q == nil {
			ok = false
			continue
		}
		if !l.loadKeysAgree(ld, q, keys) {
			ok = false
			continue
		}
		loads = append(loads, LoadInfo{Name: name, Query: q})
	}
	return loads, ok
}

// loadKeysAgree holds the query-string keys the loads of a route bind to
// one Go type each (spec §route-loads, §generated-client): the client
// method takes a key once, so a key two loads bind as two types, or as a
// list one load requires and another takes optional, which no one
// argument of the method can serve, is an error.
func (l *routeLowering) loadKeysAgree(ld *ast.Load, q *QueryRef, keys map[string]keyBinding) bool {
	for _, p := range q.Params {
		if p.Source != FromQuery && p.Source != FromList {
			continue
		}
		first, seen := keys[p.Name]
		if !seen {
			keys[p.Name] = keyBinding{p, ld.Name.Name()}
			continue
		}
		switch {
		case first.p.GoType != p.GoType:
			l.errorf(ld.Call.Pos(), "route-loads", "load %s binds the query key %q as %s, and load %s binds it as %s; one key binds one type, the one the client sends it as (§route-loads, §generated-client)", ld.Name.Name(), p.Name, p.Type(), first.load, first.p.Type())
			return false
		case p.Source == FromList && first.p.Optional != p.Optional:
			l.errorf(ld.Call.Pos(), "route-loads", "load %s binds the list key %q %s, and load %s binds it %s; a missing key is an empty list to one and absent to the other, which no one argument of the client tells apart (§route-loads, §generated-client)", ld.Name.Name(), p.Name, optionalWord(p.Optional), first.load, optionalWord(first.p.Optional))
			return false
		}
	}
	return true
}

type keyBinding struct {
	p    QueryParam
	load string
}

func optionalWord(optional bool) string {
	if optional {
		return "optional"
	}
	return "required"
}

// loadNameOK reports whether a load may be named so (spec §route-loads,
// §scope-loads): a lower-case Go identifier, so no Go keyword, that is
// neither reserved, nor err or ok, which the shim declares beside it,
// nor a name the route files import a package under or a predeclared Go
// identifier, which the shim's closures would shadow, nor shaped like a
// generated local.
func loadNameOK(name string) bool {
	if naming.RouteSignatureName(name) || name == "_" || name == "err" || name == "ok" || goKeywords[name] || generatedImportNames[name] != "" || goPredeclared[name] || naming.LocalName(name) || !exportedIdentOK(strings.ToUpper(name[:1])+name[1:]) {
		return false
	}
	return name[0] >= 'a' && name[0] <= 'z'
}

// goPredeclared are the identifiers of Go's universe block, which a
// generated closure parameter would shadow (spec §route-loads).
var goPredeclared = map[string]bool{
	"any": true, "bool": true, "byte": true, "comparable": true, "complex64": true, "complex128": true,
	"error": true, "float32": true, "float64": true, "int": true, "int8": true, "int16": true,
	"int32": true, "int64": true, "rune": true, "string": true, "uint": true, "uint8": true,
	"uint16": true, "uint32": true, "uint64": true, "uintptr": true,
	"true": true, "false": true, "iota": true, "nil": true,
	"append": true, "cap": true, "clear": true, "close": true, "complex": true, "copy": true,
	"delete": true, "imag": true, "len": true, "make": true, "max": true, "min": true,
	"new": true, "panic": true, "print": true, "println": true, "real": true, "recover": true,
}

// loadNameQualifier reports whether name is the package's own name or
// one of its import qualifiers, which a load cannot shadow.
func loadNameQualifier(name, pkgName string, imports map[string]string) bool {
	if name == pkgName {
		return true
	}
	_, ok := imports[name]
	return ok
}

func (l *routeLowering) callQuery(what string, call *ast.QueryCall, byName map[string]Param, values map[string]string) *QueryRef {
	parts := call.Ref.Parts
	last := parts[len(parts)-1]
	name := last.Name()
	qual, target, local := l.facts.pkgName, l.facts.pkgPath, true
	if len(parts) == 2 {
		qual = parts[0].Name()
		t, kind := l.qualify(qual)
		switch kind {
		case qualImport:
			target, local = t, false
		case qualSelf:
			l.diags = append(l.diags, selfQualified(call.Ref.Pos(), qual, name))
			return nil
		default:
			l.errorf(call.Ref.Pos(), "route-loads", "load %s names %q, which is not an import of this package (§route-loads, §imports)", what, qual)
			return nil
		}
	}
	pkgName, hasSchema, _ := l.dataPackage(target)
	ref := qual + "." + name
	if local {
		ref = name
	}
	if !hasSchema {
		l.errorf(call.Ref.Pos(), "route-loads", "package %q declares no tables, so it has no query %s to load (§route-loads)", target, name)
		return nil
	}
	importPath := l.facts.module
	if target != "." {
		importPath += "/" + target
	}
	qr := &QueryRef{Qualifier: qual, Package: target, Import: importPath, PkgName: pkgName, Local: local, Method: name, Status: 200}

	type sigParam struct {
		name, goType     string
		limits, optional bool
	}
	var sig []sigParam
	found := false
	if si, m, ok := l.selectByMethod(target, name); ok {
		found = true
		for _, p := range si.Params {
			sig = append(sig, sigParam{p.GoName, p.GoType, p.GoName == si.LimitParam, p.Optional})
		}
		switch {
		case si.With != nil:
			qr.Result = si.With.RowType
		case si.Shared != "":
			qr.Result = si.Shared
		case len(si.Excluded) > 0:
			qr.Result = name + "Row"
		default:
			qr.Result = modelOrBase(m)
		}
		qr.Many = !si.Lookup
		if si.Unresolved {
			qr.Unresolved = true
		}
	} else if _, _, cm, ok := l.crudByName(target, name); ok {
		found = true
		switch cm.Op {
		case "get", "list":
		default:
			l.errorf(call.Ref.Pos(), "route-loads", "%s writes; a load reads: a select function, Get or List (§route-loads)", ref)
			return nil
		}
		for _, k := range cm.Key {
			sig = append(sig, sigParam{name: k.GoName, goType: k.GoType})
		}
		qr.Result, qr.Many = cm.Result, cm.Many
	}
	if !found {
		l.errorf(call.Ref.Pos(), "route-loads", "no generated query %s in package %q%s; a load names a select function, Get or List (§route-loads)", ref, target, l.queryHint(target, name))
		return nil
	}
	if len(call.Args) != len(sig) {
		l.errorf(call.Ref.Pos(), "route-loads", "%s takes %d argument(s), the load gives %d (§route-loads)", ref, len(sig), len(call.Args))
		return nil
	}
	ok := true
	for i, arg := range call.Args {
		sp := sig[i]
		if a, isName := arg.(*ast.Ident); isName && byName[a.Name()].Name == "" && a.Name() != what {
			if kind := values[a.Name()]; kind != "" {
				l.errorf(a.Pos(), "route-loads", "argument %d of %s names %q, %s, whose value no load binds: a name that is no path parameter reads the query string by that name, so the client would choose it; name the key otherwise, or load in the handler with Then (§route-loads)", i+1, ref, a.Name(), kind)
				ok = false
				continue
			}
		}
		qp := QueryParam{Name: sp.name, GoType: sp.goType, Limit: sp.limits}
		if sp.goType == "" {
			qr.Unresolved = true
			qr.Params = append(qr.Params, qp)
			continue
		}
		list := strings.HasPrefix(sp.goType, "[]")
		qp.Optional = sp.optional
		if !queryValueTypes[strings.TrimPrefix(sp.goType, "[]")] {
			l.errorf(arg.Pos(), "route-loads", "parameter %q of %s is %s, which no load binds: the query string cannot carry it, and neither a path parameter nor a literal spells it (§route-loads)", sp.name, ref, qp.Type())
			ok = false
			qr.Params = append(qr.Params, qp)
			continue
		}
		switch a := arg.(type) {
		case *ast.Ident:
			if a.Name() == "true" || a.Name() == "false" {
				if sp.goType != "bool" {
					l.errorf(a.Pos(), "route-loads", "argument %d of %s is %s, not bool (§route-loads)", i+1, ref, sp.goType)
					ok = false
				}
				qp.Source, qp.Literal = FromLiteral, a.Name()
				break
			}
			if pp, inPath := byName[a.Name()]; inPath {
				switch {
				case list:
					l.errorf(a.Pos(), "route-loads", "list parameter %q of %s cannot come from a path parameter; name a query-string key (§route-loads)", sp.name, ref)
					ok = false
				case pp.Wild:
					l.errorf(a.Pos(), "route-loads", "parameter %q of %s cannot be the wildcard %q (§route-loads)", sp.name, ref, pp.Name)
					ok = false
				case pp.Type != tUnresolved && pp.Type.GoType() != sp.goType:
					l.errorf(a.Pos(), "route-loads", "path parameter %q is %s but %s takes %s (§route-loads)", pp.Name, pp.Type.GoType(), ref, sp.goType)
					ok = false
				}
				qp.Source, qp.Name = FromPath, pp.Name
				break
			}
			qp.Source, qp.Name = FromQuery, a.Name()
			if list {
				qp.Source = FromList
			}
		case *ast.BasicLit:
			qp.Source = FromLiteral
			switch {
			case a.Tok.Kind == token.STRING:
				if sp.goType != "string" {
					l.errorf(a.Pos(), "route-loads", "argument %d of %s is %s, not a string (§route-loads)", i+1, ref, sp.goType)
					ok = false
				}
				qp.Literal = strconv.Quote(a.Tok.Val)
			default:
				qp.Literal = numberLiteral(a.Tok.Val, sp.goType)
				if qp.Literal == "" {
					l.errorf(a.Pos(), "route-loads", "argument %d of %s is %s, which the number %s does not spell (§route-loads)", i+1, ref, sp.goType, a.Tok.Val)
					ok = false
				} else if sp.limits && !limitLiteralOK(qp.Literal) {
					l.errorf(a.Pos(), "route-loads", "argument %d of %s is its limit, at least 1, not %s, or every request would be refused (§route-loads, §selects-over-groups)", i+1, ref, a.Tok.Val)
					ok = false
				}
			}
		case *ast.NegNumber:
			qp.Source = FromLiteral
			qp.Literal = numberLiteral("-"+a.Num.Tok.Val, sp.goType)
			if qp.Literal == "" {
				l.errorf(a.Pos(), "route-loads", "argument %d of %s is %s, which the number -%s does not spell (§route-loads)", i+1, ref, sp.goType, a.Num.Tok.Val)
				ok = false
			} else if sp.limits {
				l.errorf(a.Pos(), "route-loads", "argument %d of %s is its limit, at least 1, not -%s, or every request would be refused (§route-loads, §selects-over-groups)", i+1, ref, a.Num.Tok.Val)
				ok = false
			}
		}
		qr.Params = append(qr.Params, qp)
	}
	if !ok {
		return nil
	}
	return qr
}

// limitLiteralOK reports whether an int literal a load binds to a
// select's limit: parameter is at least 1 (spec §route-loads).
func limitLiteralOK(lit string) bool {
	n, err := strconv.ParseInt(lit, 10, 64)
	return err == nil && n >= 1
}

// numberLiteral is the canonical Go spelling of lit as a value of goType
// (spec §route-loads, §predicates), "" when the number does not spell
// one: a decimal integer for int, so 010 is 10 and never Go's octal 8,
// and a float literal for float64.
func numberLiteral(lit, goType string) string {
	if goType != "int" && goType != "float64" {
		return ""
	}
	s, err := numberGo(lit, goType)
	if err != nil {
		return ""
	}
	return s
}

// limitSetting reads a route's or a dataset's default_limit: or
// max_limit: setting (spec §route-loads, §dataset-settings): a positive
// integer of rows that fits a 32-bit int, as the limit it governs is an
// int on every target.
func (l *routeLowering) limitSetting(s *ast.Setting, code, cite string) (int, bool) {
	lit, ok := s.Value.(*ast.BasicLit)
	if ok && lit.Tok.Kind == token.NUMBER {
		if n, err := strconv.ParseInt(lit.Tok.Val, 10, 32); err == nil && n > 0 {
			return int(n), true
		}
	}
	l.errorf(s.Pos(), code, "%s: takes a positive integer of rows that fits a 32-bit int, at most 2147483647, the limit it governs being an int (%s)", s.Name, cite)
	return 0, false
}

// timeout reads a route's timeout: setting (spec §route-loads): a
// positive integer of seconds.
func (l *routeLowering) timeout(s *ast.Setting) (int, bool) {
	lit, ok := s.Value.(*ast.BasicLit)
	if ok && lit.Tok.Kind == token.NUMBER {
		if n, err := strconv.Atoi(lit.Tok.Val); err == nil && n > 0 {
			return n, true
		}
	}
	l.errorf(s.Pos(), "route-loads", "timeout: takes a positive integer of seconds (§route-loads)")
	return 0, false
}

// loadsEqual reports whether two routes' loads give an action the same
// signature (§handler-references): the same names and Go types in order.
func loadsEqual(a, b []LoadInfo) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].GoType() != b[i].GoType() {
			return false
		}
	}
	return true
}

func loadsString(ls []LoadInfo) string {
	parts := make([]string, len(ls))
	for i, ld := range ls {
		parts[i] = ld.Name + " " + ld.GoType()
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// BodyKind says what a controller route's body: setting binds (spec
// §request-bodies): a changeset of a write query, or a struct of the
// routes package.
type BodyKind int

const (
	BodyChangeset BodyKind = iota
	BodyStruct
)

// BodyInfo is how a controller route binds its request body (spec
// §request-bodies): for BodyChangeset the write query whose changeset
// the handler receives, decoded, loaded, cast and validated by the shim,
// with Query.Changeset carrying the allow list; for BodyStruct the name
// of the struct type the body decodes into.
type BodyInfo struct {
	Kind   BodyKind
	Query  *QueryRef
	Struct string
}

// GoType is the Go type of the value the handler receives: the
// changeset type, qualified unless the query is the package's own, or
// the struct.
func (b BodyInfo) GoType() string {
	if b.Kind == BodyStruct {
		return b.Struct
	}
	t := b.Query.Changeset.Type
	if !b.Query.Local {
		t = b.Query.Qualifier + "." + t
	}
	return t
}

// body lowers a controller route's body: and only: settings (spec
// §request-bodies); nil with ok when the route has no body.
func (l *routeLowering) body(r *ast.Route, params []Param, bodyS, onlyS *ast.Setting) (*BodyInfo, bool) {
	if bodyS == nil {
		if onlyS != nil {
			l.errorf(onlyS.Pos(), "request-bodies", "only: narrows the columns a body: changeset casts; the route has no body (§request-bodies)")
			return nil, false
		}
		return nil, true
	}
	byName := map[string]Param{}
	for _, p := range params {
		byName[p.Name] = p
	}
	switch v := bodyS.Value.(type) {
	case *ast.QueryCall:
		q := l.bodyQuery(v, byName)
		if q == nil {
			return nil, false
		}
		if !l.narrow(q, onlyS) {
			return nil, false
		}
		return &BodyInfo{Kind: BodyChangeset, Query: q}, true
	case *ast.QualIdent:
		q := l.bodyQuery(&ast.QueryCall{Ref: &ast.GoRef{Parts: []*ast.Ident{v.Qual, v.Name}}, Rparen: v.End()}, byName)
		if q == nil {
			return nil, false
		}
		if !l.narrow(q, onlyS) {
			return nil, false
		}
		return &BodyInfo{Kind: BodyChangeset, Query: q}, true
	case *ast.Ident:
		name := v.Name()
		if _, hasSchema, _ := l.dataPackage(l.facts.pkgPath); hasSchema {
			if _, _, cm, ok := l.crudByName(l.facts.pkgPath, name); ok && cm.Body != "" {
				q := l.bodyQuery(&ast.QueryCall{Ref: &ast.GoRef{Parts: []*ast.Ident{v}}, Rparen: v.End()}, byName)
				if q == nil || !l.narrow(q, onlyS) {
					return nil, false
				}
				return &BodyInfo{Kind: BodyChangeset, Query: q}, true
			}
		}
		if onlyS != nil {
			l.errorf(onlyS.Pos(), "request-bodies", "only: narrows the columns a body: changeset casts; %s is a struct of this package (§request-bodies)", name)
			return nil, false
		}
		if !exportedIdentOK(name) {
			l.errorf(v.Pos(), "request-bodies", "body: takes a write query of a data package or an exported struct type of this package, found %q (§request-bodies)", name)
			return nil, false
		}
		underlying, ok := l.goType(name)
		switch {
		case !ok:
			l.errorf(v.Pos(), "request-bodies", "no type %s in package %s's Go files; body: names a write query of a data package or a struct type declared beside the routes (§request-bodies)", name, l.facts.pkgName)
			return nil, false
		case !strings.HasPrefix(underlying, "struct"):
			l.errorf(v.Pos(), "request-bodies", "%s is %s, not a struct type; a body decodes into a struct (§request-bodies)", name, underlying)
			return nil, false
		}
		return &BodyInfo{Kind: BodyStruct, Struct: name}, true
	}
	l.errorf(bodyS.Pos(), "request-bodies", "body: takes a write query of a data package, with the key of an update, or a struct type of this package (§request-bodies)")
	return nil, false
}

func (l *routeLowering) bodyQuery(call *ast.QueryCall, byName map[string]Param) *QueryRef {
	parts := call.Ref.Parts
	name := parts[len(parts)-1].Name()
	qual, target, local := l.facts.pkgName, l.facts.pkgPath, true
	if len(parts) == 2 {
		qual = parts[0].Name()
		t, kind := l.qualify(qual)
		switch kind {
		case qualImport:
			target, local = t, false
		case qualSelf:
			l.diags = append(l.diags, selfQualified(call.Ref.Pos(), qual, name))
			return nil
		default:
			l.errorf(call.Ref.Pos(), "request-bodies", "body: names %q, which is not an import of this package (§request-bodies, §imports)", qual)
			return nil
		}
	}
	pkgName, hasSchema, _ := l.dataPackage(target)
	ref := qual + "." + name
	if local {
		ref = name
	}
	if !hasSchema {
		l.errorf(call.Ref.Pos(), "request-bodies", "package %q declares no tables, so it has no write %s (§request-bodies)", target, name)
		return nil
	}
	key, _, cm, ok := l.crudByName(target, name)
	if !ok || cm.Body == "" {
		l.errorf(call.Ref.Pos(), "request-bodies", "%s is not a write a body binds to: body: names a Create or an Update of a data package (§request-bodies)", ref)
		return nil
	}
	importPath := l.facts.module
	if target != "." {
		importPath += "/" + target
	}
	qr := &QueryRef{Qualifier: qual, Package: target, Import: importPath, PkgName: pkgName, Local: local, Method: name, Status: 200,
		Result: cm.Result, Changeset: &ChangesetBind{Type: cm.Body, Params: cm.Params, New: cm.New, Get: cm.Get, Allow: cm.Allow, Table: key}}
	if len(call.Args) != len(cm.Key) {
		if len(cm.Key) == 0 {
			l.errorf(call.Ref.Pos(), "request-bodies", "%s takes no argument: a create loads no row, the body: gives %d (§request-bodies)", ref, len(call.Args))
			return nil
		}
		l.errorf(call.Ref.Pos(), "request-bodies", "%s loads the row to update by %d key column(s), the body: gives %d argument(s) (§request-bodies)", ref, len(cm.Key), len(call.Args))
		return nil
	}
	ok = true
	for i, arg := range call.Args {
		k := cm.Key[i]
		qp := QueryParam{Name: k.GoName, GoType: k.GoType}
		switch a := arg.(type) {
		case *ast.Ident:
			pp, inPath := byName[a.Name()]
			switch {
			case !inPath:
				l.errorf(a.Pos(), "request-bodies", "the key %q of %s must be a path parameter of the route (§request-bodies)", a.Name(), ref)
				ok = false
			case pp.Wild:
				l.errorf(a.Pos(), "request-bodies", "the key %q of %s cannot be the wildcard (§request-bodies)", a.Name(), ref)
				ok = false
			case pp.Type != tUnresolved && pp.Type.GoType() != k.GoType:
				l.errorf(a.Pos(), "request-bodies", "path parameter %q is %s but %s takes %s (§request-bodies)", pp.Name, pp.Type.GoType(), ref, k.GoType)
				ok = false
			}
			qp.Source, qp.Name = FromPath, a.Name()
		case *ast.BasicLit:
			qp.Source = FromLiteral
			if a.Tok.Kind == token.STRING {
				if k.GoType != "string" {
					l.errorf(a.Pos(), "request-bodies", "key %d of %s is %s, not a string (§request-bodies)", i+1, ref, k.GoType)
					ok = false
				}
				qp.Literal = strconv.Quote(a.Tok.Val)
			} else if qp.Literal = numberLiteral(a.Tok.Val, k.GoType); qp.Literal == "" {
				l.errorf(a.Pos(), "request-bodies", "key %d of %s is %s, which the number %s does not spell (§request-bodies)", i+1, ref, k.GoType, a.Tok.Val)
				ok = false
			}
		case *ast.NegNumber:
			qp.Source = FromLiteral
			if qp.Literal = numberLiteral("-"+a.Num.Tok.Val, k.GoType); qp.Literal == "" {
				l.errorf(a.Pos(), "request-bodies", "key %d of %s is %s, which the number -%s does not spell (§request-bodies)", i+1, ref, k.GoType, a.Num.Tok.Val)
				ok = false
			}
		}
		qr.Params = append(qr.Params, qp)
	}
	if !ok {
		return nil
	}
	return qr
}

// narrow applies an only: setting to the body's allow list (spec
// §request-bodies): every named column must be one the write casts.
func (l *routeLowering) narrow(q *QueryRef, onlyS *ast.Setting) bool {
	if onlyS == nil {
		return true
	}
	list, ok := onlyS.Value.(*ast.IdentList)
	if !ok {
		l.errorf(onlyS.Pos(), "request-bodies", "only: takes a column list like (title, body) (§request-bodies)")
		return false
	}
	ok = true
	var allow []string
	for i, id := range list.Names {
		if list.Mods[i] != nil {
			l.errorf(list.Mods[i].Pos(), "request-bodies", "only: entries are column names; %q does not belong after %q (§request-bodies)", list.Mods[i].Name(), id.Name())
			ok = false
			continue
		}
		h, found := l.columnHandle(q.Package, q.Changeset.Table, id.Name())
		if !found || !contains(q.Changeset.Allow, h) {
			l.errorf(id.Pos(), "request-bodies", "only: names %q, which %s does not cast; the columns it casts: %s (§request-bodies)", id.Name(), q.Ref(), strings.Join(q.Changeset.Allow, ", "))
			ok = false
			continue
		}
		if contains(allow, h) {
			l.errorf(id.Pos(), "request-bodies", "only: names %q twice (§request-bodies)", id.Name())
			ok = false
			continue
		}
		allow = append(allow, h)
	}
	if !ok {
		return false
	}
	q.Changeset = &ChangesetBind{Type: q.Changeset.Type, Params: q.Changeset.Params, New: q.Changeset.New, Get: q.Changeset.Get, Allow: allow, Table: q.Changeset.Table}
	return true
}

func bodiesEqual(a, b *BodyInfo) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	return a == nil || a.GoType() == b.GoType()
}

func signatureString(sl []ScopeLoad, ps []Param, b *BodyInfo, ls []LoadInfo) string {
	s := scopeLoadsString(sl) + paramsString(ps)
	if b != nil {
		s += " [body: " + b.GoType() + "]"
	}
	return s + " " + loadsString(ls)
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
