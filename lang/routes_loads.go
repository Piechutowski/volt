package lang

import (
	"strconv"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/token"
)

// loads lowers the load block of a controller route (spec §route-loads):
// every load names a read of a data package and binds its arguments from
// the route's parameters, the query string or literals. It reports every
// fault and answers ok only when all of them lowered.
func (l *routeLowering) loads(r *ast.Route, params []Param) (loads []LoadInfo, ok bool) {
	ok = true
	byName := map[string]Param{}
	for _, p := range params {
		byName[p.Name] = p
	}
	seen := map[string]token.Position{}
	for _, ld := range r.Loads {
		name := ld.Name.Name()
		switch {
		case !loadNameOK(name):
			l.errorf(ld.Name.Pos(), "route-loads", "load %q is not a name a handler parameter can take: a lower-case Go identifier that is not r (§route-loads)", name)
			ok = false
			continue
		case byName[name].Name != "":
			l.errorf(ld.Name.Pos(), "route-loads", "load %q takes the name of a path parameter of the route (§route-loads)", name)
			ok = false
			continue
		}
		if prev, dup := seen[name]; dup {
			l.errorf(ld.Name.Pos(), "route-loads", "load %q is declared twice (first at %s) (§route-loads)", name, prev)
			ok = false
			continue
		}
		seen[name] = ld.Name.Pos()
		q := l.loadQuery(ld, byName)
		if q == nil {
			ok = false
			continue
		}
		loads = append(loads, LoadInfo{Name: name, Query: q})
	}
	return loads, ok
}

func loadNameOK(name string) bool {
	if name == "r" || name == "_" || !exportedIdentOK(strings.ToUpper(name[:1])+name[1:]) {
		return false
	}
	return name[0] >= 'a' && name[0] <= 'z'
}

func (l *routeLowering) loadQuery(ld *ast.Load, byName map[string]Param) *QueryRef {
	parts := ld.Query.Parts
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
			l.diags = append(l.diags, selfQualified(ld.Query.Pos(), qual, name))
			return nil
		default:
			l.errorf(ld.Query.Pos(), "route-loads", "load %s names %q, which is not an import of this package (§route-loads, §imports)", ld.Name.Name(), qual)
			return nil
		}
	}
	pkgName, hasSchema, _ := l.dataPackage(target)
	ref := qual + "." + name
	if local {
		ref = name
	}
	if !hasSchema {
		l.errorf(ld.Query.Pos(), "route-loads", "package %q declares no tables, so it has no query %s to load (§route-loads)", target, name)
		return nil
	}
	importPath := l.facts.module
	if target != "." {
		importPath += "/" + target
	}
	qr := &QueryRef{Qualifier: qual, Package: target, Import: importPath, PkgName: pkgName, Local: local, Method: name, Status: 200}

	type sigParam struct{ name, goType string }
	var sig []sigParam
	found := false
	if si, m, ok := l.selectByMethod(target, name); ok {
		found = true
		for _, p := range si.Params {
			sig = append(sig, sigParam{p.GoName, p.GoType})
		}
		switch {
		case si.Shared != "":
			qr.Result = si.Shared
		case len(si.Excluded) > 0:
			qr.Result = name + "Row"
		default:
			qr.Result = modelOrBase(m)
		}
		qr.Many = true
		if si.Unresolved {
			qr.Unresolved = true
		}
	} else if _, _, cm, ok := l.crudByName(target, name); ok {
		found = true
		switch cm.Op {
		case "get", "list":
		default:
			l.errorf(ld.Query.Pos(), "route-loads", "%s writes; a load reads: a select function, Get or List (§route-loads)", ref)
			return nil
		}
		for _, k := range cm.Key {
			sig = append(sig, sigParam{k.GoName, k.GoType})
		}
		qr.Result, qr.Many = cm.Result, cm.Many
	}
	if !found {
		l.errorf(ld.Query.Pos(), "route-loads", "no generated query %s in package %q%s; a load names a select function, Get or List (§route-loads)", ref, target, l.queryHint(target, name))
		return nil
	}
	if len(ld.Args) != len(sig) {
		l.errorf(ld.Query.Pos(), "route-loads", "%s takes %d argument(s), the load gives %d (§route-loads)", ref, len(sig), len(ld.Args))
		return nil
	}
	ok := true
	for i, arg := range ld.Args {
		sp := sig[i]
		qp := QueryParam{Name: sp.name, GoType: sp.goType}
		if sp.goType == "" {
			qr.Unresolved = true
			qr.Params = append(qr.Params, qp)
			continue
		}
		list := strings.HasPrefix(sp.goType, "[]")
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
			if !queryValueTypes[strings.TrimPrefix(sp.goType, "[]")] {
				l.errorf(a.Pos(), "route-loads", "parameter %q of %s is %s, which the query string cannot carry; write a literal (§route-loads)", sp.name, ref, sp.goType)
				ok = false
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
				}
			}
		case *ast.NegNumber:
			qp.Source = FromLiteral
			qp.Literal = numberLiteral("-"+a.Num.Tok.Val, sp.goType)
			if qp.Literal == "" {
				l.errorf(a.Pos(), "route-loads", "argument %d of %s is %s, which the number -%s does not spell (§route-loads)", i+1, ref, sp.goType, a.Num.Tok.Val)
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

// numberLiteral is the Go spelling of lit as a value of goType, "" when
// the number does not spell one: an integer for int, any number for
// float64.
func numberLiteral(lit, goType string) string {
	switch goType {
	case "int":
		if _, err := strconv.ParseInt(lit, 10, 64); err != nil {
			return ""
		}
		return lit
	case "float64":
		if _, err := strconv.ParseFloat(lit, 64); err != nil {
			return ""
		}
		if !strings.ContainsAny(lit, ".eE") {
			return lit + ".0"
		}
		return lit
	}
	return ""
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
