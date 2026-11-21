# Volt style in Volt

> [!CAUTION]
> **This project is an alpha version, and everything may change in the future.**

`docs/VOLT_STYLE.md` is Volt style, written for any Go project. This
file is how the Volt repository applies it: what the style binds here,
how each rule meets Volt's code and the debt Volt still carries to it,
Volt's exceptions, and the gate that checks the mechanical rules. A
citation of this file is `style_in_volt/` followed by a heading's
anchor: style_in_volt/the-style-gate.

## Scope

The style covers the compiler, the tools, the runtime, the generators,
the tests and the code `volt gen` writes. The editor stack (`grammar/`,
`zed-extension/`) is written in the languages its editors dictate and
stays outside it (volt_style/one-toolbox).

`spec.md` says what Volt is; Volt style and this file say how its code
is written. All three are normative. The functional rules are the
spec's (spec/values-and-effects), applied to every package. Where this
file asks for more than the spec fixes today, it names the `plan.md`
step that changes the spec, and until that step lands the spec holds.

A rule the gate checks is listed in style_in_volt/the-style-gate with
the check that enforces it. Every other rule is checked in review. A
change that breaks either is not done.

## Dependencies

The guarantee, in exact terms:

- the root module, the library and the runtime that generated code
  links against, requires nothing beyond the Go toolchain; its tests
  use mattn;
- `lsp/` requires only the root module;
- `cmd/volt/` requires mattn, for the fold and `volt migration up`, and
  `golang.org/x/tools`, which `volt gen --verify` and the gates load
  packages with, with the two modules it needs (`x/mod`, `x/sync`);
- a generated application brings mattn itself, since the runtime
  registers no SQLite driver.

These requirements are held by the table of style_in_volt/the-style-gate.
Neither is forked yet (volt_style/zero-dependencies). `urfave/cli/v3`
leaves the tool for the standard library's `flag`
(plan/flags-from-the-standard-library).

## How Volt applies the rules

### Design first

The design is a spec section, written before anything big is built.

### Integrated, not glued

The language, its checker, the generators, the runtime and SQLite are
one system: `volt` is one binary for checking, generating, the language
server and migrations, and a fact is declared once in a `.volt` file
and derived everywhere else.

### Modules are black boxes

The interface of a module is architecture and the maintainer's to
decide; the implementation behind it is the agent's (`AGENT.md`, The
method). `assert`, `checked`, `tint` and `lang/source` are black boxes of a few
exported names each. `cmd/` holds the commands, and `internal/` the
modules only this repository uses, the agent's tools among them.

### Zero technical debt

The ledger of style_in_volt/the-style-gate holds the debt to the gated
rules, and plan/paying-off-the-style-debt carries the debt to the rest.

### Always say why

For now an agent's reasons go into the spec section, the `runner.md`
section or the `plan.md` step its change touches (`AGENT.md`,
Conventions), not into comments.

### The functional rules

These are the five rules of spec/values-and-effects, and they hold for
every package of the repository, not only for the generated code and
the runtime.

### Owned state only

The function values that may run later are the `next` of `rt.Then` and
the `f` of `rt.Map`; `packages.Visit` runs its visitor in place. The
gate's `argument-mutation` counts the writes through parameters and
receivers, and `package-write` the writes to package variables.

### Effects at the edge, as data

The edges are `main`; the runner `nao/run`; the network runtime of the
root package (the generated shim, the event broker and the client); the
loader behind `lang/source`; the final write of `volt gen`; the
transport loop of the language server; and the worker, when
plan/jobs-the-worker-periodic-effects builds it. `rt.Parallel`,
`rt.ParallelMap` and `internal/par` are the CPU parallelism, as
plan/decisions states. `cmd/volt/layers_test.go` holds the imports of
the layers. The effects outside these places today are
plan/effects-at-the-edge.

### Mutable state is inventoried

The connection pool, the statement cache, the event broker's log and
the logger are the whole list: the methods of `nao/run`'s `readers`,
`StmtCache` and `cacheEntry` and of the root package's `Broker` (the
gate's `styleInventory`), and those of `log/slog`'s logger, whose state
lives inside the standard library, with the writer of a `tint` handler,
which its `Handle` writes under the mutex it shares with the handlers
derived from it. A test that holds the import graph
to this list is plan/the-gates-reach-further. The gate also counts as
inventoried every type that embeds `guigui.DefaultWidget`, for the
examples of `assert.md`.

### Pure phases

The scanner, the parser, the checker, the fold and the generators are
those functions, and so is the formatter when `volt fmt` lands. The
purity gate (`cmd/volt/purity_test.go`) proves it for the memoized
computations; plan/the-gates-reach-further extends it to every phase.
It trusts a call into the `assert` package and nothing else of the
kind: `assert` reads the call stack only on the path of a failed
assertion, which panics and never returns a value a memo could keep,
while a memoized function that calls `runtime.Caller` itself is
refused, as the gate's fixture proves. The fold runs on SQLite
files today, which plan/effects-at-the-edge settles.

### Go idioms

The optional value is `rt.Null[T]`. The effect kinds live in `nao/rt`
and implement a sealed `Effect[T]`, so a type of another package can
only embed a kind or an `Effect` and is then the effect it embeds; the
runner performs one through `rt.Perform`, and each kind describes
itself for `rt.Describe` and `rt.Same`.

The gates do not see a write through a method call on a parameter or
receiver, through a local alias of one, through an address passed to a
call, or by a callee other than the builtins and standard library
functions style_in_volt/the-style-gate lists; nor reflection, `unsafe`
or cgo. Review does.

### Simple control flow

The runner raises a `Parallel` step's panic again, and the shim's exit
path of plan/the-assert-package stops the process. Today the parsers
panic on a syntax error and recover it, and `renderEncode` recovers any
panic into an error; plan/no-recursion and plan/the-assert-package
remove both. The gate's `recursion` counts cycles of calls, where
passing a function to a call counts as calling it and a function
literal that refers to the variable it is assigned to counts as
recursion; recursion through an interface or through a function value
stored elsewhere is checked in review.

### A limit on everything

The gate's `loop-unbounded` counts the loops that are not bounded; the
other limits are checked in review. The shape of an event loop gets a
check in plan/bounded-loops, and until then the gate counts it.

### Fixed-size integers at boundaries

Volt builds for 64-bit targets only, so `int`, `int64` and `uintptr`
are 64 bits wide and converting between `int` and `int64` loses
nothing. `nao/rt` and `lang/token` hold it at compile time with
`const _ uint = strconv.IntSize - 64`, so a 32-bit build stops there.
The schema's `int` is Go's `int64` in every generated signature, field
and runtime helper it reaches (spec/go-types). The front end keeps
positions in `int`, so no source size needs a limit for them to fit
(spec/source-text), and the language server saturates them at the
protocol's 32-bit largest value when it sends them to the editor; the
rest of plan/fixed-size-integers-at-the-boundaries is the arithmetic at
the boundaries.

### Arithmetic that cannot wrap

The gate's `conversion-unchecked` counts a conversion of a value that
is not constant to an integer type that cannot hold every value of its
source type; arithmetic is checked in review, since a check on every
operator would flag each loop counter and index.

### Assertions

Hand-written and generated code alike assert, once
plan/the-assert-package wires the package in. A surprising invariant
stated as an assertion is, for now, how an agent documents one
(`AGENT.md`, Conventions). No package of the repository imports
`unsafe` (`TestNoUnsafeImports`), so a size is asserted at compile time
through a constant of the standard library, such as `strconv.IntSize`,
not `unsafe.Sizeof`.

### Short functions

The gate's `function-length` counts the lines of each function.

### Every error is handled

The gate's `error-discarded` counts the errors dropped, and trusts only
the writes to a `strings.Builder` or a `bytes.Buffer`, documented never
to fail; a pure call written as a statement is checked in review.

### No warnings

Held by the verification bar; plan/the-gates-reach-further adds the
`nilness` and `unusedwrite` analyzers.

### Run at your own pace

The worker and the periodic loops are not built
(plan/jobs-the-worker-periodic-effects), and the broker today sends
each event as it is published, which is debt (plan/limits);
runner/batching-writes-not-built keeps the note on batching writes,
which is not built.

### Calls a reader can follow

The closed sets are the sealed effect kinds and the `next` of
`rt.Then`.

### Sketch before building

`runner.md` is where the sketches meet the measurements.

### Control plane and data plane

Inside one request the batches are `rt.Batch`, `CreateMany`, a
statement prepared once, and nested loads as separate batched
statements.

### Go casing, big-endian order

The handlers of the language server keep the names of the protocol
methods they serve.

### File names

The gate's `file-name` counts the Go files not in lowercase snake case.

### Comments say why

Comments are the maintainer's: an agent writes none, and puts its
reasons where style_in_volt/always-say-why says (`AGENT.md`,
Conventions).

### Commit messages inform

A commit message is `.`, and the reasons live in the docs the commit
changes (`AGENT.md`, Conventions).

### Short lines

The gate's `line-length` counts the lines past 120 columns. Generated Go
aims at 100 columns instead, under spec/layout-of-generated-code, which
`gen/layout` lays out and `TestGeneratedCodeFollowsTheRules` checks:
the generator breaks a line at the break points that section names,
and a line stays wider, past 120 too, only when none is left on it,
since Volt does not limit how long the schema's names are. The gate
counts the committed goldens and fixtures as well, whose names are
short enough to stay within 120.

### One toolbox

AGENT.md's stack says the same. The exceptions are the editor stack, in
the languages its editors dictate, and the external tools
plan/no-external-tools-in-the-performance-report removes.

### Parsers and decoders are fuzzed

The scanner, the parser, the checker and every decoder of what a
request carries have targets, and the formatter will when `volt fmt`
lands.

### Randomness is reproducible

The simulated executor sits behind `rt.Perform`, and the seed orders
the steps of a `Parallel`. It tests Volt's effects, not the threading
of mattn or SQLite, which Go's scheduler and cgo keep out of a seed's
reach.

### Round trips are checked

Generate, compile and run (the goldens and the fixtures); migrate and
compare with the declared schema (the fold); encode and decode the wire
formats; and parse, format and parse again, once `volt fmt` lands.

### Fast paths agree with plain ones

A cached statement and a fresh one, `CreateMany` and repeated `Create`,
batched nested loads and a join.

### SQLite

`runner.md` says how `nao/run` meets SQLite: the two pools
(runner/two-pools), which connection an effect runs on, the prepared
statements and the bulk insert.

## Examples compile

An example that does not compile teaches nothing. `TestStyleDoc`
compiles every Go block of `VOLT_STYLE.md` and of this file, checks it
with `gofmt` and runs it through the gate: an example marked bad must
fail its check, and every other must pass them all.

`TestAssertDoc` in `cmd/volt` treats every Go block of `assert.md` as a
file of one package, type-checks that package against the standard
library and the real `assert` and `checked` packages, formats every
block with `gofmt` and runs the gate over it, so an example that breaks
a gated rule fails the build, the same as Volt's own code. It counts
`pool` as the inventoried state of that package. The blocks marked
`go guigui` are a second package, checked against a stub of the part of
Guigui's API they use, copied from Guigui v0.1.0, because Guigui is not
a dependency of this repository.

## The style gate

`cmd/volt/style_test.go` checks the rules of the first table below and
runs with `go test ./cmd/volt/...`, inside the verification bar. It reads every
`.go`, `.go.golden`, `.volt`, `.sql` and `.sql.golden` file of the
repository outside `.git`, `grammar`, `node_modules` and
`zed-extension`. `line-length` runs on every file it reads
(volt_style/short-lines). The checks
that need only syntax run on every Go file and Go golden:
`function-length`, `file-name` and the `for` statements of
`loop-unbounded`. The checks that need types run on every package of
the three modules: `recursion`, `argument-mutation`, `package-write`,
`error-discarded`, `conversion-unchecked` and the `range` statements of
`loop-unbounded`. They
reach the test files with plan/the-gates-reach-further; `testdata` and
the conformance snippets, which are fixtures, stay outside them.

| Rule | Check | What it counts | Limit |
|---|---|---|---|
| volt_style/short-functions | `function-length` | the lines of a function or a package-level function literal, from `func` to its closing brace | 70 |
| volt_style/short-lines | `line-length` | the lines of a file wider than the limit, a tab counting four columns | 120 |
| volt_style/simple-control-flow | `recursion` | the functions on a cycle of calls, a function passed to a call counting as called, and the functions holding a function literal that refers to the variable it is assigned to | none |
| volt_style/owned-state-only | `argument-mutation` | the writes of a function, or of a function literal inside it, through a parameter or receiver into memory a caller can see: an assignment or range assignment through a pointer, slice, reslice or map; `append`, `copy`, `delete` and `clear`; and the editing functions of `sort`, `slices` and `maps` that the purity gate lists; inside the methods of an inventoried type, that type is exempt | none |
| volt_style/mutable-state-is-inventoried | `package-write` | the writes to a package-level variable outside `init`: assigning it or through it, taking its address, and calling a method that takes its address, except the `Load` methods of `sync/atomic` | none |
| volt_style/every-error-is-handled | `error-discarded` | the error results a function drops: a call written as a statement, in a `defer` or a `go`, and a blank for an error in a tuple assignment | none |
| volt_style/a-limit-on-everything | `loop-unbounded` | the loops of a function that are not bounded as volt_style/a-limit-on-everything defines | none |
| volt_style/arithmetic-that-cannot-wrap | `conversion-unchecked` | the conversions of a function from a value that is not constant to an integer type that cannot hold every value of the source's integer type, `int` counting as 64 bits, outside the package `checked` | none |
| volt_style/file-names | `file-name` | the Go files not named in lowercase snake case | none |
| volt_style/zero-dependencies | `dependency` | the requirements of a `go.mod` outside the table below | none |

| Module file | Requirements it may have |
|---|---|
| `go.mod` | `github.com/mattn/go-sqlite3` |
| `lsp/go.mod` | `github.com/Piechutowski/volt` |
| `cmd/volt/go.mod` | `github.com/Piechutowski/volt`, `github.com/Piechutowski/volt/lsp`, `github.com/mattn/go-sqlite3`, `golang.org/x/mod`, `golang.org/x/sync`, `golang.org/x/tools` |

The standard library iterators that volt_style/a-limit-on-everything counts
as finite are the `All`, `Keys` and `Values` of `maps`; the `All`,
`Backward` and `Values` of `slices`; and the `Lines`, `SplitSeq`,
`SplitAfterSeq`, `FieldsSeq` and `FieldsFuncSeq` of `strings` and of
`bytes`.

**The ledger.** `cmd/volt/testdata/style.ledger` lists the debt to the
gated rules that is older than the style, one line per function, file,
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
stored elsewhere; a pure call written as a statement; a conversion in a
package-level declaration, a conversion to or from a type parameter,
and a conversion from a floating-point type; arithmetic that wraps; and
every rule held in review.

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

### `conversion-unchecked`

```go bad conversion-unchecked
func offsetStored(offset int) int32 {
    return int32(offset)
}
```

```go good conversion-unchecked
func offsetStored(offset int32) int64 {
    return int64(offset)
}
```

```go good conversion-unchecked
import "github.com/Piechutowski/volt/checked"

func offsetStored(offset int) (int32, bool) {
    return checked.Convert[int32](offset)
}
```

```go bad conversion-unchecked
func lengthSigned(length uint64) int64 {
    return int64(length)
}
```
