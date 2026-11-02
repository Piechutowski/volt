package rt

import "context"

// With is a parent query and, for the parents it yields, the rows a
// reverse relation leads to (spec §selects-over-groups): two statements
// in one transaction, so both read one snapshot, the children grouped
// by key in memory, in parent order, and each parent built into R with
// its children. Child is the child statement with one placeholder, the
// JSON array of the parent keys; no child statement runs for no
// parents, and a parent without children is built with an empty slice.
type With[P, C any, K comparable, R any] struct {
	Parent    Query[P]
	Child     string
	ChildScan func(Row) (C, error)
	ParentKey func(P) K
	ChildKey  func(C) K
	Build     func(P, []C) R
}

func (With[P, C, K, R]) effect() {}

// Perform runs both statements in one transaction and nests the
// children under their parents; the result is an empty slice, never
// nil, unless the parent query has no statement.
func (w With[P, C, K, R]) Perform(ctx context.Context, x Executor) ([]R, error) {
	if w.Parent.Err != nil {
		return nil, w.Parent.Err
	}
	if w.Parent.SQL == "" {
		return nil, nil
	}
	var out []R
	err := x.Transaction(ctx, func(tx Executor) error {
		parents, err := w.Parent.Perform(ctx, tx)
		if err != nil {
			return err
		}
		out = make([]R, 0, len(parents))
		if len(parents) == 0 {
			return nil
		}
		keys := make([]K, len(parents))
		for i, p := range parents {
			keys[i] = w.ParentKey(p)
		}
		children, err := Query[C]{SQL: w.Child, Args: []any{JSONList(keys)}, Scan: w.ChildScan}.Perform(ctx, tx)
		if err != nil {
			return err
		}
		byKey := make(map[K][]C, len(parents))
		for _, c := range children {
			k := w.ChildKey(c)
			byKey[k] = append(byKey[k], c)
		}
		for i, p := range parents {
			cs := byKey[keys[i]]
			if cs == nil {
				cs = []C{}
			}
			out = append(out, w.Build(p, cs))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// PerformAny is Perform with its result as any.
func (w With[P, C, K, R]) PerformAny(ctx context.Context, x Executor) (any, error) {
	return w.Perform(ctx, x)
}

func (w With[P, C, K, R]) describe() string {
	return describeStatement("with", w.Parent.SQL, w.Parent.Args, w.Parent.Err) + "; " + describeStatement("children", w.Child, nil, nil)
}
