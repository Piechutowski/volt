package app

import (
	"net/http"
	"strconv"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/wire"
	"github.com/Piechutowski/volt/nao/rt"
)

// RunPageMax caps the page size the run list accepts.
const RunPageMax = 100

// Runs lists, shows, annotates and deletes runs (spec §resources); the
// list is keyset-paged by hand, since the dynamic layer is not reachable
// over the wire.
type Runs struct{}

// Index is one page of runs, newest first: limit sizes it (20 unless
// given), after continues past a run id from the previous page's Next,
// offset skips rows instead, and tag keeps the runs carrying that tag.
// It reads one run past the page, so Next is set only when a run
// follows and a full last page offers no empty one, then reads the
// page's breaches to name the runs that went over a budget.
func (Runs) Index(r volt.Request) volt.Response {
	q := r.Query()
	limit := 20
	if q.Has("limit") {
		n, err := volt.QueryParam[int](r, "limit")
		if err != nil {
			return volt.Fail(err)
		}
		if n < 1 || n > RunPageMax {
			return volt.Reject(http.StatusBadRequest, "limit runs from 1 to "+strconv.Itoa(RunPageMax))
		}
		limit = n
	}
	var preds []rt.Pred[db.Run]
	if tag := q.Get("tag"); tag != "" {
		preds = append(preds, db.RunTag.Eq(tag))
	}
	opts := []rt.Opt[db.Run]{db.RunOrderBy(db.RunID.Desc()), db.RunLimit(limit + 1)}
	for _, p := range preds {
		opts = append(opts, p)
	}
	switch {
	case q.Has("after") && q.Has("offset"):
		return volt.Reject(http.StatusBadRequest, "after and offset do not combine")
	case q.Has("after"):
		after, err := volt.QueryParam[int](r, "after")
		if err != nil {
			return volt.Fail(err)
		}
		opts = append(opts, db.RunAfter(after))
	case q.Has("offset"):
		offset, err := volt.QueryParam[int](r, "offset")
		if err != nil {
			return volt.Fail(err)
		}
		opts = append(opts, db.RunOffset(offset))
	}
	return volt.Then(rt.All(db.RunQuery(opts...), db.RunCount(preds...)), func(res []any) volt.Response {
		page := wire.RunPage{Runs: res[0].([]db.Run), Total: res[1].(int64), Breached: []int{}}
		if len(page.Runs) > limit {
			page.Runs = page.Runs[:limit]
			page.Next = page.Runs[limit-1].ID
		}
		ids := make([]int, len(page.Runs))
		for i, run := range page.Runs {
			ids[i] = run.ID
		}
		return volt.Then(db.BreachQuery(db.BreachRunID.InList(ids)), func(breaches []db.Breach) volt.Response {
			over := map[int]bool{}
			for _, b := range breaches {
				over[b.RunID] = true
			}
			for _, id := range ids {
				if over[id] {
					page.Breached = append(page.Breached, id)
				}
			}
			return volt.Render(page).WithHeader("X-Total", strconv.FormatInt(page.Total, 10))
		})
	})
}

// Show is everything the run measured, read in parallel by rt.All, then
// the budgets its breaches name; a missing run is the 404 of its lookup.
func (Runs) Show(r volt.Request, id int) volt.Response {
	return volt.Then(rt.All(
		db.RunGet(id),
		db.MsLoadRows(id),
		db.MsCheckRows(id),
		db.MsVetRows(id),
		db.MsGenerateRows(id),
		db.KeystrokeQuery(db.KeystrokeRunID.Eq(id), db.KeystrokeOrderBy(db.KeystrokeTables.Asc())),
		db.SampleLatency(id),
		db.SampleCount(db.SampleRunID.Eq(id), db.SampleStatus.Ge(400)),
		db.BreachQuery(db.BreachRunID.Eq(id), db.BreachOrderBy(db.BreachID.Asc())),
	), func(res []any) volt.Response {
		breaches := res[8].([]db.Breach)
		ids := make([]int, len(breaches))
		for i, b := range breaches {
			ids[i] = b.BudgetID
		}
		return volt.Then(db.BudgetQuery(db.BudgetID.InList(ids), db.BudgetOrderBy(db.BudgetID.Asc())), func(over []db.Budget) volt.Response {
			return volt.Render(wire.RunDetail{
				Run:        res[0].(db.Run),
				Phases:     wire.PhaseRowsOf(res[1].([]db.MsLoad), res[2].([]db.MsCheck), res[3].([]db.MsVet), res[4].([]db.MsGenerate)),
				Keystrokes: res[5].([]db.Keystroke),
				Latency:    res[6].(db.Latency),
				Failed:     res[7].(int64),
				Breaches:   breaches,
				Over:       over,
			})
		})
	})
}

// Delete removes the run and, by the schema's cascades, everything it
// measured; no row deleted is a 404.
func (Runs) Delete(r volt.Request, id int) volt.Response {
	return volt.Then(db.RunDeleteWhere(db.RunID.Eq(id)), func(n int64) volt.Response {
		if n == 0 {
			return volt.Reject(http.StatusNotFound, "no run "+strconv.Itoa(id))
		}
		return volt.NoContent()
	})
}

// Compare sets the load phase of a run beside its baseline at the sizes
// asked, from the route's loads (spec §route-loads): the run binds from
// the path, the baseline and the repeated sizes key from the query
// string. A request missing the baseline is a 400 before any load runs;
// absent sizes read as an empty list and compare nothing.
func (Runs) Compare(r volt.Request, id int, run db.Run, against db.Run, mine []db.MsLoad, theirs []db.MsLoad) volt.Response {
	return volt.Render(wire.ComparisonOf(run, against, mine, theirs))
}

// Options names the methods the run collection takes.
func (Runs) Options(r volt.Request) volt.Response {
	return volt.NoContent().WithHeader("Allow", "GET, HEAD, OPTIONS")
}

// Annotate sets a run's tag and note from a changeset body narrowed by
// only: (spec §request-bodies); a tag no row has is the foreign key's
// 422.
func (Runs) Annotate(r volt.Request, id int, body db.RunChangeset) volt.Response {
	if !body.Valid() {
		return volt.Reject(http.StatusUnprocessableEntity, "check the fields").WithDetails(body.Errors())
	}
	return negotiated(r, func() volt.Response {
		return volt.Then(db.RunUpdate(body), func(run db.Run) volt.Response {
			return volt.Render(run)
		})
	})
}

// Samples stores the collector's timed calls.
type Samples struct{}

// Create stores every sample of the batch in one transaction; a sample
// the validators refuse refuses the batch before anything is written.
func (Samples) Create(r volt.Request, id int, body SampleBatch) volt.Response {
	if len(body.Samples) == 0 {
		return volt.Reject(http.StatusUnprocessableEntity, "the batch holds no sample")
	}
	steps := make([]rt.AnyEffect, 0, len(body.Samples))
	var errs []rt.FieldError
	for _, s := range body.Samples {
		cs := db.SampleNew().RunIDSet(id).RouteSet(s.Route).StatusSet(s.Status).MsSet(s.MS).AtSet(s.At).Validate()
		errs = append(errs, cs.Errors()...)
		steps = append(steps, db.SampleCreate(cs))
	}
	if len(errs) > 0 {
		return volt.Reject(http.StatusUnprocessableEntity, "check the samples").WithDetails(errs)
	}
	return negotiated(r, func() volt.Response {
		return volt.Then(rt.Tx(steps...), func([]any) volt.Response {
			return volt.Created(wire.Stored{Written: len(steps)})
		})
	})
}
