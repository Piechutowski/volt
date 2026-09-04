package db

import (
	"fmt"
	"strings"
)

// EmailValid is the Go-reference check the schema names (§V12.5). Go
// ignores this testdata directory; the Volt checker reads it.
func EmailValid(email string) error {
	if !strings.Contains(email, "@") {
		return fmt.Errorf("%q is not an email address", email)
	}
	return nil
}
