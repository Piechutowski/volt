package byhand

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

const (
	formatJSON = "application/json"
	formatGOB  = "application/x-gob"
	bodyLimit  = 4 << 20
)

// Problem is the body of every refusal.
type Problem struct {
	Status  int      `json:"status"`
	Message string   `json:"message"`
	Details []Detail `json:"details,omitempty"`
}

// Detail is one field error of a Problem.
type Detail struct {
	Check   string   `json:"check"`
	Columns []string `json:"columns,omitempty"`
	Message string   `json:"message"`
}

// Handler is the notes API on db: list, create, show, update and delete.
func Handler(db *sql.DB) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/notes", func(w http.ResponseWriter, r *http.Request) {
		format, ok := accepted(w, r)
		if !ok {
			return
		}
		notes, err := noteList(r.Context(), db)
		reply(w, format, http.StatusOK, notes, err)
	})
	mux.HandleFunc("POST /api/notes", func(w http.ResponseWriter, r *http.Request) {
		format, ok := accepted(w, r)
		if !ok {
			return
		}
		fields, details, ok := fieldsRead(w, r, format)
		if !ok {
			return
		}
		n, details := noteApply(Note{}, fields, details)
		if len(details) > 0 {
			refuse(w, format, Problem{Status: http.StatusUnprocessableEntity, Message: detailsMessage(details), Details: details})
			return
		}
		n, err := noteCreate(r.Context(), db, n)
		reply(w, format, http.StatusCreated, n, err)
	})
	mux.HandleFunc("GET /api/notes/{id}", func(w http.ResponseWriter, r *http.Request) {
		format, ok := accepted(w, r)
		if !ok {
			return
		}
		id, ok := idRead(w, r, format)
		if !ok {
			return
		}
		n, err := noteGet(r.Context(), db, id)
		reply(w, format, http.StatusOK, n, err)
	})
	mux.HandleFunc("PATCH /api/notes/{id}", func(w http.ResponseWriter, r *http.Request) {
		format, ok := accepted(w, r)
		if !ok {
			return
		}
		id, ok := idRead(w, r, format)
		if !ok {
			return
		}
		fields, details, ok := fieldsRead(w, r, format)
		if !ok {
			return
		}
		n, err := noteGet(r.Context(), db, id)
		if err != nil {
			reply(w, format, 0, nil, err)
			return
		}
		n, details = noteApply(n, fields, details)
		if len(details) > 0 {
			refuse(w, format, Problem{Status: http.StatusUnprocessableEntity, Message: detailsMessage(details), Details: details})
			return
		}
		n, err = noteUpdate(r.Context(), db, n)
		reply(w, format, http.StatusOK, n, err)
	})
	mux.HandleFunc("DELETE /api/notes/{id}", func(w http.ResponseWriter, r *http.Request) {
		format, ok := accepted(w, r)
		if !ok {
			return
		}
		id, ok := idRead(w, r, format)
		if !ok {
			return
		}
		if err := noteDelete(r.Context(), db, id); err != nil {
			reply(w, format, 0, nil, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func accepted(w http.ResponseWriter, r *http.Request) (string, bool) {
	header := r.Header.Get("Accept")
	if header == "" {
		return formatJSON, true
	}
	for _, part := range strings.Split(header, ",") {
		media, _, _ := strings.Cut(strings.TrimSpace(part), ";")
		switch strings.TrimSpace(media) {
		case formatJSON, "application/*", "*/*":
			return formatJSON, true
		case formatGOB:
			return formatGOB, true
		}
	}
	refuse(w, formatJSON, Problem{Status: http.StatusNotAcceptable, Message: "acceptable formats: application/json, application/x-gob"})
	return "", false
}

func idRead(w http.ResponseWriter, r *http.Request, format string) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		refuse(w, format, Problem{Status: http.StatusNotFound, Message: "not found"})
		return 0, false
	}
	return id, true
}

func fieldsRead(w http.ResponseWriter, r *http.Request, format string) (NoteFields, []Detail, bool) {
	var fields NoteFields
	media := formatJSON
	if ct := r.Header.Get("Content-Type"); ct != "" {
		m, _, err := mime.ParseMediaType(ct)
		if err != nil || (m != formatJSON && m != formatGOB) {
			refuse(w, format, Problem{Status: http.StatusUnsupportedMediaType, Message: "supported request formats: application/json, application/x-gob"})
			return fields, nil, false
		}
		media = m
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, bodyLimit))
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		refuse(w, format, Problem{Status: http.StatusRequestEntityTooLarge, Message: "request body over the limit"})
		return fields, nil, false
	case err != nil:
		refuse(w, format, Problem{Status: http.StatusBadRequest, Message: err.Error()})
		return fields, nil, false
	case len(body) == 0:
		refuse(w, format, Problem{Status: http.StatusBadRequest, Message: "malformed " + media + " body: empty body"})
		return fields, nil, false
	}
	if media == formatGOB {
		if err := gob.NewDecoder(bytes.NewReader(body)).Decode(&fields); err != nil {
			refuse(w, format, Problem{Status: http.StatusBadRequest, Message: "malformed " + media + " body: " + err.Error()})
			return fields, nil, false
		}
		return fields, nil, true
	}
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&raw); err != nil {
		refuse(w, format, Problem{Status: http.StatusBadRequest, Message: "malformed " + media + " body: " + err.Error()})
		return fields, nil, false
	}
	var details []Detail
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if k != "id" && k != "title" && k != "done" {
			details = append(details, Detail{Check: "unknown_field", Columns: []string{k}, Message: "unknown field " + strconv.Quote(k)})
		}
	}
	if v, ok := raw["title"]; ok {
		var title string
		if d, ok := fieldCast(v, &title, "title", "string"); ok {
			fields.Title = &title
		} else {
			details = append(details, d)
		}
	}
	if v, ok := raw["done"]; ok {
		var done bool
		if d, ok := fieldCast(v, &done, "done", "bool"); ok {
			fields.Done = &done
		} else {
			details = append(details, d)
		}
	}
	return fields, details, true
}

func fieldCast(v json.RawMessage, into any, column, typ string) (Detail, bool) {
	if string(bytes.TrimSpace(v)) == "null" {
		return Detail{Check: "cast", Columns: []string{column}, Message: column + ": null is not allowed (" + typ + ")"}, false
	}
	if err := json.Unmarshal(v, into); err != nil {
		return Detail{Check: "cast", Columns: []string{column}, Message: column + ": expected " + typ}, false
	}
	return Detail{}, true
}

func noteApply(n Note, fields NoteFields, details []Detail) (Note, []Detail) {
	if fields.Title != nil {
		n.Title = *fields.Title
	}
	if fields.Done != nil {
		n.Done = *fields.Done
	}
	if n.Title == "" {
		details = append(details, Detail{Check: "required", Columns: []string{"title"}, Message: "title is required"})
	}
	return n, details
}

func detailsMessage(details []Detail) string {
	parts := make([]string, len(details))
	for i, d := range details {
		parts[i] = strings.Join(d.Columns, ", ") + ": " + d.Message
	}
	return "Note: " + strings.Join(parts, "; ")
}

func reply(w http.ResponseWriter, format string, status int, v any, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		refuse(w, format, Problem{Status: http.StatusNotFound, Message: "not found"})
	case err != nil:
		refuse(w, format, Problem{Status: http.StatusInternalServerError, Message: "internal server error"})
	default:
		write(w, format, status, v)
	}
}

func refuse(w http.ResponseWriter, format string, p Problem) {
	write(w, format, p.Status, p)
}

func write(w http.ResponseWriter, format string, status int, v any) {
	var b bytes.Buffer
	var err error
	if format == formatGOB {
		err = gob.NewEncoder(&b).Encode(v)
	} else {
		err = json.NewEncoder(&b).Encode(v)
	}
	if err != nil {
		status, format = http.StatusInternalServerError, formatJSON
		b.Reset()
		json.NewEncoder(&b).Encode(Problem{Status: status, Message: "internal server error"})
	}
	w.Header().Set("Content-Type", format)
	w.Header().Set("Content-Length", strconv.Itoa(b.Len()))
	w.WriteHeader(status)
	w.Write(b.Bytes())
}
