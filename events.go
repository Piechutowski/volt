package volt

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Event is one server-sent event (spec §V4.11): a name the client
// dispatches on and a JSON payload. The broker assigns IDs in
// publication order so a client that reconnects can say where it was.
type Event struct {
	ID   uint64
	Name string
	Data json.RawMessage
}

// Decode unmarshals the payload into v.
func (e Event) Decode(v any) error { return json.Unmarshal(e.Data, v) }

// Broker fans events out to every open event-stream connection and
// keeps the last Replay events so a reconnecting client resumes from
// its Last-Event-ID. One process, in memory: the shape the runtime
// promises; a multi-process bus is not its job.
type Broker struct {
	// Replay is how many recent events are kept for resumption; 0
	// means the default of 256.
	Replay int
	// Heartbeat is the interval of keep-alive comments on idle streams;
	// 0 means 15 seconds.
	Heartbeat time.Duration

	mu     sync.Mutex
	subs   map[chan Event]struct{}
	buffer []Event
	next   uint64
}

// NewBroker returns a broker with the default replay depth.
func NewBroker() *Broker { return &Broker{} }

func (b *Broker) replay() int {
	if b.Replay > 0 {
		return b.Replay
	}
	return 256
}

// Publish assigns the next ID, remembers the event for replay and
// hands it to every subscriber. A subscriber whose buffer is full is
// dropped — its connection ends and the client reconnects with
// Last-Event-ID, catching up from the replay buffer — so one stalled
// reader never blocks the publisher or the others.
func (b *Broker) Publish(name string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.next++
	ev := Event{ID: b.next, Name: name, Data: raw}
	b.buffer = append(b.buffer, ev)
	if n := b.replay(); len(b.buffer) > n {
		b.buffer = b.buffer[len(b.buffer)-n:]
	}
	for ch := range b.subs {
		select {
		case ch <- ev:
		default:
			delete(b.subs, ch)
			close(ch)
		}
	}
	return nil
}

// Subscribe returns a channel of events after the given ID (0 for
// none), replaying the buffer first, and a cancel to unsubscribe. The
// channel is closed when the subscriber falls too far behind.
func (b *Broker) Subscribe(after uint64) (<-chan Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan Event, b.replay()+64)
	for _, ev := range b.buffer {
		if ev.ID > after {
			ch <- ev
		}
	}
	if b.subs == nil {
		b.subs = map[chan Event]struct{}{}
	}
	b.subs[ch] = struct{}{}
	cancel := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
	}
	return ch, cancel
}

// Serve is the handler of an event route (`get /events volt.Events`,
// §V4.11): a text/event-stream response that replays from the
// request's Last-Event-ID, then forwards every published event until
// the client goes away, with keep-alive comments in between.
func (b *Broker) Serve(w http.ResponseWriter, r *Request) error {
	flusher, _ := w.(http.Flusher)
	var after uint64
	if v := r.Header.Get("Last-Event-ID"); v != "" {
		after, _ = strconv.ParseUint(v, 10, 64)
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if flusher != nil {
		flusher.Flush()
	}

	ch, cancel := b.Subscribe(after)
	defer cancel()
	hb := b.Heartbeat
	if hb <= 0 {
		hb = 15 * time.Second
	}
	ticker := time.NewTicker(hb)
	defer ticker.Stop()
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-ch:
			if !ok {
				return nil // fell behind: the client reconnects and replays
			}
			if err := writeEvent(w, ev); err != nil {
				return nil // the client went away mid-write
			}
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return nil
			}
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
}

func writeEvent(w http.ResponseWriter, ev Event) error {
	var b strings.Builder
	fmt.Fprintf(&b, "id: %d\nevent: %s\n", ev.ID, ev.Name)
	for _, line := range strings.Split(string(ev.Data), "\n") {
		fmt.Fprintf(&b, "data: %s\n", line)
	}
	b.WriteString("\n")
	_, err := fmt.Fprint(w, b.String())
	return err
}

// Stream is the client side of an event route: it connects to url,
// delivers every event on the returned channel, and on any drop
// reconnects with Last-Event-ID and exponential backoff (1s to 30s)
// until ctx is done, when the channel closes. Generated clients wrap
// it as a typed Events method.
func (c *Client) Stream(ctx context.Context, url string) <-chan Event {
	out := make(chan Event, 64)
	go func() {
		defer close(out)
		var last uint64
		delay := time.Second
		for {
			ok := c.streamOnce(ctx, url, &last, out)
			if ctx.Err() != nil {
				return
			}
			if ok {
				delay = time.Second
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			if delay < 30*time.Second {
				delay *= 2
			}
		}
	}()
	return out
}

// streamOnce runs one connection; it reports whether any event
// arrived (so backoff resets), and updates last as events flow.
func (c *Client) streamOnce(ctx context.Context, url string, last *uint64, out chan<- Event) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", c.Base+url, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Accept", "text/event-stream")
	if *last > 0 {
		req.Header.Set("Last-Event-ID", strconv.FormatUint(*last, 10))
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	got := false
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	var ev Event
	var data []string
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			if ev.Name != "" || len(data) > 0 {
				ev.Data = json.RawMessage(strings.Join(data, "\n"))
				if ev.ID > *last {
					*last = ev.ID
				}
				got = true
				select {
				case out <- ev:
				case <-ctx.Done():
					return got
				}
			}
			ev, data = Event{}, nil
		case strings.HasPrefix(line, ":"):
			// comment / keep-alive
		case strings.HasPrefix(line, "id:"):
			ev.ID, _ = strconv.ParseUint(strings.TrimSpace(line[3:]), 10, 64)
		case strings.HasPrefix(line, "event:"):
			ev.Name = strings.TrimSpace(line[6:])
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line[5:], " "), ""))
		}
	}
	return got
}
