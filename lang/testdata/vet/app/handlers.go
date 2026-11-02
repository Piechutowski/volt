package app

import (
	"io"
	"os"
	"time"

	"github.com/Piechutowski/volt"
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
