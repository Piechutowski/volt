// The dynamic query layer and the one SQL renderer (spec §predicates): predicates, orderings, limits and assignments as inert data,
// plain values forming a small expression tree, never closures and never
// a mutable builder. The interpreter is deterministic: identical trees
// render identical SQL, which is what makes the rendered text a
// statement-cache key, and the generator renders its statement constants
// through the same functions. The layer only filters, orders and limits
// an existing row shape (the shape rule).
package rt

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
)

// Column is the typed handle of one column of model M: its methods build
// the Pred, Order and Assign values of the dynamic query layer. M is a
// phantom type: a predicate over one model cannot enter another model's
// query. T is the column's Go value type; operator arguments are T.
type Column[M, T any] struct {
	// Name is the SQL column name, unquoted.
	Name string
}

// NullColumn is the handle of a nullable column. Comparisons and Set take
// present values (T, not Null[T]), so Set(nil) of a []byte is empty bytes;
// NULL is tested with IsNull/IsNotNull and assigned with SetNull.
type NullColumn[M, T any] struct {
	Column[M, T]
}

func (c Column[M, T]) cmp(op string, v T) Pred[M] {
	return Pred[M]{kind: predCmp, col: c.Name, op: op, vals: []any{v}}
}

// Eq is "column = v". NULL never matches (SQL three-valued logic); test
// NULL with IsNull.
func (c Column[M, T]) Eq(v T) Pred[M] { return c.cmp("=", v) }

// Ne is "column != v", the one spelling of inequality (spec
// §predicates). Rows where the column is NULL never match.
func (c Column[M, T]) Ne(v T) Pred[M] { return c.cmp("!=", v) }

// Gt is "column > v".
func (c Column[M, T]) Gt(v T) Pred[M] { return c.cmp(">", v) }

// Ge is "column >= v".
func (c Column[M, T]) Ge(v T) Pred[M] { return c.cmp(">=", v) }

// Lt is "column < v".
func (c Column[M, T]) Lt(v T) Pred[M] { return c.cmp("<", v) }

// Le is "column <= v".
func (c Column[M, T]) Le(v T) Pred[M] { return c.cmp("<=", v) }

// In is "column IN (vs...)". With no values it renders constant false.
func (c Column[M, T]) In(vs ...T) Pred[M] {
	return Pred[M]{kind: predIn, col: c.Name, vals: valsAny(vs)}
}

// NotIn is "column NOT IN (vs...)". With no values it renders constant
// true. Rows where the column is NULL never match a non-empty NOT IN.
func (c Column[M, T]) NotIn(vs ...T) Pred[M] {
	return Pred[M]{kind: predIn, col: c.Name, vals: valsAny(vs), not: true}
}

// InList is "column IN (SELECT value FROM json_each(?))" over the list
// as one JSON array, one statement text whatever the length, and an empty
// list matches no row; a list whose items bind as bytes is In over them,
// since JSON carries no blob (spec §predicates).
func (c Column[M, T]) InList(vs []T) Pred[M] {
	if listBytes(vs) {
		return c.In(vs...)
	}
	return Pred[M]{kind: predInList, col: c.Name, vals: []any{JSONList(vs)}}
}

func listBytes[T any](vs []T) bool {
	for _, v := range vs {
		dv, err := driver.DefaultParameterConverter.ConvertValue(v)
		if _, ok := dv.([]byte); ok && err == nil {
			return true
		}
	}
	return false
}

// Like is "column LIKE pattern" (SQLite LIKE: case-insensitive ASCII,
// '%' and '_' wildcards).
func (c Column[M, T]) Like(pattern string) Pred[M] {
	return Pred[M]{kind: predCmp, col: c.Name, op: "LIKE", vals: []any{pattern}}
}

// NotLike is "column NOT LIKE pattern".
func (c Column[M, T]) NotLike(pattern string) Pred[M] {
	return Pred[M]{kind: predCmp, col: c.Name, op: "NOT LIKE", vals: []any{pattern}}
}

// IsNull is "column IS NULL".
func (c Column[M, T]) IsNull() Pred[M] {
	return Pred[M]{kind: predNull, col: c.Name}
}

// IsNotNull is "column IS NOT NULL".
func (c Column[M, T]) IsNotNull() Pred[M] {
	return Pred[M]{kind: predNull, col: c.Name, not: true}
}

func (c Column[M, T]) cmpCol(op string, o Column[M, T]) Pred[M] {
	return Pred[M]{kind: predColCmp, col: c.Name, op: op, col2: o.Name}
}

// EqCol compares two columns of the same model: "a = b".
func (c Column[M, T]) EqCol(o Column[M, T]) Pred[M] { return c.cmpCol("=", o) }

// NeCol is "a != b" (spec §predicates).
func (c Column[M, T]) NeCol(o Column[M, T]) Pred[M] { return c.cmpCol("!=", o) }

// GtCol is "a > b".
func (c Column[M, T]) GtCol(o Column[M, T]) Pred[M] { return c.cmpCol(">", o) }

// GeCol is "a >= b".
func (c Column[M, T]) GeCol(o Column[M, T]) Pred[M] { return c.cmpCol(">=", o) }

// LtCol is "a < b".
func (c Column[M, T]) LtCol(o Column[M, T]) Pred[M] { return c.cmpCol("<", o) }

// LeCol is "a <= b".
func (c Column[M, T]) LeCol(o Column[M, T]) Pred[M] { return c.cmpCol("<=", o) }

// Asc orders by this column, ascending.
func (c Column[M, T]) Asc() Order[M] { return Order[M]{col: c.Name} }

// Desc orders by this column, descending.
func (c Column[M, T]) Desc() Order[M] { return Order[M]{col: c.Name, desc: true} }

// Set assigns v to this column in an UpdateWhere.
func (c Column[M, T]) Set(v T) Assign[M] { return Assign[M]{col: c.Name, val: v} }

// SetNull assigns NULL to this column in an UpdateWhere.
func (c NullColumn[M, T]) SetNull() Assign[M] { return Assign[M]{col: c.Name} }

func valsAny[T any](vs []T) []any {
	if len(vs) == 0 {
		return nil
	}
	out := make([]any, len(vs))
	for i, v := range vs {
		out[i] = v
	}
	return out
}

// Pred is one predicate over model M: a small immutable expression tree,
// shared across Query, Count, Exists, DeleteWhere and UpdateWhere.
//
// The zero value is the empty predicate: it filters nothing and vanishes
// from And, Or, Not and WHERE clauses.
type Pred[M any] struct {
	kind  predKind
	col   string
	op    string
	col2  string
	vals  []any
	not   bool
	raw   string
	kids  []Pred[M]
	pairs [][2]string
}

type predKind uint8

const (
	predEmpty predKind = iota
	predCmp
	predColCmp
	predIn
	predInList
	predNull
	predRaw
	predAnd
	predOr
	predNot
	predExists
	predOptional
	predNullSlot
	predVanish
)

// And is the conjunction of the given predicates. Empty predicates are
// dropped; no (surviving) operands is the empty predicate, one is that
// operand unchanged.
func And[M any](ps ...Pred[M]) Pred[M] { return predJoin(predAnd, ps) }

// Or is the disjunction of the given predicates, with the same empty-
// predicate normalization as And.
func Or[M any](ps ...Pred[M]) Pred[M] { return predJoin(predOr, ps) }

// Not negates a predicate. Not of the empty predicate is empty.
func Not[M any](p Pred[M]) Pred[M] {
	if p.kind == predEmpty {
		return p
	}
	return Pred[M]{kind: predNot, kids: []Pred[M]{p}}
}

// Raw is the last-resort escape hatch: verbatim SQL as a predicate, with
// '?' placeholders bound to args. It is outside the safety net:
// nothing validates the fragment until SQLite prepares the statement.
func Raw[M any](sql string, args ...any) Pred[M] {
	if sql == "" {
		return Pred[M]{}
	}
	return Pred[M]{kind: predRaw, raw: sql, vals: args}
}

func predJoin[M any](kind predKind, ps []Pred[M]) Pred[M] {
	var kids []Pred[M]
	for _, p := range ps {
		if p.kind != predEmpty {
			kids = append(kids, p)
		}
	}
	switch len(kids) {
	case 0:
		return Pred[M]{}
	case 1:
		return kids[0]
	}
	return Pred[M]{kind: kind, kids: kids}
}

func (p Pred[M]) render(b *strings.Builder, args *[]any) error {
	switch p.kind {
	case predCmp:
		b.WriteString(Ident(p.col) + " " + p.op + " ?")
		*args = argsAppend(*args, p.vals[0])
	case predColCmp:
		b.WriteString(Ident(p.col) + " " + p.op + " " + Ident(p.col2))
	case predIn:
		if len(p.vals) == 0 {
			if p.not {
				b.WriteString("1 = 1")
			} else {
				b.WriteString("1 = 0")
			}
			return nil
		}
		b.WriteString(Ident(p.col))
		if p.not {
			b.WriteString(" NOT")
		}
		b.WriteString(" IN (?" + strings.Repeat(", ?", len(p.vals)-1) + ")")
		*args = argsAppend(*args, p.vals...)
	case predInList:
		b.WriteString(Ident(p.col) + " IN (SELECT value FROM json_each(?))")
		*args = argsAppend(*args, p.vals[0])
	case predNull:
		b.WriteString(Ident(p.col) + " IS")
		if p.not {
			b.WriteString(" NOT")
		}
		b.WriteString(" NULL")
	case predRaw:
		b.WriteString("(" + p.raw + ")")
		*args = argsAppend(*args, p.vals...)
	case predAnd, predOr:
		sep := " AND "
		if p.kind == predOr {
			sep = " OR "
		}
		b.WriteString("(")
		for i, k := range p.kids {
			if i > 0 {
				b.WriteString(sep)
			}
			if err := k.render(b, args); err != nil {
				return err
			}
		}
		b.WriteString(")")
	case predNot:
		b.WriteString("NOT ")
		return p.kids[0].parenRender(b, args)
	case predExists:
		b.WriteString("EXISTS (SELECT 1 FROM " + Ident(p.col) + " AS " + Ident(p.col2) + " WHERE ")
		for i, pair := range p.pairs {
			if i > 0 {
				b.WriteString(" AND ")
			}
			b.WriteString(Ident(pair[0]) + " = " + Ident(pair[1]))
		}
		if len(p.kids) > 0 {
			b.WriteString(" AND ")
			if err := p.kids[0].parenRender(b, args); err != nil {
				return err
			}
		}
		b.WriteString(")")
	case predOptional:
		guard, sep := "? IS NULL OR ", ""
		if p.not {
			guard, sep = "? IS NOT NULL AND ", ""
		}
		b.WriteString("(" + sep)
		for _, v := range p.vals {
			b.WriteString(guard)
			*args = argsAppend(*args, v)
		}
		if err := p.kids[0].render(b, args); err != nil {
			return err
		}
		b.WriteString(")")
	case predNullSlot:
		b.WriteString("? IS NULL")
		*args = argsAppend(*args, p.vals[0])
	case predVanish:
		b.WriteString("(")
		var err error
		if p.not {
			b.WriteString("NOT ")
			err = p.kids[0].parenRender(b, args)
		} else {
			err = p.kids[0].render(b, args)
		}
		if err != nil {
			return err
		}
		if p.not {
			b.WriteString(" AND ")
		} else {
			b.WriteString(" OR ")
		}
		if err := p.kids[1].render(b, args); err != nil {
			return err
		}
		b.WriteString(")")
	default:
		return errors.New("rt: cannot render an empty predicate")
	}
	return nil
}

func (p Pred[M]) parenRender(b *strings.Builder, args *[]any) error {
	if p.kind == predAnd || p.kind == predOr || p.kind == predRaw || p.kind == predOptional || p.kind == predVanish {
		return p.render(b, args)
	}
	b.WriteString("(")
	if err := p.render(b, args); err != nil {
		return err
	}
	b.WriteString(")")
	return nil
}

// Order is one ORDER BY term, built by Column.Asc/Desc.
type Order[M any] struct {
	col  string
	desc bool
}

// Assign is one "column = value" of an UpdateWhere, built by Column.Set
// and NullColumn.SetNull.
type Assign[M any] struct {
	col string
	val any
}

// Opt is one option of a dynamic query: a predicate (Pred is an Opt), an
// ordering, a limit, an offset, a keyset position or a distinct flag.
// Value-less options cannot infer M, so the generator emits per-model
// wrappers (UserLimit, UserOrderBy) over the constructors here.
type Opt[M any] interface{ optApply(*querySpec[M]) }

type querySpec[M any] struct {
	preds     []Pred[M]
	orders    []Order[M]
	limit     any
	limitSet  bool
	offset    int
	offsetSet bool
	after     []any
	afterSet  bool
	distinct  bool
	groups    []string
}

func (p Pred[M]) optApply(s *querySpec[M]) { s.preds = append(s.preds, p) }

type orderOpt[M any] struct{ terms []Order[M] }

func (o orderOpt[M]) optApply(s *querySpec[M]) { s.orders = append(s.orders, o.terms...) }

// OrderBy sorts the result by the given terms, in order. Repeated
// OrderBy options append.
func OrderBy[M any](terms ...Order[M]) Opt[M] { return orderOpt[M]{terms: terms} }

type limitOpt[M any] struct{ n any }

func (o limitOpt[M]) optApply(s *querySpec[M]) { s.limit, s.limitSet = o.n, true }

// Limit caps the number of rows returned; the last Limit wins. The value
// is bound as a parameter, so page size does not change the SQL text
// (one cached statement). A limit below 1 is a rendering that fails with
// ErrLimit (spec §generated-query-surface), since SQLite reads a negative
// limit as none.
func Limit[M any](n int) Opt[M] { return limitOpt[M]{n: n} }

// LimitVal is a limit over a value slot (spec §selects-over-groups), the
// option a select's limit: lowers to: the literal or the parameter the
// generated function binds behind LIMIT ?.
func LimitVal(v any) Opt[Gen] { return limitOpt[Gen]{n: v} }

// ErrLimit is what a select's effect carries when its limit parameter is
// below 1, and what a dynamic Limit below 1 fails to render with (spec
// §selects-over-groups, §generated-query-surface): SQLite reads a negative
// LIMIT as no limit at all, so the runner returns it without running a
// statement.
var ErrLimit = errors.New("rt: a limit is at least 1")

// LimitCheck is nil for a limit of at least 1 and ErrLimit, naming n,
// otherwise (spec §selects-over-groups); a generated select sets its
// effect's Err to it.
func LimitCheck(n int) error {
	if n >= 1 {
		return nil
	}
	return fmt.Errorf("%w, not %d", ErrLimit, n)
}

type offsetOpt[M any] struct{ n int }

func (o offsetOpt[M]) optApply(s *querySpec[M]) { s.offset, s.offsetSet = o.n, true }

// Offset skips n rows; the last Offset wins. Keyset pagination (After)
// is the scalable page mechanism.
func Offset[M any](n int) Opt[M] { return offsetOpt[M]{n: n} }

type afterOpt[M any] struct{ key []any }

func (o afterOpt[M]) optApply(s *querySpec[M]) { s.after, s.afterSet = o.key, true }

// After positions the query strictly after the row with the given key
// (keyset pagination): the previous page's last row's values for
// the ORDER BY columns, one per term, in the same order. It renders as
// one more predicate of the WHERE clause (spec §predicates) and
// requires an OrderBy and one key value per term. NULLs in keyset
// columns are not paginated over; keep keyset columns NOT NULL.
func After[M any](key ...any) Opt[M] { return afterOpt[M]{key: key} }

type groupOpt[M any] struct{ cols []string }

func (o groupOpt[M]) optApply(s *querySpec[M]) { s.groups = append(s.groups, o.cols...) }

// GroupBy groups the rows by the named items, a column or an alias
// (spec §selects-over-groups); the generator's selects use it, the
// dynamic layer has no grouping.
func GroupBy[M any](cols ...string) Opt[M] { return groupOpt[M]{cols: cols} }

type distinctOpt[M any] struct{}

func (distinctOpt[M]) optApply(s *querySpec[M]) { s.distinct = true }

// Distinct deduplicates the result rows (SELECT DISTINCT).
func Distinct[M any]() Opt[M] { return distinctOpt[M]{} }

// SelectRender renders a SELECT over the model's fixed shape (spec
// §predicates): table and columns come pre-rendered from generated
// code, everything else from the options. Deterministic: identical option
// values render identical SQL. Argument order matches placeholder order
// by construction.
func SelectRender[M any](table, columns string, opts []Opt[M]) (string, []any, error) {
	return selectRenderOver(table, columns, nil, opts)
}

// SelectRenderItems is SelectRender over a rendered select list of
// items (spec §selects-over-groups), whose arguments come first.
func SelectRenderItems[M any](table string, items []Item, opts []Opt[M]) (string, []any, error) {
	columns, args, err := ItemsRender(items)
	if err != nil {
		return "", nil, err
	}
	return selectRenderOver(table, columns, args, opts)
}

func selectRenderOver[M any](table, columns string, columnArgs []any, opts []Opt[M]) (string, []any, error) {
	var s querySpec[M]
	for _, o := range opts {
		if o != nil {
			o.optApply(&s)
		}
	}

	var b strings.Builder
	args := append([]any(nil), columnArgs...)
	b.WriteString("SELECT ")
	if s.distinct {
		b.WriteString("DISTINCT ")
	}
	b.WriteString(columns + " FROM " + table)

	preds := s.preds
	if s.afterSet {
		after, err := afterPred(s.orders, s.after)
		if err != nil {
			return "", nil, err
		}
		preds = append(preds, after)
	}
	where, wargs, err := WhereRender(preds)
	if err != nil {
		return "", nil, err
	}
	if where != "" {
		b.WriteString(" WHERE " + where)
		args = append(args, wargs...)
	}

	if len(s.groups) > 0 {
		names := make([]string, len(s.groups))
		for i, g := range s.groups {
			names[i] = Ident(g)
		}
		b.WriteString(" GROUP BY " + strings.Join(names, ", "))
	}
	if len(s.orders) > 0 {
		b.WriteString(" ORDER BY " + OrderRender(s.orders))
	}

	switch {
	case s.limitSet:
		if n, ok := s.limit.(int); ok {
			if err := LimitCheck(n); err != nil {
				return "", nil, err
			}
		}
		b.WriteString(" LIMIT ?")
		args = append(args, s.limit)
		if s.offsetSet {
			b.WriteString(" OFFSET ?")
			args = append(args, s.offset)
		}
	case s.offsetSet:
		b.WriteString(" LIMIT -1 OFFSET ?")
		args = append(args, s.offset)
	}
	return b.String(), args, nil
}

// CountRender renders "SELECT count(*)" filtered by the predicates; none
// counts the whole table.
func CountRender[M any](table string, preds []Pred[M]) (string, []any, error) {
	where, args, err := WhereRender(preds)
	if err != nil {
		return "", nil, err
	}
	q := "SELECT count(*) FROM " + table
	if where != "" {
		q += " WHERE " + where
	}
	return q, args, nil
}

// ExistsRender renders "SELECT EXISTS (...)" over the predicates.
func ExistsRender[M any](table string, preds []Pred[M]) (string, []any, error) {
	where, args, err := WhereRender(preds)
	if err != nil {
		return "", nil, err
	}
	q := "SELECT EXISTS (SELECT 1 FROM " + table
	if where != "" {
		q += " WHERE " + where
	}
	return q + ")", args, nil
}

// DeleteRender renders a predicate-guarded DELETE. No effective
// predicate is an error, not a full-table delete.
func DeleteRender[M any](table string, preds []Pred[M]) (string, []any, error) {
	where, args, err := WhereRender(preds)
	if err != nil {
		return "", nil, err
	}
	if where == "" {
		return "", nil, errors.New(`rt: DeleteWhere with no predicate would delete every row; say it out loud (e.g. rt.Raw("1 = 1")) or use the CRUD surface`)
	}
	return "DELETE FROM " + table + " WHERE " + where, args, nil
}

// UpdateRender renders a predicate-guarded partial UPDATE from typed
// assignments. No assignments, or no effective predicate, is an error.
func UpdateRender[M any](table string, set []Assign[M], preds []Pred[M]) (string, []any, error) {
	if len(set) == 0 {
		return "", nil, errors.New("rt: UpdateWhere with no assignments has nothing to set")
	}
	var b strings.Builder
	var args []any
	b.WriteString("UPDATE " + table + " SET ")
	for i, a := range set {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(Ident(a.col) + " = ?")
		args = argsAppend(args, a.val)
	}
	where, wargs, err := WhereRender(preds)
	if err != nil {
		return "", nil, err
	}
	if where == "" {
		return "", nil, errors.New(`rt: UpdateWhere with no predicate would rewrite every row; say it out loud (e.g. rt.Raw("1 = 1"))`)
	}
	b.WriteString(" WHERE " + where)
	return b.String(), append(args, wargs...), nil
}

// WhereRender renders the text after WHERE for the conjunction of the
// predicates (spec §predicates), "" when none survives; every
// statement renderer reads its WHERE clause from it.
func WhereRender[M any](preds []Pred[M]) (string, []any, error) {
	var w strings.Builder
	var args []any
	for _, p := range preds {
		if p.kind == predEmpty {
			continue
		}
		if w.Len() > 0 {
			w.WriteString(" AND ")
		}
		if err := p.render(&w, &args); err != nil {
			return "", nil, err
		}
	}
	return w.String(), args, nil
}

// OrderRender renders the text after ORDER BY for the terms (spec
// §predicates), every direction spelled.
func OrderRender[M any](terms []Order[M]) string {
	var b strings.Builder
	for i, o := range terms {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(Ident(o.col))
		if o.desc {
			b.WriteString(" DESC")
		} else {
			b.WriteString(" ASC")
		}
	}
	return b.String()
}

func afterPred[M any](orders []Order[M], key []any) (Pred[M], error) {
	if len(orders) == 0 {
		return Pred[M]{}, errors.New("rt: After needs an OrderBy: a keyset is a position in some order")
	}
	if len(key) != len(orders) {
		return Pred[M]{}, fmt.Errorf("rt: After got %d key values for %d ORDER BY terms; pass the previous row's value for every term", len(key), len(orders))
	}
	terms := make([]Pred[M], 0, len(orders))
	for i, o := range orders {
		conj := make([]Pred[M], 0, i+1)
		for j := 0; j < i; j++ {
			conj = append(conj, Pred[M]{kind: predCmp, col: orders[j].col, op: "=", vals: []any{key[j]}})
		}
		op := ">"
		if o.desc {
			op = "<"
		}
		conj = append(conj, Pred[M]{kind: predCmp, col: o.col, op: op, vals: []any{key[i]}})
		terms = append(terms, And(conj...))
	}
	return Or(terms...), nil
}

// Ident is the one spelling of an identifier in every statement (spec
// §predicates): always double-quoted, a quote inside doubled.
func Ident(name string) string {
	if i := strings.IndexByte(name, '.'); i >= 0 {
		return Ident(name[:i]) + "." + Ident(name[i+1:])
	}
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// Join is one join of a generated select (spec §selects-over-groups):
// the joined table under the relation's name as its alias, matched on
// the column pairs, the joined table's column first and the base
// table's second; Left joins it LEFT, else INNER.
type Join struct {
	Table string
	As    string
	On    [][2]string
	Left  bool
}

// FromRender renders the source of a select with joins (spec
// §predicates, rendering): the base table followed by one INNER JOIN or
// LEFT JOIN per join, in order, each ON its pairs joined by AND.
func FromRender(table string, joins []Join) string {
	var b strings.Builder
	b.WriteString(Ident(table))
	for _, j := range joins {
		if j.Left {
			b.WriteString(" LEFT JOIN ")
		} else {
			b.WriteString(" INNER JOIN ")
		}
		b.WriteString(Ident(j.Table) + " AS " + Ident(j.As) + " ON ")
		for i, pair := range j.On {
			if i > 0 {
				b.WriteString(" AND ")
			}
			b.WriteString(Ident(j.As) + "." + Ident(pair[0]) + " = " + Ident(table) + "." + Ident(pair[1]))
		}
	}
	return b.String()
}
