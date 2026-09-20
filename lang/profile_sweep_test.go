package lang_test

// The profiling sweep (PERF-2) at the reference sizes: one-file
// projects of growing table counts, every phase timed and profiled on
// its own, numbers, profiles and charts written for the report under
// docs/reference. The method is internal/perfsweep, which
// scripts/perf.go runs on any machine with its specification. Skipped
// unless VOLT_SWEEP_DIR names the output directory:
//
//	VOLT_SWEEP_DIR=/tmp/sweep go test ./lang -run TestProfileSweep -count=1 -timeout 30m
//
// Wall clock is reported, never asserted.

import (
	"os"
	"testing"
	"time"

	"github.com/Piechutowski/volt/internal/perfsweep"
)

func TestProfileSweep(t *testing.T) {
	dir := os.Getenv("VOLT_SWEEP_DIR")
	if dir == "" {
		t.Skip("set VOLT_SWEEP_DIR to run the profiling sweep")
	}
	spec := perfsweep.Spec{Sizes: []int{10, 20, 40, 80, 160}, Columns: 150, Reps: 3, Profile: 2 * time.Second}
	rows, err := perfsweep.Run(dir, spec, t.Logf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := perfsweep.Charts(dir, rows, spec.Columns); err != nil {
		t.Fatal(err)
	}
}
