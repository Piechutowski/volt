// analyzers: handlercalls
package app

import (
	"io"
	"log"
	"log/slog"
	"net/http"

	"github.com/Piechutowski/volt"
)

type Home struct{}

func (Home) Index(r volt.Request) volt.Response {
	log.Printf("index %s", r.Path())
	slog.Info("index", "path", r.Path())
	if r.Path() == "/quit" {
		log.Fatal("asked to quit") //WANT handlercalls
	}
	if r.Path() == "/panic" {
		log.Panicf("asked to panic at %s", r.Path()) //WANT handlercalls
	}
	log.SetOutput(io.Discard)                      //WANT handlercalls
	slog.SetDefault(slog.New(slog.DiscardHandler)) //WANT handlercalls
	return volt.RenderStatus(http.StatusOK, nil)
}

func (Home) Fetch(r volt.Request) volt.Response {
	res, err := http.Get("https://example.com") //WANT handlercalls
	if err != nil {
		return volt.Fail(err)
	}
	client := http.DefaultClient //WANT handlercalls
	_ = client
	return volt.RenderStatus(res.StatusCode, nil)
}
