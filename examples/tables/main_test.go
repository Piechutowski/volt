package main

import (
	"context"
	"errors"
	"testing"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/tables/app/client"
	"github.com/Piechutowski/volt/examples/tables/db"
)

// TestNotesRoundTrip drives the routes resources [default] generates
// through the generated client: create, a refused create, list, update
// and delete.
func TestNotesRoundTrip(t *testing.T) {
	base, err := serve()
	if err != nil {
		t.Fatal(err)
	}
	c, ctx := client.New(base), context.Background()
	n, err := c.CreateNote(ctx, db.NoteNew().TitleSet("Write the spec"))
	if err != nil || n.ID == 0 || n.Done {
		t.Fatalf("create = %+v, %v", n, err)
	}
	var pe *volt.ProblemError
	if _, err := c.CreateNote(ctx, db.NoteNew()); !errors.As(err, &pe) || pe.Status != 422 {
		t.Errorf("a note without a title = %v, want a 422", err)
	}
	if n, err = c.UpdateNote(ctx, n.ID, db.NoteNew().DoneSet(true)); err != nil || !n.Done || n.Title != "Write the spec" {
		t.Errorf("update = %+v, %v", n, err)
	}
	if notes, err := c.Notes(ctx); err != nil || len(notes) != 1 || notes[0] != n {
		t.Errorf("list = %+v, %v", notes, err)
	}
	if err := c.DeleteNote(ctx, n.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Note(ctx, n.ID); !errors.Is(err, volt.ErrNotFound) {
		t.Errorf("a deleted note = %v, want not found", err)
	}
}
