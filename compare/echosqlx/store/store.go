// Package store is the blog's data access: sqlx queries over SQLite that
// scan into the structs below. Times are written as UTC time.Time values,
// which go-sqlite3 stores as text in one layout, so SQL compares and
// orders them as it does strings.
package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/mattn/go-sqlite3"
)

// ErrSlugTaken is returned when a new post's slug is already in use.
var ErrSlugTaken = errors.New("slug already taken")

// Member is a member of the blog.
type Member struct {
	ID    int    `db:"id" json:"id"`
	Name  string `db:"name" json:"name"`
	Email string `db:"email" json:"email"`
}

// Post is a post with its tags.
type Post struct {
	ID          int        `db:"id" json:"id"`
	Slug        string     `db:"slug" json:"slug"`
	Title       string     `db:"title" json:"title"`
	Body        string     `db:"body" json:"body"`
	State       string     `db:"state" json:"state"`
	AuthorID    int        `db:"author_id" json:"author_id"`
	Tags        []string   `db:"-" json:"tags"`
	PublishedAt *time.Time `db:"published_at" json:"published_at"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
}

// FeedItem is one published post in the feed.
type FeedItem struct {
	ID           int       `db:"id" json:"id"`
	Slug         string    `db:"slug" json:"slug"`
	Title        string    `db:"title" json:"title"`
	AuthorName   string    `db:"author_name" json:"author_name"`
	PublishedAt  time.Time `db:"published_at" json:"published_at"`
	CommentCount int       `db:"comment_count" json:"comment_count"`
}

// Comment is a comment on a post.
type Comment struct {
	ID        int       `db:"id" json:"id"`
	PostID    int       `db:"post_id" json:"post_id"`
	MemberID  int       `db:"member_id" json:"member_id"`
	Body      string    `db:"body" json:"body"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// Audit is one audit entry of a post.
type Audit struct {
	Action   string    `db:"action" json:"action"`
	MemberID int       `db:"member_id" json:"member_id"`
	At       time.Time `db:"at" json:"at"`
}

// Stats are a member's counts.
type Stats struct {
	Drafts           int `db:"drafts" json:"drafts"`
	Published        int `db:"published" json:"published"`
	CommentsReceived int `db:"comments_received" json:"comments_received"`
}

// FeedQuery selects one page of the feed. A zero AfterID starts at the
// newest post.
type FeedQuery struct {
	Tag              string
	AfterPublishedAt time.Time
	AfterID          int
	Limit            int
}

// Store runs the blog's queries.
type Store struct {
	db *sqlx.DB
}

// New wraps an open database.
func New(db *sqlx.DB) *Store {
	return &Store{db: db}
}

// Member reads a member by id.
func (s *Store) Member(ctx context.Context, id int) (Member, error) {
	var m Member
	err := s.db.GetContext(ctx, &m, `SELECT id, name, email FROM members WHERE id = ?`, id)
	return m, err
}

// CreatePost stores a draft and its tags in one transaction.
func (s *Store) CreatePost(ctx context.Context, p Post) (Post, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return Post{}, err
	}
	defer tx.Rollback()

	p.State = "draft"
	p.CreatedAt = time.Now().UTC()
	res, err := tx.NamedExecContext(ctx, `
		INSERT INTO posts (slug, title, body, state, author_id, created_at)
		VALUES (:slug, :title, :body, :state, :author_id, :created_at)`, p)
	if err != nil {
		var se sqlite3.Error
		if errors.As(err, &se) && se.ExtendedCode == sqlite3.ErrConstraintUnique {
			return Post{}, ErrSlugTaken
		}
		return Post{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Post{}, err
	}
	for _, tag := range p.Tags {
		if _, err := tx.ExecContext(ctx, `INSERT INTO post_tags (post_id, tag) VALUES (?, ?)`, id, tag); err != nil {
			return Post{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Post{}, err
	}
	return s.Post(ctx, int(id))
}

// Post reads a post and its sorted tags.
func (s *Store) Post(ctx context.Context, id int) (Post, error) {
	var p Post
	if err := s.db.GetContext(ctx, &p, `SELECT * FROM posts WHERE id = ?`, id); err != nil {
		return Post{}, err
	}
	p.Tags = []string{}
	if err := s.db.SelectContext(ctx, &p.Tags, `SELECT tag FROM post_tags WHERE post_id = ? ORDER BY tag`, id); err != nil {
		return Post{}, err
	}
	return p, nil
}

// UpdatePost changes a post's title and body; a nil value keeps the
// stored one.
func (s *Store) UpdatePost(ctx context.Context, id int, title, body *string) (Post, error) {
	_, err := s.db.NamedExecContext(ctx, `
		UPDATE posts SET title = COALESCE(:title, title), body = COALESCE(:body, body)
		WHERE id = :id`, map[string]any{"id": id, "title": title, "body": body})
	if err != nil {
		return Post{}, err
	}
	return s.Post(ctx, id)
}

// PublishPost publishes a draft and writes its audit entry in one
// transaction. It returns sql.ErrNoRows when the post is no longer a
// draft.
func (s *Store) PublishPost(ctx context.Context, id, memberID int) (Post, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return Post{}, err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	res, err := tx.ExecContext(ctx, `
		UPDATE posts SET state = 'published', published_at = ?
		WHERE id = ? AND state = 'draft'`, now, id)
	if err != nil {
		return Post{}, err
	}
	if n, err := res.RowsAffected(); err != nil {
		return Post{}, err
	} else if n == 0 {
		return Post{}, sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO audits (post_id, member_id, action, at) VALUES (?, ?, 'publish', ?)`,
		id, memberID, now); err != nil {
		return Post{}, err
	}
	if err := tx.Commit(); err != nil {
		return Post{}, err
	}
	return s.Post(ctx, id)
}

// Audits lists a post's audit entries, oldest first.
func (s *Store) Audits(ctx context.Context, postID int) ([]Audit, error) {
	audits := []Audit{}
	err := s.db.SelectContext(ctx, &audits, `
		SELECT action, member_id, at FROM audits WHERE post_id = ? ORDER BY id`, postID)
	return audits, err
}

// Feed lists published posts newest first, by published_at then id.
func (s *Store) Feed(ctx context.Context, q FeedQuery) ([]FeedItem, error) {
	where := []string{"p.state = 'published'"}
	var args []any
	if q.Tag != "" {
		where = append(where, "EXISTS (SELECT 1 FROM post_tags t WHERE t.post_id = p.id AND t.tag = ?)")
		args = append(args, q.Tag)
	}
	if q.AfterID != 0 {
		where = append(where, "(p.published_at < ? OR (p.published_at = ? AND p.id < ?))")
		args = append(args, q.AfterPublishedAt, q.AfterPublishedAt, q.AfterID)
	}
	args = append(args, q.Limit)

	items := []FeedItem{}
	err := s.db.SelectContext(ctx, &items, `
		SELECT p.id, p.slug, p.title, m.name AS author_name, p.published_at,
			(SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id) AS comment_count
		FROM posts p
		JOIN members m ON m.id = p.author_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY p.published_at DESC, p.id DESC
		LIMIT ?`, args...)
	return items, err
}

// CreateComment stores a comment.
func (s *Store) CreateComment(ctx context.Context, c Comment) (Comment, error) {
	c.CreatedAt = time.Now().UTC()
	res, err := s.db.NamedExecContext(ctx, `
		INSERT INTO comments (post_id, member_id, body, created_at)
		VALUES (:post_id, :member_id, :body, :created_at)`, c)
	if err != nil {
		return Comment{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Comment{}, err
	}
	return s.PostComment(ctx, c.PostID, int(id))
}

// PostComment reads a comment of a post by id.
func (s *Store) PostComment(ctx context.Context, postID, id int) (Comment, error) {
	var c Comment
	err := s.db.GetContext(ctx, &c, `SELECT * FROM comments WHERE id = ? AND post_id = ?`, id, postID)
	return c, err
}

// Comments lists a post's comments, oldest first.
func (s *Store) Comments(ctx context.Context, postID int) ([]Comment, error) {
	comments := []Comment{}
	err := s.db.SelectContext(ctx, &comments, `SELECT * FROM comments WHERE post_id = ? ORDER BY id`, postID)
	return comments, err
}

// CountComments counts a member's comments on a post since a time.
func (s *Store) CountComments(ctx context.Context, postID, memberID int, since time.Time) (int, error) {
	var n int
	err := s.db.GetContext(ctx, &n, `
		SELECT COUNT(*) FROM comments
		WHERE post_id = ? AND member_id = ? AND created_at > ?`, postID, memberID, since.UTC())
	return n, err
}

// DeleteComment removes a comment.
func (s *Store) DeleteComment(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM comments WHERE id = ?`, id)
	return err
}

// Stats counts a member's drafts, published posts and the comments on
// their posts.
func (s *Store) Stats(ctx context.Context, memberID int) (Stats, error) {
	var st Stats
	err := s.db.GetContext(ctx, &st, `
		SELECT
			COALESCE(SUM(state = 'draft'), 0) AS drafts,
			COALESCE(SUM(state = 'published'), 0) AS published,
			(SELECT COUNT(*) FROM comments c JOIN posts p ON p.id = c.post_id
				WHERE p.author_id = ?) AS comments_received
		FROM posts WHERE author_id = ?`, memberID, memberID)
	return st, err
}
