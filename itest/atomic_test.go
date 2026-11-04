package itest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func cappedSignupHandler(email string, most int64) volt.HandlerFunc {
	return func(r volt.Request) volt.Response {
		return volt.Then(rt.Atomic(rt.Then(db.UserCreate(db.UserNew().EmailSet(email)), func(u db.User) rt.Effect[db.User] {
			return rt.Then(db.UserCount(), func(n int64) rt.Effect[db.User] {
				if n > most {
					return rt.Fail[db.User](volt.Error(http.StatusConflict, "the blog is full"))
				}
				return rt.Pure(u)
			})
		})), func(u db.User) volt.Response { return volt.Created(u) })
	}
}

func firstOf(ids []int) int { return ids[0] }

func brokenSignupHandler(email string) volt.HandlerFunc {
	return func(r volt.Request) volt.Response {
		return volt.Then(rt.Atomic(rt.All(rt.Then(db.UserCreate(db.UserNew().EmailSet(email)), func(db.User) rt.Effect[db.User] {
			return db.UserGet(firstOf(nil))
		}))), func([]any) volt.Response { return volt.NoContent() })
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

// TestAtomicRefusalAndPanicThroughTheShim proves rt.Fail, rt.Pure and a
// panicking next through the shim (spec §generated-query-surface): a
// next that refuses what it read answers its HTTPError's 409 through
// the route's error handler with the user rolled back, one with nothing
// left to perform commits, and a next that panics inside an rt.All
// ends its request alone, the server serving the next one and nothing
// of the panicking request stored.
func TestAtomicRefusalAndPanicThroughTheShim(t *testing.T) {
	ctx := context.Background()
	dbh := fixtureDB()
	defer dbh.Close()
	stored := func(email string) bool {
		t.Helper()
		_, err := run.Run(ctx, dbh, db.UserGetByEmail(email))
		if err != nil && !errors.Is(err, rt.ErrNotFound) {
			t.Fatal(err)
		}
		return err == nil
	}
	mux := http.NewServeMux()
	for pattern, h := range map[string]volt.HandlerFunc{
		"POST /full":   cappedSignupHandler("full@example.com", 1),
		"POST /room":   cappedSignupHandler("room@example.com", 2),
		"POST /broken": brokenSignupHandler("broken@example.com"),
	} {
		mux.Handle(pattern, volt.Handler(pattern, h, app.Errors, run.Performer(dbh)))
	}
	srv := httptest.NewUnstartedServer(mux)
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.Start()
	defer srv.Close()
	post := func(path string) (*http.Response, error) {
		resp, err := srv.Client().Post(srv.URL+path, "application/json", nil)
		if err == nil {
			resp.Body.Close()
		}
		return resp, err
	}

	resp, err := post("/full")
	if err != nil || resp.StatusCode != http.StatusConflict || resp.Header.Get("X-Error-Handler") != "app" {
		t.Fatalf("a refusing next = %v, %v; want 409 through the error handler", resp, err)
	}
	if stored("full@example.com") {
		t.Error("the user stayed after its next refused")
	}
	if resp, err := post("/room"); err != nil || resp.StatusCode != http.StatusCreated || !stored("room@example.com") {
		t.Fatalf("a next with nothing left to perform = %v, %v; want 201 with the user stored", resp, err)
	}

	if resp, err := post("/broken"); err == nil {
		t.Fatalf("a next panicking inside an rt.All answered %d; want its request ended", resp.StatusCode)
	}
	if stored("broken@example.com") {
		t.Error("the panicking request's user stayed")
	}
	if resp, err := post("/full"); err != nil || resp.StatusCode != http.StatusConflict {
		t.Fatalf("the server after a panic = %v, %v; want it serving the next request", resp, err)
	}
}

func slowSignupHandler(email string, mapped bool) volt.HandlerFunc {
	slow := func(u db.User) db.User {
		time.Sleep(150 * time.Millisecond)
		return u
	}
	return func(r volt.Request) volt.Response {
		create := db.UserCreate(db.UserNew().EmailSet(email))
		e := rt.Atomic(rt.Then(create, func(u db.User) rt.Effect[db.User] { return rt.Pure(slow(u)) }))
		if mapped {
			e = rt.Atomic(rt.Map(create, slow))
		}
		return volt.Then(e, func(u db.User) volt.Response { return volt.Created(u) })
	}
}

// TestAtomicTimeoutAnswers504 proves a route's timeout that expires
// while an Atomic's f or its last next runs, after its last statement
// and before its commit (spec §generated-query-surface, the runner, and
// §route-loads): the transaction fails with the context's error, which
// the shim answers with a 504 through the error handler, and nothing
// commits. The database is a file, since database/sql drops the
// connection of a transaction its context ended, and with it an
// in-memory database.
func TestAtomicTimeoutAnswers504(t *testing.T) {
	ctx := context.Background()
	dbh, err := run.Open("sqlite3", filepath.Join(t.TempDir(), "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dbh.Close()
	ddl, err := os.ReadFile(filepath.Join("blog", "db", "volt_schema.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dbh.Pool().Exec(string(ddl)); err != nil {
		t.Fatal(err)
	}
	for _, mapped := range []bool{true, false} {
		email := fmt.Sprintf("slow-%t@example.com", mapped)
		rec := httptest.NewRecorder()
		volt.Handler("POST /slow", slowSignupHandler(email, mapped), app.Errors, run.Performer(dbh), volt.Timeout(50*time.Millisecond)).ServeHTTP(rec, httptest.NewRequest("POST", "/slow", nil))
		if rec.Code != http.StatusGatewayTimeout || rec.Header().Get("X-Error-Handler") != "app" {
			t.Errorf("an Atomic whose pure step outlives the timeout (Map %t) = %d via %q %s; want 504 through the error handler", mapped, rec.Code, rec.Header().Get("X-Error-Handler"), rec.Body.String())
		}
		if _, err := run.Run(ctx, dbh, db.UserGetByEmail(email)); !errors.Is(err, rt.ErrNotFound) {
			t.Errorf("the user of the timed-out Atomic (Map %t): %v; want it never committed", mapped, err)
		}
	}
}
