package byhand

import (
	"context"
	"database/sql"
	"encoding/gob"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/Piechutowski/volt/examples/tables/app"
	"github.com/Piechutowski/volt/nao/run"
)

type exchange struct {
	method, path, contentType, accept, body string
}

var exchanges = []exchange{
	{"GET", "/api/notes", "", "", ""},
	{"POST", "/api/notes", "application/json", "", `{"title":"Write the spec"}`},
	{"POST", "/api/notes", "application/json", "", `{}`},
	{"POST", "/api/notes", "application/json", "", `{"title":""}`},
	{"POST", "/api/notes", "application/json", "", `{"titel":"x"}`},
	{"POST", "/api/notes", "application/json", "", `{"title":5}`},
	{"POST", "/api/notes", "application/json", "", `{"title":null}`},
	{"POST", "/api/notes", "application/json", "", `{"id":99,"title":"Ship it","done":true}`},
	{"POST", "/api/notes", "application/json", "", `{`},
	{"POST", "/api/notes", "application/json", "", ``},
	{"POST", "/api/notes", "", "", `{"title":"no content type"}`},
	{"POST", "/api/notes", "text/plain", "", `title`},
	{"POST", "/api/notes", "application/json", "", `{"title":"a","done":"yes"}`},
	{"GET", "/api/notes", "", "text/html", ""},
	{"GET", "/api/notes", "", "application/json, text/html", ""},
	{"GET", "/api/notes/1", "", "", ""},
	{"GET", "/api/notes/999", "", "", ""},
	{"GET", "/api/notes/abc", "", "", ""},
	{"PATCH", "/api/notes/1", "application/json", "", `{"done":true}`},
	{"PATCH", "/api/notes/1", "application/json", "", `{"title":""}`},
	{"PATCH", "/api/notes/1", "application/json", "", `{}`},
	{"PATCH", "/api/notes/999", "application/json", "", `{"done":true}`},
	{"PATCH", "/api/notes/1", "application/json", "", `{"id":7}`},
	{"GET", "/api/notes", "", "", ""},
	{"GET", "/api/notes", "", "application/x-gob", ""},
	{"DELETE", "/api/notes/2", "", "", ""},
	{"DELETE", "/api/notes/2", "", "", ""},
	{"PUT", "/api/notes/1", "application/json", "", `{}`},
	{"GET", "/api/notes", "", "", ""},
	{"POST", "/api/notes", "application/json", "", `{"title":"` + strings.Repeat("x", 5<<20) + `"}`},
}

func replies(t *testing.T, h http.Handler) []string {
	t.Helper()
	var out []string
	for _, e := range exchanges {
		req := httptest.NewRequest(e.method, e.path, strings.NewReader(e.body))
		if e.contentType != "" {
			req.Header.Set("Content-Type", e.contentType)
		}
		if e.accept != "" {
			req.Header.Set("Accept", e.accept)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		length, body := rec.Header().Get("Content-Length"), rec.Body.String()
		if rec.Header().Get("Content-Type") == "application/x-gob" {
			var notes []Note
			if err := gob.NewDecoder(rec.Body).Decode(&notes); err != nil {
				t.Fatal(err)
			}
			length, body = "gob numbers its types per process", fmt.Sprintf("%+v", notes)
		}
		out = append(out, strings.Join([]string{e.method + " " + e.path, rec.Result().Status, rec.Header().Get("Content-Type"), length, rec.Header().Get("Allow"), body}, " | "))
	}
	return out
}

func generated(t *testing.T) http.Handler {
	t.Helper()
	d, err := run.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	ddl, err := os.ReadFile("../tables/db/volt_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool().Exec(string(ddl)); err != nil {
		t.Fatal(err)
	}
	return app.NewRouter(app.Controllers{DB: d})
}

func byHand(t *testing.T) (http.Handler, *sql.DB) {
	t.Helper()
	d, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	d.SetMaxOpenConns(1)
	if _, err := d.Exec(Schema); err != nil {
		t.Fatal(err)
	}
	return Handler(d), d
}

// TestRepliesMatchTheGenerated sends the hand-written server and the
// one volt gen writes for the tables example the same requests, good
// and bad, and requires byte-identical replies: status, Content-Type,
// Content-Length, Allow and body; a GOB body is compared decoded, since
// GOB numbers its types per process.
func TestRepliesMatchTheGenerated(t *testing.T) {
	h, _ := byHand(t)
	want, got := replies(t, generated(t)), replies(t, h)
	for i := range exchanges {
		if got[i] != want[i] {
			t.Errorf("exchange %d differs\n generated: %.400q\n by hand:   %.400q", i, want[i], got[i])
		}
	}
}

// TestClientRoundTrip drives the hand-written client against the
// hand-written server in both formats: create, a refused create, list,
// update and delete.
func TestClientRoundTrip(t *testing.T) {
	for _, useGOB := range []bool{false, true} {
		h, _ := byHand(t)
		srv := httptest.NewServer(h)
		c, ctx := &Client{Base: srv.URL, GOB: useGOB}, context.Background()
		title, done := "Write the spec", true
		n, err := c.CreateNote(ctx, NoteFields{Title: &title})
		if err != nil || n.ID == 0 || n.Done {
			t.Fatalf("gob %v: create = %+v, %v", useGOB, n, err)
		}
		var pe *ProblemError
		if _, err := c.CreateNote(ctx, NoteFields{}); !errors.As(err, &pe) || pe.Status != 422 || len(pe.Details) != 1 {
			t.Errorf("gob %v: a note without a title = %v", useGOB, err)
		}
		if n, err = c.UpdateNote(ctx, n.ID, NoteFields{Done: &done}); err != nil || !n.Done || n.Title != title {
			t.Errorf("gob %v: update = %+v, %v", useGOB, n, err)
		}
		if notes, err := c.Notes(ctx); err != nil || len(notes) != 1 || notes[0] != n {
			t.Errorf("gob %v: list = %+v, %v", useGOB, notes, err)
		}
		if err := c.DeleteNote(ctx, n.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Note(ctx, n.ID); !errors.Is(err, ErrNotFound) {
			t.Errorf("gob %v: a deleted note = %v, want not found", useGOB, err)
		}
		srv.Close()
	}
}
