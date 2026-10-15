package volt

import "net/http"

// HTTPError is the error-to-status contract: a handler error carrying
// its own response code.
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

// Is matches another status error by code, whatever its message.
func (e *statusError) Is(target error) bool {
	t, ok := target.(*statusError)
	return ok && t.code == e.code
}

// Error builds an HTTPError with the given status code and message.
func Error(code int, msg string) error {
	if msg == "" {
		msg = http.StatusText(code)
	}
	return &statusError{code: code, msg: msg}
}

// Sentinels for the common cases. ErrNotFound is also what generated
// shims return when a typed path parameter fails to parse (R6).
var (
	ErrNotFound   = Error(http.StatusNotFound, "not found")
	ErrBadRequest = Error(http.StatusBadRequest, "bad request")
	ErrForbidden  = Error(http.StatusForbidden, "forbidden")
)

// Detail is one item of a structured failure: which check failed and
// which columns it reads. It is an alias of an unnamed struct: nao's
// runtime declares the identical alias, so its errors satisfy Detailer.
type Detail = struct {
	Check   string   `json:"check"`             // the check's name or rendered form
	Columns []string `json:"columns,omitempty"` // the columns the check reads
	Message string   `json:"message"`           // the failure as text
}

// Detailer is implemented by errors that can itemize their failures;
// DefaultErrorHandler renders them into the Problem body.
type Detailer interface {
	Details() []Detail
}

// Problem is the body of every error response DefaultErrorHandler
// writes, in the negotiated format: the status, the message, and the
// details when the error itemizes them. A generated client decodes it
// back into a ProblemError.
type Problem struct {
	Status  int      `json:"status"`
	Message string   `json:"message"`
	Details []Detail `json:"details,omitempty"`
}

// ProblemError is a Problem received by a client: an HTTPError whose
// details a caller can attribute to columns.
type ProblemError struct {
	Problem
}

func (e *ProblemError) Error() string     { return e.Message }
func (e *ProblemError) StatusCode() int   { return e.Status }
func (e *ProblemError) Details() []Detail { return e.Problem.Details }
func (e *ProblemError) Is(target error) bool {
	if t, ok := target.(*statusError); ok {
		return t.code == e.Status
	}
	return false
}
