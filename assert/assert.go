// Package assert checks invariants: states that are impossible unless the
// code is wrong.
//
// Use errors, not assertions, for anything that can legitimately happen at
// runtime: bad input, network and disk failures, timeouts, running out of
// resources. A failed assertion means the program's model of its own state is
// wrong, so the only safe response is to stop.
//
// A failed assertion panics. net/http recovers panics in handlers and keeps the
// server running, so wrap your handler with [Crash] to make a failed assertion
// stop the process.
package assert

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
)

// failure is the panic value of a failed assertion. It is unexported so that
// only this package can create one, and it is deliberately not an error:
// assertion failures are never returned or handled.
type failure struct {
	msg  string
	file string
	line int
}

// String is printed by the runtime when a failed assertion is not recovered.
func (f failure) String() string {
	if f.file == "" {
		return "assertion failed: " + f.msg
	}
	return fmt.Sprintf("assertion failed: %s (%s:%d)", f.msg, f.file, f.line)
}

// That panics if ok is false.
//
// Keep msg a constant string. Arguments are evaluated even when ok is true, so
// formatting a message here costs time and allocations on every call. To put
// values in the message, use [Fail] inside an if statement.
func That(ok bool, msg string) {
	if !ok {
		fail(msg)
	}
}

// Fail panics unconditionally. Use it for code that must be unreachable and for
// messages that need values, so the formatting only happens on failure:
//
//	if balance < 0 {
//		assert.Fail("balance %d is negative", balance)
//	}
func Fail(format string, args ...any) {
	fail(fmt.Sprintf(format, args...))
}

// Maybe documents that cond is legitimately sometimes true and sometimes
// false, so a reader knows both cases were considered. It does nothing at
// runtime.
func Maybe(cond bool) {}

// IsFailure reports whether v, a value returned by recover, came from a failed
// assertion. It is meant for tests that check an assertion trips.
func IsFailure(v any) bool {
	_, ok := v.(failure)
	return ok
}

// Crash wraps an HTTP handler so that a failed assertion prints its message and
// stack to stderr and exits the process with status 2, instead of being
// recovered and logged by net/http. Other panics, including
// http.ErrAbortHandler, are passed on unchanged.
//
// Wrap the whole router once:
//
//	http.ListenAndServe(addr, assert.Crash(mux))
func Crash(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if f, ok := v.(failure); ok {
				if _, err := fmt.Fprintf(os.Stderr, "%s\n\n%s", f, debug.Stack()); err != nil {
					os.Exit(2)
				}
				os.Exit(2)
			}
			panic(v)
		}()
		next.ServeHTTP(w, r)
	})
}

// fail builds and panics with a failure. It is kept out of line so that That
// stays small enough to be inlined at every call site.
//
//go:noinline
func fail(msg string) {
	f := failure{msg: msg}
	// Skip fail and the exported function that called it, so the location is
	// the line that made the assertion.
	if _, file, line, ok := runtime.Caller(2); ok {
		f.file, f.line = file, line
	}
	panic(f)
}
