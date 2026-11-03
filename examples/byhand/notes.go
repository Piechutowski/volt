// Package byhand is the tables example's server and client written by
// hand on the standard library, with the behaviour volt gen gives the
// generated one, so the two can be compared line for line. The parity
// test sends both the same requests and requires the same replies.
package byhand

import (
	"context"
	"database/sql"
	"errors"
)

// Schema is the DDL of the notes table.
const Schema = `CREATE TABLE notes (
  id INTEGER PRIMARY KEY,
  title TEXT NOT NULL,
  done INTEGER NOT NULL DEFAULT 0
);`

// Note is one row of notes.
type Note struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

// NoteFields are the columns a write sets; a nil field is left alone.
type NoteFields struct {
	Title *string
	Done  *bool
}

// ErrNotFound is a missing note.
var ErrNotFound = errors.New("not found")

const noteColumns = `id, title, done`

func noteScan(row interface{ Scan(...any) error }) (Note, error) {
	var n Note
	err := row.Scan(&n.ID, &n.Title, &n.Done)
	if errors.Is(err, sql.ErrNoRows) {
		return n, ErrNotFound
	}
	return n, err
}

func noteList(ctx context.Context, db *sql.DB) ([]Note, error) {
	rows, err := db.QueryContext(ctx, `SELECT `+noteColumns+` FROM notes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	notes := []Note{}
	for rows.Next() {
		n, err := noteScan(rows)
		if err != nil {
			return nil, err
		}
		notes = append(notes, n)
	}
	return notes, rows.Err()
}

func noteGet(ctx context.Context, db *sql.DB, id int) (Note, error) {
	return noteScan(db.QueryRowContext(ctx, `SELECT `+noteColumns+` FROM notes WHERE id = ?`, id))
}

func noteCreate(ctx context.Context, db *sql.DB, n Note) (Note, error) {
	return noteScan(db.QueryRowContext(ctx, `INSERT INTO notes (title, done) VALUES (?, ?) RETURNING `+noteColumns, n.Title, n.Done))
}

func noteUpdate(ctx context.Context, db *sql.DB, n Note) (Note, error) {
	return noteScan(db.QueryRowContext(ctx, `UPDATE notes SET title = ?, done = ? WHERE id = ? RETURNING `+noteColumns, n.Title, n.Done, n.ID))
}

func noteDelete(ctx context.Context, db *sql.DB, id int) error {
	res, err := db.ExecContext(ctx, `DELETE FROM notes WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		return ErrNotFound
	}
	return err
}
