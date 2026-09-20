// Package impure is the purity gate's negative fixture: each function
// does one thing a memoized computation must not, so the gate is
// proven to refuse each.
package impure

import (
	"os"
	"sort"
)

var counter int

type Table struct{ Rows []string }

// ReadsGlobal reads mutable package state.
func ReadsGlobal(n int) int { return n + counter }

// WritesInput writes through its input.
func WritesInput(t *Table) { t.Rows = append(t.Rows, "x") }

// WritesInputByCallee hands its input to a callee that writes it.
func WritesInputByCallee(t *Table) { mutate(t) }

func mutate(t *Table) { t.Rows = nil }

// CallsOS calls into a package that keeps state.
func CallsOS(name string) bool { _, err := os.Stat(name); return err == nil }

// WritesResultOfInput writes through a value a callee derived from an input.
func WritesResultOfInput(t *Table) { first(t).Rows = nil }

func first(t *Table) *Table { return t }

// Pure is what the gate allows: a value of its own, mutated, returned.
func Pure(t *Table) *Table {
	out := &Table{}
	out.Rows = append(out.Rows, t.Rows...)
	return out
}

// SortsInput sorts its input in place: a write by a standard mutator.
func SortsInput(t *Table) { sort.Strings(t.Rows) }

// Namer is an interface one of whose implementations is impure.
type Namer interface{ Name() string }

// Noisy counts its calls.
type Noisy struct{}

func (Noisy) Name() string { counter++; return "noisy" }

// Quiet answers from itself.
type Quiet struct{ N string }

func (q *Quiet) Name() string { return q.N }

// CallsInterface calls a method that one implementation makes impure.
func CallsInterface(n Namer) string { return n.Name() }

// AppendsToInput appends to a slice of its input: with spare capacity,
// the write lands in the input's own array.
func AppendsToInput(t *Table) []string { return append(t.Rows, "x") }

// CallsFuncValue calls a function value it did not make, which could
// do anything.
func CallsFuncValue(t *Table, f func(*Table)) { f(t) }

// Hooked carries a function value.
type Hooked struct{ Hook func() }

// CallsFuncField calls a function value held by its input.
func CallsFuncField(h *Hooked) { h.Hook() }

// CallsOwnClosure calls a closure it made, whose body the walk covers.
func CallsOwnClosure(t *Table) int {
	f := func() int { return len(t.Rows) }
	return f()
}

// RecursesThenWrites writes through what a recursive call returns:
// the input, once the recursion bottoms out.
func RecursesThenWrites(t *Table, n int) *Table {
	if n == 0 {
		return t
	}
	r := RecursesThenWrites(t, n-1)
	r.Rows = nil
	return r
}

// AppendsToOwn appends to a slice of its own making, elements from
// the input included: fresh memory, allowed.
func AppendsToOwn(t *Table) []string {
	out := append([]string{}, t.Rows...)
	out = append(out, "x")
	return out
}

// AppendsToAlias appends to a local that aliases the input's array.
func AppendsToAlias(t *Table) []string {
	rows := t.Rows
	return append(rows, "x")
}

// RecursesPurely recurses and writes nothing: allowed.
func RecursesPurely(t *Table, n int) int {
	if n == 0 {
		return len(t.Rows)
	}
	return RecursesPurely(t, n-1) + 1
}
