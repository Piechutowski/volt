// Package server is the blog's HTTP API: a chi router whose handlers
// enforce the blog's rules over the queries sqlc generates.
package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mattn/go-sqlite3"

	"github.com/Piechutowski/volt/compare/chisqlc/api"
	"github.com/Piechutowski/volt/compare/chisqlc/db"
)

type server struct {
	db *sql.DB
	q  *db.Queries
}

// New returns the blog's routes over the migrated database conn.
func New(conn *sql.DB) http.Handler {
	s := &server{db: conn, q: db.New(conn)}
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(s.memberCtx)
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	})

	r.Route("/posts", func(r chi.Router) {
		r.Post("/", s.createPost)
		r.Get("/", s.feed)
		r.Route("/{id}", func(r chi.Router) {
			r.Use(s.postCtx)
			r.Get("/", s.getPost)
			r.Patch("/", s.editPost)
			r.Post("/publish", s.publishPost)
			r.Get("/audit", s.listAudits)
			r.Post("/comments", s.createComment)
			r.Get("/comments", s.listComments)
			r.Delete("/comments/{cid}", s.deleteComment)
		})
	})
	r.Get("/me/stats", s.stats)
	return r
}

type ctxKey int

const (
	memberKey ctxKey = iota
	postKey
)

func (s *server) memberCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.Header.Get("X-User"), 10, 64)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "X-User must be a member id")
			return
		}
		m, err := s.q.GetMember(r.Context(), id)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, "no such member")
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), memberKey, m)))
	})
}

func (s *server) postCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusNotFound, "post not found")
			return
		}
		p, err := s.q.GetPost(r.Context(), id)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "post not found")
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), postKey, p)))
	})
}

func memberFrom(r *http.Request) db.Member { return r.Context().Value(memberKey).(db.Member) }

func postFrom(r *http.Request) db.Post { return r.Context().Value(postKey).(db.Post) }

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "malformed JSON body: "+err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writing a reply: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, api.Error{Message: message})
}

func writeInvalid(w http.ResponseWriter, fields map[string]string) {
	writeJSON(w, http.StatusUnprocessableEntity, api.Error{Message: "invalid fields", Fields: fields})
}

func serverError(w http.ResponseWriter, err error) {
	log.Printf("internal error: %v", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}

func isUniqueViolation(err error) bool {
	var se sqlite3.Error
	return errors.As(err, &se) && se.ExtendedCode == sqlite3.ErrConstraintUnique
}
