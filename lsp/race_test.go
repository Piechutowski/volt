//go:build race

package lsp

// raceEnabled reports a race-detector build, whose instrumentation
// changes what a run allocates.
const raceEnabled = true
