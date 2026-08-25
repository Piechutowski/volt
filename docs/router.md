# Volt Router — Specification (draft v0.1)

**Status:** Draft for discussion — nothing here is locked yet.
**Part of:** Volt. Grounded in [`research/features/`](../research/features/):
the `NO` rows of [`gostd.md`](../research/features/gostd.md) §P1 (ROUTE-7,
ROUTE-16, ROUTE-17, ROUTE-18), the P1 matrix and Volt notes of
[`comparison.md`](../research/features/comparison.md), and the codegen
position staked by
[not-an-orm](https://github.com/Piechutowski/not-an-orm).

**TLDR.** One authored file — `routes.volt` — and the router is
*generated*: a static Go matcher (routing as code, not as a data
structure), typed handler interfaces, compile-checked reverse-URL
helpers, and a route table for introspection. Rails' ergonomics
(`resources`, named routes, URL helpers), Phoenix's compile-time safety
(taken one step further: what Phoenix warns about, Volt fails to
compile), Go's runtime cost model (zero allocations, no reflection, no
runtime magic). Plain Go you can read, grep, and step through in a
debugger.

```text
        routes.volt  ──  the single source of truth (and your route table)
             │
   volt gen ─┼──► volt_router.go    the matcher: nested switches, static dispatch
             ├──► volt_handlers.go  per-controller interfaces, typed params
             ├──► volt_paths.go     reverse URLs: paths.User(42), compile-checked
             └──► volt_routes.go    route table: introspection, metrics labels
```

---

## 1. The idea

Every framework router converges on the same matching problem; where
they differ is **when** the route table is known. Rails/Laravel/Django
build it at boot and pay with runtime lookups and route caches. Phoenix
proves the alternative: routes known at compile time become compiled
pattern-match clauses, and URL literals get verified against them. Go
has no macros — but it has `go generate`, and not-an-orm has already
demonstrated the pattern: **one canonical authored file, everything else
derived, validated at gen time, generated as plain code**.

The router is the best possible candidate for this treatment because a
route table is *closed at build time in practice* — nobody's production
Go service registers routes conditionally at runtime — yet every Go
router (stdlib included) is built as if routes were dynamic: trees built
at startup, walked per request. If the table is static, the matcher can
be *code*: a nested `switch` over path segments that the Go compiler
turns into jump tables and inlined comparisons. That is strictly less
machinery than bunrouter's radix trie with its index tables and parent
pointers — and it is exactly what Phoenix compiles its routes into, one
language down.

The ergonomics follow from the same move. Once gen time knows every
route, it can emit the three things Go routing has never had
(ROUTE-16/17/18): groups with scoped middleware as a first-class
construct, **named routes with typed reverse-URL functions**, and a
route table you can enumerate. And because the checker runs before the
compiler, every mistake Rails discovers in production
(`NoMethodError`), Django at request time (`NoReverseMatch`), and
Phoenix at compile time (warnings), Volt turns into a **gen error or a
type error** — with the `routes.volt` line number.

## 2. Non-goals

- **A context object.** No `volt.Ctx` with 120 methods. Handlers speak
  `http.ResponseWriter` and `*http.Request` (§5) — the bunrouter/chi
  lesson, and gostd.md's "small interfaces as the universal seam."
- **Runtime route registration.** No `router.GET(...)` API at all. The
  DSL is the only way in. (Escape hatch: `mount`, §4.7, forwards a
  subtree to any `http.Handler`.)
- **Being the whole web layer.** Sessions, auth, rendering, negotiation
  are Volt siblings, not router features. The router's surface: match,
  dispatch, reverse.
- **A custom transport (v1).** The generated matcher is
  transport-agnostic (§7.3), but v1 ships a `net/http` adapter only.
  gnet/fasthttp adapters are possible later; they are not what makes
  this fast (§7.4).

## 3. The authored file: `routes.volt`

Same lexical family as EDBML: newline-sensitive, `{}` blocks, `[]`
settings lists, `//` comments, contextual keywords. One file per
application (splittable via `use`, same module semantics as EDBML §7).

```volt
Project app {
  package: 'routes'          // Go package for generated code
  module:  'example.com/app' // import path root for controller resolution
}

Pipeline browser {
  use volt.RequestID
  use volt.Logger
  use app.Session
  use volt.CSRF
}

Pipeline api {
  use volt.RequestID
  use app.BearerAuth
}

Scope / [pipe: browser] {
  get  /            Home.Index   [name: root]
  get  /about       Home.About

  resources users [model: User, only: (index, show, new, create)] {
    resources posts [shallow]
    member {
      post /promote  Users.Promote
    }
  }

  get /files/*path  Files.Serve
}

Scope /api/v1 [pipe: api, name: api] {
  resources users [api, model: User, bind]
  get /health   Health.Check
}
```

### 3.1 Routes

```
route  = verb, path, handler ref, [ settings ], newline ;
verb   = "get" | "post" | "put" | "patch" | "delete" | "options" | "head" | "any" ;
path   = "/", { segment } ;
segment = literal | ":" name [ "(" type ")" ] | "*" name ;
handler ref = controller name, ".", action name ;
```

- **Params.** `:id` captures one segment; `*path` captures the rest
  (end-of-path only). Params are **typed**: `(int64)`, `(int)`,
  `(uuid)`, `(string)` — explicit, or inferred (§6) from the bound
  model's PK. Default when neither applies: `string`. Types affect the
  *handler signature*, never the match (a non-numeric `:id(int64)` is a
  matched route whose parse fails → 404 by default; Django's converter
  semantics were considered and rejected, R6).
- **Names.** Every route gets a reverse-URL helper. Names are derived
  (`Home.About` → `About`; resources per Rails conventions:
  `User`, `Users`, `NewUser`, `EditUser`), overridable with
  `[name: …]`, prefixed by scope names (`api` scope →
  `APIUsers()`). Collisions are gen errors.

### 3.2 Pipelines

A `Pipeline` is a named, ordered middleware list. Entries are Go
references — `volt.CSRF` is `github.com/…/volt.CSRF`, `app.Session`
resolves within your module — each of type
`func(volt.Handler) volt.Handler` (§5). Scopes attach pipelines with
`[pipe: name]`; nested scopes **append** (Phoenix semantics). Pipelines
compose in generated code as static function wrapping — there is no
`[]Middleware` slice iterated per request.

### 3.3 Scopes

`Scope <prefix> [pipe: …, name: …] { … }` — path prefix + pipeline
attachment + helper-name prefix, arbitrarily nested. This is
ROUTE-16 (groups/namespaces/per-group middleware) as one construct.

### 3.4 Resources

`resources <table>` mints the Rails seven:

| Action | Verb + path | Helper |
|---|---|---|
| index   | `GET /users`          | `Users()` |
| new     | `GET /users/new`      | `NewUser()` |
| create  | `POST /users`         | — |
| show    | `GET /users/:id`      | `User(id)` |
| edit    | `GET /users/:id/edit` | `EditUser(id)` |
| update  | `PATCH/PUT /users/:id`| — |
| delete  | `DELETE /users/:id`   | — |

Settings: `[api]` (5 actions: no new/edit), `[only: (…)]`,
`[except: (…)]`, `[model: Name]` (§6), `[bind]` (§6), `[param: slug]`
(rename `:id`). Nesting mints `/users/:user_id/posts/…`; `[shallow]`
gives Rails shallow semantics (member routes lose the parent prefix).
`member { … }` / `collection { … }` blocks add extra routes at the
member/collection level.

### 3.5 The rest of P1 table stakes

`redirect /old → /new [status: 308]`, `mount /admin → app.AdminHandler`
(any `http.Handler`, prefix stripped), `any` verb, host scoping
(`Scope [host: 'api.example.com']`) — each one line in the DSL, each
compiled statically. Deliberately absent from v1: regex routes (ROUTE-8
— nobody's proud of theirs), localized paths, format suffixes.

## 4. Generated artifacts

All output is `gofmt`-stable, imports only stdlib + the tiny `volt`
runtime package (mirroring nao's `rt`), and is regenerated wholesale —
never edited (the not-an-orm contract).

### 4.1 `volt_handlers.go` — the handler contract

One interface per controller, method per action, **typed params in the
signature**:

```go
type UsersController interface {
	Index(w http.ResponseWriter, r *volt.Request) error
	Show(w http.ResponseWriter, r *volt.Request, id int64) error
	New(w http.ResponseWriter, r *volt.Request) error
	Create(w http.ResponseWriter, r *volt.Request) error
	Promote(w http.ResponseWriter, r *volt.Request, id int64) error
}

type Controllers struct {
	Home   HomeController
	Users  UsersController
	Files  FilesController
	Health HealthController
}

func New(c Controllers, opts ...volt.Option) http.Handler
```

This is the Rails "action missing" failure mode moved to the compiler:
add a route, and the build breaks until the method exists, with the
right parameter types. Delete a route, and the unused method is
`staticcheck`-visible. `volt.Request` embeds `*http.Request` and adds
only route identity (`Route() string` for metrics — ROUTE-19 parity)
and raw param access for the escape hatch. Handlers return `error`;
status mapping via `volt.HTTPError` interface, centralized per scope
with `[error_handler: app.Errors]` (the bunrouter lesson: one error
spine, compiler-enforced).

### 4.2 `volt_router.go` — the matcher

Routing compiled to nested switches on path segments — the direct Go
translation of what the BEAM's pattern-match compiler does to Phoenix's
clauses:

```go
func (rt *router) match(method, path string) (h volt.Handler, p params) {
	seg, rest := cut(path)
	switch seg {
	case "users":
		if rest == "" {
			switch method {
			case "GET":  return rt.usersIndex, p
			case "POST": return rt.usersCreate, p
			}
			return rt.notAllowed(getPostAllow), p
		}
		seg, rest = cut(rest)
		if seg == "new" && rest == "" { … }
		// :id — parsed inline, zero alloc
		id, ok := atoi64(seg)
		…
	}
}
```

Static-beats-param priority falls out of `switch` case ordering, decided
at gen time — no backtracking, because the generator resolves overlaps
(`/users/new` vs `/users/:id`) *once*, when it writes the switch, not
per request. 404/405 (with correct `Allow`), trailing-slash and
clean-path redirects: same behaviors ServeMux/bunrouter provide, all
emitted as code.

### 4.3 `volt_paths.go` — reverse URLs

```go
paths.User(42)                          // "/users/42"
paths.APIUsers()                        // "/api/v1/users"
paths.User(42, volt.Query("tab", "posts"))
urls.User(r, 42)                        // absolute, scheme/host from request
```

Typed arguments matching the route's param types. This is ROUTE-17 —
the single biggest routing gap in Go — solved *stronger* than any of the
four references: Phoenix's `~p` warns at compile time; a Volt path
helper that doesn't exist, or is called with an `int` where the route
takes a `uuid`, **does not compile**. Structs can interpolate via a
generated `paths.For(u models.User)` when `[model:]` is set (the
`Phoenix.Param` direction of model coupling — generation-side only,
R9).

### 4.4 `volt_routes.go` — introspection

`var Table = []volt.Route{ {Method, Pattern, Handler, Name, File, Line}, … }`
— powering `volt routes` (the `bin/rails routes` / `mix phx.routes`
table stake), runtime enumeration for OpenAPI derivation later (the
universal gap comparison.md #1), and metrics label sets.

## 5. The runtime package

Mirrors nao's `rt`: tiny, stdlib-only, semver-stable.

```go
type Handler        func(w http.ResponseWriter, r *Request) error
type Middleware     func(Handler) Handler
type Request        struct { *http.Request; /* route identity, raw params */ }
type HTTPError      interface { error; StatusCode() int }
func Query(k, v string) URLOption
```

Interop both ways: `volt.Wrap(http.Handler) volt.Handler` and
`volt.HTTP(volt.Handler) http.Handler`, so the ecosystem's
`func(http.Handler) http.Handler` middleware mounts into pipelines with
one adapter (the chi-compatibility tax, paid once, at the boundary you
choose).

## 6. Symbiosis with not-an-orm

This is the Rails lesson (§"why Rails is fast to build in" — the
symbiosis is the product): components derive from each other through
one shared name. Volt does it without runtime reflection, at gen time,
**explicitly and optionally**:

- `[model: User]` on a resource makes `volt gen` read `schema.edbml`
  (path from `Project` settings). It infers the `:id` param type from
  the PK (`integer [pk, increment]` → `int64`), names helpers by the
  model, and enables `paths.For(u)`.
- `[bind]` (requires `model:`) generates the Laravel move — implicit
  binding — as *visible code*: the shim calls `q.UserGet(ctx, id)`,
  maps `rt.ErrNotFound` → 404, and the handler signature becomes
  `Show(w, r, user models.User) error`. The DB boundary is explicit:
  `New(c Controllers, volt.WithQueries(q))`. No `model:`/`bind:` — no
  coupling; the router stands alone.
- Gen-time cross-validation: `[model: User]` naming a model absent from
  the schema is a gen error, same class as nao preparing every query
  against the generated DDL.

## 7. Performance model

### 7.1 The bar

bunrouter is the measured Go ceiling for runtime routers: ~24ns/0
allocs (1 param), ~20.9µs GitHub-172-route sweep. Volt's matcher must
meet or beat it. The structural argument: a generated switch compares
against *immediate constants* (no node loads, no index-table lookups,
no parent-pointer walks for params — offsets are known statically), the
Go compiler emits binary-search/jump-table dispatch over cases, and the
whole match inlines into the adapter. Params parse inline
(`strconv`-free `atoi64` on the slice) — typed params arrive on the
stack. Zero allocations end to end, including reverse URLs under
`paths.…` with a stack buffer.

### 7.2 What we give up

A generated matcher's cost is *binary size and compile time*, linear in
routes. At Rails scale (hundreds of routes) this is noise; the gen-time
budget is: check + gen for 500 routes < 1s (nao's bar).

### 7.3 Transport decoupling

The match core is `func(method, path string) (routeID, paramSpans)` —
no `net/http` types. The v1 adapter binds it to
`http.Handler`. A future gnet/fasthttp adapter reuses the matcher
unchanged; only param materialization and the `Request` shim differ.

### 7.4 The gnet honesty clause

Routing is 25–100ns of a request that spends milliseconds in the DB.
The router being maximally fast is a *non-negotiable hygiene property*
(never the bottleneck, zero GC pressure), not the speed story. The
speed ceiling the user actually feels is transport + serialization —
gnet territory — and that is an adapter decision deliberately deferred,
not a router property. We do not trade one line of ergonomics for
nanoseconds below the noise floor; we don't have to — the design gets
both.

## 8. Compile-time guarantees

The scoreboard this spec exists for — "compile/build-time route safety"
was a one-entry column in comparison.md P1:

| Failure | Rails | Django | Phoenix | **Volt** |
|---|---|---|---|---|
| Route conflict / shadowing | silent (order wins) | silent (order wins) | compile warn | **gen error** (file:line of both routes) |
| Handler missing / wrong arity | `NoMethodError` at request | import error at boot | compile error | **compile error** (interface) |
| Reverse URL to unknown route | `NoMethodError` at request | `NoReverseMatch` at request | compile warn (`~p`) | **compile error** (undefined func) |
| Reverse URL wrong param type | silent string interpolation | silent | silent | **compile error** |
| Param type vs handler mismatch | n/a (strings) | runtime | n/a (strings) | **impossible by construction** |
| Route → missing model | n/a | n/a | n/a | **gen error** (cross-checked vs schema.edbml) |
| Unused pipeline, dead name, unreachable route | `--unused` (CLI, opt-in) | — | — | **`volt vet`** |

## 9. CLI (parity with nao)

```sh
volt check  routes.volt     # syntax + semantics: conflicts, refs, names
volt vet    routes.volt     # legal-but-suspicious: dead names, shadow-prone patterns
volt gen                    # ./routes.volt -> volt_*.go
volt routes                 # the introspection table (verb, path, handler, helper)
volt lsp                    # language server: diagnostics, goto-handler, rename
```

Same architecture as nao (`token`/`scanner`/`ast`/`parser`/`check`/
`vet`/`gen`), everything importable as a library, tree-sitter grammar +
Zed extension + LSP so `routes.volt` gets the same editor experience as
`schema.edbml` — including **go-to-definition from a route to its
handler method** and back.

## 10. Decisions (proposed, R-series)

- **R1 — DSL file, not Go DSL.** Routes are authored in `routes.volt`,
  not discovered from Go code. Static analysis of a chi-style Go DSL
  ("codegen from partial evaluation") breaks on the first variable;
  a language gets a real checker, LSP, and `volt routes` for free, and
  EDBML has proven the toolchain cost is payable.
- **R2 — Routing is generated code, not a runtime data structure.**
  Nested switches, not tries. The Phoenix translation, and the reason
  conflicts are gen errors instead of first-match accidents.
- **R3 — Handlers are stdlib-typed and return `error`.**
  `(w http.ResponseWriter, r *volt.Request, typed params…) error`. One
  error spine (bunrouter's proof), no context object (chi's proof).
- **R4 — Controllers are interfaces, wired in one constructor.** The
  route table is also the dependency manifest; "action missing" is a
  compile error; no global registration, no `init()` magic (gostd.md
  design-decision #1 and tension #9).
- **R5 — Reverse URLs are generated functions.** Never string lookup by
  name at runtime. `route('name')`/`reverse()` fight the type system;
  generated funcs *are* the type system.
- **R6 — Param types shape signatures, never matching.** One route owns
  a path shape; a parse failure is that route's 404, not a fallthrough
  to a sibling route (Django converters' most confusing property).
- **R7 — Priority is static > param > wildcard, resolved at gen time.**
  Registration order is meaningless; ambiguity is an error (ServeMux's
  safety property, kept — with better error messages).
- **R8 — Pipelines are the only middleware attachment point.** No
  per-route middleware settings; a route needing different middleware
  belongs in a different scope. Keeps the mental model Phoenix-simple.
- **R9 — Model coupling is opt-in, per-resource, and generation-side
  by default.** `model:` gives naming + `paths.For` (Phoenix's
  position); `bind` additionally generates the loader (Laravel's
  position, made explicit). The router core never imports the models
  package unless asked.
- **R10 — Generated code is regenerated, never patched.** The
  regeneration story (tension #2's cost side) is: `volt_*.go` files are
  build artifacts in your repo — reviewed in diffs, owned by `volt gen`,
  with a version header and `volt gen --check` for CI drift detection.
- **R11 — v1 transport is net/http, matcher core is transport-free.**
  §7.3/§7.4.

## 11. Open questions (need your call)

1. **File extension & name** — `routes.volt`? `app.routes`? Multiple
   files by convention (`web.volt`, `api.volt` — the Laravel layout) or
   one file + `use` imports (the EDBML layout)?
2. **`volt.Request` vs bare `*http.Request`** — the wrapper buys
   `Route()` and raw params without context allocation; bare +
   `r.Context()` values buys perfect stdlib purity at ~1 alloc/request.
   Spec currently says wrapper (R3); genuinely close call.
3. **How far does `bind` go** — PK-by-`:id` only, or also
   `[param: slug]`-keyed lookups (`UserGetBySlug`), nested-resource
   scoping (Laravel's scoped bindings: `/users/:user_id/posts/:id`
   checks ownership)? Each step deepens the nao contract.
4. **`any`-verb and custom verbs** — worth the matrix, or YAGNI?
5. **Dev mode** — `volt dev` file-watcher that re-runs gen on
   `routes.volt` save (the P17 "no watch mode" gap), or leave it to
   `air`/`wgo` and keep the CLI pure?
6. **OpenAPI** — `volt_routes.go` + typed handler signatures make
   schema derivation (universal gap #1) reachable. In-scope for the
   router spec as a generated artifact, or a separate Volt component
   reading the same table?
