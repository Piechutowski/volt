package lang

import (
	"strings"
	"unicode"

	"github.com/Piechutowski/volt/internal/par"
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/token"
	"github.com/Piechutowski/volt/nao/gen/golang"
)

func (c *checker) tableValidators(pkg *Package) {
	c.pkg = pkg
	info := c.schemas[pkg.Path]
	if info == nil {
		return
	}
	type lowered struct {
		specs []golang.ValidatorSpec
		diags []diag.Diagnostic
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
		if memo != nil {
			if e := memo.prev[ti]; e != nil && e.holds(model, funcs, pkg.Name) {
				hit[i] = e
				outs[i] = lowered{e.specs, e.diags}
				return
			}
		}
		_, fields, err := pkg.plan.ModelFields(ti.Key)
		specs, diags := tableSpecs(ti, fields, err, pkg.Name, funcs)
		outs[i] = lowered{specs, diags}
		if memo != nil {
			hit[i] = &checksEntry{model: model, funcs: funcs, pkgName: pkg.Name, specs: specs, diags: diags}
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
	for i, ti := range info.Tables {
		c.diags = append(c.diags, outs[i].diags...)
		if specs := outs[i].specs; len(specs) > 0 {
			pkg.Validators = append(pkg.Validators, golang.ValidatorFn{TableKey: ti.Key, Validators: specs})
		}
	}
}

type lowering struct {
	ti        *check.TableInfo
	fields    []golang.FieldSig
	fieldsErr error
	pkgName   string
	funcs     *goScan

	diags []diag.Diagnostic
}

func (l *lowering) errorf(pos token.Position, format string, args ...any) {
	l.diags = append(l.diags, diag.Errorf(pos, "spec/validators", format, args...))
}

func tableSpecs(ti *check.TableInfo, fields []golang.FieldSig, fieldsErr error, pkgName string, funcs *goScan) (specs []golang.ValidatorSpec, diags []diag.Diagnostic) {
	l := &lowering{ti: ti, fields: fields, fieldsErr: fieldsErr, pkgName: pkgName, funcs: funcs}
	l.changesetNameCheck()
	for _, v := range ti.Validators {
		if spec, ok := l.validatorSpec(v); ok {
			specs = append(specs, spec)
		}
	}
	return specs, l.diags
}

// changesetNameCheck refuses a column whose Go field would be Changeset,
// the name of the row's changeset method (spec §changesets).
func (l *lowering) changesetNameCheck() {
	for _, f := range l.fields {
		if f.Name != "Changeset" {
			continue
		}
		if cd := l.ti.Column(f.Col); cd != nil {
			l.diags = append(l.diags, diag.Errorf(cd.Col.Name.Pos(), "spec/changesets",
				"column %q of table %q maps to the Go field Changeset, the name of the row's changeset method; rename the column (§changesets)", f.Col, l.ti.Decl.Name.Name()))
		}
	}
}

func exportedName(s string) bool {
	for _, r := range s {
		return unicode.IsUpper(r)
	}
	return false
}

func (l *lowering) validatorSpec(v *ast.Validator) (golang.ValidatorSpec, bool) {
	name := v.Ref.Base()
	if q := v.Ref.Qualifier(); q != "" {
		if _, kind := qualifierResolve(q, l.pkgName, nil); kind == qualSelf {
			l.diags = append(l.diags, selfQualified(v.Ref.Pos(), q, name))
			return golang.ValidatorSpec{}, false
		}
		l.errorf(v.Ref.Pos(), "a validator is a function of the containing package, not of %q: write a local wrapper (§validators)", q)
		return golang.ValidatorSpec{}, false
	}
	if !exportedName(name) {
		l.errorf(v.Ref.Pos(), "validator %s must be exported: a validator is part of the package's API — name it %s (§validators)", name, strings.ToUpper(name[:1])+name[1:])
		return golang.ValidatorSpec{}, false
	}
	if l.fieldsErr != nil {
		l.errorf(v.Pos(), "validator: %v", l.fieldsErr)
		return golang.ValidatorSpec{}, false
	}
	byCol := make(map[string]golang.FieldSig, len(l.fields))
	for _, f := range l.fields {
		byCol[f.Col] = f
	}
	var args, src, argTypes, declTypes, used []string
	var unknown []bool
	anyUnknown := false
	for _, a := range v.Args {
		f, has := byCol[a.Name()]
		if !has {
			l.errorf(a.Pos(), "no column %q in table %q (§validators)", a.Name(), l.ti.Decl.Name.Name())
			return golang.ValidatorSpec{}, false
		}
		cd := l.ti.Column(a.Name())
		isUnknown := cd != nil && !cd.Type.Known()
		anyUnknown = anyUnknown || isUnknown
		unknown = append(unknown, isUnknown)
		args = append(args, "v."+f.Name)
		src = append(src, a.Name())
		argTypes = append(argTypes, f.Type)
		declTypes = append(declTypes, cd.Col.Type.String())
		seen := false
		for _, u := range used {
			if u == a.Name() {
				seen = true
			}
		}
		if !seen {
			used = append(used, a.Name())
		}
	}
	want := "func " + name + "(" + strings.Join(argTypes, ", ") + ") error"
	if anyUnknown {
		want = "func " + name
	}

	sc := l.funcs
	gf, found := sc.funcs[name]
	if !found {
		l.errorf(v.Ref.Pos(), "no function %s in package %s's Go files — declare %s beside the schema (§validators)%s", name, l.pkgName, want, sc.brokenHint())
		return golang.ValidatorSpec{}, false
	}
	if gf.Generic {
		l.errorf(v.Ref.Pos(), "%s is generic (%s); a validator cannot instantiate it — wrap it in a plain %s (§validators)", name, gf.Sig, want)
		return golang.ValidatorSpec{}, false
	}
	if gf.Variadic {
		l.errorf(v.Ref.Pos(), "%s is variadic (%s); a validator passes a fixed column list — expected %s (§validators)", name, gf.Sig, want)
		return golang.ValidatorSpec{}, false
	}
	if len(gf.Params) != len(args) {
		l.errorf(v.Ref.Pos(), "%s takes %d parameter(s) but the validator passes %d column(s): found %s, expected %s (§validators)",
			name, len(gf.Params), len(args), gf.Sig, want)
		return golang.ValidatorSpec{}, false
	}
	for i, prm := range gf.Params {
		if !unknown[i] && prm.Type != argTypes[i] {
			l.errorf(v.Args[i].Pos(), "column %q (%s, Go %s) but parameter %d of %s is %s — change the column type or the function: expected %s (§validators)",
				src[i], declTypes[i], argTypes[i], i+1, name, prm.Type, want)
			return golang.ValidatorSpec{}, false
		}
	}
	if len(gf.Results) != 1 || gf.Results[0] != "error" {
		l.errorf(v.Ref.Pos(), "%s must return exactly error: found %s, expected %s (§validators)", name, gf.Sig, want)
		return golang.ValidatorSpec{}, false
	}
	if anyUnknown {
		return golang.ValidatorSpec{}, false
	}
	return golang.ValidatorSpec{
		Name: name,
		Src:  name + "(" + strings.Join(src, ", ") + ")",
		Call: name + "(" + strings.Join(args, ", ") + ")",
		Cols: used,
	}, true
}
