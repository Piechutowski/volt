// resources [default] (spec §default-handlers): the action table expanded into
// query routes over the table's CRUD — no controller — reached through
// the generated client in both wire formats, with §query-routes's statuses.
package itest

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
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

		made, err := c.RefCreateUser(ctx, db.UserNew().EmailSet("ref@example.com"))
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
		up, err := c.RefUpdateUser(ctx, made.ID, db.UserNew().EmailSet("fer@example.com"))
		if err != nil || up.Email != "fer@example.com" {
			t.Fatalf("%v: update = %+v, %v", f, up, err)
		}
		same, err := c.RefUpdateUser(ctx, made.ID, db.UserNew())
		if err != nil || same != up {
			t.Fatalf("%v: empty update = %+v, %v; want the unchanged row (§changesets)", f, same, err)
		}
		var pe *volt.ProblemError
		if _, err := c.RefUpdateUser(ctx, made.ID, db.UserNew().EmailSet("")); !errors.As(err, &pe) || pe.StatusCode() != 422 || !detailNamed(pe.Details(), "required", "email") {
			t.Fatalf("%v: empty email = %v, want 422 with required on email", f, err)
		}
		if err := c.RefDeleteUser(ctx, made.ID); err != nil {
			t.Fatalf("%v: delete = %v", f, err)
		}
		if _, err := c.RefUser(ctx, made.ID); !errors.Is(err, volt.ErrNotFound) {
			t.Fatalf("%v: after delete = %v, want volt.ErrNotFound", f, err)
		}
		srv.Close()
	}

	if app.PathRefUsers() != "/ref/users" || app.PathRefUser(7) != "/ref/users/7" {
		t.Fatalf("helpers: %q %q", app.PathRefUsers(), app.PathRefUser(7))
	}
	h := handlerWithDB(t)
	if rec := call(h, "PUT", "/ref/users/1", nil); rec.Code != 400 {
		t.Fatalf("put with no body: %d, want 400", rec.Code)
	}
	if rec := call(h, "PUT", "/ref/users/1", strings.NewReader(`{}`)); rec.Code != 200 || !strings.Contains(rec.Body.String(), "one@example.com") {
		t.Fatalf("put with an empty object: %d %s, want 200 with the unchanged row", rec.Code, rec.Body.String())
	}
}
