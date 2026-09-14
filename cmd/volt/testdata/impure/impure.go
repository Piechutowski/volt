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
