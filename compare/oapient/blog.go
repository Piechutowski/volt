// Package oapient is the comparison's blog written with oapi-codegen and
// Ent: openapi.yaml is the contract, ent/schema the data model, and
// Handler serves both from a SQLite file.
package oapient

import (
	"context"
	"net/http"

	"entgo.io/ent/dialect"
	_ "github.com/mattn/go-sqlite3"

	"github.com/Piechutowski/volt/compare/oapient/ent"
	"github.com/Piechutowski/volt/compare/oapient/server"
)

// Handler opens the SQLite file at path, migrates it to the Ent schema,
// seeds the three members if there are none and serves the blog.
func Handler(path string) (http.Handler, error) {
	client, err := ent.Open(dialect.SQLite, "file:"+path+"?_fk=1&_busy_timeout=5000&_txlock=immediate")
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	if err := client.Schema.Create(ctx); err != nil {
		return nil, err
	}
	if err := seed(ctx, client); err != nil {
		return nil, err
	}
	return server.Handler(client)
}

func seed(ctx context.Context, client *ent.Client) error {
	n, err := client.Member.Query().Count(ctx)
	if err != nil || n > 0 {
		return err
	}
	return client.Member.CreateBulk(
		client.Member.Create().SetName("Ada").SetEmail("ada@blog.test"),
		client.Member.Create().SetName("Grace").SetEmail("grace@blog.test"),
		client.Member.Create().SetName("Ken").SetEmail("ken@blog.test"),
	).Exec(ctx)
}
