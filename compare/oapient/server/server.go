// Package server implements the blog's generated strict server interface
// on an Ent client and serves it behind the member lookup and the
// OpenAPI request validator.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	middleware "github.com/oapi-codegen/nethttp-middleware"

	"github.com/Piechutowski/volt/compare/oapient/api"
	"github.com/Piechutowski/volt/compare/oapient/ent"
)

// Server answers every operation of openapi.yaml from the database.
type Server struct {
	db *ent.Client
}

var _ api.StrictServerInterface = (*Server)(nil)

// Handler serves the blog on db: X-User is resolved to a member, the
// request is validated against the embedded OpenAPI document, and the
// strict handlers answer it.
func Handler(db *ent.Client) (http.Handler, error) {
	spec, err := api.GetSwagger()
	if err != nil {
		return nil, err
	}
	spec.Servers = nil
	strict := api.NewStrictHandlerWithOptions(&Server{db: db}, nil, api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			refuse(w, http.StatusBadRequest, err.Error(), "")
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			refuse(w, http.StatusInternalServerError, err.Error(), "")
		},
	})
	routes := api.HandlerWithOptions(strict, api.StdHTTPServerOptions{
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			refuse(w, http.StatusUnprocessableEntity, err.Error(), "")
		},
	})
	validate := middleware.OapiRequestValidatorWithOptions(spec, &middleware.Options{
		Options:              openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
		ErrorHandlerWithOpts: invalid,
	})
	return members(db, validate(routes)), nil
}

type memberKey struct{}

func members(db *ent.Client, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.Header.Get("X-User"))
		if err != nil {
			refuse(w, http.StatusUnauthorized, "send X-User: a member id", "")
			return
		}
		m, err := db.Member.Get(r.Context(), id)
		switch {
		case ent.IsNotFound(err):
			refuse(w, http.StatusUnauthorized, "no such member", "")
			return
		case err != nil:
			refuse(w, http.StatusInternalServerError, err.Error(), "")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), memberKey{}, m)))
	})
}

func member(ctx context.Context) *ent.Member {
	return ctx.Value(memberKey{}).(*ent.Member)
}

func invalid(_ context.Context, err error, w http.ResponseWriter, _ *http.Request, opts middleware.ErrorHandlerOpts) {
	var re *openapi3filter.RequestError
	switch {
	case errors.Is(err, routers.ErrMethodNotAllowed):
		refuse(w, http.StatusMethodNotAllowed, err.Error(), "")
		return
	case !errors.As(err, &re):
		refuse(w, opts.StatusCode, err.Error(), "")
		return
	}
	if re.Parameter != nil {
		reason, _, _ := strings.Cut(re.Error(), "\n")
		refuse(w, http.StatusUnprocessableEntity, reason, re.Parameter.Name)
		return
	}
	var se *openapi3.SchemaError
	if !errors.As(re.Err, &se) {
		refuse(w, http.StatusBadRequest, re.Error(), "")
		return
	}
	field := strings.Join(se.JSONPointer(), ".")
	if field == "" {
		refuse(w, http.StatusUnprocessableEntity, se.Reason, "")
		return
	}
	refuse(w, http.StatusUnprocessableEntity, field+": "+se.Reason, field)
}

func refuse(w http.ResponseWriter, status int, message, field string) {
	body := api.Error{Message: message}
	if field != "" {
		body.Field = &field
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func refusal(err error) api.Error {
	var ve *ent.ValidationError
	if errors.As(err, &ve) {
		return api.Error{Message: err.Error(), Field: &ve.Name}
	}
	return api.Error{Message: err.Error()}
}

func withTx(ctx context.Context, db *ent.Client, fn func(tx *ent.Tx) error) error {
	tx, err := db.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if v := recover(); v != nil {
			_ = tx.Rollback()
			panic(v)
		}
	}()
	if err := fn(tx); err != nil {
		if rerr := tx.Rollback(); rerr != nil {
			err = fmt.Errorf("%w: rolling back transaction: %v", err, rerr)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}
