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
the debt to the gated rules, steps 22 to 35 carry the debt to the rest,
and the same steps pay both off. Step 36 lists the defects that the
audit against the style found, which are bugs whatever the style.

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
   `math/rand`, `crypto/rand`. The layers test enforces all of it. Not
   all of it holds today; step 33 lists what does not.
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
   generated SQL is not caught, and the prepare tests of generated SQL
   (`internal/dbtest.StatementPrepares`) cannot see one either; how to
   catch it. The audit proposes that the prepare check rewrite each
   double-quoted identifier outside a string literal into brackets
   before its `EXPLAIN`, since SQLite never reads a bracketed name as a
   string.
4. A cancellable context makes mattn start a goroutine per statement and
   per row; what that costs request traffic.

---

## Steps 22 to 35. Paying off the style debt

`docs/style.md` is the standard. `cmd/volt/testdata/style.ledger`
inventories the debt to its gated rules, 1,216 lines when the style
landed, and these steps carry the debt to the rules held in review. A
step lowers the ledger with `go test ./cmd/volt -run TestStyleLedger
-update` in the commit that pays, and deletes its section when its debt
is gone. A step that changes generated code changes the generator,
refreshes the goldens and the fixtures, and updates the spec where the
spec fixes the output. The places a step names by file and line come
from an audit of every area against the style, each finding confirmed
by a second reader; the gate counts the rest.

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

The sites worth most come first. The fallbacks that change the output
when a value is not the kind the checker guarantees: `rebuild` copies
NULL into every row when `from:` is not a function
(`nao/migrate/lower.go:208`), `IndexDDL` drops a partial index's WHERE
and so turns a partial unique index into a full one
(`nao/gen/sqlite/gen.go:250`), `refEndpoint` and `actionSQL` drop a
foreign key or its actions (`nao/migrate/shape.go:139`,
`nao/gen/sqlite/gen.go:338`), and `ModelName` and `fieldTag` skip a
setting that is not a literal (`nao/naming/names.go:77`, `189`). The
sealed switches without a `default` (`lang/check/migrations.go:176`,
`nao/naming/plan.go:651`, `lang/selects.go:2196`). The pairs asserted on
both sides: `Desc.Values` against `Desc.Fields` (`nao/rt/desc.go:207`),
the seven generated copies of a model's field order against each other,
the keys encoding/json writes against the desc's `Wire` names. The
generated `NewRouter`, which accepts nil controllers, a zero `run.DB` and
a nil broker, so a miswired server starts and fails per request. And
the programmer errors returned as ordinary errors or as user
diagnostics (`nao/gen/golang/queries.go:60`, `nao/rt/effect.go:86`,
`lang/selects.go:815`, `cmd/volt/main.go:594`).

## Step 23. Limits

`style.md` S2, S3, S10 and S12. Every input and every structure that
grows gets a named limit (`fileBytesMax`, `nestingDepthMax`), asserted
where the program guarantees it and reported where input crosses it:

- the language: the size of a source file, the number of tokens and of
  diagnostics per file, the nesting depth of predicates, arithmetic and
  scopes; the spec states each, with a diagnostic that cites its
  section. Today about a million nested parentheses, a file of a few
  megabytes, end the process with a stack overflow no `recover` catches
  (`lang/parser/volt.go:461`, and the walkers of `lang/selects.go`
  behind it), a mebibyte of `!` yields a million diagnostics and 753
  MiB, and a source past 2 GiB wraps its `int32` offsets
  (`lang/token/token.go:44`). The session's memos and
  the language server's analyses are never evicted
  (`lang/session.go:20`, `lsp/analysis.go:102`);
- the network runtime: a deadline on every route, where 99 of the 101
  routes of the router goldens that serve no event stream run on the
  bare request context today, since `timeout:` is optional (spec
  §route-loads); the server's timeouts and header size; the size
  of a request body and of what decoding it builds, where 4 MiB of
  unknown keys yields 471,363 field errors and a 422 of 51.7 MB
  (`nao/rt/changeset.go:317`); the count of a list parameter and of an
  `In`; the event broker's subscribers, payload size and `Replay`, and a
  write deadline on each stream, whose `Write` blocks forever on a
  client that stops reading (`events.go:192`); the client, which runs on
  `http.DefaultClient` with no timeout and decodes replies of any size
  (`client.go:17`), and whose stream reader grows an event without bound
  until a blank line (`events.go:309`). Every write route decodes GOB
  from any client (spec §formats), and encoding/gob is not hardened
  against adversarial input and has no limits to set; the maintainer
  decides whether GOB request bodies stay open to every client;
- the data runtime: the rows a read may return, where `List`, `Query`,
  `With` and every select without `limit:` read whole tables and a
  `limit:` parameter is checked only against 1, so `?limit=1000000000`
  passes (spec §generated-query-surface, which makes `List` always
  exist, and §query-routes, which routes it, change); the keys of
  `GetMany` and the changesets of `CreateMany`,
  where `GetMany` renders one placeholder per key and fails past
  SQLite's 32,766 variables; the goroutines of `rt.All` and
  `rt.ParallelMap`, one per element today (`nao/rt/effect.go:381`); the
  statement cache and its closing goroutines (`nao/run/stmtcache.go:45`);
  the busy retry and the writer's queue;
- the tools: the size of a language server message, where one
  `Content-Length` allocates that many bytes and a header line is read
  without a bound (`lsp/protocol/jsonrpc.go:153`); the number of open
  documents; the size of a file `internal/disk` reads and of what
  `volt gen` writes.

Calls that relied on a library default pass the option explicitly,
among them `PRAGMA synchronous = NORMAL` among the writer's and the
readers' pragmas, which mattn's DSN default sets today. The busy
timeout becomes a Go constant, and the read-back of `PRAGMA
busy_timeout` around every `BEGIN IMMEDIATE` goes with it: three
unprepared statements per write transaction today
(`nao/run/run.go:345`). The broker runs at its own pace (S10):
`Publish` appends to the bounded log, and each
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
(`bailout` in `lang/parser`): an error is a value they return (S1). The
gate counts calls whose callee it can name; the recursion it cannot see
goes too: through interfaces, in the AST methods that walk the chains
the parser builds (`lang/ast/volt.go:436`) and in the effect interpreter
of `nao/rt` (`effect.go:435`), and through callbacks, where vet's
`capturedwrite` enters `ast.Inspect` again from inside its callback
(`lang/vet/handlers.go:426`).

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

The writes the gate cannot see go the same way: through a method call
on a cursor or a builder the caller owns (the parsers' cursor, the
generators' `strings.Builder`); through `internal/par.For`, whose
callbacks write captured slots from worker goroutines
(`internal/par/par.go:14`, `nao/naming/table.go:284`,
`lang/semantics.go:153`); and through the request value, whose
`Header`, `Query` and `Body` hand out net/http's live maps
(`request.go:27`). The mutable state outside F4's inventory joins it in
the spec or goes: `check.Memo`, `vet.Memo`, `naming.PlanMemo` and the
`nameBase` that every plan built from one memo shares and each new
build rewrites (`nao/naming/plan.go:81`), the session's caches, the
language server's documents and indexes, `MaxBodyBytes`, the broker's
exported `Replay` and `Heartbeat`, and the generated client.

## Step 26. Every error handled

`style.md` S8. 94 dropped error results in 54 functions: the `Close` of
rows and connections in `nao/migrate` and `nao/run`, the output writes
and database closes of `cmd/volt`, the scripts, and the blanks of
`strconv` parses in the root package's event replay. A `Close` error
joins the function's error; an output write that fails is reported once
and ends the command with exit 1; a parse that fails is handled.

The errors that turn into an absence or an empty value go too, though
no result is dropped: `GoSourcesRead` and `hasVoltFiles` turn an
unreadable file or directory into a misleading diagnostic
(`lang/gofuncs.go:69`, `lang/project.go:219`), `disk.Source.Exists`
reads every `Stat` error as absence (`internal/disk/disk.go:35`),
navigation and hover answer empty (`lsp/handlernav.go:113`), the stream
client swallows every failure (`events.go:270`), and
`nao/rt/changeset.go:94` classifies errors by their message text.

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
overflow are checked, the front end's narrowing of offsets, lines and
lengths to `int32` among them (`lang/token/token.go:44`).

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
reach the test files, which they load none of today (145 files, with 62
dropped errors among them), while `testdata` and the conformance
snippets stay outside them. The purity gate extends from the memoized
computations to every phase: the scanner, the parser, the checker, the
fold and the generators; and it gains the memos AGENT.md already
requires it to hold, which it misses: `GoFuncsOf` and the per-package
check of `lang`, `selectHoverMD` and `goScanBuild` of
`lsp/voltnav.go`, and `buildVoltIndex` once step 35 makes it a function
of a snapshot. A layers test holds the import graph to F4's inventory:
the packages that import `database/sql` are `nao/rt`, `nao/run`,
`nao/migrate`, the `nao/itest` fixture and the tool, and the list does
not grow; nothing under `lang/`, `gen/` or `nao/gen` imports `nao/run`
or the root package. The layers test of the pure phases checks direct
imports only, so `lang`, `lang/check`, `lang/vet` and the generators
reach `database/sql` through `nao/rt`, `gen/model` through
`nao/migrate` as well, and `lang` reaches `os/exec` through `go/build`;
it follows the module's own packages transitively and names the
standard library packages allowed to reach the world.

Part 0's import boundaries are not all true today. `nao/rt` imports
`database/sql` (`bytes.go`, `null.go`). The hand-written plugs of a
routes package import `net/http`, as spec §pipelines requires
(`itest/blog/app/user.go`), and no layers test checks a hand-written
file of a routes package: vet's `handlerimports` covers only the files
that declare a handler or a scope load. The maintainer settles which
side changes for each. The bar runs the `nilness` and `unusedwrite`
analyzers of golang.org/x/tools beside `go vet`.

## Step 34. Verification

`style.md` section 6:

- V2: 79 of the front end's 477 diagnostics are produced by no test,
  §reference-resolution has no snippet, and 86 invalid snippets are
  refused for any reason, with no `want:` line; each uncovered
  diagnostic gets a snippet, and a test matches every `errorf` and
  `warnf` under `lang/` against the invalid goldens, keeping a list of
  the sites no input reaches that only shrinks;
- V3: native fuzz targets for the scanner, the parser followed by the
  checker, a one-file project, `Decode` of JSON and GOB, the query
  parameters in a round trip through `FormatQuery`, the event stream
  reader, the language server's framing and the index SQL parser of
  `nao/migrate`, with their corpora checked in, replayed by `go test`
  and run under `go test -fuzz` before a release; a 90-second probe of
  the parser and the checker found no crash;
- V4: failure injection in `nao/run` and the migrations through a
  wrapping driver that fails the Nth call once and then persistently,
  `PRAGMA max_page_count` for a full database and a child process killed
  in the middle of a write, each run ending in `PRAGMA integrity_check`;
  70 of `nao/run`'s 490 statements run in no test today, and they are
  its failure paths (`nao/run/run.go:333`);
- V5: seeds printed by every randomized test, and a simulated executor
  behind `rt.Perform` with a seeded order for the steps of a `Parallel`;
  34 wall-clock sleeps and timeouts pace the interleavings today, and 5
  reads of the clock or of randomness cannot be injected;
- V6: a coverage floor per package that does not fall, and a test-only
  record of both outcomes of each boundary condition. Under 80% today:
  `gen/model`, `internal/disk`, `lang/diag`, `lang/token` and the
  scripts at 0, `internal/perfsweep` at 40.1, `lang/ast` at 48.1,
  `nao/itest` at 50.2, `nao/naming` at 62.1 and `cmd/volt` at 77.7;
  `volt check`, `volt vet`, `--werror`, `--json`, `volt routes` and the
  Logger middleware run in no test;
- V7: test databases close with `PRAGMA integrity_check`,
  `foreign_key_check` and a check for leaked connections
  (`internal/dbtest/dbtest.go:31`); every parsed input is checked for
  spans that nest in their parent and slice to their own spelling; and
  the corpus writes every construct of the spec, where today it leaves
  out migrations, indexes, records, joins, aggregates, `with`, loads and
  bodies, so the budgets and the walks never meet them;
- V8 and V9: the round trips and the agreement of fast and plain paths:
  every valid snippet generates, builds and vets, where 12 of 126 do
  today, and every migration snippet folds, where 1 of 15 does; the
  fold's down-and-up round trip is read back
  (`cmd/volt/migration.go:417`); the chunked, reusing parse is checked
  against a plain one, Volt layer included
  (`lang/parser/reuse_test.go:73`);
- V10: the bar races `itest`, `internal/par`, `gen` and `cmd/volt` too;
  a broker test publishes from many goroutines while subscribers come
  and go, a statement cache test closes the cache during prepares, and
  the tests of the packages that start goroutines or hold inventoried
  state check that no goroutine, connection or statement outlives them;
- V11: the bar run on every push.

## Step 35. Effects at the edge

`style.md` F3 and F5, which the gates see only in part. The code breaks
the pure phases of spec §values-and-effects in two places, and the spec
or the code changes:

- The spec lists the fold among the pure phases, but `foldCheck`
  (`cmd/volt/migration.go:383`) makes a temporary directory, opens two
  SQLite pools, runs the migrations up, down and up again and reads the
  structure back (`nao/migrate.Read`), in the middle of `volt gen`.
  Either the runner performs the fold and returns the two structures as
  values, leaving the comparison as the pure phase, or the spec and F3
  name the fold as an edge.
- `volt gen` does not write in one step at the end: `--verify` creates
  the output directories before the overwrite check can still refuse
  the run, stale files are removed before any new file is written, and
  the writes run at once through `internal/par`, so one failed write
  leaves new and old files mixed (`cmd/volt/main.go:494`). `genRun`
  splits into a pure planner that decides every refusal and a performer
  that writes each file under a temporary name, renames it into place
  and then removes the stale ones.

The rest moves to the edges F3 names:

- The checker reads each package's Go files while it runs, inside a
  `sync.Once` on `internal/par` workers, and again for vet
  (`lang/gofuncs.go:298`, `lang/lint.go:24`), so Check and Vet can judge
  different sources; the loader reads them once into the `Project`. The
  checker matches their build constraints through `build.Default`,
  which holds the process environment (`lang/gofuncs.go:181`), and
  `filepath.Abs` reads the process's working directory
  (`lang/project.go:123`); both become inputs the loader records.
- The language server analyzes on an unjoined goroutine per root that
  sleeps, loads from disk and writes to the editor
  (`lsp/analysis.go:121`). The transport loop becomes the only
  performer, and the analysis a function of a snapshot (the overlay
  texts, their versions, the Go sources) whose result the loop
  publishes. Definition runs `go list` on the transport loop with no
  timeout, for every request, and drops its failure
  (`lsp/gotype.go:59`); the import paths are resolved once per
  analysis, at the edge, with a deadline. Features and the index build
  read files themselves (`lsp/voltnav.go:65`, `415`, `528`,
  `lsp/handlernav.go:108`), so every completion, hover, definition and
  rename reads all the Go sources again; they read the snapshot.
- Pipelines put hand-written `func(http.Handler) http.Handler` plugs in
  front of every shim (spec §pipelines), so application code writes
  headers and reads the body outside the effect model
  (`itest/blog/app/user.go:17`). The spec decides whether plugs belong
  to the network edge, named among the edges of §values-and-effects and
  F3 and held by vet as handlers are, or become descriptions the shim
  performs, as it performs a scope load.

---

## Step 36. Defects the audit found

Wrong whatever the style; the auditor or the second reader reproduced
each. Each is fixed with the test that shows it (`style.md` V1), and its
bullet goes in the commit that fixes it. Where a bullet names a spec
section, the spec changes in the same commit.

The language and the checker:

- Reusing a chunk relocates the `token.File` that an earlier parse
  shares (`lang/parser/reuse.go:56`), so the earlier result's
  declarations and diagnostics move by the lines inserted above them,
  and the results the language server publishes move under it
  (`lsp/analysis.go:46`). A reused chunk keeps its file, and the new
  parse gets positions of its own.
- `check.File` returns `Info.Partials` in map order, which changes from
  call to call, and completion lists partials in that order
  (`lang/check/check.go:183`).
- `Check` writes into the `Project` it is given, so a second `Check` of
  one `Project` repeats every "imported and not used" error
  (`lang/semantics.go:251`).
- The package memo's key leaves out the module path, so after a
  `go.mod` rename the session keeps the old import paths
  (`lang/session.go:70`); `checksEntry.holds` and the selects memo leave
  out the naming plan's error and replay an old one
  (`lang/selects.go:240`); and a memo hit returns declared-type columns
  that point at the previous run's `TypeInfo`
  (`lang/check/check.go:207`).
- `Check` builds the naming plan and drops its error, and the spec
  makes Go-name collisions an error of generation, so `table user`
  beside `table users`, or columns `a_b` beside `aB`, check and vet
  clean and only `volt gen` fails, with no position
  (`lang/semantics.go:80`, `nao/naming/table.go:314`, spec
  §mapping-to-go). The checker reports them.
- `timeout:` has no upper bound: from 9,223,372,037 seconds on, the
  generated `volt.Timeout` overflows and the router does not compile
  (`lang/routes_loads.go:363`, §route-loads).
- A pred is expanded again at each reference: twenty preds that each
  name the one before twice check clean and render a WHERE clause of
  18 MB with a million binds, so about 30 lines exhaust the language
  server's memory (`lang/selects.go:2161`). §predicates bounds a
  select's expanded predicate.
- Integer literals outside the `int64` range pass as defaults and
  records: the DDL keeps the default, SQLite stores a real, and the
  model drops its seed (`lang/check/types.go:363`,
  `nao/gen/golang/gen.go:353`, §default-values).
- A table of more than 2,000 columns checks clean and fails at
  migration, on SQLite's `SQLITE_MAX_COLUMN` (`lang/check/check.go:301`).
- Two schemas SQLite refuses pass `volt check` with a vet warning, as
  the spec has it: tables or columns whose names differ only in ASCII
  case (`lang/check/check.go:171`, §case-sensitivity-and-terminology),
  and a foreign key whose parent key is neither the primary key nor
  unique, which fails every write that checks it
  (`lang/check/resolve.go:17`, §relationships-ref). Both become errors.
- `GoName` returns an unexported name for a name in a script without
  letter case (`名前`), so encoding/json and gob skip the field and the
  column disappears from every response (`nao/naming/names.go:63`).
- For a json tag without a name (`json:",omitempty"`), `wireName` takes
  the column name where encoding/json takes the Go field name, so
  reads and writes use different keys (`nao/naming/names.go:217`).
- A negative literal in a predicate or a projection gets a token whose
  `Text` and `End` stop short, so hover shows `a = -` and editor ranges
  come up short (`lang/parser/volt.go:571`).
- An invalid UTF-8 byte inside a string or an SQL expression literal
  becomes U+FFFD with no diagnostic, so the generated SQL and data
  differ from the file (`lang/scanner/scanner.go:200`).
- A diagnostic raised while scanning the token that starts the next
  element is reported in both chunks, so twice, and stays stale after
  reuse (`lang/scanner/scanner.go:239`).

What `volt gen` writes:

- Routers that do not compile while `volt gen` exits 0: write query
  routes name their locals `volt_params`, `volt_cs` and `volt_row`,
  which a scope load may take (`gen/router/gen.go:504`); a path
  parameter `:Body(int)` becomes the local `voltBody`, which body routes
  declare too, and `Params` and `Row` collide the same way
  (`gen/router/gen.go:476`, §route-paths); `volt_handlers.go` imports a
  scope load's package that only query routes use
  (`gen/router/gen.go:1253`, §scope-loads); and `--package` is printed
  into every file unchecked (`cmd/volt/main.go:281`). The type check
  behind `--verify` becomes the default.
- A column tagged `json:"-"` is left out of JSON and sent over GOB,
  which ignores tags, so a client that asks for GOB reads the hidden
  column (`nao/gen/golang/testdata/changesets.go.golden:39`, §formats).
- An update route validates the row an earlier read returned and writes
  only the marked columns, so a concurrent write in between lets a
  cross-column validator pass on a row that is never stored
  (`gen/router/testdata/blog_app_router.go.golden:277`). The read and
  the write go in one write transaction (§query-routes).
- `[]byte` setters and `From` keep the caller's slice, so a buffer
  reused across rows stores its last contents in every row
  (`changesets.go.golden:133`).
- Time columns are declared `TIMESTAMP`, so mattn reads text it cannot
  parse as the zero time, with no error (`nao/gen/sqlite/types.go:19`;
  §go-types accepts it today).
- `GetBy` on a nullable unique column takes an `rt.Null`, and an absent
  value renders `= NULL`, which matches nothing
  (`nao/gen/golang/queries.go:175`).
- The generated client's `Header` is read by every call and written by
  callers, as §generated-client tells them to, so a `Set` during a
  call on another goroutine is a fatal concurrent map access
  (`gen/router/testdata/blog_app_client.go.golden:15`).
- Migrations carry no digest of their statements, so `MigrateUp` and
  `volt migration status` miss an applied migration that was edited
  later (§migration-ledger).
- A package that drops its last table keeps its data files, and a
  project left with nothing to generate keeps every generated file,
  against the spec's generation contract (`cmd/volt/main.go:389`).
- The overwrite guard reads any error as no file, so a dangling symlink
  at a generated path is followed and written through
  (`cmd/volt/main.go:367`); `--verify` type-checks a directory that
  still holds the stale files the run will delete, and blames the
  generator for what they break (`cmd/volt/main.go:359`).

The runtime:

- `Error(0, ...)` and `Reject(0, ...)` are answered 200 OK with a
  problem body, and `Reject(1000, ...)` drops the connection: no status
  code is checked (`errors.go:27`).
- The client takes an error's status from the body, so a 500 whose
  body says 404 matches `ErrNotFound` (`client.go:99`).
- Lowering `Replay` on a serving broker deadlocks it, since `subscribe`
  replays into a channel sized by the new value while it holds the
  mutex, and `Serve` reads `Heartbeat` without the lock
  (`events.go:42`).
- An event stream behind a `ResponseWriter` wrapper without `Flush`
  delivers nothing and reports nothing (`events.go:164`).
- A body with data after its JSON value decodes without error, and the
  GOB arm ignores trailing bytes too (`formats.go:141`).
- A malformed query pair (`?author=%zz`) is dropped, so an optional
  filter disappears and the query returns unfiltered rows
  (`request.go:27`).
- `Raw`, `After`, `OrderBy`, `GroupBy`, `PredOptional` and `PredExists`
  keep the caller's slices, so reusing a slice rewrites predicates
  already built (`nao/rt/query.go:215`).
- `RequestID` writes into the incoming request's header map, which
  net/http's `Handler` contract forbids, and echoes a client ID of any
  length (`middleware.go:21`).

The language server:

- A completion request's line is not checked against the document, so
  a line near 2^32 costs about four billion iterations on the transport
  loop (`lsp/completion.go:155`).
- No result is paired with the document version it was built from, and
  edits carry no version, so a rename sent in the debounce window after
  an edit replaces the wrong ranges (`lsp/analysis.go:205`).
- A `didClose` between the analysis goroutine's snapshot and its publish
  leaves the closed document's old diagnostics in the editor
  (`lsp/analysis.go:121`).
- A project that fails to load is dropped silently and the editor keeps
  the last diagnostics (`lsp/analysis.go:50`); a document whose
  `go.mod` disappears republishes its old project diagnostics stamped
  with the new version (`lsp/analysis.go:196`); and the rename safety
  check passes when the project cannot be loaded (`lsp/rename.go:282`).
- URIs are built as `"file://" + path` but parsed with percent-decoding,
  so a path holding `#`, `%` or `?` does not survive the round trip
  (`lsp/voltnav.go:538`).
- `didChange` applies an inverted range, which duplicates text and puts
  the server's copy out of step with the editor's, and stores versions
  without checking that they rise (`lsp/server.go:186`).
- The identifier rule is written five ways that disagree with the
  scanner's, so a non-ASCII table name misses completions, gets ranges
  that run long and draws false rename refusals
  (`lsp/voltnav.go:969`).
- The hover memo is keyed on the `SelectInfo` alone, though the hover
  reads the package's plan, merged schema and schema
  (`lsp/voltnav.go:135`).

The tools and the tests:

- `VOLT_GRAMMAR_MIRROR=.` deletes the working tree apart from `.git`,
  since the mirror directory is emptied unchecked, and a mirror inside
  `grammar/` is copied into itself (`scripts/sync-grammar/main.go:147`).
- `TestMigrateUpConcurrentRunnersApplyOnce` reaches the double-apply
  guard in none of six runs, so removing the guard leaves it green, and
  no test reaches the `no_transaction` skip or `MigrateDown`'s "another
  runner reverted it" (`nao/run/migrate_test.go:259`); a driver hook
  forces each interleaving.
- Three assertions pass on a dropped error, since the zero value they
  then read satisfies them (`nao/itest/dyn_test.go:190`,
  `nao/itest/validate_test.go:75`, `problem_test.go:126`).
- `ast.Inspect` and `ast.Children` enter no query or route declaration,
  so the three tests that walk every parsed input agree on that half of
  the language without looking at it (`lang/ast/walk.go:30`).
- The corpus walkers of the conformance tests drop errors and shrink
  what they check (`lang/conformance_test.go:188`), and the
  `internal/par` tests prove nothing on one CPU
  (`internal/par/par_test.go:24`).
- One test skips itself when `go` is missing, which AGENT.md forbids
  (`lsp/gotype_test.go:23`). Seven open SQLite through `database/sql`
  outside `nao/rt`, where AGENT.md allows it only
  (`lang/check/timetext_test.go:95`, `nao/run/run_test.go:88`,
  `nao/run/stmtcache_test.go`, `cmd/volt/migration_test.go:433`):
  either they go through `internal/dbtest`, or AGENT.md names the
  exceptions, the statement cache's tests among them, which need a
  plain `*sql.DB`.

---

## Step 37. The request bench

A tool that measures where the time of a request goes in a Volt
backend, from the moment a client means to send it to the last byte of
its response, reliably enough that the numbers can drive a redesign. It
finds bottlenecks and fixes none: nothing in the runtime, the runner or
the generators gets faster in this step, and the hooks it adds change
no behavior.

### What exists, and why this is a new tool

`internal/perfsweep`, run by `scripts/perf.go`, measures the tooling:
Load, Check, Vet and Generate over `internal/corpus` projects of
growing table counts, the best wall time of a few runs, the bytes and
allocations of one run, CPU profiles through `go tool pprof`, and the
language server's keystroke through `go test ./lsp -bench`. It never
starts a server, opens a database or sends a request, and it reports a
best time, not a distribution. `nao/itest/bench_test.go` holds `go
test` benchmarks of the runner alone (a bulk insert, statements in a
transaction). `runner.md` records one-off measurements of the runner.
No code path stamps the clock inside a request: `DB.WithLogger` logs
each statement's duration at debug level, but without the request's
context, so no line can be tied to a request.

The bench shares nothing with the sweep but the description of the
machine, so it extends nothing and replaces nothing: the sweep stays as
it is. The machine description moves to `internal/machine`, read from
files alone (`/proc`, `/etc/os-release`, `.git`), which lands step 20
for both tools, since the bench may run no other program.

### Where it lives

- `internal/bench`, standard library only: the load generator, the
  per-request recorder, the attribution, the histograms, the statistics
  across runs, the results file, the text report and the compare. It
  also holds the driver wrapper, which wraps any `driver.Driver` it is
  handed.
- `cmd/volt/bench`, a `main` in the tool's module, which links mattn:
  `go run ./cmd/volt/bench run --out <dir>` and `go run ./cmd/volt/bench
  compare <a.json> <b.json>`. It registers mattn's driver twice, plain
  as `sqlite3` and wrapped as `sqlite3-traced`, and opens the database
  through `run.Open` on a file in a temporary directory.
- `itest/bench/db` and `itest/bench/app`, the fixture: a blog and
  metrics schema and the routes and controllers the scenarios call,
  generated and committed like `itest/blog`, with the same drift test.
  `itest/bench/wide` is a project `internal/corpus` writes (one data
  package, two tables of 150 columns, its default resource routes),
  generated and committed the same way, for the wide rows scenario; the
  drift test regenerates it from `corpus.Files`.
- A smoke test runs every scenario and variant once for 300 ms and
  asserts its invariants and the attribution bound, so the verification
  bar holds the tool without running the bench.

The client runs in the same process as the server, over a real TCP
listener on 127.0.0.1, so every stamp, client and server, reads one
monotonic clock and a request's stamps line up without correction. The
price is that client and server share the cores; the floor scenarios
measure what the client costs, and every per-request cost is reported
raw and above that floor.

### The hooks in the runtime

Stage attribution needs stamps where only the runtime can take them.
The hooks change no behavior and cost one `ctx.Value` lookup per request
in the shim and one per statement in the runner when no recorder is
present.

- A package `nao/trace`, standard library only, holds the recorder: a
  `trace.Spans` with a fixed array of spans (a stage, a begin, an end
  and an inner duration) filled through an atomic index, so the steps of
  an `rt.All` record from their goroutines without a lock, and a count
  of spans that did not fit, which fails the run. `trace.SpansWith(ctx,
  spans)` and `trace.SpansOf(ctx)` carry it in a context. The callee is
  a concrete type, so the hook is a closed call (S15). The recorder is
  mutable state, so it joins the inventory of `style.md` F4 and of the
  spec. The root package imports `nao/trace` beside `nao/rt`, which
  changes the import rule of spec §data-and-network-layers in the same
  commit.
- The shim stamps the body read, each call of the handler and of a
  continuation, each perform, the encode and the response write.
  `Request` carries the recorder in an unexported field, so `Decode`
  stamps the decode inside the handler.
- The runner stamps each statement outside a transaction and each
  transaction: for a statement it puts a statement token in the context
  it hands database/sql, and stamps the end; for a transaction, the wait
  for its connection, the `BEGIN` or `BEGIN IMMEDIATE` with the busy
  timeout pragmas around it, each statement, and the `COMMIT` or
  `ROLLBACK`. The rows of a query are stamped from their first `Next`
  to their `Close`.
- The driver wrapper, in the bench only, finds the token in the context
  of each call. database/sql hands a pooled connection to a caller
  through `ResetSession(ctx)` with that caller's context, before any
  statement runs on it (`database/sql` in Go 1.27, `resetSession`), and
  opens a new one through `Connect(ctx)`; the wrapper stamps either as
  the moment the connection was granted, or its first call when neither
  came. It adds the time of `Prepare` and of each `Next` to the token,
  so a statement splits into the wait for its pool, its prepare, its
  execution in SQLite and the scan of its rows in Go. A test pins the
  `ResetSession` behavior, so a Go release that changes it fails the
  bar instead of skewing the waits.
- `run.DB.Stats` returns both pools' `sql.DBStats`, for the consistency
  checks below.

### Stages

Every request has a root span, from its scheduled send to the last byte
of its body, and spans nested inside it. Each stage is a span with a
fixed depth. Attribution sweeps the request's timeline: at each instant
the deepest spans then open share it equally, so nesting gives a span's
time to its children, the parallel steps of an `rt.All` share the wall
clock instead of counting it twice, and the stages sum to the total by
construction. A span's inner duration (the SQLite part of a statement
or of its rows) splits its share in the same ratio. The time a
container span holds outside its children is a stage of its own, named
for the code it is, so a hole in the stamps shows as a large row there,
not as a silent addition to a neighbor.

| Stage | Depth | From, to |
|---|---|---|
| send lag | 1 | the scheduled send, the call of `Transport.RoundTrip` |
| connect | 1 | `GetConn`, `GotConn` (the dial included when one was needed) |
| request write | 1 | `GotConn`, `WroteRequest` |
| transit | 1 | `WroteRequest`, `GotFirstResponseByte`, outside the server's spans: loopback, net/http reading the request and flushing the response |
| body read | 1 | the first byte, the body's end |
| routing and middleware | 2 | the outer wrapper's entry and exit, outside the shim: ServeMux, the pipeline's plugs |
| limiter wait | 3 | not built (step 16); the row prints `not built` |
| shim | 3 | the shim's entry and exit, outside its children |
| server body read | 4 | `bodyRead` |
| handler | 4 | each call of the handler, a continuation or an error handler, outside decode |
| decode | 5 | `Decode` |
| effect interpretation | 4 | each perform, outside the runner's spans: `nao/rt` and the statement cache |
| read pool wait | 6 | the token, the grant of a reader |
| writer wait | 6 | the token, the grant of the writer |
| begin | 6 | `BEGIN` or `BEGIN IMMEDIATE` with its pragmas and busy retries |
| prepare | 6 | the driver's `Prepare` |
| statement exec | 6 | the driver's `Exec`, `Query` and every `Next` |
| row scan | 6 | the rows' span outside `Next`: database/sql and the generated `Scan` |
| commit | 6 | `COMMIT` |
| rollback | 6 | `ROLLBACK` |
| encode | 4 | `renderEncoded` |
| response write | 4 | `Response.write` |
| unattributed | 0 | the instants no span covers: the client's own code between stamps, and every stamp that went missing |

A write outside a transaction commits inside its statement, so its
commit is part of `statement exec`; the `commit` row holds the commits
of transactions alone. A run fails when the unattributed time passes
1% of the total, when a span did not fit, or when a consistency check
fails: the writer waits summed over the requests against the growth of
the writer pool's `WaitDuration`, the same for the readers, within 10%;
and the server spans of a request inside its client `transit` span.
Time the Go runtime takes from a request, a collection or a scheduler
delay, is charged to the stage it interrupted; the report prints the
run's GC CPU fraction and pause total beside the stages.

### Measures

Per stage and request class (a scenario mixing reads and writes reports
each route on its own): the share of the total time, and p50, p99,
p99.9 and max over the requests where the stage occurred, with that
count. Histograms are log-linear, 128 sub-buckets per power of two from
100 ns to 100 s, so a percentile is within 0.8%; the maximum is exact.

Per run: the offered and the achieved throughput; completed requests by
status, transport errors and requests the client never sent; the peak
RSS (`/proc/self/status`, sampled every 10 ms), the peak Go heap and
the peak goroutine count (sampled the same); allocations and CPU time
per request (`runtime/metrics` and `/proc/self/stat` over the measured
window), raw and above the null floor; the peak size of the WAL file
and the peak count of frames in the WAL, read from the `-shm` index
header (both copies compared, a torn read retried); the database file's
size at the end.

### Load

- **Open loop**, the default. One dispatcher computes the schedule,
  `start + i/rate`, and hands each request to a pool of client workers
  at its time; latency is measured from the scheduled time, so a
  server that stalls is charged for every request it held up and a late
  dispatcher shows as `send lag`. The pool is bounded (4,096 in flight
  by default); a request with no free worker waits in the dispatcher's
  bounded queue, and when that is full it is never sent and counted, and
  a run with any is labeled saturated.
- **Closed loop**, labeled so in every line of the report: C workers,
  each sending its next request when the last one ends, latency from the
  actual send. It hides queueing, and it calibrates the overload
  scenario.
- **Warm-up**: 2 s at the scenario's rate, discarded with every counter
  and peak reset after it; then a measured window of 10 s.
- **Seeds**: one `math/rand/v2` PCG seed per scenario, in the results;
  every request's target and body, and every seeded row, is a function
  of the seed and the request's index.
- **Data**: every scenario's sizes are constants of the scenario. A
  database is seeded once per scenario, checkpointed, and copied for
  each run, so each run starts from the same file.
- **Runs**: every scenario and variant runs N = 5 times, in a
  round-robin order across the scenarios so a slow minute of the
  machine spreads over all of them. The report prints each metric's
  median and its spread, (max - min) / median, across the runs.

### Reliability

- **Noise floor.** Three reference configurations (floor-volt,
  read-mostly, uniform writes at NORMAL) run 2N times interleaved. For
  every metric and stage, the 126 ways to split the ten runs into two
  halves of five give 126 differences of medians; the 95th percentile of
  their size is the noise floor of that metric. The report prints it.
- **Differences.** A compare reports a difference only when the change
  of medians is larger than the noise floor of the metric and the two
  sides' ranges of run values do not overlap; anything else prints `no
  difference`.
- **Cost of the instrumentation.** The same reference configurations,
  and N+1 sequential (the most spans), run N times with the recorder,
  the wrapper and `httptrace` off, stamping only the scheduled send and
  the body's end: the total's p50 and p99, the closed loop throughput,
  the CPU and the allocations per request, on against off, with the
  noise floor verdict. The cost of the hooks being present but off, one
  `ctx.Value` per request and per statement, is measured by a
  microbenchmark and reported beside it.
- **Correctness.** Every scenario asserts its invariants after the run
  and checks every response body as it arrives; every run ends with
  `PRAGMA integrity_check` and `foreign_key_check`. A run that breaks
  one fails, however fast it was. The hot row scenario has a negative
  control that must fail.

### Output

- `results.json`: the format's version, the machine, the commit, the
  flags, and per scenario, variant and run the configuration, the seed,
  every histogram's buckets (so the compare merges runs and recomputes
  percentiles), the run measures, the invariants and the sketch.
- `report.txt`: plain text. Per scenario and variant, one line per
  stage: its name, share, a bar of `#` scaled to the share, p50, p99,
  p99.9 and max; then the run measures, the spread, the sketch's
  expected dominant stages, and a `SURPRISE` line for every stage that
  dominates against the sketch and every total outside twice the
  sketched range.
- `compare` prints two results side by side per scenario, variant and
  stage, with the delta and its verdict.

### The matrix

| Resource | Bandwidth | Latency |
|---|---|---|
| Network | large bodies, slow clients | floor, ramp, connection per request |
| Disk | bulk ingestion, large request bodies, sustained | `synchronous = FULL`, checkpoint stall |
| Memory | larger than the page cache (scans), large responses, rollup scans | larger than the page cache (point reads), reads behind commits |
| CPU | overload, wide rows, N+1 | hot row, floor-volt |

Two cells cannot be measured here. Network latency of a real link:
loopback has a round trip of tens of microseconds, and shaping it needs
a tool outside Go. Disk latency of reads: every database fits in the
machine's 16 GB, so reads miss SQLite's cache but hit the operating
system's; a cold read needs a file larger than memory or eviction of
the system cache, which the tool does not do.

### Variants

Every write scenario runs under `synchronous = NORMAL`, mattn's default
and so today's, and `synchronous = FULL`, which the runner supports today
through `_synchronous=FULL` in the DSN; the FULL rate is lower, since a
commit then waits for an fsync. Named slots, printed as `not built`
with the step that would build them: `group-commit` (`runner.md`
section 7), `limiter` (step 16), `begin-without-pragmas` (step 23
removes the busy timeout read-back around `BEGIN IMMEDIATE`),
`readers-capped` (step 21, question 1) and `chunk-1000` for the bulk
insert (step 21, question 2).

### The sketch

Assumed before measuring, from `runner.md` where it has a number: a
loopback request through net/http costs about 40 µs of CPU, client and
server together, so the harness tops out near 40,000 requests a second
on four cores; the shim, the router and a small JSON encode about 4 µs;
a point read on a reader about 5 µs; a one-row write transaction about
30 µs (`runner.md` section 1); `BEGIN IMMEDIATE` with its three pragma
statements about 15 µs; a commit under NORMAL about 10 µs; an fsync on
this machine unknown, guessed at 1 ms; a scanned row about 1.2 µs
(`runner.md` section 5); a row of `CreateMany` about 3.7 µs (section 4);
encoding/json about 250 MB/s encoding and 80 MB/s decoding structs.
Writes serialize on one connection, so a writing route's capacity is one
over its transaction's time, whatever the cores.

### Scenarios

Rates are open loop unless the scenario says otherwise. Each scenario
lists what it models, what it measures, its load, the sketch (the stage
expected to dominate and why) and the invariant.

1. **floor-null.** A handler that writes two bytes, outside Volt.
   Measures the harness: client, net/http, loopback. 5,000/s. Sketch:
   `transit` and `request write` dominate, p50 near 50 µs. Invariant:
   every body is the two bytes.
2. **floor-volt.** `GET /` rendering a small value, no effect. Measures
   the shim, the router and the encode above floor-null. 5,000/s.
   Sketch: the same as floor-null plus 4 µs, with `shim`, `encode` and
   `routing and middleware` under 10% together. Invariant: every body
   decodes to the value.
3. **hot-row.** An organization's monthly ingest quota: every client
   claims a unit of the same quota row at once, and the quota must never
   be oversold. `POST /orgs/1/claims` runs `rt.Atomic` of the quota's
   read and its update, 409 when nothing is left; the quota is sized to
   run out halfway through the window, so claims and refusals report as
   two classes. Measures serialization on the writer. 15,000/s; FULL at
   500/s. Sketch: a transaction of about 45 µs (begin 15, read 5,
   update 10, commit 10, interpretation 5) gives about 22,000 a second,
   so at 15,000 the writer is two-thirds busy: `writer wait` dominates
   at p99 and p99.9, `transit` at p50. Under FULL the fsync dominates
   `commit` and the capacity falls to about 1,000. Invariant: claims at
   most the initial quota, remaining = initial - claims, remaining never
   below zero, claims + refusals = completed, no 5xx. Negative control,
   once: the same claim without `rt.Atomic` (read on a reader, write on
   the writer) must fail the invariant.
4. **uniform-writes.** The same claim spread over 10,000 quotas chosen
   uniformly, the control for hot-row. Same rates. Sketch: SQLite locks
   the database, not a row, so it matches hot-row within the noise
   floor; distinct pages per commit could make it a few percent slower
   and its checkpoints larger. Invariant: the sum of remaining = the
   sum of initial - claims, none below zero.
5. **read-mostly.** A blog's post pages: 99% `GET /posts/:id`, 1%
   `POST /posts/:id/comments`. 100,000 posts of 500 bytes. 20,000/s.
   Sketch: reads are `transit` and `request write` first (about 70%),
   then `statement exec` (about 10%) and `encode`; the 200 commits a
   second reset the readers' caches (`runner.md` section 2), a few
   percent on `statement exec` against a read-only run. Writes: `writer
   wait` near zero, `statement exec` first. Invariant: every post body
   is its seeded title and text; comments in the database = initial +
   201s.
6. **multi-statement.** Publishing a post: read the author, insert the
   post, insert three tags, update the author's post count, in one
   transaction. 7,000/s; FULL at 500/s. Sketch: about 70 µs a
   transaction, capacity about 14,000; `statement exec` dominates (six
   statements, about 45 µs), `writer wait` at p99. Under FULL, `commit`.
   Invariant: per author, the count = the posts in the database, every
   post has its three tags, none is half written.
7. **n-plus-one.** A front page of 50 posts with their authors' names,
   in four forms: 50 `UserGet` in sequence inside one effect, 50 in an
   `rt.All`, `UserGetMany` of the ids, and a `with` nested load.
   2,000/s. Sketch: sequential, 51 statements of 5 µs plus interpretation,
   about 300 µs, `statement exec` and `effect interpretation` dominate;
   parallel, 51 goroutines over 4 readers, `read pool wait` dominates
   and the wall time falls about threefold; batched and nested, two
   statements and 50 scanned rows, `encode` and `row scan` dominate,
   about 80 µs. Invariant: the four forms answer byte-identical bodies
   for the same page, every name is its author's seeded name.
8. **bulk-vs-single.** Metrics ingestion, 10,000 samples a second: one
   `POST /samples` per sample (`Create`), against `POST /samples/bulk`
   with 1,000 samples (`CreateMany`) ten times a second. FULL at a tenth
   of the rate. Sketch: single costs about 70 µs per sample (`transit`
   40, the write 30) and `writer wait` grows at p99; bulk about 6 µs per
   sample: `statement exec` (3.7 ms a request) first, then `decode` (60
   KB of JSON, about 1 ms) and validation in `handler`. Invariant:
   samples in the database = samples accepted, each present by its key.
9. **ramp.** A notification sends a thousand clients to the blog at
   once: from an idle server, 1,000 new connections within 10 ms, then
   10,000/s over them for 5 s; the first second and the rest report
   apart. Variant: a new connection per request. Sketch: `connect`
   dominates the first second, since net/http accepts on one goroutine
   and the readers open lazily, then the run looks like read-mostly; per
   connection, `connect` dominates every request at about 30 µs more.
   Invariant: every request answered 200 with its post, no connection
   reset.
10. **slow-clients.** Mobile clients on a slow link exporting a 256 KB
    listing at 64 KB/s, 100 of them in closed loop with a 16 KB receive
    buffer, while 2,000/s fast clients read posts. Sketch: for the slow,
    `body read` and `response write` take about 4 s; Volt encodes the
    whole body before writing and holds no connection to the database
    while it writes, so the fast clients match read-mostly within the
    noise floor, and memory grows by the bodies held, about 25 MB.
    Invariant: every export arrives whole, its length and checksum
    matching the seeded listing.
11. **checkpoint-stall.** A metrics dashboard: 3,000 sample inserts a
    second while `GET /rollup` aggregates two million samples every two
    seconds; variant: the bench holds a read transaction open for 5 s
    through `run.OpenReadOnly`, as a backup would. Sketch: a reader's
    snapshot pins the WAL, so the WAL grows by a frame or two a commit,
    about 20 MB a second, and once the reader ends the next commit past
    the autocheckpoint copies the backlog and syncs twice: a stall of
    100 ms or more in that write's `statement exec`, so p99.9 and max of
    the inserts rise while p50 does not; the WAL file's peak grows to the
    backlog and stays. The rollup itself is `statement exec`, 200 to 400
    ms. Invariant: every insert present; successive rollup counts never
    fall and never exceed the inserts acknowledged before they began.
12. **large-bodies.** A post of 1 MB written (`POST /posts`, 100/s) and
    read (`GET /posts/:id`, 200/s); a 5 MB body, one in a hundred, must
    be refused with 413 (`MaxBodyBytes` is 4 MiB). Sketch: writes go to
    `statement exec`, 256 overflow pages a post, about 100 MB/s of WAL,
    then `server body read` and `decode`; reads go to `encode` (escaping
    1 MB, about 3 ms) and then `statement exec`, `response write` and
    `body read`. Invariant: what is read back equals what was written,
    every oversized body answered 413.
13. **larger-than-cache.** A samples table of 4 million rows, about 300
    MB, 150 times a reader's 2 MB page cache, smaller than the system's
    cache: random `GET /samples/:id` at 10,000/s, and a range of 1,000
    samples at 500/s. Sketch: the interior pages stay cached, the leaves
    miss to the system's cache, so `statement exec` grows from 5 to
    about 10 µs and `transit` still dominates point reads; the ranges
    are `row scan` and `encode`. Invariant: every sample equals its
    seeded value.
14. **wide-rows.** The corpus project's 150-column metrics tables
    through their default resource routes: reads at 5,000/s and creates
    at 2,000/s. Sketch: per column costs dominate, about 50 ns each in
    `statement exec` and `row scan` and as much in `encode`, so about
    25 µs a read, near `transit`; creates add `decode` and the cast and
    validation in `handler`. Invariant: every row read back equals the
    row created.
15. **overload.** The uniform-writes claim and the read-mostly read, at
    0.5, 1, 1.5, 2 and 3 times the capacity a closed loop calibration
    finds, 10 s each; variant: the route with `timeout: 1`. Sketch:
    nothing sheds load, since there is no limiter: past capacity the
    writes queue in database/sql's waiters and the goroutines grow with
    the backlog, so latency grows through the window to seconds, and the
    client's in-flight bound is reached and counted. With `timeout: 1`,
    a write that waits a second fails with 504, so the achieved rate
    stays at capacity, latency is capped near 1 s and the 504s are the
    shed. Reads past capacity slow every request alike, all four cores
    busy. Invariant: claims persisted = claims answered 201, no 5xx but
    504.
16. **sustained.** The read-mostly mix with the ingestion of scenario 8
    and the comments, at half their rates, for 5 minutes, N = 3, reported
    in windows of 10 s. Sketch: flat: the WAL stays near 1,000 frames
    under the autocheckpoint, the caches are bounded, the heap and the
    goroutines level after warm-up, and only the database file grows,
    linearly. Invariant: the counts of scenario 5 and 8, and the
    goroutines at the end equal those at the start.

The first full run is about 75 minutes on this machine. Its results,
the bar chart of every scenario, the sketch against the measurement
with every surprise, the noise floor, the cost of the instrumentation,
a ranked list of what dominates and what the bench cannot see go into a
new `runner.md` section in the commit that lands the bench; the spec
gains the hooks (§values-and-effects for the inventory,
§data-and-network-layers for the import, the runner's paragraph for the
statement token), `style.md` F4 the recorder, `AGENT.md` the command,
and the roadmap its line.

### Decisions the maintainer makes before it is built

1. The hooks live in the runtime, in a new `nao/trace` imported by the
   root package and `nao/run`; the alternative, `rt.Spans` inside
   `nao/rt`, keeps the import rule but puts a recorder among the values.
2. `Request` carries the recorder so that `Decode` is a stage; without
   it, decode is counted in `handler`.
3. The recorder joins the inventoried mutable state.
4. The wait for a pool is measured through `ResetSession` in a driver
   wrapper, a property of database/sql that a test pins, rather than by
   restructuring how the runner takes its connections.
5. The wide rows fixture is committed generated code from the corpus,
   about two tables of 150 columns.
6. Step 20 lands with this step, through `internal/machine`.

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
| 35 | | spec first |
| 36 | | first |
| 37 | 20 | after the maintainer's review of its design |

The blog fixture under `itest/blog` is the application every step
rebuilds. Measured at each step on it and reported when the step
lands: Volt lines, hand-written Go lines, generated Go lines. The
thesis is that the first two fall while the third is paid for once.
