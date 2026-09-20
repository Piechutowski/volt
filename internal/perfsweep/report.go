package perfsweep

import (
	"fmt"
	"strings"
	"time"
)

// Report renders the Markdown report: the machine, the method, the
// numbers per phase and size, the charts, the keystroke, and the top
// functions of every phase at the largest size. charts and
// keystrokeChart are file names beside the report; ks may be nil.
func Report(m Machine, spec Spec, rows []Row, charts []string, ks []Keystroke, keystrokeChart, command string) string {
	var b strings.Builder
	xs := Sizes(rows)
	largest := xs[len(xs)-1]
	fmt.Fprintf(&b, "# Volt performance report, %s\n\n", m.Date)
	b.WriteString("One-file projects of growing table counts, every phase timed and profiled on its own, and the language server's keystroke at each size; the method is `internal/perfsweep`, run by `scripts/perf.go`. Wall clock is reported, never asserted: compare this report against another of the same machine, or against the reference sweep in `docs/reference`.\n\n")

	b.WriteString("## Machine\n\n| | |\n|---|---|\n")
	fmt.Fprintf(&b, "| CPU | %s, %d logical cores |\n", m.CPU, m.Cores)
	if m.MemoryGB > 0 {
		fmt.Fprintf(&b, "| Memory | %.1f GB |\n", m.MemoryGB)
	} else {
		b.WriteString("| Memory | unknown |\n")
	}
	fmt.Fprintf(&b, "| System | %s, %s |\n", m.OS, m.Arch)
	fmt.Fprintf(&b, "| Go | %s |\n", m.Go)
	fmt.Fprintf(&b, "| Volt | commit %s |\n", m.Commit)
	fmt.Fprintf(&b, "| Run | `%s` |\n\n", command)

	b.WriteString("## Method\n\n")
	fmt.Fprintf(&b, "`internal/corpus` writes one-file projects of %s tables with %d columns each, the stress shape of D80 at every size, using every feature of the language. Each phase, Load (scan and parse), Check, Vet and Generate (every file of every package, DDL included), runs on its own: the best wall time of %d unprofiled runs, one run's allocated bytes, allocation count and heap in use right after it", joinInts(xs), spec.Columns, max(spec.Reps, 1))
	if spec.Profile > 0 {
		fmt.Fprintf(&b, ", then enough profiled runs to fill about %s of CPU profile (the `.prof` files beside this report, readable with `go tool pprof`)", spec.Profile)
	}
	b.WriteString(". The keystroke is `lsp.BenchmarkKeystroke`: one edit inside one table of the same project, from the didChange notification to the publish of its diagnostics, in process with the debounce zeroed, the best of the benchmark's repetitions.\n\n")

	b.WriteString("## Wall time\n\nBest of the repetitions, in milliseconds:\n\n")
	b.WriteString("| Tables | Source | Load | Check | Vet | Generate | Output |\n|---|---|---|---|---|---|---|\n")
	for _, x := range xs {
		l, c, v, g := At(rows, x, "load"), At(rows, x, "check"), At(rows, x, "vet"), At(rows, x, "generate")
		fmt.Fprintf(&b, "| %d | %s | %.1f | %.1f | %.1f | %.1f | %s |\n", x, sizeKB(l.SourceKB), l.WallMS, c.WallMS, v.WallMS, g.WallMS, sizeMB(g.OutputMB))
	}
	if len(xs) > 1 {
		first := xs[0]
		fmt.Fprintf(&b, "| growth %d/%d | %.0f× | %s | %s | %s | %s | %.0f× |\n", largest, first, float64(largest)/float64(first),
			ratio(At(rows, largest, "load").WallMS, At(rows, first, "load").WallMS),
			ratio(At(rows, largest, "check").WallMS, At(rows, first, "check").WallMS),
			ratio(At(rows, largest, "vet").WallMS, At(rows, first, "vet").WallMS),
			ratio(At(rows, largest, "generate").WallMS, At(rows, first, "generate").WallMS),
			At(rows, largest, "generate").OutputMB/At(rows, first, "generate").OutputMB)
	}
	b.WriteString("\n## Allocation\n\nBytes allocated by one run, in MB, and allocations in thousands:\n\n")
	b.WriteString("| Tables | Load | Check | Vet | Generate |\n|---|---|---|---|---|\n")
	for _, x := range xs {
		fmt.Fprintf(&b, "| %d |", x)
		for _, ph := range Phases {
			r := At(rows, x, ph)
			fmt.Fprintf(&b, " %.1f / %.0f |", r.AllocMB, float64(r.Allocs)/1000)
		}
		b.WriteString("\n")
	}
	b.WriteString("\nHeap in use right after the run, before collection, in MB:\n\n")
	b.WriteString("| Tables | Load | Check | Vet | Generate |\n|---|---|---|---|---|\n")
	for _, x := range xs {
		fmt.Fprintf(&b, "| %d |", x)
		for _, ph := range Phases {
			fmt.Fprintf(&b, " %.1f |", At(rows, x, ph).HeapMB)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "\nPer table at %d, the steady-state cost:", largest)
	var costs []string
	for _, ph := range Phases {
		r := At(rows, largest, ph)
		costs = append(costs, fmt.Sprintf(" %s %.2f ms, %.0f KB and %.0f allocations", strings.ToUpper(ph[:1])+ph[1:], r.WallMS/float64(largest), r.AllocMB*1024/float64(largest), float64(r.Allocs)/float64(largest)))
	}
	b.WriteString(strings.Join(costs, ";") + ".\n\n## Charts\n\n")
	for _, name := range charts {
		fmt.Fprintf(&b, "![%s](%s)\n\n", strings.TrimSuffix(name, ".svg"), name)
	}

	if len(ks) > 0 {
		b.WriteString("## The keystroke\n\nOne edit inside one table, didChange to the publish of its diagnostics, in process; the best repetition:\n\n")
		b.WriteString("| Tables | ms | MB allocated | Allocations |\n|---|---|---|---|\n")
		for _, k := range ks {
			fmt.Fprintf(&b, "| %d | %.1f | %.2f | %d |\n", k.Tables, k.MSPerOp, k.MBPerOp, k.AllocsPerOp)
		}
		if keystrokeChart != "" {
			fmt.Fprintf(&b, "\n![keystroke](%s)\n", keystrokeChart)
		}
		b.WriteString("\n")
	}

	if spec.Profile > 0 {
		fmt.Fprintf(&b, "## Per function at %d tables\n\nThe top of each phase's CPU profile by flat time, as `go tool pprof -top` prints it; the `-cum` order is in `sweep.json`.\n\n", largest)
		for _, ph := range Phases {
			r := At(rows, largest, ph)
			if r.TopByFlat == "" {
				continue
			}
			fmt.Fprintf(&b, "### %s (%d runs, `%s`)\n\n```\n%s```\n\n", ph, r.Iterations, r.Profile, r.TopByFlat)
		}
	}
	return b.String()
}

func joinInts(xs []int) string {
	var parts []string
	for _, x := range xs {
		parts = append(parts, fmt.Sprint(x))
	}
	if len(parts) > 1 {
		return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
	}
	return strings.Join(parts, "")
}

func ratio(a, b float64) string {
	if b == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f×", a/b)
}

func sizeKB(kb float64) string {
	if kb >= 1024 {
		return fmt.Sprintf("%.1f MB", kb/1024)
	}
	return fmt.Sprintf("%.0f KB", kb)
}

func sizeMB(mb float64) string {
	if mb < 1 {
		return fmt.Sprintf("%.0f KB", mb*1024)
	}
	return fmt.Sprintf("%.1f MB", mb)
}

// Duration formats the whole run's wall time for the log.
func Duration(d time.Duration) string { return d.Round(time.Second).String() }
