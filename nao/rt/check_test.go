package rt

import (
	"errors"
	"strings"
	"testing"
)

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

func TestConstraintTranslation(t *testing.T) {
	cases := []struct {
		msg, kind, detail string
		status            int
	}{
		{"UNIQUE constraint failed: users.email", "unique", "users.email", 409},
		{"CHECK constraint failed: counts_positive", "check", "counts_positive", 422},
		{"CHECK constraint failed: site LIKE '%_'", "check", "site LIKE '%_'", 422},
		{"FOREIGN KEY constraint failed", "foreign key", "", 422},
		{"NOT NULL constraint failed: users.email", "not null", "users.email", 422},
	}
	for _, tc := range cases {
		err := Constraint(errors.New(tc.msg))
		var ce ConstraintError
		if !errors.As(err, &ce) || ce.Kind != tc.kind || ce.Detail != tc.detail || ce.StatusCode() != tc.status {
			t.Errorf("%q → %+v (status %d)", tc.msg, ce, ce.StatusCode())
		}
	}
	plain := errors.New("disk I/O error")
	if Constraint(plain) != plain {
		t.Error("a non-constraint error must pass through unchanged")
	}
	if Constraint(nil) != nil {
		t.Error("nil must stay nil")
	}
}

func TestValidationErrorShape(t *testing.T) {
	if Validation(nil) != nil {
		t.Fatal("no failures must be nil")
	}
	err := Validation([]CheckError{{Model: "User", Check: "a"}, {Model: "User", Check: "b"}})
	var ce CheckError
	if !errors.As(err, &ce) || ce.Check != "a" {
		t.Errorf("errors.As should find the first failure: %v", err)
	}
	if sc, ok := err.(interface{ StatusCode() int }); !ok || sc.StatusCode() != 422 {
		t.Errorf("a validation failure is a 422: %v", err)
	}
	if !strings.Contains(err.Error(), "check a failed; User: check b failed") {
		t.Errorf("message = %q", err.Error())
	}
}
