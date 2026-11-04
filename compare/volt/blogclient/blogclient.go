// Package blogclient is the typed client of the Volt blog. The generated
// client answers a controller route with a raw *http.Response, so this
// layer names each route's reply type; X-User rides on the runtime
// client's Header.
package blogclient

import (
	"context"
	"net/http"
	"strconv"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/compare/volt/app"
	"github.com/Piechutowski/volt/compare/volt/db"
)

// Client calls the blog as one member.
type Client struct {
	volt.Client
}

// New is a client of the blog at base acting as member id.
func New(base string, id int) *Client {
	return &Client{volt.Client{Base: base, Header: http.Header{"X-User": {strconv.Itoa(id)}}}}
}

// CreatePost stores a draft.
func (c *Client) CreatePost(ctx context.Context, f app.PostForm) (app.PostView, error) {
	var p app.PostView
	return p, c.Do(ctx, "POST", "/posts", f, &p)
}

// Post reads a post.
func (c *Client) Post(ctx context.Context, id int) (app.PostView, error) {
	var p app.PostView
	return p, c.Do(ctx, "GET", app.PathPost(id), nil, &p)
}

// EditPost changes a draft's title or body.
func (c *Client) EditPost(ctx context.Context, id int, e app.PostEdit) (app.PostView, error) {
	var p app.PostView
	return p, c.Do(ctx, "PATCH", app.PathPostEdit(id), e, &p)
}

// Publish publishes a draft.
func (c *Client) Publish(ctx context.Context, id int) (app.PostView, error) {
	var p app.PostView
	return p, c.Do(ctx, "POST", app.PathPublish(id), nil, &p)
}

// Audit lists a post's audit entries.
func (c *Client) Audit(ctx context.Context, id int) ([]db.Audit, error) {
	var a []db.Audit
	return a, c.Do(ctx, "GET", app.PathAudit(id), nil, &a)
}

// Feed reads one page of the feed; tag and after may be empty.
func (c *Client) Feed(ctx context.Context, tag, after string, limit int) (app.FeedPage, error) {
	var opts []volt.URLOption
	if tag != "" {
		opts = append(opts, volt.Query("tag", tag))
	}
	if after != "" {
		opts = append(opts, volt.Query("after", after))
	}
	if limit != 0 {
		opts = append(opts, volt.Query("limit", strconv.Itoa(limit)))
	}
	var f app.FeedPage
	return f, c.Do(ctx, "GET", app.PathFeed(opts...), nil, &f)
}

// Comment comments on a published post.
func (c *Client) Comment(ctx context.Context, id int, body string) (db.Comment, error) {
	var cm db.Comment
	return cm, c.Do(ctx, "POST", app.PathCommentCreate(id), app.CommentForm{Body: body}, &cm)
}

// Comments lists a post's comments.
func (c *Client) Comments(ctx context.Context, id int) ([]db.Comment, error) {
	var cs []db.Comment
	return cs, c.Do(ctx, "GET", app.PathComments(id), nil, &cs)
}

// DeleteComment deletes a comment.
func (c *Client) DeleteComment(ctx context.Context, id, cid int) error {
	return c.Do(ctx, "DELETE", app.PathCommentDelete(id, cid), nil, nil)
}

// Stats reads the member's counts.
func (c *Client) Stats(ctx context.Context) (app.Stats, error) {
	var s app.Stats
	return s, c.Do(ctx, "GET", app.PathStats(), nil, &s)
}
