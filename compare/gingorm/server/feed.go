package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Piechutowski/volt/compare/gingorm/api"
	"github.com/Piechutowski/volt/compare/gingorm/models"
)

const (
	feedDefaultLimit = 10
	feedMaxLimit     = 50
)

type feedCursor struct {
	PublishedAt time.Time `json:"published_at"`
	ID          uint      `json:"id"`
}

func encodeCursor(item api.FeedItem) string {
	data, _ := json.Marshal(feedCursor{PublishedAt: item.PublishedAt.UTC(), ID: item.ID})
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeCursor(s string) (feedCursor, error) {
	var cur feedCursor
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return cur, err
	}
	if err := json.Unmarshal(data, &cur); err != nil {
		return cur, err
	}
	if cur.ID == 0 || cur.PublishedAt.IsZero() {
		return cur, errors.New("incomplete cursor")
	}
	cur.PublishedAt = cur.PublishedAt.UTC()
	return cur, nil
}

func (h *handlers) feed(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(feedDefaultLimit)))
	if err != nil || limit < 1 || limit > feedMaxLimit {
		abortWithMessage(c, http.StatusUnprocessableEntity, "limit: must be a number from 1 to 50")
		return
	}
	query := h.db.WithContext(c.Request.Context()).
		Model(&models.Post{}).
		Select("posts.id, posts.slug, posts.title, posts.published_at, members.name AS author_name, COUNT(comments.id) AS comment_count").
		Joins("JOIN members ON members.id = posts.author_id").
		Joins("LEFT JOIN comments ON comments.post_id = posts.id").
		Where("posts.state = ?", models.StatePublished).
		Group("posts.id").
		Order("posts.published_at DESC, posts.id DESC").
		Limit(limit + 1)
	if after := c.Query("after"); after != "" {
		cur, err := decodeCursor(after)
		if err != nil {
			abortWithMessage(c, http.StatusUnprocessableEntity, "after: not a cursor of this feed")
			return
		}
		query = query.Where("(posts.published_at < ? OR (posts.published_at = ? AND posts.id < ?))", cur.PublishedAt, cur.PublishedAt, cur.ID)
	}
	if tag := c.Query("tag"); tag != "" {
		tagged := h.db.Table("post_tags").
			Select("post_tags.post_id").
			Joins("JOIN tags ON tags.id = post_tags.tag_id").
			Where("tags.name = ?", tag)
		query = query.Where("posts.id IN (?)", tagged)
	}
	items := []api.FeedItem{}
	if err := query.Scan(&items).Error; err != nil {
		abortWithError(c, err)
		return
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		next = encodeCursor(items[limit-1])
	}
	c.JSON(http.StatusOK, api.Feed{Posts: items, Next: next})
}
