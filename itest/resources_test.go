// resources [default] (spec §V5.5): the action table expanded into
// query routes over the table's CRUD — no controller — reached through
// the generated client in both wire formats, with §V4.8's statuses.
package itest

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/itest/blog/app"
	"github.com/Piechutowski/volt/itest/blog/app/client"
	"github.com/Piechutowski/volt/itest/blog/db"
)

func TestResourcesDefault(t *testing.T) {
	for _, f := range []volt.Format{volt.FormatJSON, volt.FormatGOB} {
		srv := httptest.NewServer(handlerWithDB(t))
		c := client.New(srv.URL)
		c.Format = f
		ctx := context.Background()

		made, err := c.RefCreateUser(ctx, db.UserCreateParams{Email: "ref@example.com"})
		if err != nil || made.ID == 0 {
			t.Fatalf("%v: create = %+v, %v", f, made, err)
		}
		all, err := c.RefUsers(ctx)
		if err != nil || len(all) != 2 {
			t.Fatalf("%v: index = %+v, %v", f, all, err)
		}
		one, err := c.RefUser(ctx, made.ID)
		if err != nil || one.Email != "ref@example.com" {
			t.Fatalf("%v: show = %+v, %v", f, one, err)
		}
		up, err := c.RefUpdateUser(ctx, made.ID, db.UserUpdateParams{Email: "fer@example.com"})
		if err != nil || up.Email != "fer@example.com" {
			t.Fatalf("%v: update = %+v, %v", f, up, err)
		}
		// A [required] column is validated before the query (§V12.6).
		if _, err := c.RefUpdateUser(ctx, made.ID, db.UserUpdateParams{}); !errors.Is(err, volt.Error(422, "")) {
			t.Fatalf("%v: empty email = %v, want 422", f, err)
		}
		if err := c.RefDeleteUser(ctx, made.ID); err != nil {
			t.Fatalf("%v: delete = %v", f, err)
		}
		if _, err := c.RefUser(ctx, made.ID); !errors.Is(err, volt.ErrNotFound) {
			t.Fatalf("%v: after delete = %v, want volt.ErrNotFound", f, err)
		}
		srv.Close()
	}

	// The reverse-URL helpers exist for the reads only (§V5.5.6).
	if app.PathRefUsers() != "/ref/users" || app.PathRefUser(7) != "/ref/users/7" {
		t.Fatalf("helpers: %q %q", app.PathRefUsers(), app.PathRefUser(7))
	}
	// PUT shares the PATCH route's handler.
	h := handlerWithDB(t)
	if rec := call(h, "PUT", "/ref/users/1", nil); rec.Code != 400 {
		t.Fatalf("put with no body: %d, want 400", rec.Code)
	}
}
