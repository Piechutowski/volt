// Package models holds the blog's GORM models.
package models

import "time"

// The states a post moves through.
const (
	StateDraft     = "draft"
	StatePublished = "published"
)

// Member is a member of the blog.
type Member struct {
	ID    uint   `gorm:"primaryKey"`
	Name  string `gorm:"not null"`
	Email string `gorm:"not null;uniqueIndex"`
}

// Post is a draft or a published post with its author and tags.
type Post struct {
	ID          uint   `gorm:"primaryKey"`
	Slug        string `gorm:"not null;uniqueIndex"`
	Title       string `gorm:"not null"`
	Body        string `gorm:"not null"`
	State       string `gorm:"not null;index"`
	AuthorID    uint   `gorm:"not null;index"`
	Author      Member
	Tags        []Tag      `gorm:"many2many:post_tags"`
	PublishedAt *time.Time `gorm:"index"`
	CreatedAt   time.Time
}

// Tag is a label shared by the posts that carry it.
type Tag struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"not null;uniqueIndex"`
}

// Comment is a member's comment on a published post.
type Comment struct {
	ID        uint `gorm:"primaryKey"`
	PostID    uint `gorm:"not null;index"`
	Post      Post
	MemberID  uint `gorm:"not null"`
	Member    Member
	Body      string `gorm:"not null"`
	CreatedAt time.Time
}

// Audit records an action a member took on a post.
type Audit struct {
	ID       uint `gorm:"primaryKey"`
	PostID   uint `gorm:"not null;index"`
	Post     Post
	MemberID uint `gorm:"not null"`
	Member   Member
	Action   string    `gorm:"not null"`
	At       time.Time `gorm:"not null"`
}
