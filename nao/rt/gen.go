package rt

import "fmt"

// Gen is the phantom model of the trees the generator builds (spec
// §predicates rule 6). A value slot of a Gen tree holds the Go expression
// that supplies the value when the generated method runs; SelectRender
// hands the slots back as its arguments, in placeholder order, and the
// generator writes them into the method's binding list.
type Gen struct{}

// GenPred is a predicate of the generator's view (spec §predicates rule
// 6): the tree a Volt predicate lowers to.
type GenPred = Pred[Gen]

// GenOrder is an ORDER BY term of the generator's view (spec §predicates
// rule 6).
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
// §predicates rule 6), the tree Column.Eq, Ne, Lt, Le, Gt and Ge build.
// The operator is one of =, !=, <, <=, >, >=; any other spelling, <>
// among them, is an error.
func PredCmp(col, op string, v any) (GenPred, error) {
	if !cmpOpOK(op) {
		return GenPred{}, cmpOpErr(op)
	}
	return Column[Gen, any]{Name: col}.cmp(op, v), nil
}

// PredColCmp is the comparison of two columns (spec §predicates rule 6),
// the tree Column.EqCol and its siblings build, under the operator rule
// of PredCmp.
func PredColCmp(col, op, other string) (GenPred, error) {
	if !cmpOpOK(op) {
		return GenPred{}, cmpOpErr(op)
	}
	return Column[Gen, any]{Name: col}.cmpCol(op, Column[Gen, any]{Name: other}), nil
}

// PredIn is membership in value slots (spec §predicates rule 6), the tree
// Column.In builds.
func PredIn(col string, vs ...any) GenPred {
	return Column[Gen, any]{Name: col}.In(vs...)
}

// PredInList is membership in a list slot, one JSON array (spec
// §predicates rule 6), the tree Column.InList builds; the slot is the
// expression that encodes the list, such as rt.JSONList(names).
func PredInList(col string, list any) GenPred {
	return GenPred{kind: predInList, col: col, vals: []any{list}}
}

// PredLike is a pattern test over a pattern slot (spec §predicates rule
// 6), the tree Column.Like builds.
func PredLike(col string, pattern any) GenPred {
	return GenPred{kind: predCmp, col: col, op: "LIKE", vals: []any{pattern}}
}

// PredNull is "column IS NULL" (spec §predicates rule 6), the tree
// Column.IsNull builds.
func PredNull(col string) GenPred {
	return Column[Gen, any]{Name: col}.IsNull()
}

// PredNotNull is "column IS NOT NULL" (spec §predicates rule 6), the tree
// Column.IsNotNull builds.
func PredNotNull(col string) GenPred {
	return Column[Gen, any]{Name: col}.IsNotNull()
}

// OrderTerm is one ORDER BY term (spec §predicates rule 6), the value
// Column.Asc or Column.Desc builds.
func OrderTerm(col string, desc bool) GenOrder {
	return GenOrder{col: col, desc: desc}
}
