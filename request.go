package volt

import (
	"net/http"
	"net/url"
	"time"
)

// Request is what a handler receives (spec §requests-and-responses): a
// value holding the method, the path, the matched route, the path
// parameters, the query string, the headers, the body and the time the
// shim captured once. It carries no writer and no context; the shim
// holds both.
type Request struct {
	method, path, route string
	params              map[string]string
	query               url.Values
	header              http.Header
	body                []byte
	now                 time.Time
}

// RequestOf is the Request the shim builds from hr for the route: the
// named path parameters are read once, the body is what the shim read,
// and now is the request's clock.
func RequestOf(hr *http.Request, route string, now time.Time, body []byte, params ...string) Request {
	r := Request{method: hr.Method, path: hr.URL.Path, route: route, query: hr.URL.Query(), header: hr.Header, body: body, now: now}
	if len(params) > 0 {
		r.params = make(map[string]string, len(params))
		for _, name := range params {
			r.params[name] = hr.PathValue(name)
		}
	}
	return r
}

// Method is the HTTP method.
func (r Request) Method() string { return r.method }

// Path is the request path.
func (r Request) Path() string { return r.path }

// Route is the matched route's ServeMux pattern.
func (r Request) Route() string { return r.route }

// PathValue is the named path parameter as ServeMux captured it, "" for
// a name the route does not capture.
func (r Request) PathValue(name string) string { return r.params[name] }

// Query is the parsed query string; a handler reads it and never
// writes it.
func (r Request) Query() url.Values { return r.query }

// Header is the request headers; a handler reads them and never writes
// them.
func (r Request) Header() http.Header { return r.header }

// Body is the request body, read by the shim up to MaxBodyBytes.
func (r Request) Body() []byte { return r.body }

// Now is the time the shim captured when the request arrived (spec
// §values-and-effects): the one clock a handler reads.
func (r Request) Now() time.Time { return r.now }
