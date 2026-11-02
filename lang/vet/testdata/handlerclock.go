// analyzers: handlerclock
package app

import (
	"time"

	"github.com/Piechutowski/volt"
)

type Home struct{}

func (Home) Index(r volt.Request) volt.Response {
	fresh := r.Now().Add(-time.Hour)
	late := time.Now()                 //WANT handlerclock
	if time.Since(fresh) > time.Hour { //WANT handlerclock
		return volt.NoContent()
	}
	return volt.Render(late)
}
