package volt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Piechutowski/volt/nao/rt"
)

func serveWith(h HandlerFunc, eh ErrorHandler, perform Performer, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	Handler("GET /x", h, eh, perform).ServeHTTP(rec, req)
	return rec
}

func TestHandlerErrorSpine(t *testing.T) {
	rec := serveWith(func(Request) Response { return Fail(Error(418, "teapot")) }, nil, nil, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 418 || !strings.Contains(rec.Body.String(), "teapot") {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestHandlerGenericErrorIs500(t *testing.T) {
	rec := serveWith(func(Request) Response { return Fail(errors.New("secret database detail")) }, nil, nil, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatal("internal error detail leaked to the response body")
	}
}

func TestStreamFailureIsLogOnly(t *testing.T) {
	rec := serveWith(func(Request) Response {
		return Stream(func(w io.Writer) error {
			w.Write([]byte("partial"))
			return errors.New("late failure")
		})
	}, nil, nil, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 200 || rec.Body.String() != "partial" {
		t.Fatalf("a committed stream was modified: %d %q", rec.Code, rec.Body.String())
	}
}

func TestCustomErrorHandlerSeesRoute(t *testing.T) {
	var gotRoute string
	eh := func(r Request, err error) Response {
		gotRoute = r.Route()
		return Reject(400, "custom")
	}
	rec := httptest.NewRecorder()
	Handler("GET /users/{id}", func(r Request) Response { return Fail(errors.New("x")) }, eh, nil, Params("id")).
		ServeHTTP(rec, httptest.NewRequest("GET", "/users/7", nil))
	if gotRoute != "GET /users/{id}" || rec.Code != 400 {
		t.Fatalf("route=%q code=%d", gotRoute, rec.Code)
	}
}

func TestWrappedHTTPErrorUnwraps(t *testing.T) {
	rec := serveWith(func(Request) Response {
		return Fail(fmt.Errorf("saving draft: %w", Error(422, "title required")))
	}, nil, nil, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 422 {
		t.Fatalf("status = %d, want 422 (wrapped HTTPError must unwrap)", rec.Code)
	}
}

func TestRequestValue(t *testing.T) {
	mux := http.NewServeMux()
	var got Request
	mux.Handle("GET /users/{id}", Handler("GET /users/{id}", func(r Request) Response {
		got = r
		return NoContent()
	}, nil, nil, Params("id")))
	req := httptest.NewRequest("GET", "/users/7?tab=x", strings.NewReader("body"))
	req.Header.Set("X-Test", "yes")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 204 || got.Method() != "GET" || got.Path() != "/users/7" || got.Route() != "GET /users/{id}" ||
		got.PathValue("id") != "7" || got.Query().Get("tab") != "x" || got.Header().Get("X-Test") != "yes" ||
		string(got.Body()) != "body" || got.Now().IsZero() {
		t.Fatalf("request value = %+v (%d)", got, rec.Code)
	}
}

type fakeEffect struct {
	rt.Query[int]
	rows []int
	err  error
}

func performFake(ctx context.Context, e rt.AnyEffect) (any, error) {
	f := e.(fakeEffect)
	if f.err != nil {
		return nil, f.err
	}
	return f.rows, nil
}

func TestThenPerformsThroughThePerformer(t *testing.T) {
	h := func(Request) Response {
		return Then(rt.Query[int]{SQL: "SELECT 1"}, func(rows []int) Response { return Render(len(rows)) })
	}
	perform := func(ctx context.Context, e rt.AnyEffect) (any, error) { return []int{1, 2, 3}, nil }
	rec := serveWith(h, nil, perform, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "3" {
		t.Fatalf("Then = %d %q", rec.Code, rec.Body.String())
	}
	boom := func(ctx context.Context, e rt.AnyEffect) (any, error) { return nil, rt.ErrNotFound }
	rec = serveWith(h, nil, boom, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 404 {
		t.Fatalf("a failed effect = %d, want 404 through the spine", rec.Code)
	}
	rescued := func(Request) Response {
		return Then(rt.Query[int]{SQL: "SELECT 1"}, func([]int) Response { return NoContent() }).
			Rescue(func(err error) Response { return Redirect("/login") })
	}
	rec = serveWith(rescued, nil, boom, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 303 || rec.Header().Get("Location") != "/login" {
		t.Fatalf("Rescue = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	rec = serveWith(h, nil, nil, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 500 {
		t.Fatalf("Then with no performer = %d, want 500", rec.Code)
	}
}

func TestLoad(t *testing.T) {
	found := fakeEffect{Query: rt.Query[int]{SQL: "SELECT user"}, rows: []int{1}}
	miss := fakeEffect{Query: rt.Query[int]{SQL: "SELECT user"}, err: rt.ErrNotFound}
	eh := func(r Request, err error) Response {
		return DefaultErrorHandler(r, err).WithHeader("X-Error-Handler", "route")
	}
	var ran bool
	serve := func(l Load[[]int]) *httptest.ResponseRecorder {
		ran = false
		return serveWith(func(Request) Response {
			return Continue(l, func(rows []int) Response {
				ran = true
				return Render(len(rows))
			})
		}, eh, performFake, httptest.NewRequest("GET", "/x", nil))
	}
	if rec := serve(LoadWith[[]int](found)); rec.Code != 200 || !ran || strings.TrimSpace(rec.Body.String()) != "1" {
		t.Errorf("LoadWith = %d %q, ran %v; want the value handed beneath", rec.Code, rec.Body.String(), ran)
	}
	if rec := serve(LoadWith[[]int](miss)); rec.Code != 404 || ran || rec.Header().Get("X-Error-Handler") != "route" {
		t.Errorf("a failed load = %d via %q, ran %v; want 404 through the error handler", rec.Code, rec.Header().Get("X-Error-Handler"), ran)
	}
	if rec := serve(Halt[[]int](Reject(401, "sign in"))); rec.Code != 401 || ran || rec.Header().Get("X-Error-Handler") != "" {
		t.Errorf("Halt = %d via %q, ran %v; want its own 401", rec.Code, rec.Header().Get("X-Error-Handler"), ran)
	}
	if rec := serve(Load[[]int]{}); rec.Code != 500 || ran || rec.Header().Get("X-Error-Handler") != "route" {
		t.Errorf("the zero Load = %d via %q, ran %v; want 500 through the error handler", rec.Code, rec.Header().Get("X-Error-Handler"), ran)
	}
}

func TestLoadRescue(t *testing.T) {
	found := fakeEffect{Query: rt.Query[int]{SQL: "SELECT user"}, rows: []int{1}}
	miss := fakeEffect{Query: rt.Query[int]{SQL: "SELECT user"}, err: rt.ErrNotFound}
	down := fakeEffect{Query: rt.Query[int]{SQL: "SELECT user"}, err: errors.New("disk I/O error")}
	eh := func(r Request, err error) Response {
		return DefaultErrorHandler(r, err).WithHeader("X-Error-Handler", "route")
	}
	var rescued []error
	signIn := func(err error) Response {
		rescued = append(rescued, err)
		if errors.Is(err, rt.ErrNotFound) {
			return Reject(401, "sign in")
		}
		return Fail(err)
	}
	var ran bool
	serve := func(l Load[[]int], next func([]int) Response) *httptest.ResponseRecorder {
		ran, rescued = false, nil
		return serveWith(func(Request) Response {
			return Continue(l, func(rows []int) Response {
				ran = true
				return next(rows)
			})
		}, eh, performFake, httptest.NewRequest("GET", "/x", nil))
	}
	render := func(rows []int) Response { return Render(len(rows)) }
	via := func(rec *httptest.ResponseRecorder) string { return rec.Header().Get("X-Error-Handler") }

	rec := serve(LoadWith[[]int](miss).Rescue(signIn), render)
	if rec.Code != 401 || via(rec) != "" || ran || len(rescued) != 1 || !errors.Is(rescued[0], rt.ErrNotFound) {
		t.Errorf("a rescued miss = %d via %q, ran %v, rescued %v; want the rescue's 401 and nothing beneath", rec.Code, via(rec), ran, rescued)
	}
	rec = serve(LoadWith[[]int](down).Rescue(signIn), render)
	if rec.Code != 500 || via(rec) != "route" || ran || len(rescued) != 1 {
		t.Errorf("a rescue's Fail = %d via %q, ran %v; want 500 through the error handler", rec.Code, via(rec), ran)
	}
	rec = serve(LoadWith[[]int](found).Rescue(signIn), render)
	if rec.Code != 200 || !ran || len(rescued) != 0 || strings.TrimSpace(rec.Body.String()) != "1" {
		t.Errorf("a rescued load that succeeds = %d %q, ran %v; want the value handed beneath", rec.Code, rec.Body.String(), ran)
	}

	beneath := map[string]func([]int) Response{
		"the handler's continuation": func([]int) Response { return Then[[]int](miss, render) },
		"a route load's continuation": func([]int) Response {
			return Then[[]int](found, func([]int) Response { return Then[[]int](miss, render) })
		},
		"a later scope load":     func([]int) Response { return Continue(LoadWith[[]int](miss), render) },
		"the handler's own Fail": func([]int) Response { return Fail(rt.ErrNotFound) },
	}
	for name, next := range beneath {
		rec = serve(LoadWith[[]int](found).Rescue(signIn), next)
		if rec.Code != 404 || via(rec) != "route" || !ran || len(rescued) != 0 {
			t.Errorf("%s failing under a rescued load = %d via %q, rescued %v; want 404 through the error handler, never the load's rescue", name, rec.Code, via(rec), rescued)
		}
	}
	other := func(error) Response { return Reject(403, "not yours") }
	rec = serve(LoadWith[[]int](found).Rescue(signIn), func([]int) Response {
		return Continue(LoadWith[[]int](miss).Rescue(other), render)
	})
	if rec.Code != 403 || len(rescued) != 0 {
		t.Errorf("a later scope load's own rescue = %d, outer rescued %v; want the inner load's 403", rec.Code, rescued)
	}

	halted := Halt[[]int](Reject(401, "sign in"))
	if rec = serve(halted.Rescue(other), render); rec.Code != 401 || ran || len(rescued) != 0 {
		t.Errorf("Rescue on a Halt = %d, ran %v; want the halt's own 401", rec.Code, ran)
	}
	if !Same(Continue(halted.Rescue(other), render), Reject(401, "sign in")) {
		t.Error("Rescue changed a Halt")
	}
	called := false
	zero := Load[[]int]{}.Rescue(func(error) Response { called = true; return NoContent() })
	if rec = serve(zero, render); rec.Code != 500 || via(rec) != "route" || called {
		t.Errorf("Rescue on the zero Load = %d via %q, called %v; want 500 through the error handler", rec.Code, via(rec), called)
	}

	plain := LoadWith[[]int](miss)
	_ = plain.Rescue(signIn)
	if rec = serve(plain, render); rec.Code != 404 || len(rescued) != 0 {
		t.Errorf("Rescue changed its receiver: %d, rescued %v", rec.Code, rescued)
	}
	if rec = serve(plain.Rescue(signIn).Rescue(other), render); rec.Code != 403 || len(rescued) != 0 {
		t.Errorf("a second Rescue = %d, first rescued %v; want the second to replace the first", rec.Code, rescued)
	}
	if rec = serve(plain.Rescue(signIn).Rescue(nil), render); rec.Code != 404 || via(rec) != "route" || len(rescued) != 0 {
		t.Errorf("Rescue(nil) = %d via %q; want the error handler back", rec.Code, via(rec))
	}
	if Same(Continue(plain.Rescue(signIn), render), Continue(plain, render)) || !Same(Continue(plain.Rescue(signIn), render), Then[[]int](miss, render).Rescue(signIn)) {
		t.Error("Same does not see a load's rescue as a continuation's")
	}

	slow := func(ctx context.Context, e rt.AnyEffect) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	rescued = nil
	rec = httptest.NewRecorder()
	Handler("GET /x", func(Request) Response {
		return Continue(LoadWith[[]int](found).Rescue(signIn), render)
	}, eh, slow, Timeout(10*time.Millisecond)).ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	var he HTTPError
	if rec.Code != http.StatusGatewayTimeout || via(rec) != "route" || len(rescued) != 1 || !errors.As(rescued[0], &he) || he.StatusCode() != http.StatusGatewayTimeout {
		t.Errorf("a load past its timeout = %d via %q, rescued %v; want the rescue to see the 504 and Fail it to the error handler", rec.Code, via(rec), rescued)
	}
}

func TestResponseValues(t *testing.T) {
	a := Render(1).WithHeader("X-A", "1").WithCookie(http.Cookie{Name: "c", Value: "v"})
	b := a.WithStatus(202)
	if a.Status() != 200 || b.Status() != 202 || a.header.Get("X-A") != "1" || len(a.cookies) != 1 {
		t.Errorf("With* mutated its receiver: %+v %+v", a, b)
	}
	if !Same(Render(1), Render(1)) || Same(Render(1), Render(2)) || Same(Render(1), Created(1)) {
		t.Error("Same disagrees on rendered responses")
	}
	q := rt.Query[int]{SQL: "SELECT ?", Args: []any{1}}
	next := func([]int) Response { return NoContent() }
	if !Same(Then(q, next), Then(q, next)) || Same(Then(q, next), Then(rt.Query[int]{SQL: "SELECT 2"}, next)) {
		t.Error("Same disagrees on continuations")
	}
	if !Same(Fail(errors.New("x")), Fail(errors.New("x"))) || Same(Then(q, next), Render(1)) {
		t.Error("Same disagrees on failures")
	}
	rec := httptest.NewRecorder()
	Handler("GET /x", func(Request) Response { return HTML([]byte("<p>hi</p>")) }, nil, nil).ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "text/html; charset=utf-8" || rec.Body.String() != "<p>hi</p>" {
		t.Errorf("HTML = %d %q %q", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
	rec = httptest.NewRecorder()
	Handler("GET /x", func(Request) Response {
		return Reject(422, "check the fields").WithDetails([]rt.FieldError{{Check: "required", Columns: []string{"title"}, Message: "title is required"}})
	}, nil, nil).ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), `"check":"required"`) {
		t.Errorf("Reject with details = %d %q", rec.Code, rec.Body.String())
	}
}

func TestStatusWriterUnwrap(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec}
	if sw.Unwrap() != http.ResponseWriter(rec) {
		t.Fatal("Unwrap must expose the underlying writer for http.ResponseController")
	}
}

func TestChainOrder(t *testing.T) {
	var order []string
	mw := func(tag string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, tag)
				next.ServeHTTP(w, r)
			})
		}
	}
	h := Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		order = append(order, "h")
	}), mw("a"), mw("b"))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if got := strings.Join(order, ","); got != "a,b,h" {
		t.Fatalf("order = %s, want a,b,h (declaration order, outermost first)", got)
	}
}

func TestParsers(t *testing.T) {
	if v, ok := ParseInt("42"); !ok || v != 42 {
		t.Error("ParseInt(42)")
	}
	if _, ok := ParseInt("4x"); ok {
		t.Error("ParseInt(4x) should fail")
	}
	if _, ok := ParseInt("99999999999999999999"); ok {
		t.Error("ParseInt overflow should fail")
	}
	if v, ok := ParseInt("-7"); !ok || v != -7 {
		t.Error("ParseInt(-7)")
	}
}

func TestURLBuilding(t *testing.T) {
	if got := URL("/users/" + SegInt(42)); got != "/users/42" {
		t.Errorf("got %q", got)
	}
	got := URL("/u/"+Seg("a b"), Query("tab", "x y"))
	if got != "/u/a%20b?tab=x+y" {
		t.Errorf("got %q", got)
	}
	if got := SegWild("a b/c"); got != "a%20b/c" {
		t.Errorf("SegWild = %q", got)
	}
}

func TestSegDotSegments(t *testing.T) {
	if got := Seg("."); got != "%2E" {
		t.Errorf("Seg(.) = %q", got)
	}
	if got := Seg(".."); got != "%2E%2E" {
		t.Errorf("Seg(..) = %q", got)
	}
	if got := Seg(".hidden"); got != ".hidden" {
		t.Errorf("Seg(.hidden) = %q (interior dots are fine)", got)
	}
	if got := SegWild("a/../b"); got != "a/%2E%2E/b" {
		t.Errorf("SegWild(a/../b) = %q", got)
	}
}

func TestSegWildClean(t *testing.T) {
	mux := http.NewServeMux()
	var got string
	mux.HandleFunc("GET /files/{path...}", func(w http.ResponseWriter, r *http.Request) { got = r.PathValue("path") })
	for value, want := range map[string]string{
		"a//b":  "a/b",
		"/a":    "a",
		"a/":    "a/",
		"a//":   "a/",
		"":      "",
		"/":     "",
		"a b/c": "a%20b/c",
	} {
		seg := SegWild(value)
		if seg != want {
			t.Errorf("SegWild(%q) = %q, want %q", value, seg, want)
		}
		got = "unset"
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", "/files/"+seg, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET /files/%s = %d, want 200 without a redirect", seg, rec.Code)
		}
		if got == "unset" {
			t.Errorf("GET /files/%s reached no handler", seg)
		}
	}
}

func TestRequestID(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("X-Request-ID")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if seen == "" || rec.Header().Get("X-Request-ID") != seen {
		t.Fatalf("request id not set consistently: %q vs %q", seen, rec.Header().Get("X-Request-ID"))
	}
}

func TestTimeoutIs504(t *testing.T) {
	slow := func(ctx context.Context, e rt.AnyEffect) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	h := func(Request) Response {
		return Then(rt.Query[int]{SQL: "SELECT 1"}, func([]int) Response { return NoContent() })
	}
	rec := httptest.NewRecorder()
	Handler("GET /x", h, nil, slow, Timeout(10*time.Millisecond)).ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("a chain past its timeout = %d, want 504", rec.Code)
	}

	eh := func(r Request, err error) Response {
		return DefaultErrorHandler(r, err).WithHeader("X-Error-Handler", "route")
	}
	var rescued []error
	busy := func(err error) Response {
		rescued = append(rescued, err)
		return Reject(http.StatusServiceUnavailable, "try again")
	}
	rescuedChain := func(Request) Response {
		return Then(rt.Query[int]{SQL: "SELECT 1"}, func([]int) Response { return NoContent() }).Rescue(busy)
	}
	rescuedLoad := func(Request) Response {
		return Continue(LoadWith(rt.Query[int]{SQL: "SELECT 1"}).Rescue(busy), func([]int) Response { return NoContent() })
	}
	for name, h := range map[string]HandlerFunc{"a rescued continuation": rescuedChain, "a rescued scope load": rescuedLoad} {
		rescued = nil
		rec = httptest.NewRecorder()
		Handler("GET /x", h, eh, slow, Timeout(10*time.Millisecond)).ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
		var he HTTPError
		if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("X-Error-Handler") != "" || len(rescued) != 1 || !errors.As(rescued[0], &he) || he.StatusCode() != http.StatusGatewayTimeout {
			t.Errorf("%s past its timeout = %d via %q, rescued %v; want the rescue to receive the 504 and answer for it", name, rec.Code, rec.Header().Get("X-Error-Handler"), rescued)
		}
	}
}
