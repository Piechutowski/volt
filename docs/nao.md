# nao — Volt's ORM

**An ORM-grade developer experience for Go + SQLite, without the ORM.**
nao is Volt's data layer, a sub-library of the framework the way
ActiveRecord sits inside Rails: declare tables in `.volt` files,
and `volt gen` writes the structs, CRUD, typed queries and DDL. No
reflection, no runtime magic, no `Save()` methods that lie — plain Go
you can read, grep, and step through in a debugger.

```text
        db/*.volt  ──  the single source of truth (and your ER diagram)
             │
   volt gen ─┼──► nao_models.go    structs, params structs, enums, notes as doc comments
             ├──► nao_queries.go   typed CRUD on a Queries handle
             ├──► nao_dyn.go       typed filter/order/limit values
             ├──► nao_selects.go   Group/Pred selects: one method per member
             ├──► nao_validate.go  Validate() on rows and params structs with checks (§validation-checks)
             └──► nao_schema.sql   DDL + seed data (--sql); FK/CHECK/UNIQUE real

       app/*.volt  ──  routes over the schema (Part II of the spec)
             │
   volt gen ─┼──► volt_handlers.go  controller interfaces, the Controllers manifest, NewRouter
             ├──► volt_router.go    ServeMux registrations; query routes' handlers (§query-routes); the event stream (§event-routes)
             ├──► volt_paths.go     typed reverse-URL helpers
             ├──► volt_routes.go    the route table as data
             └──► client/volt_client.go  typed client: one method per query route, Events(ctx) stream (§generated-client);
                                         with `gen -o DIR` it is DIR/volt_client.go beside the models (decision layout-independent-gen)
```

## Ten seconds of it

```volt
package db

Table users {
  id         integer   [pk, increment]
  email      varchar   [not null, unique]
  bio        text      [note: 'NULL until the user writes one']
  created_at timestamp [not null, default: `CURRENT_TIMESTAMP`]
}
```

```go
db, _ := rt.Open("sqlite3", "app.db")      // WAL, busy_timeout, foreign_keys=ON
q := models.New(db)

u, err := q.UserCreate(ctx, models.UserCreateParams{Email: "ann@example.com"})
u, err = q.UserGetByEmail(ctx, "ann@example.com")   // per unique column
us, err := q.UserQuery(ctx, models.UserEmail.Like("%@example.com"),
	models.UserOrderBy(models.UserCreatedAt.Desc()), models.UserLimit(10))
err = q.Tx(ctx, func(q *models.Queries) error {     // same methods, in a tx
	_, err := q.UserUpdate(ctx, u.ID, params)
	return err
})
```

Nullable columns are `rt.Null[T]` values (JSON shows the value or
`null`), misses are `rt.ErrNotFound`, every name is subject-first
(`UserGet`, `UserList`, `EOrderStatusPending`) so autocomplete groups by
model. Generated code imports the standard library and the tiny
[`nao/rt`](../nao/rt/) runtime — nothing else (decision zero-dependencies), and no driver is
chosen for you: a module importing only `nao/rt` inherits zero
third-party dependencies (the tool
modules carry them all).

## The idea

The model layer's real job is keeping four representations of the same
truth coherent: the diagram, the DDL, the code, and the live database
([`model-layer.md`](model-layer.md)). ORMs fight the drift with runtime
machinery; nao removes it by construction — one canonical source,
everything else derived. And because SQLite is the only target (all-in,
decision sqlite-first), SQLite itself is the gen-time type checker: every generated
statement is proven by preparing it against the generated schema before
your code ever runs (decision expressions-are-sqlites).

There is deliberately no "any query" builder. nao has a build step, so
"any query" splits by **what actually varies at runtime** (decision shape-rule) — the
split is exhaustive:

| What varies at runtime | Answer | Status |
|---|---|---|
| nothing — fixed shapes | generated CRUD (`UserGet` … `UserDelete`) | **done** |
| values only, uniform shapes | `Select` over a `Group` with typed `Pred` composition (§groups to §selects-over-groups): one method per member, one signature | **done** |
| values only, arbitrary SQL | raw-SQL `Select`/`View` blocks, typed functions out, prepare-validated | next — [roadmap](roadmap.md) |
| filter / order / limit | typed predicate **values** (`UserEmail.Eq(x)`) over generated column handles | **done** |
| validation | `checks` as predicates and `[required]`: SQL `CHECK` + generated `Validate()` on rows and params structs, Go-ref checks Go-only; 422/409 with per-column details through query routes (§validation-checks, decisions validation-rides-checks, checks-on-the-api-path, required-and-problems) | **done** |
| associations | explicit per-ref loaders, batched `IN` | scheduled |
| migrations | declarative diff, hash-pinned ledger, owned 12-step rebuild (roadmap MIG) | scheduled |
| callbacks, lazy loading, dirty tracking | never — decision non-goals | — |

## The pieces

| Package | What it is |
|---|---|
| [`nao/rt`](../nao/rt/) | the hand-written runtime generated code links against: `Null[T]`, `DBTX`, `Tx`, `Open` (pragmas), `StmtCache`, the dynamic-query core (the query-layer decisions). Stdlib-only; registers no driver |
| [`nao/gen/golang`](../nao/gen/golang/) | the Go backend: models, queries, dynamic layer |
| [`nao/gen/sqlite`](../nao/gen/sqlite/) | the DDL backend: schema + seed INSERTs |
| [`nao/inflect`](../nao/inflect/) | the deterministic singularizer behind model naming (decision singular-models); `[model:]` overrides it (routing's `singular:` renames only the member helper, §settings) |
| [`nao/itest`](../nao/itest/) | full CRUD round trips through a real SQLite driver (the only cgo dependency, test-only, decision integration-over-mocks) |

The exact language-to-Go and language-to-SQL contracts — types,
nullability, naming, notes-as-comments, dialect decisions — are
normative appendices of the spec: [`spec.md`](spec.md) §mapping-to-go and
§mapping-to-sqlite-ddl. The generators are driven by the same project load as the
router (`lang.Load` → `Check` → generate), so schema packages and
routing packages are checked and generated by one `volt gen`.

## What proves it

`go test ./...` runs the whole chain: generator goldens that are
gofmt-stable and **compiled by the real Go toolchain**, every generated
CRUD statement **prepared** against the generated DDL on a real SQLite,
every SQL golden **executed** (foreign keys on, `foreign_key_check`
after), and `nao/itest`'s round trips through a real driver. The
feature-by-feature plan with statuses lives in
[`roadmap.md`](roadmap.md); the design law in
[`decisions.md`](decisions.md).
