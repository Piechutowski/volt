package server

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"entgo.io/ent/dialect/sql"

	"github.com/Piechutowski/volt/compare/oapient/api"
	"github.com/Piechutowski/volt/compare/oapient/ent"
	"github.com/Piechutowski/volt/compare/oapient/ent/audit"
	"github.com/Piechutowski/volt/compare/oapient/ent/comment"
	"github.com/Piechutowski/volt/compare/oapient/ent/post"
	"github.com/Piechutowski/volt/compare/oapient/ent/tag"
)

func postOut(ctx context.Context, p *ent.Post) (api.Post, error) {
	tags, err := p.QueryTags().Order(tag.ByName()).Select(tag.FieldName).Strings(ctx)
	if err != nil {
		return api.Post{}, err
	}
	if tags == nil {
		tags = []string{}
	}
	return api.Post{
		ID:          p.ID,
		Slug:        p.Slug,
		Title:       p.Title,
		Body:        p.Body,
		State:       api.PostState(p.State),
		AuthorID:    p.AuthorID,
		Tags:        tags,
		PublishedAt: p.PublishedAt,
		CreatedAt:   p.CreatedAt,
	}, nil
}

func visible(p *ent.Post, m *ent.Member) bool {
	return p.State == post.StatePublished || p.AuthorID == m.ID
}

func tagsEnsure(ctx context.Context, tx *ent.Tx, names []string) ([]*ent.Tag, error) {
	have, err := tx.Tag.Query().Where(tag.NameIn(names...)).All(ctx)
	if err != nil {
		return nil, err
	}
	missing := slices.DeleteFunc(slices.Clone(names), func(n string) bool {
		return slices.ContainsFunc(have, func(t *ent.Tag) bool { return t.Name == n })
	})
	created, err := tx.Tag.MapCreateBulk(missing, func(c *ent.TagCreate, i int) { c.SetName(missing[i]) }).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("tags: %w", err)
	}
	return append(have, created...), nil
}

// CreatePost stores a draft and its tags in one transaction.
func (s *Server) CreatePost(ctx context.Context, req api.CreatePostRequestObject) (api.CreatePostResponseObject, error) {
	var names []string
	if req.Body.Tags != nil {
		names = slices.Compact(slices.Sorted(slices.Values(*req.Body.Tags)))
	}
	var out api.Post
	err := withTx(ctx, s.db, func(tx *ent.Tx) error {
		tags, err := tagsEnsure(ctx, tx, names)
		if err != nil {
			return err
		}
		p, err := tx.Post.Create().
			SetSlug(req.Body.Slug).
			SetTitle(req.Body.Title).
			SetBody(req.Body.Body).
			SetAuthorID(member(ctx).ID).
			AddTags(tags...).
			Save(ctx)
		if err != nil {
			return err
		}
		out, err = postOut(ctx, p)
		return err
	})
	switch {
	case ent.IsValidationError(err):
		return api.CreatePost422JSONResponse{UnprocessableJSONResponse: api.UnprocessableJSONResponse(refusal(err))}, nil
	case ent.IsConstraintError(err):
		return api.CreatePost409JSONResponse{ConflictJSONResponse: api.ConflictJSONResponse{Message: "slug: taken"}}, nil
	case err != nil:
		return nil, err
	}
	return api.CreatePost201JSONResponse(out), nil
}

// GetPost renders a post; a draft is a 404 to all but its author.
func (s *Server) GetPost(ctx context.Context, req api.GetPostRequestObject) (api.GetPostResponseObject, error) {
	p, err := s.db.Post.Get(ctx, req.ID)
	if ent.IsNotFound(err) || err == nil && !visible(p, member(ctx)) {
		return api.GetPost404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: "no such post"}}, nil
	}
	if err != nil {
		return nil, err
	}
	out, err := postOut(ctx, p)
	if err != nil {
		return nil, err
	}
	return api.GetPost200JSONResponse(out), nil
}

// EditPost changes the title or body of the author's own draft.
func (s *Server) EditPost(ctx context.Context, req api.EditPostRequestObject) (api.EditPostResponseObject, error) {
	p, err := s.db.Post.Get(ctx, req.ID)
	switch {
	case ent.IsNotFound(err):
		return api.EditPost404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: "no such post"}}, nil
	case err != nil:
		return nil, err
	case p.AuthorID != member(ctx).ID:
		return api.EditPost403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse{Message: "only the author edits a post"}}, nil
	case p.State != post.StateDraft:
		return api.EditPost409JSONResponse{ConflictJSONResponse: api.ConflictJSONResponse{Message: "only a draft is edited"}}, nil
	}
	update := p.Update().Where(post.StateEQ(post.StateDraft))
	if req.Body.Title != nil {
		update.SetTitle(*req.Body.Title)
	}
	if req.Body.Body != nil {
		update.SetBody(*req.Body.Body)
	}
	p, err = update.Save(ctx)
	switch {
	case ent.IsValidationError(err):
		return api.EditPost422JSONResponse{UnprocessableJSONResponse: api.UnprocessableJSONResponse(refusal(err))}, nil
	case ent.IsNotFound(err):
		return api.EditPost409JSONResponse{ConflictJSONResponse: api.ConflictJSONResponse{Message: "only a draft is edited"}}, nil
	case err != nil:
		return nil, err
	}
	out, err := postOut(ctx, p)
	if err != nil {
		return nil, err
	}
	return api.EditPost200JSONResponse(out), nil
}

// PublishPost publishes the author's draft and writes its audit entry
// in one transaction.
func (s *Server) PublishPost(ctx context.Context, req api.PublishPostRequestObject) (api.PublishPostResponseObject, error) {
	m := member(ctx)
	var out api.PublishPostResponseObject
	err := withTx(ctx, s.db, func(tx *ent.Tx) error {
		p, err := tx.Post.Get(ctx, req.ID)
		switch {
		case ent.IsNotFound(err):
			out = api.PublishPost404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: "no such post"}}
			return nil
		case err != nil:
			return err
		case p.AuthorID != m.ID:
			out = api.PublishPost403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse{Message: "only the author publishes a post"}}
			return nil
		case p.State != post.StateDraft:
			out = api.PublishPost409JSONResponse{ConflictJSONResponse: api.ConflictJSONResponse{Message: "only a draft is published"}}
			return nil
		case utf8.RuneCountInString(p.Body) < 100:
			field := "body"
			out = api.PublishPost422JSONResponse{UnprocessableJSONResponse: api.UnprocessableJSONResponse{Message: "body: at least 100 characters to publish", Field: &field}}
			return nil
		}
		at := time.Now().UTC()
		p, err = p.Update().SetState(post.StatePublished).SetPublishedAt(at).Save(ctx)
		if err != nil {
			return err
		}
		if err := tx.Audit.Create().SetPostID(p.ID).SetMemberID(m.ID).SetAction("publish").SetAt(at).Exec(ctx); err != nil {
			return err
		}
		view, err := postOut(ctx, p)
		out = api.PublishPost200JSONResponse(view)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetAudit lists a post's audit entries, oldest first, to its author.
func (s *Server) GetAudit(ctx context.Context, req api.GetAuditRequestObject) (api.GetAuditResponseObject, error) {
	p, err := s.db.Post.Get(ctx, req.ID)
	switch {
	case ent.IsNotFound(err):
		return api.GetAudit404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: "no such post"}}, nil
	case err != nil:
		return nil, err
	case p.AuthorID != member(ctx).ID:
		return api.GetAudit403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse{Message: "only the author reads the audit"}}, nil
	}
	trail, err := p.QueryAudits().Order(audit.ByAt(), audit.ByID()).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make(api.GetAudit200JSONResponse, len(trail))
	for i, a := range trail {
		out[i] = api.Audit{Action: a.Action, MemberID: a.MemberID, At: a.At}
	}
	return out, nil
}

func cursorOf(p *ent.Post) string {
	return base64.RawURLEncoding.EncodeToString(fmt.Appendf(nil, "%d.%d", p.PublishedAt.UnixNano(), p.ID))
}

func cursorParse(s string) (time.Time, int, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, 0, err
	}
	nanos, id, ok := strings.Cut(string(raw), ".")
	if !ok {
		return time.Time{}, 0, errors.New("no separator")
	}
	n, err := strconv.ParseInt(nanos, 10, 64)
	if err != nil {
		return time.Time{}, 0, err
	}
	i, err := strconv.Atoi(id)
	if err != nil {
		return time.Time{}, 0, err
	}
	return time.Unix(0, n).UTC(), i, nil
}

// GetFeed pages the published posts newest first, by published_at then
// id, after a keyset cursor and optionally with a tag.
func (s *Server) GetFeed(ctx context.Context, req api.GetFeedRequestObject) (api.GetFeedResponseObject, error) {
	limit := 10
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	query := s.db.Post.Query().Where(post.StateEQ(post.StatePublished))
	if t := req.Params.Tag; t != nil && *t != "" {
		query.Where(post.HasTagsWith(tag.Name(*t)))
	}
	if after := req.Params.After; after != nil && *after != "" {
		at, id, err := cursorParse(*after)
		if err != nil {
			field := "after"
			return api.GetFeed422JSONResponse{UnprocessableJSONResponse: api.UnprocessableJSONResponse{Message: "after: not a cursor of this feed", Field: &field}}, nil
		}
		query.Where(post.Or(post.PublishedAtLT(at), post.And(post.PublishedAt(at), post.IDLT(id))))
	}
	posts, err := query.
		WithAuthor().
		Order(post.ByPublishedAt(sql.OrderDesc()), post.ByID(sql.OrderDesc())).
		Limit(limit + 1).
		All(ctx)
	if err != nil {
		return nil, err
	}
	next := ""
	if len(posts) > limit {
		posts = posts[:limit]
		next = cursorOf(posts[limit-1])
	}
	ids := make([]int, len(posts))
	for i, p := range posts {
		ids[i] = p.ID
	}
	var counts []struct {
		PostID int `json:"post_id"`
		Count  int `json:"count"`
	}
	err = s.db.Comment.Query().
		Where(comment.PostIDIn(ids...)).
		GroupBy(comment.FieldPostID).
		Aggregate(ent.Count()).
		Scan(ctx, &counts)
	if err != nil {
		return nil, err
	}
	commented := make(map[int]int, len(counts))
	for _, c := range counts {
		commented[c.PostID] = c.Count
	}
	out := api.GetFeed200JSONResponse{Posts: make([]api.FeedItem, len(posts)), Next: next}
	for i, p := range posts {
		out.Posts[i] = api.FeedItem{
			ID:           p.ID,
			Slug:         p.Slug,
			Title:        p.Title,
			AuthorName:   p.Edges.Author.Name,
			PublishedAt:  *p.PublishedAt,
			CommentCount: commented[p.ID],
		}
	}
	return out, nil
}
