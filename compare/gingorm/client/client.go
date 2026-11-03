// Package client is a typed Go client of the Gin + GORM blog, one
// method per route, acting as one member.
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

	"github.com/Piechutowski/volt/compare/gingorm/api"
)

// Error is a refusal from the blog: its status and message.
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("blog: %d %s", e.Status, e.Message)
}

// Client calls the blog at BaseURL as the member MemberID.
type Client struct {
	BaseURL    string
	MemberID   uint
	HTTPClient *http.Client
}

// New returns a client of the blog at baseURL acting as memberID.
func New(baseURL string, memberID uint) *Client {
	return &Client{BaseURL: baseURL, MemberID: memberID, HTTPClient: http.DefaultClient}
}

// FeedParams narrow a feed page; zero values are left out.
type FeedParams struct {
	Tag   string
	After string
	Limit int
}

// CreatePost stores a draft.
func (c *Client) CreatePost(ctx context.Context, req api.CreatePostRequest) (api.Post, error) {
	var p api.Post
	return p, c.do(ctx, http.MethodPost, "/posts", nil, req, &p)
}

// GetPost reads a post.
func (c *Client) GetPost(ctx context.Context, id uint) (api.Post, error) {
	var p api.Post
	return p, c.do(ctx, http.MethodGet, postPath(id), nil, nil, &p)
}

// UpdatePost changes a draft's title or body.
func (c *Client) UpdatePost(ctx context.Context, id uint, req api.UpdatePostRequest) (api.Post, error) {
	var p api.Post
	return p, c.do(ctx, http.MethodPatch, postPath(id), nil, req, &p)
}

// PublishPost publishes a draft.
func (c *Client) PublishPost(ctx context.Context, id uint) (api.Post, error) {
	var p api.Post
	return p, c.do(ctx, http.MethodPost, postPath(id)+"/publish", nil, nil, &p)
}

// ListAudit lists a post's audit entries.
func (c *Client) ListAudit(ctx context.Context, id uint) ([]api.AuditEntry, error) {
	var entries []api.AuditEntry
	return entries, c.do(ctx, http.MethodGet, postPath(id)+"/audit", nil, nil, &entries)
}

// Feed reads one page of the feed.
func (c *Client) Feed(ctx context.Context, params FeedParams) (api.Feed, error) {
	query := url.Values{}
	if params.Tag != "" {
		query.Set("tag", params.Tag)
	}
	if params.After != "" {
		query.Set("after", params.After)
	}
	if params.Limit != 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}
	var f api.Feed
	return f, c.do(ctx, http.MethodGet, "/posts", query, nil, &f)
}

// CreateComment comments on a published post.
func (c *Client) CreateComment(ctx context.Context, postID uint, body string) (api.Comment, error) {
	var cm api.Comment
	return cm, c.do(ctx, http.MethodPost, postPath(postID)+"/comments", nil, api.CreateCommentRequest{Body: body}, &cm)
}

// ListComments lists a post's comments, oldest first.
func (c *Client) ListComments(ctx context.Context, postID uint) ([]api.Comment, error) {
	var cs []api.Comment
	return cs, c.do(ctx, http.MethodGet, postPath(postID)+"/comments", nil, nil, &cs)
}

// DeleteComment deletes a comment.
func (c *Client) DeleteComment(ctx context.Context, postID, commentID uint) error {
	path := fmt.Sprintf("%s/comments/%d", postPath(postID), commentID)
	return c.do(ctx, http.MethodDelete, path, nil, nil, nil)
}

// Stats reads the member's counts.
func (c *Client) Stats(ctx context.Context) (api.Stats, error) {
	var s api.Stats
	return s, c.do(ctx, http.MethodGet, "/me/stats", nil, nil, &s)
}

func postPath(id uint) string {
	return "/posts/" + strconv.FormatUint(uint64(id), 10)
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, in, out any) error {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	u := c.BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return err
	}
	req.Header.Set("X-User", strconv.FormatUint(uint64(c.MemberID), 10))
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var refusal api.Error
		if err := json.NewDecoder(resp.Body).Decode(&refusal); err != nil || refusal.Message == "" {
			refusal.Message = http.StatusText(resp.StatusCode)
		}
		return &Error{Status: resp.StatusCode, Message: refusal.Message}
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
