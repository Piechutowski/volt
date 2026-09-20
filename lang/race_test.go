//go:build race

package lang_test

// raceEnabled reports a race-detector build, whose instrumentation
// changes what a phase allocates: the budget is asserted only without it.
const raceEnabled = true
