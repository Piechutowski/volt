// Package db is the itest data fixture: the schema's Go-reference
// check target (§validators) beside the generated models and queries.
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

// EmailShape is the validator users declares on email.
func EmailShape(email string) error {
	if !strings.Contains(email, "@") || strings.HasPrefix(email, "@") || strings.HasSuffix(email, "@") {
		return fmt.Errorf("%q does not look like an address", email)
	}
	return nil
}
