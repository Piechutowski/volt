package oapient

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/securityprovider"

	"github.com/Piechutowski/volt/compare/oapient/api"
	"github.com/Piechutowski/volt/compare/suite"
)

// TestContract runs the comparison's contract against the oapi-codegen
// and Ent blog.
func TestContract(t *testing.T) {
	h, err := Handler(filepath.Join(t.TempDir(), "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	suite.Run(t, srv.URL)
}

func memberClient(t *testing.T, base string, id int) *api.ClientWithResponses {
	t.Helper()
	key, err := securityprovider.NewSecurityProviderApiKey("header", "X-User", strconv.Itoa(id))
	if err != nil {
		t.Fatal(err)
	}
	c, err := api.NewClientWithResponses(base, api.WithRequestEditorFn(key.Intercept))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestClient drives the generated client through a post's life: create,
// a refused publish, edit, publish, comment, feed and stats.
func TestClient(t *testing.T) {
	h, err := Handler(filepath.Join(t.TempDir(), "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	ctx := context.Background()
	ada, grace := memberClient(t, srv.URL, 1), memberClient(t, srv.URL, 2)

	tags := []string{"go"}
	created, err := ada.CreatePostWithResponse(ctx, api.NewPost{Slug: "hello", Title: "Hello", Body: "Short.", Tags: &tags})
	if err != nil || created.JSON201 == nil || created.JSON201.State != api.Draft {
		t.Fatalf("create = %+v, %v", created, err)
	}
	id := created.JSON201.ID
	refused, err := ada.PublishPostWithResponse(ctx, id)
	if err != nil || refused.JSON422 == nil || !strings.Contains(refused.JSON422.Message, "body") {
		t.Errorf("publishing a short body = %+v, %v", refused, err)
	}
	long := strings.Repeat("x", 120)
	if edited, err := ada.EditPostWithResponse(ctx, id, api.PostEdit{Body: &long}); err != nil || edited.JSON200 == nil {
		t.Fatalf("edit = %+v, %v", edited, err)
	}
	published, err := ada.PublishPostWithResponse(ctx, id)
	if err != nil || published.JSON200 == nil || published.JSON200.State != api.Published || published.JSON200.PublishedAt == nil {
		t.Fatalf("publish = %+v, %v", published, err)
	}
	if c, err := grace.CreateCommentWithResponse(ctx, id, api.NewComment{Body: "Nice"}); err != nil || c.JSON201 == nil {
		t.Fatalf("comment = %+v, %v", c, err)
	}
	tag := "go"
	feed, err := grace.GetFeedWithResponse(ctx, &api.GetFeedParams{Tag: &tag})
	if err != nil || feed.JSON200 == nil || len(feed.JSON200.Posts) != 1 || feed.JSON200.Posts[0].CommentCount != 1 || feed.JSON200.Next != "" {
		t.Errorf("feed = %+v, %v", feed, err)
	}
	stats, err := ada.GetStatsWithResponse(ctx)
	if err != nil || stats.JSON200 == nil || *stats.JSON200 != (api.Stats{Published: 1, CommentsReceived: 1}) {
		t.Errorf("stats = %+v, %v", stats, err)
	}
}
