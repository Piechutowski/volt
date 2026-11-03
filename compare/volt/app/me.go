package app

import (
	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/compare/volt/db"
	"github.com/Piechutowski/volt/nao/rt"
)

// Stats are a member's counts.
type Stats struct {
	Drafts           int `json:"drafts"`
	Published        int `json:"published"`
	CommentsReceived int `json:"comments_received"`
}

// Me is the route of the member's own numbers.
type Me struct{}

// Stats counts the member's drafts, published posts and the comments on
// them.
func (Me) Stats(r volt.Request, ms []db.Member) volt.Response {
	m, ok := memberOf(ms)
	if !ok {
		return unauthorized()
	}
	return volt.Then(rt.All(db.PostByState(m.ID), db.PostReceived(m.ID)), func(out []any) volt.Response {
		s := Stats{CommentsReceived: out[1].(db.Received).N}
		for _, b := range out[0].([]db.ByState) {
			switch b.State {
			case "draft":
				s.Drafts = b.N
			case "published":
				s.Published = b.N
			}
		}
		return volt.Render(s)
	})
}
