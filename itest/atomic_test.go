package itest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/itest/blog/app"
	"github.com/Piechutowski/volt/itest/blog/db"
	"github.com/Piechutowski/volt/nao/rt"
	"github.com/Piechutowski/volt/nao/run"
)

func signupHandler(email string, bios ...string) volt.HandlerFunc {
	return func(r volt.Request) volt.Response {
		return volt.Then(rt.Atomic(rt.Then(db.UserCreate(db.UserNew().EmailSet(email)), func(u db.User) rt.Effect[db.User] {
			profiles := make([]rt.AnyEffect, len(bios))
			for i, bio := range bios {
				profiles[i] = db.ProfileCreate(db.ProfileNew().UserIDSet(u.ID).BioSet(bio))
			}
			return rt.Map(rt.Tx(profiles...), func([]any) db.User { return u })
		})), func(u db.User) volt.Response { return volt.Created(u) })
	}
}

func relinkHandler(email, next string) volt.HandlerFunc {
	return func(r volt.Request) volt.Response {
		return volt.Then(rt.Atomic(rt.Then(db.UserCreate(db.UserNew().EmailSet(email)), func(db.User) rt.Effect[db.User] {
			return db.UserCreate(db.UserNew().EmailSet(next))
		})), func(u db.User) volt.Response { return volt.Created(u) })
	}
}

// TestAtomicThroughTheShim proves a handler whose continuation performs
// an rt.Atomic over an rt.Then (spec §requests-and-responses,
// §generated-query-surface): the shim performs it as one effect, a user
// and the profile that carries its new key commit together, a child the
// database refuses rolls the user back and answers 409 through the
// route's error handler, a step its changeset refuses 422, and two
// calls describe the same effect whatever next would make of it.
func TestAtomicThroughTheShim(t *testing.T) {
	ctx := context.Background()
	dbh := fixtureDB()
	defer dbh.Close()
	serveSignup := func(h volt.HandlerFunc) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		volt.Handler("POST /signup", h, app.Errors, run.Performer(dbh)).ServeHTTP(rec, httptest.NewRequest("POST", "/signup", nil))
		return rec
	}
	stored := func(email string) bool {
		t.Helper()
		_, err := run.Run(ctx, dbh, db.UserGetByEmail(email))
		if err != nil && !errors.Is(err, rt.ErrNotFound) {
			t.Fatal(err)
		}
		return err == nil
	}

	rec := serveSignup(signupHandler("two@example.com", "writes about Go"))
	var u db.User
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &u) != nil || u.Email != "two@example.com" {
		t.Fatalf("signup = %d %s; want 201 with the stored user", rec.Code, rec.Body.String())
	}
	if p, err := run.Run(ctx, dbh, db.ProfileGet(u.ID)); err != nil || p.Bio != "writes about Go" {
		t.Fatalf("the profile of user %d = %+v, %v; want it stored with the user", u.ID, p, err)
	}

	rec = serveSignup(signupHandler("three@example.com", "one", "two"))
	if rec.Code != http.StatusConflict || rec.Header().Get("X-Error-Handler") != "app" {
		t.Fatalf("a refused child = %d via %q %s; want 409 through the error handler", rec.Code, rec.Header().Get("X-Error-Handler"), rec.Body.String())
	}
	if stored("three@example.com") {
		t.Error("the user stayed after its profile was refused")
	}

	rec = serveSignup(relinkHandler("four@example.com", "not an address"))
	if rec.Code != http.StatusUnprocessableEntity || rec.Header().Get("X-Error-Handler") != "app" {
		t.Fatalf("a step its changeset refuses = %d via %q %s; want 422 through the error handler", rec.Code, rec.Header().Get("X-Error-Handler"), rec.Body.String())
	}
	if stored("four@example.com") {
		t.Error("the first write stayed after the second step's changeset was refused")
	}

	r := volt.RequestOf(httptest.NewRequest("POST", "/signup", nil), "POST /signup", time.Unix(1700000000, 0), nil)
	if !volt.Same(signupHandler("five@example.com", "a")(r), signupHandler("five@example.com", "a")(r)) {
		t.Error("one handler called twice answers two values")
	}
	if !volt.Same(signupHandler("five@example.com", "a")(r), signupHandler("five@example.com", "b", "c")(r)) {
		t.Error("two continuations over one first effect differ; a Then compares by its first effect")
	}
	if volt.Same(signupHandler("five@example.com", "a")(r), signupHandler("six@example.com", "a")(r)) {
		t.Error("continuations over two first effects compare the same")
	}
}
