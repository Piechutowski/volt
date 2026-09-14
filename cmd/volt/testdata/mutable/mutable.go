// Package mutable is the immutability gate's fixture (D92): a producer
// and the ways its result may be touched afterwards, so the gate is
// proven to refuse each write and to pass the rest.
package mutable

import "sort"

type Result struct {
	Rows  []Row
	Index map[string]int
}

type Row struct{ N int }

// Produce is the memoized computation: a fresh Result.
func Produce(n int) *Result {
	r := &Result{Index: map[string]int{}}
	r.Rows = append(r.Rows, Row{n})
	return r
}

// Tamper writes a result it was handed.
func Tamper(r *Result) { r.Rows[0].N++ }

// TamperByCallee hands a result to a callee that writes it.
func TamperByCallee(r *Result) { touch(r) }

func touch(r *Result) { r.Index["x"] = 1 }

// Sorts sorts a result's rows in place: a write by a standard mutator.
func Sorts(r *Result) {
	sort.Slice(r.Rows, func(i, j int) bool { return r.Rows[i].N < r.Rows[j].N })
}

// Rebuilds writes the result of the producer it just called.
func Rebuilds() *Result {
	r := Produce(1)
	r.Rows = nil
	return r
}

// Fresh writes a result of its own making.
func Fresh() *Result {
	r := &Result{}
	r.Rows = append(r.Rows, Row{1})
	r.Rows[0].N = 2
	return r
}

// Copies writes a row it holds by value: its own copy.
func Copies(r *Result) int {
	row := r.Rows[0]
	row.N = 3
	return row.N
}

// Reads reads a result.
func Reads(r *Result) int { return r.Rows[0].N + r.Index["x"] }
