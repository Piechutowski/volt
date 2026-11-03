// Package client is a typed Go client of the blog's HTTP API, one
// method per route.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/Piechutowski/volt/compare/chisqlc/api"
)

// Client calls the blog at a base URL as one member.
type Client struct {
	base   string
	member int64
	http   *http.Client
}

// New returns a client of the blog at base that acts as member.
func New(base string, member int64) *Client {
	return &Client{base: base, member: member, http: http.DefaultClient}
}

// Error is a refusal: the reply's status and its message and fields.
type Error struct {
	Status  int
	Message string
	Fields  map[string]string
}

func (e *Error) Error() string {
	return fmt.Sprintf("blog: %d %s", e.Status, e.Message)
}

// FeedQuery selects a page of the feed; zero fields are left out.
type FeedQuery struct {
	Tag   string
	After string
	Limit int
}

// CreatePost stores a draft.
func (c *Client) CreatePost(ctx context.Context, in api.NewPost) (api.Post, error) {
	var p api.Post
	return p, c.do(ctx, http.MethodPost, "/posts", in, &p)
}

// Post reads a post.
func (c *Client) Post(ctx context.Context, id int64) (api.Post, error) {
	var p api.Post
	return p, c.do(ctx, http.MethodGet, fmt.Sprintf("/posts/%d", id), nil, &p)
}

// EditPost changes a draft's title or body.
func (c *Client) EditPost(ctx context.Context, id int64, in api.PostEdit) (api.Post, error) {
	var p api.Post
	return p, c.do(ctx, http.MethodPatch, fmt.Sprintf("/posts/%d", id), in, &p)
}

// Publish publishes a draft.
func (c *Client) Publish(ctx context.Context, id int64) (api.Post, error) {
	var p api.Post
	return p, c.do(ctx, http.MethodPost, fmt.Sprintf("/posts/%d/publish", id), nil, &p)
}

// Audit lists a post's audit entries.
func (c *Client) Audit(ctx context.Context, id int64) ([]api.Audit, error) {
	var a []api.Audit
	return a, c.do(ctx, http.MethodGet, fmt.Sprintf("/posts/%d/audit", id), nil, &a)
}

// Feed reads one page of the feed.
func (c *Client) Feed(ctx context.Context, q FeedQuery) (api.Feed, error) {
	v := url.Values{}
	if q.Tag != "" {
		v.Set("tag", q.Tag)
	}
	if q.After != "" {
		v.Set("after", q.After)
	}
	if q.Limit != 0 {
		v.Set("limit", strconv.Itoa(q.Limit))
	}
	path := "/posts"
	if len(v) > 0 {
		path += "?" + v.Encode()
	}
	var f api.Feed
	return f, c.do(ctx, http.MethodGet, path, nil, &f)
}

// Comment comments on a published post.
func (c *Client) Comment(ctx context.Context, id int64, body string) (api.Comment, error) {
	var cm api.Comment
	return cm, c.do(ctx, http.MethodPost, fmt.Sprintf("/posts/%d/comments", id), api.NewComment{Body: body}, &cm)
}

// Comments lists a post's comments, oldest first.
func (c *Client) Comments(ctx context.Context, id int64) ([]api.Comment, error) {
	var cs []api.Comment
	return cs, c.do(ctx, http.MethodGet, fmt.Sprintf("/posts/%d/comments", id), nil, &cs)
}

// DeleteComment deletes a comment of a post.
func (c *Client) DeleteComment(ctx context.Context, id, cid int64) error {
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("/posts/%d/comments/%d", id, cid), nil, nil)
}

// Stats reads the member's counts.
func (c *Client) Stats(ctx context.Context) (api.Stats, error) {
	var s api.Stats
	return s, c.do(ctx, http.MethodGet, "/me/stats", nil, &s)
}

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-User", strconv.FormatInt(c.member, 10))
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var refusal api.Error
		if err := json.NewDecoder(resp.Body).Decode(&refusal); err != nil {
			refusal.Message = http.StatusText(resp.StatusCode)
		}
		return &Error{Status: resp.StatusCode, Message: refusal.Message, Fields: refusal.Fields}
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
