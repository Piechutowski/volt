package rt

import "context"

// With is a parent query and, for the parents it yields, the rows a
// reverse relation leads to (spec §selects-over-groups): two statements
// in one transaction, so both read one snapshot, the children grouped
// by key in memory, in parent order, and each parent built into R with
// its children, an empty slice for none. Child is the child statement,
// run only when there are parents, with one placeholder, the JSON array
// of their keys; the result is an empty slice, never nil, unless the
// parent query has no statement.
type With[P, C any, K comparable, R any] struct {
	Parent    Query[P]
	Child     string
	ChildScan func(Row) (C, error)
	ParentKey func(P) K
	ChildKey  func(C) K
	Build     func(P, []C) R
}

func (w With[P, C, K, R]) perform(ctx context.Context, x Executor) ([]R, error) {
	if w.Parent.Err != nil {
		return nil, w.Parent.Err
	}
	if w.Parent.SQL == "" {
		return nil, nil
	}
	var out []R
	err := txRun(ctx, x, func(tx Executor) error {
		parents, err := w.Parent.perform(ctx, tx)
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
		children, err := Query[C]{SQL: w.Child, Args: []any{JSONList(keys)}, Scan: w.ChildScan}.perform(ctx, tx)
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

func (w With[P, C, K, R]) performAny(ctx context.Context, x Executor) (any, error) {
	return w.perform(ctx, x)
}

func (w With[P, C, K, R]) describe() description {
	d := statementDescribe("with", w.Parent.SQL, w.Parent.Args, w.Parent.Err)
	children := statementDescribe("children", w.Child, nil, nil)
	d.kind += "; " + children.kind
	d.spelled += "; " + children.spelled
	return d
}
