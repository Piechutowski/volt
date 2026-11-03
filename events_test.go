package volt

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

	b.Publish("compute.finished", progress{3, 3})
	resumed := c.streamAfter(t, srv.URL, 3)
	if resumed.ID != 4 || resumed.Name != "compute.finished" {
		t.Fatalf("resumed event = %+v", resumed)
	}
}

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
// ID from a broker that is gone sees what the new broker published
// before it reconnected (spec §event-routes): the restarted process
// counts from 1 again, below the client's Last-Event-ID.
func TestStreamResumesAcrossABrokerRestart(t *testing.T) {
	before := NewBroker()
	for i := range 6 {
		before.Publish("old", i)
	}
	after := NewBroker()
	after.Publish("created", 1)
	after.Publish("deleted", 2)
	srv := httptest.NewServer(EventsHandler(after))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL, nil)
	req.Header.Set("Last-Event-ID", "6")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	sc := bufio.NewScanner(resp.Body)
	var got []string
	for len(got) < 2 && sc.Scan() {
		if name, ok := strings.CutPrefix(sc.Text(), "event: "); ok {
			got = append(got, name)
		}
	}
	if strings.Join(got, " ") != "created deleted" {
		t.Errorf("resuming at 6 on a restarted broker = %q, want created deleted", got)
	}
	ch, stop := after.Subscribe(1)
	defer stop()
	if ev := <-ch; ev.ID != 2 {
		t.Errorf("resuming at 1, an ID this broker assigned, starts at %d, want 2", ev.ID)
	}
}
