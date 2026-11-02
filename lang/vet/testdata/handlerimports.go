// analyzers: handlerimports
package app

import (
	"database/sql" //WANT handlerimports
	"net/http"
	"os" //WANT handlerimports

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/nao/run" //WANT handlerimports
)

type Home struct{}

func (Home) Index(r volt.Request) volt.Response {
	_, _ = sql.Open, os.Getenv
	_ = run.DB{}
	return volt.RenderStatus(http.StatusOK, nil)
}
