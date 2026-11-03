// Package api holds the blog's request and response bodies, shared by
// the server and the client. The binding tags are the validation rules
// gin applies to a request.
package api

import "time"

// CreatePostRequest is the body of a new post.
type CreatePostRequest struct {
	Slug  string   `json:"slug" binding:"required,slug"`
	Title string   `json:"title" binding:"required,max=200"`
	Body  string   `json:"body"`
	Tags  []string `json:"tags" binding:"max=5,dive,tagname"`
}

// UpdatePostRequest is the body of an edit; a nil field is left alone.
type UpdatePostRequest struct {
	Title *string `json:"title" binding:"omitnil,min=1,max=200"`
	Body  *string `json:"body"`
}

// CreateCommentRequest is the body of a new comment.
type CreateCommentRequest struct {
	Body string `json:"body" binding:"required,max=1000"`
}

// Post is a post with its sorted tags.
type Post struct {
	ID          uint       `json:"id"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Body        string     `json:"body"`
	State       string     `json:"state"`
	AuthorID    uint       `json:"author_id"`
	Tags        []string   `json:"tags"`
	PublishedAt *time.Time `json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// FeedItem is one published post in the feed.
type FeedItem struct {
	ID           uint      `json:"id"`
	Slug         string    `json:"slug"`
	Title        string    `json:"title"`
	AuthorName   string    `json:"author_name"`
	PublishedAt  time.Time `json:"published_at"`
	CommentCount int       `json:"comment_count"`
}

// Feed is one page of the feed; Next is the cursor of the following
// page, empty on the last.
type Feed struct {
	Posts []FeedItem `json:"posts"`
	Next  string     `json:"next"`
}

// Comment is a comment on a post.
type Comment struct {
	ID        uint      `json:"id"`
	PostID    uint      `json:"post_id"`
	MemberID  uint      `json:"member_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// AuditEntry is one action recorded on a post.
type AuditEntry struct {
	Action   string    `json:"action"`
	MemberID uint      `json:"member_id"`
	At       time.Time `json:"at"`
}

// Stats are a member's counts.
type Stats struct {
	Drafts           int `json:"drafts"`
	Published        int `json:"published"`
	CommentsReceived int `json:"comments_received"`
}

// Error is the body of every refusal.
type Error struct {
	Message string `json:"message"`
}
