// The generated client (spec §generated-client) against the fixture server: the
// typed methods round-trip the query routes in both wire formats, a
// miss is errors.Is(err, volt.ErrNotFound), and a named controller
// route is reachable as a raw response.
package itest

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/itest/blog/app/client"
	"github.com/Piechutowski/volt/itest/blog/db"
)

func TestClientQueryRoutes(t *testing.T) {
	for _, f := range []volt.Format{volt.FormatJSON, volt.FormatGOB} {
		srv := httptest.NewServer(handlerWithDB(t))
		c := client.New(srv.URL)
		c.Format = f
		ctx := context.Background()

		made, err := c.APIUserCreate(ctx, db.UserNew().EmailSet("two@example.com"))
		if err != nil || made.Email != "two@example.com" || made.ID == 0 {
			t.Fatalf("%v: create = %+v, %v", f, made, err)
		}
		all, err := c.APIUserList(ctx)
		if err != nil || len(all) != 2 {
			t.Fatalf("%v: list = %+v, %v", f, all, err)
		}
		since, err := c.APIUserSince(ctx, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
		if err != nil || len(since) != 2 {
			t.Fatalf("%v: since 2000 = %+v, %v; a time.Time parameter travels as RFC 3339 (§generated-client)", f, since, err)
		}
		if none, err := c.APIUserSince(ctx, time.Date(2999, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil || len(none) != 0 {
			t.Fatalf("%v: since 2999 = %+v, %v", f, none, err)
		}
		one, err := c.APIUserGet(ctx, made.ID)
		if err != nil || one.ID != made.ID {
			t.Fatalf("%v: get = %+v, %v", f, one, err)
		}
		if _, err := c.APIUserGet(ctx, 999); !errors.Is(err, volt.ErrNotFound) {
			t.Fatalf("%v: miss = %v, want volt.ErrNotFound", f, err)
		}
		up, err := c.APIUserUpdate(ctx, made.ID, db.UserNew().EmailSet("dos@example.com"))
		if err != nil || up.Email != "dos@example.com" {
			t.Fatalf("%v: update = %+v, %v", f, up, err)
		}
		picked, err := c.APIUserPicked(ctx, []int{made.ID, 1})
		if err != nil || len(picked) != 2 || picked[0].ID != 1 {
			t.Fatalf("%v: picked = %+v, %v", f, picked, err)
		}
		if none, err := c.APIUserPicked(ctx, nil); err != nil || len(none) != 0 {
			t.Fatalf("%v: picked none = %+v, %v", f, none, err)
		}
		if err := c.APIUserDelete(ctx, made.ID); err != nil {
			t.Fatalf("%v: delete = %v", f, err)
		}
		if err := c.APIUserDelete(ctx, made.ID); !errors.Is(err, volt.ErrNotFound) {
			t.Fatalf("%v: second delete = %v, want volt.ErrNotFound", f, err)
		}

		if _, err := c.MsRevenueBrowse(ctx, 2024); err != nil {
			t.Fatalf("%v: dataset route = %v", f, err)
		}
		if _, err := c.MsUsageBrowse(ctx, 2024); err != nil {
			t.Fatalf("%v: dataset route = %v", f, err)
		}

		resp, err := c.User(ctx, 7)
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("%v: raw user = %v %v", f, resp, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if !strings.Contains(string(body), "/users/{id}") {
			t.Fatalf("%v: raw body %q", f, body)
		}
		srv.Close()
	}
}

// TestEventRouteStreams (§event-routes): what the server publishes reaches
// every connected client through the generated Events method, in
// order, and a late subscriber replays what it missed.
func TestEventRouteStreams(t *testing.T) {
	h, broker := fixture(t)
	srv := httptest.NewServer(h)
	defer srv.Close()
	c := client.New(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a := c.Events(ctx)
	b := c.Events(ctx)
	type change struct {
		Table string
		IDs   []int
	}
	if err := broker.Publish("table.changed", change{"users", []int{1}}); err != nil {
		t.Fatal(err)
	}
	for name, ch := range map[string]<-chan volt.Event{"a": a, "b": b} {
		select {
		case ev := <-ch:
			var got change
			if err := ev.Decode(&got); err != nil || ev.Name != "table.changed" || got.Table != "users" || len(got.IDs) != 1 {
				t.Fatalf("%s: event = %+v (%v)", name, ev, err)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("%s: no event", name)
		}
	}
	late := c.Events(ctx)
	select {
	case ev := <-late:
		if ev.Name != "table.changed" {
			t.Fatalf("late subscriber got %+v", ev)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("late subscriber saw no replay")
	}
}

// TestClientValidationErrors: the generated client sees a refused
// write as a *volt.ProblemError carrying the status and the field
// details the server itemized (§formats, §changesets), in GOB too.
func TestClientValidationErrors(t *testing.T) {
	srv := httptest.NewServer(handlerWithDB(t))
	defer srv.Close()
	c := client.New(srv.URL)
	c.Format = volt.FormatGOB
	ctx := context.Background()

	_, err := c.APIUserCreate(ctx, db.UserNew().EmailSet("nope"))
	var he volt.HTTPError
	if !errors.As(err, &he) || he.StatusCode() != 422 {
		t.Fatalf("invalid email = %v", err)
	}
	var pe *volt.ProblemError
	if !errors.As(err, &pe) || !detailNamed(pe.Details(), "EmailValid", "email") {
		t.Fatalf("no attribution through the client: %v", err)
	}
	_, err = c.APIUserUpdate(ctx, 1, db.UserNew().EmailSet(""))
	if !errors.As(err, &pe) || pe.StatusCode() != 422 || !detailNamed(pe.Details(), "required", "email") {
		t.Fatalf("empty email on update = %v, want 422 with required on email", err)
	}
	_, err = c.APIUserCreate(ctx, db.UserNew().EmailSet("one@example.com"))
	if !errors.As(err, &pe) || pe.StatusCode() != 409 || !detailNamed(pe.Details(), "unique", "email") {
		t.Fatalf("duplicate = %v, want 409 with unique on email", err)
	}
	if _, err := c.APIUserUpdate(ctx, 999, db.UserNew().EmailSet("x@example.com")); !errors.Is(err, volt.ErrNotFound) {
		t.Fatalf("update of a missing row = %v, want volt.ErrNotFound", err)
	}
}
