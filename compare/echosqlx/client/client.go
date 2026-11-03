// Package client is a typed Go client of the Echo + sqlx blog, written
// by hand: one method per route, each decoding its reply or returning an
// *Error.
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
	"time"
)

// Post is a post with its sorted tags.
type Post struct {
	ID          int        `json:"id"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Body        string     `json:"body"`
	State       string     `json:"state"`
	AuthorID    int        `json:"author_id"`
	Tags        []string   `json:"tags"`
	PublishedAt *time.Time `json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// NewPost is the body of a new post.
type NewPost struct {
	Slug  string   `json:"slug"`
	Title string   `json:"title"`
	Body  string   `json:"body"`
	Tags  []string `json:"tags,omitempty"`
}

// PostEdit is the body of an edit; a nil field is left alone.
type PostEdit struct {
	Title *string `json:"title,omitempty"`
	Body  *string `json:"body,omitempty"`
}

// FeedItem is one published post in the feed.
type FeedItem struct {
	ID           int       `json:"id"`
	Slug         string    `json:"slug"`
	Title        string    `json:"title"`
	AuthorName   string    `json:"author_name"`
	PublishedAt  time.Time `json:"published_at"`
	CommentCount int       `json:"comment_count"`
}

// FeedPage is one page of the feed; Next is empty on the last page.
type FeedPage struct {
	Posts []FeedItem `json:"posts"`
	Next  string     `json:"next"`
}

// Comment is a comment on a post.
type Comment struct {
	ID        int       `json:"id"`
	PostID    int       `json:"post_id"`
	MemberID  int       `json:"member_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// Audit is one audit entry of a post.
type Audit struct {
	Action   string    `json:"action"`
	MemberID int       `json:"member_id"`
	At       time.Time `json:"at"`
}

// Stats are a member's counts.
type Stats struct {
	Drafts           int `json:"drafts"`
	Published        int `json:"published"`
	CommentsReceived int `json:"comments_received"`
}

// Error is a refusal from the server: its status and message, and the
// fields a 422 names.
type Error struct {
	Status  int               `json:"-"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("blog: %d %s", e.Status, e.Message)
}

// Client calls the blog at BaseURL as the member MemberID.
type Client struct {
	BaseURL    string
	MemberID   int
	HTTPClient *http.Client
}

// New is a client of the blog at baseURL acting as member id.
func New(baseURL string, id int) *Client {
	return &Client{BaseURL: baseURL, MemberID: id, HTTPClient: http.DefaultClient}
}

// CreatePost stores a draft.
func (c *Client) CreatePost(ctx context.Context, p NewPost) (Post, error) {
	var out Post
	return out, c.do(ctx, http.MethodPost, "/posts", p, &out)
}

// Post reads a post.
func (c *Client) Post(ctx context.Context, id int) (Post, error) {
	var out Post
	return out, c.do(ctx, http.MethodGet, fmt.Sprintf("/posts/%d", id), nil, &out)
}

// EditPost changes a draft's title or body.
func (c *Client) EditPost(ctx context.Context, id int, e PostEdit) (Post, error) {
	var out Post
	return out, c.do(ctx, http.MethodPatch, fmt.Sprintf("/posts/%d", id), e, &out)
}

// Publish publishes a draft.
func (c *Client) Publish(ctx context.Context, id int) (Post, error) {
	var out Post
	return out, c.do(ctx, http.MethodPost, fmt.Sprintf("/posts/%d/publish", id), nil, &out)
}

// Audit lists a post's audit entries.
func (c *Client) Audit(ctx context.Context, id int) ([]Audit, error) {
	var out []Audit
	return out, c.do(ctx, http.MethodGet, fmt.Sprintf("/posts/%d/audit", id), nil, &out)
}

// Feed reads one page of the feed; an empty tag or after and a zero
// limit are left to the server's defaults.
func (c *Client) Feed(ctx context.Context, tag, after string, limit int) (FeedPage, error) {
	q := url.Values{}
	if tag != "" {
		q.Set("tag", tag)
	}
	if after != "" {
		q.Set("after", after)
	}
	if limit != 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	path := "/posts"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var out FeedPage
	return out, c.do(ctx, http.MethodGet, path, nil, &out)
}

// Comment writes a comment on a published post.
func (c *Client) Comment(ctx context.Context, postID int, body string) (Comment, error) {
	var out Comment
	in := struct {
		Body string `json:"body"`
	}{body}
	return out, c.do(ctx, http.MethodPost, fmt.Sprintf("/posts/%d/comments", postID), in, &out)
}

// Comments lists a post's comments, oldest first.
func (c *Client) Comments(ctx context.Context, postID int) ([]Comment, error) {
	var out []Comment
	return out, c.do(ctx, http.MethodGet, fmt.Sprintf("/posts/%d/comments", postID), nil, &out)
}

// DeleteComment removes a comment.
func (c *Client) DeleteComment(ctx context.Context, postID, commentID int) error {
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("/posts/%d/comments/%d", postID, commentID), nil, nil)
}

// Stats reads the member's counts.
func (c *Client) Stats(ctx context.Context) (Stats, error) {
	var out Stats
	return out, c.do(ctx, http.MethodGet, "/me/stats", nil, &out)
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
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-User", strconv.Itoa(c.MemberID))
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		e := &Error{Status: resp.StatusCode}
		if err := json.NewDecoder(resp.Body).Decode(e); err != nil {
			e.Message = http.StatusText(resp.StatusCode)
		}
		return e
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
