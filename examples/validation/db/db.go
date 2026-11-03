// Package db is the data package: schema.volt declares the members
// table, and this file the Go validator it names (§validators).
package db

import (
	"errors"
	"strings"
)

// EmailShape refuses an email without one @ between two non-empty parts.
func EmailShape(email string) error {
	local, domain, ok := strings.Cut(email, "@")
	if !ok || local == "" || domain == "" || strings.Contains(domain, "@") {
		return errors.New("is not an email address")
	}
	return nil
}
