// YOURS — plain Go. Implements the interfaces volt_handlers.go declares.
// Note: implementing them needs NO import of the generated router — Go
// interfaces are implicit; you import only the runtime and your models.
package app

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/dataset"

	"example.com/metrics/db"
)

// App is your controller: your struct, your dependencies, your rules.
type App struct {
	Q   *db.Queries
	Log *slog.Logger
}

// MsRevenueList replaces the generated list for GET /ms/revenue
// (routes.volt: `ms_revenue [list: App.MsRevenueList]`).
//
// Business rule: with no year filter, default to the latest reporting
// year instead of dumping every year ever collected.
//
// `q` arrives already parsed and validated against the generated column
// whitelist. `next` IS the generated default handler — call it to wrap
// (as here), or ignore it and render entirely yourself.
func (a *App) MsRevenueList(w http.ResponseWriter, r *volt.Request, q volt.GridQuery,
	next dataset.ListFunc[db.MsRevenue]) error {

	if !q.HasFilter("year") {
		latest, err := a.Q.MsRevenueLatestYear(r.Context()) // the Select block from schema.volt
		if err != nil {
			return err // one error spine: app.Errors decides status + shape
		}
		q = q.WithFilter("year", volt.Eq(latest))
	}

	a.Log.Info("ms_revenue list", "route", r.Route(), "filters", q.FilterNames())
	return next(w, r, q) // delegate: generated query, generated negotiation
}

// Health satisfies HealthController.
type Health struct{}

func (Health) Check(w http.ResponseWriter, r *volt.Request) error {
	return volt.JSON(w, map[string]string{"status": "ok"})
}

// Errors is the scope's error handler (routes.volt: [error_handler: app.Errors]).
// Every handler's `return err` lands here — the single place that decides
// status codes, logging, and response shape. If the response is already
// committed, volt invokes this in log-only mode (§4.1).
func Errors(w http.ResponseWriter, r *volt.Request, err error) {
	var httpErr volt.HTTPError
	if errors.As(err, &httpErr) {
		http.Error(w, httpErr.Error(), httpErr.StatusCode())
		return
	}
	slog.Error("unhandled", "route", r.Route(), "err", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}
