package chisqlc

import (
	"context"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/compare/chisqlc/api"
	"github.com/Piechutowski/volt/compare/chisqlc/client"
	"github.com/Piechutowski/volt/compare/suite"
)

// TestContract runs the comparison's contract against the chi and sqlc
// blog.
func TestContract(t *testing.T) {
	h, err := Handler(filepath.Join(t.TempDir(), "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	suite.Run(t, srv.URL)
}

// TestClient drives the typed client through a post's life: create,
// a refused publish, edit, publish, comment, feed and stats.
func TestClient(t *testing.T) {
	h, err := Handler(filepath.Join(t.TempDir(), "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	ctx := context.Background()
	ada, grace := client.New(srv.URL, 1), client.New(srv.URL, 2)
	p, err := ada.CreatePost(ctx, api.NewPost{Slug: "hello", Title: "Hello", Body: "Short.", Tags: []string{"go"}})
	if err != nil || p.State != "draft" {
		t.Fatalf("create = %+v, %v", p, err)
	}
	var refusal *client.Error
	if _, err := ada.Publish(ctx, p.ID); !errors.As(err, &refusal) || refusal.Status != 422 || refusal.Fields["body"] == "" {
		t.Errorf("publishing a short body = %v", err)
	}
	long := strings.Repeat("x", 120)
	if _, err := ada.EditPost(ctx, p.ID, api.PostEdit{Body: &long}); err != nil {
		t.Fatal(err)
	}
	if p, err = ada.Publish(ctx, p.ID); err != nil || p.State != "published" || p.PublishedAt == nil {
		t.Fatalf("publish = %+v, %v", p, err)
	}
	if _, err := grace.Comment(ctx, p.ID, "Nice"); err != nil {
		t.Fatal(err)
	}
	f, err := grace.Feed(ctx, client.FeedQuery{Tag: "go"})
	if err != nil || len(f.Posts) != 1 || f.Posts[0].CommentCount != 1 || f.Next != "" {
		t.Errorf("feed = %+v, %v", f, err)
	}
	if s, err := ada.Stats(ctx); err != nil || s != (api.Stats{Published: 1, CommentsReceived: 1}) {
		t.Errorf("stats = %+v, %v", s, err)
	}
}
