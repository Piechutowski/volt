package app

import (
	"net/http"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/nao/rt"
)

// Budgets lists, adds and edits the ceilings on phase wall times
// (spec §resources).
type Budgets struct{}

// Index is every budget by phase and size; state keeps those in one
// state.
func (Budgets) Index(r volt.Request) volt.Response {
	opts := []rt.Opt[db.Budget]{db.BudgetOrderBy(db.BudgetPhase.Asc(), db.BudgetTables.Asc())}
	if s := r.Query().Get("state"); s != "" {
		if err := db.HealthKnown(db.Health(s)); err != nil {
			return volt.Reject(http.StatusBadRequest, err.Error())
		}
		opts = append(opts, db.BudgetState.Eq(db.Health(s)))
	}
	return volt.Then(db.BudgetQuery(opts...), func(bs []db.Budget) volt.Response {
		return volt.Render(bs)
	})
}

// Create adds a budget from a body it decodes itself, since a resources
// action binds no body (spec §resources): the changeset's findings are a
// 422 itemized per field, and a second budget for one phase and size is
// the unique index's 409.
func (Budgets) Create(r volt.Request) volt.Response {
	var p db.BudgetParams
	if err := volt.Decode(r, &p); err != nil {
		return volt.Fail(err)
	}
	cs := db.BudgetNew().Cast(p, db.BudgetPhase, db.BudgetTables, db.BudgetMaxMs).Validate()
	if !cs.Valid() {
		return volt.Reject(http.StatusUnprocessableEntity, "check the fields").WithDetails(cs.Errors())
	}
	return volt.Then(db.BudgetCreate(cs), func(b db.Budget) volt.Response {
		return volt.Created(b)
	})
}

// Show is the budget and every breach against it, newest first.
func (Budgets) Show(r volt.Request, id int) volt.Response {
	return volt.Then(rt.All(db.BudgetGet(id), db.BreachQuery(db.BreachBudgetID.Eq(id), db.BreachOrderBy(db.BreachID.Desc()))), func(res []any) volt.Response {
		return volt.Render(BudgetDetail{Budget: res[0].(db.Budget), Breaches: res[1].([]db.Breach)})
	})
}

// Update sets a budget's ceiling from a changeset body narrowed to
// max_ms (spec §request-bodies); a new ceiling starts healthy.
func (Budgets) Update(r volt.Request, id int, body db.BudgetChangeset) volt.Response {
	if !body.Valid() {
		return volt.Reject(http.StatusUnprocessableEntity, "check the ceiling").WithDetails(body.Errors())
	}
	return volt.Then(db.BudgetUpdate(body.StateSet(db.HealthOK)), func(b db.Budget) volt.Response {
		return volt.Render(b)
	})
}

// Reset returns every breached budget to ok in one statement.
func (Budgets) Reset(r volt.Request) volt.Response {
	return volt.Then(db.BudgetUpdateWhere(db.BudgetSet(db.BudgetState.Set(db.HealthOK)), db.BudgetState.Eq(db.HealthBreached)), func(n int64) volt.Response {
		return volt.Render(Reset{Reset: n})
	})
}
