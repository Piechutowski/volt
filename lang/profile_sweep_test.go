package lang_test

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
