// Runnable CRUD example: two `resources` lines in app/routes.volt
// become 13 of the 14 routes, plus typed controller interfaces and
// reverse-URL helpers. Run it with:
//
//	go run ./examples/crud
//
// then: curl -s localhost:8080/posts
//
//	curl -s -XPOST localhost:8080/posts -d '{"title":"hello"}'
package main

import (
	"log"
	"net/http"

	"github.com/Piechutowski/volt/examples/crud/app"
)

// newHandler wires the generated router to your implementations. It is
// separate from main so the test can serve it over httptest.
func newHandler() http.Handler {
	store := app.NewStore()
	return app.New(app.Controllers{
		Posts:    app.Posts{S: store},
		Comments: app.Comments{S: store},
		Health:   app.Health{},
	})
}

func main() {
	// A plain http.Handler — mount it, wrap it, serve it.
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", newHandler()))
}
