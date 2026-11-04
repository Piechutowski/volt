// analyzers: handlerimports,handlerclock
package app

import (
	"os" //WANT handlerimports
	"strconv"
	"time"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/nao/rt"
)

type Member struct{ ID int }

func CurrentMember(r volt.Request) volt.Load[Member] {
	if os.Getenv("BLOCK") != "" {
		return volt.Halt[Member](volt.Reject(503, "closed"))
	}
	id, err := strconv.Atoi(r.Header().Get("X-User"))
	if err != nil {
		return volt.Halt[Member](volt.Reject(401, "sign in"))
	}
	return volt.LoadWith(rt.Then(rt.Lookup[int]{SQL: "SELECT ?", Args: []any{id}}, func(n int) rt.Effect[Member] {
		if time.Now().Hour() > 22 { //WANT handlerclock
			return rt.Fail[Member](volt.Error(503, "closed for the night"))
		}
		return rt.Pure(Member{ID: n})
	}))
}
