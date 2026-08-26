// YOURS — the whole dependency graph, visible in one function.
package main

import (
	"embed"
	"log"
	"log/slog"
	"net/http"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/nao/rt"

	"example.com/metrics/app"
	"example.com/metrics/db"
)

//go:embed templates
var templates embed.FS

func main() {
	conn, err := rt.Open("sqlite3", "metrics.db") // WAL, busy_timeout, FKs on
	if err != nil {
		log.Fatal(err)
	}
	q := db.New(conn)

	handler := app.New(
		app.Controllers{ // seam 1: your implementations of generated interfaces
			App:    &app.App{Q: q, Log: slog.Default()},
			Health: app.Health{},
		},
		volt.WithQueries(q),           // the explicit DB boundary (R9)
		volt.WithTemplates(templates), // HTML arm of the renderer (§12.3)
	)

	// handler is a plain http.Handler — wrap it, mount it, test it with
	// httptest, serve it. Stdlib from here on out (§0).
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", handler))
}
