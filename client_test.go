package volt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

// TestClientHeaderOnEveryRequest proves the client's Header rides on every
// request Do, Raw and the event stream send, while Accept, Content-Type
// and a stream's Last-Event-ID stay the client's own, and that a stream
// keeps the headers of the moment it was opened across reconnections
// (spec §generated-client).
func TestClientHeaderOnEveryRequest(t *testing.T) {
	b := &Broker{Heartbeat: 10 * time.Millisecond}
	events := EventsHandler(b)
	seen := make(chan http.Header, 16)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Header.Clone()
		if r.URL.Path == "/events" {
			events.ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	next := func(what string) http.Header {
		t.Helper()
		select {
		case h := <-seen:
			return h
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: no request reached the server", what)
			return nil
		}
	}
	expect := func(what string, h http.Header, user, accept, contentType string) {
		t.Helper()
		if got := h.Get("X-User"); got != user {
			t.Errorf("%s: X-User %q, want %q", what, got, user)
		}
		if got := h.Values("X-Trace"); !reflect.DeepEqual(got, []string{"a", "b"}) {
			t.Errorf("%s: X-Trace %q, want both values in order", what, got)
		}
		if got := h.Values("Accept"); !reflect.DeepEqual(got, []string{accept}) {
			t.Errorf("%s: Accept %q, want the client's own %q alone", what, got, accept)
		}
		if got := h.Values("Content-Type"); contentType == "" && len(got) != 0 || contentType != "" && !reflect.DeepEqual(got, []string{contentType}) {
			t.Errorf("%s: Content-Type %q, want %q", what, got, contentType)
		}
	}
	expectResumeFromHeader := func(what string, h http.Header) {
		t.Helper()
		if got := h.Values("Last-Event-ID"); !reflect.DeepEqual(got, []string{"stale-9"}) {
			t.Errorf("%s: Last-Event-ID %q, want the Header's %q: only a stream writes its own", what, got, "stale-9")
		}
	}

	c := &Client{Base: srv.URL, Header: http.Header{}}
	c.Header.Set("X-User", "1")
	c.Header.Add("X-Trace", "a")
	c.Header.Add("X-Trace", "b")
	c.Header.Set("Accept", "text/html")
	c.Header.Set("Content-Type", "text/plain")
	c.Header.Set("Last-Event-ID", "stale-9")
	ctx := context.Background()
	for _, f := range []Format{FormatJSON, FormatGOB} {
		c.Format = f
		if err := c.Do(ctx, "GET", "/posts", nil, nil); err != nil {
			t.Fatal(err)
		}
		h := next("Do")
		expect(f.MIME()+" Do without a body", h, "1", f.MIME(), "")
		expectResumeFromHeader(f.MIME()+" Do without a body", h)
		if err := c.Do(ctx, "POST", "/posts", row{1, "first post"}, nil); err != nil {
			t.Fatal(err)
		}
		h = next("Do")
		expect(f.MIME()+" Do with a body", h, "1", f.MIME(), f.MIME())
		expectResumeFromHeader(f.MIME()+" Do with a body", h)
		for _, in := range []any{nil, row{2, "second post"}} {
			resp, err := c.Raw(ctx, "POST", "/posts/1/publish", in)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			want := ""
			if in != nil {
				want = f.MIME()
			}
			h = next("Raw")
			expect(f.MIME()+" Raw", h, "1", f.MIME(), want)
			expectResumeFromHeader(f.MIME()+" Raw", h)
		}
	}
	if got := c.Header.Get("Accept"); got != "text/html" {
		t.Errorf("the client wrote into its Header: Accept %q", got)
	}

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream := c.Stream(streamCtx, "/events")
	h := next("Stream")
	expect("Stream", h, "1", "text/event-stream", "")
	if got := h.Values("Last-Event-ID"); len(got) != 0 {
		t.Errorf("first connection: Last-Event-ID %q, want none: the Header's is not the stream's", got)
	}
	c.Header.Set("X-User", "2")
	b.Publish("post.published", 1)
	var ev Event
	select {
	case ev = <-stream:
	case <-time.After(5 * time.Second):
		t.Fatal("no event")
	}
	srv.CloseClientConnections()
	h = next("Stream reconnection")
	expect("Stream reconnection", h, "1", "text/event-stream", "")
	if got := h.Values("Last-Event-ID"); !reflect.DeepEqual(got, []string{ev.WireID()}) {
		t.Errorf("reconnection: Last-Event-ID %q, want the delivered %q alone", got, ev.WireID())
	}
	if err := c.Do(ctx, "GET", "/posts", nil, nil); err != nil {
		t.Fatal(err)
	}
	expect("Do after the change", next("Do"), "2", c.Format.MIME(), "")
	cancel()
	for range stream {
	}

	bare := &Client{Base: srv.URL}
	if err := bare.Do(ctx, "GET", "/posts", nil, nil); err != nil {
		t.Fatal(err)
	}
	if h := next("nil Header"); h.Get("X-User") != "" || h.Get("Accept") != MIMEJSON {
		t.Errorf("a nil Header: X-User %q, Accept %q; want none and JSON", h.Get("X-User"), h.Get("Accept"))
	}
}
