// YOURS — the hand-written half. Nothing here imports the generated
// router: Go interfaces are implicit, so implementing the methods
// volt_handlers.go declares is all the wiring there is.
//
// Storage is an in-memory map on purpose: this example is about route
// and handler generation, not persistence. In a real project the rows
// would be nao-generated structs and queries over the same schema.
package app

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/Piechutowski/volt"
)

type Post struct {
	ID    int32  `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

type Comment struct {
	ID     int32  `json:"id"`
	PostID int32  `json:"post_id"`
	Author string `json:"author"`
	Body   string `json:"body"`
}

// Store is the toy database shared by both controllers.
type Store struct {
	mu       sync.Mutex
	posts    map[int32]Post
	comments map[int32]Comment
	nextID   int32
}

func NewStore() *Store {
	return &Store{posts: map[int32]Post{}, comments: map[int32]Comment{}, nextID: 1}
}

func (s *Store) id() int32 { id := s.nextID; s.nextID++; return id }

/* ===== Posts: the full resource (index, new, create, show, edit, update, delete) ===== */

type Posts struct{ S *Store }

func (p Posts) Index(w http.ResponseWriter, r *volt.Request) error {
	p.S.mu.Lock()
	defer p.S.mu.Unlock()
	out := make([]Post, 0, len(p.S.posts))
	for _, post := range p.S.posts {
		out = append(out, post)
	}
	return volt.JSON(w, out)
}

// New and Edit are the HTML-form actions: they render a form, they do
// not mutate. An [api] resource omits them entirely.
func (p Posts) New(w http.ResponseWriter, r *volt.Request) error {
	return volt.JSON(w, map[string]string{"form": "new post"})
}

func (p Posts) Edit(w http.ResponseWriter, r *volt.Request, id int32) error {
	post, err := p.find(id)
	if err != nil {
		return err
	}
	return volt.JSON(w, map[string]any{"form": "edit post", "post": post})
}

func (p Posts) Create(w http.ResponseWriter, r *volt.Request) error {
	var in Post
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		return volt.Error(http.StatusBadRequest, "invalid JSON body")
	}
	p.S.mu.Lock()
	defer p.S.mu.Unlock()
	in.ID = p.S.id()
	p.S.posts[in.ID] = in
	w.WriteHeader(http.StatusCreated)
	return volt.JSON(w, in)
}

// id arrives already parsed as int32 — the generated shim did it, and a
// non-numeric id never reaches this method (it is that route's 404).
func (p Posts) Show(w http.ResponseWriter, r *volt.Request, id int32) error {
	post, err := p.find(id)
	if err != nil {
		return err
	}
	return volt.JSON(w, post)
}

// One method serves both PATCH and PUT (§V5.2).
func (p Posts) Update(w http.ResponseWriter, r *volt.Request, id int32) error {
	var in Post
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		return volt.Error(http.StatusBadRequest, "invalid JSON body")
	}
	p.S.mu.Lock()
	defer p.S.mu.Unlock()
	post, ok := p.S.posts[id]
	if !ok {
		return volt.ErrNotFound
	}
	if in.Title != "" {
		post.Title = in.Title
	}
	if in.Body != "" {
		post.Body = in.Body
	}
	p.S.posts[id] = post
	return volt.JSON(w, post)
}

func (p Posts) Delete(w http.ResponseWriter, r *volt.Request, id int32) error {
	p.S.mu.Lock()
	defer p.S.mu.Unlock()
	if _, ok := p.S.posts[id]; !ok {
		return volt.ErrNotFound
	}
	delete(p.S.posts, id)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (p Posts) find(id int32) (Post, error) {
	p.S.mu.Lock()
	defer p.S.mu.Unlock()
	post, ok := p.S.posts[id]
	if !ok {
		return Post{}, volt.ErrNotFound
	}
	return post, nil
}

/* ===== Comments: the [api] subset, minus delete ===== */

type Comments struct{ S *Store }

func (c Comments) Index(w http.ResponseWriter, r *volt.Request) error {
	c.S.mu.Lock()
	defer c.S.mu.Unlock()
	out := make([]Comment, 0, len(c.S.comments))
	for _, cm := range c.S.comments {
		out = append(out, cm)
	}
	return volt.JSON(w, out)
}

func (c Comments) Create(w http.ResponseWriter, r *volt.Request) error {
	var in Comment
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		return volt.Error(http.StatusBadRequest, "invalid JSON body")
	}
	c.S.mu.Lock()
	defer c.S.mu.Unlock()
	in.ID = c.S.id()
	c.S.comments[in.ID] = in
	w.WriteHeader(http.StatusCreated)
	return volt.JSON(w, in)
}

func (c Comments) Show(w http.ResponseWriter, r *volt.Request, id int32) error {
	c.S.mu.Lock()
	defer c.S.mu.Unlock()
	cm, ok := c.S.comments[id]
	if !ok {
		return volt.ErrNotFound
	}
	return volt.JSON(w, cm)
}

func (c Comments) Update(w http.ResponseWriter, r *volt.Request, id int32) error {
	var in Comment
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		return volt.Error(http.StatusBadRequest, "invalid JSON body")
	}
	c.S.mu.Lock()
	defer c.S.mu.Unlock()
	cm, ok := c.S.comments[id]
	if !ok {
		return volt.ErrNotFound
	}
	if in.Body != "" {
		cm.Body = in.Body
	}
	c.S.comments[id] = cm
	return volt.JSON(w, cm)
}

/* ===== Health, and the one error handler for everything above ===== */

type Health struct{}

func (Health) Check(w http.ResponseWriter, r *volt.Request) error {
	return volt.JSON(w, map[string]string{"status": "ok"})
}

// Errors is named by routes.volt ([error_handler: Errors]). Every
// `return err` above lands here — one place decides status and shape.
func Errors(w http.ResponseWriter, r *volt.Request, err error) {
	volt.DefaultErrorHandler(w, r, err)
}
