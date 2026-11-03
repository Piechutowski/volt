package db_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/nao/rt"
	"github.com/Piechutowski/volt/nao/run"
)

func manySamples(t *testing.T, d run.DB, runID, n int) {
	t.Helper()
	steps := make([]rt.AnyEffect, n)
	for i := range n {
		steps[i] = db.SampleCreate(db.SampleNew().RunIDSet(runID).RouteSet(fmt.Sprintf("/r%d", i%7)).StatusSet(200 + 100*(i%4)).MsSet(float64(i % 13)).AtSet(i))
	}
	do(t, d, rt.Tx(steps...))
}

func page(t *testing.T, d run.DB, n int, desc bool, preds ...rt.Pred[db.Sample]) []int {
	t.Helper()
	terms := []rt.Order[db.Sample]{db.SampleMs.Asc(), db.SampleID.Asc()}
	if desc {
		terms = []rt.Order[db.Sample]{db.SampleMs.Desc(), db.SampleID.Desc()}
	}
	var seen []int
	var after []any
	for range 1000 {
		opts := []rt.Opt[db.Sample]{db.SampleOrderBy(terms...), db.SampleLimit(n)}
		for _, p := range preds {
			opts = append(opts, p)
		}
		if after != nil {
			opts = append(opts, db.SampleAfter(after...))
		}
		rows := do(t, d, db.SampleQuery(opts...))
		if len(rows) == 0 {
			return seen
		}
		for _, r := range rows {
			seen = append(seen, r.ID)
		}
		last := rows[len(rows)-1]
		after = []any{last.Ms, last.ID}
	}
	t.Fatal("paging did not end")
	return nil
}

// TestKeysetPagingNeverRepeatsOrSkips pages 1000 samples whose ordering
// column repeats, by a page size that divides nothing, in both directions
// and under a filter, and proves the pages concatenate to the one-shot
// ordered query (spec §predicates, the keyset position, and
// §generated-query-surface).
func TestKeysetPagingNeverRepeatsOrSkips(t *testing.T) {
	d := open(t)
	w := seed(t, d)
	manySamples(t, d, w.runs[1].ID, 1000)

	for _, tc := range []struct {
		name  string
		desc  bool
		preds []rt.Pred[db.Sample]
	}{
		{"ascending", false, nil},
		{"descending", true, nil},
		{"filtered", false, []rt.Pred[db.Sample]{db.SampleRunID.Eq(w.runs[1].ID), db.SampleStatus.Ne(300)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			terms := []rt.Order[db.Sample]{db.SampleMs.Asc(), db.SampleID.Asc()}
			if tc.desc {
				terms = []rt.Order[db.Sample]{db.SampleMs.Desc(), db.SampleID.Desc()}
			}
			opts := []rt.Opt[db.Sample]{db.SampleOrderBy(terms...)}
			for _, p := range tc.preds {
				opts = append(opts, p)
			}
			want := ids(do(t, d, db.SampleQuery(opts...)), func(s db.Sample) int { return s.ID })
			got := page(t, d, 37, tc.desc, tc.preds...)
			if !slices.Equal(got, want) {
				t.Fatalf("paged %d ids, want %d in the one-shot order", len(got), len(want))
			}
			uniq := slices.Clone(got)
			slices.Sort(uniq)
			if len(slices.Compact(uniq)) != len(got) {
				t.Fatal("a page repeated a row")
			}
		})
	}

	if _, err := run.Run(context.Background(), d, db.SampleQuery(db.SampleAfter(1.0, 2))); err == nil {
		t.Error("After without OrderBy ran; want the rendering error")
	}
}

// TestDynamicVerbs proves Count, Exists, Offset, Distinct, UpdateWhere
// and DeleteWhere over predicates the dynamic layer composes, and that a
// bulk write without a predicate is refused (spec
// §generated-query-surface, §changesets bulk paths).
func TestDynamicVerbs(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	w := seed(t, d)
	r := w.runs[2].ID
	manySamples(t, d, r, 100)
	ofRun := db.SampleRunID.Eq(r)

	if n := do(t, d, db.SampleCount(ofRun)); n != 100 {
		t.Errorf("SampleCount = %d", n)
	}
	if n := do(t, d, db.SampleCount(ofRun, rt.Or(db.SampleStatus.Eq(500), db.SampleRoute.Like("/r1%")))); n != 36 {
		t.Errorf("SampleCount(500 or /r1) = %d, want 36", n)
	}
	if n := do(t, d, db.SampleCount(ofRun, rt.Not(db.SampleStatus.In(200, 300)))); n != 50 {
		t.Errorf("SampleCount(not in 200, 300) = %d", n)
	}
	if !do(t, d, db.SampleExists(ofRun, db.SampleMs.Ge(12))) || do(t, d, db.SampleExists(ofRun, db.SampleMs.Gt(12))) {
		t.Error("SampleExists disagrees with the data, whose ms runs 0 to 12")
	}

	byID := db.SampleOrderBy(db.SampleID.Asc())
	all := do(t, d, db.SampleQuery(ofRun, byID))
	off := do(t, d, db.SampleQuery(ofRun, byID, db.SampleLimit(5), db.SampleOffset(10)))
	equal(t, "Limit 5 Offset 10", ids(off, func(s db.Sample) int { return s.ID }), ids(all[10:15], func(s db.Sample) int { return s.ID }))
	tail := do(t, d, db.SampleQuery(ofRun, byID, db.SampleOffset(95)))
	equal(t, "Offset without Limit", ids(tail, func(s db.Sample) int { return s.ID }), ids(all[95:], func(s db.Sample) int { return s.ID }))
	if q := db.SampleQuery(byID, db.SampleOffset(95)); !strings.Contains(q.SQL, "LIMIT -1 OFFSET ?") {
		t.Errorf("an offset without a limit renders %q", q.SQL)
	}

	dq := db.SampleQuery(ofRun, db.SampleDistinct(), byID)
	if !strings.HasPrefix(dq.SQL, "SELECT DISTINCT ") {
		t.Errorf("Distinct renders %q", dq.SQL)
	}
	if got := do(t, d, dq); len(got) != len(all) {
		t.Errorf("Distinct over whole rows with their key returned %d rows, want %d", len(got), len(all))
	}

	n := do(t, d, db.SampleUpdateWhere(db.SampleSet(db.SampleStatus.Set(503), db.SampleMs.Set(99)), ofRun, db.SampleStatus.Eq(500)))
	if n != 25 {
		t.Errorf("UpdateWhere affected %d, want 25", n)
	}
	if c := do(t, d, db.SampleCount(ofRun, db.SampleStatus.Eq(503), db.SampleMs.Eq(99))); c != 25 {
		t.Errorf("after UpdateWhere %d rows carry the new values", c)
	}
	n = do(t, d, db.SampleDeleteWhere(ofRun, db.SampleStatus.Ge(400)))
	if n != 50 || do(t, d, db.SampleCount(ofRun)) != 50 {
		t.Errorf("DeleteWhere affected %d", n)
	}
	if _, err := run.Run(ctx, d, db.SampleDeleteWhere()); err == nil {
		t.Error("DeleteWhere without a predicate ran")
	}
	if _, err := run.Run(ctx, d, db.SampleUpdateWhere(db.SampleSet(db.SampleMs.Set(0)))); err == nil {
		t.Error("UpdateWhere without a predicate ran")
	}
	if c := do(t, d, db.SampleCount()); c != 55 {
		t.Errorf("the refused bulk writes touched rows: %d samples left, want 55", c)
	}

	nulls := do(t, d, db.RunQuery(db.RunTag.IsNull(), db.RunOrderBy(db.RunID.Asc())))
	if len(nulls) != 1 || nulls[0].ID != r {
		t.Errorf("RunTag.IsNull = %+v", nulls)
	}
	cleared := do(t, d, db.RunUpdateWhere(db.RunSet(db.RunNote.SetNull()), db.RunNote.IsNotNull()))
	if cleared != 1 || do(t, d, db.RunExists(db.RunNote.IsNotNull())) {
		t.Errorf("SetNull over notes cleared %d", cleared)
	}
}
