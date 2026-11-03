// Package db is Voltmeter's data package: the schema, selects and
// migrations of schema.volt, selects.volt and migrations.volt, the
// generated models and queries beside them, and the value sets and
// validators the schema names (spec §types, §validators).
package db

import (
	"errors"
	"fmt"
	"math"
	"path"
	"strings"
	"time"

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
	if !(maxMS > 0) || math.IsInf(maxMS, 1) {
		return errors.New("the ceiling must be a finite number above zero")
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

// MillisMeasured refuses a negative or non-finite wall time (spec
// §validators).
func MillisMeasured(ms units.Millis) error {
	if !measured(float64(ms)) {
		return fmt.Errorf("wall time %v is not a measurement", ms)
	}
	return nil
}

// TablesCounted refuses a size below one table (spec §validators): the
// sweep never measures an empty project.
func TablesCounted(tables int) error {
	if tables < 1 {
		return fmt.Errorf("%d tables is not a size the sweep measures", tables)
	}
	return nil
}

// AllocsCounted refuses a negative allocation count (spec §validators).
func AllocsCounted(allocs int) error {
	if allocs < 0 {
		return fmt.Errorf("%d allocations is negative", allocs)
	}
	return nil
}

// KeystrokeSane refuses a negative or non-finite cost per keystroke
// (spec §validators).
func KeystrokeSane(msPerOp, mbPerOp float64, allocsPerOp int) error {
	if !measured(msPerOp, mbPerOp) || allocsPerOp < 0 {
		return errors.New("a keystroke's time, bytes and allocations are finite and never negative")
	}
	return nil
}

// SampleSane refuses a timed call no HTTP exchange produces (spec
// §validators): a status outside 100 to 599 or a negative or non-finite
// duration.
func SampleSane(status int, ms float64) error {
	if status < 100 || status > 599 {
		return fmt.Errorf("%d is not an HTTP status", status)
	}
	if !measured(ms) {
		return fmt.Errorf("a call cannot take %v ms", ms)
	}
	return nil
}

// MegabytesMeasured refuses a negative or non-finite size (spec
// §validators): bytes allocated, held and emitted, and a schema's size,
// are finite and never below zero.
func MegabytesMeasured(allocMB, heapMB, outputMB, sourceKB float64) error {
	if !measured(allocMB, heapMB, outputMB, sourceKB) {
		return errors.New("a phase's allocated, heap, output and source sizes are finite and never negative")
	}
	return nil
}

// MachineNamed refuses a machine missing a part of its identity (spec
// §validators): the processor, the system, the architecture and the Go
// version together tell one machine from another.
func MachineNamed(cpu, os, arch, toolchain string) error {
	if cpu == "" || os == "" || arch == "" || toolchain == "" {
		return errors.New("a machine names its processor, system, architecture and Go version")
	}
	return nil
}

// MachineSized refuses a machine with no processor or a negative or
// non-finite memory (spec §validators); zero memory is what perfsweep.Info reports
// on a system it cannot ask.
func MachineSized(cores int, memoryGB float64) error {
	if cores < 1 {
		return fmt.Errorf("%d cores is not a machine", cores)
	}
	if !measured(memoryGB) {
		return fmt.Errorf("%v GiB of memory is not a machine", memoryGB)
	}
	return nil
}

// ShaShape refuses a commit id that is not lower-case hex, as
// units.CommitParse reads one (spec §validators).
func ShaShape(sha string) error {
	c, err := units.CommitParse(sha)
	if err != nil {
		return err
	}
	if string(c) != sha {
		return fmt.Errorf("commit %q is not lower-case", sha)
	}
	return nil
}

// CommitDated refuses a commit without an author date (spec
// §validators).
func CommitDated(at time.Time) error {
	if at.IsZero() {
		return errors.New("a commit carries its author date")
	}
	return nil
}

// FixtureShape refuses a fixture that is not a directory of the
// repository written as git writes it: relative, slash-separated and
// clean (spec §validators).
func FixtureShape(fixture string) error {
	if fixture == "" || path.IsAbs(fixture) || path.Clean(fixture) != fixture || fixture == "." || fixture == ".." || strings.HasPrefix(fixture, "../") {
		return fmt.Errorf("%q is not a directory of the repository", fixture)
	}
	return nil
}

// LinesCounted refuses a negative line count (spec §validators).
func LinesCounted(voltLines, handLines, genLines int) error {
	if voltLines < 0 || handLines < 0 || genLines < 0 {
		return errors.New("a line count is never negative")
	}
	return nil
}

func measured(vs ...float64) bool {
	for _, v := range vs {
		if !(v >= 0) || math.IsInf(v, 1) {
			return false
		}
	}
	return true
}
