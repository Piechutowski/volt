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
// failed transaction deletes the run again, and the machine and tag
// this upload added when no other run took them, before the error
// handler answers. Two first uploads of one machine or tag race to add
// it; the one the unique index refuses reads the winner's row.
func (Uploads) Create(r volt.Request, body Upload) volt.Response {
	return negotiated(r, func() volt.Response {
		commit, err := units.CommitParse(body.Commit)
		if err != nil {
			return volt.Reject(http.StatusUnprocessableEntity, "the upload is not valid").WithDetails(fieldErrors("commit", "commit", err.Error()))
		}
		if _, errs := UploadWrites(db.Run{}, body, nil); len(errs) > 0 {
			return volt.Reject(http.StatusUnprocessableEntity, "the upload is not valid").WithDetails(errs)
		}
		return uploadMachined(body, commit)
	})
}

type uploaded struct {
	machine    db.Machine
	machineNew bool
	tagNew     bool
}

func uploadMachined(body Upload, commit units.Commit) volt.Response {
	m := body.Machine
	identified := db.MachineIdentified(m.CPU, m.Cores, m.MemoryGB, m.OS, m.Arch, m.Go)
	return volt.Then(identified, func(found []db.Machine) volt.Response {
		if len(found) > 0 {
			return uploadTagged(body, commit, uploaded{machine: found[0]})
		}
		add := db.MachineNew().CPUSet(m.CPU).CoresSet(m.Cores).MemoryGbSet(m.MemoryGB).OsSet(m.OS).ArchSet(m.Arch).GoSet(m.Go)
		return volt.Then(db.MachineCreate(add), func(machine db.Machine) volt.Response {
			return uploadTagged(body, commit, uploaded{machine: machine, machineNew: true})
		}).Rescue(func(err error) volt.Response {
			if !isDuplicate(err) {
				return volt.Fail(err)
			}
			return volt.Then(identified, func(found []db.Machine) volt.Response {
				if len(found) == 0 {
					return volt.Fail(err)
				}
				return uploadTagged(body, commit, uploaded{machine: found[0]})
			})
		})
	})
}

func uploadTagged(body Upload, commit units.Commit, u uploaded) volt.Response {
	if body.Tag == "" {
		return uploadStored(body, commit, u)
	}
	return volt.Then(db.TagExists(db.TagSlug.Eq(body.Tag)), func(ok bool) volt.Response {
		if ok {
			return uploadStored(body, commit, u)
		}
		return volt.Then(db.TagCreate(db.TagNew().SlugSet(body.Tag).NameSet(body.Tag)), func(db.Tag) volt.Response {
			u.tagNew = true
			return uploadStored(body, commit, u)
		}).Rescue(func(err error) volt.Response {
			if !isDuplicate(err) {
				return volt.Fail(err)
			}
			return uploadStored(body, commit, u)
		})
	})
}

func uploadStored(body Upload, commit units.Commit, u uploaded) volt.Response {
	add := db.RunNew().CommitSet(commit).DirtySet(body.Dirty).MachineIDSet(u.machine.ID)
	if body.Tag != "" {
		add = add.TagSet(rt.Some(body.Tag))
	}
	if body.Note != "" {
		add = add.NoteSet(rt.Some(body.Note))
	}
	if body.Baseline > 0 {
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
					return uploadUndone(body, u, err)
				})
			})
		}).Rescue(func(err error) volt.Response {
			return uploadUndone(body, u, err)
		})
	})
}

func uploadUndone(body Upload, u uploaded, err error) volt.Response {
	if u.machineNew {
		machine := u.machine.ID
		u.machineNew = false
		return volt.Then(db.RunExists(db.RunMachineID.Eq(machine)), func(taken bool) volt.Response {
			if taken {
				return uploadUndone(body, u, err)
			}
			return volt.Then(db.MachineDelete(machine), func(int64) volt.Response {
				return uploadUndone(body, u, err)
			})
		})
	}
	if u.tagNew {
		u.tagNew = false
		return volt.Then(db.RunExists(db.RunTag.Eq(body.Tag)), func(taken bool) volt.Response {
			if taken {
				return uploadUndone(body, u, err)
			}
			return volt.Then(db.TagDelete(body.Tag), func(int64) volt.Response {
				return uploadUndone(body, u, err)
			})
		})
	}
	return volt.Fail(err)
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
// stored yet, in one transaction, and counts what it skipped.
func (Thesis) Record(r volt.Request, body ThesisUpload) volt.Response {
	return negotiated(r, func() volt.Response { return thesisRecorded(body) })
}

func thesisRecorded(body ThesisUpload) volt.Response {
	if len(body.Commits) == 0 {
		return volt.Reject(http.StatusUnprocessableEntity, "the upload holds no commit")
	}
	shas := make([]string, 0, len(body.Commits))
	for _, c := range body.Commits {
		shas = append(shas, c.SHA)
	}
	return volt.Then(rt.All(db.CommitGetMany(shas), db.LineCountQuery(db.LineCountSha.InList(shas))), func(res []any) volt.Response {
		steps, skipped := ThesisWrites(body, res[0].([]db.Commit), res[1].([]db.LineCount))
		if len(steps) == 0 {
			return volt.RenderStatus(http.StatusOK, wire.Stored{Skipped: skipped})
		}
		return volt.Then(rt.Tx(steps...), func([]any) volt.Response {
			return volt.Created(wire.Stored{Written: len(steps), Skipped: skipped})
		})
	})
}

// ThesisWrites is the creates an upload needs beside the commits and
// counts already stored, and how many rows it found present.
func ThesisWrites(body ThesisUpload, commits []db.Commit, counts []db.LineCount) ([]rt.AnyEffect, int) {
	known := map[string]bool{}
	for _, c := range commits {
		known[c.Sha] = true
	}
	counted := map[[2]string]bool{}
	for _, lc := range counts {
		counted[[2]string{lc.Sha, lc.Fixture}] = true
	}
	var steps []rt.AnyEffect
	skipped := 0
	for _, c := range body.Commits {
		if known[c.SHA] {
			skipped++
		} else {
			known[c.SHA] = true
			steps = append(steps, db.CommitCreate(db.CommitNew().ShaSet(c.SHA).SubjectSet(c.Subject).AuthorSet(c.Author).AtSet(c.At)))
		}
		for _, n := range c.Counts {
			key := [2]string{c.SHA, n.Fixture}
			if counted[key] {
				skipped++
				continue
			}
			counted[key] = true
			steps = append(steps, db.LineCountCreate(db.LineCountNew().ShaSet(c.SHA).FixtureSet(n.Fixture).VoltLinesSet(n.VoltLines).HandLinesSet(n.HandLines).GenLinesSet(n.GenLines)))
		}
	}
	return steps, skipped
}
