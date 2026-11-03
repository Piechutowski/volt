package server

import (
	"context"

	"github.com/Piechutowski/volt/compare/oapient/api"
	"github.com/Piechutowski/volt/compare/oapient/ent"
	"github.com/Piechutowski/volt/compare/oapient/ent/comment"
	"github.com/Piechutowski/volt/compare/oapient/ent/post"
)

// GetStats counts the member's drafts, published posts and the comments
// on them.
func (s *Server) GetStats(ctx context.Context, _ api.GetStatsRequestObject) (api.GetStatsResponseObject, error) {
	m := member(ctx)
	var states []struct {
		State post.State `json:"state"`
		Count int        `json:"count"`
	}
	err := s.db.Post.Query().
		Where(post.AuthorID(m.ID)).
		GroupBy(post.FieldState).
		Aggregate(ent.Count()).
		Scan(ctx, &states)
	if err != nil {
		return nil, err
	}
	received, err := s.db.Comment.Query().Where(comment.HasPostWith(post.AuthorID(m.ID))).Count(ctx)
	if err != nil {
		return nil, err
	}
	out := api.GetStats200JSONResponse{CommentsReceived: received}
	for _, row := range states {
		switch row.State {
		case post.StateDraft:
			out.Drafts = row.Count
		case post.StatePublished:
			out.Published = row.Count
		}
	}
	return out, nil
}
