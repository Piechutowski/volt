# Volt style

> [!CAUTION]
> **This project is an alpha version, and everything may change in the future.**

How Volt's code is written: the compiler, the tools, the runtime, the
generators, the tests and the code `volt gen` writes. Two styles are
joined here. The first is
[TigerStyle](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md),
TigerBeetle's engineering style ([tigerstyle.dev](https://tigerstyle.dev)),
built on NASA's [Power of Ten](https://spinroot.com/gerard/pdf/P10.pdf)
rules and translated here from Zig to Go. The second is the functional
style, which Richard Feldman reduces to avoiding mutation and avoiding
side effects. Where Go differs from Zig, the rule says what takes the
original's place; section 7 lists every difference.

`spec.md` says what Volt is; this file says how its code is written.
Both are normative and they agree: the functional rules are the spec's
(§values-and-effects), applied here to every package. Every rule has an
ID. A rule marked **gate** is checked by `cmd/volt/style_test.go`
(section 8), whose ledger lists the debt older than this file and only
shrinks. Every other rule is held in review: a change that breaks one
is not done.

## 1. Values

Safety, performance and experience, in that order. All three matter;
when they pull apart, the earlier one wins.

- **Zero technical debt.** Code is cheapest to change while it is
  being written. What is found is fixed now, the best way known, the
  first time. The ledger of section 8 is the one inventory of debt,
  and it holds only debt older than this file.
- **Design first.** Safety and performance are won before the code
  exists: the spec section, the sketch of the four resources (P1), the
  limits (S2) and the assertions (S5) come first. A day of design is
  worth weeks in production.
- **Simplicity is the last revision, not the first draft.** It takes
  thought, sketches and rewrites, and it is where the three values
  meet.

## 2. The functional rules

The five rules of spec §values-and-effects (values; no mutation after
construction; effects at the edge, as data; mutable state inventoried;
pure phases in the tooling) hold for every package of the repository,
not only for the generated code and the runtime.

- **F1. Values.** Data structures are values. A method on a value
  returns a new value and leaves its receiver as it was. Review.
- **F2. Owned state only.** A function mutates only what it created
  and nobody else can see: a local slice, map or struct that has not
  escaped. It never writes through an argument, a receiver, a variable
  another function captures, or package state. Leaf functions are
  pure: a helper computes what should change and returns it, and the
  parent, which owns the state, applies it. **Gate:**
  `argument-mutation`, for writes through parameters and receivers; a
  write through a method call is review.
- **F3. Effects at the edge, as data.** Files, the network, the
  database, the clock, randomness, the environment, goroutines and
  logging happen in one place per lifetime: `main`, the runner
  `nao/run`, the generated shim, the worker, the loader behind
  `lang/source`, the final write of `volt gen` and the transport loop
  of the language server. Everything else computes values,
  descriptions of effects included. Review; `cmd/volt/layers_test.go`
  holds the imports.
- **F4. Mutable state is inventoried.** The connection pool, the
  statement cache, the event broker's log and the logger are the whole
  list, and `cmd/volt/layers_test.go` holds the import graph to it.
  Their types (`nao/run`'s `readers`, `StmtCache` and `cacheEntry`, the
  root package's `Broker`, and `log/slog`'s logger) may mutate
  themselves, and nothing else may. **Gate:**
  `package-state`, for writes to package-level variables outside
  `init`; the exemption of the inventory from `argument-mutation` is
  the gate's `styleInventory`.
- **F5. Pure phases.** The scanner, the parser, the checker, the fold,
  the generators and the formatter are functions from values to
  values. The purity gate (`cmd/volt/purity_test.go`) proves it for the
  memoized computations; `plan.md` extends it to every phase.

Go idioms, one per need, so the code stays uniform:

| Need | Idiom |
|---|---|
| Sum types | A sealed interface: one method whose name starts with a lowercase letter, one small struct per kind in the same package, constructors that return the interface, a type switch in the single consumer. A test compares the list of kinds with the switch, since Go does not check exhaustiveness. |
| Immutable records | Structs with unexported fields, exported constructors, `With*` methods on value receivers. |
| Copy on write | `slices.Clone` before appending to a slice that came from outside; never `append` to an argument. |
| Optional values | `rt.Null[T]`; never a pointer in a public API. |
| Effects | A small set of effect kinds in `nao/rt`, each implementing a sealed `Effect[T]` whose methods are unexported, so a type of another package can only embed a kind or an `Effect` and is then the effect it embeds; the runner performs one through `rt.Perform`, each kind describes itself for `rt.Describe` and `rt.Same`, and a test holds the kinds to their descriptions. |

The gates do not see a write through reflection, `unsafe`, cgo or a
method call; review does.

## 3. Safety

- **S1. Simple control flow, no recursion.** Recursion becomes a loop
  over an explicit stack, with a depth limit that is asserted (S2,
  S5). Abstractions are few, and each must be the best sense of the
  domain: none is free, and every one can leak. **Gate:** `recursion`,
  for cycles of calls, where passing a function to a call counts as
  calling it. Recursion through an interface or a stored function value
  is review.
- **S2. A limit on everything.** Every loop, queue, channel, cache, map
  that grows with input or time, input size, nesting depth, retry and
  timeout has a fixed upper bound. Crossing a bound is a reported error
  when input crossed it and an assertion when the program's own state
  did. A loop is bounded when it steps a counter that its condition
  compares, when it ranges over a finite value (a slice, array, string,
  map, integer or function iterator), or when it provably never ends: a
  `for` without a condition that no `return`, `goto`, labeled `break` or
  `continue`, or `break` of its own leaves.
  **Gate:** `loop-unbounded`; the other limits are review.
- **S3. Bounded allocation.** Go collects garbage, so TigerStyle's
  "allocate everything at startup" becomes: nothing grows without a
  limit, and the hot paths (per request, per row, per token)
  preallocate and reuse instead of allocating per item. Review.
- **S4. Fixed-size integers at boundaries.** What is stored, sent over
  the wire or declared in the schema has a fixed size (`int32`,
  `int64`, `uint32`); `int` is for in-memory lengths and indexes, as
  Go's `len` returns it. Review.
- **S5. Assertions.** An assertion checks what the program itself
  guarantees, so its failure is a programmer error. An operating error
  (bad input, a full disk, a refused connection) is expected and
  handled; an assertion failure is not, and the only correct response
  to corrupt code is to crash. Assertions:
  - use the assert package of `plan.md`, which panics and stays on in
    production, in hand-written and generated code alike;
  - average at least two per function, over arguments, results,
    preconditions, postconditions and invariants;
  - check the positive space, what must hold, and the negative space,
    what must not;
  - come in pairs: the call site and the definition, the write and the
    read back, the generator and the parser of its output
    ([It Takes Two to Contract](https://tigerbeetle.com/blog/2023-12-27-it-takes-two-to-contract));
  - are split, two assertions rather than one over `a && b`, and an
    implication is `if a { assert(b) }`;
  - check relationships between constants at compile time, with a
    constant expression that fails to compile when the relationship
    breaks;
  - are a safety net, not understanding: the model of the code comes
    first and the assertions encode it.

  Review until the assert package lands; its gate lands with it.
- **S6. Short functions.** At most 70 lines from `func` to the closing
  brace, so a function fits on a screen. A good shape has few
  parameters, a simple result and the work between: the parent keeps
  the branches and the state, the helpers do the work without branching
  (push ifs up and fors down). **Gate:** `function-length`.
- **S7. Smallest scope.** A variable is declared where it is first
  needed and is not used after its purpose ends; few variables are in
  scope at once. Review.
- **S8. Every error is handled.** No error result is dropped. Discarding
  one is written `_ = f()`, and only where failure cannot change the
  outcome. Most catastrophic failures of distributed systems come from
  mishandled non-fatal errors
  ([Yuan et al., OSDI 2014](https://www.usenix.org/system/files/conference/osdi14/osdi14-paper-yuan.pdf)).
  **Gate:** `error-discarded`, which trusts only the writes to a
  `strings.Builder` or a `bytes.Buffer`, documented never to fail.
- **S9. No warnings.** `go vet` and the gates report nothing. When an
  analyzer is confused, the code is rewritten until it is plainly
  right, and the analyzer is not silenced. Held by the verification
  bar.
- **S10. Run at your own pace.** The worker, the event broker and the
  periodic loops do not react to each external event: they wake on an
  interval, take a bounded batch and finish it. Requests are served one
  by one, as `net/http` delivers them; `runner.md` section 7 keeps the
  note on batching writes, which is not built. Review.
- **S11. Control flow a reader can check.** A compound condition
  becomes nested branches; a long `else if` chain becomes nested blocks
  or a `switch` whose cases exclude each other; an invariant is stated
  positively (`index < count`, not `index >= count`); an `if` without an
  `else` is a decision that the negative space needs nothing. Review.
- **S12. Explicit options.** A call passes the options that matter
  instead of trusting a library default that may change: the timeouts
  and header limits of `http.Server`, `http.MaxBytesReader`, pool
  sizes, file modes, deadlines on contexts. Review.

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
  runs through the batch without branching. The CPU sprints through
  large units of work.
- **P4. Explicit over clever.** The compiler is not counted on: a hot
  loop goes into a function that takes primitive arguments, not a
  struct whose fields it would have to prove it can keep in registers.
- **P5. No copies in the data plane.** No copying, encoding or decoding
  that the data plane does not need. A buffer reused for speed is owned
  (F2) and cleared to the length it is used at (X4).

## 5. Experience

### Names

- **N1. Go casing, big-endian order.** MixedCaps as Go requires; the
  most significant word first and the qualifiers and units last, so
  related names sort and line up together: `delayMsMax` and
  `delayMsMin`, not `maxDelayMs`. This is AGENT.md's subject-first,
  verb-last rule carried to every word.
- **N2. No abbreviations,** except Go's idioms (`ctx`, `err`, `ok`,
  `i`, `j` and `k` for indexes, `w` and `r` in a handler, `t`, `b` and
  `f` in tests) and the carve-outs AGENT.md names.
- **N3. One case per acronym:** `ID`, `URL`, `SQL`, as Go has them.
- **N4. Units in the name.** A duration, a size or a count says which:
  `timeoutMs`, `bodyBytesMax`, `rowCount`. An index, a count and a size
  differ: an index is zero-based, a count one-based, and a size is a
  count times a unit.
- **N5. Equal lengths for related names** where it is easy: `source`
  and `target` line up, `src` and `dest` do not.
- **N6. A helper is named after its caller,** subject first:
  `sectorRead` calls `sectorReadDone`.
- **N7. Callbacks last** in a parameter list, since they run last.
- **N8. One meaning per name,** and a noun over a participle, since a
  noun can head a sentence: `pipeline`, not `preparing`.
- **N9. No swappable arguments.** Two parameters of one type that a
  caller could exchange go into an options struct.
- **N10. File names** are lowercase snake case. **Gate:** `file-name`.
- **N11. Order.** What matters comes first: the entry point at the top
  of a file, a type before its methods, a struct's fields before its
  nested types.

### Shape

- **X1. Fewer dimensions.** A simpler result removes branches from every
  caller: no result beats `bool`, `bool` beats an integer, an integer
  beats `(T, bool)`, and `(T, bool)` beats `(T, error)`.
- **X2. Values, not aliases.** State is neither duplicated nor aliased,
  so it cannot fall out of sync. Go has no const pointer, so
  TigerStyle's "pass a large argument by const pointer" becomes "pass a
  value", and its initialization in place through an out pointer
  becomes a constructor that returns a value, except for an owned
  buffer in the data plane.
- **X3. Division says how it rounds:** exact, floor or ceiling, through
  a function that asserts which.
- **X4. No buffer bleeds.** A reused buffer is cleared to the length it
  is used at, so no stale byte is read or sent.

### Numbers

- **L1.** `gofmt`, with tabs; its output is not argued with.
- **L2. 100 columns.** No line is wider, counting a tab as four columns,
  so two files fit side by side. **Gate:** `line-length`.

### Comments and commits

- **C1. Comments are the maintainer's.** For now an agent writes none,
  doc comments included. When the maintainer writes them, they say why,
  in full sentences. A generated file carries its schema's doc comments
  (spec §doc-comments-in-generated-code).
- **C2. A commit message is `.`,** the maintainer's rule. TigerStyle
  asks for descriptive messages; Volt does not.

### Dependencies and tools

- **D1. Zero dependencies** beyond the Go toolchain, except the table of
  section 8: mattn/go-sqlite3, the settled driver, and
  golang.org/x/tools with its own two for the gates. `urfave/cli/v3`
  leaves the tool for the standard library's `flag` (`plan.md`).
  **Gate:** `dependency`.
- **D2. One toolbox.** Tools, tests and scripts are Go, as AGENT.md's
  stack says. The exceptions are the editor stack and the external
  tools that `plan.md` step 20 removes.

## 6. Verification

People trust SQLite because of how it is tested
([How SQLite Is Tested](https://www.sqlite.org/testing.html),
[the use of assert in SQLite](https://www.sqlite.org/assert.html)), and
TigerBeetle because of its assertions and its deterministic simulator.
The rules Volt keeps toward the same trust:

- **V1. A bug is fixed with the test that shows it.** It is not fixed
  until a test fails without the fix.
- **V2. The negative space is tested:** invalid input, input at and
  past every limit (S2), and valid data becoming invalid, not only the
  inputs that work.
- **V3. Parsers and decoders are fuzzed** with Go's native fuzzing: the
  scanner, the parser, the checker, the formatter and every decoder of
  what a request carries. A crasher becomes a seed of the corpus and
  stays there.
- **V4. Failures are injected.** I/O errors, a full disk, SQLite's
  errors and crashes are injected at each point of a write and of a
  migration, the point moved one step at a time until the operation
  completes; after each, `PRAGMA integrity_check` passes and the data
  is as it was before or after, never in between.
- **V5. Randomness is reproducible.** A randomized test takes a seed
  and prints it, and the runner's concurrency is tested by
  deterministic simulation, as TigerBeetle's VOPR tests its cluster.
- **V6. Coverage does not fall.** Go measures statements where SQLite
  measures branches, so V2 and the assertions stand in for branch
  coverage.
- **V7. Assertions multiply the tests.** Every test, fuzz case and
  simulation also checks every assertion it passes through (S5).
- **V8. Round trips are checked:** parse, format and parse again;
  generate, compile and run; encode and decode; migrate and compare
  with the declared schema.
- **V9. The verification bar runs on every change,** and nothing is
  weakened to pass it.

Where each rule stands is `plan.md`'s business.

## 7. Where Go differs from Zig

| TigerStyle | Volt |
|---|---|
| `snake_case` names | Go casing in big-endian order (N1) |
| All memory allocated at startup | Bounded allocation (S3) |
| `u32` rather than `usize` | Fixed sizes at boundaries, `int` for lengths and indexes (S4) |
| `assert` and `comptime` assertions | The assert package, and constant expressions that fail to compile (S5) |
| Pass more than 16 bytes as `*const` | Pass values: Go has no const pointer (X2) |
| Initialization in place through an out pointer | Constructors return values (X2) |
| `zig fmt` with four spaces | `gofmt` with tabs counted as four columns (L1, L2) |
| Braces on every `if` | Go requires them |
| Scripts in Zig | Tools in Go (D2) |
| Comments say why | The maintainer writes the comments (C1) |
| Descriptive commit messages | `.` (C2) |
| Batch rather than react to each event | The worker, the broker and the loops; requests one by one (S10) |

## 8. The style gate

`cmd/volt/style_test.go` checks the rules marked gate and runs with
`go test ./cmd/volt/...`, inside the verification bar. It reads every
`.go` and `.go.golden` file of the repository outside `.git`,
`grammar`, `node_modules` and `zed-extension` for the checks that need
only syntax (`function-length`, `line-length`, `loop-unbounded` and
`file-name`), and every package of the three modules, with its types,
for the others. The type checks do not yet reach the test files,
`testdata` or the conformance snippets (`plan.md`).

| Rule | Check | What it counts | Limit |
|---|---|---|---|
| S6 | `function-length` | the lines of a function, from `func` to its closing brace | 70 |
| L2 | `line-length` | the lines of a file wider than the limit, a tab counting four columns | 100 |
| S1 | `recursion` | the functions on a cycle of calls, a function passed to a call counting as called, and the functions holding a function literal that calls the variable it is assigned to | none |
| F2 | `argument-mutation` | the writes of a function through a parameter or receiver into memory its caller can see: an assignment through a pointer, slice or map, `append`, `copy`, `delete` or `clear`, and the sorting and editing functions of `sort`, `slices` and `maps` | none |
| F4 | `package-state` | the writes to a package-level variable outside `init`, including taking its address and calling a method that takes its address | none |
| S8 | `error-discarded` | the calls of a function whose error result is dropped, in a statement, a `defer` or a `go` | none |
| S2 | `loop-unbounded` | the loops of a function that are not bounded as S2 defines | none |
| N10 | `file-name` | the Go files not named in lowercase snake case | none |
| D1 | `dependency` | the requirements of a `go.mod` outside the table below | none |

| Module file | Requirements it may have |
|---|---|
| `go.mod` | `github.com/mattn/go-sqlite3` |
| `lsp/go.mod` | `github.com/Piechutowski/volt` |
| `cmd/volt/go.mod` | `github.com/Piechutowski/volt`, `github.com/Piechutowski/volt/lsp`, `github.com/mattn/go-sqlite3`, `golang.org/x/mod`, `golang.org/x/sync`, `golang.org/x/tools` |

**The ledger.** `cmd/volt/testdata/style.ledger` lists the debt older
than this file, one line per function, file, variable or requirement,
with its count or, for a long function, its length. The gate fails when
the code holds more than the ledger: a new violation, or more of an old
one. It also fails when the code holds less, until the ledger is
lowered with `go test ./cmd/volt -run TestStyleLedger -update`, which
never raises an entry. Renaming a function or a file moves its line by
hand in the same commit, where the diff shows it.

**What the gate cannot see**, and review covers: a write through a
method call on a parameter or receiver; recursion through an interface
or a stored function value; an effect behind an interface; and, until
the type checks reach them, the test files.

**The examples are executable.** `TestStyleDocumentsTheGate` runs every
example below through the gate: a bad example must fail its check and a
good one must pass it. The same test holds the two tables above to the
gate's limits and allowed requirements, so this section cannot drift
from the code.

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

const nodeCountMax = 1 << 20

func nodeCount(root Node) int {
	stack := []Node{root}
	count := 0
	for ; count < nodeCountMax && len(stack) > 0; count++ {
		node := stack[len(stack)-1]
		stack = append(stack[:len(stack)-1], node.Children...)
	}
	if len(stack) > 0 {
		panic("nodeCount: the tree has more than nodeCountMax nodes")
	}
	return count
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

```go good argument-mutation
func tagsAdded(tags []string, tag string) []string {
	added := make([]string, 0, len(tags)+1)
	added = append(added, tags...)
	return append(added, tag)
}
```

### `package-state`

```go bad package-state
var requestCount int

func requestCounted() {
	requestCount++
}
```

```go good package-state
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
func requestsServe(requests chan int, handle func(int)) {
	for request := range requests {
		handle(request)
	}
}
```

```go good loop-unbounded
func requestsServe(requests chan int, handle func(int)) {
	for {
		handle(<-requests)
	}
}
```
