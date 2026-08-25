// YOURS — plain Go. Implements the interfaces volt_handlers.go declares.
// Note: implementing them needs NO import of the generated router — Go
// interfaces are implicit; you import only the runtime and your models.
package app

import (
	"log/slog"
	"net/http"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/dataset"

	"example.com/fadn/db"
)

// App is your controller: your struct, your dependencies, your rules.
type App struct {
	Q   *db.Queries
	Log *slog.Logger
}

// DaRRList replaces the generated list for GET /da/r_r
// (routes.volt: `da_r_r [list: App.DaRRList]`).
//
// Business rule: bez filtra roku pokazujemy tylko najnowszy rok ankiety —
// with no year filter, default to the latest survey year instead of
// dumping every year ever collected.
//
// `q` arrives already parsed and validated against the generated column
// whitelist. `next` IS the generated default handler — call it to wrap
// (as here), or ignore it and render entirely yourself.
func (a *App) DaRRList(w http.ResponseWriter, r *volt.Request, q volt.GridQuery,
	next dataset.ListFunc[db.DaRR]) error {

	if !q.HasFilter("rok") {
		latest, err := a.Q.DaRRLatestRok(r.Context()) // the Select block from schema.volt
		if err != nil {
			return err // one error spine: app.Errors decides status + shape
		}
		q = q.WithFilter("rok", volt.Eq(latest))
	}

	a.Log.Info("da_r_r list", "route", r.Route(), "filters", q.FilterNames())
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
