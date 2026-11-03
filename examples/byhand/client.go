package byhand

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client calls the notes API at Base, in JSON unless GOB is set.
type Client struct {
	Base string
	HTTP *http.Client
	GOB  bool
}

// ProblemError is a refusal the server answered.
type ProblemError struct {
	Problem
}

func (e *ProblemError) Error() string { return e.Message }

// Is makes errors.Is(err, ErrNotFound) hold for a 404.
func (e *ProblemError) Is(target error) bool {
	return target == ErrNotFound && e.Status == http.StatusNotFound
}

// Notes lists every note.
func (c *Client) Notes(ctx context.Context) ([]Note, error) {
	var out []Note
	return out, c.do(ctx, "GET", "/api/notes", nil, &out)
}

// CreateNote stores a note from the fields set.
func (c *Client) CreateNote(ctx context.Context, f NoteFields) (Note, error) {
	var out Note
	return out, c.do(ctx, "POST", "/api/notes", &f, &out)
}

// Note reads one note.
func (c *Client) Note(ctx context.Context, id int) (Note, error) {
	var out Note
	return out, c.do(ctx, "GET", fmt.Sprintf("/api/notes/%d", id), nil, &out)
}

// UpdateNote sets the fields set on note id.
func (c *Client) UpdateNote(ctx context.Context, id int, f NoteFields) (Note, error) {
	var out Note
	return out, c.do(ctx, "PATCH", fmt.Sprintf("/api/notes/%d", id), &f, &out)
}

// DeleteNote deletes note id.
func (c *Client) DeleteNote(ctx context.Context, id int) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/api/notes/%d", id), nil, nil)
}

func (c *Client) do(ctx context.Context, method, path string, in *NoteFields, out any) error {
	format := formatJSON
	if c.GOB {
		format = formatGOB
	}
	var body io.Reader
	if in != nil {
		var b bytes.Buffer
		var err error
		if c.GOB {
			err = gob.NewEncoder(&b).Encode(*in)
		} else {
			err = json.NewEncoder(&b).Encode(fieldsJSON(*in))
		}
		if err != nil {
			return err
		}
		body = &b
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", format)
	if body != nil {
		req.Header.Set("Content-Type", format)
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	decode := func(v any) error {
		if strings.HasPrefix(resp.Header.Get("Content-Type"), formatGOB) {
			return gob.NewDecoder(bytes.NewReader(data)).Decode(v)
		}
		return json.Unmarshal(data, v)
	}
	if resp.StatusCode >= 300 {
		pe := &ProblemError{}
		if err := decode(&pe.Problem); err != nil || pe.Status == 0 {
			pe.Problem = Problem{Status: resp.StatusCode, Message: strings.TrimSpace(string(data))}
		}
		return pe
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return decode(out)
}

func fieldsJSON(f NoteFields) map[string]any {
	m := map[string]any{}
	if f.Title != nil {
		m["title"] = *f.Title
	}
	if f.Done != nil {
		m["done"] = *f.Done
	}
	return m
}
