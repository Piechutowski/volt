package app

import (
	"time"

	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/nao/rt"
)

// SignIn is the struct body of the sign-in route (spec §request-bodies):
// a member names themself by email.
type SignIn struct {
	Email string `json:"email"`
}

// Upload is the collector's report of one run, the struct body of the
// upload route (spec §request-bodies): the commit and the machine it
// measured on, and every phase and keystroke row it took.
type Upload struct {
	Commit     string            `json:"commit"`
	Dirty      bool              `json:"dirty"`
	Tag        string            `json:"tag"`
	Note       string            `json:"note"`
	Baseline   int               `json:"baseline"`
	Machine    UploadMachine     `json:"machine"`
	Phases     []UploadPhase     `json:"phases"`
	Keystrokes []UploadKeystroke `json:"keystrokes"`
}

// UploadMachine is where an upload was measured, as perfsweep.Info
// reports it; the server finds the machine by these columns or adds it.
type UploadMachine struct {
	CPU      string  `json:"cpu"`
	Cores    int     `json:"cores"`
	MemoryGB float64 `json:"memory_gb"`
	OS       string  `json:"os"`
	Arch     string  `json:"arch"`
	Go       string  `json:"go"`
}

// UploadPhase is one measured phase at one size, a perfsweep.Row.
type UploadPhase struct {
	Phase    db.Phase `json:"phase"`
	Tables   int      `json:"tables"`
	WallMS   float64  `json:"wall_ms"`
	AllocMB  float64  `json:"alloc_mb"`
	HeapMB   float64  `json:"heap_mb"`
	Allocs   int      `json:"allocs"`
	OutputMB float64  `json:"output_mb"`
	SourceKB float64  `json:"source_kb"`
}

// UploadKeystroke is the keystroke benchmark at one size, a
// perfsweep.Keystroke.
type UploadKeystroke struct {
	Tables      int     `json:"tables"`
	MSPerOp     float64 `json:"ms_per_op"`
	MBPerOp     float64 `json:"mb_per_op"`
	AllocsPerOp int     `json:"allocs_per_op"`
}

// SampleBatch is the struct body of the samples route: the collector's
// timed calls of the read routes for one run.
type SampleBatch struct {
	Samples []SampleTimed `json:"samples"`
}

// SampleTimed is one timed call.
type SampleTimed struct {
	Route  string  `json:"route"`
	Status int     `json:"status"`
	MS     float64 `json:"ms"`
	At     int     `json:"at"`
}

// ThesisUpload is the struct body of the thesis route: line counts of
// the fixtures at some commits of Volt's history.
type ThesisUpload struct {
	Commits []ThesisCommit `json:"commits"`
}

// ThesisCommit is one counted commit and its fixtures' counts.
type ThesisCommit struct {
	SHA     string        `json:"sha"`
	Subject string        `json:"subject"`
	Author  string        `json:"author"`
	At      time.Time     `json:"at"`
	Counts  []ThesisCount `json:"counts"`
}

// ThesisCount is one fixture's lines at one commit.
type ThesisCount struct {
	Fixture   string `json:"fixture"`
	VoltLines int    `json:"volt_lines"`
	HandLines int    `json:"hand_lines"`
	GenLines  int    `json:"gen_lines"`
}

func fieldErrors(check, column, msg string) []rt.FieldError {
	return []rt.FieldError{{Check: check, Columns: []string{column}, Message: msg}}
}
