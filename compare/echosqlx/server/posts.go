package server

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/labstack/echo/v4"

	"github.com/Piechutowski/volt/compare/echosqlx/store"
)

type postInput struct {
	Slug  string   `json:"slug" validate:"slug"`
	Title string   `json:"title" validate:"min=1,max=200"`
	Body  string   `json:"body"`
	Tags  []string `json:"tags" validate:"max=5,dive,tag"`
}

type postEdit struct {
	Title *string `json:"title" validate:"omitnil,min=1,max=200"`
	Body  *string `json:"body"`
}

type feedPage struct {
	Posts []store.FeedItem `json:"posts"`
	Next  string           `json:"next"`
}

const publishMinBody = 100

func (h *handler) loadPost(c echo.Context) (store.Post, error) {
	id, err := pathID(c, "id")
	if err != nil {
		return store.Post{}, err
	}
	p, err := h.store.Post(c.Request().Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return store.Post{}, echo.NewHTTPError(http.StatusNotFound, "post not found")
	}
	return p, err
}

func visible(p store.Post, m store.Member) bool {
	return p.State == "published" || p.AuthorID == m.ID
}

func (h *handler) createPost(c echo.Context) error {
	var in postInput
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := c.Validate(&in); err != nil {
		return err
	}
	p, err := h.store.CreatePost(c.Request().Context(), store.Post{
		Slug:     in.Slug,
		Title:    in.Title,
		Body:     in.Body,
		AuthorID: currentMember(c).ID,
		Tags:     slices.Compact(slices.Sorted(slices.Values(in.Tags))),
	})
	if errors.Is(err, store.ErrSlugTaken) {
		return echo.NewHTTPError(http.StatusConflict, "slug: already taken")
	}
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, p)
}

func (h *handler) getPost(c echo.Context) error {
	p, err := h.loadPost(c)
	if err != nil {
		return err
	}
	if !visible(p, currentMember(c)) {
		return echo.NewHTTPError(http.StatusNotFound, "post not found")
	}
	return c.JSON(http.StatusOK, p)
}

func (h *handler) editPost(c echo.Context) error {
	var in postEdit
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := c.Validate(&in); err != nil {
		return err
	}
	p, err := h.loadPost(c)
	if err != nil {
		return err
	}
	switch {
	case p.AuthorID != currentMember(c).ID:
		return echo.NewHTTPError(http.StatusForbidden, "only the author edits a post")
	case p.State != "draft":
		return echo.NewHTTPError(http.StatusConflict, "only a draft is edited")
	}
	p, err = h.store.UpdatePost(c.Request().Context(), p.ID, in.Title, in.Body)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, p)
}

func (h *handler) publishPost(c echo.Context) error {
	p, err := h.loadPost(c)
	if err != nil {
		return err
	}
	m := currentMember(c)
	switch {
	case p.AuthorID != m.ID:
		return echo.NewHTTPError(http.StatusForbidden, "only the author publishes a post")
	case p.State != "draft":
		return echo.NewHTTPError(http.StatusConflict, "only a draft is published")
	case utf8.RuneCountInString(p.Body) < publishMinBody:
		return fieldError("body", "needs at least "+strconv.Itoa(publishMinBody)+" characters to publish")
	}
	p, err = h.store.PublishPost(c.Request().Context(), p.ID, m.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return echo.NewHTTPError(http.StatusConflict, "only a draft is published")
	}
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, p)
}

func (h *handler) audit(c echo.Context) error {
	p, err := h.loadPost(c)
	if err != nil {
		return err
	}
	if p.AuthorID != currentMember(c).ID {
		return echo.NewHTTPError(http.StatusForbidden, "only the author reads the audit")
	}
	audits, err := h.store.Audits(c.Request().Context(), p.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, audits)
}

func (h *handler) feed(c echo.Context) error {
	q := store.FeedQuery{Tag: c.QueryParam("tag"), Limit: 10}
	if s := c.QueryParam("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 50 {
			return fieldError("limit", "must be a number from 1 to 50")
		}
		q.Limit = n
	}
	if s := c.QueryParam("after"); s != "" {
		at, id, err := decodeCursor(s)
		if err != nil {
			return fieldError("after", "is not a cursor of this feed")
		}
		q.AfterPublishedAt, q.AfterID = at, id
	}
	limit := q.Limit
	q.Limit++
	items, err := h.store.Feed(c.Request().Context(), q)
	if err != nil {
		return err
	}
	page := feedPage{Posts: items}
	if len(items) > limit {
		page.Posts = items[:limit]
		last := page.Posts[limit-1]
		page.Next = encodeCursor(last.PublishedAt, last.ID)
	}
	return c.JSON(http.StatusOK, page)
}

func encodeCursor(at time.Time, id int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + "|" + strconv.Itoa(id)))
}

func decodeCursor(s string) (time.Time, int, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, 0, err
	}
	atText, idText, ok := strings.Cut(string(raw), "|")
	if !ok {
		return time.Time{}, 0, errors.New("cursor without an id")
	}
	at, err := time.Parse(time.RFC3339Nano, atText)
	if err != nil {
		return time.Time{}, 0, err
	}
	id, err := strconv.Atoi(idText)
	if err != nil || id < 1 {
		return time.Time{}, 0, errors.New("cursor with a bad id")
	}
	return at.UTC(), id, nil
}
