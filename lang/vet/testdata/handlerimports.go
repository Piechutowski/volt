// analyzers: handlerimports
package app

import (
	"database/sql" //WANT handlerimports
	"net/http"
	"net/url"
	"os"      //WANT handlerimports
	"syscall" //WANT handlerimports

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/nao/run" //WANT handlerimports
)

type Home struct{}

func (Home) Index(r volt.Request) volt.Response {
	_, _, _ = sql.Open, os.Getenv, syscall.Getpid
	_, _ = url.Parse(r.Path())
	_ = run.DB{}
	return volt.RenderStatus(http.StatusOK, nil)
}
