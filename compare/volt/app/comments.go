package app

import (
	"time"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/compare/volt/db"
)

// Comments are the comment routes.
type Comments struct{}

// Create stores a comment on a published post, at most three per
// member per post in an hour.
func (Comments) Create(r volt.Request, ms []db.Member, id int, body CommentForm, p db.Post) volt.Response {
	m, ok := memberOf(ms)
	if !ok {
		return unauthorized()
	}
	if p.State != "published" {
		return volt.Reject(409, "only a published post takes comments")
	}
	cs := db.CommentNew().PostIDSet(id).MemberIDSet(m.ID).BodySet(body.Body).Validate()
	if !cs.Valid() {
		return volt.Fail(cs.Err())
	}
	return volt.Then(db.CommentRecent(id, m.ID, r.Now().Add(-time.Hour)), func(recent db.Recent) volt.Response {
		if recent.N >= 3 {
			return volt.Reject(429, "at most 3 comments on a post in an hour")
		}
		return volt.Then(db.CommentCreate(cs), func(c db.Comment) volt.Response { return volt.Created(c) })
	})
}

// List renders a post's comments, oldest first.
func (Comments) List(r volt.Request, ms []db.Member, id int, thread []db.Comment) volt.Response {
	if _, ok := memberOf(ms); !ok {
		return unauthorized()
	}
	return volt.Render(thread)
}

// Delete removes a comment; only its author or the post's may.
func (Comments) Delete(r volt.Request, ms []db.Member, id, cid int, p db.Post, c db.Comment) volt.Response {
	m, ok := memberOf(ms)
	if !ok {
		return unauthorized()
	}
	switch {
	case c.PostID != id:
		return volt.Reject(404, "not found")
	case m.ID != c.MemberID && m.ID != p.AuthorID:
		return volt.Reject(403, "only the comment's or the post's author deletes it")
	}
	return volt.Then(db.CommentDelete(cid), func(int64) volt.Response { return volt.NoContent() })
}
