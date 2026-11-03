// Package chisqlc is the comparison's blog written with chi, sqlc and
// goose: sql holds the migrations and queries, sqlc generates db from
// them, and server routes the requests.
package chisqlc

//go:generate go tool sqlc generate

import (
	"context"
	"database/sql"
	"embed"
	"io/fs"
	"net/http"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"

	"github.com/Piechutowski/volt/compare/chisqlc/server"
)

//go:embed sql/schema/*.sql
var migrations embed.FS

// Handler opens the SQLite file at path, applies the goose migrations
// and serves the blog.
func Handler(path string) (http.Handler, error) {
	conn, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000&_txlock=immediate")
	if err != nil {
		return nil, err
	}
	if err := migrate(conn); err != nil {
		conn.Close()
		return nil, err
	}
	return server.New(conn), nil
}

func migrate(conn *sql.DB) error {
	schema, err := fs.Sub(migrations, "sql/schema")
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, conn, schema)
	if err != nil {
		return err
	}
	_, err = provider.Up(context.Background())
	return err
}
