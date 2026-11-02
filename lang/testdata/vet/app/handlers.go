package app

import (
	"io"
	"os"
	"time"

	"github.com/Piechutowski/volt"
	"vetgolden/data"
)

var hits int

func init() { hits = 0 }

type Home struct{ seen bool }

func (Home) Index(r volt.Request) volt.Response {
	hits++
	go func() {}()
	_ = os.Getenv("HOME")
	if time.Since(r.Now()) > time.Second {
		return volt.Stream(func(w io.Writer) error { return nil })
	}
	return volt.NoContent()
}

type Posts struct{}

func (Posts) Show(r volt.Request, id int) volt.Response {
	return volt.Then(data.PostGet(id), func(p data.Post) volt.Response { return volt.Render(p) })
}
