package main

import (
	"strings"
	"testing"

	"github.com/Piechutowski/volt/examples/validation/app/client"
)

// TestFieldErrorsReachTheirFields submits the form the window shows: an
// empty one is refused by required and EmailShape (422), a good one is
// stored, the same email again is refused by unique (409), and each
// error lands under its own field.
func TestFieldErrorsReachTheirFields(t *testing.T) {
	base, err := serve()
	if err != nil {
		t.Fatal(err)
	}
	r := &Root{client: client.New(base)}
	for _, c := range []struct {
		email, name string
		status      string
		errs        map[string]string
	}{
		{"", "", "422", map[string]string{"email": "required: email is required; EmailShape: is not an email address", "name": "required: name is required"}},
		{"nope", "Ada", "422", map[string]string{"email": "EmailShape: is not an email address"}},
		{"ada@example.com", "Ada", "stored", map[string]string{}},
		{"ada@example.com", "Grace", "409", map[string]string{"email": "unique: already exists"}},
	} {
		r.email.ForceSetValue(c.email)
		r.name.ForceSetValue(c.name)
		r.submit()
		if !strings.Contains(r.last, c.status) || len(r.errs) != len(c.errs) {
			t.Errorf("%q %q: %q, errors %v", c.email, c.name, r.last, r.errs)
		}
		for col, msg := range c.errs {
			if r.errs[col] != msg {
				t.Errorf("%q %q: %s shows %q, want %q", c.email, c.name, col, r.errs[col], msg)
			}
		}
	}
	if len(r.rows) != 1 || !strings.Contains(r.rows[0], "ada@example.com") {
		t.Errorf("members %v", r.rows)
	}
}
