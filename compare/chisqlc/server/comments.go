package server

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Piechutowski/volt/compare/chisqlc/api"
	"github.com/Piechutowski/volt/compare/chisqlc/db"
)

const commentsPerHour = 3

func toComment(c db.Comment) api.Comment {
	return api.Comment{ID: c.ID, PostID: c.PostID, MemberID: c.MemberID, Body: c.Body, CreatedAt: c.CreatedAt.UTC()}
}

func (s *server) createComment(w http.ResponseWriter, r *http.Request) {
	p, m := postFrom(r), memberFrom(r)
	if p.State != "published" {
		writeError(w, http.StatusConflict, "only a published post takes comments")
		return
	}
	var in api.NewComment
	if !decodeJSON(w, r, &in) {
		return
	}
	if errs := validateNewComment(in); len(errs) > 0 {
		writeInvalid(w, errs)
		return
	}

	now := time.Now().UTC()
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback()
	qtx := s.q.WithTx(tx)
	recent, err := qtx.CountCommentsSince(r.Context(), db.CountCommentsSinceParams{PostID: p.ID, MemberID: m.ID, CreatedAt: now.Add(-time.Hour)})
	if err != nil {
		serverError(w, err)
		return
	}
	if recent >= commentsPerHour {
		writeError(w, http.StatusTooManyRequests, "at most 3 comments on a post in an hour")
		return
	}
	c, err := qtx.CreateComment(r.Context(), db.CreateCommentParams{PostID: p.ID, MemberID: m.ID, Body: in.Body, CreatedAt: now})
	if err != nil {
		serverError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toComment(c))
}

func (s *server) listComments(w http.ResponseWriter, r *http.Request) {
	p := postFrom(r)
	if p.State != "published" && p.AuthorID != memberFrom(r).ID {
		writeError(w, http.StatusNotFound, "post not found")
		return
	}
	rows, err := s.q.ListComments(r.Context(), p.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	out := make([]api.Comment, len(rows))
	for i, c := range rows {
		out[i] = toComment(c)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) deleteComment(w http.ResponseWriter, r *http.Request) {
	p, m := postFrom(r), memberFrom(r)
	cid, err := strconv.ParseInt(chi.URLParam(r, "cid"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "comment not found")
		return
	}
	c, err := s.q.GetComment(r.Context(), db.GetCommentParams{ID: cid, PostID: p.ID})
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "comment not found")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if m.ID != c.MemberID && m.ID != p.AuthorID {
		writeError(w, http.StatusForbidden, "only the comment's or the post's author deletes it")
		return
	}
	if err := s.q.DeleteComment(r.Context(), c.ID); err != nil {
		serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
