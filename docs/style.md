# Volt style

> [!CAUTION]
> **This project is an alpha version, and everything may change in the future.**

How Volt's Go code is written: the compiler, the tools, the runtime,
the generators, the tests and the code `volt gen` writes. The editor
stack (`grammar/`, `zed-extension/`) is written in the languages its
editors dictate and stays outside (D2). Two styles are joined here. The
first is
[TigerStyle](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md),
TigerBeetle's engineering style ([tigerstyle.dev](https://tigerstyle.dev)),
built on NASA's [Power of Ten](https://spinroot.com/gerard/pdf/P10.pdf)
rules and translated here from Zig to Go. The second is the functional
style, which Richard Feldman reduces to avoiding mutation and avoiding
side effects. Where Go differs from Zig, the rule says what takes the
original's place; section 7 lists the main differences.

`spec.md` says what Volt is; this file says how its code is written.
Both are normative. The functional rules are the spec's
(§values-and-effects), applied here to every package. Where this file
asks for more than the spec fixes today, the rule names the `plan.md`
step that changes the spec, and until that step lands the spec holds.
Every rule has an ID. A rule marked **gate** is checked by
`cmd/volt/style_test.go` (section 8); every other rule is held in
review, and a change that breaks one is not done.

## 1. Values

Safety, performance and experience, in that order. All three matter;
when they pull apart, the earlier one wins.

- **Zero technical debt.** Code is cheapest to change while it is
  being written. What is found is fixed now, the best way known, the
  first time, and a showstopper is solved when it is found: no latency
  spike from a large copy, no algorithm of exponential cost slips
  through. The debt older than this file is inventoried and only
  shrinks: the ledger of section 8 holds the debt to the gated rules,
  and `plan.md` steps 22 to 34 carry the debt to the rest.
- **Design first.** Safety and performance are won before the code
  exists: the spec section, the sketch of the four resources (P1), the
  limits (S2) and the assertions (S5) come first. A day of design is
  worth weeks in production.
- **Simplicity is the last revision, not the first draft.** It takes
  thought, sketches and rewrites, and it is where the three values
  meet.
- **Always say why.** A reason given is a reason that can be checked.
  For now an agent's reasons go into the spec section, the `runner.md`
  section or the `plan.md` step its change touches (C1).

## 2. The functional rules

The five rules of spec §values-and-effects (values; no mutation after
construction; effects at the edge, as data; mutable state inventoried;
pure phases in the tooling) hold for every package of the repository,
not only for the generated code and the runtime.

- **F1. Values.** Data structures are values. A method on a value
  returns a new value and leaves its receiver as it was. Review.
- **F2. Owned state only.** A function mutates only what it created
  and nobody else can see: a local slice, map or struct that has not
  escaped. It never writes through an argument or a receiver, nor
  package state. Leaf functions are pure: a helper computes what should
  change and returns it, and the parent, which owns the state, applies
  it. A function literal may write the locals of the function that
  creates it only when it runs synchronously inside that function, as
  the visitor of `ast.Inspect`, `filepath.WalkDir` or `packages.Visit`
  does; a function value that may run later or concurrently (the `next`
  of `rt.Then`, the `f` of `rt.Map`, a goroutine) writes nothing it
  captures. An `init` function builds package state, which is
  construction; nothing writes that state after. **Gate:**
  `argument-mutation` for writes through parameters and receivers,
  `package-write` for package variables.
- **F3. Effects at the edge, as data.** Files, the network, the
  database, the clock, randomness, the environment and logging happen
  in one place per lifetime: `main`; the runner `nao/run`; the network
  runtime of the root package (the generated shim, the event broker and
  the client); the loader behind `lang/source`; the final write of
  `volt gen`; the transport loop of the language server; and the
  worker, when `plan.md` step 15 builds it. Goroutines that compute
  without I/O and join before they return (`rt.Parallel`,
  `rt.ParallelMap`, `internal/par`) are CPU parallelism inside pure
  code, as `plan.md` part 0 states, not effects. In a test, the edge is
  the test function and the helpers it calls to read and write; what
  they read is handed to pure functions. Everything else computes
  values, descriptions of effects included. Review;
  `cmd/volt/layers_test.go` holds the imports of the layers.
- **F4. Mutable state is inventoried.** The connection pool, the
  statement cache, the event broker's log and the logger are the whole
  list. Their own methods may mutate them: the methods of `nao/run`'s
  `readers`, `StmtCache` and `cacheEntry` and of the root package's
  `Broker` (the gate's `styleInventory`), and those of `log/slog`'s
  logger, whose state lives inside the standard library. Nothing else
  may. A test that holds the import graph to this list is `plan.md`
  step 33.
- **F5. Pure phases.** The scanner, the parser, the checker, the fold
  and the generators are functions from values to values, and so is the
  formatter when `volt fmt` lands. The purity gate
  (`cmd/volt/purity_test.go`) proves it for the memoized computations;
  `plan.md` step 33 extends it to every phase.

Go idioms, one per need, so the code stays uniform:

| Need | Idiom |
|---|---|
| Sum types | A sealed interface: one method whose name starts with a lowercase letter, one small struct per kind in the same package, constructors that return the interface, a type switch in the single consumer. A test compares the list of kinds with the switch, since Go does not check exhaustiveness. |
| Immutable records | Structs with unexported fields, exported constructors, `With*` methods on value receivers. |
| Copy on write | `slices.Clone` before appending to a slice that came from outside; never `append` to an argument. |
| Optional values | `rt.Null[T]`; never a pointer in a public API. |
| Effects | A small set of effect kinds in `nao/rt`, each implementing a sealed `Effect[T]` whose methods are unexported, so a type of another package can only embed a kind or an `Effect` and is then the effect it embeds; the runner performs one through `rt.Perform`, each kind describes itself for `rt.Describe` and `rt.Same`, and a test holds the kinds to their descriptions. |

The gates do not see a write through a method call on a parameter or
receiver, through a local alias of one, through an address passed to a
call, or by a callee other than the builtins and standard library
functions section 8 lists; nor reflection, `unsafe` or cgo. Review does.

## 3. Safety

- **S1. Simple control flow.** No recursion: it becomes a loop over an
  explicit stack, with a depth limit that is reported when input
  crosses it and asserted where the program guarantees it (S2, S5). No
  `goto`; a labeled `break` or `continue` only leaves nested loops. A
  panic is an assertion failure and nothing else: an operating error is
  returned, never panicked, and nothing recovers a panic except the
  runner, which raises a `Parallel` step's panic again, and the shim's
  exit path of `plan.md` step 22. Today the parsers panic on a syntax
  error and recover it, and `renderEncode` recovers any panic into an
  error; `plan.md` steps 24 and 22 remove both. Abstractions are few,
  and each must be the best sense of the domain: none is free, and
  every one can leak. **Gate:** `recursion`, for cycles of calls, where
  passing a function to a call counts as calling it and a function
  literal that refers to the variable it is assigned to counts as
  recursion. Recursion through an interface or through a function value
  stored elsewhere is review.
- **S2. A limit on everything.** Every loop, queue, channel, buffer,
  cache, map that grows with input or time, input size, nesting depth,
  retry and timeout has a fixed upper bound. Crossing a bound is a
  reported error when input crossed it and an assertion when the
  program's own state did. A loop is bounded when:
  - it is counted: its post statement steps an identifier with `++`,
    `--`, `+=` or `-=`, its condition, or one of the condition's `&&`
    terms, compares that identifier with `<`, `<=`, `>` or `>=` against
    a bound in the direction it moves, and the body writes neither the
    identifier nor the bound;
  - it ranges over a finite value: a slice, array, string, map or
    integer, or one of the standard library iterators section 8 lists;
    a range over a channel or over any other function is not bounded;
  - or it provably never ends: a `for` without a condition that no
    `return`, `goto`, labeled branch to outside it, or unlabeled
    `break` of its own leaves. Such a loop lives as long as the process
    and asserts what keeps it alive, a channel that must stay open for
    one.

  A loop that serves a connection or a process and ends with its input
  or its context, an event loop, does bounded work per iteration; its
  shape gets a check in `plan.md` step 27, and until then the gate
  counts it. **Gate:** `loop-unbounded`; the other limits are review.
- **S3. Bounded allocation.** Go collects garbage, so TigerStyle's
  "allocate everything at startup" becomes: nothing grows without a
  limit, and the hot paths (per request, per row, per token)
  preallocate and reuse instead of allocating per item. Reuse happens
  inside one owning function, across the rows or tokens of a single
  call; a buffer kept from one call to the next is inventoried state
  (F4). Review.
- **S4. Fixed-size integers at boundaries.** What is stored, sent over
  the wire or declared in the schema has a fixed size (`int32`,
  `int64`, `uint32`); `int` is for in-memory lengths and indexes, as
  Go's `len` returns it. Go wraps on overflow and truncates on
  conversion without a sound, where Zig's safe builds trap, so a
  conversion to a narrower or differently signed type, and arithmetic
  at a boundary that can overflow, is checked: an assertion for the
  program's own values, a reported error for input. Until `plan.md`
  step 30 changes spec §go-types, the schema's `int` is Go's `int` on
  the 64-bit targets the spec fixes. Review.
- **S5. Assertions.** An assertion checks what the program itself
  guarantees, so its failure is a programmer error. An operating error
  (bad input, a full disk, a refused connection) is expected and
  handled; an assertion failure is not, and the only correct response
  to corrupt code is to crash. Assertions:
  - use the assert package of `plan.md` step 22, which panics and stays
    on in production, in hand-written and generated code alike;
  - average at least two per function, over arguments, results,
    preconditions, postconditions and invariants;
  - have no side effect: an assertion reads, calls nothing that writes,
    and the program means the same without it;
  - are never padding: an assertion a tool can prove always true or
    always false does not count;
  - check the positive space, what must hold, and the negative space,
    what must not;
  - come in pairs: the call site and the definition, the write and the
    read back, the generator and the parser of its output
    ([It Takes Two to Contract](https://tigerbeetle.com/blog/2023-12-27-it-takes-two-to-contract));
  - are split, two assertions rather than one over `a && b`, and an
    implication is `if a { assert(b) }`;
  - check relationships between constants, and type sizes through
    `unsafe.Sizeof`, at compile time, with a constant expression that
    fails to compile when the relationship breaks;
  - may state a surprising invariant where a comment would explain it,
    which for now is how an agent documents one (C1);
  - encode a model of the code that was built first, and do not stand
    in for it.

  Volt sides with TigerStyle over SQLite and the Power of Ten here:
  assertions are never compiled out or disabled, a failure crashes
  instead of returning an error, and there is one kind of assertion; a
  check that must recover is an operating error and is handled as one.
  Until `plan.md` step 22 lands, a panic inside a request ends that
  request alone (spec §generated-query-surface). Review until then; the
  density gate lands with the package.
- **S6. Short functions.** At most 70 lines from `func` to the closing
  brace, so a function fits on a screen. A good shape has few
  parameters, a simple result and the work between: the parent keeps
  the branches and the state, the helpers do the work without branching
  (push ifs up and fors down). **Gate:** `function-length`.
- **S7. Smallest scope.** A variable is declared where it is first
  needed and is not used after its purpose ends; few variables are in
  scope at once. Review.
- **S8. Every error is handled.** No error result is dropped: not by a
  call written as a statement, in a `defer` or a `go`, nor by a blank in
  a tuple. Discarding one is written `_ = f()`, and only where failure
  cannot change the outcome. The result of a call to a pure function is
  used, since as a statement it does nothing. Most catastrophic failures
  of distributed systems come from mishandled non-fatal errors
  ([Yuan et al., OSDI 2014](https://www.usenix.org/system/files/conference/osdi14/osdi14-paper-yuan.pdf)).
  **Gate:** `error-discarded`, which trusts only the writes to a
  `strings.Builder` or a `bytes.Buffer`, documented never to fail; a
  pure call written as a statement is review.
- **S9. No warnings.** `go vet` and the gates report nothing, and
  `plan.md` step 33 adds the `nilness` and `unusedwrite` analyzers.
  When an analyzer is confused, the code is rewritten until it is
  plainly right, without changing what it does, and the analyzer is not
  silenced. Held by the verification bar.
- **S10. Run at your own pace.** The worker, the event broker and the
  periodic loops do not react to each external event: they wake on an
  interval, take a bounded batch and finish it. The worker and the
  periodic loops are not built (`plan.md` step 15), and the broker today
  sends each event as it is published, which is debt (`plan.md` step
  23). Each request is served on its own, in parallel as `net/http`
  delivers them, and is never batched with another; `runner.md` section
  7 keeps the note on batching writes, which is not built. Review.
- **S11. Control flow a reader can check.** A compound condition
  becomes nested branches; a long `else if` chain becomes nested blocks
  or a `switch` whose cases exclude each other; an invariant is stated
  positively (`index < count`, not `index >= count`); an `if` without an
  `else` is a decision that the negative space needs nothing. Review.
- **S12. Explicit options.** A call passes the options that matter
  instead of trusting a library default that may change: the timeouts
  and header limits of `http.Server`, `http.MaxBytesReader`, pool
  sizes, file modes, deadlines on contexts, and the SQLite pragmas,
  `synchronous` included. Review.
- **S13. Run to completion.** A function does not wait (on a channel, a
  lock or I/O) between an assertion and the code that relies on it;
  after a wait it asserts again, or holds the lock that keeps the
  assertion true. A pure function never waits. Review.
- **S14. Few configurations.** Build constraints are few, each is
  justified, and every configuration they select runs in the
  verification bar. No `//go:linkname`. Review.
- **S15. Calls a reader can follow.** A function value or an interface
  method is called only where its set of callees is closed and known:
  the sealed effect kinds, the `next` of `rt.Then`, a callback passed
  down and called in place. No pointer to a pointer. Review.

## 4. Performance

- **P1. Sketch before building.** Performance is won in design, when
  there is nothing yet to profile. A back-of-the-envelope sketch of the
  four resources (network, storage, memory, compute), each by bandwidth
  and latency, is cheap and lands close to the best design.
  `runner.md` is where the sketches meet the measurements.
- **P2. The slowest resource first.** Network, then storage, then
  memory, then compute, each weighted by how often it is used: a cache
  miss repeated a million times can cost more than one fsync.
- **P3. Control plane and data plane.** Batching separates them: the
  control plane decides and asserts once per batch, and the data plane
  runs through the batch without branching, so the CPU sprints through
  large units of work. Batching amortizes the network, storage, memory
  and compute inside one request (`rt.Batch`, `CreateMany`, a statement
  prepared once, nested loads as separate batched statements) and
  inside one own-pace loop, never across requests (S10).
- **P4. Explicit over clever.** The compiler is not counted on: a hot
  loop goes into a function that takes primitive arguments, not a
  struct whose fields it would have to prove it can keep in registers.
- **P5. No copies in the data plane.** No copying, encoding or decoding
  that the data plane does not need. A buffer reused for speed is owned
  (S3) and cleared to the length it is used at (X4).

## 5. Experience

### Names

- **N1. Go casing, big-endian order.** MixedCaps as Go requires; the
  most significant word first and the qualifiers and units last, so
  related names sort and line up together: `queueLengthMax` and
  `queueLengthMin`, not `maxQueueLength`. This is AGENT.md's
  subject-first, verb-last rule carried to every word.
- **N2. No abbreviations,** except Go's idioms: `ctx`, `err` and `ok`;
  `i`, `j` and `k` for indexes; `w` and `r` in a handler; `t`, `b` and
  `f` for the values of a test, a benchmark and a fuzz target.
- **N3. One case per acronym:** `ID`, `URL`, `SQL`, as Go has them.
- **N4. Units in the name, or in the type.** In memory a duration is a
  `time.Duration` and its name carries no unit (`timeoutMax`); a size or
  a count names its unit (`bodyBytesMax`, `rowCount`). On the wire and
  in storage, where S4 makes a value a fixed-size integer, the unit goes
  into the name (`timeoutMs int64`). An index, a count and a size
  differ: an index is zero-based, a count one-based, and a size is a
  count times a unit.
- **N5. Equal lengths for related names** where it is easy: `source`
  and `target` line up, `src` and `dest` do not.
- **N6. A helper is named after its caller,** subject first:
  `sectorRead` calls `sectorReadDone`.
- **N7. Arguments in order.** A `context.Context` comes first, then the
  singletons such as a logger, from the most general to the most
  specific, and callbacks last, since they run last.
- **N8. One meaning per name,** and a noun over a participle, since a
  noun can head a sentence: `pipeline`, not `preparing`.
- **N9. No swappable arguments.** Two parameters of one type that a
  caller could exchange go into an options struct. A parameter that can
  be nil is named so that a `nil` at the call site reads clearly.
- **N10. File names** are lowercase snake case. **Gate:** `file-name`.
- **N11. Order.** What matters comes first: the entry point at the top
  of a file, then each type with its constructors and its methods, then
  the types it uses; where no order is right, big-endian names sorted
  alphabetically.
- **N12. Long flags.** A command's flags have long names (`--force`,
  not `-f`); a one-letter flag is for interactive use only.
- **N13. A test says what it proves:** its name and the names of its
  subtests state the goal and the case, so a reader can skip to what
  they need.
- **N14. For the rest,** Effective Go and Go Code Review Comments.

### Shape

- **X1. Fewer dimensions.** A simpler result removes branches from every
  caller: no result beats `bool`, `bool` beats an integer, an integer
  beats `(T, bool)`, and `(T, bool)` beats `(T, error)`.
- **X2. Values, not aliases.** State is neither duplicated nor aliased,
  so it cannot fall out of sync. TigerStyle passes a large argument by
  const pointer; Go has no const, but F2 and the `argument-mutation`
  gate make every pointer parameter read-only, so a large argument may
  be passed by pointer. TigerStyle's initialization in place through an
  out pointer becomes a constructor that returns a value.
- **X3. Division says how it rounds.** Go's `/` truncates toward zero,
  which for a negative operand is neither floor nor ceiling, so a
  division goes through a function that names its rounding (exact,
  floor or ceiling) and asserts it.
- **X4. No buffer bleeds.** A reused buffer is cleared to the length it
  is used at, so no stale byte is read or sent.
- **X5. Acquire and release together.** A resource is acquired and its
  `defer` release written next to each other, with a blank line before
  and after them, so a leak shows.

### Numbers

- **L1.** `gofmt`, with tabs; its output is not argued with.
- **L2. 100 columns.** No line of Go, Volt or SQL is wider than 100
  columns, counting a tab as four, so two files fit side by side; the
  repository's `.editorconfig` sets four-column tabs for GitHub and
  editors. **Gate:** `line-length`.

### Comments and commits

- **C1. Comments are the maintainer's.** For now an agent writes no
  comment in Go or Volt source, doc comments included. These are not
  comments in this sense: directives (`//go:build`, `//go:generate`,
  `//go:embed`), the header of a generated file, the `// spec: §…` tag
  of a conformance snippet, and the doc comments a fixture or a snippet
  needs in order to test them. The maintainer's comments are doc
  comments on exported identifiers, one or two sentences that cite the
  spec section and say why; nothing else is commented. A generated file
  carries its schema's doc comments (spec
  §doc-comments-in-generated-code).
- **C2. A commit message is `.`,** the maintainer's rule. TigerStyle
  asks for descriptive messages; Volt does not.

### Dependencies and tools

- **D1. Zero dependencies** beyond the Go toolchain, except the table of
  section 8: mattn/go-sqlite3, the settled driver, and
  golang.org/x/tools, which `volt gen --verify` and the gates load
  packages with, with the two modules it needs (x/mod and x/sync).
  `urfave/cli/v3` leaves the tool for the standard library's `flag`
  (`plan.md` step 32). **Gate:** `dependency`.
- **D2. One toolbox.** Tools, tests and scripts are Go, as AGENT.md's
  stack says. The exceptions are the editor stack, in the languages its
  editors dictate, and the external tools `plan.md` step 20 removes.

## 6. Verification

People trust SQLite because of how it is tested
([How SQLite Is Tested](https://www.sqlite.org/testing.html)), and
TigerBeetle because of its assertions and its deterministic simulator.
Volt's assertions are TigerStyle's, not SQLite's: SQLite compiles
`assert()` out of its release builds and keeps `ALWAYS` and `NEVER` with
recovery code behind them
([the use of assert in SQLite](https://www.sqlite.org/assert.html)),
where Volt keeps one kind, always on (S5). The rules Volt keeps toward
the same trust:

- **V1. A bug is fixed with the test that shows it.** It is not fixed
  until a test fails without the fix.
- **V2. The negative space is tested:** invalid input, input at and
  past every limit (S2), and valid data becoming invalid, not only the
  inputs that work.
- **V3. Parsers and decoders are fuzzed.** The scanner, the parser, the
  checker and every decoder of what a request carries have native Go
  fuzz targets, and the formatter will when `volt fmt` lands. `go test`
  replays their corpora on every run; each target runs under
  `go test -fuzz` for a fixed time before a release and after a change
  to its package. A crasher becomes a seed and stays.
- **V4. Failures are injected.** At each driver call of a write and of
  a migration (`Exec`, `Query`, `Next`, `Commit`), stepped one call at a
  time until the operation completes, a failure is injected once and
  then persistently, and again during the recovery from an earlier one;
  a full database comes from `PRAGMA max_page_count`, and a crash from a
  child process killed in the middle of a write. Every run ends with
  `PRAGMA integrity_check`, and the data is as it was before or after,
  never in between. The I/O inside SQLite's own commit is left to
  SQLite's tests, since mattn exposes no VFS.
- **V5. Randomness is reproducible.** A randomized test takes a seed
  and prints it. The effect layer's concurrency is tested by
  deterministic simulation, after TigerBeetle's VOPR: a simulated
  executor behind `rt.Perform` and a seeded order for the steps of a
  `Parallel`. It tests Volt's effects, not the threading of mattn or
  SQLite, which Go's scheduler and cgo keep out of a seed's reach.
- **V6. Coverage does not fall.** Statement coverage is measured per
  package and does not fall. Go has no branch coverage, so a boundary
  condition carries a test-only record of both of its outcomes, after
  SQLite's `testcase()`, and the suite fails when one side never ran.
- **V7. Assertions multiply the tests.** Every test, fuzz case and
  simulation also checks every assertion it passes through (S5).
- **V8. Round trips are checked:** generate, compile and run (the
  goldens and the fixtures); migrate and compare with the declared
  schema (the fold); encode and decode the wire formats; and parse,
  format and parse again, once `volt fmt` lands.
- **V9. Fast paths agree with plain ones.** A fast path and its plain
  path give the same answer on the same input: a cached statement and a
  fresh one, `CreateMany` and repeated `Create`, batched nested loads
  and a join.
- **V10. No races and no leaks.** Every package that starts goroutines
  or holds inventoried state runs under `-race` in the verification
  bar, and its tests check that no goroutine, connection or statement
  outlives them.
- **V11. The verification bar runs on every change,** and nothing is
  weakened to pass it.

Where each rule stands is `plan.md` step 34's business.

## 7. Where Go differs from Zig

| TigerStyle | Volt |
|---|---|
| `snake_case` names | Go casing in big-endian order (N1) |
| Units in the names of integer durations | `time.Duration` in memory, units in the names on the wire (N4) |
| Do not abbreviate | Go's idioms `ctx`, `err`, `ok`, `w`, `r`, `t` stay (N2) |
| Follow the Zig style guide | Effective Go and Go Code Review Comments (N14) |
| Fields, then types, then methods, inside a struct | The entry point, then each type with its constructors and methods (N11) |
| All memory allocated at startup | Bounded allocation (S3) |
| `u32` rather than `usize` | Fixed sizes at boundaries, `int` for lengths and indexes (S4) |
| Safe builds trap on overflow and lossy casts | Conversions and boundary arithmetic checked by hand (S4) |
| `assert` and `comptime` assertions | The assert package, and constant expressions that fail to compile (S5) |
| A function runs to completion without suspending | No wait between an assertion and what relies on it (S13) |
| Pass more than 16 bytes as `*const` | Pass by pointer, read-only by F2 (X2) |
| Initialization in place through an out pointer | Constructors return values (X2) |
| `@divExact`, `@divFloor`, `div_ceil` | `/` truncates; a division names its rounding (X3) |
| `zig fmt` with four spaces | `gofmt` with tabs counted as four columns (L1, L2) |
| Braces on every `if` | Go requires them |
| Compiler warnings at the strictest setting | `go vet` and the gates, more analyzers in `plan.md` step 33 (S9) |
| Zero dependencies | mattn and golang.org/x/tools (D1) |
| Scripts in Zig | Tools in Go, the editor stack excepted (D2) |
| Comments say why | The maintainer writes the comments (C1) |
| Descriptive commit messages | `.` (C2) |
| Batch rather than react to each event | The worker, the broker and the loops; requests never batched (S10) |
| The VOPR simulates a whole cluster | A simulation of the effect layer (V5) |

The Power of Ten rules that TigerStyle leaves implicit, on the
preprocessor and on pointers, are S14 and S15.

## 8. The style gate

`cmd/volt/style_test.go` checks the rules marked gate and runs with
`go test ./cmd/volt/...`, inside the verification bar. It reads every
`.go`, `.go.golden`, `.volt`, `.sql` and `.sql.golden` file of the
repository outside `.git`, `grammar`, `node_modules` and
`zed-extension`. `line-length` runs on every file it reads. The checks
that need only syntax run on every Go file and Go golden:
`function-length`, `file-name` and the `for` statements of
`loop-unbounded`. The checks that need types run on every package of
the three modules: `recursion`, `argument-mutation`, `package-write`,
`error-discarded` and the `range` statements of `loop-unbounded`. They
reach the test files with `plan.md` step 33; `testdata` and the
conformance snippets, which are fixtures, stay outside them.

| Rule | Check | What it counts | Limit |
|---|---|---|---|
| S6 | `function-length` | the lines of a function or a package-level function literal, from `func` to its closing brace | 70 |
| L2 | `line-length` | the lines of a file wider than the limit, a tab counting four columns | 100 |
| S1 | `recursion` | the functions on a cycle of calls, a function passed to a call counting as called, and the functions holding a function literal that refers to the variable it is assigned to | none |
| F2 | `argument-mutation` | the writes of a function, or of a function literal inside it, through a parameter or receiver into memory a caller can see: an assignment or range assignment through a pointer, slice, reslice or map; `append`, `copy`, `delete` and `clear`; and the editing functions of `sort`, `slices` and `maps` that the purity gate lists; inside the methods of an inventoried type, that type is exempt | none |
| F4 | `package-write` | the writes to a package-level variable outside `init`: assigning it or through it, taking its address, and calling a method that takes its address, except the `Load` methods of `sync/atomic` | none |
| S8 | `error-discarded` | the error results a function drops: a call written as a statement, in a `defer` or a `go`, and a blank for an error in a tuple assignment | none |
| S2 | `loop-unbounded` | the loops of a function that are not bounded as S2 defines | none |
| N10 | `file-name` | the Go files not named in lowercase snake case | none |
| D1 | `dependency` | the requirements of a `go.mod` outside the table below | none |

| Module file | Requirements it may have |
|---|---|
| `go.mod` | `github.com/mattn/go-sqlite3` |
| `lsp/go.mod` | `github.com/Piechutowski/volt` |
| `cmd/volt/go.mod` | `github.com/Piechutowski/volt`, `github.com/Piechutowski/volt/lsp`, `github.com/mattn/go-sqlite3`, `golang.org/x/mod`, `golang.org/x/sync`, `golang.org/x/tools` |

The standard library iterators that S2 counts as finite are the `All`,
`Keys` and `Values` of `maps`; the `All`, `Backward` and `Values` of
`slices`; and the `Lines`, `SplitSeq`, `SplitAfterSeq`, `FieldsSeq` and
`FieldsFuncSeq` of `strings` and of `bytes`.

**The ledger.** `cmd/volt/testdata/style.ledger` lists the debt to the
gated rules that is older than this file, one line per function, file,
variable or requirement, with its count or, for a long function, its
length. The gate fails when the code holds more than the ledger: a new
violation, or more of an old one. It also fails when the code holds
less, until the ledger is lowered with `go test ./cmd/volt -run
TestStyleLedger -update`, which never raises an entry and never
recreates a missing ledger. An entry is raised by hand only in the
commit that changes what a check counts, together with this section,
where the diff shows it; a rename moves its entry by hand in the same
way.

**What the gate cannot see:** a write through a method call on a
parameter or receiver, through a local alias of one, through an address
passed to a call, or by a callee other than the builtins and the
standard library functions listed above; a write to package state made
by a call, such as `flag.Parse`, or through a method of a package-level
pointer; recursion through an interface or through a function value
stored elsewhere; a pure call written as a statement; and every rule
held in review.

**The examples are executable.** `TestStyleDoc` runs every example
below through the gate: a bad example must fail its check and a good
one must pass it. It holds the limits of the first table and the
requirements of the second to the gate's. The rest of this section is
held in review.

### `recursion`

```go bad recursion
type Node struct {
	Children []Node
}

func nodeCount(node Node) int {
	count := 1
	for _, child := range node.Children {
		count += nodeCount(child)
	}
	return count
}
```

```go good recursion
type Node struct {
	Children []Node
}

const (
	nodeCountMax = 1 << 20
	nodeDepthMax = 64
)

type nodeFrame struct {
	node  Node
	depth int
}

func nodeCount(root Node) (int, bool) {
	stack := []nodeFrame{{root, 1}}
	count := 0
	for ; count < nodeCountMax && len(stack) > 0; count++ {
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if frame.depth > nodeDepthMax {
			return 0, false
		}
		for _, child := range frame.node.Children {
			stack = append(stack, nodeFrame{child, frame.depth + 1})
		}
	}
	return count, len(stack) == 0
}
```

### `argument-mutation`

```go bad argument-mutation
type User struct {
	Name string
}

func userNameSet(user *User, name string) {
	user.Name = name
}
```

```go good argument-mutation
type User struct {
	Name string
}

func userNameSet(user User, name string) User {
	user.Name = name
	return user
}
```

```go bad argument-mutation
func tagsAdded(tags []string, tag string) []string {
	return append(tags, tag)
}
```

```go bad argument-mutation
func tagLastReplaced(tags []string, tag string) []string {
	return append(tags[:len(tags)-1], tag)
}
```

```go good argument-mutation
import "slices"

func tagsAdded(tags []string, tag string) []string {
	return append(slices.Clone(tags), tag)
}
```

### `package-write`

```go bad package-write
var requestCount int

func requestCounted() {
	requestCount++
}
```

```go good package-write
var statusTexts = map[int]string{200: "OK", 404: "Not Found"}

func statusText(code int) string {
	return statusTexts[code]
}
```

### `error-discarded`

```go bad error-discarded
type File struct{}

func (File) Close() error { return nil }

func fileUse(file File) {
	defer file.Close()
}
```

```go good error-discarded
type File struct{}

func (File) Close() error { return nil }

func fileUse(file File) error {
	return file.Close()
}
```

```go bad error-discarded
import "strconv"

func port(text string) int {
	value, _ := strconv.Atoi(text)
	return value
}
```

```go good error-discarded
import "strconv"

func port(text string) (int, error) {
	return strconv.Atoi(text)
}
```

### `loop-unbounded`

```go bad loop-unbounded
func queueSum(queue []int) int {
	total := 0
	for len(queue) > 0 {
		total += queue[0]
		queue = queue[1:]
	}
	return total
}
```

```go good loop-unbounded
func queueSum(queue []int) int {
	total := 0
	for index := 0; index < len(queue); index++ {
		total += queue[index]
	}
	return total
}
```

```go bad loop-unbounded
func halvings(start int) int {
	queue := []int{start}
	for index := 0; index < len(queue); index++ {
		if queue[index] > 1 {
			queue = append(queue, queue[index]/2)
		}
	}
	return len(queue)
}
```

```go good loop-unbounded
const halvingsMax = 64

func halvings(start int) int {
	queue := []int{start}
	index := 0
	for ; index < len(queue) && index < halvingsMax; index++ {
		if queue[index] > 1 {
			queue = append(queue, queue[index]/2)
		}
	}
	if index < len(queue) {
		panic("halvings: more halvings than halvingsMax")
	}
	return len(queue)
}
```

```go bad loop-unbounded
func requestsServe(requests chan int, handle func(int)) {
	for request := range requests {
		handle(request)
	}
}
```

```go good loop-unbounded
func requestsServe(requests chan int, handle func(int)) {
	for {
		request, open := <-requests
		if !open {
			panic("requestsServe: the requests channel closed")
		}
		handle(request)
	}
}
```

```go bad loop-unbounded
func naturals(yield func(int) bool) {
	for number := 0; number < 10; number++ {
		if !yield(number) {
			return
		}
	}
}

func naturalsSum() int {
	total := 0
	for number := range naturals {
		total += number
	}
	return total
}
```
