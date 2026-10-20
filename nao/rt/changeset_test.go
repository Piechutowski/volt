package rt

import (
	"bytes"
	"database/sql/driver"
	"encoding/gob"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type post struct {
	ID    int          `db:"id" json:"id"`
	Title string       `db:"title" json:"title"`
	Body  Null[string] `db:"body" json:"body"`
	Views int          `db:"views" json:"hits"`
}

type cents int

func (c cents) Value() (driver.Value, error) { return int64(c), nil }

func TestRefused(t *testing.T) {
	cases := []struct {
		msg, check string
		columns    []string
		status     int
	}{
		{"UNIQUE constraint failed: users.email", "unique", []string{"email"}, 409},
		{"UNIQUE constraint failed: tags.site, tags.name", "unique", []string{"site", "name"}, 409},
		{"NOT NULL constraint failed: users.email", "not_null", []string{"email"}, 422},
		{"FOREIGN KEY constraint failed", "foreign_key", nil, 422},
		{"UNIQUE constraint failed: users.email (2067)", "unique", []string{"email"}, 409},
		{"UNIQUE constraint failed: tags.site, tags.name (2067)", "unique", []string{"site", "name"}, 409},
		{"NOT NULL constraint failed: users.email (1299)", "not_null", []string{"email"}, 422},
		{"FOREIGN KEY constraint failed (787)", "foreign_key", nil, 422},
		{"UNIQUE constraint failed: users.name (x)", "unique", []string{"name (x)"}, 409},
	}
	for _, tc := range cases {
		cause := errors.New(tc.msg)
		err := Refused("User", cause)
		var inv *Invalid
		if !errors.As(err, &inv) {
			t.Fatalf("%q: not an *Invalid: %v", tc.msg, err)
		}
		if !errors.Is(err, cause) {
			t.Errorf("%q: must unwrap to the driver error", tc.msg)
		}
		if len(inv.Fields) != 1 || inv.Fields[0].Check != tc.check || !reflect.DeepEqual(inv.Fields[0].Columns, tc.columns) || inv.StatusCode() != tc.status {
			t.Errorf("%q → %+v (status %d)", tc.msg, inv.Fields, inv.StatusCode())
		}
		if d := inv.Details(); len(d) != 1 || d[0].Check != tc.check || d[0].Message == "" {
			t.Errorf("%q: details %+v", tc.msg, d)
		}
	}
	plain := errors.New("disk I/O error")
	if Refused("User", plain) != plain {
		t.Error("a non-constraint error must pass through unchanged")
	}
	if Refused("User", nil) != nil {
		t.Error("nil must stay nil")
	}
}

func TestInvalidShape(t *testing.T) {
	inv := &Invalid{Model: "Post", Fields: []FieldError{
		{Check: "required", Columns: []string{"title"}, Message: "title is required"},
		{Check: "unknown_field", Columns: []string{"titel"}, Message: `unknown field "titel"`},
	}}
	if inv.StatusCode() != 422 {
		t.Errorf("status %d, want 422", inv.StatusCode())
	}
	if !strings.Contains(inv.Error(), "title: title is required; titel: unknown field") {
		t.Errorf("message %q", inv.Error())
	}
	if d := inv.Details(); len(d) != 2 || d[1].Columns[0] != "titel" {
		t.Errorf("details %+v", d)
	}
}

func TestParamsJSON(t *testing.T) {
	var p Params[post]
	if err := json.Unmarshal([]byte(`{"title": "Hi", "body": null, "hits": 3, "extra": 1}`), &p); err != nil {
		t.Fatal(err)
	}
	if !p.Has("title") || p.Has("id") {
		t.Error("Has must follow the keys carried")
	}
	if got := UnknownFields(p, "id", "title", "body", "hits"); len(got) != 1 || got[0].Check != "unknown_field" || got[0].Columns[0] != "extra" || got[0].Message != `unknown field "extra"` {
		t.Errorf("UnknownFields = %+v", got)
	}
	title, ok, err := Field(p, "title", "string", false, func(r *post) string { return r.Title })
	if err != nil || !ok || title != "Hi" {
		t.Errorf("title = %q, %v, %v", title, ok, err)
	}
	body, ok, err := Field(p, "body", "string", true, func(r *post) Null[string] { return r.Body })
	if err != nil || !ok || body.Valid {
		t.Errorf("body = %+v, %v, %v; want present NULL", body, ok, err)
	}
	if _, ok, _ := Field(p, "id", "int", false, func(r *post) int { return r.ID }); ok {
		t.Error("an absent key is not present")
	}
	var bad Params[post]
	if err := json.Unmarshal([]byte(`{"title": null, "hits": 1.5, "body": 1}`), &bad); err != nil {
		t.Fatal(err)
	}
	fieldErrorsCheck(t, bad)
	for _, in := range []string{`[1]`, `null`, `"x"`} {
		var q Params[post]
		if err := json.Unmarshal([]byte(in), &q); err == nil {
			t.Errorf("%s must not decode as a write body", in)
		}
	}
	out, err := json.Marshal(ParamsOf(post{ID: 7, Title: "Hi", Views: 2}, Mark{"title", "Hi"}, Mark{"hits", 2}))
	if err != nil || string(out) != `{"hits":2,"title":"Hi"}` {
		t.Errorf("MarshalJSON = %s, %v", out, err)
	}
	back, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var again Params[post]
	if err := json.Unmarshal(back, &again); err != nil || !reflect.DeepEqual(again.Keys(), p.Keys()) {
		t.Errorf("a JSON body re-encodes to the same keys: %s, %v", back, err)
	}
}

// fieldErrorsCheck pins the JSON arm's cast errors on a body of
// {"title": null, "hits": 1.5, "body": 1}: the key as sent and the
// column's type, whichever wire carried the body.
func fieldErrorsCheck(t *testing.T, p Params[post]) {
	t.Helper()
	if _, _, err := Field(p, "title", "string", false, func(r *post) string { return r.Title }); err == nil || err.Error() != "title: null is not allowed (string)" {
		t.Errorf("null on a non-nullable column: %v", err)
	}
	if _, _, err := Field(p, "hits", "int", false, func(r *post) int { return r.Views }); err == nil || err.Error() != "hits: expected int" {
		t.Errorf("1.5 into int: %v", err)
	}
	if _, _, err := Field(p, "body", "string", true, func(r *post) Null[string] { return r.Body }); err == nil || err.Error() != "body: expected string" {
		t.Errorf("1 into a nullable string: %v", err)
	}
}

type tagged struct {
	ID     int          `json:"id"`
	Hits   int          `json:"hits,omitempty"`
	Secret string       `json:"-"`
	Note   Null[string] `json:"note"`
	Raw    []byte       `json:"raw"`
	At     time.Time    `json:"at"`
}

func TestParamsMarks(t *testing.T) {
	when := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	p := ParamsOf(tagged{}, Mark{"hits", 0}, Mark{"secret", ""}, Mark{"note", Null[string]{}}, Mark{"raw", []byte{1, 2}}, Mark{"at", when})
	out, err := json.Marshal(p)
	if err != nil || string(out) != `{"at":"2024-01-15T10:30:00Z","hits":0,"note":null,"raw":"AQI=","secret":""}` {
		t.Errorf("MarshalJSON renders each mark's value under its wire name, omitempty and - notwithstanding: %s, %v", out, err)
	}
	if !reflect.DeepEqual(p.Keys(), []string{"at", "hits", "note", "raw", "secret"}) || !p.Has("secret") || p.Has("id") {
		t.Errorf("Keys = %v", p.Keys())
	}
	if hits, ok, err := Field(p, "hits", "int", false, func(r *tagged) int { return r.Hits }); err != nil || !ok || hits != 0 {
		t.Errorf("hits = %d, %v, %v", hits, ok, err)
	}
	if none, err := json.Marshal(ParamsOf(tagged{ID: 3})); err != nil || string(none) != `{}` {
		t.Errorf("no marks is the empty object: %s, %v", none, err)
	}
}

func gobTrip[M any](t *testing.T, in Params[M]) Params[M] {
	t.Helper()
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(in); err != nil {
		t.Fatal(err)
	}
	var out Params[M]
	if err := gob.NewDecoder(&buf).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestParamsGob(t *testing.T) {
	when := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	type stamped struct {
		ID int
		At time.Time
		Bs []byte
	}
	out := gobTrip(t, ParamsOf(stamped{ID: 1, At: when, Bs: []byte{1, 2}}, Mark{"at", when}, Mark{"bs", []byte{1, 2}}))
	if !out.Has("at") || out.Has("id") {
		t.Errorf("marks lost: %v", out.Keys())
	}
	at, ok, err := Field(out, "at", "time.Time", false, func(r *stamped) time.Time { return r.At })
	if err != nil || !ok || !at.Equal(when) {
		t.Errorf("at = %v, %v, %v", at, ok, err)
	}
	if _, ok, _ := Field(out, "id", "int", false, func(r *stamped) int { return r.ID }); ok {
		t.Error("an unmarked column is not present")
	}
	if doc, err := json.Marshal(out); err != nil || string(doc) != `{"at":"2024-01-15T10:30:00Z","bs":"AQI="}` {
		t.Errorf("the marks' JSON survives the trip: %s, %v", doc, err)
	}
	if none := gobTrip(t, ParamsOf(stamped{ID: 1})); none.Has("id") || len(none.Keys()) != 0 {
		t.Errorf("nothing marked stays nothing: %v", none.Keys())
	}

	var fromJSON Params[post]
	if err := json.Unmarshal([]byte(`{"title": "Hi", "extra": 1}`), &fromJSON); err != nil {
		t.Fatal(err)
	}
	viaGob := gobTrip(t, fromJSON)
	if title, ok, err := Field(viaGob, "title", "string", false, func(r *post) string { return r.Title }); err != nil || !ok || title != "Hi" {
		t.Errorf("a JSON body reads the same after GOB: %q, %v, %v", title, ok, err)
	}
	if !reflect.DeepEqual(viaGob.Keys(), fromJSON.Keys()) || len(UnknownFields(viaGob, "title")) != 1 {
		t.Errorf("a JSON body keeps its keys after GOB: %v", viaGob.Keys())
	}
	var bad Params[post]
	if err := json.Unmarshal([]byte(`{"title": null, "hits": 1.5, "body": 1}`), &bad); err != nil {
		t.Fatal(err)
	}
	fieldErrorsCheck(t, gobTrip(t, bad))
	if empty := gobTrip(t, paramsOfJSON(t, `{}`)); len(empty.Keys()) != 0 {
		t.Errorf("an empty object stays empty: %v", empty.Keys())
	}
}

func paramsOfJSON(t *testing.T, doc string) Params[post] {
	t.Helper()
	var p Params[post]
	if err := json.Unmarshal([]byte(doc), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestBlank(t *testing.T) {
	for _, v := range []any{nil, 0, int64(0), 0.0, "", []byte{}, time.Time{}, cents(0), Null[int]{}, Some(0), Some("")} {
		if !Blank(v) {
			t.Errorf("%#v should be blank", v)
		}
	}
	for _, v := range []any{1, int64(2), 0.5, "x", []byte{0}, time.Now(), cents(3), Some(5), Some("x"), true, false} {
		if Blank(v) {
			t.Errorf("%#v should not be blank", v)
		}
	}
}

func TestChangesetRenderers(t *testing.T) {
	cols := `"id", "title"`
	if got := InsertRender(`"posts"`, nil, cols); got != `INSERT INTO "posts" DEFAULT VALUES RETURNING "id", "title"` {
		t.Errorf("InsertRender(none) = %s", got)
	}
	if got := InsertRender(`"posts"`, []string{"title", "body"}, cols); got != `INSERT INTO "posts" ("title", "body") VALUES (?, ?) RETURNING "id", "title"` {
		t.Errorf("InsertRender = %s", got)
	}
	if got := UpdateRowRender(`"posts"`, []string{"title"}, []string{"id"}, cols); got != `UPDATE "posts" SET "title" = ? WHERE "id" = ? RETURNING "id", "title"` {
		t.Errorf("UpdateRowRender = %s", got)
	}
	if got := UpdateRowRender(`"t"`, []string{"a", "b"}, []string{"k1", "k2"}, `"k1"`); got != `UPDATE "t" SET "a" = ?, "b" = ? WHERE "k1" = ? AND "k2" = ? RETURNING "k1"` {
		t.Errorf("UpdateRowRender(composite) = %s", got)
	}
}

func TestColumnOf(t *testing.T) {
	var allow []ColumnOf[post]
	allow = append(allow, Column[post, string]{Name: "title"}, NullColumn[post, string]{Column[post, string]{Name: "body"}})
	if allow[0].ColumnName() != "title" || allow[1].ColumnName() != "body" {
		t.Errorf("handles name their columns: %v", allow)
	}
}

func TestFieldErrorsAppendDoesNotAlias(t *testing.T) {
	base := FieldErrorsAppend(make([]FieldError, 0, 4), FieldError{Check: "a"})
	one := FieldErrorsAppend(base, FieldError{Check: "b"})
	two := FieldErrorsAppend(base, FieldError{Check: "c"})
	if one[1].Check != "b" || two[1].Check != "c" || len(base) != 1 {
		t.Errorf("appends from one changeset must not overwrite each other: %v %v %v", base, one, two)
	}
}
