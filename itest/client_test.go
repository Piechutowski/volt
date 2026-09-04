// The generated client (spec §V4.10) against the fixture server: the
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

		made, err := c.APIUserCreate(ctx, db.UserCreateParams{Email: "two@example.com"})
		if err != nil || made.Email != "two@example.com" || made.ID == 0 {
			t.Fatalf("%v: create = %+v, %v", f, made, err)
		}
		all, err := c.APIUserList(ctx)
		if err != nil || len(all) != 2 {
			t.Fatalf("%v: list = %+v, %v", f, all, err)
		}
		one, err := c.APIUserGet(ctx, made.ID)
		if err != nil || one.ID != made.ID {
			t.Fatalf("%v: get = %+v, %v", f, one, err)
		}
		if _, err := c.APIUserGet(ctx, 999); !errors.Is(err, volt.ErrNotFound) {
			t.Fatalf("%v: miss = %v, want volt.ErrNotFound", f, err)
		}
		up, err := c.APIUserUpdate(ctx, made.ID, db.UserUpdateParams{Email: "dos@example.com"})
		if err != nil || up.Email != "dos@example.com" {
			t.Fatalf("%v: update = %+v, %v", f, up, err)
		}
		picked, err := c.APIUserPicked(ctx, []int32{made.ID, 1})
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

		// Dataset routes (§V13): the same signature for every member.
		if _, err := c.MsRevenueBrowse(ctx, 2024); err != nil {
			t.Fatalf("%v: dataset route = %v", f, err)
		}
		if _, err := c.MsUsageBrowse(ctx, 2024); err != nil {
			t.Fatalf("%v: dataset route = %v", f, err)
		}

		// A named controller route: raw response, the echo body.
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

// TestEventRouteStreams (§V4.11): what the server publishes reaches
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
		IDs   []int32
	}
	if err := broker.Publish("table.changed", change{"users", []int32{1}}); err != nil {
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
	// A subscriber arriving after the fact still sees it (replay).
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

// TestClientValidationErrors: the generated client sees the same
// statuses as HTTPError values (§V12.7).
func TestClientValidationErrors(t *testing.T) {
	srv := httptest.NewServer(handlerWithDB(t))
	defer srv.Close()
	c := client.New(srv.URL)
	c.Format = volt.FormatGOB
	ctx := context.Background()

	_, err := c.APIUserCreate(ctx, db.UserCreateParams{Email: "nope"})
	var he volt.HTTPError
	if !errors.As(err, &he) || he.StatusCode() != 422 || !strings.Contains(he.Error(), "EmailValid") {
		t.Fatalf("invalid email = %v", err)
	}
	_, err = c.APIUserCreate(ctx, db.UserCreateParams{Email: "one@example.com"})
	if !errors.As(err, &he) || he.StatusCode() != 409 {
		t.Fatalf("duplicate = %v", err)
	}
}
