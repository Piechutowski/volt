// analyzers: handlerclock
package app

import (
	"time"
	clock "time"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/nao/rt"
)

type Home struct{}

func (Home) Index(r volt.Request) volt.Response {
	fresh := r.Now().Add(-time.Hour)
	late := time.Now()                 //WANT handlerclock
	now := clock.Now                   //WANT handlerclock
	if time.Since(fresh) > time.Hour { //WANT handlerclock
		return volt.NoContent()
	}
	return volt.Render(late.Sub(now()))
}

func (Home) Stamp(r volt.Request) volt.Response {
	at := r.Now()
	return volt.Then(rt.Atomic(rt.Then(rt.Lookup[int]{SQL: "SELECT 1"}, func(n int) rt.Effect[int64] {
		stale := time.Since(at) //WANT handlerclock
		return rt.Exec{SQL: "UPDATE posts SET idle = ? WHERE id = ?", Args: []any{stale, n}}
	})), func(int64) volt.Response { return volt.NoContent() })
}
