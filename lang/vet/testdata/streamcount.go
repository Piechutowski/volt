// analyzers: streamcount
package app

import (
	"io"

	"github.com/Piechutowski/volt"
)

type Home struct{}

func (Home) Index(r volt.Request) volt.Response { return volt.Render("plain") }

func (Home) Export(r volt.Request) volt.Response {
	return volt.Stream(func(w io.Writer) error { return nil }) //WANT streamcount
}
