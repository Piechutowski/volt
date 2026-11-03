// Package api holds the JSON bodies the blog server reads and writes,
// shared by the server and its client.
package api

import "time"

// NewPost is the body of POST /posts.
type NewPost struct {
	Slug  string   `json:"slug"`
	Title string   `json:"title"`
	Body  string   `json:"body"`
	Tags  []string `json:"tags"`
}

// PostEdit is the body of PATCH /posts/{id}; a nil field is left as is.
type PostEdit struct {
	Title *string `json:"title,omitempty"`
	Body  *string `json:"body,omitempty"`
}

// NewComment is the body of POST /posts/{id}/comments.
type NewComment struct {
	Body string `json:"body"`
}

// Post is a post with its sorted tags.
type Post struct {
	ID          int64      `json:"id"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Body        string     `json:"body"`
	State       string     `json:"state"`
	AuthorID    int64      `json:"author_id"`
	Tags        []string   `json:"tags"`
	PublishedAt *time.Time `json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Comment is a comment on a post.
type Comment struct {
	ID        int64     `json:"id"`
	PostID    int64     `json:"post_id"`
	MemberID  int64     `json:"member_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// Audit is one entry of a post's audit trail.
type Audit struct {
	Action   string    `json:"action"`
	MemberID int64     `json:"member_id"`
	At       time.Time `json:"at"`
}

// FeedItem is one published post in the feed.
type FeedItem struct {
	ID           int64     `json:"id"`
	Slug         string    `json:"slug"`
	Title        string    `json:"title"`
	AuthorName   string    `json:"author_name"`
	PublishedAt  time.Time `json:"published_at"`
	CommentCount int64     `json:"comment_count"`
}

// Feed is one page of the feed; Next is the cursor of the following
// page, empty on the last.
type Feed struct {
	Posts []FeedItem `json:"posts"`
	Next  string     `json:"next"`
}

// Stats are the counts of GET /me/stats.
type Stats struct {
	Drafts           int64 `json:"drafts"`
	Published        int64 `json:"published"`
	CommentsReceived int64 `json:"comments_received"`
}

// Error is the body of every refusal; Fields maps each refused field to
// its reason on a 422.
type Error struct {
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}
