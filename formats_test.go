package volt

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type row struct {
	ID   int32
	Name string
}

func serve(t *testing.T, h HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	Handler("GET /x", h, nil).ServeHTTP(rec, req)
	return rec
}

func TestNegotiate(t *testing.T) {
	cases := []struct {
		accept string
		want   Format
		ok     bool
	}{
		{"", FormatJSON, true},
		{"*/*", FormatJSON, true},
		{"application/json", FormatJSON, true},
		{"application/json; charset=utf-8", FormatJSON, true},
		{"application/x-gob", FormatGOB, true},
		{"text/html, application/x-gob;q=0.9", FormatGOB, true},
		{"text/html", FormatJSON, false},
		{"application/*", FormatJSON, true},
	}
	for _, tc := range cases {
		req := httptest.NewRequest("GET", "/x", nil)
		if tc.accept != "" {
			req.Header.Set("Accept", tc.accept)
		}
		got, ok := Negotiate(req)
		if got != tc.want || ok != tc.ok {
			t.Errorf("Accept %q: got (%v, %v), want (%v, %v)", tc.accept, got, ok, tc.want, tc.ok)
		}
	}
}

func TestRenderJSONAndGOB(t *testing.T) {
	rows := []row{{1, "a"}, {2, "b"}}
	h := func(w http.ResponseWriter, r *Request) error { return Render(w, r, rows) }

	rec := serve(t, h, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 200 || rec.Header().Get("Content-Type") != MIMEJSON {
		t.Fatalf("json: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var gotJSON []row
	if err := json.Unmarshal(rec.Body.Bytes(), &gotJSON); err != nil || len(gotJSON) != 2 || gotJSON[1].Name != "b" {
		t.Fatalf("json body = %q (%v)", rec.Body.String(), err)
	}

	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Accept", MIMEGOB)
	rec = serve(t, h, req)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != MIMEGOB {
		t.Fatalf("gob: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var gotGOB []row
	if err := gob.NewDecoder(rec.Body).Decode(&gotGOB); err != nil || len(gotGOB) != 2 || gotGOB[0].ID != 1 {
		t.Fatalf("gob body decode: %+v (%v)", gotGOB, err)
	}

	req = httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Accept", "text/html")
	rec = serve(t, h, req)
	if rec.Code != http.StatusNotAcceptable {
		t.Fatalf("unacceptable: %d, want 406", rec.Code)
	}

	rec = serve(t, func(w http.ResponseWriter, r *Request) error { return RenderStatus(w, r, 204, nil) },
		httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 204 || rec.Body.Len() != 0 {
		t.Fatalf("nil render: %d %q", rec.Code, rec.Body.String())
	}
}

func TestDecodeByContentType(t *testing.T) {
	decode := func(w http.ResponseWriter, r *Request) error {
		var v row
		if err := Decode(r, &v); err != nil {
			return err
		}
		fmt.Fprint(w, v.Name)
		return nil
	}
	req := httptest.NewRequest("POST", "/x", strings.NewReader(`{"ID":1,"Name":"json"}`))
	if rec := serve(t, decode, req); rec.Code != 200 || rec.Body.String() != "json" {
		t.Fatalf("json (no header): %d %q", rec.Code, rec.Body.String())
	}

	var buf bytes.Buffer
	gob.NewEncoder(&buf).Encode(row{2, "gob"})
	req = httptest.NewRequest("POST", "/x", &buf)
	req.Header.Set("Content-Type", MIMEGOB)
	if rec := serve(t, decode, req); rec.Code != 200 || rec.Body.String() != "gob" {
		t.Fatalf("gob: %d %q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest("POST", "/x", strings.NewReader("x"))
	req.Header.Set("Content-Type", "text/plain")
	if rec := serve(t, decode, req); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("unsupported: %d, want 415", rec.Code)
	}

	req = httptest.NewRequest("POST", "/x", strings.NewReader(""))
	if rec := serve(t, decode, req); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "empty body") {
		t.Fatalf("empty: %d %q, want 400 naming the empty body", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest("POST", "/x", strings.NewReader("{bad"))
	if rec := serve(t, decode, req); rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed: %d, want 400", rec.Code)
	}
}

func TestQueryBinding(t *testing.T) {
	req := httptest.NewRequest("GET", "/x?rok=2024&idgr=a&idgr=b&flag=true&at=2024-01-02T03:04:05Z&f=1.5", nil)
	h := func(w http.ResponseWriter, r *Request) error {
		rok, err := QueryParam[int32](r, "rok")
		if err != nil || rok != 2024 {
			t.Errorf("rok = %d, %v", rok, err)
		}
		idgr, err := QueryParams[string](r, "idgr")
		if err != nil || len(idgr) != 2 || idgr[1] != "b" {
			t.Errorf("idgr = %v, %v", idgr, err)
		}
		flag, err := QueryParam[bool](r, "flag")
		if err != nil || !flag {
			t.Errorf("flag = %v, %v", flag, err)
		}
		at, err := QueryParam[time.Time](r, "at")
		if err != nil || at.Year() != 2024 || at.Hour() != 3 {
			t.Errorf("at = %v, %v", at, err)
		}
		f, err := QueryParam[float64](r, "f")
		if err != nil || f != 1.5 {
			t.Errorf("f = %v, %v", f, err)
		}
		none, err := QueryParams[int64](r, "none")
		if err != nil || len(none) != 0 {
			t.Errorf("absent list = %v, %v; want empty, nil", none, err)
		}
		if _, err := QueryParam[int32](r, "missing"); err == nil || !strings.Contains(err.Error(), "missing") {
			t.Errorf("missing scalar should be a 400 naming it: %v", err)
		}
		if _, err := QueryParam[int32](r, "idgr"); err == nil || !strings.Contains(err.Error(), "idgr") {
			t.Errorf("unparseable scalar should be a 400 naming it: %v", err)
		}
		return nil
	}
	serve(t, h, req)

	// A bad value is a 400 through the spine, with the parameter named.
	rec := serve(t, func(w http.ResponseWriter, r *Request) error {
		_, err := QueryParam[int](r, "n")
		return err
	}, httptest.NewRequest("GET", "/x?n=abc", nil))
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "n") {
		t.Fatalf("bad value: %d %q", rec.Code, rec.Body.String())
	}
}

func TestNoRowsIs404(t *testing.T) {
	rec := serve(t, func(w http.ResponseWriter, r *Request) error {
		return fmt.Errorf("get: %w", sql.ErrNoRows)
	}, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 404 {
		t.Fatalf("sql.ErrNoRows: %d, want 404", rec.Code)
	}
}

func TestFormatQueryRoundTrip(t *testing.T) {
	at := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	// Built the way a generated client builds it: FormatQuery values
	// through volt.Query, which escapes them.
	req := httptest.NewRequest("GET", URL("/x",
		Query("s", FormatQuery("a b")), Query("i", FormatQuery(int32(-7))), Query("u", FormatQuery(uint64(9))),
		Query("f", FormatQuery(1.25)), Query("b", FormatQuery(true)), Query("t", FormatQuery(at))), nil)
	r := &Request{Request: req}
	if v, _ := QueryParam[string](r, "s"); v != "a b" {
		t.Errorf("s = %q", v)
	}
	if v, _ := QueryParam[int32](r, "i"); v != -7 {
		t.Errorf("i = %d", v)
	}
	if v, _ := QueryParam[uint64](r, "u"); v != 9 {
		t.Errorf("u = %d", v)
	}
	if v, _ := QueryParam[float64](r, "f"); v != 1.25 {
		t.Errorf("f = %v", v)
	}
	if v, _ := QueryParam[bool](r, "b"); !v {
		t.Errorf("b = %v", v)
	}
	if v, _ := QueryParam[time.Time](r, "t"); !v.Equal(at) {
		t.Errorf("t = %v", v)
	}
}

func TestClientDoAndErrors(t *testing.T) {
	srv := httptest.NewServer(Handler("GET /x", func(w http.ResponseWriter, r *Request) error {
		switch r.URL.Path {
		case "/rows":
			var in row
			if r.Method == "POST" {
				if err := Decode(r, &in); err != nil {
					return err
				}
				return RenderStatus(w, r, 201, in)
			}
			return Render(w, r, []row{{1, "a"}})
		case "/gone":
			return ErrNotFound
		case "/none":
			return RenderStatus(w, r, 204, nil)
		}
		return Error(418, "teapot")
	}, nil))
	defer srv.Close()

	for _, f := range []Format{FormatJSON, FormatGOB} {
		c := &Client{Base: srv.URL, Format: f}
		var rows []row
		if err := c.Do(context.Background(), "GET", "/rows", nil, &rows); err != nil || len(rows) != 1 || rows[0].Name != "a" {
			t.Fatalf("%v: rows = %+v, %v", f, rows, err)
		}
		var made row
		if err := c.Do(context.Background(), "POST", "/rows", row{2, "b"}, &made); err != nil || made.ID != 2 {
			t.Fatalf("%v: create = %+v, %v", f, made, err)
		}
		if err := c.Do(context.Background(), "GET", "/none", nil, &made); err != nil {
			t.Fatalf("%v: 204 = %v", f, err)
		}
		err := c.Do(context.Background(), "GET", "/gone", nil, &made)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("%v: 404 should match ErrNotFound: %v", f, err)
		}
		var he HTTPError
		if err := c.Do(context.Background(), "GET", "/tea", nil, nil); !errors.As(err, &he) || he.StatusCode() != 418 || !strings.Contains(he.Error(), "teapot") {
			t.Fatalf("%v: 418 = %v", f, err)
		}
		resp, err := c.Raw(context.Background(), "GET", "/rows")
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("%v: raw = %v %v", f, resp, err)
		}
		resp.Body.Close()
	}
}

func TestDecodeStrictAndBounded(t *testing.T) {
	decode := func(w http.ResponseWriter, r *Request) error {
		var v row
		if err := Decode(r, &v); err != nil {
			return err
		}
		return Render(w, r, v)
	}
	// An unknown JSON field is a 400 naming it: a typo must not vanish.
	rec := serve(t, decode, httptest.NewRequest("POST", "/x", strings.NewReader(`{"ID":1,"Nmae":"typo"}`)))
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "Nmae") {
		t.Fatalf("unknown field: %d %q", rec.Code, rec.Body.String())
	}
	// Over the cap is a 413, and the cap is settable.
	old := MaxBodyBytes
	MaxBodyBytes = 16
	defer func() { MaxBodyBytes = old }()
	rec = serve(t, decode, httptest.NewRequest("POST", "/x", strings.NewReader(`{"ID":1,"Name":"a name longer than sixteen bytes"}`)))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize: %d, want 413", rec.Code)
	}
}

type itemized struct{ msg string }

func (e itemized) Error() string   { return e.msg }
func (e itemized) StatusCode() int { return 422 }
func (e itemized) Details() []Detail {
	return []Detail{{Check: "email_required", Columns: []string{"email"}, Message: e.msg}}
}

func TestProblemBodyRoundTrip(t *testing.T) {
	srv := httptest.NewServer(Handler("GET /x", func(w http.ResponseWriter, r *Request) error {
		return itemized{"email required"}
	}, nil))
	defer srv.Close()
	for _, f := range []Format{FormatJSON, FormatGOB} {
		c := &Client{Base: srv.URL, Format: f}
		err := c.Do(context.Background(), "POST", "/x", nil, nil)
		var pe *ProblemError
		if !errors.As(err, &pe) || pe.Status != 422 || len(pe.Details()) != 1 || pe.Details()[0].Columns[0] != "email" {
			t.Fatalf("%v: %v", f, err)
		}
		if !errors.Is(err, Error(422, "")) {
			t.Errorf("%v: a problem matches its status", f)
		}
	}
	// The body itself is a Problem in the negotiated format.
	rec := serve(t, func(w http.ResponseWriter, r *Request) error { return ErrNotFound }, httptest.NewRequest("GET", "/x", nil))
	var p Problem
	if rec.Code != 404 || json.Unmarshal(rec.Body.Bytes(), &p) != nil || p.Status != 404 || p.Message != "not found" {
		t.Fatalf("problem body = %d %q", rec.Code, rec.Body.String())
	}
}
