// Package db is Voltmeter's data package: the schema, selects and
// migrations of schema.volt, selects.volt and migrations.volt, the
// generated models and queries beside them, and the value sets and
// validators the schema names (spec §types, §validators).
package db

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Piechutowski/volt/examples/voltmeter/units"
)

// Phase names one phase of the profiling sweep, a value set stored as
// text (spec §types).
type Phase string

const (
	PhaseLoad     Phase = "load"
	PhaseCheck    Phase = "check"
	PhaseVet      Phase = "vet"
	PhaseGenerate Phase = "generate"
)

// Phases are the sweep's phases in the order they run on one project.
var Phases = []Phase{PhaseLoad, PhaseCheck, PhaseVet, PhaseGenerate}

// PhaseKnown refuses a phase outside the constants (spec §validators).
func PhaseKnown(p Phase) error {
	switch p {
	case PhaseLoad, PhaseCheck, PhaseVet, PhaseGenerate:
		return nil
	}
	return fmt.Errorf("%q is not a phase", string(p))
}

// Health is whether a budget holds, a value set stored as text
// (spec §types).
type Health string

const (
	HealthOK       Health = "ok"
	HealthBreached Health = "breached"
)

// HealthKnown refuses a state outside the constants (spec §validators).
func HealthKnown(s Health) error {
	switch s {
	case HealthOK, HealthBreached:
		return nil
	}
	return fmt.Errorf("%q is not a budget state", string(s))
}

// BudgetSane refuses a budget no measurement can meet or miss (spec
// §validators).
func BudgetSane(tables int, maxMS float64) error {
	if tables < 1 {
		return errors.New("tables must be at least 1")
	}
	if maxMS <= 0 {
		return errors.New("the ceiling must be above zero")
	}
	return nil
}

// EmailShape refuses an address without a local part and a domain (spec
// §validators).
func EmailShape(email string) error {
	local, domain, ok := strings.Cut(email, "@")
	if !ok || local == "" || !strings.Contains(domain, ".") {
		return fmt.Errorf("%q is not an email address", email)
	}
	return nil
}

// MillisMeasured refuses a negative wall time (spec §validators).
func MillisMeasured(ms units.Millis) error {
	if ms < 0 {
		return fmt.Errorf("wall time %v is negative", ms)
	}
	return nil
}
