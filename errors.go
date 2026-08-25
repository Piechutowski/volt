package volt

import "net/http"

// HTTPError is the error-to-status contract: a handler error carrying
// its own response code. DefaultErrorHandler and user error handlers
// map it via errors.As-compatible assertion.
type HTTPError interface {
	error
	StatusCode() int
}

type statusError struct {
	code int
	msg  string
}

func (e *statusError) Error() string   { return e.msg }
func (e *statusError) StatusCode() int { return e.code }

// Error builds an HTTPError with the given status code and message.
func Error(code int, msg string) error {
	if msg == "" {
		msg = http.StatusText(code)
	}
	return &statusError{code: code, msg: msg}
}

// Sentinels for the common cases. ErrNotFound is also what generated
// shims return when a typed path parameter fails to parse (R6: a parse
// failure is that route's 404, never a fallthrough).
var (
	ErrNotFound   = Error(http.StatusNotFound, "not found")
	ErrBadRequest = Error(http.StatusBadRequest, "bad request")
	ErrForbidden  = Error(http.StatusForbidden, "forbidden")
)
