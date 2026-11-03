package server

import (
	"context"
	"time"

	"github.com/Piechutowski/volt/compare/oapient/api"
	"github.com/Piechutowski/volt/compare/oapient/ent"
	"github.com/Piechutowski/volt/compare/oapient/ent/comment"
	"github.com/Piechutowski/volt/compare/oapient/ent/post"
)

func commentOut(c *ent.Comment) api.Comment {
	return api.Comment{ID: c.ID, PostID: c.PostID, MemberID: c.MemberID, Body: c.Body, CreatedAt: c.CreatedAt}
}

// CreateComment stores a comment on a published post, at most three per
// member and post in an hour; the count and the insert share a
// transaction.
func (s *Server) CreateComment(ctx context.Context, req api.CreateCommentRequestObject) (api.CreateCommentResponseObject, error) {
	m := member(ctx)
	var out api.CreateCommentResponseObject
	err := withTx(ctx, s.db, func(tx *ent.Tx) error {
		p, err := tx.Post.Get(ctx, req.ID)
		switch {
		case ent.IsNotFound(err):
			out = api.CreateComment404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: "no such post"}}
			return nil
		case err != nil:
			return err
		case p.State != post.StatePublished:
			out = api.CreateComment409JSONResponse{ConflictJSONResponse: api.ConflictJSONResponse{Message: "only a published post takes comments"}}
			return nil
		}
		recent, err := p.QueryComments().
			Where(comment.MemberID(m.ID), comment.CreatedAtGT(time.Now().UTC().Add(-time.Hour))).
			Count(ctx)
		if err != nil {
			return err
		}
		if recent >= 3 {
			out = api.CreateComment429JSONResponse{TooManyRequestsJSONResponse: api.TooManyRequestsJSONResponse{Message: "at most 3 comments on a post in an hour"}}
			return nil
		}
		c, err := tx.Comment.Create().SetPost(p).SetMember(m).SetBody(req.Body.Body).Save(ctx)
		if ent.IsValidationError(err) {
			out = api.CreateComment422JSONResponse{UnprocessableJSONResponse: api.UnprocessableJSONResponse(refusal(err))}
			return nil
		}
		if err != nil {
			return err
		}
		out = api.CreateComment201JSONResponse(commentOut(c))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListComments lists a post's comments, oldest first.
func (s *Server) ListComments(ctx context.Context, req api.ListCommentsRequestObject) (api.ListCommentsResponseObject, error) {
	p, err := s.db.Post.Get(ctx, req.ID)
	if ent.IsNotFound(err) || err == nil && !visible(p, member(ctx)) {
		return api.ListComments404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: "no such post"}}, nil
	}
	if err != nil {
		return nil, err
	}
	thread, err := p.QueryComments().Order(comment.ByCreatedAt(), comment.ByID()).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make(api.ListComments200JSONResponse, len(thread))
	for i, c := range thread {
		out[i] = commentOut(c)
	}
	return out, nil
}

// DeleteComment removes a comment; only its author or the post's may.
func (s *Server) DeleteComment(ctx context.Context, req api.DeleteCommentRequestObject) (api.DeleteCommentResponseObject, error) {
	m := member(ctx)
	c, err := s.db.Comment.Query().Where(comment.ID(req.Cid), comment.PostID(req.ID)).WithPost().Only(ctx)
	switch {
	case ent.IsNotFound(err):
		return api.DeleteComment404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: "no such comment"}}, nil
	case err != nil:
		return nil, err
	case c.MemberID != m.ID && c.Edges.Post.AuthorID != m.ID:
		return api.DeleteComment403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse{Message: "only the comment's or the post's author deletes it"}}, nil
	}
	if err := s.db.Comment.DeleteOne(c).Exec(ctx); err != nil {
		return nil, err
	}
	return api.DeleteComment204Response{}, nil
}
