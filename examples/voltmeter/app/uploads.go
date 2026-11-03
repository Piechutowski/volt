package app

import (
	"errors"
	"net/http"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/units"
	"github.com/Piechutowski/volt/examples/voltmeter/wire"
	"github.com/Piechutowski/volt/nao/rt"
)

// Uploads stores the collector's runs.
type Uploads struct{}

// Create stores one upload: it finds or adds the machine and the tag,
// adds the run, then writes every measurement, every budget it breaks
// and the breaches in one transaction. The run's id is the database's,
// so the run is inserted before the transaction that needs it, and a
// failed transaction deletes the run again before the error handler
// answers. The machine and the tag stay: a concurrent upload may have
// found them and stored its run on them, and a delete would cascade
// through that run. Two first uploads of one machine or tag race to add
// it; the one the unique index refuses reads the winner's row.
func (Uploads) Create(r volt.Request, body Upload) volt.Response {
	return negotiated(r, func() volt.Response {
		if errs := UploadJudged(body); len(errs) > 0 {
			return volt.Reject(http.StatusUnprocessableEntity, "the upload is not valid").WithDetails(errs)
		}
		commit, _ := units.CommitParse(body.Commit)
		return uploadMachined(body, commit)
	})
}

// UploadJudged is every finding against an upload before anything is
// stored: its commit, its machine's identity, that it measured at least
// one phase, that a baseline it names is a run id, and each row's own
// validators (spec §validators).
func UploadJudged(body Upload) []rt.FieldError {
	var errs []rt.FieldError
	if _, err := units.CommitParse(body.Commit); err != nil {
		errs = append(errs, fieldErrors("commit", "commit", err.Error())...)
	}
	errs = append(errs, machineNewOf(body.Machine).Validate().Errors()...)
	if len(body.Phases) == 0 {
		errs = append(errs, fieldErrors("required", "phases", "an upload measures at least one phase")...)
	}
	if body.Baseline < 0 {
		errs = append(errs, fieldErrors("foreign_key", "baseline_id", "a baseline names a run by its id, never below 1")...)
	}
	_, werrs := UploadWrites(db.Run{}, body, nil)
	return append(errs, werrs...)
}

func machineNewOf(m UploadMachine) db.MachineChangeset {
	return db.MachineNew().CPUSet(m.CPU).CoresSet(m.Cores).MemoryGbSet(m.MemoryGB).OsSet(m.OS).ArchSet(m.Arch).GoSet(m.Go)
}

func uploadMachined(body Upload, commit units.Commit) volt.Response {
	m := body.Machine
	identified := db.MachineIdentified(m.CPU, m.Cores, m.MemoryGB, m.OS, m.Arch, m.Go)
	return volt.Then(identified, func(found []db.Machine) volt.Response {
		if len(found) > 0 {
			return uploadTagged(body, commit, found[0])
		}
		return volt.Then(db.MachineCreate(machineNewOf(m)), func(machine db.Machine) volt.Response {
			return uploadTagged(body, commit, machine)
		}).Rescue(func(err error) volt.Response {
			if !isDuplicate(err) {
				return volt.Fail(err)
			}
			return volt.Then(identified, func(found []db.Machine) volt.Response {
				if len(found) == 0 {
					return volt.Fail(err)
				}
				return uploadTagged(body, commit, found[0])
			})
		})
	})
}

func uploadTagged(body Upload, commit units.Commit, machine db.Machine) volt.Response {
	if body.Tag == "" {
		return uploadStored(body, commit, machine)
	}
	return volt.Then(db.TagExists(db.TagSlug.Eq(body.Tag)), func(ok bool) volt.Response {
		if ok {
			return uploadStored(body, commit, machine)
		}
		return volt.Then(db.TagCreate(db.TagNew().SlugSet(body.Tag).NameSet(body.Tag)), func(db.Tag) volt.Response {
			return uploadStored(body, commit, machine)
		}).Rescue(func(err error) volt.Response {
			if !isDuplicate(err) {
				return volt.Fail(err)
			}
			return uploadStored(body, commit, machine)
		})
	})
}

func uploadStored(body Upload, commit units.Commit, machine db.Machine) volt.Response {
	add := db.RunNew().CommitSet(commit).DirtySet(body.Dirty).MachineIDSet(machine.ID)
	if body.Tag != "" {
		add = add.TagSet(rt.Some(body.Tag))
	}
	if body.Note != "" {
		add = add.NoteSet(rt.Some(body.Note))
	}
	if body.Baseline != 0 {
		add = add.BaselineIDSet(rt.Some(body.Baseline))
	}
	return volt.Then(db.BudgetList(), func(budgets []db.Budget) volt.Response {
		return volt.Then(db.RunCreate(add), func(run db.Run) volt.Response {
			steps, _ := UploadWrites(run, body, budgets)
			return volt.Then(rt.Tx(steps...), func(res []any) volt.Response {
				out := wire.UploadResult{Run: run, Breaches: []db.Breach{}}
				for _, v := range res {
					if b, ok := v.(db.Breach); ok {
						out.Breaches = append(out.Breaches, b)
					}
				}
				return volt.Created(out)
			}).Rescue(func(err error) volt.Response {
				return volt.Then(db.RunDelete(run.ID), func(int64) volt.Response {
					return volt.Fail(err)
				})
			})
		})
	})
}

func isDuplicate(err error) bool {
	var inv *rt.Invalid
	if !errors.As(err, &inv) {
		return false
	}
	for _, f := range inv.Fields {
		if f.Check == "unique" {
			return true
		}
	}
	return false
}

// UploadWrites is every write an upload makes for run once the run is
// stored: a create per phase and keystroke row, and for each phase row
// over a budget of its phase and size the budget's state and a breach.
// The errors are the changesets' own findings, so a caller judges an
// upload before anything is written.
func UploadWrites(run db.Run, body Upload, budgets []db.Budget) ([]rt.AnyEffect, []rt.FieldError) {
	var steps []rt.AnyEffect
	var errs []rt.FieldError
	breached := map[int]bool{}
	for _, p := range body.Phases {
		step, perr := phaseWrite(run.ID, p)
		if step == nil {
			errs = append(errs, fieldErrors("PhaseKnown", "phase", "phase "+string(p.Phase)+" is not one of the sweep's")...)
			continue
		}
		errs = append(errs, perr...)
		steps = append(steps, step)
		for _, b := range budgets {
			if b.Phase != p.Phase || b.Tables != p.Tables || p.WallMS <= b.MaxMs {
				continue
			}
			if !breached[b.ID] && b.State != db.HealthBreached {
				steps = append(steps, db.BudgetUpdate(b.Changeset().StateSet(db.HealthBreached)))
			}
			breached[b.ID] = true
			steps = append(steps, db.BreachCreate(db.BreachNew().BudgetIDSet(b.ID).RunIDSet(run.ID).WallMsSet(p.WallMS)))
		}
	}
	for _, k := range body.Keystrokes {
		cs := db.KeystrokeNew().RunIDSet(run.ID).TablesSet(k.Tables).MsPerOpSet(k.MSPerOp).MbPerOpSet(k.MBPerOp).AllocsPerOpSet(k.AllocsPerOp).Validate()
		errs = append(errs, cs.Errors()...)
		steps = append(steps, db.KeystrokeCreate(cs))
	}
	return steps, errs
}

func phaseWrite(run int, p UploadPhase) (rt.AnyEffect, []rt.FieldError) {
	ms := units.Millis(p.WallMS)
	switch p.Phase {
	case db.PhaseLoad:
		cs := db.MsLoadNew().RunIDSet(run).TablesSet(p.Tables).WallMsSet(ms).AllocMbSet(p.AllocMB).HeapMbSet(p.HeapMB).AllocsSet(p.Allocs).OutputMbSet(p.OutputMB).SourceKbSet(p.SourceKB).Validate()
		return db.MsLoadCreate(cs), cs.Errors()
	case db.PhaseCheck:
		cs := db.MsCheckNew().RunIDSet(run).TablesSet(p.Tables).WallMsSet(ms).AllocMbSet(p.AllocMB).HeapMbSet(p.HeapMB).AllocsSet(p.Allocs).OutputMbSet(p.OutputMB).SourceKbSet(p.SourceKB).Validate()
		return db.MsCheckCreate(cs), cs.Errors()
	case db.PhaseVet:
		cs := db.MsVetNew().RunIDSet(run).TablesSet(p.Tables).WallMsSet(ms).AllocMbSet(p.AllocMB).HeapMbSet(p.HeapMB).AllocsSet(p.Allocs).OutputMbSet(p.OutputMB).SourceKbSet(p.SourceKB).Validate()
		return db.MsVetCreate(cs), cs.Errors()
	case db.PhaseGenerate:
		cs := db.MsGenerateNew().RunIDSet(run).TablesSet(p.Tables).WallMsSet(ms).AllocMbSet(p.AllocMB).HeapMbSet(p.HeapMB).AllocsSet(p.Allocs).OutputMbSet(p.OutputMB).SourceKbSet(p.SourceKB).Validate()
		return db.MsGenerateCreate(cs), cs.Errors()
	}
	return nil, nil
}

// Thesis stores line counts of Volt's history.
type Thesis struct{}

// Record stores the commits and counts the upload carries that are not
// stored yet, in one transaction, and counts what it skipped. A row the
// upload would add is judged by its validators first, so nothing of a
// malformed upload is stored. Two collectors may upload overlapping
// history at once: when the unique key refuses a commit another upload
// stored meanwhile, the upload reads again and writes what is still
// missing.
func (Thesis) Record(r volt.Request, body ThesisUpload) volt.Response {
	return negotiated(r, func() volt.Response { return thesisRecorded(body, thesisTries) })
}

const thesisTries = 3

func thesisRecorded(body ThesisUpload, tries int) volt.Response {
	if len(body.Commits) == 0 {
		return volt.Reject(http.StatusUnprocessableEntity, "the upload holds no commit")
	}
	shas := make([]string, 0, len(body.Commits))
	for _, c := range body.Commits {
		shas = append(shas, c.SHA)
	}
	return volt.Then(rt.All(db.CommitGetMany(shas), db.LineCountQuery(db.LineCountSha.InList(shas))), func(res []any) volt.Response {
		steps, skipped, errs := ThesisWrites(body, res[0].([]db.Commit), res[1].([]db.LineCount))
		if len(errs) > 0 {
			return volt.Reject(http.StatusUnprocessableEntity, "the upload is not valid").WithDetails(errs)
		}
		if len(steps) == 0 {
			return volt.RenderStatus(http.StatusOK, wire.Stored{Skipped: skipped})
		}
		return volt.Then(rt.Tx(steps...), func([]any) volt.Response {
			return volt.Created(wire.Stored{Written: len(steps), Skipped: skipped})
		}).Rescue(func(err error) volt.Response {
			if tries <= 1 || !isDuplicate(err) {
				return volt.Fail(err)
			}
			return thesisRecorded(body, tries-1)
		})
	})
}

// ThesisWrites is the creates an upload needs beside the commits and
// counts already stored, how many rows it found present, and the
// validators' findings against the rows it would create.
func ThesisWrites(body ThesisUpload, commits []db.Commit, counts []db.LineCount) ([]rt.AnyEffect, int, []rt.FieldError) {
	known := map[string]bool{}
	for _, c := range commits {
		known[c.Sha] = true
	}
	counted := map[[2]string]bool{}
	for _, lc := range counts {
		counted[[2]string{lc.Sha, lc.Fixture}] = true
	}
	var steps []rt.AnyEffect
	var errs []rt.FieldError
	skipped := 0
	for _, c := range body.Commits {
		if known[c.SHA] {
			skipped++
		} else {
			known[c.SHA] = true
			cs := db.CommitNew().ShaSet(c.SHA).SubjectSet(c.Subject).AuthorSet(c.Author).AtSet(c.At).Validate()
			errs = append(errs, cs.Errors()...)
			steps = append(steps, db.CommitCreate(cs))
		}
		for _, n := range c.Counts {
			key := [2]string{c.SHA, n.Fixture}
			if counted[key] {
				skipped++
				continue
			}
			counted[key] = true
			cs := db.LineCountNew().ShaSet(c.SHA).FixtureSet(n.Fixture).VoltLinesSet(n.VoltLines).HandLinesSet(n.HandLines).GenLinesSet(n.GenLines).Validate()
			errs = append(errs, cs.Errors()...)
			steps = append(steps, db.LineCountCreate(cs))
		}
	}
	return steps, skipped, errs
}
