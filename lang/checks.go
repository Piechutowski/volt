// Validation checks (docs/spec.md §V12): typed checks are §V10
// predicate expressions over one table's not-null columns, lowered to
// BOTH a SQL CHECK fragment (consumed by gen/sqlite through the AST)
// and a Go condition (consumed by the validator generator) in a single
// walk, so the two tiers cannot disagree. Go-reference checks resolve
// their arguments here and land in the validator only — SQLite cannot
// call Go, the documented asymmetry.
package lang

import (
	"strconv"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/token"
	"github.com/Piechutowski/volt/nao/gen/golang"
	"github.com/Piechutowski/volt/nao/gen/sqlite"
)

// tableChecks lowers a package's extended checks (§V12). Runs after
// dataQueries so Pred declarations are resolved.
func (c *checker) tableChecks(pkg *Package) {
	c.pkg = pkg
	info := c.schemas[pkg.Path]
	if info == nil {
		return
	}
	for _, ti := range info.Tables {
		var specs []golang.CheckSpec
		// Direct and injected checks alike (§6.9.3): a partial's checks
		// belong to every table it is injected into.
		for _, ck := range ti.Checks {
			var spec golang.CheckSpec
			var ok bool
			switch {
			case ck.Pred != nil:
				spec, ok = c.typedCheck(ti, ck, info)
			case ck.Ref != nil:
				spec, ok = c.goRefCheck(ti, ck, info)
			default:
				continue // opaque SQL: §6.6's business, SQL CHECK only
			}
			if !ok {
				continue
			}
			if n := ck.Settings.Get("name"); n != nil {
				if lit, isStr := n.Value.(*ast.BasicLit); isStr && lit.Tok.Kind == token.STRING {
					spec.Name = lit.Tok.Val
				}
			}
			specs = append(specs, spec)
		}
		if len(specs) > 0 {
			pkg.CheckFns = append(pkg.CheckFns, golang.CheckFn{TableKey: ti.Key, Checks: specs})
		}
	}
}

// typedCheck types one predicate-form check against its table and
// renders the SQL and Go tiers together (§V12.2-§V12.4).
func (c *checker) typedCheck(ti *check.TableInfo, ck *ast.Check, info *check.Info) (golang.CheckSpec, bool) {
	env := c.checkEnv(ti, ck, info)
	if env == nil {
		return golang.CheckSpec{}, false
	}
	sql, gocode := env.render(ck.Pred, map[string]bool{})
	if env.failed {
		return golang.CheckSpec{}, false
	}
	ck.SQL = sql // gen/sqlite emits CHECK (<this>) — one rendering, both tiers
	return golang.CheckSpec{Src: sql, Cond: gocode, Cols: env.used}, true
}

// goRefCheck resolves one Go-reference check (§V12.5): a function of
// the containing package, column arguments, validator tier only.
func (c *checker) goRefCheck(ti *check.TableInfo, ck *ast.Check, info *check.Info) (golang.CheckSpec, bool) {
	if q := ck.Ref.Qualifier(); q != "" && q != c.pkg.Name {
		c.errorf(ck.Ref.Pos(), "V12",
			"a check references a function of the containing package, not of %q — write a local wrapper (§V12.5)", q)
		return golang.CheckSpec{}, false
	}
	env := c.checkEnv(ti, ck, info)
	if env == nil {
		return golang.CheckSpec{}, false
	}
	var args, src, argTypes, declTypes []string
	var fields []golang.FieldSig
	for _, a := range ck.Args {
		f, ok := env.fieldOf(a, false)
		if !ok {
			return golang.CheckSpec{}, false
		}
		args = append(args, "v."+f.Name)
		src = append(src, a.Name())
		argTypes = append(argTypes, f.Type)
		declTypes = append(declTypes, ti.Column(a.Name()).Col.Type.String())
		fields = append(fields, f)
	}
	name := ck.Ref.Base()
	want := "func " + name + "(" + strings.Join(argTypes, ", ") + ") error"

	// The function must exist in this package's Go files with exactly
	// the contract, spelled as the generated field types (§V12.5, D63):
	// the typo and the wrong type are caught here, not by the compiler.
	sc := c.goFuncs(c.pkg)
	gf, found := sc.funcs[name]
	if !found {
		c.errorf(ck.Ref.Pos(), "V12", "no function %s in package %s's Go files — declare %s beside the schema (§V12.5)%s", name, c.pkg.Name, want, sc.brokenHint())
		return golang.CheckSpec{}, false
	}
	if gf.Generic {
		c.errorf(ck.Ref.Pos(), "V12", "%s is generic (%s); a check cannot instantiate it — wrap it in a plain %s (§V12.5)", name, gf.Sig, want)
		return golang.CheckSpec{}, false
	}
	if gf.Variadic {
		c.errorf(ck.Ref.Pos(), "V12", "%s is variadic (%s); a check passes a fixed column list — expected %s (§V12.5)", name, gf.Sig, want)
		return golang.CheckSpec{}, false
	}
	if len(gf.Params) != len(args) {
		c.errorf(ck.Ref.Pos(), "V12", "%s takes %d parameter(s) but the check passes %d column(s): found %s, expected %s (§V12.5)",
			name, len(gf.Params), len(args), gf.Sig, want)
		return golang.CheckSpec{}, false
	}
	for i, prm := range gf.Params {
		if prm.Type != argTypes[i] {
			c.errorf(ck.Args[i].Pos(), "V12", "column %q (%s, Go %s) but parameter %d of %s is %s — change the column type or the function: expected %s (§V12.5)",
				fields[i].Col, declTypes[i], argTypes[i], i+1, name, prm.Type, want)
			return golang.CheckSpec{}, false
		}
	}
	if len(gf.Results) != 1 || gf.Results[0] != "error" {
		c.errorf(ck.Ref.Pos(), "V12", "%s must return exactly error: found %s, expected %s (§V12.5)", name, gf.Sig, want)
		return golang.CheckSpec{}, false
	}
	return golang.CheckSpec{
		Src:  name + "(" + strings.Join(src, ", ") + ")",
		Call: name + "(" + strings.Join(args, ", ") + ")",
		Cols: env.used,
	}, true
}

/* ===== the dual renderer ===== */

// chkEnv threads one check's typing: the enclosing table's generated
// fields, and the class rules of §V10.3 narrowed by §V12 (not-null
// columns; no time class — the Go tier cannot mirror SQL's text-time
// comparison).
type chkEnv struct {
	c      *checker
	ti     *check.TableInfo
	ck     *ast.Check
	byCol  map[string]golang.FieldSig
	failed bool
	used   []string // columns the check reads, first-use order (§V12.6)
}

func (c *checker) checkEnv(ti *check.TableInfo, ck *ast.Check, info *check.Info) *chkEnv {
	_, fields, err := golang.ModelFields(c.pkg.merged, info, ti.Key)
	if err != nil {
		c.errorf(ck.Pos(), "V12", "check: %v", err)
		return nil
	}
	byCol := make(map[string]golang.FieldSig, len(fields))
	for _, f := range fields {
		byCol[f.Col] = f
	}
	return &chkEnv{c: c, ti: ti, ck: ck, byCol: byCol}
}

func (e *chkEnv) errorf(pos token.Position, format string, args ...any) {
	e.failed = true
	e.c.errorf(pos, "V12", format, args...)
}

// fieldOf resolves a column reference under §V12's rules. typed marks
// the typed-check tiers, which additionally reject the time class.
func (e *chkEnv) fieldOf(id *ast.Ident, typed bool) (golang.FieldSig, bool) {
	f, has := e.byCol[id.Name()]
	if !has {
		e.errorf(id.Pos(), "no column %q in table %q (§V12.2)", id.Name(), e.ti.Decl.Name.Base())
		return f, false
	}
	seen := false
	for _, u := range e.used {
		if u == id.Name() {
			seen = true
		}
	}
	if !seen {
		e.used = append(e.used, id.Name())
	}
	if f.Nullable {
		e.errorf(id.Pos(),
			"column %q is nullable; a check's Go and SQL tiers must agree, and three-valued NULL logic is not mirrored — make it not null, or use an opaque `SQL` check (§V12.3)",
			id.Name())
		return f, false
	}
	if typed {
		cd := e.ti.Column(id.Name())
		switch classOf(cd.Col.Type.String()) {
		case classTime:
			e.errorf(id.Pos(),
				"column %q is a date/time; the Go tier cannot mirror SQL's text-time comparison — use an opaque `SQL` check or a Go reference (§V12.4)", id.Name())
			return f, false
		case classForbidden:
			if _, mapped := golang.GoTypeName(normalizeType(cd.Col.Type.String())); !mapped {
				e.errorf(id.Pos(), "column %q is enum-typed (%s); predicates do not compare enums in v1 (§V10.3, hypotheses H5)", id.Name(), cd.Col.Type.String())
				return f, false
			}
			e.errorf(id.Pos(), "column %q (blob/json) cannot appear in a typed check (§V10.3)", id.Name())
			return f, false
		}
	}
	return f, true
}

func (e *chkEnv) classOfCol(name string) typeClass {
	return classOf(e.ti.Column(name).Col.Type.String())
}

// render walks a typed check once, producing the SQL fragment and the
// Go condition together — one typing, two tiers (§V12.4).
func (e *chkEnv) render(x ast.PredExpr, preds map[string]bool) (sql, gocode string) {
	switch x := x.(type) {
	case *ast.PredBinary:
		s1, g1 := e.render(x.X, preds)
		s2, g2 := e.render(x.Y, preds)
		if strings.EqualFold(x.Op, "and") {
			return s1 + " AND " + s2, g1 + " && " + g2
		}
		return s1 + " OR " + s2, g1 + " || " + g2
	case *ast.PredNot:
		s, g := e.render(x.X, preds)
		return "NOT (" + s + ")", "!(" + g + ")"
	case *ast.PredParen:
		s, g := e.render(x.X, preds)
		return "(" + s + ")", "(" + g + ")"
	case *ast.PredRef:
		name := x.Name.Name()
		d := e.c.pkg.Preds[name]
		if d == nil {
			e.errorf(x.Name.Pos(), "unknown predicate %q (§V10.2)", name)
			return "1", "true"
		}
		if preds[name] {
			return "1", "true" // cycle already reported by predRefCycles
		}
		preds[name] = true
		s, g := e.render(d.X, preds)
		delete(preds, name)
		return "(" + s + ")", "(" + g + ")"
	case *ast.PredCompare:
		return e.compare(x)
	case *ast.PredIn:
		return e.in(x)
	case *ast.PredLike:
		return e.like(x)
	case *ast.PredNull:
		e.errorf(x.Col.Pos(),
			"is null is constant in a check: its columns are not null by §V12.3")
		return "1", "true"
	}
	return "1", "true"
}

func (e *chkEnv) compare(x *ast.PredCompare) (string, string) {
	var anchor *golang.FieldSig
	for _, side := range []ast.Operand{x.X, x.Y} {
		if cr, isCol := side.(*ast.ColRef); isCol {
			if f, ok := e.fieldOf(cr.Name, true); ok {
				anchor = &f
			}
		}
	}
	if anchor == nil {
		if !e.failed {
			e.errorf(x.Pos(), "a comparison needs a column on one side (§V10)")
		}
		return "1", "true"
	}
	class := e.classOfCol(anchor.Col)
	if x.Op != token.EQ && x.Op != token.NEQ {
		switch class {
		case classNumeric:
		case classEqOnly:
			e.errorf(x.Pos(), "column %q is decimal-exact (%s); it supports = and != only (§V10.3-§V10.4)", anchor.Col, anchor.Type)
			return "1", "true"
		default:
			e.errorf(x.Pos(), "column %q (%s) is not orderable in a check; < <= > >= need a numeric column (§V12.4)", anchor.Col, anchor.Type)
			return "1", "true"
		}
	}

	goOps := map[token.Kind]string{
		token.EQ: "==", token.NEQ: "!=", token.LT: "<",
		token.LE: "<=", token.GT: ">", token.GE: ">=",
	}
	render := func(o ast.Operand) (string, string) {
		switch o := o.(type) {
		case *ast.ColRef:
			f, ok := e.fieldOf(o.Name, true)
			if !ok {
				return "1", "true"
			}
			if f.Type != anchor.Type {
				e.errorf(o.Pos(), "cannot compare column %q (%s) with column %q (%s) (§V10.3)", anchor.Col, anchor.Type, f.Col, f.Type)
			}
			return sqlite.Ident(f.Col), "v." + f.Name
		case *ast.Param:
			e.errorf(o.Pos(), "a check takes no :params — it judges one row, not a query (§V12.2)")
			return "1", "true"
		case *ast.Lit:
			return e.lit(o, anchor, class)
		}
		return "1", "true"
	}
	s1, g1 := render(x.X)
	s2, g2 := render(x.Y)
	if e.failed {
		return "1", "true"
	}
	return s1 + " " + compareSQL[x.Op] + " " + s2, g1 + " " + goOps[x.Op] + " " + g2
}

// lit renders one literal for both tiers against the anchoring column.
func (e *chkEnv) lit(l *ast.Lit, f *golang.FieldSig, class typeClass) (string, string) {
	switch l.Tok.Kind {
	case token.NUMBER:
		if class != classNumeric {
			e.errorf(l.Pos(), "column %q (%s) cannot be compared with the number %s (§V10.3)", f.Col, f.Type, l.Tok.Val)
			return "1", "true"
		}
		if strings.ContainsAny(l.Tok.Val, ".eE") && !strings.HasPrefix(f.Type, "float") {
			e.errorf(l.Pos(), "the fractional literal %s needs a float column; %q is %s (§V12.4)", l.Tok.Val, f.Col, f.Type)
			return "1", "true"
		}
		return l.Tok.Val, l.Tok.Val
	case token.STRING:
		if class != classText && class != classEqOnly {
			e.errorf(l.Pos(), "column %q (%s) cannot be compared with the string %q (§V10.3)", f.Col, f.Type, l.Tok.Val)
			return "1", "true"
		}
		return "'" + strings.ReplaceAll(l.Tok.Val, "'", "''") + "'", strconv.Quote(l.Tok.Val)
	default: // true/false idents
		if class != classBool {
			e.errorf(l.Pos(), "column %q (%s) is not boolean (§V10.3)", f.Col, f.Type)
			return "1", "true"
		}
		if strings.EqualFold(l.Tok.Val, "true") {
			return "1", "true"
		}
		return "0", "false"
	}
}

func (e *chkEnv) in(x *ast.PredIn) (string, string) {
	f, ok := e.fieldOf(x.Col, true)
	if !ok {
		return "1", "true"
	}
	class := e.classOfCol(f.Col)
	var sqls, gos []string
	for _, it := range x.Items {
		s, g := e.lit(it, &f, class)
		sqls = append(sqls, s)
		gos = append(gos, "v."+f.Name+" == "+g)
	}
	if e.failed {
		return "1", "true"
	}
	return sqlite.Ident(f.Col) + " IN (" + strings.Join(sqls, ", ") + ")",
		"(" + strings.Join(gos, " || ") + ")"
}

func (e *chkEnv) like(x *ast.PredLike) (string, string) {
	f, ok := e.fieldOf(x.Col, true)
	if !ok {
		return "1", "true"
	}
	if e.classOfCol(f.Col) != classText {
		e.errorf(x.Col.Pos(), "like needs a text column; %q is %s (§V10.3)", f.Col, f.Type)
		return "1", "true"
	}
	pat, isLit := x.Pattern.(*ast.Lit)
	if !isLit || pat.Tok.Kind != token.STRING {
		e.errorf(x.Pattern.Pos(), "a check's like pattern is a string literal — no :params in checks (§V12.2)")
		return "1", "true"
	}
	return sqlite.Ident(f.Col) + " LIKE '" + strings.ReplaceAll(pat.Tok.Val, "'", "''") + "'",
		"rt.Like(v." + f.Name + ", " + strconv.Quote(pat.Tok.Val) + ")"
}
