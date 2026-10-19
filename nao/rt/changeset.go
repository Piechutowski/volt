// Changeset support (docs/spec.md §changesets): the wire form of a write,
// the field reader Cast uses, the field errors a changeset records and the
// one error a refused write returns.
package rt

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// FieldError is one failure a changeset records (spec §changesets): the
// check that failed, the columns it reads and the failure as text.
type FieldError struct {
	Check   string
	Columns []string
	Message string
}

func (e FieldError) Error() string {
	if len(e.Columns) == 0 {
		return e.Message
	}
	return strings.Join(e.Columns, ", ") + ": " + e.Message
}

// Invalid is every refused write (spec §changesets): the field errors of
// an invalid changeset, or the database's refusal mapped onto fields with
// the driver's error as Cause.
type Invalid struct {
	Model  string
	Fields []FieldError
	Cause  error
}

func (e *Invalid) Error() string {
	parts := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		parts[i] = f.Error()
	}
	return e.Model + ": " + strings.Join(parts, "; ")
}

func (e *Invalid) Unwrap() error { return e.Cause }

// StatusCode is 409 (Conflict) when a unique violation is among the
// fields and 422 (Unprocessable Content) otherwise.
func (e *Invalid) StatusCode() int {
	for _, f := range e.Fields {
		if f.Check == "unique" {
			return 409
		}
	}
	return 422
}

// Details itemizes every field error (the volt Detailer contract).
func (e *Invalid) Details() []Detail {
	out := make([]Detail, len(e.Fields))
	for i, f := range e.Fields {
		out[i] = Detail{Check: f.Check, Columns: f.Columns, Message: f.Message}
	}
	return out
}

// Refused translates a driver error into an *Invalid (spec §changesets):
// "UNIQUE constraint failed: t.a, t.b" is a unique field error on a and
// b, "NOT NULL constraint failed: t.a" a not_null one on a, "FOREIGN KEY
// constraint failed" a foreign_key one with no column. Any other error is
// returned unchanged.
func Refused(model string, err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	for _, k := range [...]struct{ sqlite, check, message string }{
		{"UNIQUE", "unique", "already exists"},
		{"NOT NULL", "not_null", "must not be null"},
		{"FOREIGN KEY", "foreign_key", "references a row that does not exist"},
	} {
		prefix := k.sqlite + " constraint failed"
		i := strings.Index(msg, prefix)
		if i < 0 {
			continue
		}
		f := FieldError{Check: k.check, Message: k.message}
		detail := strings.TrimSpace(strings.TrimPrefix(msg[i+len(prefix):], ":"))
		if detail != "" && k.check != "foreign_key" {
			for _, c := range strings.Split(detail, ",") {
				c = strings.TrimSpace(c)
				if j := strings.LastIndexByte(c, '.'); j >= 0 {
					c = c[j+1:]
				}
				f.Columns = append(f.Columns, c)
			}
		}
		return &Invalid{Model: model, Fields: []FieldError{f}, Cause: err}
	}
	return err
}

// ErrNoRow is Update on a changeset that was not made from a row (spec
// §changesets): there is no key to update by.
var ErrNoRow = errors.New("rt: the changeset was not made from a row")

// ColumnOf is a column handle of model M, the element of a Cast allow
// list (spec §changesets). Only the handles of this package implement it.
type ColumnOf[M any] interface {
	ColumnName() string
	columnOf(M)
}

// ColumnName is the SQL column name, unquoted.
func (c Column[M, T]) ColumnName() string { return c.Name }

func (Column[M, T]) columnOf(M) {}

// Params is a write body as the wire carries it (spec §changesets): as
// JSON an object of the changed columns keyed by wire name, as GOB the
// row and the list of marked wire names. A changeset's Params builds
// one, volt.Decode reads one, Cast consumes one.
type Params[M any] struct {
	fields map[string]json.RawMessage
	row    *M
	set    []string
}

// ParamsOf builds the Params a changeset sends: the row and the wire
// names it marked.
func ParamsOf[M any](row M, set []string) Params[M] {
	return Params[M]{row: &row, set: slices.Clone(set)}
}

type paramsWire[M any] struct {
	Row M
	Set []string
}

// UnmarshalJSON reads a JSON object; anything else is an error.
func (p *Params[M]) UnmarshalJSON(b []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	if m == nil {
		return errors.New("a write body is a JSON object")
	}
	p.fields, p.row, p.set = m, nil, nil
	return nil
}

// MarshalJSON writes the object of the marked columns by wire name.
func (p Params[M]) MarshalJSON() ([]byte, error) {
	if p.row == nil {
		if p.fields == nil {
			return []byte("{}"), nil
		}
		return json.Marshal(p.fields)
	}
	all, err := json.Marshal(*p.row)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(all, &m); err != nil {
		return nil, err
	}
	out := make(map[string]json.RawMessage, len(p.set))
	for _, w := range p.set {
		if v, ok := m[w]; ok {
			out[w] = v
		}
	}
	return json.Marshal(out)
}

// GobEncode writes the row and the marked wire names; a Params read from
// JSON is converted through the row first.
func (p Params[M]) GobEncode() ([]byte, error) {
	w := paramsWire[M]{Set: p.set}
	switch {
	case p.row != nil:
		w.Row = *p.row
	case p.fields != nil:
		b, err := json.Marshal(p.fields)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &w.Row); err != nil {
			return nil, err
		}
		w.Set = p.Keys()
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(w); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GobDecode reads what GobEncode wrote.
func (p *Params[M]) GobDecode(b []byte) error {
	var w paramsWire[M]
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&w); err != nil {
		return err
	}
	p.fields, p.row, p.set = nil, &w.Row, w.Set
	return nil
}

// Has reports whether p carries the wire name.
func (p Params[M]) Has(wire string) bool {
	if p.row != nil {
		return slices.Contains(p.set, wire)
	}
	_, ok := p.fields[wire]
	return ok
}

// Keys lists the wire names p carries, sorted.
func (p Params[M]) Keys() []string {
	var keys []string
	if p.row != nil {
		keys = slices.Clone(p.set)
	} else {
		for k := range p.fields {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// Field reads one column of p (spec §changesets): present is false when p
// does not carry the wire name; a JSON value that does not convert to T,
// or null when nullable is false, is an error naming the wire name. get
// reads the column from the row of a GOB body.
func Field[M, T any](p Params[M], wire string, nullable bool, get func(*M) T) (v T, present bool, err error) {
	if p.row != nil {
		if !slices.Contains(p.set, wire) {
			return v, false, nil
		}
		return get(p.row), true, nil
	}
	raw, ok := p.fields[wire]
	if !ok {
		return v, false, nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), jsonNull) {
		if !nullable {
			return v, true, fmt.Errorf("%s cannot be null", wire)
		}
		return v, true, nil
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, true, fmt.Errorf("%s cannot be read as %T", wire, v)
	}
	return v, true, nil
}

// UnknownFields is one unknown_field error per wire name p carries that
// is none of wires, in sorted order (spec §changesets).
func UnknownFields[M any](p Params[M], wires ...string) []FieldError {
	var out []FieldError
	for _, k := range p.Keys() {
		if !slices.Contains(wires, k) {
			out = append(out, FieldError{Check: "unknown_field", Columns: []string{k}, Message: "unknown field " + strconv.Quote(k)})
		}
	}
	return out
}

// FieldErrorsAppend appends to a changeset's errors without sharing the
// backing array with the changeset it was copied from, so a changeset
// value stays immutable to its holder (spec §changesets).
func FieldErrorsAppend(errs []FieldError, more ...FieldError) []FieldError {
	if len(more) == 0 {
		return errs
	}
	return append(slices.Clip(errs), more...)
}

// Blank reports whether v is the empty value of its storage type, read
// through driver.Valuer when v implements it (spec §changesets): 0, "",
// a zero length, NULL or the zero time.
func Blank(v any) bool {
	if dv, ok := v.(driver.Valuer); ok {
		val, err := dv.Value()
		if err != nil {
			return false
		}
		v = val
	}
	switch x := v.(type) {
	case nil:
		return true
	case int64:
		return x == 0
	case int:
		return x == 0
	case float64:
		return x == 0
	case string:
		return x == ""
	case []byte:
		return len(x) == 0
	case time.Time:
		return x.IsZero()
	}
	return false
}

// InsertRender renders a changeset's INSERT (spec §changesets): the marked
// columns with positional placeholders, DEFAULT VALUES when none, and
// RETURNING the row's columns.
func InsertRender(table string, cols []string, returning string) string {
	if len(cols) == 0 {
		return "INSERT INTO " + table + " DEFAULT VALUES RETURNING " + returning
	}
	var b strings.Builder
	b.WriteString("INSERT INTO " + table + " (")
	for i, c := range cols {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(identQuote(c))
	}
	b.WriteString(") VALUES (")
	for i := range cols {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteByte('?')
	}
	b.WriteString(") RETURNING " + returning)
	return b.String()
}

// UpdateRowRender renders a changeset's UPDATE (spec §changesets): the
// marked columns, the key columns as the WHERE, both with positional
// placeholders in that order, and RETURNING the row's columns.
func UpdateRowRender(table string, set, key []string, returning string) string {
	var b strings.Builder
	b.WriteString("UPDATE " + table + " SET ")
	for i, c := range set {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(identQuote(c) + " = ?")
	}
	b.WriteString(" WHERE ")
	for i, c := range key {
		if i > 0 {
			b.WriteString(" AND ")
		}
		b.WriteString(identQuote(c) + " = ?")
	}
	b.WriteString(" RETURNING " + returning)
	return b.String()
}

// Row is what a generated scan function reads: *sql.Row and *sql.Rows
// both satisfy it.
type Row interface {
	Scan(dest ...any) error
}

// StmtQueryRow runs a single-row statement on db, through cache when one
// is given (nil runs directly).
func StmtQueryRow(ctx context.Context, db DBTX, cache *StmtCache, query string, args ...any) (Row, error) {
	if cache != nil {
		stmt, err := cache.Prepare(ctx, db, query)
		if err != nil {
			return nil, err
		}
		return stmt.QueryRowContext(ctx, args...), nil
	}
	return db.QueryRowContext(ctx, query, args...), nil
}

var _ Row = (*sql.Row)(nil)
