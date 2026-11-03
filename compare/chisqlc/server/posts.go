package server

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Piechutowski/volt/compare/chisqlc/api"
	"github.com/Piechutowski/volt/compare/chisqlc/db"
)

const (
	defaultFeedLimit = 10
	maxFeedLimit     = 50
)

func toPost(p db.Post, tags []string) api.Post {
	out := api.Post{
		ID:        p.ID,
		Slug:      p.Slug,
		Title:     p.Title,
		Body:      p.Body,
		State:     p.State,
		AuthorID:  p.AuthorID,
		Tags:      tags,
		CreatedAt: p.CreatedAt.UTC(),
	}
	if out.Tags == nil {
		out.Tags = []string{}
	}
	if p.PublishedAt != nil {
		at := p.PublishedAt.UTC()
		out.PublishedAt = &at
	}
	return out
}

func (s *server) writePost(w http.ResponseWriter, r *http.Request, status int, p db.Post) {
	tags, err := s.q.ListPostTags(r.Context(), p.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, status, toPost(p, tags))
}

func (s *server) createPost(w http.ResponseWriter, r *http.Request) {
	var in api.NewPost
	if !decodeJSON(w, r, &in) {
		return
	}
	if errs := validateNewPost(in); len(errs) > 0 {
		writeInvalid(w, errs)
		return
	}
	tags := slices.Compact(slices.Sorted(slices.Values(in.Tags)))

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback()
	qtx := s.q.WithTx(tx)
	p, err := qtx.CreatePost(r.Context(), db.CreatePostParams{
		Slug:      in.Slug,
		Title:     in.Title,
		Body:      in.Body,
		AuthorID:  memberFrom(r).ID,
		CreatedAt: time.Now().UTC(),
	})
	if isUniqueViolation(err) {
		writeError(w, http.StatusConflict, "slug is taken")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	for _, tag := range tags {
		if err := qtx.AddPostTag(r.Context(), db.AddPostTagParams{PostID: p.ID, Tag: tag}); err != nil {
			serverError(w, err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toPost(p, tags))
}

func (s *server) getPost(w http.ResponseWriter, r *http.Request) {
	p := postFrom(r)
	if p.State != "published" && p.AuthorID != memberFrom(r).ID {
		writeError(w, http.StatusNotFound, "post not found")
		return
	}
	s.writePost(w, r, http.StatusOK, p)
}

func (s *server) editPost(w http.ResponseWriter, r *http.Request) {
	p := postFrom(r)
	if p.AuthorID != memberFrom(r).ID {
		writeError(w, http.StatusForbidden, "only the author edits a post")
		return
	}
	if p.State != "draft" {
		writeError(w, http.StatusConflict, "only a draft can be edited")
		return
	}
	var in api.PostEdit
	if !decodeJSON(w, r, &in) {
		return
	}
	if errs := validatePostEdit(in); len(errs) > 0 {
		writeInvalid(w, errs)
		return
	}
	p, err := s.q.UpdateDraft(r.Context(), db.UpdateDraftParams{ID: p.ID, Title: in.Title, Body: in.Body})
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusConflict, "only a draft can be edited")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	s.writePost(w, r, http.StatusOK, p)
}

func (s *server) publishPost(w http.ResponseWriter, r *http.Request) {
	p, m := postFrom(r), memberFrom(r)
	if p.AuthorID != m.ID {
		writeError(w, http.StatusForbidden, "only the author publishes a post")
		return
	}
	if p.State != "draft" {
		writeError(w, http.StatusConflict, "only a draft can be published")
		return
	}
	if utf8.RuneCountInString(p.Body) < minPublishedBody {
		writeInvalid(w, map[string]string{"body": fmt.Sprintf("must be at least %d characters to publish", minPublishedBody)})
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
	p, err = qtx.PublishPost(r.Context(), db.PublishPostParams{ID: p.ID, PublishedAt: &now})
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusConflict, "only a draft can be published")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if err := qtx.CreateAudit(r.Context(), db.CreateAuditParams{PostID: p.ID, MemberID: m.ID, Action: "publish", At: now}); err != nil {
		serverError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		serverError(w, err)
		return
	}
	s.writePost(w, r, http.StatusOK, p)
}

func (s *server) listAudits(w http.ResponseWriter, r *http.Request) {
	p := postFrom(r)
	if p.AuthorID != memberFrom(r).ID {
		writeError(w, http.StatusForbidden, "only the author reads the audit")
		return
	}
	rows, err := s.q.ListAudits(r.Context(), p.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	out := make([]api.Audit, len(rows))
	for i, a := range rows {
		out[i] = api.Audit{Action: a.Action, MemberID: a.MemberID, At: a.At.UTC()}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) feed(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit := defaultFeedLimit
	if v := query.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxFeedLimit {
			writeInvalid(w, map[string]string{"limit": fmt.Sprintf("must be a number from 1 to %d", maxFeedLimit)})
			return
		}
		limit = n
	}
	params := db.ListFeedParams{Limit: int64(limit + 1)}
	if tag := query.Get("tag"); tag != "" {
		params.Tag = &tag
	}
	if v := query.Get("after"); v != "" {
		at, id, err := decodeCursor(v)
		if err != nil {
			writeInvalid(w, map[string]string{"after": "is not a cursor of this feed"})
			return
		}
		params.BeforeAt, params.BeforeID = &at, &id
	}

	rows, err := s.q.ListFeed(r.Context(), params)
	if err != nil {
		serverError(w, err)
		return
	}
	page := api.Feed{Posts: make([]api.FeedItem, 0, len(rows))}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[limit-1]
		page.Next = encodeCursor(*last.PublishedAt, last.ID)
	}
	for _, row := range rows {
		page.Posts = append(page.Posts, api.FeedItem{
			ID:           row.ID,
			Slug:         row.Slug,
			Title:        row.Title,
			AuthorName:   row.AuthorName,
			PublishedAt:  row.PublishedAt.UTC(),
			CommentCount: row.CommentCount,
		})
	}
	writeJSON(w, http.StatusOK, page)
}

func encodeCursor(at time.Time, id int64) string {
	return base64.RawURLEncoding.EncodeToString(fmt.Appendf(nil, "%d:%d", at.UnixNano(), id))
}

func decodeCursor(s string) (time.Time, int64, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, 0, err
	}
	nanos, id, ok := strings.Cut(string(raw), ":")
	if !ok {
		return time.Time{}, 0, errors.New("cursor has no separator")
	}
	n, err := strconv.ParseInt(nanos, 10, 64)
	if err != nil {
		return time.Time{}, 0, err
	}
	i, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return time.Time{}, 0, err
	}
	return time.Unix(0, n).UTC(), i, nil
}
