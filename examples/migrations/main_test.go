package main

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Piechutowski/volt/examples/migrations/app/client"
	"github.com/Piechutowski/volt/examples/migrations/db"
	"github.com/Piechutowski/volt/nao/run"
)

// TestMigrationsMoveTheSchema walks the database the window drives: no
// table before Up, the views column 0002 adds after it, the column gone
// after one Down, and the ledger saying so at each step.
func TestMigrationsMoveTheSchema(t *testing.T) {
	d, base, err := serve(filepath.Join(t.TempDir(), "migrations.db"))
	if err != nil {
		t.Fatal(err)
	}
	c, ctx := client.New(base), context.Background()
	if _, err := c.Posts(ctx); err == nil {
		t.Error("posts read before any migration")
	}
	applied, err := run.MigrateUp(ctx, d, db.Migrations)
	if err != nil || !slices.Equal(applied, []string{"0001__create_posts", "0002__posts_views"}) {
		t.Fatalf("up = %v, %v", applied, err)
	}
	p, err := c.CreatePost(ctx, db.PostNew().TitleSet("Hello"))
	if err != nil || p.Views != 0 {
		t.Fatalf("create = %+v, %v", p, err)
	}
	if name, err := run.MigrateDown(ctx, d, db.Migrations); err != nil || name != "0002__posts_views" {
		t.Fatalf("down = %q, %v", name, err)
	}
	if _, err := c.Posts(ctx); err == nil {
		t.Error("the generated query read posts without the views column")
	}
	status, err := run.MigrateStatus(ctx, d, db.Migrations)
	if err != nil || len(status) != 2 || !status[0].Applied || status[1].Applied {
		t.Errorf("status = %+v, %v", status, err)
	}
	if applied, err := run.MigrateUp(ctx, d, db.Migrations); err != nil || !slices.Equal(applied, []string{"0002__posts_views"}) {
		t.Errorf("up again = %v, %v", applied, err)
	}
	if posts, err := c.Posts(ctx); err != nil || len(posts) != 1 || posts[0].Title != "Hello" {
		t.Errorf("posts after up = %+v, %v", posts, err)
	}
}
