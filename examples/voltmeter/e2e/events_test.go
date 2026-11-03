package e2e

import (
	"bufio"
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/voltmeter/app"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/server"
)

// TestEvents proves the event route end to end (spec §event-routes):
// the server publishes one event per successful write and none for a
// read, a refused write or a sign-in, the generated client receives them
// in order, reconnects after the server drops it and resumes after the
// last ID it saw without a repeat, a raw stream opened without
// Last-Event-ID starts at the oldest kept event, one opened with
// Last-Event-ID replays exactly what came after, and one naming an ID
// this broker never assigned, as after a restart, replays everything.
func TestEvents(t *testing.T) {
	e := start(t)
	ctx, cancel := context.WithTimeout(bg, 30*time.Second)
	defer cancel()
	c := e.gob(0)
	events := c.Events(ctx)
	next := func() (volt.Event, server.Change) {
		t.Helper()
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatal("the event channel closed")
			}
			var ch server.Change
			if err := ev.Decode(&ch); err != nil {
				t.Fatalf("event %d: %v", ev.ID, err)
			}
			return ev, ch
		case <-ctx.Done():
			t.Fatal("no event arrived")
		}
		return volt.Event{}, server.Change{}
	}

	if _, err := c.APICreateTag(bg, db.TagNew().SlugSet("release").NameSet("Release")); err != nil {
		t.Fatal(err)
	}
	ev, ch := next()
	if ev.ID != 1 || ev.Name != server.EventChanged || ch != (server.Change{Method: "POST", Route: "POST /api/tags", Handler: "db.TagCreate", Path: "/api/tags", Status: 201}) {
		t.Errorf("first event %d %q %+v", ev.ID, ev.Name, ch)
	}
	if _, err := c.APICreateTag(bg, db.TagNew().SlugSet("release").NameSet("Again")); err == nil {
		t.Fatal("a duplicate slug was stored")
	}
	if _, err := c.APITags(bg); err != nil {
		t.Fatal(err)
	}
	if _, err := c.APIUpdateTag(bg, "release", db.TagNew().NameSet("Releases")); err != nil {
		t.Fatal(err)
	}
	ev, ch = next()
	if ev.ID != 2 || ch.Handler != "db.TagUpdate" || ch.Method != "PATCH" || ch.Path != "/api/tags/release" {
		t.Errorf("after a refused write and a read, the next event is %d %+v; want 2, the update", ev.ID, ch)
	}

	e.srv.CloseClientConnections()
	c.HTTP.CloseIdleConnections()
	if err := c.APIDeleteTag(bg, "release"); err != nil {
		t.Fatal(err)
	}
	ev, ch = next()
	if ev.ID != 3 || ch.Handler != "db.TagDelete" || ch.Status != 204 {
		t.Errorf("after the drop the client resumed at %d %+v; want 3, the delete, and no repeat", ev.ID, ch)
	}
	if r := e.raw("POST", app.PathSignIn(), `{"email": "ada@voltmeter.test"}`); r.StatusCode != 200 {
		t.Fatalf("sign in = %d %s", r.StatusCode, r.body)
	}
	up := upload(t, c, app.Upload{Commit: "beef0001", Machine: machine})
	ev, ch = next()
	if ev.ID != 4 || ch.Handler != "Uploads.Create" || ch.Route != "POST /api/uploads" || ch.Status != 201 || up.Run.ID != 1 {
		t.Errorf("the upload's event %d %+v; want 4, the sign-in changing no row", ev.ID, ch)
	}

	replayed := func(lastID string) []string {
		t.Helper()
		sctx, stop := context.WithTimeout(ctx, 5*time.Second)
		defer stop()
		req, err := http.NewRequestWithContext(sctx, "GET", e.srv.URL+app.PathEvents(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if lastID != "" {
			req.Header.Set("Last-Event-ID", lastID)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.Header.Get("Content-Type") != "text/event-stream" || resp.Header.Values("X-Pipeline")[0] != "api" {
			t.Errorf("stream headers %v", resp.Header)
		}
		want := 4
		if n, _ := strconv.Atoi(lastID); n > 0 && n <= 4 {
			want -= n
		}
		var ids []string
		sc := bufio.NewScanner(resp.Body)
		for len(ids) < want && sc.Scan() {
			if id, ok := strings.CutPrefix(sc.Text(), "id: "); ok {
				ids = append(ids, id)
			}
		}
		return ids
	}
	if got := strings.Join(replayed(""), " "); got != "1 2 3 4" {
		t.Errorf("a new stream replays %q", got)
	}
	if got := strings.Join(replayed("2"), " "); got != "3 4" {
		t.Errorf("Last-Event-ID 2 replays %q", got)
	}
	if got := strings.Join(replayed("99"), " "); got != "1 2 3 4" {
		t.Errorf("Last-Event-ID 99, from a broker before a restart, replays %q", got)
	}
}
