// Package echosqlx is the comparison's blog written with Echo and sqlx:
// goose applies the embedded migrations, store holds the queries, server
// the routes and client a typed client of them.
package echosqlx

import (
	"context"
	"net/http"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"

	"github.com/Piechutowski/volt/compare/echosqlx/migrations"
	"github.com/Piechutowski/volt/compare/echosqlx/server"
	"github.com/Piechutowski/volt/compare/echosqlx/store"
)

// Handler opens the SQLite file at path, applies the migrations and
// returns the Echo instance that serves the blog.
func Handler(path string) (http.Handler, error) {
	db, err := sqlx.Open("sqlite3", path+"?_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL&_txlock=immediate")
	if err != nil {
		return nil, err
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db.DB, migrations.FS)
	if err != nil {
		db.Close()
		return nil, err
	}
	if _, err := provider.Up(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return server.New(store.New(db)), nil
}
