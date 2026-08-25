package volt

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerErrorSpine(t *testing.T) {
	h := Handler("GET /x", func(w http.ResponseWriter, r *Request) error {
		return Error(418, "teapot")
	}, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 418 || !strings.Contains(rec.Body.String(), "teapot") {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestHandlerGenericErrorIs500(t *testing.T) {
	h := Handler("GET /x", func(w http.ResponseWriter, r *Request) error {
		return errors.New("secret database detail")
	}, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatal("internal error detail leaked to the response body")
	}
}

func TestCommittedResponseIsLogOnly(t *testing.T) {
	h := Handler("GET /x", func(w http.ResponseWriter, r *Request) error {
		w.WriteHeader(200)
		w.Write([]byte("partial"))
		if !r.Committed() {
			t.Error("Committed() should be true after write")
		}
		return errors.New("late failure")
	}, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 200 || rec.Body.String() != "partial" {
		t.Fatalf("committed response was modified: %d %q", rec.Code, rec.Body.String())
	}
}

func TestCustomErrorHandlerSeesRoute(t *testing.T) {
	var gotRoute string
	eh := func(w http.ResponseWriter, r *Request, err error) {
		gotRoute = r.Route()
		http.Error(w, "custom", 400)
	}
	h := Handler("GET /users/{id}", func(w http.ResponseWriter, r *Request) error {
		return errors.New("x")
	}, eh)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/users/7", nil))
	if gotRoute != "GET /users/{id}" || rec.Code != 400 {
		t.Fatalf("route=%q code=%d", gotRoute, rec.Code)
	}
}

func TestWrappedHTTPErrorUnwraps(t *testing.T) {
	h := Handler("GET /x", func(w http.ResponseWriter, r *Request) error {
		return fmt.Errorf("saving draft: %w", Error(422, "title required"))
	}, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 422 {
		t.Fatalf("status = %d, want 422 (wrapped HTTPError must unwrap)", rec.Code)
	}
}

func TestFlushCommits(t *testing.T) {
	h := Handler("GET /x", func(w http.ResponseWriter, r *Request) error {
		w.Write([]byte("chunk"))
		w.(http.Flusher).Flush()
		if !r.Committed() {
			t.Error("Committed() should be true after Flush")
		}
		return Error(500, "too late")
	}, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 200 || rec.Body.String() != "chunk" {
		t.Fatalf("flushed response was modified: %d %q", rec.Code, rec.Body.String())
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
	if v, ok := ParseInt64("42"); !ok || v != 42 {
		t.Error("ParseInt64(42)")
	}
	if _, ok := ParseInt64("4x"); ok {
		t.Error("ParseInt64(4x) should fail")
	}
	if _, ok := ParseInt32("2147483648"); ok {
		t.Error("ParseInt32 overflow should fail")
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
	// "." and ".." must never appear as literal segments in a built URL:
	// path cleaning would reshape them (§V4.6.2).
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
