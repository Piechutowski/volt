// Package blog is the comparison's blog written in Volt: db and app
// declare it, volt gen writes the rest, and Handler serves it.
package blog

//go:generate go run github.com/Piechutowski/volt/cmd/volt gen --sql ./...

import (
	"context"
	"net/http"

	_ "github.com/mattn/go-sqlite3"

	"github.com/Piechutowski/volt/compare/volt/app"
	"github.com/Piechutowski/volt/compare/volt/db"
	"github.com/Piechutowski/volt/nao/run"
)

// Handler opens the SQLite file at path, applies the migrations and
// serves the blog.
func Handler(path string) (http.Handler, error) {
	d, err := run.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	if _, err := run.MigrateUp(context.Background(), d, db.Migrations); err != nil {
		return nil, err
	}
	return app.NewRouter(app.Controllers{DB: d, Posts: app.Posts{}, Comments: app.Comments{}, Me: app.Me{}}), nil
}
