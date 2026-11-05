package rt

import "slices"

// FieldDesc is one column as a changeset knows it (spec §changesets): its
// SQL name, its wire name, whether it is part of the key, whether it is
// the rowid alias, the DDL fact, and whether it is the database-assigned
// key, which From leaves unmarked and an insert may leave out.
type FieldDesc struct {
	Column     string
	Wire       string
	Key        bool
	RowidAlias bool
	Assigned   bool
}

// Desc is what a changeset knows of its model and reflection would
// otherwise read (spec §changesets): the names, the columns, the row of
// literal defaults, and the generated functions that scan a row, list a
// row's bound values in column order, apply marked values to a row, and
// cast one column of a write body.
type Desc[M any] struct {
	Model    string
	Table    string
	Columns  string
	Fields   []FieldDesc
	Defaults M
	Scan     func(Row) (M, error)
	Values   func(M) []any
	Apply    func(data, vals M, set []bool) M
	Cast     func(c Changeset[M], p Params[M], i int) (Changeset[M], error)
}

// zero is the desc of a changeset no constructor made (spec §changesets):
// no columns, no name.
func zero[M any]() *Desc[M] { return &Desc[M]{} }

func (c Changeset[M]) d() *Desc[M] {
	if c.desc == nil {
		return zero[M]()
	}
	return c.desc
}

func (d *Desc[M]) index(col string) int {
	for i, f := range d.Fields {
		if f.Column == col {
			return i
		}
	}
	return -1
}

// Changeset carries a row of M, the changes proposed to it and the errors
// found so far (spec §changesets). It is a value: every method returns a
// new changeset and leaves its receiver as it was. A generated
// <Model>Changeset embeds it and adds the typed setters. The zero value,
// which no constructor made, knows no column: it casts and marks
// nothing and writes nothing but DEFAULT VALUES.
type Changeset[M any] struct {
	desc  *Desc[M]
	data  M
	vals  M
	row   bool
	set   []bool
	errs  []FieldError
	valid []FieldError
}

// NewChangeset starts a changeset for a row to create: the literal
// defaults in place and nothing set (spec §changesets).
func NewChangeset[M any](d *Desc[M]) Changeset[M] {
	return Changeset[M]{desc: d, data: d.Defaults}
}

// ChangesetOf starts a changeset for an update of r, nothing set, that
// remembers it was made from a row (spec §changesets).
func ChangesetOf[M any](d *Desc[M], r M) Changeset[M] {
	return Changeset[M]{desc: d, data: r, row: true}
}

// Set marks column i of c with the value set writes into a copy of the
// proposed values (spec §changesets); the generated setters call it. A
// changeset no constructor made, or an index outside its columns, is a
// bug and panics rather than drop the write.
func Set[M any](c Changeset[M], i int, set func(*M)) Changeset[M] {
	vals := c.vals
	set(&vals)
	c.vals = vals
	marks := make([]bool, len(c.d().Fields))
	copy(marks, c.set)
	marks[i] = true
	c.set = marks
	return c
}

// From marks every column but the database-assigned key with its value
// in r (spec §changesets), so a zero row never inserts the key 0.
func (c Changeset[M]) From(r M) Changeset[M] {
	c.vals = r
	marks := make([]bool, len(c.d().Fields))
	for i, f := range c.d().Fields {
		marks[i] = !f.Assigned
	}
	c.set = marks
	return c
}

// FromRow reports whether c was made from a row, so an update has a key
// (spec §changesets).
func (c Changeset[M]) FromRow() bool { return c.row }

// Cast sets every column of allow that p carries, recording a
// cast error for a value that does not convert and an unknown_field
// error for a key that names no column (spec §changesets).
func (c Changeset[M]) Cast(p Params[M], allow ...ColumnOf[M]) Changeset[M] {
	wires := make([]string, len(c.d().Fields))
	for i, f := range c.d().Fields {
		wires[i] = f.Wire
	}
	c.errs = FieldErrorsAppend(c.errs, UnknownFields(p, wires...)...)
	for _, col := range allow {
		i := c.d().index(col.ColumnName())
		if i < 0 {
			continue
		}
		next, err := c.d().Cast(c, p, i)
		if err != nil {
			c = c.errorAdd("cast", err.Error(), c.d().Fields[i].Column)
			continue
		}
		c = next
	}
	return c
}

// Changed reports whether col is marked (spec §changesets).
func (c Changeset[M]) Changed(col ColumnOf[M]) bool {
	i := c.d().index(col.ColumnName())
	return i >= 0 && i < len(c.set) && c.set[i]
}

// Row is the data with the marked values applied (spec §changesets).
func (c Changeset[M]) Row() M {
	if len(c.set) == 0 {
		return c.data
	}
	return c.d().Apply(c.data, c.vals, c.set)
}

// Params is the marked columns of c as a write body, each under its wire
// name (spec §changesets).
func (c Changeset[M]) Params() Params[M] {
	var marks []Mark
	if len(c.set) > 0 {
		vals := c.d().Values(c.vals)
		for i, f := range c.d().Fields {
			if c.set[i] {
				marks = append(marks, Mark{Wire: f.Wire, Value: vals[i]})
			}
		}
	}
	return ParamsOf(c.Row(), marks...)
}

// ErrorAdd records a field error with check custom on col (spec
// §changesets).
func (c Changeset[M]) ErrorAdd(col ColumnOf[M], msg string) Changeset[M] {
	return c.errorAdd("custom", msg, col.ColumnName())
}

func (c Changeset[M]) errorAdd(check, msg string, cols ...string) Changeset[M] {
	c.errs = FieldErrorsAppend(c.errs, FieldError{Check: check, Columns: cols, Message: msg})
	return c
}

// WithFindings is c with the findings of the last Validate replaced
// (spec §changesets); the generated Validate calls it, and no fixed
// method ends in Set, so no setter collides with one.
func (c Changeset[M]) WithFindings(errs []FieldError) Changeset[M] {
	c.valid = slices.Clone(errs)
	return c
}

// Errors lists every field error recorded so far (spec §changesets):
// casts and custom errors first, then the last Validate's findings.
func (c Changeset[M]) Errors() []FieldError {
	return FieldErrorsAppend(FieldErrorsAppend(nil, c.errs...), c.valid...)
}

// Valid reports whether no field error was recorded (spec §changesets).
func (c Changeset[M]) Valid() bool { return len(c.errs) == 0 && len(c.valid) == 0 }

// Err is nil when c is valid and otherwise the *Invalid carrying its
// field errors (spec §changesets).
func (c Changeset[M]) Err() error {
	if c.Valid() {
		return nil
	}
	return &Invalid{Model: c.d().Model, Fields: c.Errors()}
}

func (c Changeset[M]) marked() (cols []string, args []any) {
	if len(c.set) == 0 {
		return nil, nil
	}
	vals := c.d().Values(c.Row())
	for i, f := range c.d().Fields {
		if c.set[i] {
			cols = append(cols, f.Column)
			args = append(args, vals[i])
		}
	}
	return cols, args
}

// keyUnmarkedRefuse records a not_null field error on an unmarked
// rowid alias that is not the database-assigned key (spec §changesets),
// the one NOT NULL column SQLite fills rather than refuses when an
// insert leaves it out, unless an error on it is recorded already.
func (c Changeset[M]) keyUnmarkedRefuse() Changeset[M] {
	errs := c.Errors()
	for i, f := range c.d().Fields {
		if !f.RowidAlias || f.Assigned || (i < len(c.set) && c.set[i]) {
			continue
		}
		named := false
		for _, e := range errs {
			named = named || slices.Contains(e.Columns, f.Column)
		}
		if !named {
			c = c.errorAdd("not_null", "must not be null", f.Column)
		}
	}
	return c
}

// InsertRender renders the INSERT of the marked columns, DEFAULT VALUES
// when none, RETURNING the row (spec §changesets).
func (c Changeset[M]) InsertRender() (sql string, args []any) {
	cols, args := c.marked()
	return InsertRender(c.d().Table, cols, c.d().Columns), args
}

// UpdateRender renders the UPDATE of the marked columns keyed on the row
// c was made from, RETURNING the row (spec §changesets): "" and no
// arguments when nothing is marked, since there is nothing to write.
func (c Changeset[M]) UpdateRender() (sql string, args []any) {
	cols, args := c.marked()
	if len(cols) == 0 {
		return "", nil
	}
	vals := c.d().Values(c.data)
	var keys []string
	for i, f := range c.d().Fields {
		if f.Key {
			keys = append(keys, f.Column)
			args = append(args, vals[i])
		}
	}
	return UpdateRowRender(c.d().Table, cols, keys, c.d().Columns), args
}
