package volt

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type progress struct {
	Done, Total int
}

func TestBrokerFanOutReplayAndReconnect(t *testing.T) {
	b := &Broker{Replay: 8, Heartbeat: 20 * time.Millisecond}
	srv := httptest.NewServer(EventsHandler(b))
	defer srv.Close()

	if err := b.Publish("compute.started", progress{0, 3}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := &Client{Base: srv.URL}
	a := c.Stream(ctx, "/events")
	z := c.Stream(ctx, "/events")

	first := <-a
	if first.Name != "compute.started" || first.ID != 1 || !first.Replayed || first.Epoch != b.epochOf() {
		t.Fatalf("replayed event = %+v, want ID 1 of this broker marked replayed", first)
	}
	<-z

	b.Publish("compute.progress", progress{1, 3})
	b.Publish("compute.progress", progress{2, 3})
	for name, ch := range map[string]<-chan Event{"a": a, "z": z} {
		for want := 1; want <= 2; want++ {
			select {
			case ev := <-ch:
				var p progress
				if err := ev.Decode(&p); err != nil || ev.Name != "compute.progress" || p.Done != want || ev.Replayed {
					t.Fatalf("%s: event %d = %+v (%v)", name, want, ev, err)
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("%s: no event %d", name, want)
			}
		}
	}

	b.Publish("compute.finished", progress{3, 3})
	resumed := c.streamAfter(t, srv.URL, Event{ID: 3, Epoch: b.epochOf()}.WireID())
	if resumed.ID != 4 || resumed.Name != "compute.finished" {
		t.Fatalf("resumed event = %+v", resumed)
	}
}

func (c *Client) streamAfter(t *testing.T, base string, last string) Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out := make(chan Event, 8)
	go c.streamOnce(ctx, "/events", &last, out)
	select {
	case ev := <-out:
		return ev
	case <-ctx.Done():
		t.Fatal("no resumed event")
		return Event{}
	}
}

func TestBrokerDropsStalledSubscriber(t *testing.T) {
	b := &Broker{Replay: 2}
	ch, cancel := b.Subscribe(0)
	defer cancel()
	for i := 0; i < 2+64+1; i++ {
		b.Publish("x", i)
	}
	closed := false
	for range 200 {
		if _, ok := <-ch; !ok {
			closed = true
			break
		}
	}
	if !closed {
		t.Fatal("a stalled subscriber must be dropped, not block the publisher")
	}
}

func TestStreamReconnectsAfterServerRestart(t *testing.T) {
	b := &Broker{Heartbeat: 10 * time.Millisecond}
	h := EventsHandler(b)
	srv := httptest.NewServer(h)
	c := &Client{Base: srv.URL}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := c.Stream(ctx, "/events")
	b.Publish("one", 1)
	if ev := <-events; ev.Name != "one" {
		t.Fatalf("first = %+v", ev)
	}
	srv.CloseClientConnections()
	time.Sleep(50 * time.Millisecond)
	b.Publish("two", 2)
	select {
	case ev := <-events:
		if ev.Name != "two" {
			t.Fatalf("after reconnect = %+v", ev)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no event after reconnect")
	}
	cancel()
	for range events {
	}
	srv.Close()
}

// TestStreamResumesAcrossABrokerRestart proves a client resuming with an
// ID from a broker that is gone sees everything the new broker published
// before it reconnected, however far the new broker has counted, and
// then resumes on the new broker's IDs without a repeat (spec
// §event-routes): a wire ID names its broker, so the new one never
// mistakes an old ID for its own.
func TestStreamResumesAcrossABrokerRestart(t *testing.T) {
	var current atomic.Pointer[Broker]
	before := NewBroker()
	current.Store(before)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		EventsHandler(current.Load()).ServeHTTP(w, r)
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	events := (&Client{Base: srv.URL}).Stream(ctx, "/events")
	names := func(n int) []string {
		t.Helper()
		var got []string
		for len(got) < n {
			select {
			case ev := <-events:
				got = append(got, ev.Name)
			case <-ctx.Done():
				t.Fatalf("after %q no further event", got)
			}
		}
		return got
	}
	for i := range 6 {
		before.Publish(fmt.Sprintf("old%d", i), i)
	}
	if got := strings.Join(names(6), " "); got != "old0 old1 old2 old3 old4 old5" {
		t.Fatalf("before the restart = %q", got)
	}

	after := NewBroker()
	current.Store(after)
	for i := range 7 {
		after.Publish(fmt.Sprintf("new%d", i), i)
	}
	srv.CloseClientConnections()
	if got := strings.Join(names(7), " "); got != "new0 new1 new2 new3 new4 new5 new6" {
		t.Errorf("resuming on a restarted broker that counted past the old ID = %q, want new0 to new6", got)
	}

	srv.CloseClientConnections()
	after.Publish("new7", 7)
	if got := names(1); got[0] != "new7" {
		t.Errorf("after a second drop the client resumed at %q, want new7 and no repeat", got)
	}
	select {
	case ev := <-events:
		t.Errorf("a repeat after the second drop: %+v", ev)
	case <-time.After(300 * time.Millisecond):
	}

	for _, last := range []string{"6", Event{ID: 6, Epoch: before.epochOf()}.WireID(), "garbage", ""} {
		ch, stop := after.SubscribeAfter(last)
		if ev := <-ch; ev.ID != 1 {
			t.Errorf("Last-Event-ID %q on the new broker starts at %d, want the whole buffer", last, ev.ID)
		}
		stop()
	}
	ch, stop := after.SubscribeAfter(Event{ID: 1, Epoch: after.epochOf()}.WireID())
	defer stop()
	if ev := <-ch; ev.ID != 2 {
		t.Errorf("resuming at 1, an ID this broker assigned, starts at %d, want 2", ev.ID)
	}
}
