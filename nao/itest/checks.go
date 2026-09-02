// Hand-written Go-reference check target (spec §V12.5): the generated
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
