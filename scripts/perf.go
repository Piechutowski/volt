//usr/bin/env go run "$0" "$@"; exit "$?"

// perf writes the performance report of the machine it runs on: the
// profiling sweep (PERF-2) at every size given, the language server's
// keystroke at each, the charts, and the machine's specification, as
// a Markdown file with its charts and profiles beside it. Run it from
// the repository root:
//
//	./scripts/perf.go -out perf-report
//
// The report is for comparing one machine against itself over time,
// or against the reference sweep in docs/reference; wall clock is
// never asserted anywhere.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Piechutowski/volt/internal/perfsweep"
)

func main() {
	out := flag.String("out", "perf-report", "the directory the report, charts and profiles are written to")
	sizes := flag.String("sizes", "10,20,40,80,160,320", "table counts to measure, ascending")
	columns := flag.Int("columns", 150, "columns per table")
	reps := flag.Int("reps", 3, "unprofiled timing runs per phase; the best is reported")
	profile := flag.Duration("profile", 2*time.Second, "CPU profile to collect per phase and size; 0 skips profiling")
	keystroke := flag.Bool("keystroke", true, "measure the language server's keystroke at each size (runs go test ./lsp)")
	count := flag.Int("count", 3, "repetitions of the keystroke benchmark per size")
	flag.Parse()

	repo, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "go.work")); err != nil {
		fail(fmt.Errorf("run from the repository root (no go.work in %s)", repo))
	}
	var spec perfsweep.Spec
	for _, s := range strings.Split(*sizes, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil || n <= 0 {
			fail(fmt.Errorf("-sizes: %q is not a table count", s))
		}
		spec.Sizes = append(spec.Sizes, n)
	}
	spec.Columns, spec.Reps, spec.Profile = *columns, *reps, *profile
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail(err)
	}
	log := func(format string, args ...any) { fmt.Fprintf(os.Stderr, format+"\n", args...) }
	command := "./scripts/perf.go " + strings.Join(os.Args[1:], " ")

	started := time.Now()
	machine := perfsweep.Info(repo)
	log("%s, %d cores, %s, %s, commit %s", machine.CPU, machine.Cores, machine.OS, machine.Go, machine.Commit)
	rows, err := perfsweep.Run(*out, spec, log)
	if err != nil {
		fail(err)
	}
	charts, err := perfsweep.Charts(*out, rows, spec.Columns)
	if err != nil {
		fail(err)
	}
	var ks []perfsweep.Keystroke
	var keystrokeChart string
	if *keystroke {
		if ks, err = perfsweep.Keystrokes(repo, spec.Sizes, spec.Columns, *count, log); err != nil {
			fail(err)
		}
		if keystrokeChart, err = perfsweep.KeystrokeChart(*out, ks, spec.Columns); err != nil {
			fail(err)
		}
	}
	report := filepath.Join(*out, "report.md")
	if err := os.WriteFile(report, []byte(perfsweep.Report(machine, spec, rows, charts, ks, keystrokeChart, strings.TrimSpace(command))), 0o644); err != nil {
		fail(err)
	}
	log("report written to %s in %s", report, perfsweep.Duration(time.Since(started)))
	fmt.Println(report)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "perf:", err)
	os.Exit(1)
}
