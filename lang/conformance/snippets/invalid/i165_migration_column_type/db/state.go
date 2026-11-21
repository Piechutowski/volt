package db

import "fmt"

// State is the state of a user (spec/types).
type State string

const (
	StateActive State = "active"
	StateBanned State = "banned"
)

// StateKnown refuses a state outside the constants (spec/validators).
func StateKnown(s State) error {
	switch s {
	case StateActive, StateBanned:
		return nil
	}
	return fmt.Errorf("%q is not a state", string(s))
}
