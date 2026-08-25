# Volt Router — Specification (draft v0.3)

**Status:** Draft for discussion — nothing here is locked yet.
**Part of:** Volt. Grounded in [`research/features/`](../research/features/):
the `NO` rows of [`gostd.md`](../research/features/gostd.md) §P1 (ROUTE-7,
ROUTE-16, ROUTE-17, ROUTE-18), the P1 matrix and Volt notes of
[`comparison.md`](../research/features/comparison.md), and the codegen
position staked by
[not-an-orm](https://github.com/Piechutowski/not-an-orm).

**Changes in v0.2.** v1 generates registrations onto `http.ServeMux`
instead of emitting its own matcher (R2/R11 rewritten; the custom matcher
is a deferred optimization behind the same generated API). Pipelines now
speak the ecosystem contract `func(http.Handler) http.Handler`; the
`error` return lives only on controller interfaces (R3/R8 rewritten,
§4.1 gains the committed-response clause and the layering rationale).
New §0 positioning statement. New §12 **Datasets** — schema-driven
group-expanded resources — with decisions R12–R14.

**Changes in v0.3.** The file/module model moved to its own document,
[`language.md`](./language.md) (L-series decisions): one language in
three dialect layers, `.volt` files, package = directory, Go-style
`package`/`import`, DBML `use`/`reuse` removed (legacy reference:
[`dbml-imports.md`](./dbml-imports.md)). `Project { schema: }` is
replaced by importing the schema package. §11 question 1 is resolved by
L1. not-an-orm's source now lives in this repository under `nao/`.

**TLDR.** One authored file — `routes.volt` — and the router is
*generated*: registration onto the stdlib's own `ServeMux`, typed handler
interfaces, compile-checked reverse-URL helpers, a route table for
introspection, and schema-driven datasets that turn a `TableGroup` into a
full CRUD surface. Rails' ergonomics (`resources`, named routes, URL
helpers), Phoenix's compile-time safety (taken one step further: what
Phoenix warns about, Volt fails to compile), Go's runtime cost model
(no reflection, no runtime magic). Plain Go you can read, grep, and step
through in a debugger.

```text
        routes.volt  ──  the single source of truth (and your route table)
             │
   volt gen ─┼──► volt_router.go    ServeMux registrations + typed shims
             ├──► volt_handlers.go  per-controller interfaces, typed params
             ├──► volt_paths.go     reverse URLs: paths.User(42), compile-checked
             ├──► volt_routes.go    route table: introspection, metrics labels
             └──► volt_dataset_*.go schema-driven CRUD surfaces (§12)
```

---

## 0. Positioning

Volt does not replace the standard library's router — **it writes the
code you would have written around it.** At runtime, a Volt application
is stdlib vocabulary top to bottom: `http.Handler` middleware wrapping
`ServeMux` dispatch, handlers that touch a real `http.ResponseWriter`
and `*http.Request`. Everything Volt adds exists *before the program
runs*: the checked route table, the typed shims, the reverse-URL
functions, the introspection data, the dataset expansions.

Two consequences, stated plainly:

- **If all you need is matching, use `ServeMux`.** Post-1.22 it is a
  real router with a safety property (registration-time conflict
  detection) most frameworks lack. A DSL that only routes would not
  justify its toolchain.
- **The router earns its place as the hub other components derive
  from.** `ServeMux` is a sink — routes go in, dispatch comes out, the
  table is unrecoverable as data (ROUTE-18). Volt's route table exists
  before compilation, which is the precondition for typed reverse URLs,
  handler interfaces, OpenAPI derivation, and §12's datasets. The
  derived artifacts are therefore not "later": **v1 must ship typed
  handlers, `paths`, the routes table, and datasets on day one**, so the
  DSL pays for itself immediately.

## 1. The idea

Every framework router converges on the same matching problem; where
they differ is **when** the route table is known. Rails/Laravel/Django
build it at boot and pay with runtime lookups and route caches. Phoenix
proves the alternative: routes known at compile time become compiled
pattern-match clauses, and URL literals get verified against them. Go
has no macros — but it has `go generate`, and not-an-orm has already
demonstrated the pattern: **one canonical authored file, everything else
derived, validated at gen time, generated as plain code**.

A route table is *closed at build time in practice* — nobody's
production Go service registers routes conditionally at runtime. Once
gen time knows every route, it can emit the three things Go routing has
never had (ROUTE-16/17/18): groups with scoped middleware as a
first-class construct, **named routes with typed reverse-URL
functions**, and a route table you can enumerate. And because the
checker runs before the compiler, every mistake Rails discovers in
production (`NoMethodError`), Django at request time
(`NoReverseMatch`), and Phoenix at compile time (warnings), Volt turns
into a **gen error or a type error** — with the `routes.volt` line
number.

Matching itself is a solved problem — `ServeMux` solves it — so v1
spends no code there (§4.2, §7). What is generated is everything
*around* the match: the registrations, the typed parameter shims, the
statically composed pipelines, and the derivations.

## 2. Non-goals

- **A context object.** No `volt.Ctx` with 120 methods. Handlers speak
  `http.ResponseWriter` and `*http.Request` (§5) — the bunrouter/chi
  lesson, and gostd.md's "small interfaces as the universal seam."
- **A matcher of our own (v1).** `ServeMux` matches; Volt registers
  onto it (§4.2). A generated static matcher remains possible later
  behind the same API (§7.3) and is built only if profiling ever
  justifies it.
- **Runtime route registration.** No `router.GET(...)` API at all. The
  DSL is the only way in. (Escape hatch: `mount`, §3.5, forwards a
  subtree to any `http.Handler`.)
- **Being the whole web layer.** Sessions, auth, rendering beyond the
  §12 renderer seam, negotiation beyond `Accepts` — Volt siblings, not
  router features.
- **A custom transport (v1).** gnet/fasthttp adapters are possible
  later; they are not what makes this fast (§7.4).

## 3. The authored file: `routes.volt`

Routing elements are the outermost dialect layer of the Volt language
(L2): newline-sensitive, `{}` blocks, `[]` settings lists, `//`
comments, contextual keywords. They live in ordinary `.volt` files of a
package — in one file with the models, split across many, or in a
dedicated routes package importing the schema package; all layouts are
equivalent (L1). The schema connection is an ordinary package import:

```volt
package app

import (
	db                       // the schema package — enables [model:], [bind], Dataset
)

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

  resources users [model: db.User, only: (index, show, new, create)] {
    resources posts [shallow]
    member {
      post /promote  Users.Promote
    }
  }

  get /files/*path  Files.Serve
}

Scope /api/v1 [pipe: api, name: api] {
  resources users [api, model: db.User, bind]
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
resolves within your module — each of the **ecosystem contract**
`func(http.Handler) http.Handler` (R8). Every existing Go middleware —
chi's collection, otelhttp, CORS libraries — drops in unchanged; zero
adapters. Scopes attach pipelines with `[pipe: name]`; nested scopes
**append** (Phoenix semantics). Pipelines compose in generated code as
static function wrapping — there is no `[]Middleware` slice iterated
per request.

Errors do not traverse pipelines: the generated shim consumes a
controller method's `error` (§4.1) before the pipeline sees the
response. Middleware needing error awareness (error-value logging,
domain-error mapping) belongs in the scope's `[error_handler:]`, not in
a pipeline.

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
— nobody's proud of theirs), localized paths, format suffixes
(negotiation is the `Accept` header, §12.3).

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
and raw param access for the escape hatch.

**The error contract.** Controller methods return `error`; the
generated shim consumes it — mapping via the `volt.HTTPError`
interface, centralized per scope with `[error_handler: app.Errors]`
(the bunrouter lesson: one error spine, compiler-enforced). This is the
**only non-stdlib signature in the system**, and it is deliberate
layering, not disagreement with net/http: an error return is only as
good as the thing that catches it. At the protocol layer there is no
legitimate catcher — which is why `http.Handler` correctly omits it,
and why the Go blog's own "Error handling and Go" recommends exactly
this shape (`appHandler` returning an error, adapted down to
`http.Handler`) as the *application-level* pattern. Volt has the
catcher (`[error_handler:]`), so it uses the type system to feed it.

**Committed-response clause.** If a handler returns an error after the
response header has been written, the error handler is invoked in
log-only mode: it MUST NOT attempt to write status or body (HTTP cannot
un-send a 200). The `volt.Request` exposes `Committed() bool` for
handlers that need to know.

### 4.2 `volt_router.go` — registration onto ServeMux

v1 emits no matcher. It emits **registrations onto the stdlib's
`http.ServeMux`** plus typed shims — the code you would have written by
hand, specified and checked:

```go
func New(c Controllers, opts ...volt.Option) http.Handler {
	mux := http.NewServeMux()

	// GET /users/{id}  →  Users.Show   [pipe: browser]     routes.volt:14
	mux.Handle("GET /users/{id}", browser(volt.HTTP(
		func(w http.ResponseWriter, r *volt.Request) error {
			id, err := atoi64(r.PathValue("id")) // type from schema.edbml PK
			if err != nil {
				return volt.ErrNotFound
			}
			return c.Users.Show(w, r, id)
		})))
	// … one block per route; `browser` is the pipeline pre-composed once:
	// browser := chain(volt.RequestID, volt.Logger, app.Session, volt.CSRF)
	return mux
}
```

What this buys, for free and forever: `ServeMux`'s matching semantics
verbatim — specificity precedence, path sanitization, escaped-slash
handling, host patterns, 404/405 synthesis, trailing-slash redirects —
maintained by the Go team under the compatibility promise. Conflict
safety is doubled: `volt check` reports conflicts at gen time with
`routes.volt` line numbers for both routes, and `ServeMux`'s
registration panic remains as a backstop the generator provably never
triggers. The accepted constraint: the DSL offers nothing `ServeMux`
cannot match — which currently costs nothing, since the v1 envelope
(typed single-segment params, rest-wildcards, exact hosts) is exactly
`ServeMux`'s envelope.

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
universal gap comparison.md #1), and metrics label sets. Dataset rows
(§12) are marked as such, including whether an operation dispatches to
generated or overridden code.

## 5. The runtime package

Mirrors nao's `rt`: tiny, stdlib-only, semver-stable.

```go
type Request   struct{ *http.Request /* route identity, raw params, Committed() */ }
type HTTPError interface { error; StatusCode() int }

func HTTP(h func(http.ResponseWriter, *Request) error) http.Handler // the shim adapter
func JSON(w http.ResponseWriter, v any) error
func Accepts(r *http.Request, offers ...string) string
func Query(k, v string) URLOption
```

There is no `volt.Handler` or `volt.Middleware` public type: pipelines
are `func(http.Handler) http.Handler` (§3.2, R8), and the
error-returning shape appears only on generated controller interfaces
(§4.1). At runtime the system is stdlib vocabulary end to end; the one
seam that departs — the controller method signature — buys
compile-enforced error handling and bridges to std in one line in
either direction (`volt.HTTP`, or render-and-return-nothing).

## 6. Symbiosis with not-an-orm

This is the Rails lesson — the symbiosis is the product: components
derive from each other through one shared name. Volt does it without
runtime reflection, at gen time, **explicitly and optionally**:

- `[model: db.User]` on a resource points `volt gen` at the imported
  schema package (L3). It infers the `:id` param type from the PK
  (`integer [pk, increment]` → `int64`), names helpers by the model,
  and enables `paths.For(u)`.
- `[bind]` (requires `model:`) generates the Laravel move — implicit
  binding — as *visible code*: the shim calls `q.UserGet(ctx, id)`,
  maps `rt.ErrNotFound` → 404, and the handler signature becomes
  `Show(w, r, user models.User) error`. The DB boundary is explicit:
  `New(c Controllers, volt.WithQueries(q))`. No `model:`/`bind:` — no
  coupling; the router stands alone.
- Gen-time cross-validation: `[model: db.User]` naming a model absent
  from the imported package is a gen error, same class as nao preparing
  every query against the generated DDL.
- §12's `Dataset` is this symbiosis at group scale: the schema's
  `TableGroup` becomes the loop the generator expands.

## 7. Performance model

### 7.1 v1 rides ServeMux

Routing is 25–100ns of a request that spends milliseconds in the DB.
`ServeMux` (1.22+) is a competent pattern router; the generated shims
add a parameter parse and a statically composed pipeline — no slice
iteration, no context allocation for params, no reflection. That is
already "never the bottleneck," which is the only performance property
the router owes the application.

### 7.2 The deferred matcher

A generated static matcher (nested switches over path segments — the
direct Go translation of what the BEAM's pattern-match compiler does to
Phoenix's clauses) remains the known optimization: it would compare
against immediate constants, inline into the adapter, and target
bunrouter's measured ceiling (~24ns/0 allocs per match). It is
deliberately deferred behind the same generated API — built only if
profiling of a real Volt application ever shows `ServeMux` matching as
a material cost, which §7.4 predicts it will not.

### 7.3 Transport decoupling

Because dispatch is generated, the binding to `net/http` is one layer
of emitted code. A future gnet/fasthttp adapter regenerates that layer;
the DSL, checker, and derived artifacts are unchanged.

### 7.4 The gnet honesty clause

The speed ceiling the user actually feels is transport + serialization
— gnet territory — and that is an adapter decision deliberately
deferred, not a router property. We do not trade one line of ergonomics
for nanoseconds below the noise floor.

## 8. Compile-time guarantees

The scoreboard this spec exists for — "compile/build-time route safety"
was a one-entry column in comparison.md P1:

| Failure | Rails | Django | Phoenix | **Volt** |
|---|---|---|---|---|
| Route conflict / shadowing | silent (order wins) | silent (order wins) | compile warn | **gen error** (file:line of both routes; ServeMux panic as backstop) |
| Handler missing / wrong arity | `NoMethodError` at request | import error at boot | compile error | **compile error** (interface) |
| Reverse URL to unknown route | `NoMethodError` at request | `NoReverseMatch` at request | compile warn (`~p`) | **compile error** (undefined func) |
| Reverse URL wrong param type | silent string interpolation | silent | silent | **compile error** |
| Param type vs handler mismatch | n/a (strings) | runtime | n/a (strings) | **impossible by construction** |
| Route → missing model / dataset table | n/a | n/a | n/a | **gen error** (cross-checked vs schema.edbml) |
| Unused pipeline, dead name, unreachable route | `--unused` (CLI, opt-in) | — | — | **`volt vet`** |

## 9. CLI (parity with nao)

```sh
volt check  routes.volt     # syntax + semantics: conflicts, refs, names
volt vet    routes.volt     # legal-but-suspicious: dead names, shadow-prone patterns
volt gen                    # ./routes.volt (+ schema.edbml) -> volt_*.go
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
- **R2 — v1 generates onto `http.ServeMux`; dispatch is still decided
  at gen time.** The generator resolves conflicts and writes the
  registrations; ServeMux executes them. A generated static matcher is
  a deferred optimization behind the same API (§7.2), never a v1
  requirement. *(v0.1 specified an own matcher; overturned — matching
  is solved, the derivations are the product.)*
- **R3 — Controller methods are stdlib-typed plus one `error` return.**
  `(w http.ResponseWriter, r *volt.Request, typed params…) error`. The
  error return is the application-layer pattern the Go blog itself
  recommends; std omits it because the protocol layer has no catcher —
  Volt has one (`[error_handler:]`), so the type system feeds it
  (§4.1). No context object (chi's proof).
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
  safety property, kept — with better error messages, and ServeMux's
  own registration panic as the never-triggered backstop).
- **R8 — Pipelines speak `func(http.Handler) http.Handler`; errors end
  at the shim.** The ecosystem's entire middleware corpus drops in
  unchanged. Error-aware concerns (error-value logging, domain-error
  mapping) live in the scope's `[error_handler:]`, not in middleware.
  *(v0.1 had error-returning middleware; overturned — the error spine's
  value is at the controller boundary, and ecosystem compatibility is
  worth more than mid-chain error visibility.)*
- **R9 — Model coupling is opt-in, per-resource, and generation-side
  by default.** `model:` gives naming + `paths.For` (Phoenix's
  position); `bind` additionally generates the loader (Laravel's
  position, made explicit). The router core never imports the models
  package unless asked.
- **R10 — Generated code is regenerated, never patched.** `volt_*.go`
  files are build artifacts in your repo — reviewed in diffs, owned by
  `volt gen`, with a version header and `volt gen --check` for CI drift
  detection.
- **R11 — v1 transport is net/http via ServeMux; the generated layer is
  the seam.** §4.2, §7.3. *(v0.1's "transport-free matcher core" is
  subsumed: the seam is now the generated registration layer.)*
- **R12 — Dataset expansion is static, never runtime dispatch.** A
  `Dataset` block expands at gen time into per-table routes and typed
  instantiations (§12.1). There is no `/:table` parameter, no
  `map[string]` registry, no reflection; an unknown table is a routing
  404 and `volt routes` lists every expanded row.
- **R13 — Column metadata is parsed and validated at gen time.** The
  note conventions (`j.m.=`, `słownik=`) — or their future first-class
  nao settings (`[unit:]`, `[dict:]`) — are a declared contract:
  `volt vet` warns on unparseable notes, and the generated
  `[]volt.Column` is the single source the grid protocol, templates,
  and filter validation all consume.
- **R14 — Defaults are generated, overrides are declared, escape is
  omission.** The override ladder (§12.4) is the extensibility model
  for every generated surface: each rung replaces only what it names,
  every override is visible in `routes.volt` or the file tree, and
  nothing is discovered by reflection at runtime.

## 11. Open questions (need your call)

1. ~~File extension & name~~ — **resolved by L1/L2**
   ([`language.md`](./language.md)): everything is `.volt`, the package
   (directory) is the unit, file layout carries no semantics.
2. **`volt.Request` vs bare `*http.Request`** — narrowed by v0.2: the
   wrapper now appears only in controller signatures (pipelines are
   pure std). It still buys `Route()`, raw params, and `Committed()`
   without a context allocation; bare `*http.Request` + context values
   buys total signature purity at ~1 alloc/request.
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
7. **Per-route middleware sugar** — R8 keeps pipelines the only
   attachment point. Litestar and Phoenix (`when action in`) both
   support route/action-level middleware; a `[plug: app.Audit]` route
   setting would expand into the same static wrapping at gen time and
   keep every attachment visible in the one routes file. Worth the
   second attachment point, or does it erode the Phoenix-simple mental
   model?
8. **The `GridQuery` wire contract** (§12.3) — needs its own small
   normative section once stable: parameter names (`sort`, `f.<col>`,
   operators), operator set (`=`, `~=`, ranges), limits and defaults.

## 12. Datasets: schema-driven group expansion

The construct that closes the loop between Volt's two authored files:
`resources` where the loop variable is bound by the **schema** instead
of by hand. Motivating case: a survey database with ~40 uniform tables
(`da_*`, all injecting the same TablePartials, keyed `idpk`, with
note-encoded column metadata), one generic grid UI over all of them,
URLs like `/da/r_r` — and zero appetite for 40 hand-written resources.

### 12.1 Declaration and expansion

```volt
Dataset da [from: db.group(DA), pipe: api, formats: (html, json, gob)] {
  path:   strip('da_')                 // da_r_r → /da/r_r
  key:    idpk
  ops:    (list, create, update, delete)

  da_r_r [list: App.DaRRList]          // rung-2 override (§12.4)

  except: (da_kolumny, da_kolumny__podtabele, da_podtabele)
}
```

`volt gen` reads the schema's `TableGroup DA`, checks every member has
the declared key (gen error naming the table if not), and emits per
table: the four routes, a typed instantiation (§12.2), a
`[]volt.Column` spec, and `paths` helpers (`paths.DaRR()`). ~40 tables
→ ~160 static routes, each with a `volt routes` row pointing at the one
`Dataset` block. Adding a table next year is: add it to the DBML group,
run `nao gen && volt gen` — routes, adapter, columns, dictionary wiring
appear; nothing else to touch. A `GET /<dataset>/_meta` index route
(table list with titles from table notes) replaces hand-maintained
meta-tables.

### 12.2 Matching by instantiation

There is no runtime matching decision to make: the generator's
expansion loop emits the route string and the handler call **from the
same loop iteration**, so they cannot disagree. The typed bridge is
**generics instantiated by generation** — the runtime ships one generic
implementation per operation; generation picks the type:

```go
// volt runtime — written once:
func List[T any](
	query func(context.Context, GridQuery) ([]T, int, error),
	cols []Column,
	render Renderer,
) func(http.ResponseWriter, *Request) error { … }

// volt_dataset_da.go — generated, one block per table:
mux.Handle("GET /da/r_r", api(volt.HTTP(
	dataset.List[models.DaRR](deps.Q.DaRRGrid, daRRColumns, deps.Render))))
```

`/da/r_r` therefore returns `[]models.DaRR` — the actual nao structs,
compiler-held end to end. Zero reflection; Go's generics do at compile
time what Rails does with `const_get` at runtime. Write paths decode
into nao's params structs (`models.DaRRCreateParams`) symmetrically.

Filtering and sorting are the one runtime-dynamic surface: `GridQuery`
(`?sort=-rok&f.rok=2024&f.kod~=61-%`) is validated against the
generated column set — unknown column → 400, values parsed to the
column's type, predicates mapped onto nao's typed predicate values
(nao v2). Injection is impossible by construction: columns come from a
gen-time whitelist, values are bound parameters.

### 12.3 The renderer seam: HTML, JSON, GOB

Generic handlers never marshal; they end with
`return render(w, r, page)`. The default renderer negotiates on
`Accept`:

```go
switch volt.Accepts(r, "text/html", "application/json", "application/x-gob") {
case "application/x-gob": return gob.NewEncoder(w).Encode(page)
case "application/json":  return volt.JSON(w, page)
default:                  return tmpls.ExecuteTemplate(w, page.Template(), page)
}
```

- **HTML** resolves by convention with fallback:
  `templates/da/r_r.html` if present, else the shared
  `templates/dataset.html` grid template rendering any page from its
  `Columns` (titles, units, dictionaries — §12.5). `html` enabled with
  no generic template = gen error, not a runtime 500.
- **JSON** is the API/browser-client arm.
- **GOB** is the Go-native arm, and the reason it's first-class: nao's
  `--models-only` mode exists precisely to share row types between a
  server and a GUI process. A Go client (e.g. a guigui desktop app)
  imports the same generated models package and decodes
  `dataset.Page[models.DaRR]` in four lines — no DTOs, no mapping, no
  JSON tag maintenance. Schema → structs → SQL → wire → widget: one
  type the whole way, every hop generated.

`formats:` is declared per Dataset; the renderer is a `deps` seam —
replace it wholesale without touching routing.

### 12.4 The override ladder

The extensibility model (R14): **defaults are generated, overrides are
declared, escape is omission.** Each rung replaces only what it names:

1. **Nothing** — generated wiring, generic handler, generic template.
2. **One operation, one table** — `da_r_r [list: App.DaRRList]`: the
   generator swaps that single instantiation for a call to your method
   (which lands on a controller interface — forgetting to implement it
   is a compile error). The table's other operations stay generated.
   The override receives the parsed, validated query **and the
   generated default as a `next` argument**
   (`DaRRList(w, r, q, next dataset.ListFunc[db.DaRR]) error`) — call
   `next` to wrap (adjust the query, log, then delegate), ignore it to
   replace. Wrap-or-replace with zero extra wiring; see
   `docs/example/`.
   - **2.5 — presentation only**: drop a `templates/da/r_r.html` file,
     or supply your own `Renderer` in deps. No routing touched.
3. **One table entirely** — `except:` it; write it as an ordinary
   `resources`/routes by hand in the same file.
4. **The whole group** — don't declare a `Dataset`.

Every override is visible in `routes.volt` or the file tree; `volt
routes` shows per-row whether dispatch is generated or overridden.
There are no reflection-discovered hooks and no subtraction operator.
Contrast intended: Rails scaffolding generates once and abandons you;
Django's admin owns you until you subclass your way out; a Dataset
regenerates forever *around* the pieces you have claimed.

### 12.5 Column metadata

The generator lifts per-column UI metadata from the schema (R13):
today, the note conventions `'Title, j.m.=unit, słownik=Dict'` parse
into `Column{Name, Type, Title, Unit, Dict}`; the intended end state is
first-class nao extensions (`[unit: 'zł', dict: TakNie]`) with the note
parser as the migration path. The `[]volt.Column` spec is consumed by
filter validation (§12.2), the generic template, the `_meta` endpoint,
and any client (a grid widget renders headers, formats `zł`/`ha`/`%`,
and populates dictionary dropdowns from it). `volt vet` warns on notes
that don't parse — the convention is a contract, not folklore.

### 12.6 Honest costs

- `GridQuery` is runtime-dynamic by nature; the gen-time column
  whitelist is the containment, not elimination, of that dynamism.
- Datasets lean on nao v2 (typed predicate values) for filter/sort;
  until then, `List` generation is limited to whole-table + key lookup.
- The note-metadata convention is load-bearing (R13) and must be
  vetted, versioned, and eventually promoted into the schema language.
- ~160 generated routes are ~160 real registrations: binary size and
  gen time grow linearly with group size. At survey scale (dozens of
  tables) this is noise; the §7.2 gen-time budget (<1s for 500 routes)
  covers datasets too.
