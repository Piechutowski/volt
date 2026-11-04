// Race sends the five servers of the comparison the same concurrent
// requests and counts the rules each breaks, the probe the README's
// concurrency paragraph reports: every round publishes one draft twenty
// times at once, comments on it ten times at once as one member, whom
// the rules allow three, and races sixteen edits to a short body against
// four publishes of a fresh draft, which may only be published with a
// long one. Counts change from run to run, since they are races; GORM's
// log of the writes SQLite refuses is dropped, the replies counting them.
// After go generate ./compare/..., run
//
//	go run ./compare/race [-rounds 30] [stack]
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/Piechutowski/volt/compare/chisqlc"
	"github.com/Piechutowski/volt/compare/echosqlx"
	"github.com/Piechutowski/volt/compare/gingorm"
	"github.com/Piechutowski/volt/compare/oapient"
	blog "github.com/Piechutowski/volt/compare/volt"
)

type stack struct {
	name    string
	handler func(string) (http.Handler, error)
}

var stacks = []stack{
	{"Volt", blog.Handler},
	{"Gin + GORM", gingorm.Handler},
	{"chi + sqlc + goose", chisqlc.Handler},
	{"Echo + sqlx + goose", echosqlx.Handler},
	{"oapi-codegen + Ent", oapient.Handler},
}

type reply struct {
	status int
	body   []byte
}

type probe struct {
	base   string
	client *http.Client
}

func (p probe) send(method, path string, body any) reply {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, p.base+path, r)
	if err != nil {
		panic(err)
	}
	req.Header.Set("X-User", "1")
	if strings.HasSuffix(path, "/comments") {
		req.Header.Set("X-User", "2")
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return reply{status: -1, body: []byte(err.Error())}
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return reply{status: resp.StatusCode, body: b}
}

func (p probe) draftCreate(slug string) (int, error) {
	long := strings.Repeat("x", 120)
	r := p.send("POST", "/posts", map[string]any{"slug": slug, "title": "Race", "body": long, "tags": []string{}})
	var post struct {
		ID int `json:"id"`
	}
	if r.status != http.StatusCreated || json.Unmarshal(r.body, &post) != nil {
		return 0, fmt.Errorf("create %s: %d %s", slug, r.status, r.body)
	}
	return post.ID, nil
}

func concurrently(n int, call func(i int) reply) []reply {
	out := make([]reply, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			<-start
			out[i] = call(i)
		})
	}
	close(start)
	wg.Wait()
	return out
}

type tally struct {
	publishedTwice, commentsOver, commentsUnder, shortPublished, failed int
}

func statusCount(rs []reply, status int) int {
	n := 0
	for _, r := range rs {
		if r.status == status {
			n++
		}
	}
	return n
}

func (t *tally) failures(rs []reply) {
	for _, r := range rs {
		if r.status >= 500 || r.status < 0 {
			t.failed++
		}
	}
}

func (p probe) round(t *tally, round int) error {
	id, err := p.draftCreate(fmt.Sprintf("race-%d", round))
	if err != nil {
		return err
	}
	publishes := concurrently(20, func(int) reply { return p.send("POST", fmt.Sprintf("/posts/%d/publish", id), nil) })
	t.failures(publishes)
	if statusCount(publishes, http.StatusOK) > 1 {
		t.publishedTwice++
	}
	comments := concurrently(10, func(int) reply {
		return p.send("POST", fmt.Sprintf("/posts/%d/comments", id), map[string]any{"body": "Nice"})
	})
	t.failures(comments)
	var stored []json.RawMessage
	_ = json.Unmarshal(p.send("GET", fmt.Sprintf("/posts/%d/comments", id), nil).body, &stored)
	switch {
	case len(stored) > 3:
		t.commentsOver++
	case len(stored) < 3:
		t.commentsUnder++
	}
	id, err = p.draftCreate(fmt.Sprintf("edit-race-%d", round))
	if err != nil {
		return err
	}
	t.failures(concurrently(20, func(i int) reply {
		if i%5 == 4 {
			return p.send("POST", fmt.Sprintf("/posts/%d/publish", id), nil)
		}
		return p.send("PATCH", fmt.Sprintf("/posts/%d", id), map[string]any{"body": "short"})
	}))
	var after struct {
		State string `json:"state"`
		Body  string `json:"body"`
	}
	_ = json.Unmarshal(p.send("GET", fmt.Sprintf("/posts/%d", id), nil).body, &after)
	if after.State == "published" && len(after.Body) < 100 {
		t.shortPublished++
	}
	return nil
}

func main() {
	rounds := flag.Int("rounds", 30, "rounds per stack")
	flag.Parse()
	gin.SetMode(gin.ReleaseMode)
	log.SetOutput(io.Discard)
	dir, err := os.MkdirTemp("", "race")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(dir)
	quiet, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		fail(err)
	}
	defer quiet.Close()
	stderr := os.Stderr
	fmt.Println("| Stack | Rounds | Published more than once | More than three comments | Fewer than three comments | Published with a short body | Replies 5xx or none |")
	fmt.Println("|---|---:|---:|---:|---:|---:|---:|")
	for i, s := range stacks {
		if flag.NArg() > 0 && !strings.Contains(strings.ToLower(s.name), strings.ToLower(flag.Arg(0))) {
			continue
		}
		os.Stderr = quiet
		h, err := s.handler(filepath.Join(dir, fmt.Sprintf("blog%d.db", i)))
		os.Stderr = stderr
		if err != nil {
			fail(fmt.Errorf("%s: %w", s.name, err))
		}
		srv := httptest.NewServer(h)
		p := probe{base: srv.URL, client: &http.Client{Transport: &http.Transport{MaxIdleConnsPerHost: 64}}}
		var t tally
		for round := range *rounds {
			if err := p.round(&t, round); err != nil {
				srv.Close()
				fail(fmt.Errorf("%s: %w", s.name, err))
			}
		}
		srv.Close()
		fmt.Printf("| %s | %d | %d | %d | %d | %d | %d |\n", s.name, *rounds, t.publishedTwice, t.commentsOver, t.commentsUnder, t.shortPublished, t.failed)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "race:", err)
	os.Exit(1)
}
