package db

import "fmt"

// Status is the declared type the schema names (§types).
type Status string

const (
	StatusActive  Status = "active"
	StatusBlocked Status = "blocked"
)

// StatusKnown refuses a status outside the constants (§validators).
func StatusKnown(s Status) error {
	switch s {
	case StatusActive, StatusBlocked:
		return nil
	}
	return fmt.Errorf("%q is not a status", string(s))
}
