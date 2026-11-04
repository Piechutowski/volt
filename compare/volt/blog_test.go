package blog

import (
	"context"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/compare/suite"
	"github.com/Piechutowski/volt/compare/volt/app"
	"github.com/Piechutowski/volt/compare/volt/blogclient"
	"github.com/Piechutowski/volt/compare/volt/db"
	"github.com/Piechutowski/volt/nao/rt"
	"github.com/Piechutowski/volt/nao/run"
)

// TestContract runs the comparison's contract against the Volt blog.
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
	ada, grace := blogclient.New(srv.URL, 1), blogclient.New(srv.URL, 2)
	p, err := ada.Create(ctx, app.PostForm{Slug: "hello", Title: "Hello", Body: "Short.", Tags: []string{"go"}})
	if err != nil || p.State != "draft" {
		t.Fatalf("create = %+v, %v", p, err)
	}
	var pe *volt.ProblemError
	if _, err := ada.Publish(ctx, p.ID); !errors.As(err, &pe) || pe.Status != 422 {
		t.Errorf("publishing a short body = %v", err)
	}
	long := strings.Repeat("x", 120)
	if _, err := ada.PostEdit(ctx, p.ID, app.PostEdit{Body: &long}); err != nil {
		t.Fatal(err)
	}
	if p, err = ada.Publish(ctx, p.ID); err != nil || p.State != "published" {
		t.Fatalf("publish = %+v, %v", p, err)
	}
	if _, err := grace.CommentCreate(ctx, p.ID, "Nice"); err != nil {
		t.Fatal(err)
	}
	f, err := grace.Feed(ctx, "go", "", 0)
	if err != nil || len(f.Posts) != 1 || f.Posts[0].CommentCount != 1 {
		t.Errorf("feed = %+v, %v", f, err)
	}
	if s, err := ada.Stats(ctx); err != nil || s != (app.Stats{Published: 1, CommentsReceived: 1}) {
		t.Errorf("stats = %+v, %v", s, err)
	}
}

// TestCreateIsAtomic proves a post and its tags are written in one
// transaction (spec §generated-query-surface): a trigger the test adds
// refuses the second of two tags once the post and the first are
// inserted, and the refusal leaves neither, so the slug is free again.
func TestCreateIsAtomic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blog.db")
	h, err := Handler(path)
	if err != nil {
		t.Fatal(err)
	}
	d, err := run.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	refuse := rt.Exec{SQL: "CREATE TRIGGER post_tags_refused BEFORE INSERT ON post_tags WHEN NEW.tag = 'refused' BEGIN SELECT RAISE(ABORT, 'the tag is refused'); END"}
	if _, err := run.Run(ctx, d, refuse); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	ada := blogclient.New(srv.URL, 1)
	form := app.PostForm{Slug: "half-written", Title: "Half written", Body: "x", Tags: []string{"refused", "go"}}
	var pe *volt.ProblemError
	if p, err := ada.Create(ctx, form); !errors.As(err, &pe) || pe.Status != 500 {
		t.Fatalf("a create whose second tag is refused = %+v, %v; want the 500 of an unmapped refusal", p, err)
	}
	counts, err := run.Run(ctx, d, rt.Tx(db.PostCount(), db.PostTagCount()))
	if err != nil {
		t.Fatal(err)
	}
	if counts[0] != int64(0) || counts[1] != int64(0) {
		t.Errorf("the refused create left %v posts and %v tags, want none", counts[0], counts[1])
	}
	form.Tags = []string{"go"}
	if p, err := ada.Create(ctx, form); err != nil || p.Slug != form.Slug {
		t.Errorf("the create again without the refused tag = %+v, %v", p, err)
	}
}
