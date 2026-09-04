// Package db is the itest data fixture: the schema's Go-reference
// check target (§V12.5) beside the generated models and queries.
package db

import (
	"fmt"
	"strings"
)

// EmailValid is the Go-reference check users declares.
func EmailValid(email string) error {
	if !strings.Contains(email, "@") {
		return fmt.Errorf("%q is not an email address", email)
	}
	return nil
}
