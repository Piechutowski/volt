package volt

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"encoding/json"
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
