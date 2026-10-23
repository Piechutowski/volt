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

// Detail is the itemized form the volt runtime renders into an error
// body: the check, its columns, the message. It aliases the same unnamed
// struct the volt runtime aliases, so Details() satisfies volt's Detailer
// interface without either package importing the other.
type Detail = struct {
	Check   string   `json:"check"`
	Columns []string `json:"columns,omitempty"`
	Message string   `json:"message"`
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
// constraint failed" a foreign_key one with no column; a trailing result
// code such as " (2067)" is not part of the last column. A unique index
// over an expression is named as "index 'idx_...'" and constrains no
// column, so its unique field error has none. Any other error is
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
		detail := resultCodeTrim(strings.TrimSpace(strings.TrimPrefix(msg[i+len(prefix):], ":")))
		if detail != "" && k.check != "foreign_key" && !strings.HasPrefix(detail, "index '") {
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

func resultCodeTrim(detail string) string {
	if !strings.HasSuffix(detail, ")") {
		return detail
	}
	i := strings.LastIndex(detail, " (")
	if i < 0 {
		return detail
	}
	code := detail[i+2 : len(detail)-1]
	if code == "" {
		return detail
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return detail
		}
	}
	return detail[:i]
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
// row and the marked columns, typed by the shared model. A changeset's
// Params builds one from its marks, volt.Decode reads one, Cast consumes
// one; a body keeps its arm through a GOB round trip.
type Params[M any] struct {
	fields map[string]json.RawMessage
	row    *M
	set    []string
	marks  []Mark
}

// Mark is one marked column as a changeset hands it to ParamsOf (spec
// §changesets): the wire name and the value set.
type Mark struct {
	Wire  string
	Value any
}

// ParamsOf builds the Params a changeset sends: the row and one Mark per
// marked column.
func ParamsOf[M any](row M, marks ...Mark) Params[M] {
	set := make([]string, len(marks))
	for i, m := range marks {
		set[i] = m.Wire
	}
	return Params[M]{row: &row, set: set, marks: slices.Clone(marks)}
}

type paramsWire[M any] struct {
	Row    M
	Set    []string
	Fields map[string]json.RawMessage
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
	*p = Params[M]{fields: m}
	return nil
}

// MarshalJSON writes the object of the marked columns by wire name, each
// value as encoding/json renders it: an rt.Null as the value or null,
// []byte as base64 and nil as "" (spec §columns), time.Time as RFC 3339.
func (p Params[M]) MarshalJSON() ([]byte, error) {
	fields := p.fields
	if p.marks != nil {
		var err error
		if fields, err = marksJSON(p.marks); err != nil {
			return nil, err
		}
	}
	if fields == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(fields)
}

func marksJSON(marks []Mark) (map[string]json.RawMessage, error) {
	out := make(map[string]json.RawMessage, len(marks))
	for _, m := range marks {
		v, err := json.Marshal(bytesWrap(m.Value))
		if err != nil {
			return nil, err
		}
		out[m.Wire] = v
	}
	return out, nil
}

// GobEncode writes the arm p holds: the row, the marked wire names and
// the marks' JSON for a Params a changeset built, the JSON object alone
// for one read from JSON, so Field and MarshalJSON answer the same on
// both sides of the trip.
func (p Params[M]) GobEncode() ([]byte, error) {
	w := paramsWire[M]{Fields: p.fields}
	if p.row != nil {
		w.Row, w.Set = *p.row, p.set
		if p.marks != nil {
			fields, err := marksJSON(p.marks)
			if err != nil {
				return nil, err
			}
			w.Fields = fields
		}
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(w); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GobDecode reads what GobEncode wrote; a body with no marked column is
// the empty JSON object either way.
func (p *Params[M]) GobDecode(b []byte) error {
	var w paramsWire[M]
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&w); err != nil {
		return err
	}
	*p = Params[M]{fields: w.Fields}
	if len(w.Set) > 0 {
		p.row, p.set = &w.Row, w.Set
	}
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
// does not carry the wire name; a JSON value that does not convert to T
// is the error "<wire>: expected <typ>", null when nullable is false
// "<wire>: null is not allowed (<typ>)", typ being the column's type as
// the schema spells it. get reads the column from the row of a GOB body.
func Field[M, T any](p Params[M], wire, typ string, nullable bool, get func(*M) T) (v T, present bool, err error) {
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
			return v, true, fmt.Errorf("%s: null is not allowed (%s)", wire, typ)
		}
		return v, true, nil
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, true, fmt.Errorf("%s: expected %s", wire, typ)
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
		b.WriteString(Ident(c))
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
		b.WriteString(Ident(c) + " = ?")
	}
	b.WriteString(" WHERE ")
	for i, c := range key {
		if i > 0 {
			b.WriteString(" AND ")
		}
		b.WriteString(Ident(c) + " = ?")
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
