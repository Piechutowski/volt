// Hand-written Go-reference check target (spec §validators): the generated
// validator calls it directly — same package, no import, and the Go
// compiler proves the signature.
package itest

import (
	"fmt"
	"strings"
)

// EmailValid is the func <name>(<column types>) error contract of a
// Go-reference check.
func EmailValid(email string) error {
	if !strings.Contains(email, "@") {
		return fmt.Errorf("%q is not an email address", email)
	}
	return nil
}

// CountsPositive is the validator page_views declares on hits and day.
func CountsPositive(hits, day int) error {
	if hits < 0 || day < 1 {
		return fmt.Errorf("hits %d and day %d must be non-negative and positive", hits, day)
	}
	return nil
}

// SiteShape is the validator page_views declares on site.
func SiteShape(site string) error {
	if site == "" {
		return fmt.Errorf("site must not be empty")
	}
	return nil
}
