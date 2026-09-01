// Validation support for generated validators (docs/spec.md §V12):
// the error type every failed check reports, and the LIKE evaluator
// that mirrors SQLite's operator so the Go tier and the SQL tier of a
// typed check agree.
package rt

import "fmt"

// CheckError is one failed schema check (spec §V12). Typed checks
// carry no Cause; a Go-reference check wraps the error the referenced
// function returned.
type CheckError struct {
	Model string // the Go model name, e.g. "User"
	Check string // the check's [name:] when set, else its rendered form
	Cause error  // non-nil only for Go-reference checks
}

func (e CheckError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: check %s: %v", e.Model, e.Check, e.Cause)
	}
	return fmt.Sprintf("%s: check %s failed", e.Model, e.Check)
}

func (e CheckError) Unwrap() error { return e.Cause }

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
