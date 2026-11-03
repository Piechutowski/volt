package gingorm

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Piechutowski/volt/compare/gingorm/api"
	"github.com/Piechutowski/volt/compare/gingorm/client"
	"github.com/Piechutowski/volt/compare/suite"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	h, err := Handler(filepath.Join(t.TempDir(), "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

// TestContract runs the comparison's contract against the Gin + GORM blog.
func TestContract(t *testing.T) {
	suite.Run(t, newServer(t).URL)
}

// TestClient drives the typed client through a post's life: create,
// a refused publish, edit, publish, comment, feed and stats.
func TestClient(t *testing.T) {
	srv := newServer(t)
	ctx := context.Background()
	ada, grace := client.New(srv.URL, 1), client.New(srv.URL, 2)
	p, err := ada.CreatePost(ctx, api.CreatePostRequest{Slug: "hello", Title: "Hello", Body: "Short.", Tags: []string{"go"}})
	if err != nil || p.State != "draft" {
		t.Fatalf("create = %+v, %v", p, err)
	}
	var refusal *client.Error
	if _, err := ada.PublishPost(ctx, p.ID); !errors.As(err, &refusal) || refusal.Status != 422 {
		t.Errorf("publishing a short body = %v", err)
	}
	long := strings.Repeat("x", 120)
	if _, err := ada.UpdatePost(ctx, p.ID, api.UpdatePostRequest{Body: &long}); err != nil {
		t.Fatal(err)
	}
	if p, err = ada.PublishPost(ctx, p.ID); err != nil || p.State != "published" {
		t.Fatalf("publish = %+v, %v", p, err)
	}
	if _, err := grace.CreateComment(ctx, p.ID, "Nice"); err != nil {
		t.Fatal(err)
	}
	f, err := grace.Feed(ctx, client.FeedParams{Tag: "go"})
	if err != nil || len(f.Posts) != 1 || f.Posts[0].CommentCount != 1 {
		t.Errorf("feed = %+v, %v", f, err)
	}
	if s, err := ada.Stats(ctx); err != nil || s != (api.Stats{Published: 1, CommentsReceived: 1}) {
		t.Errorf("stats = %+v, %v", s, err)
	}
}
