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
	"sync/atomic"
	"time"
)

// Event is one server-sent event (spec §event-routes): a name the client
// dispatches on and a JSON payload, numbered in publication order by the
// broker its Epoch names, so the pair tells a restart and deduplicates.
// Replayed marks an event delivered from the replay buffer, published
// before the stream opened.
type Event struct {
	ID       uint64
	Epoch    string
	Name     string
	Data     json.RawMessage
	Replayed bool
}

// WireID is the event's ID as Serve writes it, the broker's epoch, a dash
// and the position (spec §event-routes).
func (e Event) WireID() string { return e.Epoch + "-" + strconv.FormatUint(e.ID, 10) }

// Decode unmarshals the payload into v.
func (e Event) Decode(v any) error { return json.Unmarshal(e.Data, v) }

// Broker fans events out to every open event-stream connection and
// keeps the last Replay events so a reconnecting client resumes from
// its Last-Event-ID. One process, in memory.
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
	epoch  string
}

var epochLast atomic.Int64

func (b *Broker) epochOf() string {
	if b.epoch == "" {
		for {
			old := epochLast.Load()
			n := max(time.Now().UnixNano(), old+1)
			if epochLast.CompareAndSwap(old, n) {
				b.epoch = strconv.FormatInt(n, 36)
				break
			}
		}
	}
	return b.epoch
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
// dropped.
func (b *Broker) Publish(name string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.next++
	ev := Event{ID: b.next, Epoch: b.epochOf(), Name: name, Data: raw}
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

// Subscribe returns a channel of events after the given position (0 for
// none), replaying the buffer first, each replayed event marked so, and
// a cancel to unsubscribe. A position this broker never assigned
// replays the whole buffer. The channel is closed when the subscriber
// falls too far behind.
func (b *Broker) Subscribe(after uint64) (<-chan Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.subscribe(after)
}

// SubscribeAfter is Subscribe from a Last-Event-ID as Serve writes IDs
// (spec §event-routes): an ID of this broker resumes after it, and any
// other, none, one of an earlier process or a bare position, replays
// the whole buffer, so a client never misses what this process
// published.
func (b *Broker) SubscribeAfter(lastEventID string) (<-chan Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var after uint64
	if epoch, n, ok := strings.Cut(lastEventID, "-"); ok && epoch == b.epochOf() {
		after, _ = strconv.ParseUint(n, 10, 64)
	}
	return b.subscribe(after)
}

func (b *Broker) subscribe(after uint64) (<-chan Event, func()) {
	if after > b.next {
		after = 0
	}
	ch := make(chan Event, b.replay()+64)
	for _, ev := range b.buffer {
		if ev.ID > after {
			ev.Replayed = true
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
// §event-routes): a text/event-stream response that replays from the
// request's Last-Event-ID, then forwards every published event until
// the client goes away, with keep-alive comments in between.
func (b *Broker) Serve(w http.ResponseWriter, r *http.Request) error {
	flusher, _ := w.(http.Flusher)
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if flusher != nil {
		flusher.Flush()
	}

	ch, cancel := b.SubscribeAfter(r.Header.Get("Last-Event-ID"))
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
				return nil
			}
			if err := writeEvent(w, ev); err != nil {
				return nil
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
	if ev.Replayed {
		b.WriteString(": replayed\n")
	}
	fmt.Fprintf(&b, "id: %s\nevent: %s\n", ev.WireID(), ev.Name)
	for _, line := range strings.Split(string(ev.Data), "\n") {
		fmt.Fprintf(&b, "data: %s\n", line)
	}
	b.WriteString("\n")
	_, err := fmt.Fprint(w, b.String())
	return err
}

// Stream is the client side of an event route: it connects to url,
// delivers every event on the returned channel, and on any drop
// reconnects with exponential backoff (1s to 30s), sending back as
// Last-Event-ID the ID of the last event it delivered, until ctx is
// done, when the channel closes. It reads the client, Header among it,
// once when called (spec §generated-client).
func (c *Client) Stream(ctx context.Context, url string) <-chan Event {
	snapshot := *c
	snapshot.Header = c.Header.Clone()
	out := make(chan Event, 64)
	go func() {
		defer close(out)
		var last string
		delay := time.Second
		for {
			ok := snapshot.streamOnce(ctx, url, &last, out)
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

func (c *Client) streamOnce(ctx context.Context, url string, last *string, out chan<- Event) bool {
	req, err := c.requestNew(ctx, "GET", url, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Accept", "text/event-stream")
	if *last != "" {
		req.Header.Set("Last-Event-ID", *last)
	}
	resp, err := c.httpClient().Do(req)
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
	var id string
	var data []string
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			if ev.Name != "" || len(data) > 0 {
				ev.Data = json.RawMessage(strings.Join(data, "\n"))
				if id != "" {
					*last = id
				}
				got = true
				select {
				case out <- ev:
				case <-ctx.Done():
					return got
				}
			}
			ev, id, data = Event{}, "", nil
		case line == ": replayed":
			ev.Replayed = true
		case strings.HasPrefix(line, ":"):
		case strings.HasPrefix(line, "id:"):
			id = strings.TrimSpace(line[3:])
			n := id
			if i := strings.LastIndexByte(id, '-'); i >= 0 {
				ev.Epoch, n = id[:i], id[i+1:]
			}
			ev.ID, _ = strconv.ParseUint(n, 10, 64)
		case strings.HasPrefix(line, "event:"):
			ev.Name = strings.TrimSpace(line[6:])
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line[5:], " "), ""))
		}
	}
	return got
}

// EventsHandler is the http.Handler of an event route (§event-routes):
// the broker's Serve, or 503 when the application configured none.
func EventsHandler(b *Broker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if b == nil {
			http.Error(w, "no event broker configured", http.StatusServiceUnavailable)
			return
		}
		_ = b.Serve(w, r)
	})
}
