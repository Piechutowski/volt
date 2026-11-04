package blog

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync"
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
	p, err := ada.PostCreate(ctx, app.PostForm{Slug: "hello", Title: "Hello", Body: "Short.", Tags: []string{"go"}})
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
	if p, err := ada.PostCreate(ctx, form); !errors.As(err, &pe) || pe.Status != 500 {
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
	if p, err := ada.PostCreate(ctx, form); err != nil || p.Slug != form.Slug {
		t.Errorf("the create again without the refused tag = %+v, %v", p, err)
	}
}

// TestRulesHoldConcurrently proves that Publish, Edit and the comment
// limit read what they check in the transaction that writes (spec
// §generated-query-surface): twenty publishes of one draft at once
// publish it once with one audit entry, ten comments at once by one
// member store three, every other call refused with 409 or 429, and
// sixteen edits to a short body racing four publishes of a draft never
// leave a published post under 100 characters. A check made before the
// transaction loses a round only when the requests interleave so, about
// one round in four for publishing and one in fifteen for editing, so the
// test runs forty and a hundred and fifty rounds, which such a check
// passes about once in thirty thousand runs.
func TestRulesHoldConcurrently(t *testing.T) {
	h, err := Handler(filepath.Join(t.TempDir(), "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	ctx := context.Background()
	ada, grace := blogclient.New(srv.URL, 1), blogclient.New(srv.URL, 2)
	long, short := strings.Repeat("x", 120), "short"
	for round := range 40 {
		p, err := ada.PostCreate(ctx, app.PostForm{Slug: fmt.Sprintf("race-%d", round), Title: "Race", Body: long})
		if err != nil {
			t.Fatal(err)
		}
		published := concurrently(t, 20, func(int) error { _, err := ada.Publish(ctx, p.ID); return err }, 409)
		stored := concurrently(t, 10, func(int) error { _, err := grace.CommentCreate(ctx, p.ID, "Nice"); return err }, 429)
		trail, err := ada.Audit(ctx, p.ID)
		if err != nil {
			t.Fatal(err)
		}
		if published != 1 || len(trail) != 1 || stored != 3 {
			t.Errorf("round %d: %d publishes succeeded with %d audit entries and %d comments were stored, want 1, 1 and 3", round, published, len(trail), stored)
		}
	}
	for round := range 150 {
		q, err := ada.PostCreate(ctx, app.PostForm{Slug: fmt.Sprintf("edit-race-%d", round), Title: "Edit race", Body: long})
		if err != nil {
			t.Fatal(err)
		}
		concurrently(t, 20, func(i int) error {
			if i%5 == 4 {
				_, err := ada.Publish(ctx, q.ID)
				return err
			}
			_, err := ada.PostEdit(ctx, q.ID, app.PostEdit{Body: &short})
			return err
		}, 409, 422)
		after, err := ada.Post(ctx, q.ID)
		if err != nil {
			t.Fatal(err)
		}
		if after.State == "published" && len(after.Body) < 100 {
			t.Errorf("round %d: an edit racing a publish left a published post with the body %q", round, after.Body)
		}
	}
}

func concurrently(t *testing.T, n int, call func(i int) error, refusals ...int) int {
	t.Helper()
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { errs[i] = call(i) })
	}
	wg.Wait()
	succeeded := 0
	for _, err := range errs {
		var pe *volt.ProblemError
		switch {
		case err == nil:
			succeeded++
		case !errors.As(err, &pe) || !slices.Contains(refusals, pe.Status):
			t.Errorf("a concurrent call = %v, want success or one of %v", err, refusals)
		}
	}
	return succeeded
}
