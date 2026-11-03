package db_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/nao/rt"
	"github.com/Piechutowski/volt/nao/run"
)

func params[M any](t *testing.T, body string) rt.Params[M] {
	t.Helper()
	var p rt.Params[M]
	if err := json.Unmarshal([]byte(body), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func status(t *testing.T, err error) int {
	t.Helper()
	p, ok := volt.ProblemOf(err)
	if !ok {
		t.Fatalf("no problem for %v", err)
	}
	return p.Status
}

// TestCastAllowsOnlyTheListedColumns proves Cast sets what the allow list
// names and the body carries, ignores what it does not name, and records
// cast and unknown_field errors (spec §changesets).
func TestCastAllowsOnlyTheListedColumns(t *testing.T) {
	ctx := context.Background()
	d := dbOpen(t)

	p := params[db.Budget](t, `{"id": 77, "phase": "vet", "tables": 16, "max_ms": 250.5, "state": "breached"}`)
	cs := db.BudgetNew().Cast(p, db.BudgetPhase, db.BudgetTables, db.BudgetMaxMs)
	if !cs.Changed(db.BudgetPhase) || !cs.Changed(db.BudgetMaxMs) || cs.Changed(db.BudgetID) || cs.Changed(db.BudgetState) {
		t.Fatalf("marks after Cast: phase %v, max_ms %v, id %v, state %v", cs.Changed(db.BudgetPhase), cs.Changed(db.BudgetMaxMs), cs.Changed(db.BudgetID), cs.Changed(db.BudgetState))
	}
	if row := cs.Row(); row.State != db.HealthOK || row.ID != 0 || row.Tables != 16 {
		t.Fatalf("Row = %+v, want the literal default state and no id", row)
	}
	b, err := run.Run(ctx, d, db.BudgetCreate(cs))
	if err != nil || b.ID == 77 || b.State != db.HealthOK || b.MaxMs != 250.5 {
		t.Fatalf("BudgetCreate(Cast) = %+v, %v", b, err)
	}

	bad := db.BudgetNew().Cast(params[db.Budget](t, `{"phase": "vet", "tables": "many", "maxms": 1, "max_ms": null}`), db.BudgetPhase, db.BudgetTables, db.BudgetMaxMs)
	if got := checks(refused(t, bad.Err())); got != "unknown_field(maxms) cast(tables) cast(max_ms)" {
		t.Errorf("errors of a bad body = %s", got)
	}
	if bad.Changed(db.BudgetTables) {
		t.Error("a value that does not convert left the column marked")
	}

	edit := b.Changeset().Cast(params[db.Budget](t, `{"max_ms": 300}`), db.BudgetMaxMs, db.BudgetState)
	if edit.Changed(db.BudgetState) || !edit.FromRow() {
		t.Error("a column the body does not carry was marked")
	}
	if u := do(t, d, db.BudgetUpdate(edit)); u.MaxMs != 300 || u.Phase != db.PhaseVet {
		t.Errorf("BudgetUpdate(partial Cast) = %+v", u)
	}

	wire, err := json.Marshal(db.TagNew().SlugSet("rc").NameSet("Candidate").Params())
	if err != nil || string(wire) != `{"name":"Candidate","slug":"rc"}` {
		t.Errorf("Params on the wire = %s, %v", wire, err)
	}
}

// TestFromAndValidate proves From marks every data column but the
// database-assigned key, and Validate runs required, then every
// validator in order, collecting every failure (spec §changesets,
// §validators).
func TestFromAndValidate(t *testing.T) {
	d := dbOpen(t)
	w := worldSeed(t, d)

	copied := db.MachineNew().From(w.machine).CoresSet(16)
	if copied.Changed(db.MachineID) || !copied.Changed(db.MachineCPU) || !copied.Changed(db.MachineCreatedAt) {
		t.Fatal("From marks every data column but the database-assigned key")
	}
	m := do(t, d, db.MachineCreate(copied))
	if m.ID == w.machine.ID || m.Cores != 16 || m.CPU != w.machine.CPU {
		t.Errorf("a machine made From another = %+v", m)
	}

	keyed := db.MsLoadNew().From(db.MsLoad{RunID: w.runs[0].ID, Tables: 64, WallMs: 1, SourceKb: 1})
	if !keyed.Changed(db.MsLoadRunID) || !keyed.Changed(db.MsLoadTables) {
		t.Error("From marks an ordinary key")
	}

	v := db.BudgetNew().PhaseSet("compile").TablesSet(0).MaxMsSet(0).StateSet("melted").Validate()
	if got := checks(refused(t, v.Err())); got != "PhaseKnown(phase) HealthKnown(state) BudgetSane(tables,max_ms)" {
		t.Errorf("Validate = %s", got)
	}
	if v.Valid() || len(v.Validate().Errors()) != 3 {
		t.Error("Validate is idempotent and its findings are replaced, not added")
	}

	u := db.UserNew().EmailSet("").NameSet("").ErrorAdd(db.UserName, "taken by a robot").Validate()
	if got := checks(refused(t, u.Err())); got != "custom(name) required(email) required(name) EmailShape(email)" {
		t.Errorf("Validate of an empty user = %s", got)
	}

	neg := db.MsVetNew().RunIDSet(w.runs[0].ID).TablesSet(64).WallMsSet(-1).Validate()
	if got := checks(refused(t, neg.Err())); got != "MillisMeasured(wall_ms)" {
		t.Errorf("the partial's validator in every table that injects it: %s", got)
	}
}

// TestRefusedWritesMapToStatuses proves a refused write is one
// *rt.Invalid whose fields the error spine answers 409 for a unique
// violation, on the index's columns or on none for an expression index,
// and 422 for a validator, a NOT NULL and a foreign key (spec
// §changesets, §validators, §formats).
func TestRefusedWritesMapToStatuses(t *testing.T) {
	ctx := context.Background()
	d := dbOpen(t)
	w := worldSeed(t, d)

	cases := []struct {
		name   string
		write  func() error
		checks string
		status int
	}{
		{"validator", func() error {
			_, err := run.Run(ctx, d, db.BudgetCreate(db.BudgetNew().PhaseSet("compile").TablesSet(1).MaxMsSet(1)))
			return err
		}, "PhaseKnown(phase)", http.StatusUnprocessableEntity},
		{"unique column", func() error {
			_, err := run.Run(ctx, d, db.UserCreate(db.UserNew().EmailSet("ada@voltmeter.test").NameSet("Ada again")))
			return err
		}, "unique(email)", http.StatusConflict},
		{"composite unique index", func() error {
			_, err := run.Run(ctx, d, db.BudgetCreate(db.BudgetNew().PhaseSet(db.PhaseLoad).TablesSet(4).MaxMsSet(9)))
			return err
		}, "unique(phase,tables)", http.StatusConflict},
		{"expression unique index", func() error {
			_, err := run.Run(ctx, d, db.TagCreate(db.TagNew().SlugSet("base").NameSet("BASELINE")))
			return err
		}, "unique()", http.StatusConflict},
		{"not null", func() error {
			_, err := run.Run(ctx, d, db.MsLoadCreate(db.MsLoadNew().RunIDSet(w.runs[0].ID).TablesSet(64).AllocMbSet(1).HeapMbSet(1).AllocsSet(1).SourceKbSet(1)))
			return err
		}, "not_null(wall_ms)", http.StatusUnprocessableEntity},
		{"foreign key", func() error {
			_, err := run.Run(ctx, d, db.RunCreate(db.RunNew().CommitSet("dddd").MachineIDSet(999)))
			return err
		}, "foreign_key()", http.StatusUnprocessableEntity},
		{"restricted delete", func() error {
			_, err := run.Run(ctx, d, db.UserDelete(1))
			return err
		}, "foreign_key()", http.StatusUnprocessableEntity},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.write()
			inv := refused(t, err)
			if got := checks(inv); got != tc.checks {
				t.Errorf("fields = %s, want %s", got, tc.checks)
			}
			if got := status(t, err); got != tc.status {
				t.Errorf("status = %d, want %d", got, tc.status)
			}
			if tc.status == http.StatusConflict && inv.Cause == nil {
				t.Error("a database refusal carries the driver's error as Cause")
			}
		})
	}
}

// TestUpdateAndDeleteByKey proves Update writes the marks by the key the
// row was read with, refuses a changeset not made from a row, and reports
// a vanished row, as a keyed Delete does (spec §changesets,
// §generated-query-surface).
func TestUpdateAndDeleteByKey(t *testing.T) {
	ctx := context.Background()
	d := dbOpen(t)
	w := worldSeed(t, d)
	r := w.runs[2]

	noted := do(t, d, db.RunUpdate(r.Changeset().NoteSet(rt.Some("rerun on a quiet machine")).CommitSet("CCCC3333")))
	if noted.Note.V != "rerun on a quiet machine" || noted.Commit != "cccc3333" || noted.CreatedAt != r.CreatedAt {
		t.Errorf("RunUpdate = %+v", noted)
	}
	if same := do(t, d, db.RunUpdate(noted.Changeset())); same != noted {
		t.Errorf("an update with nothing marked yields the row: %+v", same)
	}
	if _, err := run.Run(ctx, d, db.RunUpdate(db.RunNew().NoteSet(rt.Some("x")))); !errors.Is(err, rt.ErrNoRow) {
		t.Errorf("RunUpdate(RunNew()) = %v, want rt.ErrNoRow", err)
	}
	do(t, d, db.RunDelete(r.ID))
	if _, err := run.Run(ctx, d, db.RunUpdate(r.Changeset().NoteSet(rt.Some("gone")))); !errors.Is(err, rt.ErrNotFound) {
		t.Errorf("RunUpdate of a deleted row = %v, want rt.ErrNotFound", err)
	}
	if _, err := run.Run(ctx, d, db.RunDelete(r.ID)); !errors.Is(err, rt.ErrNotFound) {
		t.Errorf("RunDelete twice = %v, want rt.ErrNotFound", err)
	}
	if _, err := run.Run(ctx, d, db.RunGet(r.ID)); !errors.Is(err, rt.ErrNotFound) {
		t.Errorf("RunGet of a deleted row = %v, want rt.ErrNotFound", err)
	}

	key := do(t, d, db.LineCountGet("cccc3333", "examples/voltmeter"))
	moved := do(t, d, db.LineCountUpdate(key.Changeset().GenLinesSet(4800)))
	if moved.GenLines != 4800 || moved.HandLines != 500 {
		t.Errorf("LineCountUpdate by the composite key = %+v", moved)
	}
	many := do(t, d, db.CommitGetMany([]string{"cccc3333", "nope", "aaaa1111"}))
	if len(many) != 2 {
		t.Errorf("CommitGetMany skips a missing key: %+v", many)
	}
	if got := do(t, d, db.UserGetByEmail("grace@voltmeter.test")); got.ID != 2 || got.Name != "Grace" {
		t.Errorf("UserGetByEmail of a user the migration seeded = %+v", got)
	}
}

// TestChangesetsLeaveTheirReceiver proves a changeset is a value (spec
// §values-and-effects; §changesets): a setter, Cast, Validate
// and ErrorAdd return a new changeset and the one they were called on
// marks and holds what it did before.
func TestChangesetsLeaveTheirReceiver(t *testing.T) {
	base := db.BudgetNew().PhaseSet(db.PhaseLoad)
	set := base.MaxMsSet(3).TablesSet(0)
	if base.Changed(db.BudgetMaxMs) || base.Row().MaxMs != 0 || !set.Changed(db.BudgetMaxMs) {
		t.Errorf("a setter changed its receiver: %+v", base.Row())
	}
	invalid := set.Validate()
	if !set.Valid() || invalid.Valid() {
		t.Errorf("Validate changed its receiver: %v, %v", set.Errors(), invalid.Errors())
	}
	flagged := set.ErrorAdd(db.BudgetTables, "too few")
	if len(set.Errors()) != 0 || len(flagged.Errors()) != 1 {
		t.Errorf("ErrorAdd changed its receiver: %v", set.Errors())
	}
	cast := base.Cast(db.BudgetNew().TablesSet(4).Params(), db.BudgetTables)
	if base.Changed(db.BudgetTables) || !cast.Changed(db.BudgetTables) || cast.Row().Tables != 4 {
		t.Errorf("Cast changed its receiver: %+v, %+v", base.Row(), cast.Row())
	}
}
