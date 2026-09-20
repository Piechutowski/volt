package lang_test

// The profiling sweep (PERF-2): one-file projects of growing table
// counts, every phase timed and profiled on its own, numbers and CPU
// profiles written for the report under docs/reference. Skipped unless
// VOLT_SWEEP_DIR names the output directory:
//
//	VOLT_SWEEP_DIR=/tmp/sweep go test ./lang -run TestProfileSweep -count=1 -timeout 30m
//
// Wall clock is reported, never asserted.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"testing"
	"time"

	"github.com/Piechutowski/volt/gen/model"
	"github.com/Piechutowski/volt/gen/router"
	"github.com/Piechutowski/volt/internal/corpus"
	"github.com/Piechutowski/volt/lang"
)

// sweepPhase is one measured phase at one size.
type sweepPhase struct {
	Tables     int     `json:"tables"`
	Phase      string  `json:"phase"`
	WallMS     float64 `json:"wall_ms"`     // best of the repetitions
	AllocMB    float64 `json:"alloc_mb"`    // bytes allocated by one run
	HeapMB     float64 `json:"heap_mb"`     // heap in use right after the run, before collection
	Allocs     uint64  `json:"allocs"`      // allocations by one run
	OutputMB   float64 `json:"output_mb"`   // generate only: bytes emitted
	SourceKB   float64 `json:"source_kb"`   // the schema.volt size
	Profile    string  `json:"profile"`     // CPU profile file
	TopByFlat  string  `json:"top_by_flat"` // pprof -top, flat order
	TopByCum   string  `json:"top_by_cum"`  // pprof -top -cum
	Iterations int     `json:"iterations"`  // runs under the profile
}

func TestProfileSweep(t *testing.T) {
	dir := os.Getenv("VOLT_SWEEP_DIR")
	if dir == "" {
		t.Skip("set VOLT_SWEEP_DIR to run the profiling sweep")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	const reps = 3
	var rows []sweepPhase
	for _, tables := range []int{10, 20, 40, 80, 160} {
		root := corpusRoot(t, corpus.Spec{Tables: tables, Columns: 150, Single: true})
		st, err := os.Stat(filepath.Join(root, "schema.volt"))
		if err != nil {
			t.Fatal(err)
		}
		var pr *lang.Project
		phases := []struct {
			name string
			run  func() int
		}{
			{"load", func() int {
				var err error
				if pr, err = lang.Load(root); err != nil {
					t.Fatal(err)
				}
				return 0
			}},
			{"check", func() int { lang.Check(pr); return 0 }},
			{"vet", func() int { lang.Vet(pr); return 0 }},
			{"generate", func() int { return sweepGenerate(t, pr) }},
		}
		for _, ph := range phases {
			row := sweepPhase{Tables: tables, Phase: ph.name, SourceKB: float64(st.Size()) / 1024, Iterations: reps}
			// Timing runs, unprofiled: the best wall time and one run's
			// allocation and heap.
			best := time.Duration(1<<62 - 1)
			for i := 0; i < reps; i++ {
				runtime.GC()
				var before, after runtime.MemStats
				runtime.ReadMemStats(&before)
				t0 := time.Now()
				out := ph.run()
				d := time.Since(t0)
				runtime.ReadMemStats(&after)
				if d < best {
					best = d
				}
				row.AllocMB = float64(after.TotalAlloc-before.TotalAlloc) / (1 << 20)
				row.Allocs = after.Mallocs - before.Mallocs
				row.HeapMB = float64(after.HeapInuse) / (1 << 20)
				row.OutputMB = float64(out) / (1 << 20)
			}
			row.WallMS = float64(best.Microseconds()) / 1000
			// The profiled runs, enough of them to register.
			prof := filepath.Join(dir, fmt.Sprintf("%s_%03d.prof", ph.name, tables))
			f, err := os.Create(prof)
			if err != nil {
				t.Fatal(err)
			}
			iters := max(1, int(2*time.Second/max(best, time.Millisecond)))
			if iters > 200 {
				iters = 200
			}
			row.Iterations = iters
			if err := pprof.StartCPUProfile(f); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < iters; i++ {
				ph.run()
			}
			pprof.StopCPUProfile()
			f.Close()
			row.Profile = filepath.Base(prof)
			row.TopByFlat = pprofTop(t, prof, false)
			row.TopByCum = pprofTop(t, prof, true)
			rows = append(rows, row)
			t.Logf("%3d tables %-8s %8.1f ms %8.2f MB alloc %6d allocs heap %6.1f MB", tables, ph.name, row.WallMS, row.AllocMB, row.Allocs, row.HeapMB)
		}
	}
	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sweep.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	sweepCharts(t, dir, rows)
}

// sweepGenerate renders every file of every package and returns the
// bytes emitted.
func sweepGenerate(t testing.TB, pr *lang.Project) int {
	t.Helper()
	n := 0
	for path, pkg := range pr.Packages {
		if pkg.HasSchema() {
			files, err := model.Generate(pkg, model.Options{Source: "package " + path, SQL: true})
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range files {
				n += len(f.Code)
			}
		}
		if pkg.HasRouting() {
			files, err := router.Generate(pkg, router.Options{Source: "package " + path})
			if err != nil {
				t.Fatal(err)
			}
			for _, code := range files {
				n += len(code)
			}
		}
	}
	return n
}

// pprofTop renders the profile's top functions, volt's own and the
// runtime's, as pprof prints them.
func pprofTop(t testing.TB, prof string, cum bool) string {
	t.Helper()
	args := []string{"tool", "pprof", "-top", "-nodecount=25"}
	if cum {
		args = append(args, "-cum")
	}
	args = append(args, prof)
	out, err := exec.Command("go", args...).Output()
	if err != nil {
		t.Fatalf("pprof %s: %v", prof, err)
	}
	// Drop the header down to the column line.
	s := string(out)
	if i := strings.Index(s, "      flat  flat%"); i >= 0 {
		s = s[i:]
	}
	return s
}

// sweepCharts draws one SVG per measure — wall time, bytes allocated,
// heap in use — with a line per phase over the table counts, in dark
// mode, for the markdown report.
func sweepCharts(t testing.TB, dir string, rows []sweepPhase) {
	t.Helper()
	phases := []string{"load", "check", "vet", "generate"}
	colors := map[string]string{"load": "#8ab4f8", "check": "#f28b82", "vet": "#fdd663", "generate": "#81c995"}
	var xs []int
	for _, r := range rows {
		if r.Phase == "load" {
			xs = append(xs, r.Tables)
		}
	}
	for _, m := range []struct {
		name, title, unit string
		pick              func(sweepPhase) float64
	}{
		{"wall", "Wall time per phase", "ms", func(r sweepPhase) float64 { return r.WallMS }},
		{"alloc", "Bytes allocated per run", "MB", func(r sweepPhase) float64 { return r.AllocMB }},
		{"heap", "Heap in use after the run", "MB", func(r sweepPhase) float64 { return r.HeapMB }},
		{"allocs", "Allocations per run", "thousands", func(r sweepPhase) float64 { return float64(r.Allocs) / 1000 }},
	} {
		series := map[string][]float64{}
		for _, r := range rows {
			series[r.Phase] = append(series[r.Phase], m.pick(r))
		}
		svg := lineChart(m.title, m.unit, xs, phases, series, colors)
		if err := os.WriteFile(filepath.Join(dir, m.name+".svg"), []byte(svg), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// lineChart renders a dark-mode line chart: x is the table count, one
// line per named series, axes with ticks, a legend.
func lineChart(title, unit string, xs []int, names []string, series map[string][]float64, colors map[string]string) string {
	const w, h, left, right, top, bottom = 760, 420, 70, 20, 50, 50
	maxY := 0.0
	for _, name := range names {
		for _, v := range series[name] {
			maxY = max(maxY, v)
		}
	}
	if maxY == 0 {
		maxY = 1
	}
	maxX := float64(xs[len(xs)-1])
	px := func(x int) float64 { return left + float64(x)/maxX*(w-left-right) }
	py := func(v float64) float64 { return h - bottom - v/maxY*(h-top-bottom) }
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" font-family="ui-monospace, SFMono-Regular, Menlo, monospace" font-size="12">`+"\n", w, h, w, h)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#0d1117"/>`+"\n", w, h)
	fmt.Fprintf(&b, `<text x="%d" y="24" fill="#e6edf3" font-size="16">%s</text>`+"\n", left, title)
	// Grid and y ticks.
	for i := 0; i <= 5; i++ {
		v := maxY * float64(i) / 5
		y := py(v)
		fmt.Fprintf(&b, `<line x1="%d" y1="%.1f" x2="%d" y2="%.1f" stroke="#30363d"/>`+"\n", left, y, w-right, y)
		fmt.Fprintf(&b, `<text x="%d" y="%.1f" fill="#8b949e" text-anchor="end">%s</text>`+"\n", left-8, y+4, fmtNum(v))
	}
	fmt.Fprintf(&b, `<text x="%d" y="%d" fill="#8b949e" transform="rotate(-90 14 %d)">%s</text>`+"\n", 14, (h-bottom+top)/2, (h-bottom+top)/2, unit)
	// x ticks at every measured size.
	for _, x := range xs {
		fmt.Fprintf(&b, `<line x1="%.1f" y1="%d" x2="%.1f" y2="%d" stroke="#30363d"/>`+"\n", px(x), top, px(x), h-bottom)
		fmt.Fprintf(&b, `<text x="%.1f" y="%d" fill="#8b949e" text-anchor="middle">%d</text>`+"\n", px(x), h-bottom+18, x)
	}
	fmt.Fprintf(&b, `<text x="%d" y="%d" fill="#8b949e" text-anchor="middle">tables (150 columns each, one file)</text>`+"\n", (w+left-right)/2, h-8)
	// Lines and points.
	for i, name := range names {
		vals := series[name]
		var pts []string
		for j, v := range vals {
			pts = append(pts, fmt.Sprintf("%.1f,%.1f", px(xs[j]), py(v)))
		}
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="2"/>`+"\n", strings.Join(pts, " "), colors[name])
		for j, v := range vals {
			fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="3" fill="%s"/>`+"\n", px(xs[j]), py(v), colors[name])
		}
		lx := w - right - 110
		ly := top + 16*i
		fmt.Fprintf(&b, `<rect x="%d" y="%d" width="10" height="10" fill="%s"/><text x="%d" y="%d" fill="#e6edf3">%s</text>`+"\n", lx, ly-9, colors[name], lx+16, ly, name)
	}
	b.WriteString("</svg>\n")
	return b.String()
}

func fmtNum(v float64) string {
	switch {
	case v >= 100:
		return fmt.Sprintf("%.0f", v)
	case v >= 10:
		return fmt.Sprintf("%.1f", v)
	default:
		return fmt.Sprintf("%.2f", v)
	}
}
