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
and no spec text of the steps that land mentions a session,
`r.Session()` or the vet rule `session-csrf`: the request value gets
`Now()` and nothing session-shaped until step 13 is reopened.

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

### Feature scope

A feature lands only when a real program cannot be written well without
it, as scopes are for routing. A convenience a handler writes in a few
lines stays in the handler, and the language does not grow a setting or
a binding for it: a route's default or most limit, an optional list read
from the query string and a fixed value bound to an optional parameter
were built and taken out on this rule.

### The functional rules

Feldman's reduction: the functional style is avoiding mutation and
avoiding side effects, and no language feature is required for it. Go
can do the style; what it lacks is help, since nothing in Go stops a
mutation or marks an effect. Volt supplies the help the way it supplies
everything else, through the generator, `volt vet` and the gates. The
rules are spec §values-and-effects; how they hold for every package,
the Go idioms chosen for them and the gate that checks them are
`docs/style.md` section 2.

### TigerStyle

Volt's code follows TigerStyle translated to Go, joined with the
functional rules, as `docs/style.md` states. The maintainer settled how
it meets Go: names in Go casing and big-endian order; no recursion, but
explicit stacks with asserted depth limits; bounded allocation instead
of allocation at startup; an assert package that panics, stays on in
production and runs in generated code too; fixed-size integers at the
boundaries only; owned state only; `urfave/cli/v3` replaced by the
standard library while golang.org/x/tools and mattn stay; requests
served one by one, with own-pace batching for the worker, the broker
and the loops. Comments and commit messages stay the maintainer's. The
debt older than the style is inventoried: the style gate's ledger holds
the debt to the gated rules, steps 22 to 34 carry the debt to the rest,
and the same steps pay both off.

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
   and a `go` statement, and a `next` or `f` the runner calls that writes
   a variable it captures, which an `rt.All` would run at once.
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
| Parallel effects inside one request | the runner, through `rt.All(e1, e2, ...)` over `sync.WaitGroup.Go`, which joins every step and raises a step's panic again on the goroutine that performs it; inside a transaction the steps run in order on its one connection, so a failure never leaves a sibling writing outside it |
| Work after the request | a job row written in the handler's transaction, run by the worker |
| Periodic loops | `main`, through `volt.Every` |
| Push to clients | the event broker; `volt.Publish` is an effect performed after commit |
| CPU parallelism inside pure code | `rt.ParallelMap` over `sync.WaitGroup.Go`, which joins before returning and raises a panic of `f` again on the caller's goroutine |
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
`docs/spec.md`, `docs/style.md`, `docs/lint.md`, `docs/editor.md`,
`docs/runner.md`, `docs/roadmap.md` and this file. Nothing else carries opinion. `hypotheses.md`,
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

Deferred: the language first, then non-core features and tooling.

---

## Step 20. No external tools in the performance report

`internal/perfsweep` describes the machine by running `uname`, `sysctl`,
`sw_vers` and `git` (AGENT.md, the stack). It reads them from the
system instead: the kernel release from `/proc` on Linux and through
`syscall.Sysctl` on macOS, the commit from `.git` itself. The `+dirty`
mark needs `git status` and goes, unless the report takes it from the
build's own VCS stamp.

---

## Step 21. Open questions of the runner

Measure and settle what `docs/runner.md` section 6 lists, then delete this
section and that one:

1. Reader scaling past four cores: whether mattn's SQLite, built with
   its memory statistics behind one global mutex, caps parallel reads at
   high core counts, and whether the readers need a cap below
   `GOMAXPROCS`.
2. The bulk insert's statement size: 1,000-row statements measured 16%
   faster than statements cut to the 32,766-parameter limit (37 ms
   against 44 ms for ten thousand rows); whether a fixed size replaces
   the limit.
3. Double-quoted identifiers that name no column compile as string
   literals (`SQLITE_DQS=3` in mattn's build), so a misspelled column in
   generated SQL is not caught; how to catch it.
4. A cancellable context makes mattn start a goroutine per statement and
   per row; what that costs request traffic.

---

## Steps 22 to 34. Paying off the style debt

`docs/style.md` is the standard. `cmd/volt/testdata/style.ledger`
inventories the debt to its gated rules, 1,216 lines when the style
landed, and these steps carry the debt to the rules held in review. A
step lowers the ledger with `go test ./cmd/volt -run TestStyleLedger
-update` in the commit that pays, and deletes its section when its debt
is gone. A step that changes generated code changes the generator,
refreshes the goldens and the fixtures, and updates the spec where the
spec fixes the output.

## Step 22. The assert package

`style.md` S5. A package `assert` at the module root, standard library
only, importable by every package of the three modules and by generated
code: `assert.That(ok bool)` panics when `ok` is false, and
`assert.Unreachable()` panics always, for the `default` of a sealed
switch. The panic value names the caller's file and line, found only on
failure. Assertions stay on in production. A failure inside a request
ends the process: `net/http` recovers a handler's panic, so the shim
logs the failure and exits, and spec §generated-query-surface, which
today has a panic end its request alone, changes in the same commit.
`renderEncode` stops recovering every panic into an error, so an
assertion failure there crashes too. Generated files import the
package: the spec's list of what generated files import and AGENT.md's
change in the same commit. The style gate gains `assertion-density`, at
least two assertions per function on average in each package, counted
over calls into `assert`; the ledger records each package's shortfall,
which only shrinks, and S5 turns from review to gate.

## Step 23. Limits

`style.md` S2, S3, S10 and S12. Every input and every structure that
grows gets a named limit (`fileBytesMax`, `nestingDepthMax`), asserted
where the program guarantees it and reported where input crosses it:

- the language: the size of a source file, the number of tokens, the
  nesting depth of predicates, arithmetic and scopes; the spec states
  each, with a diagnostic that cites its section;
- the network runtime: the server's timeouts and header size, the size
  of a request body, the event broker's log, each subscriber's channel
  and buffer, the client's response size and timeouts;
- the data runtime: the rows a list effect may return, the busy retry;
- the tools: the size of a language server message, the number of open
  documents, the size of what `volt gen` writes.

Calls that relied on a library default pass the option explicitly,
among them `PRAGMA synchronous = NORMAL` among the writer's and the
readers' pragmas, which mattn's DSN default sets today. The broker runs
at its own pace (S10): `Publish` appends to the bounded log, and each
stream wakes on an interval or a heartbeat and sends a bounded batch of
what the log gained, instead of sending each event as it is published.

## Step 24. No recursion

`style.md` S1. The 61 functions the ledger lists on cycles of calls:
the EBNF tools of `lang/ebnf` (16), the predicate, arithmetic and scope
parsers of `lang/parser/volt.go` (10), the select lowering and checks of
`lang/selects.go` and `lang/semantics.go`, the walkers of the language
server, `lang/ast.Inspect`, and the renderers and comparisons of
`nao/rt`. Each becomes a loop over an explicit stack whose depth is
bounded by step 23's nesting limit and asserted; the parsers of
predicates and arithmetic become precedence climbing over an operator
stack. The parsers stop panicking on a syntax error and recovering it
(`bailout` in `lang/parser`): an error is a value they return (S1).

## Step 25. Owned state

`style.md` F1, F2 and F4. 285 functions write 693 times through a
parameter or receiver: the checkers and lowerings that accumulate on a
receiver (`c.diags = append(c.diags, ...)`), the cursors of the scanner
and the parsers, the handler options of the root package
(`func(s *shim)`), the generated setters that mutate the row `rt.Set`
hands them (`func(voltR *User) { voltR.ID = v }`), and `statusWriter`.
Helpers return what changed (diagnostics, the next position, the new
value) and the parent applies it; options become values; `rt.Set` takes
`func(T) T`; the response wrapper joins the inventory in the spec or
becomes the shim's local state. `epochLast`, the one package variable
written after `init`, keeps the broker epochs unique and joins the
broker's inventoried state.

## Step 26. Every error handled

`style.md` S8. 94 dropped error results in 54 functions: the `Close` of
rows and connections in `nao/migrate` and `nao/run`, the output writes
and database closes of `cmd/volt`, the scripts, and the blanks of
`strconv` parses in the root package's event replay. A `Close` error
joins the function's error; an output write that fails is reported once
and ends the command with exit 1; a parse that fails is handled.

## Step 27. Bounded loops

`style.md` S2. 156 loops in 133 functions are neither counted, finite
nor provably endless, most of them the scanning loops of
`lang/parser` (38), `lang/scanner` (19) and the EBNF tools (17), and
counted loops whose body moves the counter. Each becomes a counted loop
bounded by step 23's limits, with an assertion after it that the bound
was not the reason it stopped. An event loop, one that serves a
connection or the process and ends with its input or its context (the
broker's stream, the language server's transport, the client's
stream), is not capped: S2 names its shape, and the gate gains a check
for it.

## Step 28. Short functions

`style.md` S6. 90 functions are longer than 70 lines. The longest are
generated: the route registration that `gen/router` writes as one
function of 620 lines in the blog fixture is split by scope; then
`lang/ast.Children` (259), the route lowering of `lang/routes_lower.go`,
`cmd/volt`'s `genRun` (229) and the select checks of `lang/selects.go`.

## Step 29. 100 columns

`style.md` L2. 4,238 lines in 590 files are wider than 100 columns:
1,153 in hand-written Go, 1,840 in Go tests, 370 in the generated
fixtures, 504 in `testdata` and the goldens, 370 in Volt sources and
one in SQL. `gofmt` does not wrap, so hand-written code is wrapped by
hand and the generators wrap what they write, long SQL included.

## Step 30. Fixed-size integers at the boundaries

`style.md` S4. Generated models, changesets, wire types and the
runtime's stored or sent values use a fixed size. Spec §go-types, which
maps the schema's `int` to Go's `int` on 64-bit targets today, decides
what `int` means for a column and for the wire, and the generators
follow; conversions to a narrower type and boundary arithmetic that can
overflow are checked.

## Step 31. Names

`style.md` N1 to N14. Qualifiers and units move last, durations become
`time.Duration`, abbreviations are spelled out, swappable parameters
become options structs and flags get long names, in the hand-written
code and in what the generators emit; the spec decides the spelling of
every generated name it fixes.

## Step 32. The tool without `urfave/cli`

`style.md` D1. `cmd/volt` parses its commands with the standard
library's `flag`, keeping every command, flag and exit code; the ledger's
`dependency` line goes.

## Step 33. The gates reach further

`style.md` F4, F5, S9 and section 8. The type checks of the style gate
reach the test files, while `testdata` and the conformance snippets
stay outside them. The purity gate extends from the memoized
computations to every phase: the scanner, the parser, the checker, the
fold and the generators. A layers test holds the import graph to F4's
inventory: the packages that import `database/sql` are `nao/rt`,
`nao/run`, `nao/migrate`, the `nao/itest` fixture and the tool, and the
list does not grow;
nothing under `lang/`, `gen/` or `nao/gen` imports `nao/run` or the root
package. Part 0's import boundaries say `nao/rt` imports no
`database/sql`, which is not true today; the maintainer settles which
of the two changes. The bar runs the
`nilness` and `unusedwrite` analyzers of golang.org/x/tools beside
`go vet`.

## Step 34. Verification

`style.md` section 6:

- V3: native fuzz targets for the scanner, the parser, the checker and
  every decoder of a request, with their corpora checked in, replayed by
  `go test` and run under `go test -fuzz` before a release;
- V4: failure injection in `nao/run` and the migrations through a
  wrapping driver that fails the Nth call once and then persistently,
  `PRAGMA max_page_count` for a full database and a child process killed
  in the middle of a write, each run ending in `PRAGMA integrity_check`;
- V5: seeds printed by every randomized test, and a simulated executor
  behind `rt.Perform` with a seeded order for the steps of a `Parallel`;
- V6: a coverage floor per package that does not fall, and a test-only
  record of both outcomes of each boundary condition;
- V8 and V9: the round trips and the agreement of fast and plain paths;
- V10: the tests of the packages that start goroutines or hold
  inventoried state check that no goroutine, connection or statement
  outlives them; the bar already races `nao` and the root package;
- V11: the bar run on every push.

---

## Verification bar

Before claiming any step done:

```sh
go build ./... ./lsp/... ./cmd/volt/...
go vet   ./... ./lsp/... ./cmd/volt/...
go test  ./... ./lsp/... ./cmd/volt/...
go test  -race ./lang/... ./lsp/... ./nao/... .
gofmt -l . | grep -v zed-extension      # empty
```

Goldens: `go test ./gen/router -update`, `go test ./nao/gen/... -update`,
read the diff. Style ledger: `go test ./cmd/volt -run TestStyleLedger
-update` lowers it and never raises it. Grammar: `tree-sitter generate`,
`tree-sitter test`, the `editor.md` checklist, `go run ./scripts/sync-grammar`.

## Order

| Step | Depends on | Lands |
|---|---|---|
| 5, 13 to 19 | | deferred |
| 20 | | next |
| 21 | | open: measure first |
| 22 | | next |
| 23 | 22 | |
| 24 | 23 | |
| 25, 26, 27 | 22 | |
| 28, 29, 32 | | any time |
| 30, 31 | | spec first |
| 33 | 25 | |
| 34 | 22, 23 | |

The blog fixture under `itest/blog` is the application every step
rebuilds. Measured at each step on it and recorded in the commit
message: Volt lines, hand-written Go lines, generated Go lines. The
thesis is that the first two fall while the third is paid for once.
