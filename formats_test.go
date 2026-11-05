package volt

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Piechutowski/volt/nao/rt"
)

type row struct {
	ID   int32
	Name string
}

func serve(t *testing.T, h HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	Handler("GET /x", h, nil, nil).ServeHTTP(rec, req)
	return rec
}

func requestOf(req *http.Request) Request { return RequestOf(req, "GET /x", time.Time{}, nil) }

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
		got, err := Negotiate(requestOf(req))
		if got != tc.want || (err == nil) != tc.ok {
			t.Errorf("Accept %q: got (%v, %v), want (%v, ok=%v)", tc.accept, got, err, tc.want, tc.ok)
		}
		var he HTTPError
		if !tc.ok && (!errors.As(err, &he) || he.StatusCode() != http.StatusNotAcceptable) {
			t.Errorf("Accept %q: err = %v, want a 406", tc.accept, err)
		}
	}
}

func TestRenderJSONAndGOB(t *testing.T) {
	rows := []row{{1, "a"}, {2, "b"}}
	h := func(Request) Response { return Render(rows) }

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

	rec = serve(t, func(Request) Response { return RenderStatus(204, nil) }, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 204 || rec.Body.Len() != 0 {
		t.Fatalf("nil render: %d %q", rec.Code, rec.Body.String())
	}
}

func TestDecodeByContentType(t *testing.T) {
	decode := func(r Request) Response {
		var v row
		if err := Decode(r, &v); err != nil {
			return Fail(err)
		}
		return HTML([]byte(v.Name))
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
	r := requestOf(httptest.NewRequest("GET", "/x?year=2024&site=a&site=b&flag=true&at=2024-01-02T03:04:05Z&f=1.5", nil))
	year, err := QueryParam[int](r, "year")
	if err != nil || year != 2024 {
		t.Errorf("year = %d, %v", year, err)
	}
	site, err := QueryParams[string](r, "site")
	if err != nil || len(site) != 2 || site[1] != "b" {
		t.Errorf("site = %v, %v", site, err)
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
	none, err := QueryParams[int](r, "none")
	if err != nil || len(none) != 0 {
		t.Errorf("absent list = %v, %v; want empty, nil", none, err)
	}
	if _, err := QueryParam[int](r, "missing"); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Errorf("missing scalar should be a 400 naming it: %v", err)
	}
	if _, err := QueryParam[int](r, "site"); err == nil || !strings.Contains(err.Error(), "site") {
		t.Errorf("unparseable scalar should be a 400 naming it: %v", err)
	}
	far := requestOf(httptest.NewRequest("GET", "/x?at=9999-12-31T23:00:00-05:00", nil))
	if _, err := QueryParams[time.Time](far, "at"); err == nil || !strings.Contains(err.Error(), "at") {
		t.Errorf("a time past 9999 in UTC should be a 400 naming it: %v", err)
	}

	rec := serve(t, func(r Request) Response {
		_, err := QueryParam[int](r, "n")
		return Fail(err)
	}, httptest.NewRequest("GET", "/x?n=abc", nil))
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "n") {
		t.Fatalf("bad value: %d %q", rec.Code, rec.Body.String())
	}
}

// An optional parameter binds absent when its key is missing, present when
// it is there, an empty value included, and refuses a value that does not
// parse as the scalar binder does (spec §query-routes).
func TestQueryOptionalBinding(t *testing.T) {
	r := requestOf(httptest.NewRequest("GET", "/x?year=2024&tag=&at=2024-01-02T03:04:05Z&n=abc", nil))
	year, err := QueryParamOptional[int](r, "year")
	if err != nil || !year.Valid || year.V != 2024 {
		t.Errorf("year = %+v, %v; want present 2024", year, err)
	}
	tag, err := QueryParamOptional[string](r, "tag")
	if err != nil || !tag.Valid || tag.V != "" {
		t.Errorf("tag= = %+v, %v; want present and empty", tag, err)
	}
	at, err := QueryParamOptional[time.Time](r, "at")
	if err != nil || !at.Valid || at.V.Hour() != 3 {
		t.Errorf("at = %+v, %v; want present", at, err)
	}
	missing, err := QueryParamOptional[int](r, "missing")
	if err != nil || missing.Valid {
		t.Errorf("a missing key = %+v, %v; want absent and no error", missing, err)
	}
	if _, err := QueryParamOptional[int](r, "n"); err == nil || !strings.Contains(err.Error(), "n") {
		t.Errorf("an unparseable value should be a 400 naming it: %v", err)
	}
}

// A float64 takes no NaN from the query string, in any case and in any
// binder, since SQLite binds NaN as NULL and a present optional NaN would
// read as absent; infinities still bind (spec §query-routes, §predicates).
func TestQueryFloatRefusesNaN(t *testing.T) {
	r := requestOf(httptest.NewRequest("GET", "/x?a=NaN&b=nan&c=-NAN&l=1&l=NaN&inf=-Inf", nil))
	for _, name := range []string{"a", "b", "c"} {
		_, err := QueryParam[float64](r, name)
		if p, _ := ProblemOf(err); err == nil || !strings.Contains(err.Error(), name) || p.Status != 400 {
			t.Errorf("QueryParam %s: %v; want a 400 naming it", name, err)
		}
		if v, err := QueryParamOptional[float64](r, name); err == nil || v.Valid {
			t.Errorf("QueryParamOptional %s = %+v, %v; want a 400, never a present NaN", name, v, err)
		}
	}
	if _, err := QueryParams[float64](r, "l"); err == nil || !strings.Contains(err.Error(), "l") {
		t.Errorf("a list with a NaN item: %v; want a 400 naming it", err)
	}
	if v, err := QueryParam[float64](r, "inf"); err != nil || !math.IsInf(v, -1) {
		t.Errorf("-Inf = %v, %v; want negative infinity", v, err)
	}
}

func TestNoRowsIs404(t *testing.T) {
	rec := serve(t, func(Request) Response { return Fail(rt.ErrNotFound) }, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 404 {
		t.Fatalf("rt.ErrNotFound: %d, want 404", rec.Code)
	}
}

func TestFormatQueryRoundTrip(t *testing.T) {
	at := time.Date(2024, 1, 2, 3, 4, 5, 250_000_000, time.UTC)
	r := requestOf(httptest.NewRequest("GET", URL("/x",
		Query("s", FormatQuery("a b")), Query("i", FormatQuery(-7)),
		Query("f", FormatQuery(1.25)), Query("b", FormatQuery(true)), Query("t", FormatQuery(at))), nil))
	if v, _ := QueryParam[string](r, "s"); v != "a b" {
		t.Errorf("s = %q", v)
	}
	if v, _ := QueryParam[int](r, "i"); v != -7 {
		t.Errorf("i = %d", v)
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
	srv := httptest.NewServer(Handler("GET /x", func(r Request) Response {
		switch r.Path() {
		case "/rows":
			var in row
			if r.Method() == "POST" {
				if err := Decode(r, &in); err != nil {
					return Fail(err)
				}
				return Created(in)
			}
			return Render([]row{{1, "a"}})
		case "/gone":
			return Fail(ErrNotFound)
		case "/none":
			return NoContent()
		}
		return Fail(Error(418, "teapot"))
	}, nil, nil))
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
		resp, err := c.Raw(context.Background(), "GET", "/rows", nil)
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("%v: raw = %v %v", f, resp, err)
		}
		resp.Body.Close()
	}
}

func TestDecodeStrictAndBounded(t *testing.T) {
	decode := func(r Request) Response {
		var v row
		if err := Decode(r, &v); err != nil {
			return Fail(err)
		}
		return Render(v)
	}
	rec := serve(t, decode, httptest.NewRequest("POST", "/x", strings.NewReader(`{"ID":1,"Nmae":"typo"}`)))
	if rec.Code != 400 {
		t.Fatalf("unknown field: %d %q", rec.Code, rec.Body.String())
	}
	var p Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	want := Detail{Check: "unknown_field", Columns: []string{"Nmae"}, Message: `unknown field "Nmae"`}
	if len(p.Details) != 1 || !reflect.DeepEqual(p.Details[0], want) {
		t.Fatalf("details = %+v, want [%+v]", p.Details, want)
	}
	rec = serve(t, decode, httptest.NewRequest("POST", "/x", strings.NewReader(`{"Nmae":1,"Nam":2}`)))
	if rec.Code != 400 || strings.Contains(rec.Body.String(), "Nam\"") {
		t.Fatalf("second unknown field: %d %q", rec.Code, rec.Body.String())
	}
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
	srv := httptest.NewServer(Handler("GET /x", func(Request) Response { return Fail(itemized{"email required"}) }, nil, nil))
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
	rec := serve(t, func(Request) Response { return Fail(ErrNotFound) }, httptest.NewRequest("GET", "/x", nil))
	var p Problem
	if rec.Code != 404 || json.Unmarshal(rec.Body.Bytes(), &p) != nil || p.Status != 404 || p.Message != "not found" {
		t.Fatalf("problem body = %d %q", rec.Code, rec.Body.String())
	}
}

// TestRenderThatDoesNotEncodeIs500 proves a value the negotiated format
// cannot carry, such as an infinite float in JSON, is a 500 problem
// rather than a 200 with an empty body, and the handler's headers and
// cookies do not ride on it (spec §formats).
func TestRenderThatDoesNotEncodeIs500(t *testing.T) {
	h := func(Request) Response {
		return Render(map[string]float64{"wall_ms": math.Inf(1)}).WithHeader("X-Kept", "no")
	}
	rec := serve(t, h, httptest.NewRequest("GET", "/x", nil))
	var p Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); rec.Code != http.StatusInternalServerError || err != nil || p.Status != 500 {
		t.Fatalf("an unencodable render = %d %q (%v), want a 500 problem", rec.Code, rec.Body.String(), err)
	}
	if rec.Header().Get("X-Kept") != "" {
		t.Errorf("the failed render kept its header %q", rec.Header().Get("X-Kept"))
	}
}

type panicky struct{}

func (panicky) MarshalJSON() ([]byte, error) { panic("no JSON for this") }

func (panicky) GobEncode() ([]byte, error) { panic("no GOB for this") }

// TestRenderFailuresAre500ProblemsInEveryFormat proves a nil pointer and
// a value whose encoder panics are a 500 problem in the negotiated
// format, JSON and GOB alike, that the connection is not cut, and that a
// nil v is a status without a body in both (spec §formats).
func TestRenderFailuresAre500ProblemsInEveryFormat(t *testing.T) {
	for _, accept := range []string{MIMEJSON, MIMEGOB} {
		for name, v := range map[string]any{"nil pointer": (*row)(nil), "panicking encoder": panicky{}} {
			req := httptest.NewRequest("GET", "/x", nil)
			req.Header.Set("Accept", accept)
			rec := serve(t, func(Request) Response { return Render(v) }, req)
			f, _ := FormatOf(rec.Header().Get("Content-Type"))
			var p Problem
			err := DecodeBody(f, rec.Body, &p)
			if rec.Code != http.StatusInternalServerError || rec.Header().Get("Content-Type") != accept || err != nil || p.Status != 500 {
				t.Errorf("%s as %s = %d %q (%v), want a 500 problem in it", name, accept, rec.Code, rec.Header().Get("Content-Type"), err)
			}
		}
		req := httptest.NewRequest("GET", "/x", nil)
		req.Header.Set("Accept", accept)
		rec := serve(t, func(Request) Response { return Render(nil) }, req)
		if rec.Code != 200 || rec.Body.Len() != 0 {
			t.Errorf("Render(nil) as %s = %d %q, want 200 and no body", accept, rec.Code, rec.Body.String())
		}
	}
}

// TestRenderFailuresReachTheRouteErrorHandler proves a 406 and a value
// that does not encode go to the route's error handler, as every other
// failure does, and that the default handler answers when the error
// handler's own response does not encode either (spec §scopes, §formats).
func TestRenderFailuresReachTheRouteErrorHandler(t *testing.T) {
	var seen []error
	eh := func(r Request, err error) Response {
		seen = append(seen, err)
		return DefaultErrorHandler(r, err).WithHeader("X-Route-Handler", "yes")
	}
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Accept", "text/html")
	rec := serveWith(func(Request) Response { return Render(row{ID: 1}) }, eh, nil, req)
	if rec.Code != http.StatusNotAcceptable || rec.Header().Get("X-Route-Handler") != "yes" || len(seen) != 1 {
		t.Errorf("406 = %d, handler header %q, %d failures seen; want the route's handler to answer", rec.Code, rec.Header().Get("X-Route-Handler"), len(seen))
	}
	seen = nil
	rec = serveWith(func(Request) Response { return Render(math.Inf(1)) }, eh, nil, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != http.StatusInternalServerError || rec.Header().Get("X-Route-Handler") != "yes" || len(seen) != 1 || !strings.Contains(seen[0].Error(), "float64") {
		t.Errorf("encode failure = %d, handler header %q, failures %v; want the route's handler to answer", rec.Code, rec.Header().Get("X-Route-Handler"), seen)
	}
	calls, performed := 0, 0
	stubborn := func(Request, error) Response {
		calls++
		return Then(rt.Query[int]{SQL: "INSERT INTO audit DEFAULT VALUES"}, func([]int) Response { return Render(math.Inf(1)) })
	}
	perform := func(context.Context, rt.AnyEffect) (any, error) { performed++; return []int{}, nil }
	rec = serveWith(func(Request) Response { return Render(math.Inf(1)) }, stubborn, perform, httptest.NewRequest("GET", "/x", nil))
	var p Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); rec.Code != http.StatusInternalServerError || err != nil || p.Status != 500 {
		t.Errorf("an error handler that renders the unencodable = %d %q, want the default 500 problem", rec.Code, rec.Body.String())
	}
	if calls != 1 || performed != 1 {
		t.Errorf("an error handler whose answer does not encode ran %d times and performed %d effects, want once each", calls, performed)
	}
	calls = 0
	rendering := func(Request, error) Response { calls++; return Render(row{ID: 1}) }
	req = httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Accept", "text/html")
	rec = serveWith(func(Request) Response { return Render(row{ID: 1}) }, rendering, nil, req)
	if rec.Code != http.StatusNotAcceptable || calls != 1 {
		t.Errorf("a 406 to an error handler that renders = %d after %d calls, want the default 406 after one", rec.Code, calls)
	}
}

// TestResponsesCarryContentLength proves a rendered value, a problem and
// HTML carry the length of the body they write.
func TestResponsesCarryContentLength(t *testing.T) {
	for name, h := range map[string]HandlerFunc{
		"render":  func(Request) Response { return Render([]row{{ID: 1, Name: "a"}}) },
		"problem": func(Request) Response { return Reject(http.StatusConflict, "taken") },
		"html":    func(Request) Response { return HTML([]byte("<p>hi</p>")) },
	} {
		rec := serve(t, h, httptest.NewRequest("GET", "/x", nil))
		if got := rec.Header().Get("Content-Length"); got == "" || got != strconv.Itoa(rec.Body.Len()) {
			t.Errorf("%s: Content-Length %q for a body of %d bytes", name, got, rec.Body.Len())
		}
	}
}

// TestNestedNilPointersRenderAsTheirEncodersDo proves the difference
// §formats states below the top level: a nil pointer in a slice or map
// is JSON null but a GOB 500 problem, never a cut connection.
func TestNestedNilPointersRenderAsTheirEncodersDo(t *testing.T) {
	for name, v := range map[string]any{"slice": []*row{nil}, "map": map[string]*row{"a": nil}} {
		req := httptest.NewRequest("GET", "/x", nil)
		rec := serve(t, func(Request) Response { return Render(v) }, req)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), "null") {
			t.Errorf("%s as JSON = %d %q, want 200 with null", name, rec.Code, rec.Body.String())
		}
		req = httptest.NewRequest("GET", "/x", nil)
		req.Header.Set("Accept", MIMEGOB)
		rec = serve(t, func(Request) Response { return Render(v) }, req)
		var p Problem
		if err := DecodeBody(FormatGOB, rec.Body, &p); rec.Code != 500 || err != nil || p.Status != 500 {
			t.Errorf("%s as GOB = %d (%v), want a 500 problem", name, rec.Code, err)
		}
	}
}
