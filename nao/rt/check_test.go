package rt

import (
	"errors"
	"testing"
)

// TestLikeMirrorsSQLite pins the semantics the generated validators
// rely on (spec §V12): the cases were cross-checked against SQLite's
// LIKE with no ESCAPE clause.
func TestLikeMirrorsSQLite(t *testing.T) {
	cases := []struct {
		s, p string
		want bool
	}{
		{"abc", "abc", true},
		{"abc", "ABC", true},         // ASCII case-insensitive
		{"Straße", "straße", true},   // non-ASCII letters fold nothing…
		{"STRASSE", "strasse", true}, // …but ASCII inside still folds
		{"Straße", "STRASSE", false}, // ß != S
		{"abc", "a%", true},
		{"abc", "%c", true},
		{"abc", "%b%", true},
		{"abc", "a_c", true},
		{"abc", "a_d", false},
		{"abc", "____", false},
		{"abc", "___", true},
		{"", "%", true},
		{"", "_", false},
		{"a%c", "a\\%c", false}, // no ESCAPE clause: backslash is literal
		{"anything", "%%%", true},
		{"mail@example.com", "%@example.com", true},
	}
	for _, c := range cases {
		if got := Like(c.s, c.p); got != c.want {
			t.Errorf("Like(%q, %q) = %v, want %v", c.s, c.p, got, c.want)
		}
	}
}

func TestCheckErrorUnwraps(t *testing.T) {
	cause := errors.New("not an address")
	e := CheckError{Model: "User", Check: "EmailValid(email)", Cause: cause}
	if !errors.Is(e, cause) {
		t.Error("CheckError must unwrap to its cause")
	}
	plain := CheckError{Model: "User", Check: "hits >= 0"}
	if plain.Error() != "User: check hits >= 0 failed" {
		t.Errorf("unexpected message %q", plain.Error())
	}
}
