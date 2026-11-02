// analyzers: goroutine
package app

import "github.com/Piechutowski/volt"

type Home struct{}

func (Home) Index(r volt.Request) volt.Response {
	go func() {}() //WANT goroutine
	return volt.NoContent()
}
