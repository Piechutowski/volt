// resources [default] (spec §default-handlers): the action table expanded into
// query routes over the table's CRUD — no controller — reached through
// the generated client in both wire formats, with §query-routes's statuses.
package itest

import (
	"context"
	"encoding/json"
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
	if rec := call(h, "PATCH", "/ref/users/1", nil); rec.Code != 400 {
		t.Fatalf("patch with no body: %d, want 400", rec.Code)
	}
	if rec := call(h, "PATCH", "/ref/users/1", strings.NewReader(`{}`)); rec.Code != 200 || !strings.Contains(rec.Body.String(), "one@example.com") {
		t.Fatalf("patch with an empty object: %d %s, want 200 with the unchanged row", rec.Code, rec.Body.String())
	}
	if rec := call(h, "PUT", "/ref/users/1", strings.NewReader(`{}`)); rec.Code != 405 || strings.Contains(rec.Header().Get("Allow"), "PUT") {
		t.Fatalf("put: %d, Allow %q; want 405 without PUT, the update is PATCH alone (§the-action-table)", rec.Code, rec.Header().Get("Allow"))
	}
}

// TestResourcesDefaultStringKey (§default-handlers, §query-routes): a table
// keyed by a string column, not the rowid alias, takes its key in the
// create body, and an update ignores a body that names the key, which
// is outside the update's allow list.
func TestResourcesDefaultStringKey(t *testing.T) {
	h := handlerWithDB(t)
	rec := call(h, "POST", "/ref/tags", strings.NewReader(`{"slug":"sqlite","name":"SQLite"}`))
	var made db.Tag
	if rec.Code != 201 || json.Unmarshal(rec.Body.Bytes(), &made) != nil || made.Slug != "sqlite" || made.Name != "SQLite" {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, "GET", app.PathRefTag("sqlite"), nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"slug":"sqlite"`) {
		t.Fatalf("show: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, "PATCH", "/ref/tags/sqlite", strings.NewReader(`{"name":"SQLite 3"}`))
	var up db.Tag
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &up) != nil || up.Slug != "sqlite" || up.Name != "SQLite 3" {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, "PATCH", "/ref/tags/sqlite", strings.NewReader(`{"slug":"renamed","name":"SQLite"}`))
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &up) != nil || up.Slug != "sqlite" || up.Name != "SQLite" {
		t.Fatalf("update naming the key: %d %s, want 200 with the key unchanged (§query-routes)", rec.Code, rec.Body.String())
	}
	if rec = call(h, "GET", app.PathRefTag("renamed"), nil); rec.Code != 404 {
		t.Fatalf("the key named in an update body was applied: %d %s", rec.Code, rec.Body.String())
	}
	if rec = call(h, "DELETE", "/ref/tags/sqlite", nil); rec.Code != 204 || rec.Body.Len() != 0 {
		t.Fatalf("delete: %d %q", rec.Code, rec.Body.String())
	}
	if rec = call(h, "GET", app.PathRefTag("sqlite"), nil); rec.Code != 404 {
		t.Fatalf("after delete: %d, want 404", rec.Code)
	}

	for _, f := range []volt.Format{volt.FormatJSON, volt.FormatGOB} {
		srv := httptest.NewServer(handlerWithDB(t))
		c := client.New(srv.URL)
		c.Format = f
		ctx := context.Background()
		made, err := c.RefCreateTag(ctx, db.TagNew().SlugSet("rust").NameSet("Rust"))
		if err != nil || made.Slug != "rust" {
			t.Fatalf("%v: create = %+v, %v", f, made, err)
		}
		all, err := c.RefTags(ctx)
		if err != nil || len(all) != 2 {
			t.Fatalf("%v: index = %+v, %v", f, all, err)
		}
		up, err := c.RefUpdateTag(ctx, "rust", db.TagNew().NameSet("Rust lang"))
		if err != nil || up.Slug != "rust" || up.Name != "Rust lang" {
			t.Fatalf("%v: update = %+v, %v", f, up, err)
		}
		one, err := c.RefTag(ctx, "rust")
		if err != nil || one != up {
			t.Fatalf("%v: show = %+v, %v", f, one, err)
		}
		if err := c.RefDeleteTag(ctx, "rust"); err != nil {
			t.Fatalf("%v: delete = %v", f, err)
		}
		if _, err := c.RefTag(ctx, "rust"); !errors.Is(err, volt.ErrNotFound) {
			t.Fatalf("%v: after delete = %v, want volt.ErrNotFound", f, err)
		}
		srv.Close()
	}
}

// TestResourcesDefaultSharedKey (§default-handlers, §columns,
// §changesets): a shared-key one-to-one, whose int key is also the
// foreign key, is the rowid alias in the DDL but no database-assigned
// key, so the generated create takes user_id from the body and
// SQLite stores the value given, not a rowid of its choosing, and a body
// that leaves user_id out is refused as not_null before any insert.
func TestResourcesDefaultSharedKey(t *testing.T) {
	h := handlerWithDB(t)
	for _, email := range []string{"two@example.com", "three@example.com"} {
		if rec := call(h, "POST", "/ref/users", strings.NewReader(`{"email":"`+email+`"}`), "Content-Type", "application/json"); rec.Code != 201 {
			t.Fatalf("user create: %d %s", rec.Code, rec.Body.String())
		}
	}
	rec := call(h, "POST", "/ref/profiles", strings.NewReader(`{"user_id":3,"bio":"third"}`), "Content-Type", "application/json")
	var made db.Profile
	if rec.Code != 201 || json.Unmarshal(rec.Body.Bytes(), &made) != nil || made.UserID != 3 || made.Bio != "third" {
		t.Fatalf("create: %d %s, want 201 with user_id 3 from the body, not the rowid 2 SQLite would assign", rec.Code, rec.Body.String())
	}
	rec = call(h, "GET", app.PathRefProfile(3), nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"user_id":3`) || !strings.Contains(rec.Body.String(), `"bio":"third"`) {
		t.Fatalf("show: %d %s", rec.Code, rec.Body.String())
	}
	if rec = call(h, "GET", app.PathRefProfile(2), nil); rec.Code != 404 {
		t.Fatalf("show of a profile never created: %d %s, want 404", rec.Code, rec.Body.String())
	}
	if rec = call(h, "POST", "/ref/profiles", strings.NewReader(`{"user_id":7,"bio":"nobody"}`), "Content-Type", "application/json"); rec.Code < 400 {
		t.Fatalf("create for a user that does not exist: %d %s, want the foreign key refused", rec.Code, rec.Body.String())
	}
	if rec = call(h, "POST", "/ref/users", strings.NewReader(`{"email":"four@example.com"}`), "Content-Type", "application/json"); rec.Code != 201 {
		t.Fatalf("user create: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, "POST", "/ref/profiles", strings.NewReader(`{"bio":"nokey"}`), "Content-Type", "application/json")
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), `"check":"not_null"`) || !strings.Contains(rec.Body.String(), `"user_id"`) {
		t.Fatalf("create without user_id: %d %s, want 422 not_null on user_id, not the rowid 4 SQLite would assign to user 4's row", rec.Code, rec.Body.String())
	}
	if rec = call(h, "GET", app.PathRefProfile(4), nil); rec.Code != 404 {
		t.Fatalf("a create without user_id stored a profile for user 4: %d %s", rec.Code, rec.Body.String())
	}
	if got := db.ProfileNew().From(db.Profile{UserID: 3, Bio: "x"}); !got.Changed(db.ProfileUserID) {
		t.Fatal("From left the shared key unmarked; it is no database-assigned key (§changesets)")
	}
}
