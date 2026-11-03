// Package suite is the blog contract every implementation of the
// comparison passes unchanged, Volt's and the four other stacks'. Run
// checks a server over HTTP alone, so it holds each to the same
// behaviour whatever builds it.
//
// The contract. Every route needs X-User: a member id; missing, not a
// number or no such member is 401. The database starts with members 1
// Ada, 2 Grace and 3 Ken. Bodies are JSON; a malformed body is 400, a
// refusal is a JSON object with a "message", and a 422 names the field
// it refuses somewhere in its body. Times are RFC 3339.
//
//	POST   /posts                       {slug, title, body, tags} -> 201 post
//	GET    /posts/{id}                  -> 200 post; a draft is 404 to all but its author
//	PATCH  /posts/{id}                  {title?, body?} -> 200 post; 403 not the author, 409 not a draft
//	POST   /posts/{id}/publish          -> 200 post; 403 not the author, 409 not a draft, 422 body under 100 characters
//	GET    /posts/{id}/audit            -> 200 [{action, member_id, at}]; 403 not the author
//	GET    /posts?tag=&after=&limit=    -> 200 {posts: [feed item], next}
//	POST   /posts/{id}/comments         {body} -> 201 comment; 409 not published, 429 a fourth in an hour
//	GET    /posts/{id}/comments         -> 200 [comment], oldest first
//	DELETE /posts/{id}/comments/{cid}   -> 204; 403 neither the comment's nor the post's author
//	GET    /me/stats                    -> 200 {drafts, published, comments_received}
//
// A post is {id, slug, title, body, state, author_id, tags,
// published_at, created_at}: state is draft or published, tags are
// sorted, published_at is null until it is published. The slug is
// unique (409) and matches [a-z0-9-]{1,60}; the title is 1 to 200
// characters; a post has at most 5 tags, each matching [a-z0-9-]{1,20};
// any other value is 422. Publishing sets published_at and writes an
// audit entry {action: "publish"} in one transaction.
//
// A comment is {id, post_id, member_id, body, created_at}; its body is 1
// to 1000 characters. A member writes at most 3 comments on one post in
// an hour; the fourth is 429.
//
// The feed lists published posts newest first (published_at, then id),
// optionally those with a tag, each {id, slug, title, author_name,
// published_at, comment_count}. limit is 1 to 50, 10 by default; next
// is an opaque cursor for after, "" on the last page; a bad limit or
// cursor is 422.
//
// Stats count the member's drafts and published posts and the comments
// on their posts.
package suite

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
)

// Post is a post as the contract renders it.
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

// FeedItem is one entry of the feed.
type FeedItem struct {
	ID           int       `json:"id"`
	Slug         string    `json:"slug"`
	Title        string    `json:"title"`
	AuthorName   string    `json:"author_name"`
	PublishedAt  time.Time `json:"published_at"`
	CommentCount int       `json:"comment_count"`
}

// Feed is one page of the feed.
type Feed struct {
	Posts []FeedItem `json:"posts"`
	Next  string     `json:"next"`
}

// Comment is a comment as the contract renders it.
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

const (
	ada   = 1
	grace = 2
	ken   = 3
)

type client struct {
	t    *testing.T
	base string
}

type reply struct {
	status int
	body   []byte
}

func (c client) do(user int, method, path string, body any) reply {
	c.t.Helper()
	var r io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		r = strings.NewReader(b)
	default:
		data, err := json.Marshal(b)
		if err != nil {
			c.t.Fatal(err)
		}
		r = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.base+path, r)
	if err != nil {
		c.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if user != 0 {
		req.Header.Set("X-User", fmt.Sprint(user))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatal(err)
	}
	return reply{resp.StatusCode, data}
}

func (c client) want(r reply, status int, out any, what string) {
	c.t.Helper()
	if r.status != status {
		c.t.Fatalf("%s: status %d, want %d: %s", what, r.status, status, r.body)
	}
	if out != nil {
		if err := json.Unmarshal(r.body, out); err != nil {
			c.t.Fatalf("%s: %v in %s", what, err, r.body)
		}
	}
}

func (c client) refused(r reply, status int, field, what string) {
	c.t.Helper()
	if r.status != status {
		c.t.Errorf("%s: status %d, want %d: %s", what, r.status, status, r.body)
		return
	}
	var m map[string]any
	if err := json.Unmarshal(r.body, &m); err != nil || m["message"] == nil {
		c.t.Errorf("%s: a refusal is a JSON object with a message, got %s", what, r.body)
	}
	if field != "" && !strings.Contains(string(r.body), field) {
		c.t.Errorf("%s: the refusal does not name %q: %s", what, field, r.body)
	}
}

func longBody(n int) string { return strings.Repeat("Volt measures itself. ", n/22+1)[:n] }

// Run checks the whole contract against the server at base, which must
// start from an empty database with its three members.
func Run(t *testing.T, base string) {
	t.Run("every route needs a member", func(t *testing.T) {
		c := client{t, base}
		for _, h := range []string{"", "99", "abc"} {
			req, _ := http.NewRequest("GET", base+"/me/stats", nil)
			if h != "" {
				req.Header.Set("X-User", h)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			data, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			c.refused(reply{resp.StatusCode, data}, 401, "", "X-User "+h)
		}
	})

	var first Post
	t.Run("a draft is created and checked", func(t *testing.T) {
		c := client{t, base}
		c.want(c.do(ada, "POST", "/posts", map[string]any{"slug": "first-post", "title": "First post", "body": "Short.", "tags": []string{"sqlite", "go"}}), 201, &first, "create")
		if first.ID == 0 || first.State != "draft" || first.AuthorID != ada || first.PublishedAt != nil || !slices.Equal(first.Tags, []string{"go", "sqlite"}) || first.CreatedAt.IsZero() {
			t.Errorf("created %+v", first)
		}
		c.refused(c.do(ada, "POST", "/posts", map[string]any{"slug": "first-post", "title": "Again", "body": "x"}), 409, "", "a taken slug")
		c.refused(c.do(ada, "POST", "/posts", map[string]any{"slug": "empty-title", "title": "", "body": "x"}), 422, "title", "an empty title")
		c.refused(c.do(ada, "POST", "/posts", map[string]any{"slug": "long-title", "title": strings.Repeat("t", 201), "body": "x"}), 422, "title", "a title over 200")
		c.refused(c.do(ada, "POST", "/posts", map[string]any{"slug": "Bad Slug", "title": "t", "body": "x"}), 422, "slug", "a bad slug")
		c.refused(c.do(ada, "POST", "/posts", map[string]any{"slug": "many-tags", "title": "t", "body": "x", "tags": []string{"a", "b", "c", "d", "e", "f"}}), 422, "tags", "six tags")
		c.refused(c.do(ada, "POST", "/posts", map[string]any{"slug": "bad-tag", "title": "t", "body": "x", "tags": []string{"Bad!"}}), 422, "tags", "a bad tag")
		c.refused(c.do(ada, "POST", "/posts", `{"slug":`), 400, "", "a malformed body")
	})
	path := fmt.Sprintf("/posts/%d", first.ID)

	t.Run("a draft is its author's alone", func(t *testing.T) {
		c := client{t, base}
		var got Post
		c.want(c.do(ada, "GET", path, nil), 200, &got, "the author reads")
		if got.ID != first.ID || got.Title != "First post" || !slices.Equal(got.Tags, []string{"go", "sqlite"}) {
			t.Errorf("read %+v", got)
		}
		c.refused(c.do(grace, "GET", path, nil), 404, "", "another member reads a draft")
		c.refused(c.do(ada, "GET", "/posts/999", nil), 404, "", "a missing post")
		c.refused(c.do(grace, "PATCH", path, map[string]any{"title": "Mine now"}), 403, "", "another member edits")
		c.want(c.do(ada, "PATCH", path, map[string]any{"title": "First post, edited"}), 200, &got, "the author edits")
		if got.Title != "First post, edited" || got.Body != "Short." {
			t.Errorf("edited %+v", got)
		}
		c.refused(c.do(ada, "PATCH", path, map[string]any{"title": ""}), 422, "title", "an edit to an empty title")
	})

	t.Run("publishing is a checked transition", func(t *testing.T) {
		c := client{t, base}
		c.refused(c.do(ada, "POST", path+"/publish", nil), 422, "body", "publishing a short body")
		c.want(c.do(ada, "PATCH", path, map[string]any{"body": longBody(120)}), 200, nil, "a longer body")
		c.refused(c.do(grace, "POST", path+"/publish", nil), 403, "", "another member publishes")
		var got Post
		c.want(c.do(ada, "POST", path+"/publish", nil), 200, &got, "the author publishes")
		if got.State != "published" || got.PublishedAt == nil || got.PublishedAt.Before(got.CreatedAt.Add(-time.Second)) {
			t.Errorf("published %+v", got)
		}
		c.refused(c.do(ada, "POST", path+"/publish", nil), 409, "", "publishing twice")
		c.refused(c.do(ada, "PATCH", path, map[string]any{"title": "Too late"}), 409, "", "editing a published post")
		c.want(c.do(grace, "GET", path, nil), 200, nil, "anyone reads a published post")
		var audit []Audit
		c.want(c.do(ada, "GET", path+"/audit", nil), 200, &audit, "the audit")
		if len(audit) != 1 || audit[0].Action != "publish" || audit[0].MemberID != ada || audit[0].At.IsZero() {
			t.Errorf("audit %+v", audit)
		}
		c.refused(c.do(grace, "GET", path+"/audit", nil), 403, "", "another member's audit")
	})

	var draft Post
	t.Run("comments follow their rules", func(t *testing.T) {
		c := client{t, base}
		c.want(c.do(grace, "POST", "/posts", map[string]any{"slug": "grace-draft", "title": "Grace's draft", "body": "x"}), 201, &draft, "a second draft")
		c.refused(c.do(ada, "POST", fmt.Sprintf("/posts/%d/comments", draft.ID), map[string]any{"body": "Early"}), 409, "", "a comment on a draft")
		var mine []Comment
		for i := range 3 {
			var cm Comment
			c.want(c.do(grace, "POST", path+"/comments", map[string]any{"body": fmt.Sprintf("Comment %d", i)}), 201, &cm, "a comment")
			if cm.PostID != first.ID || cm.MemberID != grace || cm.CreatedAt.IsZero() {
				t.Errorf("comment %+v", cm)
			}
			mine = append(mine, cm)
		}
		c.refused(c.do(grace, "POST", path+"/comments", map[string]any{"body": "One too many"}), 429, "", "a fourth comment in an hour")
		var kens Comment
		c.want(c.do(ken, "POST", path+"/comments", map[string]any{"body": "Ken's"}), 201, &kens, "another member's comment")
		c.refused(c.do(ken, "POST", path+"/comments", map[string]any{"body": ""}), 422, "body", "an empty comment")
		c.refused(c.do(ken, "POST", path+"/comments", map[string]any{"body": strings.Repeat("c", 1001)}), 422, "body", "a comment over 1000")
		var all []Comment
		c.want(c.do(ken, "GET", path+"/comments", nil), 200, &all, "the comments")
		if len(all) != 4 || all[0].ID != mine[0].ID || all[3].ID != kens.ID {
			t.Errorf("comments %+v", all)
		}
		c.refused(c.do(ken, "DELETE", fmt.Sprintf("%s/comments/%d", path, mine[0].ID), nil), 403, "", "deleting another member's comment")
		c.want(c.do(grace, "DELETE", fmt.Sprintf("%s/comments/%d", path, mine[0].ID), nil), 204, nil, "deleting one's own comment")
		c.want(c.do(ada, "DELETE", fmt.Sprintf("%s/comments/%d", path, kens.ID), nil), 204, nil, "the post's author deletes a comment")
		c.refused(c.do(ada, "DELETE", fmt.Sprintf("%s/comments/%d", path, kens.ID), nil), 404, "", "a deleted comment")
	})

	t.Run("the feed pages newest first", func(t *testing.T) {
		c := client{t, base}
		want := []int{first.ID}
		for i, tags := range [][]string{{"go"}, {"web"}, {"go", "web"}, nil} {
			var p Post
			c.want(c.do(ken, "POST", "/posts", map[string]any{"slug": fmt.Sprintf("ken-%d", i), "title": fmt.Sprintf("Ken %d", i), "body": longBody(150), "tags": tags}), 201, &p, "a post to publish")
			c.want(c.do(ken, "POST", fmt.Sprintf("/posts/%d/publish", p.ID), nil), 200, nil, "publishing it")
			want = append([]int{p.ID}, want...)
		}
		var got []FeedItem
		after := ""
		for pages := 0; ; pages++ {
			var f Feed
			c.want(c.do(grace, "GET", "/posts?limit=2&after="+url.QueryEscape(after), nil), 200, &f, "a feed page")
			if len(f.Posts) > 2 || pages > 5 {
				t.Fatalf("page %d holds %d posts", pages, len(f.Posts))
			}
			got = append(got, f.Posts...)
			if f.Next == "" {
				break
			}
			after = f.Next
		}
		ids := make([]int, len(got))
		for i, it := range got {
			ids[i] = it.ID
		}
		if !slices.Equal(ids, want) {
			t.Fatalf("the feed paged %v, want %v", ids, want)
		}
		last := got[len(got)-1]
		if last.Slug != "first-post" || last.AuthorName != "Ada" || last.CommentCount != 2 || last.PublishedAt.IsZero() {
			t.Errorf("the first post in the feed %+v", last)
		}
		var tagged Feed
		c.want(c.do(grace, "GET", "/posts?tag=go", nil), 200, &tagged, "the feed of a tag")
		if len(tagged.Posts) != 3 || tagged.Next != "" || tagged.Posts[0].ID != want[1] {
			t.Errorf("tagged %+v", tagged)
		}
		var all Feed
		c.want(c.do(grace, "GET", "/posts", nil), 200, &all, "the default page")
		if len(all.Posts) != 5 || all.Next != "" {
			t.Errorf("the default page holds %d posts, next %q", len(all.Posts), all.Next)
		}
		for _, q := range []string{"limit=0", "limit=51", "limit=x", "after=garbage"} {
			c.refused(c.do(grace, "GET", "/posts?"+q, nil), 422, "", "the feed with "+q)
		}
	})

	t.Run("stats count a member's posts", func(t *testing.T) {
		c := client{t, base}
		var s Stats
		c.want(c.do(ada, "GET", "/me/stats", nil), 200, &s, "Ada's stats")
		if s != (Stats{Drafts: 0, Published: 1, CommentsReceived: 2}) {
			t.Errorf("Ada's stats %+v", s)
		}
		c.want(c.do(grace, "GET", "/me/stats", nil), 200, &s, "Grace's stats")
		if s != (Stats{Drafts: 1, Published: 0, CommentsReceived: 0}) {
			t.Errorf("Grace's stats %+v", s)
		}
	})
}
