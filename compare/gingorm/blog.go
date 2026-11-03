// Package gingorm is the comparison's blog written with Gin and GORM:
// models are the GORM models, database opens, migrates and seeds the
// SQLite file, server is the Gin router and client its typed client.
package gingorm

import (
	"net/http"

	"github.com/Piechutowski/volt/compare/gingorm/database"
	"github.com/Piechutowski/volt/compare/gingorm/server"
)

// Handler opens the SQLite file at path, migrates it, seeds the three
// members and serves the blog.
func Handler(path string) (http.Handler, error) {
	db, err := database.Open(path)
	if err != nil {
		return nil, err
	}
	if err := database.Migrate(db); err != nil {
		return nil, err
	}
	if err := database.Seed(db); err != nil {
		return nil, err
	}
	return server.New(db), nil
}
