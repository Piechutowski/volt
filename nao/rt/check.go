// Validation support for generated validators (docs/spec.md §V12):
// the error type every failed check reports, and the LIKE evaluator
// that mirrors SQLite's operator so the Go tier and the SQL tier of a
// typed check agree.
package rt

import (
	"fmt"
	"strings"
)

// CheckError is one failed schema check (spec §V12). Typed checks
// carry no Cause; a Go-reference check wraps the error the referenced
// function returned.
type CheckError struct {
	Model   string   // the Go model name, e.g. "User"
	Check   string   // the check's [name:] when set, else its rendered form
	Columns []string // the columns the check reads, for attributing the failure to fields
	Cause   error    // non-nil only for Go-reference checks
}

// Detail is the itemized form the volt runtime renders into an error
// body: the check, its columns, the message. It aliases the same
// unnamed struct the volt runtime aliases, so Details() satisfies
// volt's Detailer interface without either package importing the
// other (nao/rt stays stdlib-only, D03).
type Detail = struct {
	Check   string   `json:"check"`
	Columns []string `json:"columns,omitempty"`
	Message string   `json:"message"`
}

// Details itemizes the one failure (the volt Detailer contract).
func (e CheckError) Details() []Detail {
	return []Detail{{Check: e.Check, Columns: e.Columns, Message: e.Error()}}
}

func (e CheckError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: check %s: %v", e.Model, e.Check, e.Cause)
	}
	return fmt.Sprintf("%s: check %s failed", e.Model, e.Check)
}

func (e CheckError) Unwrap() error { return e.Cause }

// StatusCode makes a failed check an HTTP 422 (Unprocessable Content)
// wherever the volt error spine or a generated client sees it: the
// request was well-formed, the values were not.
func (e CheckError) StatusCode() int { return 422 }

// ValidationError is every check a Validate call found failing, in
// declaration order. It unwraps to its members, so errors.As finds the
// first CheckError and errors.Is matches any of them, and it is a 422
// as a whole.
type ValidationError []CheckError

func (e ValidationError) Error() string {
	parts := make([]string, len(e))
	for i, ce := range e {
		parts[i] = ce.Error()
	}
	return strings.Join(parts, "; ")
}

// Unwrap exposes the members to errors.As and errors.Is.
func (e ValidationError) Unwrap() []error {
	out := make([]error, len(e))
	for i, ce := range e {
		out[i] = ce
	}
	return out
}

// StatusCode is 422: the values violate the schema's checks.
func (e ValidationError) StatusCode() int { return 422 }

// Details itemizes every failure with its columns.
func (e ValidationError) Details() []Detail {
	out := make([]Detail, len(e))
	for i, ce := range e {
		out[i] = ce.Details()[0]
	}
	return out
}

// Validation is what a generated Validate returns: nil when every
// check passed, else the failures as one ValidationError.
func Validation(errs []CheckError) error {
	if len(errs) == 0 {
		return nil
	}
	return ValidationError(errs)
}

// ConstraintError is a database refusal translated into a status: the
// row violated a constraint the schema declared. Kind is one of
// "unique", "check", "foreign key" and "not null"; Detail is what
// SQLite named — the column (table.column) for unique and not null,
// the check's name or expression for check, nothing for foreign key.
type ConstraintError struct {
	Kind   string
	Detail string
	Cause  error
}

func (e ConstraintError) Error() string {
	switch e.Kind {
	case "unique":
		return e.Detail + " already exists"
	case "not null":
		return e.Detail + " must not be null"
	case "check":
		return "check " + e.Detail + " failed"
	}
	return e.Kind + " constraint failed"
}

func (e ConstraintError) Unwrap() error { return e.Cause }

// Details itemizes the refusal: for unique and not-null the column
// SQLite named (table.column, reduced to the column), for a check its
// name, so a form can still mark a field.
func (e ConstraintError) Details() []Detail {
	d := Detail{Check: e.Kind, Message: e.Error()}
	switch e.Kind {
	case "unique", "not null":
		col := e.Detail
		if i := strings.LastIndexByte(col, '.'); i >= 0 {
			col = col[i+1:]
		}
		d.Columns = []string{col}
	case "check":
		d.Check = e.Detail
	}
	return []Detail{d}
}

// StatusCode is 409 (Conflict) for a duplicate key and 422 for every
// other constraint: the values are unprocessable as given.
func (e ConstraintError) StatusCode() int {
	if e.Kind == "unique" {
		return 409
	}
	return 422
}

// Constraint translates a driver error into a ConstraintError when it
// is a SQLite constraint failure, and returns any other error
// unchanged. Generated queries pass every write's error through it.
// The messages are SQLite's own and identical across drivers:
// "UNIQUE constraint failed: users.email", "CHECK constraint failed:
// counts_positive", "FOREIGN KEY constraint failed", "NOT NULL
// constraint failed: users.email".
func Constraint(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	for _, kind := range []string{"UNIQUE", "CHECK", "FOREIGN KEY", "NOT NULL"} {
		prefix := kind + " constraint failed"
		i := strings.Index(msg, prefix)
		if i < 0 {
			continue
		}
		detail := strings.TrimSpace(strings.TrimPrefix(msg[i+len(prefix):], ":"))
		return ConstraintError{Kind: strings.ToLower(kind), Detail: detail, Cause: err}
	}
	return err
}

// Like reports whether s matches pattern under SQLite's LIKE operator
// with no ESCAPE clause: '%' matches any sequence (empty included),
// '_' matches exactly one character, and letters compare
// case-insensitively for ASCII only — SQLite's documented default.
// Generated validators call this so a typed check's Go tier evaluates
// exactly like its SQL tier (spec §V12).
func Like(s, pattern string) bool {
	return likeMatch([]rune(s), []rune(pattern))
}

func likeMatch(s, p []rune) bool {
	for len(p) > 0 {
		switch p[0] {
		case '%':
			// Collapse a run of wildcards, then try every suffix.
			for len(p) > 0 && p[0] == '%' {
				p = p[1:]
			}
			if len(p) == 0 {
				return true
			}
			for i := 0; i <= len(s); i++ {
				if likeMatch(s[i:], p) {
					return true
				}
			}
			return false
		case '_':
			if len(s) == 0 {
				return false
			}
			s, p = s[1:], p[1:]
		default:
			if len(s) == 0 || asciiFold(s[0]) != asciiFold(p[0]) {
				return false
			}
			s, p = s[1:], p[1:]
		}
	}
	return len(s) == 0
}

// asciiFold lower-cases ASCII letters only, matching SQLite's LIKE.
func asciiFold(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	return r
}
