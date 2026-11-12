package rt

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

const bindMax = 32766

// Batch inserts many rows of one model in one transaction and yields how
// many it inserted (spec §changesets): its statements in order, each a
// multi-row INSERT without RETURNING, since SQLite returns a multi-row
// insert's rows in no promised order. A Batch whose rows did not all
// validate carries their *InvalidRows in Err and runs nothing; one of no
// rows has no statement and yields 0. Inside a transaction it joins it.
type Batch struct {
	Steps []Exec
	Err   error
}

// InsertMany is the Batch of cs (spec §changesets): every changeset is
// checked as Insert checks one, the field errors of all that fail
// itemized by their index in cs; the rest are grouped, consecutive
// changesets that mark the same columns sharing statements of as many
// rows as SQLite's 32,766 parameters hold, and a changeset that marks
// nothing inserting DEFAULT VALUES alone.
func InsertMany[M any](cs []Changeset[M]) Batch {
	if len(cs) == 0 {
		return Batch{}
	}
	d := cs[0].d()
	var bad []RowInvalid
	for i, c := range cs {
		var inv *Invalid
		if errors.As(c.keyUnmarkedRefuse().Err(), &inv) {
			bad = append(bad, RowInvalid{Index: i, Fields: inv.Fields})
		}
	}
	if len(bad) > 0 {
		return Batch{Err: &InvalidRows{Model: d.Model, Rows: bad}}
	}
	var b Batch
	for i := 0; i < len(cs); {
		cols, args := cs[i].marked()
		if len(cols) == 0 {
			b.Steps = append(b.Steps, Exec{SQL: "INSERT INTO " + d.Table + " DEFAULT VALUES", Model: d.Model})
			i++
			continue
		}
		per := bindMax / len(cols)
		j := i + 1
		for ; j < len(cs) && j-i < per; j++ {
			more, moreArgs := cs[j].marked()
			if !slices.Equal(more, cols) {
				break
			}
			args = append(args, moreArgs...)
		}
		b.Steps = append(b.Steps, Exec{SQL: InsertManyRender(d.Table, cols, j-i), Args: args, Model: d.Model})
		i = j
	}
	return b
}

// InsertManyRender renders the INSERT of rows rows of the columns cols
// with positional placeholders, row by row, and no RETURNING (spec
// §changesets).
func InsertManyRender(table string, cols []string, rows int) string {
	var b strings.Builder
	b.WriteString("INSERT INTO " + table + " (")
	for i, c := range cols {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(Ident(c))
	}
	b.WriteString(") VALUES ")
	row := "(" + strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", ") + ")"
	for i := range rows {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(row)
	}
	return b.String()
}

func (b Batch) perform(ctx context.Context, x Executor) (int64, error) {
	if b.Err != nil {
		return 0, b.Err
	}
	if len(b.Steps) == 0 {
		return 0, nil
	}
	var n int64
	err := txRun(ctx, x, true, func(tx Executor) error {
		for _, s := range b.Steps {
			k, err := s.perform(ctx, tx)
			if err != nil {
				return err
			}
			n += k
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (b Batch) performAny(ctx context.Context, x Executor) (any, error) { return b.perform(ctx, x) }

func (b Batch) describe() description {
	if b.Err != nil {
		return description{kind: "batch error", values: []any{b.Err}, spelled: "batch error: " + b.Err.Error()}
	}
	steps := make([]AnyEffect, len(b.Steps))
	for i, s := range b.Steps {
		steps[i] = s
	}
	return effectsDescribe("batch", steps...)
}

func (b Batch) writes() bool { return true }

// RowInvalid is one refused row of a batch (spec §changesets): its index
// in the changesets given and its field errors.
type RowInvalid struct {
	Index  int
	Fields []FieldError
}

// InvalidRows is the refusal of a batch whose rows did not all validate
// (spec §changesets): every invalid row, in order, by its index.
type InvalidRows struct {
	Model string
	Rows  []RowInvalid
}

func (e *InvalidRows) Error() string {
	idx := make([]string, len(e.Rows))
	parts := make([]string, len(e.Rows))
	for i, r := range e.Rows {
		idx[i] = fmt.Sprint(r.Index)
		fields := make([]string, len(r.Fields))
		for j, f := range r.Fields {
			fields[j] = f.Error()
		}
		parts[i] = fmt.Sprintf("[%d] %s", r.Index, strings.Join(fields, ", "))
	}
	noun := "row"
	if len(e.Rows) > 1 {
		noun = "rows"
	}
	return fmt.Sprintf("%s: %s %s invalid: %s", e.Model, noun, strings.Join(idx, ", "), strings.Join(parts, "; "))
}

// Details itemizes every field error of every invalid row with the row's
// index (the volt Detailer contract).
func (e *InvalidRows) Details() []Detail {
	var out []Detail
	for _, r := range e.Rows {
		for _, f := range r.Fields {
			out = append(out, Detail{Row: &r.Index, Check: f.Check, Columns: slices.Clone(f.Columns), Message: f.Message})
		}
	}
	return out
}
