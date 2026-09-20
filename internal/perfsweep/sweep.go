// Package perfsweep is the profiling sweep (PERF-2): one-file projects
// of growing table counts, every phase timed, measured and profiled on
// its own, the numbers charted and rendered as a Markdown report. The
// test lang.TestProfileSweep runs it at the reference sizes; the
// script scripts/perf.go runs it on any machine and writes the report
// with that machine's specification. Wall clock is reported, never
// asserted.
package perfsweep

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/Piechutowski/volt/gen/model"
	"github.com/Piechutowski/volt/gen/router"
	"github.com/Piechutowski/volt/internal/corpus"
	"github.com/Piechutowski/volt/lang"
)

// Phases are the measured phases, in the order they run on one project.
var Phases = []string{"load", "check", "vet", "generate"}

// Spec says what to sweep.
type Spec struct {
	Sizes   []int // table counts, ascending
	Columns int   // columns per table
	Reps    int   // unprofiled timing runs per phase; the best is reported
	// Profile is how much CPU time the profiled runs of a phase should
	// fill; zero skips profiling.
	Profile time.Duration
}

// Row is one measured phase at one size.
type Row struct {
	Tables     int     `json:"tables"`
	Phase      string  `json:"phase"`
	WallMS     float64 `json:"wall_ms"`     // best of the repetitions
	AllocMB    float64 `json:"alloc_mb"`    // bytes allocated by one run
	HeapMB     float64 `json:"heap_mb"`     // heap in use right after the run, before collection
	Allocs     uint64  `json:"allocs"`      // allocations by one run
	OutputMB   float64 `json:"output_mb"`   // generate only: bytes emitted
	SourceKB   float64 `json:"source_kb"`   // the schema.volt size
	Profile    string  `json:"profile"`     // CPU profile file, "" when not profiled
	TopByFlat  string  `json:"top_by_flat"` // pprof -top, flat order
	TopByCum   string  `json:"top_by_cum"`  // pprof -top -cum
	Iterations int     `json:"iterations"`  // runs under the profile
}

// Run sweeps the sizes, writing the CPU profiles and sweep.json into
// dir, and reports each row as it lands through log.
func Run(dir string, spec Spec, log func(format string, args ...any)) ([]Row, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var rows []Row
	for _, tables := range spec.Sizes {
		root, err := os.MkdirTemp("", "volt-sweep-")
		if err != nil {
			return nil, err
		}
		if err := corpus.Write(root, corpus.Spec{Tables: tables, Columns: spec.Columns, Single: true}); err != nil {
			return nil, err
		}
		st, err := os.Stat(filepath.Join(root, "schema.volt"))
		if err != nil {
			return nil, err
		}
		var pr *lang.Project
		var phaseErr error
		phases := []struct {
			name string
			run  func() int
		}{
			{"load", func() int {
				if pr, phaseErr = lang.Load(root); phaseErr != nil {
					return 0
				}
				return 0
			}},
			{"check", func() int { lang.Check(pr); return 0 }},
			{"vet", func() int { lang.Vet(pr); return 0 }},
			{"generate", func() int {
				n, err := generate(pr)
				if err != nil {
					phaseErr = err
				}
				return n
			}},
		}
		for _, ph := range phases {
			row := Row{Tables: tables, Phase: ph.name, SourceKB: float64(st.Size()) / 1024, Iterations: spec.Reps}
			// Timing runs, unprofiled: the best wall time and one run's
			// allocation and heap.
			best := time.Duration(1<<62 - 1)
			for i := 0; i < max(spec.Reps, 1); i++ {
				runtime.GC()
				var before, after runtime.MemStats
				runtime.ReadMemStats(&before)
				t0 := time.Now()
				out := ph.run()
				d := time.Since(t0)
				runtime.ReadMemStats(&after)
				if phaseErr != nil {
					return nil, fmt.Errorf("%d tables, %s: %w", tables, ph.name, phaseErr)
				}
				if d < best {
					best = d
				}
				row.AllocMB = float64(after.TotalAlloc-before.TotalAlloc) / (1 << 20)
				row.Allocs = after.Mallocs - before.Mallocs
				row.HeapMB = float64(after.HeapInuse) / (1 << 20)
				row.OutputMB = float64(out) / (1 << 20)
			}
			row.WallMS = float64(best.Microseconds()) / 1000
			if spec.Profile > 0 {
				// The profiled runs, enough of them to register.
				prof := filepath.Join(dir, fmt.Sprintf("%s_%03d.prof", ph.name, tables))
				f, err := os.Create(prof)
				if err != nil {
					return nil, err
				}
				iters := max(1, int(spec.Profile/max(best, time.Millisecond)))
				if iters > 200 {
					iters = 200
				}
				row.Iterations = iters
				if err := pprof.StartCPUProfile(f); err != nil {
					return nil, err
				}
				for i := 0; i < iters; i++ {
					ph.run()
				}
				pprof.StopCPUProfile()
				f.Close()
				row.Profile = filepath.Base(prof)
				if row.TopByFlat, err = pprofTop(prof, false); err != nil {
					return nil, err
				}
				if row.TopByCum, err = pprofTop(prof, true); err != nil {
					return nil, err
				}
			}
			rows = append(rows, row)
			if log != nil {
				log("%3d tables %-8s %8.1f ms %8.2f MB alloc %7d allocs heap %6.1f MB", tables, ph.name, row.WallMS, row.AllocMB, row.Allocs, row.HeapMB)
			}
		}
		os.RemoveAll(root)
	}
	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "sweep.json"), data, 0o644); err != nil {
		return nil, err
	}
	return rows, nil
}

// generate renders every file of every package and returns the bytes
// emitted.
func generate(pr *lang.Project) (int, error) {
	n := 0
	for path, pkg := range pr.Packages {
		if pkg.HasSchema() {
			files, err := model.Generate(pkg, model.Options{Source: "package " + path, SQL: true})
			if err != nil {
				return 0, err
			}
			for _, f := range files {
				n += len(f.Code)
			}
		}
		if pkg.HasRouting() {
			files, err := router.Generate(pkg, router.Options{Source: "package " + path})
			if err != nil {
				return 0, err
			}
			for _, code := range files {
				n += len(code)
			}
		}
	}
	return n, nil
}

// pprofTop renders the profile's top functions, volt's own and the
// runtime's, as pprof prints them.
func pprofTop(prof string, cum bool) (string, error) {
	args := []string{"tool", "pprof", "-top", "-nodecount=25"}
	if cum {
		args = append(args, "-cum")
	}
	args = append(args, prof)
	out, err := exec.Command("go", args...).Output()
	if err != nil {
		return "", fmt.Errorf("pprof %s: %w", prof, err)
	}
	// Drop the header down to the column line.
	s := string(out)
	if i := strings.Index(s, "      flat  flat%"); i >= 0 {
		s = s[i:]
	}
	return s, nil
}

// Sizes returns the table counts of the rows, ascending, each once.
func Sizes(rows []Row) []int {
	var xs []int
	for _, r := range rows {
		if r.Phase == Phases[0] {
			xs = append(xs, r.Tables)
		}
	}
	return xs
}

// At returns the row of one phase at one size, or nil.
func At(rows []Row, tables int, phase string) *Row {
	for i := range rows {
		if rows[i].Tables == tables && rows[i].Phase == phase {
			return &rows[i]
		}
	}
	return nil
}
