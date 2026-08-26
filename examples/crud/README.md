# CRUD without typing routes

**Runnable**, unlike `docs/example/`: `go run ./examples/crud`, and
`go test ./examples/crud` proves every generated route works.

The whole route table comes from two lines in
[`app/routes.volt`](app/routes.volt):

```volt
resources posts [model: db.Post]

Scope /api [name: api] {
    resources comments [model: db.Comment, api, except: (delete)]
}
```

`volt routes ./examples/crud` expands them:

```
GET     /health                          Health.Check PathCheck
GET     /posts                           Posts.Index PathPosts
GET     /posts/new                       Posts.New PathNewPost
POST    /posts                           Posts.Create
GET     /posts/:id(int32)                Posts.Show PathPost
GET     /posts/:id(int32)/edit           Posts.Edit PathEditPost
PATCH   /posts/:id(int32)                Posts.Update
PUT     /posts/:id(int32)                Posts.Update
DELETE  /posts/:id(int32)                Posts.Delete
GET     /api/comments                    Comments.Index PathAPIComments
POST    /api/comments                    Comments.Create
GET     /api/comments/:id(int32)         Comments.Show PathAPIComment
PATCH   /api/comments/:id(int32)         Comments.Update
PUT     /api/comments/:id(int32)         Comments.Update
```

## What each setting did

| In routes.volt | Effect |
|---|---|
| `resources posts` | the full Rails-7 action set — 8 routes |
| `model: db.Post` | key type read from the schema's `id integer [pk]` → every handler takes `id int32`; no type spelled in routes.volt |
| `api` | drops the HTML-form actions `new` and `edit` |
| `except: (delete)` | drops DELETE (`only: (index, show)` is the allowlist form) |
| `Scope /api [name: api]` | prefixes the paths *and* the helper names (`PathAPIComment`) |
| `pipe: api`, `error_handler: Errors` | apply to every expanded route, composed once at gen time |

`update` is deliberately one action behind two verbs: PATCH and PUT
both land on `Update(w, r, id int32)`.

## The files

```
examples/crud/
├── volt.mod                ─ project root marker
├── db/schema.volt          ← YOURS      tables (the key type lives here)
├── app/routes.volt         ← YOURS      pipeline, scope, 2× resources
├── app/controllers.go      ← YOURS      implements the generated interfaces
├── app/volt_handlers.go    ← generated  interfaces + Controllers + New()
├── app/volt_router.go      ← generated  ServeMux registrations + typed shims
├── app/volt_paths.go       ← generated  reverse URLs (PathPost(id) …)
├── app/volt_routes.go      ← generated  the route table as data
└── main.go                 ← YOURS      wire once, serve a plain http.Handler
```

You never write a route line, a parameter parse, a URL string, or a
`mux.Handle` call. You write the schema, the two `resources` lines, and
the method bodies — the compiler tells you if a method is missing.

## Try it

```sh
go run ./examples/crud &
curl -s -XPOST localhost:8080/posts -d '{"title":"hello","body":"world"}'
curl -s localhost:8080/posts
curl -s -XPATCH localhost:8080/posts/1 -d '{"title":"renamed"}'
curl -s -XDELETE -o /dev/null -w '%{http_code}\n' localhost:8080/posts/1   # 204
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/posts/abc          # 404: id is typed
curl -s -o /dev/null -w '%{http_code}\n' -XDELETE localhost:8080/api/comments/1  # 405: except:(delete)
```

## Regenerating

```sh
go run ./cmd/volt check  ./examples/crud
go run ./cmd/volt routes ./examples/crud
go run ./cmd/volt gen    ./examples/crud     # rewrites the volt_*.go files
```

`gen_test.go` fails if the committed generated files ever drift from
what the generator emits.
