package rt

import (
	"errors"
	"fmt"
	"strings"
)

// Gen is the phantom model of the trees the generator builds (spec
// §predicates). A value slot of a Gen tree holds the Go expression
// that supplies the value when the generated method runs; SelectRender
// hands the slots back as its arguments, in placeholder order, and the
// generator writes them into the method's binding list.
type Gen struct{}

// GenPred is a predicate of the generator's view (spec §predicates):
// the tree a Volt predicate lowers to.
type GenPred = Pred[Gen]

// GenOrder is an ORDER BY term of the generator's view (spec
// §predicates).
type GenOrder = Order[Gen]

func cmpOpOK(op string) bool {
	switch op {
	case "=", "!=", "<", "<=", ">", ">=":
		return true
	}
	return false
}

func cmpOpErr(op string) error {
	return fmt.Errorf("rt: %q is not a comparison operator; the renderer spells =, !=, <, <=, > and >=", op)
}

// PredCmp is the comparison of a column with a value slot (spec
// §predicates), the tree Column.Eq, Ne, Lt, Le, Gt and Ge build.
// The operator is one of =, !=, <, <=, >, >=; any other spelling, <>
// among them, is an error.
func PredCmp(col, op string, v any) (GenPred, error) {
	if !cmpOpOK(op) {
		return GenPred{}, cmpOpErr(op)
	}
	return Column[Gen, any]{Name: col}.cmp(op, v), nil
}

// PredColCmp is the comparison of two columns (spec §predicates),
// the tree Column.EqCol and its siblings build, under the operator rule
// of PredCmp.
func PredColCmp(col, op, other string) (GenPred, error) {
	if !cmpOpOK(op) {
		return GenPred{}, cmpOpErr(op)
	}
	return Column[Gen, any]{Name: col}.cmpCol(op, Column[Gen, any]{Name: other}), nil
}

// PredIn is membership in value slots (spec §predicates), the tree
// Column.In builds.
func PredIn(col string, vs ...any) GenPred {
	return Column[Gen, any]{Name: col}.In(vs...)
}

// PredInList is membership in a list slot, one JSON array (spec
// §predicates), the tree Column.InList builds; the slot is the
// expression that encodes the list, such as rt.JSONList(names).
func PredInList(col string, list any) GenPred {
	return GenPred{kind: predInList, col: col, vals: []any{list}}
}

// PredLike is a pattern test over a pattern slot (spec §predicates),
// the tree Column.Like builds.
func PredLike(col string, pattern any) GenPred {
	return GenPred{kind: predCmp, col: col, op: "LIKE", vals: []any{pattern}}
}

// PredNull is "column IS NULL" (spec §predicates), the tree
// Column.IsNull builds.
func PredNull(col string) GenPred {
	return Column[Gen, any]{Name: col}.IsNull()
}

// PredNotNull is "column IS NOT NULL" (spec §predicates), the tree
// Column.IsNotNull builds.
func PredNotNull(col string) GenPred {
	return Column[Gen, any]{Name: col}.IsNotNull()
}

// PredExists is the exists term over a relation (spec §predicates):
// true for a row when a row of table, under the alias as, matches p and
// the relation's key, the pairs of on, each a column of the related
// table qualified by as and the column of the outer table it equals.
// A zero p tests the key alone.
func PredExists(table, as string, on [][2]string, p GenPred) GenPred {
	x := GenPred{kind: predExists, col: table, col2: as, pairs: on}
	if p.kind != predEmpty {
		x.kids = []GenPred{p}
	}
	return x
}

// PredOptional is p behind one guard per optional parameter it reads
// (spec §predicates), (? IS NULL OR p), each guard a value slot that
// binds the parameter, so p is true whenever one of them is absent. No
// guards is p itself, and a zero p stays zero.
func PredOptional(guards []any, p GenPred) GenPred {
	if len(guards) == 0 || p.kind == predEmpty {
		return p
	}
	return GenPred{kind: predOptional, vals: guards, kids: []GenPred{p}}
}

// OrderTerm is one ORDER BY term (spec §predicates), the value
// Column.Asc or Column.Desc builds.
func OrderTerm(col string, desc bool) GenOrder {
	return GenOrder{col: col, desc: desc}
}

// Expr is an arithmetic expression of a projected item (spec
// §selects-over-groups): a column, a value slot, or an operation over
// two expressions, which the renderer parenthesizes.
type Expr struct {
	kind exprKind
	col  string
	val  any
	op   string
	kids []Expr
}

type exprKind uint8

const (
	exprCol exprKind = iota + 1
	exprVal
	exprBin
)

// ExprCol is a column as an expression.
func ExprCol(col string) Expr { return Expr{kind: exprCol, col: col} }

// ExprVal is a value slot as an expression: a number literal or a
// parameter, bound as an argument.
func ExprVal(v any) Expr { return Expr{kind: exprVal, val: v} }

// ExprBin is the operation op, one of +, -, * and /, over l and r.
func ExprBin(op string, l, r Expr) (Expr, error) {
	switch op {
	case "+", "-", "*", "/":
		return Expr{kind: exprBin, op: op, kids: []Expr{l, r}}, nil
	}
	return Expr{}, fmt.Errorf("rt: %q is not an arithmetic operator; the renderer spells +, -, * and /", op)
}

func (e Expr) render(b *strings.Builder, args *[]any) error {
	switch e.kind {
	case exprCol:
		b.WriteString(Ident(e.col))
	case exprVal:
		b.WriteString("?")
		*args = argsAppend(*args, e.val)
	case exprBin:
		b.WriteString("(")
		if err := e.kids[0].render(b, args); err != nil {
			return err
		}
		b.WriteString(" " + e.op + " ")
		if err := e.kids[1].render(b, args); err != nil {
			return err
		}
		b.WriteString(")")
	default:
		return errors.New("rt: cannot render an empty expression")
	}
	return nil
}

// Item is one projected item of a generated select (spec
// §selects-over-groups): a column, an aggregate over one or over the
// rows, or an expression, the derived ones under an alias.
type Item struct {
	kind itemKind
	col  string
	fn   string
	expr Expr
	as   string
}

type itemKind uint8

const (
	itemCol itemKind = iota + 1
	itemAgg
	itemExpr
)

// ItemCol projects a column.
func ItemCol(col string) Item { return Item{kind: itemCol, col: col} }

// ItemAgg projects the aggregate fn, one of count, sum, avg, min and
// max, over col, or over the rows when fn is count and col is empty, as
// the alias.
func ItemAgg(fn, col, as string) (Item, error) {
	switch fn {
	case "count":
	case "sum", "avg", "min", "max":
		if col == "" {
			return Item{}, fmt.Errorf("rt: %s takes a column", fn)
		}
	default:
		return Item{}, fmt.Errorf("rt: %q is not an aggregate; the renderer spells count, sum, avg, min and max", fn)
	}
	return Item{kind: itemAgg, fn: fn, col: col, as: as}, nil
}

// ItemExpr projects the expression as the alias.
func ItemExpr(e Expr, as string) Item { return Item{kind: itemExpr, expr: e, as: as} }

// Alias is the item's alias, "" for a column.
func (it Item) Alias() string { return it.as }

// ItemsRender renders a select list (spec §predicates, rendering) and
// returns its arguments in placeholder order.
func ItemsRender(items []Item) (string, []any, error) {
	var b strings.Builder
	var args []any
	for i, it := range items {
		if i > 0 {
			b.WriteString(", ")
		}
		switch it.kind {
		case itemCol:
			b.WriteString(Ident(it.col))
		case itemAgg:
			if it.col == "" {
				b.WriteString("count(*)")
			} else {
				b.WriteString(it.fn + "(" + Ident(it.col) + ")")
			}
		case itemExpr:
			if err := it.expr.render(&b, &args); err != nil {
				return "", nil, err
			}
		default:
			return "", nil, errors.New("rt: cannot render an empty item")
		}
		if it.as != "" {
			b.WriteString(" AS " + Ident(it.as))
		}
	}
	return b.String(), args, nil
}
