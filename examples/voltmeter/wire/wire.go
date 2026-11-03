// Package wire holds the values Voltmeter's controllers render and its
// clients decode (spec §formats). They live outside the routes package
// app, so a client that reads them links no server code
// (spec §generated-client); the struct bodies stay in app, where the
// language declares them (spec §request-bodies).
package wire

import (
	"time"

	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/units"
)

// UploadResult answers an upload: the stored run and the breaches it
// caused.
type UploadResult struct {
	Run      db.Run      `json:"run"`
	Breaches []db.Breach `json:"breaches"`
}

// Stored counts the rows a batch route wrote and those it found present.
type Stored struct {
	Written int `json:"written"`
	Skipped int `json:"skipped"`
}

// RunPage is one page of the keyset-paged run list: Next is the id to
// pass as after for the following page, 0 on the last, and Breached the
// runs of the page that went over a budget, in the page's order.
type RunPage struct {
	Runs     []db.Run `json:"runs"`
	Total    int64    `json:"total"`
	Next     int      `json:"next"`
	Breached []int    `json:"breached"`
}

// RunDetail is everything one run measured: Latency summarizes every
// timed call, Failed counts those answered 400 or above, and Breaches are
// the budgets the run went over.
type RunDetail struct {
	Run        db.Run         `json:"run"`
	Phases     []PhaseRow     `json:"phases"`
	Keystrokes []db.Keystroke `json:"keystrokes"`
	Latency    db.Latency     `json:"latency"`
	Failed     int64          `json:"failed"`
	Breaches   []db.Breach    `json:"breaches"`
}

// PhaseRow is one row of a phase table with the phase it came from; the
// four tables share the measured partial, so their rows convert to
// db.MsLoad.
type PhaseRow struct {
	Phase    db.Phase     `json:"phase"`
	Tables   int          `json:"tables"`
	WallMS   units.Millis `json:"wall_ms"`
	AllocMB  float64      `json:"alloc_mb"`
	HeapMB   float64      `json:"heap_mb"`
	Allocs   int          `json:"allocs"`
	OutputMB float64      `json:"output_mb"`
	SourceKB float64      `json:"source_kb"`
}

// BudgetDetail is a budget and the breaches against it.
type BudgetDetail struct {
	Budget   db.Budget   `json:"budget"`
	Breaches []db.Breach `json:"breaches"`
}

// Reset counts the budgets a reset returned to ok.
type Reset struct {
	Reset int64 `json:"reset"`
}

// Counted answers the probe route: the database counted to Steps and
// matched Rows members on the way.
type Counted struct {
	Steps int   `json:"steps"`
	Rows  int64 `json:"rows"`
}

// Pong answers the ping route with what it was asked.
type Pong struct {
	Method string    `json:"method"`
	At     time.Time `json:"at"`
}

// PhaseRowsOf is the rows of the four phase tables as one list, phase by
// phase in the order the sweep runs them.
func PhaseRowsOf(load []db.MsLoad, check []db.MsCheck, vet []db.MsVet, generate []db.MsGenerate) []PhaseRow {
	out := make([]PhaseRow, 0, len(load)+len(check)+len(vet)+len(generate))
	for _, m := range load {
		out = append(out, phaseRowOf(db.PhaseLoad, m))
	}
	for _, m := range check {
		out = append(out, phaseRowOf(db.PhaseCheck, db.MsLoad(m)))
	}
	for _, m := range vet {
		out = append(out, phaseRowOf(db.PhaseVet, db.MsLoad(m)))
	}
	for _, m := range generate {
		out = append(out, phaseRowOf(db.PhaseGenerate, db.MsLoad(m)))
	}
	return out
}

func phaseRowOf(p db.Phase, m db.MsLoad) PhaseRow {
	return PhaseRow{Phase: p, Tables: m.Tables, WallMS: m.WallMs, AllocMB: m.AllocMb, HeapMB: m.HeapMb, Allocs: m.Allocs, OutputMB: m.OutputMb, SourceKB: m.SourceKb}
}

// Comparison is a run's load phase beside its baseline's, size by size.
type Comparison struct {
	Run     db.Run  `json:"run"`
	Against db.Run  `json:"against"`
	Sizes   []Delta `json:"sizes"`
}

// Delta is one size of a comparison: both wall times and the change in
// percent, 0 when the baseline measured nothing to compare with.
type Delta struct {
	Tables int          `json:"tables"`
	Mine   units.Millis `json:"mine"`
	Theirs units.Millis `json:"theirs"`
	Change float64      `json:"change"`
}

// ComparisonOf pairs the rows of run and against by size, in mine's
// order; a size against did not measure is left out.
func ComparisonOf(run, against db.Run, mine, theirs []db.MsLoad) Comparison {
	out := Comparison{Run: run, Against: against, Sizes: []Delta{}}
	for _, m := range mine {
		for _, o := range theirs {
			if o.Tables != m.Tables {
				continue
			}
			d := Delta{Tables: m.Tables, Mine: m.WallMs, Theirs: o.WallMs}
			if o.WallMs > 0 {
				d.Change = 100 * (float64(m.WallMs)/float64(o.WallMs) - 1)
			}
			out.Sizes = append(out.Sizes, d)
		}
	}
	return out
}
