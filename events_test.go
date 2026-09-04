package volt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type progress struct {
	Done, Total int
}

func TestBrokerFanOutReplayAndReconnect(t *testing.T) {
	b := &Broker{Replay: 8, Heartbeat: 20 * time.Millisecond}
	srv := httptest.NewServer(Handler("GET /events", func(w http.ResponseWriter, r *Request) error {
		return b.Serve(w, r)
	}, nil))
	defer srv.Close()

	// Published before anyone listens: replayed to a fresh subscriber.
	if err := b.Publish("compute.started", progress{0, 3}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := &Client{Base: srv.URL}
	a := c.Stream(ctx, "/events")
	z := c.Stream(ctx, "/events")

	first := <-a
	if first.Name != "compute.started" || first.ID != 1 {
		t.Fatalf("replayed event = %+v", first)
	}
	<-z

	b.Publish("compute.progress", progress{1, 3})
	b.Publish("compute.progress", progress{2, 3})
	for name, ch := range map[string]<-chan Event{"a": a, "z": z} {
		for want := 1; want <= 2; want++ {
			select {
			case ev := <-ch:
				var p progress
				if err := ev.Decode(&p); err != nil || ev.Name != "compute.progress" || p.Done != want {
					t.Fatalf("%s: event %d = %+v (%v)", name, want, ev, err)
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("%s: no event %d", name, want)
			}
		}
	}

	// Resume: a client that was at ID 3 gets only what came after.
	b.Publish("compute.finished", progress{3, 3})
	resumed := c.streamAfter(t, srv.URL, 3)
	if resumed.ID != 4 || resumed.Name != "compute.finished" {
		t.Fatalf("resumed event = %+v", resumed)
	}
}

// streamAfter opens one connection with Last-Event-ID and returns the
// first event it yields.
func (c *Client) streamAfter(t *testing.T, base string, last uint64) Event {
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
	// The channel was closed on overflow; draining it ends with !ok.
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
	h := Handler("GET /events", func(w http.ResponseWriter, r *Request) error { return b.Serve(w, r) }, nil)
	srv := httptest.NewServer(h)
	c := &Client{Base: srv.URL}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := c.Stream(ctx, "/events")
	b.Publish("one", 1)
	if ev := <-events; ev.Name != "one" {
		t.Fatalf("first = %+v", ev)
	}
	srv.CloseClientConnections() // the stream drops; Stream reconnects with Last-Event-ID
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
