package api

import (
	"net/http"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/nao/run"
	"shop/lib__api/lib__db"
)

// BearerAuth is the plug routes.volt names (§pipelines). Go ignores this
// testdata directory; the Volt checker reads it.
func BearerAuth(next http.Handler) http.Handler { return next }

type customers struct{}

func (customers) Card(r volt.Request, id int, customer db.Customer) volt.Response {
	return volt.NoContent()
}

// Open is the shop's API over conn, which app__api serves.
func Open(conn run.DB) http.Handler {
	return NewRouter(Controllers{Customers: customers{}, DB: conn})
}
