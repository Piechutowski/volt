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

## Step 38. The stress suite for backends

A design, under review; nothing here is built. It is a catalogue of
load cases that push a running Volt backend to its limits on the four
resources of `style.md` P1, network, disk, memory and CPU, each by
bandwidth and latency. Every case models something that happens to a
real application, measures one thing with a name, and asserts an
invariant, so that a fast wrong answer fails. The request bench of
step 37 is the instrument; this step is what it is pointed at. The
sketches below were written before anything was run: none of their
numbers is a measurement.

### What exists today

Nothing drives a running backend. No test has more than one client,
and nothing measures the network, memory over time, or the disk
beyond ten thousand inserted rows.

- `internal/perfsweep`, run by `scripts/perf.go` and
  `lang/profile_sweep_test.go`, times the tooling: load, check, vet
  and generate over growing table counts, and the language server's
  keystroke. Its subject is not a backend.
- `internal/corpus` writes projects of any size, and `volt stress`
  writes one to a directory. They carry no rows and are never served.
  The corpus leaves out indexes, joins, aggregates, `with`, loads and
  bodies (step 34, V7). Three cases below use it for the sizes of the
  program itself: columns, routes and tables.
- `nao/itest/bench_test.go` has two benchmarks of the data layer on
  one goroutine: ten thousand creates in one transaction, and
  `CreateMany` of ten thousand.
- `nao/run`'s tests prove the two pools, the statement cache under
  eviction and the migration ledger for correctness, each with a
  handful of goroutines.
- `runner.md` holds the only numbers: lookups by connection count,
  one-row writes, readers behind a committing writer, statements
  reused in a transaction, the bulk insert, and other data layers. All
  were taken by hand on one 4-core container, below HTTP, mostly with
  `context.Background()`. The sketches below start from them.
- Step 21 lists four open questions of the runner, step 23 the limits
  that are missing with three measured hazards, and step 34 the
  failure injection. Cases below measure three of step 21's four
  questions (`PostReadCores`, `SampleImport`, `PostSearch`), the one
  on quoted identifiers being no matter of load, and guard step 23's
  limits once they land.

**The request bench of step 37,** designed and not built, comes
nearest: a load generator, a recorder that gives every instant of a
request to a stage, a fixture in the two domains, a noise floor, and
sixteen scenarios. Two of them are the harness's own floor, and each
of the other fourteen is the steady arm of a case here:

| Step 37's scenario | The case here |
|---|---|
| floor-null, floor-volt | none: the floor every case is read against |
| hot-row | `SeatClaimHot`, steady |
| uniform-writes | `SeatClaimSpread`, steady |
| read-mostly | `PostMixSweep` at 99/1 |
| multi-statement | `AuditWrite` |
| n-plus-one | `PostFeed` |
| bulk-vs-single | `SampleAppend` and `SampleAppendBulk` |
| ramp | `ConnectionChurn`, and the herd |
| slow-clients | `ClientSlow`, its slow readers |
| checkpoint-stall | `WALReaderPinned`, its one pinned reader |
| large-bodies | `UploadParallel` |
| larger-than-cache | `SampleReadCold`, its cached arm |
| wide-rows | `ReportCreateWide` |
| overload | `SampleAppendOverload` and `PostReadOverload` |
| sustained | `ProcessSoak`, for five minutes |

Where a scenario is an arm of a case, the suite runs the scenario and
adds the other arms; nothing is measured twice. What the bench does
not have, and this step adds: the arrivals other than a steady rate
and one ramp; clients that vanish, cancel, send slowly, sit idle or
send large headers; what is done to a server while it runs, a
restart, a second writer on the file, a backup, a migration, a job;
the two cells its matrix leaves empty, the latency of a real link and
the cold read, which the relay and the eviction below fill in Go; the
standing probes; and the forty-six cases no scenario touches.

### Questions for the maintainer

1. **One claim or two.** `SeatClaimHot` is step 37's hot-row on
   another table: the seats of a discussion here, the units of an
   organization's quota there. The fixture should carry one. The
   seats are the story this step was given, the quota is already in
   step 37; either serves, and the other step's name follows the
   choice.
2. **A process that can die.** Step 37 runs client and server in one
   process, for the one clock, and this step follows it. Two arms
   need a process that is killed: the kill of `ServerRestart` and the
   crash of `CommitSync`. They wait for the child process of step
   34's V4, which AGENT.md's rule on what a test may run has to admit
   first. `MigrationLive` runs the `volt` the suite built, which the
   rule allows today.
3. **The word.** `volt stress` writes a corpus project to time the
   tooling; this suite loads a backend. One name, two meanings
   (`style.md` N8). Proposed: the command becomes `volt corpus`, after
   the package it wraps, and "stress" means this suite.
4. **Stand-ins for what is not built.** Sessions are a table, jobs a
   table polled over HTTP, the password hash is `crypto/pbkdf2`, and
   serving is a hand-written `main` with every `http.Server` option
   explicit. What the suite measures on them becomes the baseline for
   steps 13 to 16; their designs are not decided here.
5. **The smoke tier's clock.** Step 37's smoke test runs each scenario
   for 300 ms. The smoke runs here are sized in requests, not in time,
   so that the tier reads no clock and asserts the same thing on a
   slow machine; its budget is fifteen seconds on four cores.

### Method

**The instrument is step 37's.** A case is a scenario of the bench,
run by name and tier through `cmd/volt/bench`. The suite takes from
it, unchanged: the one process with a real listener on 127.0.0.1; the
load generator, open loop by default with latency from the scheduled
send, and a closed loop to find capacity; the warm-up and the window;
the seeds; the recorder and the stages; the measures of a run; the
five runs and the noise floor; the results file, the report and the
compare. "Twice capacity" and "ten times capacity" below mean the
capacity that closed loop finds, as in its overload scenario.

A sketch below names the stage expected to dominate in step 37's
stages: the wait for the writer is `writer wait`, the wait for a
reader `read pool wait`, a transaction's fixed cost `begin` and
`commit`, SQLite `statement exec` and `row scan`, and net/http the
client's stages and `routing and middleware`. A case fails on an
invariant. It is degraded on its own signal, or on the bench's
`SURPRISE`: a stage that dominates against the sketch, or a total
outside twice the sketched range, each judged past the noise floor.
No time is asserted.

**What this step adds to the instrument:**

- **Arrivals.** The bench's schedule, a function of the seed, gains
  shapes: a ramp, a step, a periodic burst, a herd released when the
  server opens, and clients that send again on an error or a timeout.
- **Clients.** A client that closes its connection at a seeded point
  of a request, one that cancels, one that writes or reads a few
  bytes at a time on sockets with small buffers, connections that sit
  idle, event streams, and requests with oversized headers.
- **Actions.** At a scheduled time the suite does something to the
  server: stops it and starts it again, opens a second `run.DB` on
  its file, holds a read transaction, runs a backup, runs
  `volt migration up` with the `volt` it built, or starts a job loop
  in `main`.
- **The ledger.** The client records every request and its answer. A
  request whose connection ended before its answer is unknown, and an
  invariant brackets it: the rows stored lie between the acknowledged
  and the acknowledged plus the unknown.
- **Standing invariants,** beside the bench's checks of every body,
  `integrity_check` and `foreign_key_check`: a response's
  `Content-Length` is right and its status is one the case allows, a
  500 never among them; every row read equals what the seeder or an
  acknowledged write stored, by a checksum column derived from the
  key and the version; a read that starts after a write was
  acknowledged sees it; at the end every counter equals its count
  (V7); and once the load stops, goroutines, descriptors and reader
  connections are back at their baseline (V10).
- **Standing probes,** beside every case of the full and soak tiers:
  a health check a second with a one-second deadline, one route that
  touches no database and one that reads a row, as a load balancer
  would; and a canary read and a canary write, ten a second each,
  which are the victims whose latency the cases report. The story
  "health checks during overload" is this probe: its misses are a
  column of every report, and `AuthorLogin` and `PostReadOverload`
  name it in their signal.
- **Arms.** An arm is one setting of the one thing a case varies; the
  bench calls it a variant. A case's chart has one bar or one line
  per arm.

Heap, goroutines and descriptors are the process's, the client's
included; the floor scenarios give the client's part, and a slope or
a return to baseline does not depend on it.

The sketches use step 37's assumptions where it has one, and five
more:

| Name | What | Value | From |
|---|---|---|---|
| `requestHTTP` | CPU of one keep-alive request with a small body and no effect | 40 µs | step 37, which counts client and server together; its floor scenarios measure both |
| `lookup` | a point read on a reader | 5 µs | step 37; `runner.md` 1 and 5 |
| `write` | one row written and committed; `runner.md` does not say whether as a bare statement or a transaction of one, and the floor measures both | 30 µs | step 37; `runner.md` 1 |
| `rowTx` | one statement inside a transaction, its row read back | 10 µs | step 37; `runner.md` 3 |
| `rowBulk` | one row of a `CreateMany` | 4 µs | step 37; `runner.md` 4 |
| `rowList` | one row listed; about double under a request's context, which costs a goroutine per row | 1.2 µs | step 37; `runner.md` 5 and 6 |
| `txFixed` | what a write transaction adds to its statements: `BEGIN IMMEDIATE` with the busy timeout's three statements around it, 15 µs, and `COMMIT`, 10 | 25 µs | step 37 |
| `fsync` | one fsync in the database's directory | 1 ms | step 37's guess; measured first |
| `jsonDecode`, `jsonEncode` | encoding/json | 80 and 250 MB/s | step 37 |
| `rowScan` | one row walked and not returned: an aggregate, an offset, a filter | 0.15 µs | estimate |
| `prepare` | compiling one statement | 6 µs | `runner.md` 3 |
| `frame` | one page in the WAL; SQLite checkpoints on the commit that brings the WAL to 1,000 frames, about 4 MB | 4,120 B | mattn's build |
| `deviceRead` | one 4 KB read the OS has not cached | 100 µs | measured first |
| `connectionIdle` | memory of one idle keep-alive connection | 15 KB | estimate |
| R | the readers: `GOMAXPROCS` | 4 | `runner.md` 1 |

What the cases need that is usually another tool, in Go or out of
reach:

| Need | Usual tool | In Go | Verdict |
|---|---|---|---|
| Delay, a bandwidth cap, stalls and resets on the network | `tc`, a fault proxy | a TCP relay between client and server: a token bucket, a delay queue, a paused direction, a reset through `SetLinger(0)` | designed; no packet loss and no reordering |
| Slow clients | a slow HTTP tool | the client writes and reads a few bytes at a time on sockets with small buffers | designed |
| Many source ports | several machines | the client dials from several loopback addresses | designed, Linux |
| A slow disk | cgroup I/O limits, a delay device | nothing: mattn exposes no VFS. The suite takes the directory as a flag, measures its `fsync` and `deviceRead` first, and `synchronous = FULL` puts the disk into every commit | out of reach; the disk is given, not shaped |
| A cold cache | `drop_caches` as root | `posix_fadvise` with `DONTNEED` on the database file after a sync, a raw system call | designed, Linux only (S14: one more build constraint, justified here) |
| A database larger than memory | a memory cgroup | nothing bounds the OS cache from Go without privileges. Stand-ins: the cold start, continuous eviction, which is pessimistic since it evicts interior pages too, and a file really larger than memory in the soak tier | partly out of reach |
| A memory limit | a memory cgroup | `debug.SetMemoryLimit`, the Go heap only | partial |
| Fewer cores | `taskset`, a CPU quota | `GOMAXPROCS` | partial: no quota throttling, and the client shares the cores |
| A descriptor limit | `ulimit` | `syscall.Setrlimit` | designed |
| A restart | a shell | in the process: the listener, the streams and the database are closed and opened again | designed |
| A kill | a shell | a child process | needs question 2 |
| A second writer on the file | another process | a second `run.DB` on the same file, which contends for SQLite's write lock as a second process does; and the `volt` the suite built, for `volt migration up` | designed |
| A full disk | a small mount | `PRAGMA max_page_count` (V4): the database fills, the file system does not | partial |
| TLS | | `crypto/tls` with a certificate made at start | designed |
| Power loss | | | out of reach |

### The fixture

Step 37's, under `itest/bench`, in the two neutral domains, with
what the cases need added to it:

| Package | Added | For |
|---|---|---|
| blog | on `posts`: a unique `slug`, a `version`, a `view_count`, a `comment_count`, a body of about 1 KB; `post_tags` | the read stories, the counter, the edit conflict |
| blog | `discussions`, `seats`, unless the quota stands for them (question 1) | the limited claim: `seat_count_max`, `seat_count_taken`, one seat per author |
| blog | `sessions`, `audits`, `deliveries`, `jobs`, `uploads` | the write stories: a random token key, an append-only log, a unique idempotency key, a queue with a partial index on its ready rows, a blob column |
| metrics | `accounts`, `series`, `rollups` beside its samples | tenants, dashboards and the rollup job |
| corpus | `itest/bench/wide`, as it is | `ReportCreateWide` |
| corpus, at run time | a thousand tables | `SchemaOpenLarge` applies the DDL the generator returns for them; `RouteMatchMany` registers their seven thousand patterns on a `ServeMux` in a loop, as the generated `register` does. Neither commits generated Go |

A seeder fills them: a pure function from a seed and a size to rows,
written through `CreateMany`, as the bench seeds its own. Sizes: the
smoke tier a few thousand rows; the full tier a million posts, ten
million comments and thirty million samples, ten series of a year
each at a sample every ten seconds, a few gigabytes in all; the soak
tier what its case says.

### The axes

Every case states its value on each, in this order.

1. **Key:** `one` row; `zipf`, a few hot rows; `uniform`; `monotonic`,
   appended at the right edge of the tree; `random`, a page split
   anywhere; `moving`, a hot set that shifts.
2. **Mix:** `read`; `write`; `read-write`, a read and then a write of
   the same row; `99/1`, `90/10`, `50/50`, `1/99`; and for a write its
   verb: insert, update, delete, upsert.
3. **Arrival:** `steady`; `ramp`; `spike`, a step; `burst`, periodic;
   `herd`, everyone at once after a restart; `retry`, clients that
   send again on an error; `over2` and `over10`, sustained at twice
   and ten times capacity.
4. **Size:** `tiny` rows; `wide` rows; `blob`; `body`, a large
   request; `response`, a large one; `rows`, many in one transaction;
   `keys`, thousands in one `in`.
5. **Scale:** `cached`, the file fits in memory and has been read;
   `big`, ten times what the OS can cache; `cold`; `fragmented`, after
   many deletes; `indexed`, many indexes. Two caches are meant. Each
   connection has SQLite's own, 2 MB, 500 pages, in mattn's build, and
   a reader drops it whenever the writer has committed (`runner.md`
   2); so even `cached` data is mostly fetched from the OS, a system
   call a page, and `big` and `cold` are about the OS cache.
6. **Query:** `point`; `range`; `join`; `fan-out`, the N+1;
   `aggregate` over a large range; `count` of a large table; `sort`
   without an index; `offset`; `keyset`; `none` for a write that reads
   nothing.
7. **Transaction:** `single` statement; `several`; `long-write`;
   `long-read` during writes; `upgrade`, a read that decides a write;
   `rollback`, a high rate of refusals. In Volt an upgrade is
   `rt.Atomic` over an `rt.Then`, which begins immediately and so
   never fails to upgrade; the same `Then` without the `Atomic` reads
   on a reader and writes later, which is the race.
8. **Client:** `keep-alive`; `churn`, a connection a request; `idle`,
   many held open; `slow-read`; `slow-send`; `vanish`, gone in the
   middle; `headers`, oversized; `cancel`. Over HTTP/1.1, which a
   server without TLS speaks, a client cancels by closing its
   connection, so `cancel` and `vanish` reach the server the same way.
9. **Environment:** `steady`; `restart`; `checkpoint` running;
   `backup` running; `migration` running; `jobs` competing.

The step spike and the herd are not cases. The three same-row cases
and their three controls each run under three arrivals: steady at
half capacity; a step from a tenth of capacity to one and a half
times it for ten seconds, and back; and a herd, the server started
with every client waiting. Each reports its peak queue and the time
p99 takes to return.

### The catalogue

Sixty-two cases: thirty-eight seeded from the stories, twenty-four
added by the attack below. Tiers are S for smoke, F for full and K
for soak.

| Case | From | Cell | Tiers | Measures |
|---|---|---|---|---|
| `PostReadHot` | viral post | CPU bandwidth | F | requests a second for one row: the read path's ceiling |
| `PostReadSpread` | control: uniform reads | CPU bandwidth | F | the same over a million rows |
| `SeatClaimHot` | limited claim | CPU latency | S F | the wait for the writer when everyone wants one row |
| `SeatClaimSpread` | control: uniform writes | CPU latency | F | the same claim over ten thousand rows |
| `PostViewHot` | view counter | disk bandwidth | S F | what turning a read into a write costs |
| `PostViewSpread` | control: uniform mixed | disk bandwidth | F | the same increment spread |
| `SampleAppend` | metrics ingestion, single | disk bandwidth | F K | WAL bytes and writer time per row, one row a request |
| `SampleAppendBulk` | metrics ingestion, bulk | disk bandwidth | S F | the same per row, a thousand rows a request |
| `AuditWrite` | audit log | CPU bandwidth | S F | a transaction's fixed cost against its second statement |
| `SessionTouch` | session touch | CPU latency | F | what a blind write on every read costs, and what coalescing returns |
| `DeliveryReceive` | webhook receiver | CPU bandwidth | S F | a refused write against an accepted one |
| `DeliveryKeyOrder` | attack on `DeliveryReceive` | disk latency | F K | insert cost by key order as the index outgrows memory |
| `JobClaim` | job queue | CPU latency | S F K | writer time per job and per empty poll |
| `SampleImport` | bulk import | CPU latency | S F | a victim's write latency by rows per chunk |
| `SampleAggregate` | dashboard | CPU latency | F | a short read's wait behind long reads |
| `PostFeed` | feed | CPU bandwidth | S F | a page's cost by statements per page |
| `PostSearch` | search and autocomplete | CPU latency | S F | the time from a cancel to a free reader |
| `PostCrawl` | crawler | CPU bandwidth | S F | a page's cost by depth |
| `PostExport` | export | memory bandwidth | S F | heap bytes per exported row, and how long the snapshot is held |
| `RollupRecompute` | cache stampede | CPU latency | F | recomputations per expiry |
| `PostCreateRace` | unique slug | CPU latency | S F | winners per contested value |
| `PostUpdateConflict` | edit conflict | CPU latency | S F | stored rows no validator accepts, and silent overwrites |
| `CommentListRace` | delete during read | CPU latency | S F | torn reads per thousand, by spelling |
| `AuthorLogin` | login storm | CPU latency | F | a small request's wait for a processor |
| `ReportDecodeNested` | large JSON bodies | CPU bandwidth | S F | CPU and heap per megabyte of JSON |
| `ReportCreateWide` | heavy validation | CPU bandwidth | F | a write's cost per column, by stage |
| `RouteMatchMany` | thousands of routes | CPU bandwidth | F | match time by route count |
| `StreamIdle` | many idle connections | memory bandwidth | F K | resident bytes per idle connection |
| `UploadParallel` | large uploads and downloads | memory bandwidth | S F | heap bytes per megabyte in flight |
| `ProcessSoak` | a slow leak | memory bandwidth | K | the slope of every resource over hours |
| `HeapPressureTail` | large heap | memory latency | F | the tail of small reads under allocation pressure |
| `ClientSlow` | slow clients | network latency | F | fast clients' latency and memory pinned, per slow client |
| `ConnectionChurn` | connection churn | network latency | F | the cost of a connection |
| `ClientVanish` | clients that vanish | network latency | S F | work done after the client left, and atomicity |
| `WALReaderPinned` | long reader blocks the checkpoint | disk latency | F K | WAL bytes while a snapshot is held, and the stall after |
| `SampleReadCold` | database larger than memory | disk latency | F K | device reads per lookup |
| `BackupLive` | backup during traffic | disk bandwidth | F | whether a backup finishes, and whether it is one commit |
| `MigrationLive` | index creation during traffic | disk bandwidth | F | write stall and refusals while a migration holds the lock |
| `AccountWriteImbalance` | tenant imbalance | CPU latency | F | a light client's share of the writer |
| `PostMixSweep` | attack on `PostReadHot` | memory latency | F | read cost by write share |
| `PostReadCores` | attack on `PostReadHot` | CPU bandwidth | F | speedup per core |
| `PostReadTrending` | attack on `PostReadSpread` | disk latency | F | the latency peak when the hot set moves |
| `SampleAppendOverload` | attack on `SeatClaimHot` | CPU latency | F | goodput and queue growth past the writer's capacity |
| `CommitSync` | attack on `SeatClaimSpread` | disk latency | F | what an fsync costs a commit and a checkpoint |
| `PostWriteIndexed` | attack on `PostViewHot` | disk bandwidth | F | writer time and frames per insert, by index count |
| `SampleRetention` | attack on `SampleAppend` | disk bandwidth | F K | cost per deleted row, and the file's size after |
| `DatabaseFull` | attack on `SampleAppend` | disk capacity | S F | clean refusal and recovery at capacity |
| `SampleRollup` | attack on `SampleAppendBulk` | CPU latency | S F | write stall per second of read inside a write transaction |
| `ClientRetryStorm` | attack on `AuditWrite` | network latency | F | time to recover after a stall when clients retry |
| `CommentScanFragmented` | attack on `JobClaim` | disk latency | F | pages read per live row |
| `CommentDeleteCascade` | attack on `SampleImport` | disk bandwidth | F | WAL bytes and stall per cascaded row |
| `PostSortUnindexed` | attack on `SampleAggregate` | disk bandwidth | F | temporary bytes per sorted row |
| `PostGetMany` | attack on `PostFeed` | CPU bandwidth | S F | cost per key, by spelling |
| `PostFilterShapes` | attack on `PostSearch` | CPU bandwidth | S F | statement cache hits by distinct shapes |
| `PostReadOverload` | attack on `PostCrawl` | CPU latency | F | goodput past the cores' capacity |
| `PostReadFormats` | attack on `PostExport` | network bandwidth | S F | bytes on the wire and CPU per row, by format |
| `ServerRestart` | attack on `RollupRecompute` | network latency | F | requests failed across a restart, and the time back |
| `ServerOverlap` | attack on `PostCreateRace` | disk latency | F | write latency while two processes share the file |
| `RequestHeaderLarge` | attack on `ReportDecodeNested` | memory bandwidth | S | bytes allocated per refused header byte |
| `SchemaOpenLarge` | attack on `ReportCreateWide` | memory bandwidth | F | time to first row and bytes per connection, by schema size |
| `EventFanout` | attack on `StreamIdle` | network bandwidth | S F K | deliveries a second, and what a stalled subscriber leaves |
| `LogSinkStall` | attack on `ProcessSoak` | disk latency | F | request stall per second of sink stall |

### The coverage matrix

Each case sits in the one cell it stresses most.

| | Bandwidth | Latency |
|---|---|---|
| Network | `EventFanout`, `PostReadFormats` | `ClientSlow`, `ConnectionChurn`, `ClientVanish`, `ClientRetryStorm`, `ServerRestart` |
| Disk | `PostViewHot`, `PostViewSpread`, `SampleAppend`, `SampleAppendBulk`, `BackupLive`, `MigrationLive`, `PostWriteIndexed`, `PostSortUnindexed`, `CommentDeleteCascade`, `SampleRetention` | `DeliveryKeyOrder`, `WALReaderPinned`, `SampleReadCold`, `PostReadTrending`, `CommentScanFragmented`, `CommitSync`, `LogSinkStall`, `ServerOverlap` |
| Memory | `PostExport`, `StreamIdle`, `UploadParallel`, `ProcessSoak`, `SchemaOpenLarge`, `RequestHeaderLarge` | `HeapPressureTail`, `PostMixSweep` |
| CPU | `PostReadHot`, `PostReadSpread`, `AuditWrite`, `DeliveryReceive`, `PostFeed`, `PostCrawl`, `ReportDecodeNested`, `ReportCreateWide`, `RouteMatchMany`, `PostReadCores`, `PostFilterShapes`, `PostGetMany` | `SeatClaimHot`, `SeatClaimSpread`, `SessionTouch`, `JobClaim`, `SampleImport`, `SampleAggregate`, `PostSearch`, `RollupRecompute`, `AuthorLogin`, `AccountWriteImbalance`, `SampleRollup`, `SampleAppendOverload`, `PostReadOverload`, and the three races |

The gaps:

- **Network bandwidth** holds two cases. On loopback a byte costs
  the kernel's copy, several gigabytes a second a core, so bandwidth
  exists only where the relay caps it; `PostExport` runs a capped arm
  besides.
- **Memory latency** holds two. Processor cache misses and NUMA
  cannot be told from CPU time by this harness, and page faults and
  swap need memory pressure, which is out of reach.
- **Disk bandwidth for reads** appears only in the cold and big arms;
  everything else reads from the OS cache.
- **Disk capacity** is neither bandwidth nor latency: `DatabaseFull`
  stands outside the matrix.
- **The three races** (`PostCreateRace`, `PostUpdateConflict`,
  `CommentListRace`) assert correctness and measure no resource; they
  ride on the writer's queue.
- **The single writer** is counted as CPU latency: with
  `synchronous = NORMAL` a commit does not wait for the disk, so the
  queue for the writer is a queue for one thread's time. `CommitSync`
  shows it turning into disk latency under `FULL`.

### Pairwise coverage

The nine axes make thirty-six pairs. A pair is covered where a case
exercises two values whose interaction matters; the last column names
what was left out.

| Pair | Covered by | Skipped, and why |
|---|---|---|
| Key, Mix | the six same-row cases and controls; `SampleAppend` (monotonic insert); `DeliveryKeyOrder` (random insert); `PostReadTrending` (zipf and moving reads) | zipf and moving writes: in SQLite the hot page is the cheap end and the spread the dear one, so they fall between `SeatClaimHot` and `SeatClaimSpread` |
| Key, Arrival | spike and herd on the six; `DeliveryReceive` (random, burst); `RollupRecompute` (one, burst) | retry and overload by key: a queued request waits the same whatever its key |
| Key, Size | `SampleAppendBulk`, `SampleImport` (monotonic, rows); `UploadParallel` (one hot blob); `ReportCreateWide` (uniform, wide) | the distribution of the keys inside one `in` list |
| Key, Scale | `SampleReadCold` (uniform, big and cold); `PostReadTrending` (moving, big); `DeliveryKeyOrder` (random, big); `PostWriteIndexed` (random, indexed); `JobClaim` (monotonic, fragmented); `CommentScanFragmented` (uniform, fragmented) | none |
| Key, Query | the read pair (point); `SampleAggregate` (a monotonic range); `PostCrawl` (offset and keyset in key order) | skew under an aggregate, a join or a sort: they read ranges, and which key is popular does not reach them |
| Key, Transaction | `SeatClaimHot` and its control (upgrade); `PostCreateRace` (one, rollback); `DeliveryReceive` (random, rollback); `CommentDeleteCascade` (one parent, long-write) | key distribution under a long write: it blocks every key alike |
| Key, Client | `UploadParallel` (one hot blob, slow-send) | the rest: a connection costs the same whatever key is asked through it |
| Key, Environment | the herd on `PostReadHot` (one, restart); `CommitSync` (one and uniform, checkpoint); `SampleRetention` (monotonic, jobs) | key distribution under a backup or a migration: both work on the whole file |
| Mix, Arrival | `SampleAppendOverload` (write, overload); `PostReadOverload` (read, overload); `ClientRetryStorm` (90/10, retry); the six (spike, herd) | the mixes between, under bursts: a burst lands on the readers or on the writer, and each is covered at its extreme |
| Mix, Size | `SampleAppendBulk` (insert, rows); `UploadParallel` (insert, blob); `PostExport` (read, response); `PostGetMany` (read, keys); `ReportCreateWide` (insert and update, wide); `ReportDecodeNested` (insert, body) | deletes of blobs, whose page accounting is `CommentDeleteCascade`'s; upserts of many rows |
| Mix, Scale | `PostWriteIndexed` (insert, indexed); `DeliveryKeyOrder` (insert, big); `SampleReadCold` (read, big and cold); `CommentDeleteCascade` then `CommentScanFragmented` (delete, fragmented) | updates and upserts on cold data: one device read before the write, as `DeliveryKeyOrder` shows for inserts |
| Mix, Query | `PostMixSweep` (every mix, point and range); `PostViewHot` (read-write, point); `SampleRollup` (upsert after an aggregate); `WALReaderPinned` (1/99 beside an aggregate) | fan-out under write mixes |
| Mix, Transaction | `AuditWrite` (several); `SeatClaimHot` (update, upgrade); `SampleImport` (insert, long-write); `CommentDeleteCascade` (delete, long-write); `DeliveryReceive` (insert, rollback); `SampleRollup` (upsert); `WALReaderPinned` (read, long-read) | none |
| Mix, Client | `ClientVanish` (writes, vanish); `PostSearch` (reads, cancel); `ClientSlow` (both, slow) | writes under churn: a connection costs the same whatever it carries |
| Mix, Environment | `CommitSync` (write, checkpoint); `MigrationLive`; `BackupLive`; `ServerRestart` (90/10); `SampleRollup`, `SampleRetention` (jobs); `ServerOverlap` | none |
| Arrival, Size | `HeapPressureTail` (large responses at once); `UploadParallel` (a burst of blobs); `SampleAppendOverload` (overload with bodies queued) | ramps by size |
| Arrival, Scale | `SampleReadCold` (herd on a cold start); `PostReadTrending` (a shift is a spike onto cold keys); `ServerRestart` (cold arm) | retries on fragmented data |
| Arrival, Query | `RollupRecompute`, `SampleAggregate` (bursts of aggregates); `PostSearch` (bursts of scans); `PostReadOverload` (point) | a herd of joins: it would repeat `SampleAggregate`'s queue for a reader |
| Arrival, Transaction | `SeatClaimHot` (spike and herd on an upgrade); `SampleAppendOverload` (overload, single); `ClientRetryStorm` (retry, several); `DeliveryReceive` (burst, rollback) | none |
| Arrival, Client | `ClientRetryStorm` (retry with vanish); `ServerRestart` (a herd of reconnects); `StreamIdle` (every idle connection waking at once); `PostSearch` (bursts of cancels) | overload with slow clients: they add memory, not load |
| Arrival, Environment | `ServerRestart` (herd, restart); `SampleAppendOverload` (overload, checkpoint); `ClientRetryStorm` (retry while a second writer holds the lock) | a spike during a migration: it lengthens the queue `MigrationLive` already measures |
| Size, Scale | `PostExport` (response, cold arm); `PostSortUnindexed` (rows with bodies, temporary files); `DeliveryKeyOrder` (tiny, big) | wide rows on big or fragmented data: a wide row is one row's CPU |
| Size, Query | `PostGetMany` (keys, point); `PostExport` (response, range); `PostSortUnindexed` (a sort that carries 1 KB bodies); `UploadParallel` (blob, point) | blobs under an aggregate |
| Size, Transaction | `SampleImport` (rows, long-write, a refusal at the last row); `UploadParallel` (blob, single); `SampleAppendBulk` (rows, an invalid batch) | large bodies in several statements |
| Size, Client | `ClientSlow` (body with slow-send, response with slow-read); `ClientVanish` (gone in the middle of a body); `RequestHeaderLarge` | blobs under churn |
| Size, Environment | `SampleImport` (a checkpoint a chunk); `UploadParallel` (blob, checkpoint); `ServerRestart` (an export in flight at the stop) | large bodies during a backup |
| Scale, Query | `SampleReadCold` (big, point); `SampleAggregate` (big arm); `CommentScanFragmented` (fragmented, range); `PostCrawl` (cold arm, count and offset); `PostSortUnindexed` | joins on fragmented data |
| Scale, Transaction | `WALReaderPinned` (long-read); `PostWriteIndexed` (indexed, single); `CommentDeleteCascade` (one long write makes the fragmentation) | rollbacks on cold data |
| Scale, Client | none | all of it: a client meets bytes already materialized, so the data's scale ends at the reader |
| Scale, Environment | `BackupLive`, `MigrationLive` (their time grows with the file); `ServerRestart` (cold arm); `SampleRetention` (jobs make free pages); `CommitSync` (a checkpoint of random pages) | none |
| Query, Transaction | `SeatClaimHot` (point, upgrade); `JobClaim` (range, upgrade); `SampleRollup` (an aggregate inside a write); `CommentListRace` (parent and children in one read transaction or two); `WALReaderPinned` (aggregate, long-read) | none |
| Query, Client | `PostSearch` (scan, cancel); `ClientVanish` (a long read, vanish) | fan-out with slow clients |
| Query, Environment | `MigrationLive` (reads during a migration); the herd on `PostReadHot`; `WALReaderPinned` (aggregate, checkpoint) | counts during a backup |
| Transaction, Client | `ClientVanish` (several, vanish); `SeatClaimHot` (a herd half of which gives up while queued); `SampleImport` (a chunk resent after its client left) | none |
| Transaction, Environment | `WALReaderPinned` (long-read, checkpoint); `BackupLive` beside an import (long-write, backup); `ServerRestart` (a kill in the middle of several statements); `ServerOverlap` (upgrade, two processes) | rollbacks during a migration |
| Client, Environment | `ServerRestart` (idle streams and slow clients at the stop, churn at the start); `StreamIdle` | vanishing clients during a checkpoint or a backup |

### The attack

Each seeded case was asked which production incident it would not
have caught.

| Case | The incident it misses | Added |
|---|---|---|
| `PostReadHot` | reads that slow down because a writer commits beside them; the same load on sixteen cores | `PostMixSweep`, `PostReadCores` |
| `PostReadSpread` | the popular rows changing, the new ones cold | `PostReadTrending` |
| `SeatClaimHot` | the claims arriving faster than the writer for minutes, not seconds | `SampleAppendOverload` |
| `SeatClaimSpread` | the same commits on a slow disk, or with `synchronous = FULL` | `CommitSync` |
| `PostViewHot` | an index on the counter, added to sort by views | `PostWriteIndexed` |
| `PostViewSpread` | nothing new: `SessionTouch` is its blind twin | |
| `SampleAppend` | the table that never stops growing: retention, and the full disk | `SampleRetention`, `DatabaseFull` |
| `SampleAppendBulk` | the hourly job that reads what was appended | `SampleRollup` |
| `AuditWrite` | the retry that writes the audit row twice | `ClientRetryStorm` |
| `SessionTouch` | nothing new | |
| `DeliveryReceive` | random keys once the index no longer fits in memory | `DeliveryKeyOrder` |
| `JobClaim` | reads over a table that deletes have hollowed | `CommentScanFragmented` |
| `SampleImport` | the mass delete, which scatters where an import appends | `CommentDeleteCascade` |
| `SampleAggregate` | the sort no index serves, spilling to temporary files | `PostSortUnindexed` |
| `PostFeed` | the page that loads five thousand rows by their keys | `PostGetMany` |
| `PostSearch` | the search form whose optional filters multiply the statement texts | `PostFilterShapes` |
| `PostCrawl` | ten crawlers at once: more reads than the cores have | `PostReadOverload` |
| `PostExport` | the desktop client asking for the same rows in GOB over a real link | `PostReadFormats` |
| `RollupRecompute` | the stampede after a restart instead of an expiry | `ServerRestart` |
| `PostCreateRace` | two processes on the file during a deploy, both writing | `ServerOverlap` |
| `PostUpdateConflict` | nothing new: step 36 holds the defect | |
| `CommentListRace` | the cascade's own cost, which is `CommentDeleteCascade` | |
| `AuthorLogin` | nothing new: the standing health probe sees it | |
| `ReportDecodeNested` | the megabyte in the headers instead of the body | `RequestHeaderLarge` |
| `ReportCreateWide` | a thousand such tables, read by every connection at open | `SchemaOpenLarge` |
| `RouteMatchMany` | the same | |
| `StreamIdle` | streams that receive, and subscribers that stop reading | `EventFanout` |
| `UploadParallel` | the disk filling, which is `DatabaseFull` | |
| `ProcessSoak` | the day the log's sink stops reading | `LogSinkStall` |
| `HeapPressureTail` | nothing new | |
| `ClientSlow` | nothing new | |
| `ConnectionChurn` | every client reconnecting at once, which is `ServerRestart` | |
| `ClientVanish` | clients that come back and retry, which is `ClientRetryStorm` | |
| `WALReaderPinned` | nothing new | |
| `SampleReadCold` | the moving hot set, which is `PostReadTrending` | |
| `BackupLive` | nothing new | |
| `MigrationLive` | every connection reading the new schema again, which `SchemaOpenLarge` measures | |
| `AccountWriteImbalance` | overload from everyone, which is `SampleAppendOverload` | |

### Pruning

Where two cases would draw the same chart, one stays.

- **Likes** would draw `SeatClaimHot`'s chart, an `Atomic` of an
  insert and a counter on one hot row; its invariant, the counter
  equals the rows, is asserted there. Dropped.
- **The unique email** runs the code of `PostCreateRace`. Dropped.
- **The count of a large table** is an arm of `PostCrawl`, where it
  is paid on every page.
- **The N+1** is an arm of `PostFeed`: both measure statements per
  page.
- **Heavy validation** and **wide rows** are one bar, the cost per
  column; both are `ReportCreateWide`.
- **Zipf reads** are the arm of `PostReadTrending` whose hot set
  never moves.
- **Health checks during overload** are the standing probe, not a
  case.
- **Tenant imbalance by a bulk import** is `SampleImport`.
  `AccountWriteImbalance` keeps the other mechanism, the share of the
  queue.
- **The retention job's effect on a victim** is `SampleImport`'s
  curve with deletes for inserts. `SampleRetention` keeps what is its
  own: the cost of a deleted row and the size of the file.
- **A backup by `VACUUM INTO`** grows the WAL exactly as
  `WALReaderPinned` does. `BackupLive` keeps completion and
  consistency, and does not chart the WAL again.
- **`SessionTouch`** with a touch on every request should coincide
  with `PostViewSpread`. It stays for its coalescing arm.
- **Kept though predicted equal:** each same-row case and its
  control. `PostReadHot` should equal `PostReadSpread`, and
  `SeatClaimHot` should beat `SeatClaimSpread`; the equality and the
  inversion are the claims under test, since they are what makes
  SQLite's one writer unlike a database with row locks.

### The cases

Each case gives its story, its value on the nine axes in their order,
the cell it stresses, the sketch, the invariant it adds to the
standing ones, the signal, and the decision its result informs. "The
standing ones" alone means the case adds none.

**Same row, and the controls.** Each runs steady, under the spike and
under the herd.

#### PostReadHot

A post goes viral and every request reads that one row by its slug.

- **Axes:** one; read; steady, spike, herd; tiny; cached; point;
  single; keep-alive; steady.
- **Stresses:** CPU, bandwidth.
- **Sketch:** `requestHTTP` 40 µs, `lookup` 5 and the encoding of a
  1 KB row about 3: 48 µs, so net/http is over 80% of the request and
  SQLite about 10%, some 20,000 requests a second for each core the
  server has. One row costs what a million do: SQLite has no row
  locks and readers share nothing per row, so the ratio to
  `PostReadSpread` is 1.0 within a tenth. In the herd, N identical
  lookups are N statements; nothing joins them.
- **Invariant:** the standing ones; the body is the seeded post.
- **Signal:** failed on a wrong body. Degraded when SQLite's share
  passes a quarter of the CPU, or the ratio to the control leaves 0.9
  to 1.1.
- **Informs:** whether the runner performs identical reads in flight
  once. Effects are values that `rt.Same` compares, so it could, and
  a viral row would cost one statement however many ask.

#### PostReadSpread

The control: the same reads spread evenly over a million posts.

- **Axes:** uniform; read; steady, spike, herd; tiny; cached; point;
  single; keep-alive; steady.
- **Stresses:** CPU, bandwidth.
- **Sketch:** as `PostReadHot`. Every page comes from the OS cache at
  a system call each, since a reader's own 500 pages cannot hold a
  million posts.
- **Invariant:** the standing ones.
- **Signal:** degraded when it differs from `PostReadHot` by more
  than a tenth.
- **Informs:** the baseline every read case is compared with.

#### SeatClaimHot

A discussion opens 500 seats, everyone claims one at the same moment,
and it must never be oversold.

- **Axes:** one; read-write, an update and an insert; steady, spike,
  herd; tiny; cached; point; upgrade, rollback for the losers;
  keep-alive, with a herd half of which gives up after 100 ms;
  steady.
- **Stresses:** CPU, latency: the queue for the one writer.
- **Sketch:** the claim is the spec's own shape, an `rt.Atomic` over
  the `rt.Then` of `DiscussionGet` whose `next` fails with a 409 when
  the discussion is full and otherwise returns the `rt.Tx` of the
  discussion's update and the seat's insert. On the writer: `txFixed`
  25 µs, `lookup` 5, two `rowTx` 20, about 50 µs, so 20,000 claims a
  second at any number of cores, half of it the transaction's fixed
  cost. A herd of 10,000 is answered in 0.5 s; the mean wait is
  0.25 s, five thousand times the service, and the wait is the whole
  story. A loser still takes 30 µs of the writer to be told no.
  `database/sql` hands the connection to a random waiter, so p99 of
  the wait sits near the maximum.
- **Invariant:** exactly the first 500 claims committed hold seats;
  `seat_count_taken` equals the count of `seats` and never passes
  500; no author sits twice; every other answer is a 409. Step 37's
  negative control, the same claim without the `Atomic`, must fail
  it.
- **Signal:** failed on a 501st seat, a counter that is not its
  count, or a 5xx. Degraded when `txFixed` passes 70% of the writer's
  time.
- **Informs:** step 23's removal of the busy timeout's read-back;
  preparing `BEGIN` and `COMMIT` on the writer's connection; whether
  a full discussion is refused from a reader before it reaches the
  writer; the order of the writer's queue (`runner.md` 7).

#### SeatClaimSpread

The control: the same claims spread over ten thousand discussions.

- **Axes:** uniform; read-write, an update and an insert; steady,
  spike, herd; tiny; cached; point; upgrade; keep-alive; steady.
- **Stresses:** CPU, latency.
- **Sketch:** the same 50 µs of the writer. The difference is the
  checkpoint: three frames a claim either way, so one checkpoint
  every 330 claims, but the hot discussion rewrites a handful of
  pages while the spread dirties about a thousand different ones,
  which the checkpoint writes to random places. The spread should be
  10 to 30% slower: in SQLite the hot row is the cheap end.
- **Invariant:** as `SeatClaimHot`, for every discussion.
- **Signal:** degraded when the hot row is slower than the spread by
  more than a tenth, which would contradict the model.
- **Informs:** that same-row contention in Volt is a queue and never
  a lock; the checkpoint's size.

#### PostViewHot

Every read of the viral post also increments its view count; likes
and counter caches are the same shape.

- **Axes:** one; read-write, an update; steady, spike, herd; tiny;
  cached; point; single; keep-alive; steady, checkpoint.
- **Stresses:** disk, bandwidth.
- **Sketch:** one hand-written `rt.Write` that sets `view_count =
  view_count + 1` and returns the row, since an `rt.Assign` is a
  column and a value and the generated layer cannot say it. `write`
  30 µs on the writer: 33,000 views a second at any core count,
  against 20,000 a core for `PostReadHot`, so 0.4 of it on four
  server cores and 0.1 on sixteen. One frame per view, two when an
  index holds the counter: 4 KB of WAL for eight bytes changed, five
  hundred times; 135 MB/s of WAL, and a checkpoint every 30 ms.
- **Invariant:** the counts returned are each of start + 1 to start +
  N exactly once, and the final count is the start plus the answers.
- **Signal:** failed on a repeated or a missing count. Degraded when
  a view writes more than two frames.
- **Informs:** whether the dynamic layer gains an arithmetic assign;
  the checkpoint's threshold; counters added up at the runner's own
  pace (S10).

#### PostViewSpread

The control: the same increment spread over a million posts.

- **Axes:** uniform; read-write, an update; steady, spike, herd;
  tiny; cached; point; single; keep-alive; steady, checkpoint.
- **Stresses:** disk, bandwidth.
- **Sketch:** the same 30 µs and one frame a view; every commit
  dirties another page, so the checkpoint copies a thousand pages
  where the hot row's copies one. Equal to `PostViewHot` or up to 30%
  slower.
- **Invariant:** as `PostViewHot`, for every post.
- **Signal:** degraded as `SeatClaimSpread`.
- **Informs:** the checkpoint's cost by distinct pages.

**Write heavy.**

#### SampleAppend

A metrics agent posts one sample a request, forever, its keys rising.

- **Axes:** monotonic; write, insert; steady; tiny; cached, growing
  in the soak; none; single; keep-alive; steady, checkpoint.
- **Stresses:** disk, bandwidth.
- **Sketch:** `write` 30 µs: 33,000 rows a second, bound by the
  writer once two cores serve HTTP. Two frames a row, the table's
  leaf at the right edge and the leaf of the index on series and
  time: 8 KB of WAL for a 50-byte row, 160 times, 270 MB/s. A
  checkpoint every 500 rows, 66 a second, each paid by the request
  whose commit crosses the threshold: two `fsync` and the copy, so
  one request in 500 is tens of times the median.
- **Invariant:** the rows stored are the acknowledged ones, each
  once.
- **Signal:** failed on a lost or a doubled sample. Degraded when
  checkpoints pass 30% of the writer's time, or a row writes more
  than three frames.
- **Informs:** checkpointing at the runner's own pace instead of on a
  request's commit (S10); `wal_autocheckpoint` as an explicit option
  (S12).

#### SampleAppendBulk

The agent batches a thousand samples into each request.

- **Axes:** monotonic; write, insert; steady; rows, a thousand a
  request; cached; none; long-write of 4 ms, rollback for a refused
  batch; keep-alive; steady.
- **Stresses:** disk, bandwidth.
- **Sketch:** the writer spends a thousand `rowBulk`, 4 ms, a
  request: 250,000 rows a second, eight times `SampleAppend`.
  Decoding 60 KB, under a millisecond, and casting and validating,
  about one more, run off the writer. About 30 pages per thousand
  rows: 120 bytes of WAL a row, 2.5 times its size instead of 160.
- **Invariant:** the rows stored equal the sum of the counts
  answered; a batch with one invalid row stores nothing and its 422
  names the row.
- **Signal:** failed on a partial batch. Degraded under four times
  `SampleAppend`'s rows a second.
- **Informs:** the statement size of the bulk insert (step 21,
  question 2); it is P3 measured.

#### AuditWrite

Every update of a post also writes an audit row, in the same
transaction.

- **Axes:** uniform; write, an update and an insert; steady; tiny;
  cached; none; several; keep-alive; steady.
- **Stresses:** CPU, bandwidth. Arms: the update alone; the update in
  an `rt.Tx`; the update and the audit row in an `rt.Tx`.
- **Sketch:** 30, 35 and 45 µs of the writer: `write`, then `txFixed`
  and one `rowTx`, then a second. The transaction's fixed part costs
  two and a half times what the audit row does.
- **Invariant:** audit rows equal updates applied; never one without
  the other.
- **Signal:** failed on an update without its audit row. Degraded
  when the middle arm passes twice the first.
- **Informs:** step 23, where the read-back goes; whether `BEGIN
  IMMEDIATE` and `COMMIT` stay compiled on the writer's connection,
  which the design `runner.md` 3 dropped would have allowed.

#### SessionTouch

Every request, reads included, stamps the caller's session row with
the time.

- **Axes:** random, the token is the key; read-write, a read and an
  update of another row; steady; tiny; cached; point; single;
  keep-alive; steady.
- **Stresses:** CPU, latency. Arms: no touch; a touch every request;
  a touch at most once a minute for each session.
- **Sketch:** with a touch every request the whole site, reads
  included, is capped at one `write` each: 33,000 requests a second
  at any core count, the chart of `PostViewSpread`. Coalesced, the
  writes fall sixty times for a session that asks once a second, and
  the site is back within 5% of no touch at all.
- **Invariant:** `touched_at` never moves back and is never ahead of
  the request's clock; coalesced, it is at most a minute stale.
- **Signal:** degraded when the coalesced arm is more than a tenth
  from the arm without a touch.
- **Informs:** step 13: the cookie session writes nothing, and this
  is the price of giving that up; whether the runtime offers a write
  coalesced at its own pace (S10).

#### DeliveryReceive

A provider delivers webhooks in bursts and sends three in ten twice;
the idempotency key decides.

- **Axes:** random; write, insert; burst; tiny; cached; none; single,
  rollback; keep-alive; steady.
- **Stresses:** CPU, bandwidth.
- **Sketch:** an accepted delivery is a `write`, 30 µs. A refused one
  fails at the unique index, writes no frame, and pays for mapping
  the error and encoding the problem: about 12 µs, 0.4 of an
  acceptance. At 30% duplicates the writer takes 40,000 deliveries a
  second, and a burst of B drains in B times 25 µs.
- **Invariant:** one 201 for each key and a 409 that names the key's
  column for every repeat; one row a key; never a 500.
- **Signal:** failed on two rows for a key or a 5xx. Degraded when a
  refusal costs more than an acceptance.
- **Informs:** step 26, where refusals are classified by their
  message text (`nao/rt/changeset.go`); whether a repeat answers
  with the stored row.

#### DeliveryKeyOrder

The provider's keys are random; a time-ordered key would append.

- **Axes:** random against monotonic; write, insert; steady; tiny;
  cached, then big; none; single; keep-alive; steady.
- **Stresses:** disk, latency. Arms: random keys and time-ordered
  keys, as the table grows from one million rows to a hundred.
- **Sketch:** while the index fits in memory both cost a `write`;
  random keys only make the checkpoint copy a thousand pages instead
  of twenty, at most 1.3 times. Once the index outgrows the OS cache,
  each random insert first reads a leaf from the device: 30 µs and a
  `deviceRead`, 130 µs, four times, and far worse on a slower disk.
  Ordered keys stay at 30.
- **Invariant:** as `DeliveryReceive`.
- **Signal:** degraded when random costs over 1.5 times ordered
  while cached.
- **Informs:** a vet rule for a unique key of random text on a table
  that grows; the writer's `cache_size`, whose 500 pages are spent
  after one such transaction.

#### JobClaim

Handlers enqueue jobs in a table; sixty-four workers poll it, claim a
job, work, and delete it, racing for the same rows.

- **Axes:** monotonic, enqueued at the right edge and claimed at the
  left; 50/50, insert, update and delete; steady, burst; tiny;
  cached, fragmented in the soak; range, the next ready row through a
  partial index; upgrade; keep-alive; steady.
- **Stresses:** CPU, latency.
- **Sketch:** on the writer an enqueue is 30 µs, a claim, an
  `Atomic` of a select and an update, 40, a delete 30: 100 µs a job,
  10,000 jobs a second however many workers. A poll that finds
  nothing is still a write transaction, 30 µs; sixty-four idle
  workers polling every 10 ms take 19% of the writer for nothing.
- **Invariant:** every job is claimed at most once and deleted
  exactly once, and none is lost: enqueued, distinct claimed and
  deleted are equal, and the table ends empty. In the soak the file
  stops growing.
- **Signal:** failed on a job claimed twice. Degraded when empty
  polls pass a tenth of the writer, or a poll slows as jobs pass.
- **Informs:** step 15: polling on a reader and claiming on the
  writer, the interval and the batch (S10).

#### SampleImport

A customer uploads a file of a million samples while everyone else
keeps working.

- **Axes:** monotonic; write, insert; steady, with one heavy client;
  rows and body, up to the 40,000 rows a 4 MiB body holds; cached;
  none; long-write, rollback; keep-alive; steady, checkpoint.
- **Stresses:** CPU, latency. Arms: chunks of 1,000, 8,000 and
  40,000 rows.
- **Sketch:** a chunk holds the writer for its rows times `rowBulk`:
  4, 32 and 160 ms. The canary write's p99 is one chunk, up to five
  thousand times its 30 µs. The import runs at 250,000 rows a second
  from the smallest chunk up, so the smallest costs the import
  nothing. A refusal at the last row wastes the whole chunk.
- **Invariant:** every chunk lands exactly once, also when its
  client timed out and sent it again: the chunk's key is a row
  written in the same transaction, so the repeat is a 409. The rows
  stored are the file's.
- **Signal:** failed on a chunk stored twice or in part. Degraded
  when the canary's p99 passes one and a half chunks.
- **Informs:** step 23's bound on the changesets of a `CreateMany`;
  step 21, question 2.

**Read heavy.**

#### SampleAggregate

A dashboard of six panels sums a time range per panel: the last hour,
or the last year.

- **Axes:** monotonic, a time range; read; steady, burst, eight
  dashboards opening at once; tiny; cached, with a big arm;
  aggregate, range; long-read; keep-alive; steady.
- **Stresses:** CPU, latency: the queue for a reader. Arms: an hour,
  360 rows at a sample every ten seconds; a year, three million.
- **Sketch:** an hour is 360 `rowScan`, 50 µs; a year is 0.45 s of
  one reader. Forty-eight year panels on four readers
  hold every reader for 5.4 s, and a point read arriving then waits
  in `database/sql`'s queue, which knows no priority: the canary
  read's p99 goes from 50 µs to seconds. On the big arm the scan runs
  at the device's bandwidth.
- **Invariant:** each sum equals the closed form of the seeded series
  over its range.
- **Signal:** failed on a wrong sum. Degraded when the canary read's
  p99 passes ten times its unloaded value while panels run.
- **Informs:** more readers than cores, or a lane for long reads; a
  default for `timeout:` (step 23); a covering index in the fixture's
  guidance.

#### PostFeed

The front page lists twenty posts with their authors, their tags and
their comment counts.

- **Axes:** uniform, over pages; read; steady; tiny; cached; join,
  fan-out; single; keep-alive; steady.
- **Stresses:** CPU, bandwidth. Arms: a join and a `with`, three
  statements on one snapshot; four route loads in a row; a fan-out of
  sixty-one lookups in an `rt.All`.
- **Sketch:** three statements about 100 µs, encoding 20 KB about
  80, `requestHTTP` 40: 220 µs a page. The fan-out is sixty-one
  statements at about 8 µs, half a millisecond of database time, five
  times, and one request holds every reader while it runs.
- **Invariant:** the three arms answer the same page (V9); each
  comment count equals its count.
- **Signal:** failed when the arms disagree. Degraded when the first
  arm's database time passes 150 µs.
- **Informs:** whether a route's loads run in one read transaction;
  step 23's bound on the steps of an `rt.All`.

#### PostSearch

A reader types in the search box; each keystroke sends a query and
abandons the one before.

- **Axes:** uniform; read; burst; tiny; cached; range, the whole
  table through a `like`; single; cancel; steady.
- **Stresses:** CPU, latency.
- **Sketch:** a `like` on the title walks a million rows, 0.2 s,
  unless the index serves it, which is checked first; if it does, the
  pattern becomes an infix, which no index serves. At ten keystrokes
  a second each query runs 100 ms and is cancelled, so one typist
  keeps one reader busy and four saturate four. A cancelled read
  should free its reader within a millisecond. Under a request's
  context mattn starts a goroutine for each statement and each row,
  about 1 µs each.
- **Invariant:** a cancelled request never yields a 200; a request
  nobody cancelled never fails; afterwards no goroutine is left.
- **Signal:** failed on an error in a request nobody cancelled.
  Degraded when a reader stays busy 10 ms after its cancel.
- **Informs:** step 21, question 4; how a prefix `like` is rendered,
  and whether vet reports one no index can serve.

#### PostCrawl

A crawler walks the list of posts to its last page, and the page
shows "page 3 of 20,000".

- **Axes:** monotonic, in key order; read; steady; tiny; cached, with
  a cold arm; offset, keyset, count; single; keep-alive; steady.
- **Stresses:** CPU, bandwidth. Arms: offset with a count; offset;
  keyset through `After`.
- **Sketch:** page k by offset walks 50 k rows: at the last page a
  million `rowScan`, 150 ms, against 80 µs for the keyset at any
  depth, about two thousand times. The whole crawl is 25 minutes of a
  reader by offset and two seconds by keyset. The count walks the
  smallest index of the table on every page, tens of milliseconds.
- **Invariant:** by keyset, every post that existed throughout
  appears exactly once and in order, under concurrent inserts and
  deletes. By offset, each page is sorted and from one snapshot; the
  rows it repeats or skips across pages are counted and reported.
- **Signal:** failed on a keyset page that repeats or misses a row.
  Degraded when a keyset page costs more with depth.
- **Informs:** step 23's limit on the rows of a `List`; whether a
  query route offers an offset at all; a count kept in a counter.

#### PostExport

An editor exports the list of every post, without the bodies: one
response of a million rows.

- **Axes:** uniform, every row; read; steady; response, 250 MB;
  cached, with a cold arm; range, the whole table; long-read;
  keep-alive, with a slow-read arm and an arm capped at 1 Gbit;
  steady.
- **Stresses:** memory, bandwidth.
- **Sketch:** nothing streams today: an `rt.Query` yields a slice
  and `renderEncode` fills a buffer. The scan is a million `rowList`,
  1.2 s and about double under the request's context, with the
  snapshot held throughout. The rows are about 200 MB, the encoding
  250 MB in a buffer that doubles as it grows, so some 700 MB of heap
  at the peak for one export and twice that resident at the
  collector's default. At 1 Gbit the body drains in two seconds; a
  reader at 1 MB/s pins it for four minutes.
- **Invariant:** the body is valid, its length is right, and its rows
  are one snapshot: every post up to one commit and none after.
- **Signal:** failed on a torn or a short export. Degraded when the
  peak heap passes 1 KB a row.
- **Informs:** a read effect that streams rows to the writer, against
  one that materializes; step 23's limit on rows; step 21, question
  4.

#### RollupRecompute

A cached dashboard total expires at the top of the hour, and every
open dashboard recomputes it at once.

- **Axes:** one, the rollup row; read-write, an upsert; burst, spike,
  herd; tiny; cached; aggregate; single, the read on a reader and the
  write after it; keep-alive; steady.
- **Stresses:** CPU, latency.
- **Sketch:** Volt has no cache, and a pure handler cannot hold one,
  so the cache is a `rollups` row with the time it was computed. The
  recompute is `SampleAggregate`'s year, 0.45 s. Every request that
  arrives within that time finds the row stale and recomputes: two
  hundred of them hold four readers for 22 s, then write the same
  value two hundred times. Recomputations per expiry are the arrival
  rate times 0.45 s, where one would do.
- **Invariant:** the total served is the true total of some snapshot
  no older than the lifetime plus one recompute; the stored row is
  never torn.
- **Signal:** degraded when an expiry is recomputed more than once,
  which is expected today, and reported as a count.
- **Informs:** identical reads in flight performed once by the
  runner; a recompute loop at its own pace (S10, step 15).

**Races.**

#### PostCreateRace

Sixty-four clients create a post with the same slug at the same
instant.

- **Axes:** one, a value; write, insert; spike; tiny; cached; none;
  single, rollback; keep-alive; steady.
- **Stresses:** correctness, on the writer's queue.
- **Sketch:** the one writer runs them one after another: one
  `write`, then sixty-three refusals of about 12 µs.
- **Invariant:** for each slug exactly one 201, sixty-three 409s that
  name `slug`, and one row; the same when the creations arrive as
  overlapping `CreateMany` batches, each all or nothing.
- **Signal:** failed on two winners, on none, or on a 5xx.
- **Informs:** nothing while it holds; it guards the mapping of a
  refusal onto a 409 (`ProblemOf`).

#### PostUpdateConflict

Two editors open the same post at the same version and both save.

- **Axes:** one; read-write, update; spike, in pairs; tiny; cached;
  point; upgrade, a read and a write as two effects; keep-alive;
  steady.
- **Stresses:** correctness. Arms: the generated update route; a
  hand-written route whose `UpdateWhere` names the version.
- **Sketch:** the generated route performs `Get` on a reader and
  `Update` on the writer, about 50 µs apart. A racing pair both read
  version v and both succeed: the last write stands, and nothing says
  so. A validator over two columns passes for each on a row that is
  never stored (step 36). The hand-written route answers one 200 and
  one 409.
- **Invariant:** the stored row satisfies every validator; for the
  hand-written route the final version is the start plus the 200s.
- **Signal:** failed on a stored row a validator refuses, which is
  expected today and is step 36's defect; the silent overwrites are
  counted and reported.
- **Informs:** step 36's fix, the read and the write in one write
  transaction; whether the language grows a version setting.

#### CommentListRace

A post is deleted while readers list its comments.

- **Axes:** uniform; 99/1, delete; steady; tiny; cached; join, a
  parent with its children; single against several reads; keep-alive;
  steady.
- **Stresses:** correctness. Arms: a `with`; two route loads; a
  `Then` chain in the handler.
- **Sketch:** a `with` reads parent and children in one read
  transaction: no torn read. Two loads read two snapshots about 50 µs
  apart, so a delete that commits between them leaves a post with no
  comments beside a `comment_count` of fifty.
- **Invariant:** every answer is a 404 or the post with all of its
  comments, as many as its `comment_count`.
- **Signal:** failed on a torn `with`. For the other arms the torn
  reads are counted and reported.
- **Informs:** one snapshot for all the loads of a request.

**CPU.**

#### AuthorLogin

At nine o'clock a thousand authors log in within ten seconds.

- **Axes:** uniform; read, and a hash; burst, spike; tiny; cached;
  point; single; keep-alive; steady.
- **Stresses:** CPU, latency: the wait for a processor.
- **Sketch:** the stand-in hash is PBKDF2 with SHA-256 at 600,000
  rounds, about 150 ms of one core. Four cores verify 27 logins a
  second and 100 arrive: 3.7 times capacity for ten seconds. Go runs
  every hash at once in 10 ms slices, so all of them finish late and
  together, the last after some 37 s, and a 50 µs read that arrives
  meanwhile waits behind the runnable goroutines for a slice: up to
  seconds. The health probe misses its deadline.
- **Invariant:** the right password is a 200 and a wrong one a 401,
  never the reverse; the hash runs for every attempt, counted.
- **Signal:** failed on a wrong verdict. Degraded when the canary
  read's p99 passes ten times its unloaded value, or a health check
  is missed; both are expected today.
- **Informs:** step 14: the hash as an effect the runner performs on
  a bounded pool with a bounded queue, instead of CPU spent inside a
  pure handler, which nothing can bound; its parameters.

#### ReportDecodeNested

A client posts a 4 MiB report of nested JSON and gets a large nested
one back.

- **Axes:** uniform; write, insert; steady; body, response; cached;
  none; single; keep-alive; steady.
- **Stresses:** CPU, bandwidth. Arms: a valid body into a struct; a
  body nested 10,001 deep; 4 MiB of keys the model does not have.
- **Sketch:** decoding 4 MiB at `jsonDecode` is 50 ms of a core, so
  20 requests a second a core, with SQLite under 1%. The decoded
  value is two to five times the body: a hundred at once are 2 GB.
  The unknown keys are measured already (step 23): 471,363 field
  errors and a 422 of 51.7 MB, thirteen times the request.
- **Invariant:** what is stored is what was sent, by checksum; the
  deep body is a 400, an oversized one a 413, the unknown keys a 422,
  never a 5xx; a response is at most a fixed multiple of its request,
  and the bytes allocated for a request stay inside a budget.
- **Signal:** failed on a 5xx, or on a response past the multiple,
  which is expected until step 23 lands.
- **Informs:** step 23's limits on what decoding builds.

#### ReportCreateWide

A report row has 150 columns and a rule on each.

- **Axes:** uniform; write, insert and update; steady; wide; cached;
  point; single; keep-alive; steady.
- **Stresses:** CPU, bandwidth. On a corpus project. Arms: a valid
  create; a create with every column invalid; an update of one
  column.
- **Sketch:** a create decodes 150 fields, 20 µs, casts them, 15,
  validates, 5, inserts with 150 arguments, 40, reads 150 columns
  back, 25, and encodes them, 20, under `requestHTTP` 40: 165 µs, of
  which casting and validating are an eighth and the read-back with
  the two encodings over a third. The update of one column still
  reads 150
  back. The invalid create builds 150 details, 30 KB, and costs about
  what a success does without its insert.
- **Invariant:** the stored row is the one sent; the invalid create
  names all 150 columns and stores nothing.
- **Signal:** degraded when validation passes 30% of the request.
- **Informs:** whether an update returns the whole row; preallocation
  per request (S3).

#### RouteMatchMany

An application grows to seven thousand routes.

- **Axes:** uniform, over the routes; read; steady, herd at the
  start; tiny; cached; point; single; keep-alive; restart.
- **Stresses:** CPU, bandwidth. On a corpus project of a thousand
  tables of one column.
- **Sketch:** `ServeMux` matches through a tree, so a match stays
  under a microsecond from ten routes to ten thousand. Registration
  at start checks each pattern for conflicts and is the part that may
  grow faster than the routes; it is measured, not guessed. The
  binary grows by about a kilobyte or two a route.
- **Invariant:** every route answers with its own table's row; the
  first, the middle and the last registered cost the same within a
  fifth.
- **Signal:** degraded when a match costs more with more routes.
- **Informs:** step 28, where registration splits by scope; a limit
  on routes (S2).

**Memory.**

#### StreamIdle

Ten thousand browsers keep a connection open, half of them idle
keep-alive and half an event stream.

- **Axes:** uniform; read; steady, then a spike as all wake; tiny;
  cached; point; single; idle; steady.
- **Stresses:** memory, bandwidth.
- **Sketch:** `connectionIdle` is 15 KB: a goroutine, two 4 KB
  buffers and the connection. A stream adds a channel of 320 events,
  about 28 KB, and a ticker: 45 KB. Five thousand of each are 300 MB;
  fifty thousand of each, in the soak, 3 GB. Idle CPU is the
  heartbeat, 330 writes a second. When all wake within 100 ms, ten
  thousand lookups cross four readers in about 13 ms.
- **Invariant:** every connection still serves a request after the
  hold; after the close, descriptors and heap are back at baseline;
  at the descriptor limit the open connections keep working and new
  ones succeed again once some close.
- **Signal:** failed on a leak. Degraded past twice the sketch a
  connection.
- **Informs:** step 16's idle timeout and connection limit; step
  23's bound on subscribers and on each one's buffer.

#### UploadParallel

Two hundred clients upload 3 MiB images into a blob column at once,
and others download them.

- **Axes:** uniform, with an arm where all download one blob; 50/50,
  insert; burst; blob, body; cached; point; single; keep-alive,
  slow-send; checkpoint.
- **Stresses:** memory, bandwidth.
- **Sketch:** an upload is copied about five times: the body as
  read, 4 MiB of base64 in JSON; the decoded blob, 3; SQLite's copy,
  3; the row read back, 3; the encoded response, 4. That is 14 MiB of
  Go heap for an upload in flight, 2.8 GB for two hundred before the
  collector's headroom. On the writer one blob is 750 frames and
  3 ms: 330 uploads a second would be 1 GB/s of WAL and a checkpoint
  an upload, so the disk's bandwidth is the ceiling.
- **Invariant:** every download's SHA-256 is its upload's; an upload
  past the limit is a 413 after at most one byte more than the limit
  was read.
- **Signal:** failed on a blob that differs, which the slice kept by
  a setter (step 36) could cause. Degraded past 6 bytes of heap for a
  byte in flight.
- **Informs:** P5; whether a `Create` returns its blob columns; GOB
  for binary bodies; the position that file uploads are not planned.

#### ProcessSoak

The application runs an ordinary mixed day for hours, and nothing may
creep.

- **Axes:** zipf; 90/10, every verb; steady with bursts; every size
  in turn; cached; every shape in turn; every shape in turn; every
  behaviour in turn, stalled streams among them; checkpoint, jobs.
- **Stresses:** memory, bandwidth, over time.
- **Sketch:** flat by design, since the inventory of mutable state
  is four things. The suspects: a stream blocked in `Write` on a
  client that stopped reading, which has no deadline (`events.go`);
  the statement cache's closing goroutines under eviction; the WAL
  under readers that always overlap.
- **Invariant:** over the run the last quarter's mean is at most 1.1
  times the first quarter's for live heap, goroutines, descriptors,
  WAL bytes, file bytes at constant live rows, temporary files and
  subscribers; cached statements stay within 256 a pool and reader
  connections within `GOMAXPROCS`; at the end everything is back at
  baseline.
- **Signal:** failed on any slope.
- **Informs:** step 23's limits; V10.

#### HeapPressureTail

Sixteen lists of a hundred thousand rows are in flight while everyone
else does small reads.

- **Axes:** uniform; read; steady; response, 25 MB each; cached;
  range, point; long-read; keep-alive; steady.
- **Stresses:** memory, latency. Arms: the default; a memory limit
  of 1 GB through `debug.SetMemoryLimit`, a small container.
- **Sketch:** a Volt backend keeps no long-lived heap, since
  handlers are pure and controllers are empty structs, so the
  collector's question is the allocation rate and not the live heap.
  The lists allocate about 1 GB/s, the collector takes a quarter of
  the CPU, and small reads lose processors and pay assists: p99 about
  ten times, the median 1.3. Under the limit the collector may take
  half the CPU.
- **Invariant:** the standing ones.
- **Signal:** degraded when the canary read's p99 passes twenty
  times its unloaded value, or the collector 40% of the CPU.
- **Informs:** the collector's settings a `main` states (S12); a
  streaming read; step 23's limit on rows.

**Network.**

#### ClientSlow

Five thousand phones on a bad network send bodies a few bytes at a
time and read responses as slowly.

- **Axes:** uniform; 90/10; steady; body, response; cached; point;
  single; slow-send, slow-read; steady.
- **Stresses:** network, latency. Arms: a server with no timeouts;
  one with every timeout stated.
- **Sketch:** net/http has no slots to pin: a slow client holds a
  goroutine and its buffers, never a reader or the writer, because
  the shim reads the whole body before any effect and encodes the
  whole response before its first byte. Fast clients should move
  under 5%. The price is memory: a slow sender of a 4 MiB body pins
  what has arrived, up to 4 MiB, so five thousand of them are 20 GB
  at worst, and a slow reader pins the encoded response.
- **Invariant:** every fast request succeeds; a slow client gets the
  whole right response or a 408 or a 503, never a short 200.
- **Signal:** failed on a short 200. Degraded when the fast p99
  moves a tenth, or a slow client pins more than its body and 64 KB.
- **Informs:** step 16: the read, header and write timeouts stated
  (S12); the body limit as the admission to memory.

#### ConnectionChurn

A load balancer without keep-alive opens a connection for every
request.

- **Axes:** uniform; read; steady; tiny; cached; point; single;
  churn; steady.
- **Stresses:** network, latency. Arms: keep-alive; a connection a
  request; the same over TLS.
- **Sketch:** a connection costs an accept, a goroutine, its buffers
  and a close, about 40 µs, which doubles the 48 µs request: half the
  throughput. A TLS handshake is about half a millisecond of server
  CPU: a tenth of the throughput.
- **Invariant:** no connect fails under the listener's backlog;
  descriptors and goroutines return to baseline.
- **Signal:** degraded when a plain connection costs more than three
  requests.
- **Informs:** step 16's keep-alive and TLS settings.

#### ClientVanish

Clients drop the connection at a random point of a request: in the
body, queued for the writer, inside a transaction, in the response.

- **Axes:** uniform; 50/50, the update with its audit row; steady;
  tiny, body; cached; point; several; vanish; steady.
- **Stresses:** network, latency.
- **Sketch:** gone while queued: the wait for the writer ends with
  the context and SQLite does nothing. Gone inside a transaction: it
  rolls back between two statements. Gone during a single write that
  waits under SQLite's busy handler: nothing interrupts it (spec, the
  runner). Gone after the commit: the work is done and the client
  never learns. Less than one statement should run for a request
  after its client left.
- **Invariant:** no transaction is half applied: audit rows equal
  updates; the server logs a 499, not a 500; other clients see no
  error; the readers end idle.
- **Signal:** failed on a half-applied transaction. Degraded when
  more than one statement runs after a client left.
- **Informs:** step 21, question 4; what serving HTTP/2, where a
  cancel does not close the connection, would change.

**Disk.**

#### WALReaderPinned

An export or a year-long aggregate holds its snapshot while ingestion
never stops, and the WAL cannot be checkpointed past it.

- **Axes:** monotonic; 1/99, inserts beside one long read; steady;
  rows; cached; aggregate; long-read; keep-alive; checkpoint.
- **Stresses:** disk, latency. Arms: one reader pinned for 60 s;
  readers that always overlap, four reads of 100 ms back to back and
  staggered, in the soak.
- **Sketch:** the WAL grows at the write rate while the snapshot is
  held: single appends write 270 MB/s, 16 GB in the minute; bulk
  appends 30 MB/s, 1.8 GB. At the release one commit pays the
  checkpoint with the writer held: the distinct pages, some 50,000
  or 200 MB written to their places, and two `fsync`, 0.2 to 2 s on
  an SSD. The file then keeps its size, since mattn's build sets no
  `journal_size_limit`. Under overlapping readers the checkpoint
  keeps copying, but the WAL starts over only when no reader is in
  it, so the file may grow for as long as the load lasts.
- **Invariant:** the WAL is at most the hold times the write rate
  plus three checkpoints, and is back under two checkpoints within
  ten seconds of the release; no write fails.
- **Signal:** failed when the WAL does not come back, which is
  expected today, or grows with no long reader. Degraded when the
  stall passes a second.
- **Informs:** a checkpoint at the runner's own pace that restarts
  or truncates the WAL past a bound; `journal_size_limit` stated; a
  greatest age for a reader's snapshot; a default deadline for reads.

#### SampleReadCold

The samples table is far larger than memory, and dashboards ask for
random old points.

- **Axes:** uniform; read; steady, herd on a cold start; tiny; big,
  cold; point; single; keep-alive; restart.
- **Stresses:** disk, latency. Arms: cached; cold once; evicted
  continuously; really larger than memory, in the soak.
- **Sketch:** a hundred million samples are 5 GB and their index
  2.5. A lookup by series and time walks the index and then the
  table; the interior pages stay cached and the two leaves miss: two
  `deviceRead`, 200 µs against 5, forty times, and four readers serve
  20,000 a second. On a disk of 8 ms a read it is 60 a second a
  reader. The readers' own caches are 500 pages and are dropped at
  every commit, so the OS cache is the only one that counts.
- **Invariant:** each lookup answers the seeded value.
- **Signal:** degraded when a lookup reads the device more than
  three times, which means interior pages are missing too.
- **Informs:** the readers' `cache_size` and `mmap_size`, which
  `runner.md` 2 measured on cached data only; a table with a
  composite key as `WITHOUT ROWID`, one leaf instead of two.

#### BackupLive

The nightly backup runs while traffic continues.

- **Axes:** uniform; 90/10; steady, with an import running in one
  arm; tiny; cached, 5 GB; range, the whole file; long-read;
  keep-alive; backup.
- **Stresses:** disk, bandwidth. Arms: `VACUUM INTO` on a connection
  of its own; mattn's online backup in steps from a second
  connection.
- **Sketch:** `VACUUM INTO` copies at about 300 MB/s, 5 GB in 17 s,
  on one snapshot. The stepped backup starts over whenever another
  connection commits, so at any steady write rate it may never
  finish.
- **Invariant:** the copy opens, passes `integrity_check` and
  `foreign_key_check`, and is the source at one commit: every write
  acknowledged up to its newest row is in it, and its counters equal
  its counts.
- **Signal:** failed on a copy that is not one commit. Degraded when
  the stepped backup has not finished in ten times the copy's time.
- **Informs:** a `volt backup` command, which the roadmap lacks, and
  the mechanism under it.

#### MigrationLive

`volt migration up` adds an index to a table of fifty million rows
while the site serves.

- **Axes:** uniform; 90/10; steady; rows, the whole table in one
  transaction; cached; point; long-write; keep-alive; migration.
- **Stresses:** disk, bandwidth. The migration runs in a second
  process, the `volt` the suite built.
- **Sketch:** the index takes about a microsecond a row, 50 s with
  the write lock held and a gigabyte or more of WAL. Reads go on. A
  single write waits five seconds under SQLite's busy handler and
  fails; a transaction waits five seconds in the runner's loop and
  fails. So for 50 s every write hangs five seconds and answers 500,
  at a thousand writes a second five thousand requests hang at any
  moment, and a route's deadline does not reach the single ones.
  Afterwards every cached statement compiles again.
- **Invariant:** no acknowledged write is lost; no read fails;
  `volt migration status` is clean after; and during it a write is
  served or refused with a 503, never a 500.
- **Signal:** failed on a 500, which is expected today. Degraded
  when the canary read's p99 doubles.
- **Informs:** a busy database answered 503 with `Retry-After`;
  whether the tool refuses a long migration on a database in use;
  the busy timeout as a constant (step 23).

#### AccountWriteImbalance

One account writes through sixty-four connections, sixty-three others
through one each.

- **Axes:** zipf, one heavy account; write, insert; over2; tiny;
  cached; none; single; keep-alive; steady.
- **Stresses:** CPU, latency.
- **Sketch:** the writer's connection goes to a random waiter
  (`runner.md` 7), so the heavy account, with 64 of 127 places, gets
  half the writer. A light account's write waits about 127 services,
  3.8 ms against 30 µs alone, and p99 under a random hand-off is some
  4.6 times the mean, 17 ms. A queue fair between accounts would give
  64 services, 1.9 ms.
- **Invariant:** every account's acknowledged rows are stored.
- **Signal:** degraded when a light account's p99 passes six times
  its mean wait.
- **Informs:** the order and the fairness of the writer's queue
  (`runner.md` 7); step 16's rate limit.

**Added by the attack.** `DeliveryKeyOrder` stands with the write
stories above.

#### PostMixSweep

The same site at different hours: all reads at night, half writes
during an import.

- **Axes:** uniform; read, 99/1, 90/10, 50/50, 1/99, write; steady;
  tiny; cached; point, with a range arm on the feed page; single;
  keep-alive; steady.
- **Stresses:** memory, latency: a reader refilling its cache.
- **Sketch:** a reader drops its page cache whenever the writer has
  committed, and lookups measured 1.4 times slower at a commit a
  millisecond and twice at commits back to back (`runner.md` 2). A
  lookup is a tenth of a request, so point reads lose under a tenth
  end to end; the feed page, mostly database time, up to half its
  speed. The writer caps the whole at a total rate times write share
  of 33,000 a second.
- **Invariant:** the standing ones, the read after an acknowledged
  write above all.
- **Signal:** degraded when a read at 50/50 costs over 2.5 times a
  read alone.
- **Informs:** the readers' cache settings, and `mmap_size` for
  reads that touch many pages.

#### PostReadCores

The same reads on a larger machine.

- **Axes:** uniform; read; steady; tiny; cached; point; single;
  keep-alive; steady.
- **Stresses:** CPU, bandwidth. Arms: `GOMAXPROCS` of 1, 2, 4, 8, 16
  and on to the machine's cores.
- **Sketch:** linear to four, measured (`runner.md` 1). Past that,
  every read takes the statement cache's mutex twice, the pool's
  once, the statement's list of connections once, and SQLite's
  memory statistics mutex (step 21, question 1). The curve is
  expected to bend between eight and sixteen.
- **Invariant:** the standing ones.
- **Signal:** degraded when doubling the cores gives under 1.5 times
  the reads before sixteen.
- **Informs:** step 21, question 1: a cap on the readers; a
  statement cache per reader.

#### PostReadTrending

The popular posts change every few minutes, and the new favourites
are cold.

- **Axes:** zipf, moving; read; steady; tiny; big, or evicted;
  point; single; keep-alive; steady.
- **Stresses:** disk, latency. Arms: a hot set that never moves; one
  that moves every minute.
- **Sketch:** a fixed hot set stays cached at any size and equals
  `PostReadHot`. At a shift, ten thousand newly hot posts are two
  leaves each at a `deviceRead` across four readers: half a second
  during which p99 is the device's latency, then back.
- **Invariant:** the standing ones.
- **Signal:** degraded when the recovery after a shift passes twice
  the sketch.
- **Informs:** as `SampleReadCold`; identical reads performed once
  for the first readers of a newly hot row.

#### SampleAppendOverload

Ingestion arrives at twice, then ten times, what the writer can take,
for minutes.

- **Axes:** monotonic; write, insert; ramp, over2, over10; tiny,
  with an arm of 10 KB bodies; cached; none; single; keep-alive;
  checkpoint.
- **Stresses:** CPU, latency.
- **Sketch:** past 33,000 a second the queue for the writer grows by
  the excess, 33,000 requests a second at twice capacity. Nothing
  bounds it, and 99 of 101 routes run without a deadline (step 23),
  so latency grows without limit and each queued request holds its
  goroutine and its body: a hundred thousand queued with 10 KB bodies
  are over a gigabyte. With a client's deadline the random hand-off
  keeps goodput near capacity while most requests fail; in arrival
  order every request would wait past the deadline. With step 37's
  variant, `timeout: 1` on the route, a write that waits a second is
  a 504 and the achieved rate holds at capacity. The readers stay
  idle, so the canary read does not move.
- **Invariant:** every acknowledged sample is stored, once.
- **Signal:** degraded when goodput at ten times capacity falls
  under half of capacity, when the heap grows through the run, or
  when the canary read moves.
- **Informs:** a bound on the writer's queue with a 503 past it,
  which `runner.md` 7 names for batching and which is needed without
  it; a default deadline; the queue's order.

#### CommitSync

The operator sets `synchronous = FULL` for durability, or moves the
file to a slower disk.

- **Axes:** one and uniform, as arms; write, update; steady; tiny;
  cached; none; single; keep-alive; checkpoint.
- **Stresses:** disk, latency. Arms: `NORMAL` and `FULL`; a
  checkpoint at 1,000 and at 10,000 frames.
- **Sketch:** under `NORMAL` a commit does not wait for the disk and
  one commit in a thousand frames pays the checkpoint, two `fsync`
  and the copy: p99.9 of 3 to 15 ms against a median of 30 µs. Under
  `FULL` every commit waits: 30 µs and an `fsync`, so with a 1 ms
  `fsync` 33,000 writes a second become 970, thirty-four times fewer.
- **Invariant:** every acknowledged write is present after the
  database is closed and opened again, and after the process is
  killed, an arm that waits for question 2.
- **Signal:** degraded when a commit under `FULL` passes two
  `fsync`.
- **Informs:** `runner.md` 7, where batching pays under `FULL`;
  `synchronous` stated among the pragmas (step 23); the checkpoint's
  threshold.

#### PostWriteIndexed

An index is added for each admin screen until `posts` has eight.

- **Axes:** uniform, random in the indexes; write, insert and
  update; steady; tiny; indexed, with a big arm; none; single;
  keep-alive; steady.
- **Stresses:** disk, bandwidth. Arms: no index, two, four, eight.
- **Sketch:** each index adds a tree insert, about 2 µs cached, and
  a dirty page: an insert goes from 30 µs to 45 at eight indexes,
  and from one frame to nine, 36 KB of WAL a row. Past the cache each
  index leaf is a `deviceRead`: eight of them, 0.8 ms, thirty times.
- **Invariant:** every index agrees with its table: a query through
  each finds every row.
- **Signal:** degraded when an index costs over twice its sketch.
- **Informs:** vet's `redundantindex`; the writer's `cache_size`.

#### SampleRetention

A nightly job deletes the samples older than thirty days.

- **Axes:** monotonic, at the left edge; write, delete; burst; rows;
  cached, then fragmented; range; long-write; keep-alive; jobs.
- **Stresses:** disk, bandwidth. Arms: one statement; batches of a
  thousand at an interval.
- **Sketch:** a range delete frees whole leaves in order, about a
  microsecond a row and a frame a freed page, far below an insert.
  One statement over ten million rows holds the writer about ten
  seconds, a batch one millisecond; the canary's curve is
  `SampleImport`'s and is not drawn again. The file does not shrink:
  the freed pages wait on the free list for the next appends.
- **Invariant:** exactly the rows older than the cut are gone; the
  file is never larger than its peak.
- **Signal:** degraded when the file grows from one cycle to the
  next.
- **Informs:** step 15's bound on a batch; `auto_vacuum` stated.

#### DatabaseFull

The disk fills.

- **Axes:** monotonic; write, insert; steady; tiny; cached; none;
  single, several; keep-alive; steady.
- **Stresses:** disk capacity, outside the matrix.
- **Sketch:** `PRAGMA max_page_count` stands for the full disk (V4).
  Past it every insert fails and SQLite rolls its transaction back;
  reads go on.
- **Invariant:** while full, reads succeed and every write is
  refused with one status the spec names, with no transaction half
  applied; once deletes free pages, writes succeed again with no
  restart; `integrity_check` passes.
- **Signal:** failed on a half-applied write, or on a process that
  needs a restart.
- **Informs:** how a full database maps onto a response, which the
  spec does not say today.

#### SampleRollup

A nightly job sums each series' year of samples into its rollup row.

- **Axes:** monotonic; write, upsert, after an aggregate; burst, the
  top of the hour; tiny; cached; aggregate; long-read inside a
  write, against a read and then a write; keep-alive; jobs.
- **Stresses:** CPU, latency. Arms: an `rt.Atomic` over the read and
  the upsert; the read alone and then the upsert.
- **Sketch:** inside a writing transaction the reads run on the
  writer (`runner.md` 2), so the `Atomic` holds the write lock for
  the whole aggregate, 0.45 s a series, and every write waits: the
  canary write's p99 is the aggregate's time. The second arm holds
  the writer for the upsert alone, 30 µs, and its rollup may be one
  commit behind.
- **Invariant:** each rollup equals the sum of its series at some
  commit; a second run of the job changes nothing.
- **Signal:** degraded when the canary write's p99 passes 10 ms in
  the second arm.
- **Informs:** step 15's shape for a job; a vet rule for a long read
  inside an `Atomic`; an upsert in the generated surface.

#### ClientRetryStorm

A write stall of five seconds makes every client time out and send
again, three times; the stall ends, and the site must come back.

- **Axes:** uniform; 90/10; retry; tiny; cached; point; several, the
  update with its audit row; vanish, a client that timed out closes;
  migration, a second writer holding the write lock for the five
  seconds.
- **Stresses:** network, latency.
- **Sketch:** the offered load triples during the stall. A client
  that timed out closes, its queued write leaves the queue when its
  context ends, and so the backlog at the release is the live
  requests alone: recovery within about a second. A write whose first
  attempt committed after its client gave up is applied again by the
  retry, unless a key makes it idempotent.
- **Invariant:** with an idempotency key each logical write is
  applied once; audit rows equal updates.
- **Signal:** failed on a write applied twice. Degraded when p99 is
  not back within five seconds of the stall's end.
- **Informs:** idempotency keys for writes, as a convention or in
  the runtime; `Retry-After`.

#### CommentScanFragmented

After months of deletions nine comments in ten are gone and the pages
are mostly empty.

- **Axes:** uniform; read; steady; tiny; fragmented, with a cold
  arm; range; single; keep-alive; steady.
- **Stresses:** disk, latency. Arms: a fresh table; the same after
  90% of its rows were deleted at random; the same after a `VACUUM`.
- **Sketch:** a thousand live rows that filled 25 pages now lie on
  250: cached, 0.4 ms more of page fetches; cold, 250 `deviceRead`,
  25 ms against 2.5, ten times. The file keeps its full size, since
  `auto_vacuum` is off.
- **Invariant:** the listing is complete and in order.
- **Signal:** degraded when a live row costs over twelve times the
  pages of the fresh table.
- **Informs:** `auto_vacuum = INCREMENTAL` stated at creation; a
  maintenance command.

#### CommentDeleteCascade

A moderator deletes a post with a hundred thousand comments, and the
foreign key deletes them.

- **Axes:** one, a parent whose children are scattered; write,
  delete; steady; rows; cached; none; long-write; keep-alive;
  checkpoint.
- **Stresses:** disk, bandwidth.
- **Sketch:** the post's comments lie scattered among ten million,
  rarely two to a leaf, so a hundred thousand deletes dirty some
  eighty thousand pages: 330 MB of WAL to remove 10 MB of rows, over
  thirty times. It is one statement, which nothing can batch: half a
  second to a second of the writer. The checkpoint after it writes
  those eighty thousand pages to random places.
- **Invariant:** no comment of the post remains and none of another
  post is gone; the post and its comments go in one transaction.
- **Signal:** degraded when the canary write's p99 passes two
  seconds.
- **Informs:** guidance on `delete: cascade` over large child
  tables; deletion as a job at its own pace (step 15); the writer's
  `cache_size`.

#### PostSortUnindexed

"Most liked posts" sorts a million rows on a column no index covers.

- **Axes:** uniform; read; steady; tiny, with an arm that carries
  the 1 KB body; cached; sort; long-read; keep-alive; steady.
- **Stresses:** disk, bandwidth. Arms: with a limit of twenty; with
  no limit; with no limit and the body selected.
- **Sketch:** with a limit SQLite keeps twenty rows through one
  scan: 0.2 s of CPU and no file. With none the sorter spills past
  its memory: 100 MB of temporary files written and read back. With
  the body selected the sorter carries every whole row: a gigabyte of
  temporary files for one request.
- **Invariant:** the order is right; no temporary file remains.
- **Signal:** failed on a temporary file left behind. Degraded when
  the temporary bytes pass 1.5 times the rows' width.
- **Informs:** `temp_store` and the temporary directory stated
  (S12); a vet rule for an order no index serves; step 23's limit on
  rows.

#### PostGetMany

A page loads the five thousand posts a reader bookmarked, by their
keys.

- **Axes:** uniform; read; steady; keys, from one to 32,767; cached;
  point, many at once; single; keep-alive; steady.
- **Stresses:** CPU, bandwidth. Arms: `GetMany`, a placeholder a
  key; a select with `in :ids`, one JSON array.
- **Sketch:** the text of `GetMany` changes with the count, so every
  new count misses the statement cache and compiles a text that
  grows with it, about 0.3 ms at five thousand keys, and binds five
  thousand arguments. The select has one text and parses 30 KB of
  JSON. The select should be two to three times cheaper when the
  counts vary.
- **Invariant:** both spellings answer the same rows (V9); 32,767
  keys are refused as the client's error and not a 500.
- **Signal:** failed when the spellings disagree, or on the 500,
  which is expected today (step 23).
- **Informs:** rendering `GetMany` as `InList` does; step 23's limit
  on keys.

#### PostFilterShapes

A search form with ten optional filters goes live, and every
combination of them is another statement text.

- **Axes:** uniform; read; steady; tiny; cached; range; single;
  keep-alive; steady.
- **Stresses:** CPU, bandwidth. Arms: 16, 256 and 1,024 distinct
  shapes, drawn evenly and by a Zipf law.
- **Sketch:** a pool's cache holds 256 texts. At 1,024 shapes drawn
  evenly three reads in four miss: a `prepare` or more, and an
  eviction whose close runs on a goroutine of its own. The database
  time of a point query about doubles.
- **Invariant:** each answer equals the filter applied in Go to the
  seeded rows, and a cached statement agrees with a fresh one (V9);
  no pool's cache passes 256; the closing goroutines drain.
- **Signal:** failed on a disagreement or a cache past its bound.
- **Informs:** the cache's size as a named limit (step 23); a cache
  for each reader's connection.

#### PostReadOverload

Crawlers and a traffic peak ask for ten times the reads the cores can
serve.

- **Axes:** uniform; read; ramp, over2, over10; tiny; cached; point;
  single; keep-alive; steady.
- **Stresses:** CPU, latency.
- **Sketch:** unlike the writer's overload this one takes every
  core, so everything slows together, writes and health checks
  included, since each needs CPU to be parsed and answered.
  Goroutines pile up in net/http, which has no limit, and the wait is
  for a processor. Goodput holds near capacity while clients wait,
  and falls once their deadlines pass before the work is done.
- **Invariant:** the standing ones.
- **Signal:** degraded when goodput at ten times capacity falls
  under half of capacity, or a health check is missed.
- **Informs:** step 16: a limit on requests in flight with a 503
  past it; the rate limit.

#### PostReadFormats

The desktop client asks for GOB and the browser for JSON, over a
1 Gbit link.

- **Axes:** uniform; read; steady; tiny and response, one row and a
  thousand; cached; point, range; single; keep-alive; steady.
- **Stresses:** network, bandwidth. Arms: JSON and GOB, each on
  loopback and through the relay capped at 1 Gbit.
- **Sketch:** for one row JSON is about 2 µs and 300 bytes. GOB,
  with a new encoder for each response, sends the type's description
  every time and makes the client compile a decoder every time:
  perhaps five times the CPU and twice the bytes. For a thousand rows
  GOB is perhaps half the CPU and 0.6 of the bytes, with the
  crossover at some tens of rows. At 1 Gbit a list of 250 KB passes
  470 times a second, far under what the CPU could encode: the
  uncompressed bytes are the limit.
- **Invariant:** both formats decode to equal values for every
  column type; a column tagged `json:"-"` is in neither.
- **Signal:** failed when the formats differ, which is expected for
  the tagged column (step 36).
- **Informs:** the generated client's default format; whether the
  shim compresses.

#### ServerRestart

A deploy stops the server under load and starts it again.

- **Axes:** zipf; 90/10; herd, retry; tiny, with one export in
  flight; cached, with a cold arm; point; several; keep-alive, idle
  streams and slow clients among them; restart.
- **Stresses:** network, latency. Arms: a graceful stop; a kill,
  which waits for question 2.
- **Sketch:** no listener is handed over, so connects are refused
  for the drain and the start, 50 to 200 ms: at 5,000 requests a
  second up to a thousand, all retried. `http.Server.Shutdown` waits
  for connections to fall idle and an event stream never does, so
  the stop lasts to its deadline unless the broker ends its streams
  first. At the start every client reconnects at once, and connects
  past the listener's backlog wait a second for their retransmission.
  Closing the database checkpoints the WAL and takes as long as the
  WAL is large; after a kill the next open recovers it instead, about
  a second a gigabyte.
- **Invariant:** every write acknowledged before the stop is present
  after it, in both arms; `integrity_check` passes; a response cut
  short is never counted a success.
- **Signal:** failed on a lost acknowledged write. Degraded when the
  stop passes five seconds, or the refused connects pass twice the
  rate times the start's time.
- **Informs:** step 16: the order of a shutdown (stop accepting, end
  the streams, drain, close the database), readiness, and a listener
  taken from the service manager.

#### ServerOverlap

A blue-green deploy runs the old process and the new one on the same
file for a minute.

- **Axes:** uniform; 50/50; steady; tiny; cached; point; single,
  upgrade; keep-alive; restart.
- **Stresses:** disk, latency: the file's lock between two
  processes.
- **Sketch:** in the suite, two servers in the one process, each
  with a `run.DB` of its own on the file. Each has a writer of its
  own, so SQLite's write lock is contended for the first time. A
  transaction that finds it held sleeps by the runner's schedule,
  whose first step is a millisecond (`busyDelays`): a contended write
  goes from 30 µs to a millisecond or more, thirty times. A single
  statement waits in SQLite's own handler, which no deadline reaches.
- **Invariant:** no write is lost or doubled between the two;
  counters equal their counts; no request outlives its route's
  timeout by more than 100 ms.
- **Signal:** failed on the timeout for single writes, which the
  spec admits today. Degraded when the pair writes under a quarter
  of what one process does.
- **Informs:** the busy schedule (step 23); whether two processes on
  one file are supported, or refused at `run.Open`.

#### RequestHeaderLarge

A client sends a megabyte of cookie, or a hundred thousand header
lines.

- **Axes:** uniform; read; steady; tiny, the headers being the large
  part; cached; point; single; headers; steady.
- **Stresses:** memory, bandwidth.
- **Sketch:** net/http refuses a header past its limit, 1 MiB by
  default, with a 431 before the shim runs, and allocates no more
  than the limit to find out.
- **Invariant:** a 431 and never a 5xx; an `X-Request-ID` echoed no
  longer than a bound; at most the limit and 64 KB allocated for a
  refused request.
- **Signal:** failed on an echo of any length, which is expected
  today (step 36).
- **Informs:** `MaxHeaderBytes` stated (S12, step 16).

#### SchemaOpenLarge

An application of a thousand tables restarts, and every connection
reads the whole schema before its first row.

- **Axes:** uniform; read; herd; wide; cached; point; single;
  keep-alive; restart, migration.
- **Stresses:** memory, bandwidth. On a corpus project of a thousand
  tables of 150 columns.
- **Sketch:** the schema is about 4.5 MB of SQL, and each of the
  `GOMAXPROCS` readers and the writer parses it at its first
  statement: perhaps 0.1 to 0.2 s and 20 MB a connection, so the
  first read on each reader waits that long, and sixteen cores hold
  340 MB of parsed schema. A migration makes every connection parse
  it again.
- **Invariant:** the first response on each connection is right.
- **Signal:** degraded when the first row takes over half a second.
- **Informs:** warming the readers at `run.Open`; a limit on the
  schema's size (S2).

#### EventFanout

A popular post gets a comment a second, and each is pushed to ten
thousand open event streams.

- **Axes:** one, a topic; write, a publish; ramp; tiny; cached;
  none; single; idle, with one stream in a hundred stalled; steady,
  restart as all reconnect.
- **Stresses:** network, bandwidth.
- **Sketch:** `Publish` walks every subscriber under one mutex, ten
  thousand at about 60 ns: 0.6 ms an event. Delivery is the cost: ten
  thousand writes and flushes an event, about 30 ms of CPU, so one
  core delivers some 300,000 events a second, thirty publishes a
  second to ten thousand streams. A stalled client's channel fills
  after 320 events and it is dropped, but its goroutine stays blocked
  in `Write`, which has no deadline. When all reconnect, each is
  replayed up to 256 events at once.
- **Invariant:** every healthy subscriber receives every event once
  and in order; a dropped one sees its stream end; a gap appears only
  across a reconnect and shows in the IDs; afterwards no goroutine is
  left.
- **Signal:** failed on an event missing or doubled on a healthy
  stream, or on a goroutine a stalled one left, which is expected
  today (step 23).
- **Informs:** step 23's bound on subscribers, its write deadline
  and the broker's own pace (S10); the shape of `volt.Publish`.

#### LogSinkStall

The pipe that carries the log stops being read.

- **Axes:** uniform; 90/10; steady; tiny; cached; point; single;
  keep-alive; steady.
- **Stresses:** disk, latency: the sink's.
- **Sketch:** `volt.Logger` writes a line a request through one
  handler under a mutex, so when the sink blocks every request blocks
  behind it: the site stalls for as long as the sink does.
- **Invariant:** when the sink resumes, no request was lost and none
  was answered twice.
- **Signal:** the stall is the finding, reported as request stall
  per second of sink stall; expected to be one.
- **Informs:** step 16's logging: a bounded buffer that drops and
  counts.

### Tiers

**Smoke: seconds, on every commit.** Inside `go test ./...`, in one
process, on a few thousand rows; each case under a second and the
tier under fifteen on four cores. It asserts only what holds under
every interleaving, the invariants and counted things: statements a
request, rows, frames within a loose bound. It reads no clock.

| Case | The smoke run |
|---|---|
| `SeatClaimHot` | 2,000 claimants for 500 seats |
| `PostViewHot` | 2,000 views, every count once |
| `SampleAppendBulk` | 50 batches, one in ten invalid |
| `AuditWrite` | 1,000 updates from 16 clients, every pair whole |
| `DeliveryReceive` | 2,000 deliveries, three in ten repeated |
| `JobClaim` | 1,000 jobs, 16 workers |
| `SampleImport` | 20 chunks, each sent twice |
| `PostFeed` | the three arms agree on 50 pages |
| `PostSearch` | 200 queries, half of them cancelled |
| `PostCrawl` | a keyset crawl of 5,000 rows under inserts and deletes |
| `PostExport` | 20,000 rows under appends: one snapshot |
| `PostCreateRace` | 64 clients on each of 50 slugs |
| `PostUpdateConflict` | 200 racing pairs |
| `CommentListRace` | 200 deletes under listing, the `with` arm |
| `ReportDecodeNested` | the three bodies: statuses and the allocation budget |
| `UploadParallel` | 32 uploads of 64 KB, by hash |
| `ClientVanish` | 500 requests cut at seeded points |
| `PostReadFormats` | every column type through both formats |
| `PostFilterShapes` | 1,024 shapes: the bound and the agreement |
| `PostGetMany` | counts from 1 to 5,000, and the limit |
| `SampleRollup` | both arms, run twice |
| `DatabaseFull` | a limit of a few hundred pages |
| `EventFanout` | 100 subscribers, 1,000 events, one stalled |
| `RequestHeaderLarge` | the sizes around the limit |

**Full: minutes.** Sixty cases, all but `ProcessSoak` and
`RequestHeaderLarge`, each arm run once for the bench's window with
the standing probes: about forty minutes for the tier on the
reference machine, a case alone under a minute once its rows are
seeded. One run is enough for the invariants and the surprises. The
bench's five runs and its noise floor are for comparing two commits,
on the cases named for it. Run by hand and before a release.

**Soak: an hour or more a case.**

| Case | The soak run |
|---|---|
| `ProcessSoak` | the mixed day for hours: every slope |
| `WALReaderPinned` | readers that always overlap for an hour: the WAL's size |
| `SampleAppend` | appends until the file is several times memory: frames a row and file bytes a row stay constant |
| `DeliveryKeyOrder` | to a hundred million keys: the cliff where the index leaves memory |
| `JobClaim` | ten million jobs through the queue: the file stops growing, the poll does not slow |
| `SampleRetention` | a day of appends and nightly deletes, many times over: the file's size |
| `SampleReadCold` | a file really larger than memory |
| `StreamIdle` | a hundred thousand connections held for an hour |
| `EventFanout` | one stream in a hundred stalled for an hour |

**A case that fails today.** Several invariants are expected to fail
on the code as it stands; each is named in its case. Such a case is a
defect found (V1): its assertion lands in the commit that fixes the
defect, and until then the full tier reports its count. No assertion
is weakened to make a tier pass.

### The ten expected to hurt most

Ranked by how sure the reading of the code makes the prediction, and
by what it costs in production.

1. **`WALReaderPinned`.** Under load the readers are always busy, so
   there may never be a moment without a reader in the WAL, and the
   WAL starts over only then. Nothing sets `journal_size_limit`, and
   the only checkpoint is SQLite's automatic one, on a request's
   commit. A disk that fills over hours, and a stall of seconds when
   the checkpoint finally runs.
2. **`SampleAppendOverload`.** The writer's queue has no bound, 99
   of 101 routes have no deadline, and the hand-off is random. Past
   capacity, memory grows with the queue and the tail is several
   times the mean. No answer says "busy".
3. **`PostExport`.** A million rows are about 700 MB of heap for one
   request, with a snapshot held for seconds, and a `List` route has
   no limit at all (step 23): one request can end the process.
4. **`ClientSlow`.** The shim reads a whole body into memory before
   anything else, and a `main` that states no timeouts lets a slow
   sender hold up to 4 MiB for as long as it likes. Five thousand
   connections are 20 GB.
5. **`MigrationLive`.** A migration from the tool on a database in
   use makes every write hang five seconds and answer 500 for as
   long as it runs, and a route's deadline does not reach a single
   write that waits under SQLite's handler.
6. **`AuthorLogin`.** CPU spent inside a pure handler is bounded by
   nothing. Three or four times the cores' capacity for ten seconds
   makes every request wait seconds for a processor, and the health
   check fails with them, so a load balancer removes a server that is
   working.
7. **`SampleAggregate`.** A few long reads take every reader, and
   short reads wait behind them with no priority and no deadline: the
   front page stalls because someone opened a dashboard.
8. **`EventFanout`.** A stalled subscriber leaves a goroutine
   blocked in a `Write` without a deadline, `Publish` walks every
   subscriber under one mutex, and `Shutdown` waits for streams that
   never end.
9. **`ServerOverlap`.** Two processes on the file, as during a
   blue-green deploy, meet the runner's busy schedule, whose first
   wait is a millisecond: writes thirty times slower for the length
   of the overlap, and single writes that outlive their deadline.
10. **`CommentDeleteCascade`.** One request, one statement that
    cannot be batched: a stall of up to a second for every writer,
    over thirty times its size in WAL, and a checkpoint of tens of
    thousands of random pages after it.

Close behind: `UploadParallel`, five copies of every blob;
`SessionTouch`, a whole site capped at the writer's rate;
`RollupRecompute`; and `ReportDecodeNested`, whose thirteenfold 422
step 23 has measured.

### What the cases keep pointing at

The same few decisions stand behind most of the catalogue:

- **The writer's queue:** a bound with a 503 past it, an order, and
  fairness (`SeatClaimHot`, `AccountWriteImbalance`,
  `SampleAppendOverload`, `SampleImport`).
- **The transaction's fixed cost,** two and a half statements' worth
  (`AuditWrite`, `SeatClaimHot`, `JobClaim`).
- **The checkpoint at the runner's own pace,** and a bound on the
  WAL (`SampleAppend`, `WALReaderPinned`, `CommitSync`,
  `BackupLive`).
- **Long reads beside short ones:** a lane, more readers than cores,
  a default deadline (`SampleAggregate`, `PostCrawl`, `PostSearch`,
  `RollupRecompute`).
- **Identical reads in flight performed once,** which effects as
  values make possible (`PostReadHot`, `RollupRecompute`,
  `PostReadTrending`).
- **One snapshot for a request's loads, and an update that reads and
  writes in one transaction** (`CommentListRace`, `PostFeed`,
  `PostUpdateConflict`).
- **Rows and bytes:** limits, and a read that streams (`PostExport`,
  `HeapPressureTail`, `UploadParallel`, `ReportDecodeNested`).
- **CPU work as an effect on a bounded pool** (`AuthorLogin`).
- **A busy database as a 503,** and the busy schedule
  (`MigrationLive`, `ServerOverlap`, `ClientRetryStorm`).
- **Every server and SQLite option stated** (S12): timeouts, header
  and body limits, `synchronous`, `cache_size`, `journal_size_limit`,
  `auto_vacuum`, `temp_store` (`ClientSlow`, `ConnectionChurn`,
  `StreamIdle`, `ServerRestart`, `CommentScanFragmented`,
  `PostSortUnindexed`).

### Considered and rejected

- Likes on a post: `SeatClaimHot`'s chart.
- A race on a unique email: `PostCreateRace`'s code.
- Zipf-distributed writes: between `SeatClaimHot` and
  `SeatClaimSpread`, and nearer the cheap end.
- A synthetic heap ballast: nothing in a Volt program can hold one.
- Deadlocks and lock ordering: SQLite has one write lock and no row
  locks.
- Replicas and their lag: one file, one process.
- A pool exhausted by leaked transactions: a handler holds no
  connection, and the runner ends every transaction it begins.
- A foreign key without an index on its child column: a full scan
  for every cascade and every `with`, but visible in the schema; it
  belongs to a vet rule.
- A `Then` chain past the shim's 64 continuations: a unit test of
  that bound.
- HTTP/2 multiplexing and stream resets: the fixture's `main` serves
  HTTP/1.1; it returns with step 16.
- WebSockets, templates, mail, file storage: not planned or not
  built.
- Serving large static files: that measures `http.ServeFileFS`.
- A group with millions of keys: `PostSortUnindexed`'s chart.
- An in-memory database: `run.Open` refuses one.
- Malformed HTTP: the fuzzing of V3.
- A failure at every driver call: V4; `DatabaseFull` keeps only the
  part under load.
- Clock jumps: a handler reads `r.Now()` and starts no timer.
- Slow calls to other services from a handler: no effect kind makes
  one.
- A comparison with another database: not planned.

### What one machine cannot model

- **The network.** Loopback has no loss, no reordering, no
  retransmission timers beyond the listener's backlog, no congestion,
  no NAT table and no load balancer. The relay adds delay and a
  bandwidth cap, which is not the same as a lossy link.
- **The client's own cost.** Client and server share one process,
  its cores, its heap and the OS cache. Capacity is understated, and
  every number is a proportion, as `runner.md` says of its own.
- **The disk.** One device, whichever the directory is on: no
  network block storage with its rare long stalls, no neighbour, no
  device that slows after minutes of sustained writes unless the soak
  finds it, and no throttle. No power loss, so what
  `synchronous = NORMAL` may lose is never observed; a killed process
  loses nothing the OS already holds.
- **Memory pressure.** No kernel reclaim, no swap, no out-of-memory
  kill: `debug.SetMemoryLimit` bounds the Go heap only, and the OS
  cache cannot be made small.
- **CPU limits.** No container quota with its throttled periods, no
  stolen time, no thermal throttling under control; on a laptop the
  same run differs from minute to minute, so the report states the
  machine and the bench spreads its runs across the scenarios.
- **More than one machine.** No load balancer that ejects a server,
  so the cascade that `AuthorLogin`'s missed health checks would
  start is predicted, not shown.
- **Time.** An hour is not a month: a leak under a megabyte an hour,
  statistics that drift, a certificate that expires.
- **People.** The mixes, the Zipf exponent, the bursts and the retry
  policies are guesses about users, each a parameter with its seed.
- **Other builds.** Every number holds for this mattn and its
  SQLite, 3.53.2 today, with the options it was compiled with.

### How it lands

After the review, and after step 37: the fixture's additions, the
seeder and the smoke tier; then the arrivals, the clients, the
actions and the probes in the bench; then the full tier, a cell of
the matrix at a time, each case's sketch recorded before its first
run; then the soak. The catalogue then lives in the bench as data,
one value a case with its sketch, the measurements join step 37's
section of `runner.md`, and this section goes.

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
| 38 | 37 | after the maintainer's review of its design |

The blog fixture under `itest/blog` is the application every step
rebuilds. Measured at each step on it and reported when the step
lands: Volt lines, hand-written Go lines, generated Go lines. The
thesis is that the first two fall while the third is paid for once.
