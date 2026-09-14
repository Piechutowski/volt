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

	"github.com/Piechutowski/volt/internal/par"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
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
	// Every table's checks are lowered by tableSpecs, a pure function
	// of the inputs resolved here (D86), on every CPU; the memo
	// answers a table whose inputs are what they were. The phase
	// writes the lowered specs, the DDL's check SQL and the
	// diagnostics in table order, whatever the schedule.
	type lowered struct {
		specs          []golang.CheckSpec
		sqls           map[*ast.Check]string
		diags          []diag.Diagnostic
		create, update bool // the params structs validate (§V12.6): a function of the model and the specs
	}
	outs := make([]lowered, len(info.Tables))
	var memo *checksMemo
	if c.memo != nil {
		memo = &c.memo.checks
		memo.Hits, memo.Misses = 0, 0
	}
	funcs := c.goFuncs(pkg)
	hit := make([]*checksEntry, len(info.Tables))
	par.For(len(info.Tables), func(i int) {
		ti := info.Tables[i]
		model := pkg.plan.ModelRef(ti.Key)
		preds := predsNamed(ti.Checks, pkg.Preds)
		if memo != nil {
			if e := memo.prev[ti]; e != nil && e.holds(model, funcs, pkg.Name, preds) {
				hit[i] = e
				outs[i] = lowered{e.specs, e.sqls, e.diags, e.validCreate, e.validUpdate}
				return
			}
		}
		_, fields, err := pkg.plan.ModelFields(ti.Key)
		specs, sqls, diags := tableSpecs(ti, fields, err, pkg.Name, funcs, preds)
		create, update, _ := pkg.plan.ParamsValidators(ti.Key, specs)
		outs[i] = lowered{specs, sqls, diags, create, update}
		if memo != nil {
			hit[i] = &checksEntry{model: model, funcs: funcs, pkgName: pkg.Name, preds: preds, specs: specs, sqls: sqls, diags: diags, validCreate: create, validUpdate: update}
		}
	})
	if memo != nil {
		memo.next = make(map[*check.TableInfo]*checksEntry, len(info.Tables))
		for i, ti := range info.Tables {
			if _, was := memo.prev[ti]; was && memo.prev[ti] == hit[i] {
				memo.Hits++
			} else {
				memo.Misses++
			}
			memo.next[ti] = hit[i]
		}
		memo.prev, memo.next = memo.next, nil
	}
	pkg.CheckSQL = map[*ast.Check]string{}
	pkg.ValidByKey = make(map[string][2]bool, len(info.Tables))
	for i, ti := range info.Tables {
		c.diags = append(c.diags, outs[i].diags...)
		for ck, sql := range outs[i].sqls {
			pkg.CheckSQL[ck] = sql
		}
		if specs := outs[i].specs; len(specs) > 0 {
			pkg.CheckFns = append(pkg.CheckFns, golang.CheckFn{TableKey: ti.Key, Checks: specs})
		}
		if outs[i].create || outs[i].update {
			pkg.ValidByKey[ti.Key] = [2]bool{outs[i].create, outs[i].update}
		}
	}
	pkg.checkIndex()
}

// checkIndex indexes the lowered checks by table key.
func (p *Package) checkIndex() {
	p.checkFnByKey = make(map[string][]golang.CheckSpec, len(p.CheckFns))
	for _, fn := range p.CheckFns {
		p.checkFnByKey[fn.TableKey] = fn.Checks
	}
}

// lowering is one table's check lowering as a pure function of its
// inputs (D86): the checked table, its model's fields (or why the plan
// has none), the package name a Go reference may qualify with, the
// package directory's Go functions, and the predicates the checks
// name, transitively. It accumulates only what it returns: the lowered
// specs, the SQL of every typed check for the DDL, and diagnostics.
type lowering struct {
	ti        *check.TableInfo
	fields    []golang.FieldSig
	fieldsErr error
	pkgName   string
	funcs     *goScan
	preds     map[string]*ast.Pred

	diags []diag.Diagnostic
	sqls  map[*ast.Check]string
}

func (l *lowering) errorf(pos token.Position, section, format string, args ...any) {
	l.diags = append(l.diags, diag.Errorf(pos, "spec/"+section, format, args...))
}

// tableSpecs lowers one table's checks (§V12) from exactly the inputs
// named on lowering, reading nothing else and writing nothing it did
// not create.
func tableSpecs(ti *check.TableInfo, fields []golang.FieldSig, fieldsErr error, pkgName string, funcs *goScan, preds map[string]*ast.Pred) (specs []golang.CheckSpec, sqls map[*ast.Check]string, diags []diag.Diagnostic) {
	l := &lowering{ti: ti, fields: fields, fieldsErr: fieldsErr, pkgName: pkgName, funcs: funcs, preds: preds, sqls: map[*ast.Check]string{}}
	// Direct and injected checks alike (§6.9.3): a partial's checks
	// belong to every table it is injected into.
	for _, ck := range ti.Checks {
		var spec golang.CheckSpec
		var ok bool
		switch {
		case ck.Pred != nil:
			spec, ok = l.typedCheck(ck)
		case ck.Ref != nil:
			spec, ok = l.goRefCheck(ck)
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
	// required columns (§6.3 extension, D72): one synthesized check
	// per column, "non-empty" spelled per type class; gen/sqlite
	// renders the same rule as a CHECK, both named <column>_required.
	if req := l.requiredSpecs(); len(req) > 0 {
		specs = append(specs, req...)
	}
	return specs, l.sqls, l.diags
}

// predsNamed is the predicates a table's typed checks name,
// transitively through the predicates' own bodies, by name; nil where
// the name resolves to none. A pure input of the lowering.
func predsNamed(checks []*ast.Check, all map[string]*ast.Pred) map[string]*ast.Pred {
	var exprs []ast.PredExpr
	for _, ck := range checks {
		if ck.Pred != nil {
			exprs = append(exprs, ck.Pred)
		}
	}
	return predsIn(exprs, all)
}

// predsIn is the predicates the expressions name, transitively through
// the predicates' own bodies, by name; nil where the name resolves to
// none.
func predsIn(exprs []ast.PredExpr, all map[string]*ast.Pred) map[string]*ast.Pred {
	out := map[string]*ast.Pred{}
	var walk func(x ast.PredExpr)
	walk = func(x ast.PredExpr) {
		switch x := x.(type) {
		case *ast.PredBinary:
			walk(x.X)
			walk(x.Y)
		case *ast.PredNot:
			walk(x.X)
		case *ast.PredParen:
			walk(x.X)
		case *ast.PredRef:
			name := x.Name.Name()
			if _, seen := out[name]; seen {
				return
			}
			d := all[name]
			out[name] = d
			if d != nil {
				walk(d.X)
			}
		}
	}
	for _, x := range exprs {
		walk(x)
	}
	return out
}

// requiredSpecs lowers every [required] column of a table to the Go
// tier's condition (§V12.8).
func (l *lowering) requiredSpecs() []golang.CheckSpec {
	var out []golang.CheckSpec
	fields := l.fields
	for _, cd := range l.ti.Columns {
		if cd.Col.Settings.Get("required") == nil {
			continue
		}
		if l.fieldsErr != nil {
			return nil
		}
		name := cd.Col.Name.Name()
		var f *golang.FieldSig
		for i := range fields {
			if fields[i].Col == name {
				f = &fields[i]
			}
		}
		if f == nil {
			continue
		}
		_, isEnum := golang.GoTypeName(normalizeType(cd.Col.Type.String()))
		kind, ok := check.RequiredKind(cd.Col.Type.String(), !isEnum)
		if !ok {
			continue // reported by the schema checker
		}
		var cond string
		switch kind {
		case "text":
			cond = "v." + f.Name + ` != ""`
		case "numeric":
			cond = "v." + f.Name + " != 0"
		case "bytes":
			cond = "len(v." + f.Name + ") > 0"
		}
		out = append(out, golang.CheckSpec{Name: name + "_required", Src: name + " required", Cond: cond, Cols: []string{name}})
	}
	return out
}

// typedCheck types one predicate-form check against its table and
// renders the SQL and Go tiers together (§V12.2-§V12.4).
func (l *lowering) typedCheck(ck *ast.Check) (golang.CheckSpec, bool) {
	env := l.checkEnv(ck)
	if env == nil {
		return golang.CheckSpec{}, false
	}
	sql, gocode := env.render(ck.Pred, map[string]bool{})
	if env.failed {
		return golang.CheckSpec{}, false
	}
	l.sqls[ck] = sql // gen/sqlite emits CHECK (<this>) — one rendering, both tiers
	return golang.CheckSpec{Src: sql, Cond: gocode, Cols: env.used}, true
}

// goRefCheck resolves one Go-reference check (§V12.5): a function of
// the containing package, column arguments, validator tier only.
func (l *lowering) goRefCheck(ck *ast.Check) (golang.CheckSpec, bool) {
	ti := l.ti
	if q := ck.Ref.Qualifier(); q != "" && q != l.pkgName {
		l.errorf(ck.Ref.Pos(), "V12",
			"a check references a function of the containing package, not of %q — write a local wrapper (§V12.5)", q)
		return golang.CheckSpec{}, false
	}
	env := l.checkEnv(ck)
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
	sc := l.funcs
	gf, found := sc.funcs[name]
	if !found {
		l.errorf(ck.Ref.Pos(), "V12", "no function %s in package %s's Go files — declare %s beside the schema (§V12.5)%s", name, l.pkgName, want, sc.brokenHint())
		return golang.CheckSpec{}, false
	}
	if gf.Generic {
		l.errorf(ck.Ref.Pos(), "V12", "%s is generic (%s); a check cannot instantiate it — wrap it in a plain %s (§V12.5)", name, gf.Sig, want)
		return golang.CheckSpec{}, false
	}
	if gf.Variadic {
		l.errorf(ck.Ref.Pos(), "V12", "%s is variadic (%s); a check passes a fixed column list — expected %s (§V12.5)", name, gf.Sig, want)
		return golang.CheckSpec{}, false
	}
	if len(gf.Params) != len(args) {
		l.errorf(ck.Ref.Pos(), "V12", "%s takes %d parameter(s) but the check passes %d column(s): found %s, expected %s (§V12.5)",
			name, len(gf.Params), len(args), gf.Sig, want)
		return golang.CheckSpec{}, false
	}
	for i, prm := range gf.Params {
		if prm.Type != argTypes[i] {
			l.errorf(ck.Args[i].Pos(), "V12", "column %q (%s, Go %s) but parameter %d of %s is %s — change the column type or the function: expected %s (§V12.5)",
				fields[i].Col, declTypes[i], argTypes[i], i+1, name, prm.Type, want)
			return golang.CheckSpec{}, false
		}
	}
	if len(gf.Results) != 1 || gf.Results[0] != "error" {
		l.errorf(ck.Ref.Pos(), "V12", "%s must return exactly error: found %s, expected %s (§V12.5)", name, gf.Sig, want)
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
	l      *lowering
	ti     *check.TableInfo
	ck     *ast.Check
	byCol  map[string]golang.FieldSig
	failed bool
	used   []string // columns the check reads, first-use order (§V12.6)
}

func (l *lowering) checkEnv(ck *ast.Check) *chkEnv {
	if l.fieldsErr != nil {
		l.errorf(ck.Pos(), "V12", "check: %v", l.fieldsErr)
		return nil
	}
	byCol := make(map[string]golang.FieldSig, len(l.fields))
	for _, f := range l.fields {
		byCol[f.Col] = f
	}
	return &chkEnv{l: l, ti: l.ti, ck: ck, byCol: byCol}
}

func (e *chkEnv) errorf(pos token.Position, format string, args ...any) {
	e.failed = true
	e.l.errorf(pos, "V12", format, args...)
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
		d := e.l.preds[name]
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
