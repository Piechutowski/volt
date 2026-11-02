package volt

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Piechutowski/volt/nao/rt"
)

// TestRefusedWriteStatus pins where a refused write gets its HTTP status
// (spec §changesets, §formats): the data runtime's *rt.Invalid carries
// none, and the error spine maps a unique field error among its fields to
// 409 and any other field errors to 422, keeping the message and the
// itemized details.
func TestRefusedWriteStatus(t *testing.T) {
	if _, ok := any(&rt.Invalid{}).(HTTPError); ok {
		t.Fatal("*rt.Invalid carries an HTTP status; the network layer decides it")
	}
	field := func(check string, cols ...string) rt.FieldError {
		return rt.FieldError{Check: check, Columns: cols, Message: check + " failed"}
	}
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{"unique", &rt.Invalid{Model: "User", Fields: []rt.FieldError{field("unique", "email")}}, http.StatusConflict},
		{"unique after other errors", &rt.Invalid{Model: "User", Fields: []rt.FieldError{field("required", "name"), field("EmailValid", "email"), field("unique", "email")}}, http.StatusConflict},
		{"unique on no column", &rt.Invalid{Model: "Author", Fields: []rt.FieldError{field("unique")}}, http.StatusConflict},
		{"required", &rt.Invalid{Model: "User", Fields: []rt.FieldError{field("required", "email")}}, http.StatusUnprocessableEntity},
		{"validators", &rt.Invalid{Model: "PageView", Fields: []rt.FieldError{field("CountsPositive", "hits", "day"), field("SiteShape", "site")}}, http.StatusUnprocessableEntity},
		{"unknown field", &rt.Invalid{Model: "Post", Fields: []rt.FieldError{field("unknown_field", "titel")}}, http.StatusUnprocessableEntity},
		{"cast", &rt.Invalid{Model: "Post", Fields: []rt.FieldError{field("cast", "views")}}, http.StatusUnprocessableEntity},
		{"no fields", &rt.Invalid{Model: "Post"}, http.StatusUnprocessableEntity},
		{"wrapped", fmt.Errorf("saving: %w", &rt.Invalid{Model: "User", Fields: []rt.FieldError{field("unique", "email")}}), http.StatusConflict},
		{"driver unique", rt.Refused("User", errors.New("UNIQUE constraint failed: users.email (2067)")), http.StatusConflict},
		{"driver composite unique", rt.Refused("Tag", errors.New("UNIQUE constraint failed: tags.site, tags.name")), http.StatusConflict},
		{"driver expression index", rt.Refused("Author", errors.New("UNIQUE constraint failed: index 'idx_authors_handle_ci'")), http.StatusConflict},
		{"driver not null", rt.Refused("User", errors.New("NOT NULL constraint failed: users.email (1299)")), http.StatusUnprocessableEntity},
		{"driver foreign key", rt.Refused("Order", errors.New("FOREIGN KEY constraint failed (787)")), http.StatusUnprocessableEntity},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var inv *rt.Invalid
			if !errors.As(tc.err, &inv) {
				t.Fatalf("not a refused write: %v", tc.err)
			}
			p, ok := ProblemOf(tc.err)
			if !ok || p.Status != tc.status || p.Message != inv.Error() {
				t.Fatalf("ProblemOf = %+v, %v; want status %d and message %q", p, ok, tc.status, inv.Error())
			}
			if len(p.Details) != len(inv.Fields) {
				t.Fatalf("details %+v, want one per field of %+v", p.Details, inv.Fields)
			}
			for i, f := range inv.Fields {
				d := p.Details[i]
				if d.Check != f.Check || fmt.Sprint(d.Columns) != fmt.Sprint(f.Columns) || d.Message != f.Message {
					t.Errorf("detail %d = %+v, want %+v", i, d, f)
				}
			}

			rec := httptest.NewRecorder()
			Handler("POST /x", func(w http.ResponseWriter, r *Request) error { return tc.err }, nil).
				ServeHTTP(rec, httptest.NewRequest("POST", "/x", nil))
			var body Problem
			if rec.Code != tc.status || json.Unmarshal(rec.Body.Bytes(), &body) != nil || body.Status != tc.status || len(body.Details) != len(inv.Fields) {
				t.Errorf("the default error handler wrote %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestProblemOfPrecedence pins the order the error spine reads an error
// in (spec §formats): an HTTPError keeps its own status even around a
// refused write, a row miss is a 404, and anything else has no Problem.
func TestProblemOfPrecedence(t *testing.T) {
	inv := &rt.Invalid{Model: "User", Fields: []rt.FieldError{{Check: "unique", Columns: []string{"email"}, Message: "already exists"}}}
	if p, ok := ProblemOf(&teapotAround{inv}); !ok || p.Status != http.StatusTeapot || len(p.Details) != 1 || p.Details[0].Check != "unique" {
		t.Errorf("an HTTPError around a refused write = %+v, %v; want its own 418 with the write's details", p, ok)
	}
	if p, ok := ProblemOf(fmt.Errorf("load: %w", rt.ErrNotFound)); !ok || p.Status != http.StatusNotFound {
		t.Errorf("a row miss = %+v, %v; want 404", p, ok)
	}
	if _, ok := ProblemOf(errors.New("disk I/O error")); ok {
		t.Error("an unknown error has no Problem; the default handler answers 500")
	}
}

type teapotAround struct{ err error }

func (e *teapotAround) Error() string   { return "teapot: " + e.err.Error() }
func (e *teapotAround) Unwrap() error   { return e.err }
func (e *teapotAround) StatusCode() int { return http.StatusTeapot }
