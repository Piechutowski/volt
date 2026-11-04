// Package app is the blog's routes: routes.volt declares them, and the
// Go files of this package hold the member load, the error handler, the
// request bodies and the controllers, which enforce the blog's rules.
package app

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/compare/volt/db"
	"github.com/Piechutowski/volt/nao/rt"
)

// PostForm is the body of a new post.
type PostForm struct {
	Slug  string   `json:"slug"`
	Title string   `json:"title"`
	Body  string   `json:"body"`
	Tags  []string `json:"tags"`
}

// PostEdit is the body of an edit; a nil field is left alone.
type PostEdit struct {
	Title *string `json:"title"`
	Body  *string `json:"body"`
}

// CommentForm is the body of a new comment.
type CommentForm struct {
	Body string `json:"body"`
}

// CurrentMember loads the member X-User names (§scope-loads), answering
// a member that does not exist with 401 in the load itself.
func CurrentMember(r volt.Request) volt.Load[db.Member] {
	id, err := strconv.Atoi(r.Header().Get("X-User"))
	if err != nil {
		return volt.Halt[db.Member](volt.Reject(http.StatusUnauthorized, "send X-User: a member id"))
	}
	return volt.LoadWith(db.MemberGet(id)).Rescue(func(err error) volt.Response {
		if errors.Is(err, rt.ErrNotFound) {
			return volt.Reject(http.StatusUnauthorized, "no such member")
		}
		return volt.Fail(err)
	})
}

// Errors answers every failure with the default mapping (§scopes).
func Errors(r volt.Request, err error) volt.Response {
	return volt.DefaultErrorHandler(r, err)
}
