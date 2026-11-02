# Volt build plan

The one forward-looking document of the repository. It records the
decisions already made and the steps that remain, in order, with enough
design for a Claude Code session to build each step from `AGENT.md` and
this file alone.

Scope as decided: the language first, then the non-core features and
the tooling. Steps 2, 3 (without `volt fmt`), 4 and 6 to 12 land now, in
that order. Step 5 (the Elm client), `volt fmt` and steps 13 to 19 are
deferred; each keeps a one-line section below so the file still says
what is deferred and why. The deferred spec sections are not written,
and no spec text of the steps that land mentions a session, `r.Session()`
or the vet rule `session-csrf`: the request value gets `Now()` and
nothing session-shaped until step 13 is reopened.

How to use it:

- Read `AGENT.md`, then part 0 of this file, then the step you are on.
- Steps land in order unless a step says otherwise. A step lands as one
  or a few commits that leave the verification bar green, with the spec,
  the goldens, the conformance corpus and the roadmap updated in the
  same commit.
- When a step lands, delete its section from this file and flip its line
  in `docs/roadmap.md` in the same commit. The file shrinks to nothing.
- Where a step says "the spec decides the spelling", design the exact
  Go or Volt spelling in the spec first, then build it. Do not invent a
  second way to say something the language already says.

---

## Part 0. Decisions

### The thesis

One meta-language, `.volt`, declares the schema and the routes; `volt gen`
writes SQLite DDL, migrations, models, queries, the router, the handler
signatures and the clients as plain Go a person reads line by line.
Nothing is resolved at run time. The generated code, the runtime and the
tooling follow the functional style: no mutation, no side effects outside
one performer per lifetime.

### The functional rules

Feldman's reduction: the functional style is avoiding mutation and
avoiding side effects, and no language feature is required for it. Go
can do the style; what it lacks is help, since nothing in Go stops a
mutation or marks an effect. Volt supplies the help the way it supplies
everything else, through the generator and `volt vet`. The rules:

- **R1. Values.** Rows, changesets, predicates, queries, writes, requests,
  responses, diagnostics and every compiler data structure are values. A
  method on a value returns a new value and leaves its receiver as it was.
- **R2. No mutation after construction.** Local accumulation inside one
  function (appending to a slice that has not escaped) is allowed, since
  nothing can observe it. Mutating an argument, a field of a value someone
  else holds, or package state is not.
- **R3. Effects at the edge, as data.** Anything that touches the world is
  described as a value and performed by one place: the generated shim, the
  runner in `nao/run`, the worker, or `main`. Hand-written application code
  produces descriptions.
- **R4. Mutable state is inventoried.** The connection pool, the statement
  cache, the event broker's log and the logger are the whole list. The spec
  states it and `cmd/volt/layers_test.go` holds the import graph to it.
- **R5. Pure phases in the tooling.** Scanner, parser, checker, fold,
  generators and the formatter are functions from values to values.
  `volt gen` computes a map from path to content and writes it in one step
  at the end. The purity gate that guards memos today extends to every
  phase.

Go idioms chosen, one per need, so the code stays uniform:

| Need | Idiom |
|---|---|
| Sum types | A sealed interface: one method whose name starts with a lowercase letter, one small struct per kind in the same package, constructors that return the interface, a type switch in the single consumer. A test compares the list of kinds with the switch, since Go does not check exhaustiveness. |
| Immutable records | Structs with unexported fields, exported constructors, `With*` methods on value receivers. |
| Copy on write | `slices.Clone` before appending to a slice that came from outside; never `append` to an argument. |
| Optional values | `rt.Null[T]`; never a pointer in a public API. |
| Effects | A small set of effect types, each implementing a sealed `Effect[T]` with one `run` method the runner calls. |

### What Go cannot check, and how Volt closes it

Go cannot tell a pure function from one that opens a socket. The
mitigation is the capability model: every effect needs a value, and
hand-written code never holds one.

1. **The handle is the capability.** `run.DB` is created in `main`, passed
   to the generated `NewRouter`, and held by the shim and nothing else.
   The generated controller interface fixes every handler signature, so a
   handler cannot take a handle as a parameter, and vet requires
   controller types to be empty structs, so it cannot smuggle one in a
   receiver.
2. **Import boundaries.** `nao/rt` holds values and rendering and imports
   no `database/sql`. `nao/run` holds `DB`, `Open`, `Run`, the statement
   cache and migration execution, and is imported only by `main` and
   generated route files. A hand-written file of a routes package imports
   none of `nao/run`, `database/sql`, `net`, `net/http`, `os`, `os/exec`,
   `math/rand`, `crypto/rand`. The layers test enforces all of it.
3. **The clock and randomness come from the request.** The shim captures
   the time once; handlers read `r.Now()`. Tokens are made by the runtime.
   Vet forbids `time.Now`, `time.Since` and `rand` in routes packages.
4. **No package state, no goroutines in routes packages.** Vet flags a
   package-level `var` that is not a constant table, an assignment or `++`
   whose target resolves to a package-level object, an `init` function,
   and a `go` statement.
5. **A determinism test, generated.** For every handler, a test helper
   calls it twice with the same inputs and compares the response values.
6. **The pure path is the easy path.** Loads, bodies, the session, the
   time and the CSRF check are all handed to the handler, so it has no
   reason to want an effect.

Not caught: reflection, `unsafe`, cgo, and mutation through a pointer the
handler was handed, which the generated types prevent by having value
semantics and no pointers in their public API.

### Concurrency

A goroutine is an effect, owned by whoever owns a lifetime:

| Kind | Owner |
|---|---|
| Serving requests in parallel | `net/http`, one goroutine per request; pure handlers make it lock-free |
| Parallel effects inside one request | the shim, through `volt.All(e1, e2, ...)` over `errgroup` |
| Work after the request | a job row written in the handler's transaction, run by the worker |
| Periodic loops | `main`, through `volt.Every` |
| Push to clients | the event broker; `volt.Publish` is an effect performed after commit |
| CPU parallelism inside pure code | `rt.ParallelMap` over `sync.WaitGroup.Go`, which joins before returning |
| Cancellation and timeouts | the shim's loop, which is the only holder of the context |

Handlers own no lifetime, so they start nothing.

### Front ends and wire formats

- The web UI is Elm, a `Browser.application` served by the Go binary,
  talking JSON to the API through a generated Elm client. Volt generates
  no HTML templates; the framework serves one shell page plus assets.
- The native UI is guigui, talking GOB to the API through the generated Go
  client. A Go service calling another Volt service uses the same client.
- `Accept` and `Content-Type` negotiation stays as the spec has it: JSON
  for browsers and anything else, GOB for Go. The generated Go client's
  default format is GOB; JSON is selectable.

### The query language

Decision: a closed, typed grammar for joins, aggregates, arithmetic and
grouping, checked by `volt check`, with nesting done by separate batched
statements, and hand-written SQL in Go as the only escape. The raw-SQL
block idea (opaque `where` and `having` bodies validated by preparing) is
dropped.

This is the pattern every proven typed data layer converged on: Ecto's
query DSL (`join`, `group_by`, `select` with aggregates, `preload` as
separate queries, `fragment` as the one escape), Prisma (`include`,
`groupBy`, `aggregate`, `$queryRaw`), Ent (`WithPosts`, `GroupBy` with
`Aggregate`, modifiers), EF Core LINQ (`Include`, `GroupBy`,
`FromSqlRaw`) and Diesel (declared joinables, typed aggregates,
`sql_query`). All of them keep the query language closed so it can be
typed, nest through batched queries rather than joins so results stay
tree-shaped, and expose exactly one way to drop to SQL. Volt follows
that shape and adds its own rule: there is nothing to choose, the join
kind follows nullability and the nesting depth is one.

### Sessions and CSRF

- One session mechanism: an encrypted, signed cookie carrying a small map
  of string to string, int or bool. No session table in the runtime; an
  application that needs revocation declares one in its schema and reads
  it in a scope load.
- CSRF is `net/http`'s `CrossOriginProtection` (Go 1.25), wrapped as the
  plug `volt.CSRF`. No tokens.

### Documentation policy

After step 1 the repository's prose is: `README.md`, `AGENT.md`,
`docs/spec.md`, `docs/lint.md`, `docs/editor.md`, `docs/roadmap.md` and
this file. Nothing else carries opinion. `hypotheses.md`,
`entropy-and-purity-report.md` and the spec's non-goals are deleted; a
decision that was settled lives in the spec as a rule, an open one lives
nowhere until the maintainer settles it. The roadmap is a status table
against Phoenix and Rails and carries no rationale. The maintainer writes
the essay on the functional style; nothing in this file is that essay.

### Unchanged

SQLite is the only database. Names stay subject-first, verb-last. The Go
floor stays the newest stable Go. `volt gen` keeps refusing to overwrite a
file without the generated marker.

---

## Step 5. Generated Elm client

Deferred: the language first, then non-core features and tooling.

---

## Step 10. Aggregates, arithmetic, `group:`

Depends on: 9.

```ebnf
projection   = "(", proj item, { ",", proj item }, ")" | star projection ;
proj item    = ( column ref | aggregate | arith ), [ "as", name ] ;
aggregate    = "count", "(", [ column ref ], ")"
             | ( "sum" | "avg" | "min" | "max" ), "(", column ref, ")" ;
arith        = arith term, { ( "+" | "-" ), arith term } ;
arith term   = arith factor, { ( "*" | "/" ), arith factor } ;
arith factor = column ref | number | param | "(", arith, ")" ;
```

```volt
Select hourly (metric_id, ts / 3600 * 3600 as hour, avg(value) as avg, count() as n)
	for points where metric_id = :metric_id and ts >= :since
	[group: (metric_id, hour), order: (hour asc)]
```

1. An aggregate or an `arith` item needs `as`; a plain column does not.
2. `count` is `int`; `avg` is `float64`; `sum`, `min`, `max` take the
   column's type. With `group:` every group is non-empty, so none is
   null. Without `group:`, a select with an aggregate returns one row,
   is generated as an `rt.Lookup`, and `sum`, `avg`, `min`, `max` are
   `rt.Null[T]`.
3. `group:` lists projected items by name. Every non-aggregate item must
   be listed and every listed name must be a non-aggregate item; the
   error names the item.
4. `arith` follows the predicate type rules: numeric operands, one type
   per expression, integer division on `int`. No functions beyond the five
   aggregates, no `having`. Hand-written SQL in Go is the escape, and the
   spec says so once.

Rendering: a derived item renders `<expr> AS "hour"` with its literals
bound as arguments; `GROUP BY` and `ORDER BY` name aliases, which SQLite
resolves, so no expression is rendered twice; `count(*)` for a bare
`count()`.

Done when: conformance, renderer corpus, goldens, `nao/itest` checks the
hourly example against values computed in Go.

---

## Step 11. `with`: nested results

Depends on: 10.

```ebnf
select decl = "Select", name, [ projection ], "for", name, { join }, [ with ],
              [ "where", pred expr ], [ settings ], newline ;
with        = "with", name ;                 (* a reverse relation *)
```

```volt
Select board for metrics with points
```

Mints `MetricWithPoints { Metric; Points []Point }`. Runs as two
statements: the parent query, then the child table's full rows with
`"metric_id" IN (SELECT value FROM json_each(?))` over the parent keys,
grouped in memory by key, in parent order, child order by the child's
primary key. The runner wraps both in one deferred read transaction so
they see one snapshot. `with` takes a reverse relation only, no
projection of the children, one level. Later, when needed: `with` across
a junction table for many-to-many.

Done when: conformance, goldens, `nao/itest` with an empty parent set, a
parent with no children, and a thousand children.

---

## Step 12. Scope loads

Depends on: 7.

```ebnf
scope item = route | resources | dataset | scope | scope load ;
scope load = "load", name, go ref, newline ;
```

```volt
Scope / [pipe: app, error_handler: Errors] {
	load user CurrentUser
	...
}
```

```go
func CurrentUser(r volt.Request) volt.Load[db.User] {
	id, err := strconv.Atoi(r.Header("X-User"))
	if err != nil {
		return volt.Halt[db.User](volt.Problem(401, "sign in"))
	}
	return volt.Query(db.UserGet(id))
}
```

`volt.Load[T]` is a sealed sum: run this effect, or halt with this
response. The checker holds the Go function to `func(volt.Request)
volt.Load[T]`. Scope loads run before route loads and every route
beneath receives the value as a leading parameter after `r`. Nested
scopes append. The itest's `CurrentUser` reads a header, as above; a
session is step 13's and does not exist yet.

Done when: conformance, goldens, itest with a halt and a success.

---

## Step 13. Sessions

Deferred: the language first, then non-core features and tooling.
Until it is reopened nothing session-shaped exists: no `r.Session()`,
no `WithSession`, no `volt.Keys`, no `session-csrf` vet rule.

---

## Step 14. Passwords and CSRF

Deferred: the language first, then non-core features and tooling.

---

## Step 15. Jobs, the worker, periodic effects

Deferred: the language first, then non-core features and tooling.

---

## Step 16. Serve, health, secure headers, CORS, rate limit, telemetry

Deferred: the language first, then non-core features and tooling.

---

## Step 17. `volttest`

Deferred: the language first, then non-core features and tooling.

---

## Step 18. Tree-sitter grammar from the spec

Deferred: the language first, then non-core features and tooling. Until
then `grammar/grammar.js` is edited by hand for every syntax step, and
`tree-sitter generate` and `tree-sitter test` run as `editor.md`
section 8 says; a commit that could not run them says so and leaves
section 8 marked unverified for that step.

---

## Step 19. The example applications

Deferred: the language first, then non-core features and tooling. The
blog fixture under `itest/blog` is the application every step rebuilds
until then, and the line counts below are measured on it.

---

## Verification bar

Before claiming any step done:

```sh
go build ./... ./lsp/... ./cmd/volt/...
go vet   ./... ./lsp/... ./cmd/volt/...
go test  ./... ./lsp/... ./cmd/volt/...
go test  -race ./lang/... ./lsp/...
gofmt -l . | grep -v zed-extension      # empty
```

Goldens: `go test ./gen/router -update`, `go test ./nao/gen/... -update`,
read the diff. Grammar: `tree-sitter generate`, `tree-sitter test`, the
`editor.md` checklist, `./scripts/sync-grammar.sh`.

## Order

| Step | Depends on | Lands |
|---|---|---|
| 10 aggregates, group | 9 | |
| 11 with | 10 | |
| 12 scope loads | 7 | the last step of this round |
| 5, 13 to 19 | | deferred |

Measured at each step on the blog fixture and recorded in the commit
message: Volt lines, hand-written Go lines, generated Go lines. The
thesis is that the first two fall while the third is paid for once.
