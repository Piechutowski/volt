package perfsweep

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"
)

// TestReportRenders proves the report and its charts render from
// measured rows: every size and phase lands in the tables, the
// keystroke section follows, and each chart is well-formed SVG with a
// line per phase.
func TestReportRenders(t *testing.T) {
	var rows []Row
	for i, tables := range []int{10, 20} {
		for j, ph := range Phases {
			rows = append(rows, Row{Tables: tables, Phase: ph, WallMS: float64(1+i) * float64(1+j), AllocMB: 1, Allocs: 1000, HeapMB: 2, SourceKB: 100, OutputMB: 0.5, Iterations: 3})
		}
	}
	ks := []Keystroke{{Tables: 10, MSPerOp: 1.5, MBPerOp: 0.5, AllocsPerOp: 100}, {Tables: 20, MSPerOp: 2.5, MBPerOp: 0.7, AllocsPerOp: 150}}
	m := Machine{CPU: "test cpu", Cores: 4, MemoryGB: 16, OS: "test os", Arch: "amd64", Go: "go1.27", Commit: "abc123", Date: "2026-09-20"}
	md := Report(m, Spec{Sizes: []int{10, 20}, Columns: 150, Reps: 3, Profile: 2 * time.Second}, rows, []string{"wall.svg"}, ks, "keystroke.svg", "./scripts/perf.go")
	for _, want := range []string{"| CPU | test cpu, 4 logical cores |", "| 20 | 100 KB | 2.0 | 4.0 | 6.0 | 8.0 | 512 KB |", "| growth 20/10 | 2× | 2.0× | 2.0× | 2.0× | 2.0× | 1× |", "| 20 | 2.5 | 0.70 | 150 |", "![wall](wall.svg)", "![keystroke](keystroke.svg)"} {
		if !strings.Contains(md, want) {
			t.Errorf("report lacks %q:\n%s", want, md)
		}
	}
	series := map[string][]float64{}
	for _, r := range rows {
		series[r.Phase] = append(series[r.Phase], r.WallMS)
	}
	svg := LineChart("Wall time per phase", "ms", "tables", []int{10, 20}, Phases, series, phaseColors)
	if err := xml.Unmarshal([]byte(svg), new(struct{})); err != nil {
		t.Fatalf("chart is not well-formed SVG: %v\n%s", err, svg)
	}
	if n := strings.Count(svg, "<polyline"); n != len(Phases) {
		t.Errorf("chart draws %d lines, want %d", n, len(Phases))
	}
	for _, ph := range Phases {
		if !strings.Contains(svg, ">"+ph+"</text>") {
			t.Errorf("chart lacks a label for %s", ph)
		}
	}
}
