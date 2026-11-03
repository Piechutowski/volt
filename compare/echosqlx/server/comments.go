package server

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/Piechutowski/volt/compare/echosqlx/store"
)

type commentInput struct {
	Body string `json:"body" validate:"required,max=1000"`
}

const commentsPerHour = 3

func (h *handler) createComment(c echo.Context) error {
	var in commentInput
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
	if p.State != "published" {
		return echo.NewHTTPError(http.StatusConflict, "only a published post takes comments")
	}
	ctx := c.Request().Context()
	m := currentMember(c)
	n, err := h.store.CountComments(ctx, p.ID, m.ID, time.Now().Add(-time.Hour))
	if err != nil {
		return err
	}
	if n >= commentsPerHour {
		return echo.NewHTTPError(http.StatusTooManyRequests, "at most 3 comments on a post in an hour")
	}
	cm, err := h.store.CreateComment(ctx, store.Comment{PostID: p.ID, MemberID: m.ID, Body: in.Body})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, cm)
}

func (h *handler) listComments(c echo.Context) error {
	p, err := h.loadPost(c)
	if err != nil {
		return err
	}
	if !visible(p, currentMember(c)) {
		return echo.NewHTTPError(http.StatusNotFound, "post not found")
	}
	comments, err := h.store.Comments(c.Request().Context(), p.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, comments)
}

func (h *handler) deleteComment(c echo.Context) error {
	p, err := h.loadPost(c)
	if err != nil {
		return err
	}
	cid, err := pathID(c, "cid")
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	cm, err := h.store.PostComment(ctx, p.ID, cid)
	if errors.Is(err, sql.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "comment not found")
	}
	if err != nil {
		return err
	}
	if m := currentMember(c); m.ID != cm.MemberID && m.ID != p.AuthorID {
		return echo.NewHTTPError(http.StatusForbidden, "only the comment's or the post's author deletes it")
	}
	if err := h.store.DeleteComment(ctx, cm.ID); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *handler) stats(c echo.Context) error {
	st, err := h.store.Stats(c.Request().Context(), currentMember(c).ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, st)
}
