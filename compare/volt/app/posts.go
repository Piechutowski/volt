package app

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/compare/volt/db"
	"github.com/Piechutowski/volt/nao/rt"
)

// PostView is a post with its tags, as every post route renders it.
type PostView struct {
	db.Post
	Tags []string `json:"tags"`
}

// FeedPage is one page of the feed.
type FeedPage struct {
	Posts []db.Feed `json:"posts"`
	Next  string    `json:"next"`
}

// Posts are the post routes.
type Posts struct{}

func postView(p db.Post, tags []db.PostTag) PostView {
	v := PostView{Post: p, Tags: []string{}}
	for _, t := range tags {
		v.Tags = append(v.Tags, t.Tag)
	}
	return v
}

func postRender(status int, p db.Post) volt.Response {
	return volt.Then(db.PostTagOfPost(p.ID), func(tags []db.PostTag) volt.Response {
		return volt.RenderStatus(status, postView(p, tags))
	})
}

func tagsChecked(tags []string) ([]string, volt.Response, bool) {
	if len(tags) > 5 {
		return nil, volt.Reject(422, "tags: at most 5"), false
	}
	for _, t := range tags {
		if err := db.TagShape(t); err != nil {
			return nil, volt.Reject(422, fmt.Sprintf("tags: %q %v", t, err)), false
		}
	}
	out := slices.Clone(tags)
	slices.Sort(out)
	return slices.Compact(out), volt.Response{}, true
}

// Create stores a draft and its tags in one transaction, the tags
// carrying the key the post's insert assigned (§generated-query-surface),
// so a refused tag leaves no post.
func (Posts) Create(r volt.Request, member db.Member, body PostForm) volt.Response {
	tags, refused, ok := tagsChecked(body.Tags)
	if !ok {
		return refused
	}
	cs := db.PostNew().SlugSet(body.Slug).TitleSet(body.Title).BodySet(body.Body).AuthorIDSet(member.ID).Validate()
	if !cs.Valid() {
		return volt.Fail(cs.Err())
	}
	create := rt.Then(db.PostCreate(cs), func(p db.Post) rt.Effect[db.Post] {
		writes := make([]rt.AnyEffect, len(tags))
		for i, t := range tags {
			writes[i] = db.PostTagCreate(db.PostTagNew().PostIDSet(p.ID).TagSet(t))
		}
		return rt.Map(rt.Tx(writes...), func([]any) db.Post { return p })
	})
	return volt.Then(rt.Atomic(create), func(p db.Post) volt.Response { return postRender(201, p) })
}

// Show renders a post; a draft is a 404 to all but its author.
func (Posts) Show(r volt.Request, member db.Member, id int, p db.Post) volt.Response {
	if p.State != "published" && p.AuthorID != member.ID {
		return volt.Reject(404, "not found")
	}
	return postRender(200, p)
}

// Edit changes the title or body of the author's own draft.
func (Posts) Edit(r volt.Request, member db.Member, id int, body PostEdit, p db.Post) volt.Response {
	switch {
	case p.AuthorID != member.ID:
		return volt.Reject(403, "only the author edits a post")
	case p.State != "draft":
		return volt.Reject(409, "only a draft is edited")
	}
	cs := p.Changeset()
	if body.Title != nil {
		cs = cs.TitleSet(*body.Title)
	}
	if body.Body != nil {
		cs = cs.BodySet(*body.Body)
	}
	if cs = cs.Validate(); !cs.Valid() {
		return volt.Fail(cs.Err())
	}
	return volt.Then(db.PostUpdate(cs), func(p db.Post) volt.Response { return postRender(200, p) })
}

// Publish turns the author's draft into a published post and audits it,
// both in one transaction.
func (Posts) Publish(r volt.Request, member db.Member, id int, p db.Post) volt.Response {
	switch {
	case p.AuthorID != member.ID:
		return volt.Reject(403, "only the author publishes a post")
	case p.State != "draft":
		return volt.Reject(409, "only a draft is published")
	case utf8.RuneCountInString(p.Body) < 100:
		return volt.Reject(422, "body: at least 100 characters to publish")
	}
	cs := p.Changeset().StateSet("published").PublishedAtSet(rt.Some(r.Now()))
	audit := db.AuditNew().PostIDSet(p.ID).MemberIDSet(member.ID).ActionSet("publish").AtSet(r.Now())
	return volt.Then(rt.Tx(db.PostUpdate(cs), db.AuditCreate(audit)), func(out []any) volt.Response {
		return postRender(200, out[0].(db.Post))
	})
}

// Audit lists a post's audit entries to its author.
func (Posts) Audit(r volt.Request, member db.Member, id int, p db.Post, trail []db.Audit) volt.Response {
	if p.AuthorID != member.ID {
		return volt.Reject(403, "only the author reads the audit")
	}
	return volt.Render(trail)
}

// Feed pages the published posts newest first, after a cursor and
// optionally with a tag, reading one post past the page to know whether
// another page follows.
func (Posts) Feed(r volt.Request, member db.Member) volt.Response {
	q := r.Query()
	limit := 10
	if s := q.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 50 {
			return volt.Reject(422, "limit: 1 to 50")
		}
		limit = n
	}
	at, id, tag := rt.Null[time.Time]{}, rt.Null[int]{}, rt.Null[string]{}
	if s := q.Get("after"); s != "" {
		nanos, idText, found := strings.Cut(s, ".")
		n, err1 := strconv.ParseInt(nanos, 10, 64)
		i, err2 := strconv.Atoi(idText)
		if !found || err1 != nil || err2 != nil {
			return volt.Reject(422, "after: not a cursor of this feed")
		}
		at, id = rt.Some(time.Unix(0, n).UTC()), rt.Some(i)
	}
	if s := q.Get("tag"); s != "" {
		tag = rt.Some(s)
	}
	return volt.Then(db.PostFeed(at, id, tag, limit+1), func(rows []db.Feed) volt.Response {
		if len(rows) <= limit {
			return volt.Render(FeedPage{Posts: rows})
		}
		last := rows[limit-1]
		return volt.Render(FeedPage{Posts: rows[:limit], Next: fmt.Sprintf("%d.%d", last.PublishedAt.V.UnixNano(), last.ID)})
	})
}
