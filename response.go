package volt

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"reflect"

	"github.com/Piechutowski/volt/nao/rt"
)

// Response is what a handler returns (spec §requests-and-responses): a
// value, either done, with a status, headers, cookies and a body, or a
// continuation, Then, which the shim performs before calling the next
// function. The zero Response is 200 with no body.
type Response struct {
	status  int
	header  http.Header
	cookies []http.Cookie
	body    body
	then    *step
}

type body interface{ body() }

type (
	renderBody   struct{ v any }
	redirectBody struct{ url string }
	problemBody  struct{ p Problem }
	fileBody     struct {
		fsys fs.FS
		path string
	}
	htmlBody   struct{ b []byte }
	streamBody struct{ fn func(io.Writer) error }
	emptyBody  struct{}
	failBody   struct{ err error }
)

func (renderBody) body()   {}
func (redirectBody) body() {}
func (problemBody) body()  {}
func (fileBody) body()     {}
func (htmlBody) body()     {}
func (streamBody) body()   {}
func (emptyBody) body()    {}
func (failBody) body()     {}

type step struct {
	effect rt.AnyEffect
	next   func(any) Response
	on     func(error) Response
}

// Render is v in the negotiated format with status 200 (spec §formats).
func Render(v any) Response { return Response{status: http.StatusOK, body: renderBody{v}} }

// RenderStatus is Render with an explicit status; a nil v writes the
// status and no body.
func RenderStatus(status int, v any) Response {
	if v == nil {
		return Response{status: status, body: emptyBody{}}
	}
	return Response{status: status, body: renderBody{v}}
}

// Created is v with status 201.
func Created(v any) Response { return RenderStatus(http.StatusCreated, v) }

// NoContent is status 204 and no body.
func NoContent() Response { return Response{status: http.StatusNoContent, body: emptyBody{}} }

// Redirect is a 303 to url; WithStatus changes the code.
func Redirect(url string) Response {
	return Response{status: http.StatusSeeOther, body: redirectBody{url}}
}

// Reject is a Problem body with the status and message, in the
// negotiated format (spec §formats); WithDetails itemizes it.
func Reject(status int, msg string) Response {
	return Response{status: status, body: problemBody{Problem{Status: status, Message: msg}}}
}

// Fail hands err to the route's error handler, which chooses the
// response (spec §scopes): what a handler returns for an error it does
// not care to shape.
func Fail(err error) Response { return Response{body: failBody{err}} }

// File is the file at path in fsys, served as net/http serves it.
func File(fsys fs.FS, path string) Response {
	return Response{status: http.StatusOK, body: fileBody{fsys, path}}
}

// HTML is b as text/html with status 200.
func HTML(b []byte) Response { return Response{status: http.StatusOK, body: htmlBody{b}} }

// Stream is a body written by fn, the one escape from values (spec
// §requests-and-responses); the lint annex counts its uses.
func Stream(fn func(io.Writer) error) Response {
	return Response{status: http.StatusOK, body: streamBody{fn}}
}

// WithStatus is r with the status.
func (r Response) WithStatus(code int) Response {
	r.status = code
	if p, ok := r.body.(problemBody); ok {
		p.p.Status = code
		r.body = p
	}
	return r
}

// WithHeader is r with the header set, on a copy of its headers.
func (r Response) WithHeader(k, v string) Response {
	r.header = r.header.Clone()
	if r.header == nil {
		r.header = http.Header{}
	}
	r.header.Set(k, v)
	return r
}

// WithCookie is r with the cookie added.
func (r Response) WithCookie(c http.Cookie) Response {
	r.cookies = append(append([]http.Cookie(nil), r.cookies...), c)
	return r
}

// WithDetails is a Reject with the details itemized, as a refused
// changeset's errors are (spec §formats); on any other response it is
// the response unchanged.
func (r Response) WithDetails(details []rt.FieldError) Response {
	p, ok := r.body.(problemBody)
	if !ok {
		return r
	}
	p.p.Details = make([]Detail, len(details))
	for i, f := range details {
		p.p.Details[i] = Detail{Check: f.Check, Columns: f.Columns, Message: f.Message}
	}
	r.body = p
	return r
}

// Status is the response's status code.
func (r Response) Status() int { return r.status }

// Then is the continuation that performs e and hands its result to next
// (spec §requests-and-responses): the one way a handler reaches the
// database. An effect that fails goes to the route's error handler
// unless Rescue says otherwise.
func Then[T any](e rt.Effect[T], next func(T) Response) Response {
	return Response{then: &step{effect: e, next: func(v any) Response {
		var t T
		if v != nil {
			t = v.(T)
		}
		return next(t)
	}}}
}

// Rescue is the continuation r with on answering for its effect's
// failure instead of the error handler; a done response is unchanged.
func (r Response) Rescue(on func(error) Response) Response {
	if r.then == nil {
		return r
	}
	s := *r.then
	s.on = on
	r.then = &s
	return r
}

// Same reports whether two responses are the same value: the same
// status, headers, cookies and body, or continuations of the same
// effect (spec §requests-and-responses). Streams compare as the same;
// what a function writes cannot be compared. The generated
// ControllersDeterministic calls it.
func Same(a, b Response) bool {
	if a.status != b.status || !reflect.DeepEqual(a.header, b.header) || !reflect.DeepEqual(a.cookies, b.cookies) {
		return false
	}
	if (a.then == nil) != (b.then == nil) {
		return false
	}
	if a.then != nil {
		return rt.Describe(a.then.effect) == rt.Describe(b.then.effect) && (a.then.on == nil) == (b.then.on == nil)
	}
	switch x := a.body.(type) {
	case streamBody:
		_, ok := b.body.(streamBody)
		return ok
	case failBody:
		y, ok := b.body.(failBody)
		return ok && x.err.Error() == y.err.Error()
	}
	return reflect.DeepEqual(a.body, b.body)
}

// Performer performs one effect for the shim (spec
// §requests-and-responses): the runner's, handed in by the generated
// router, since this package never holds a database.
type Performer func(ctx context.Context, e rt.AnyEffect) (any, error)

// resolve runs the continuations of resp until a done response, handing
// every failure to eh.
func resolve(ctx context.Context, req Request, resp Response, eh ErrorHandler, perform Performer) Response {
	for range 64 {
		switch {
		case resp.then != nil:
			if perform == nil {
				resp = eh(req, errors.New("volt: no database to perform the effect"))
				continue
			}
			v, err := perform(ctx, resp.then.effect)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					err = Error(http.StatusGatewayTimeout, "the route's timeout expired")
				}
				if on := resp.then.on; on != nil {
					resp = on(err)
				} else {
					resp = eh(req, err)
				}
				continue
			}
			resp = resp.then.next(v)
			continue
		}
		if f, ok := resp.body.(failBody); ok {
			resp = eh(req, f.err)
			if _, still := resp.body.(failBody); still {
				slog.Error("volt: the error handler failed", "route", req.Route(), "err", f.err)
				return Reject(http.StatusInternalServerError, "internal server error")
			}
			continue
		}
		return resp
	}
	slog.Error("volt: a response never finished", "route", req.Route())
	return Reject(http.StatusInternalServerError, "internal server error")
}

func (r Response) write(w http.ResponseWriter, hr *http.Request, req Request) error {
	var encoded bytes.Buffer
	var format Format
	if b, ok := r.body.(renderBody); ok {
		f, err := Negotiate(req)
		if err != nil {
			return DefaultErrorHandler(req, err).write(w, hr, req)
		}
		if err := Encode(&encoded, f, b.v); err != nil {
			slog.Error("volt: encoding the response", "route", req.Route(), "err", err)
			return Reject(http.StatusInternalServerError, "internal server error").write(w, hr, req)
		}
		format = f
	}
	for k, vs := range r.header {
		w.Header()[k] = vs
	}
	for i := range r.cookies {
		http.SetCookie(w, &r.cookies[i])
	}
	status := r.status
	if status == 0 {
		status = http.StatusOK
	}
	switch b := r.body.(type) {
	case renderBody:
		w.Header().Set("Content-Type", format.MIME())
		w.WriteHeader(status)
		_, err := w.Write(encoded.Bytes())
		return err
	case problemBody:
		f, _ := Negotiate(req)
		w.Header().Set("Content-Type", f.MIME())
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(status)
		return Encode(w, f, b.p)
	case redirectBody:
		http.Redirect(w, hr, b.url, status)
		return nil
	case fileBody:
		http.ServeFileFS(w, hr, b.fsys, b.path)
		return nil
	case htmlBody:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(status)
		_, err := w.Write(b.b)
		return err
	case streamBody:
		w.WriteHeader(status)
		return b.fn(w)
	default:
		w.WriteHeader(status)
		return nil
	}
}

// Load is what a scope load decides for a request (spec §scope-loads):
// an effect to perform, whose value the routes beneath receive, or a
// response that halts the request; LoadWith and Halt build it.
type Load[T any] struct {
	effect rt.Effect[T]
	halt   *Response
}

// LoadWith is the Load that performs e and hands its value on.
func LoadWith[T any](e rt.Effect[T]) Load[T] { return Load[T]{effect: e} }

// Halt is the Load that answers with resp and runs nothing beneath.
func Halt[T any](resp Response) Load[T] { return Load[T]{halt: &resp} }

// Continue is a shim's step over a scope load (spec §scope-loads): the
// halt's response, or the continuation over the effect's value.
func Continue[T any](l Load[T], next func(T) Response) Response {
	if l.halt != nil {
		return *l.halt
	}
	if l.effect == nil {
		return Fail(errors.New("volt: a scope load decided nothing; a Load is built by LoadWith or Halt"))
	}
	return Then(l.effect, next)
}
