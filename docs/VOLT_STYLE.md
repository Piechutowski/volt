# Volt style

> [!CAUTION]
> **This project is an alpha version, and everything may change in the future.**

> [!NOTE]
> Volt style is a derivative of TigerStyle, `docs/TIGER_STYLE.md` of
> [TigerBeetle](https://github.com/tigerbeetle/tigerbeetle/blob/a2a3c7064519e08233abe26c54a43fca2f9ad9bc/docs/TIGER_STYLE.md)
> at commit `a2a3c70`, by TigerBeetle, Inc. and its contributors,
> licensed under the Apache License 2.0, the license of this repository
> (`LICENSE`). Its text is copied here and changed: rewritten for Go,
> reordered under named headings, cut where it is about Zig or about
> TigerBeetle alone, and joined with the functional rules and the rules
> of Volt's own.

Volt style is a way of writing Go, for any project and any author, a
person or a language model: servers, command-line tools, desktop apps
on Guigui and Ebitengine, and Volt, the framework it was first written
for. It is three things joined. The first is
[TigerStyle](https://tigerstyle.dev), TigerBeetle's engineering style,
built on NASA's [Power of Ten](https://spinroot.com/gerard/pdf/P10.pdf)
rules, whose text this file carries. The second is the functional style,
which Richard Feldman reduces to avoiding mutation and avoiding side
effects (volt_style/the-functional-rules). The third is deep
integration, one program over concrete types in place of tools glued
together (volt_style/integrated-not-glued), built from modules that are
black boxes behind small interfaces (volt_style/modules-are-black-boxes),
with what has been learned along the way: the rules on names, the
chapter on technologies, and every rule TigerStyle does not have.
Together they are meticulous attention to detail from the first line:
the narrowest type, the assertion, no dependency, no allocation or copy
that is not needed, the algorithm the problem calls for, no mutation and
no side effect, and a program that runs at its own pace instead of
reacting to every event.

Correctness comes first, and the compiler is its first check: what the
type system can state, it states, and what it cannot, an assertion does
(volt_style/assertions). Two small packages serve the style, each one
file over the standard library, to import from
`github.com/Piechutowski/volt` or to copy: `assert`, whose use is
`assert.md`, and `checked` (volt_style/arithmetic-that-cannot-wrap).

**How to read it.** Each rule says what holds in any Go program. The
chapter volt_style/technologies says how the rules meet the libraries
used most: `net/http`, SQLite, Guigui and Ebitengine. A project that
follows the style keeps, in a document of its own, how it applies each
rule, its exceptions, the debt it still carries and the checks that
enforce the mechanical rules; Volt's is `style_in_volt.md`. Every rule
is a heading with a name, and a citation of one is the file's name in
lowercase, `volt_style/`, followed by the heading's anchor:
volt_style/owned-state-only, volt_style/short-functions.

## The essence of style

> “There are three things extremely hard: steel, a diamond, and to know
> one's self.”
>
> Benjamin Franklin

Volt style is evolving. A collective give-and-take at the intersection
of engineering and art. Numbers and human intuition. Reason and
experience. First principles and knowledge. Precision and poetry. This
is what has been learned along the way. The best is yet to come.

## Why have style

Another word for style is design.

> “The design is not just what it looks like and feels like. The design
> is how it works.”
>
> Steve Jobs

The design goals are safety, performance and developer experience, in
that order. All three are important. Good style advances these goals.
Does the code make for more or less safety, performance or developer
experience? That is why style is needed.

Put this way, style is more than readability, and readability is table
stakes, a means to an end rather than an end in itself.

> “...in programming, style is not something to pursue directly. Style
> is necessary only where understanding is missing.”
>
> [Let Over Lambda](https://letoverlambda.com/index.cl/guest/chap1.html)

## Values

Safety, performance and experience, in that order. All three matter;
when they pull apart, the earlier one wins. Simplicity is how they come
together, technical debt is what they cost when they do not, and design
is where both are decided.

### On simplicity and elegance

Simplicity is not a free pass. It is not in conflict with the design
goals. It need not be a concession or a compromise. Rather, simplicity
is how the design goals are brought together, how the "super idea" that
solves the axes at once is found, to achieve something elegant.

> “Simplicity and elegance are unpopular because they require hard work
> and discipline to achieve.”
>
> Edsger Dijkstra

Simplicity is not the first attempt but the hardest revision. It is
easy to say "let's do something simple", but to do that in practice
takes thought, multiple passes, many sketches, and still one may have to
["throw one away"](https://en.wikipedia.org/wiki/The_Mythical_Man-Month).
The hardest part, then, is how much thought goes into everything.

### Design first

The mental energy is spent up front, proactively rather than
reactively, because when the thinking is done, what is spent on the
design is dwarfed by the implementation and the testing, and then again
by the costs of operation and maintenance. Safety and performance are
won before the code exists: the design, the sketch of the four
resources (volt_style/sketch-before-building), the limits
(volt_style/a-limit-on-everything) and the assertions (volt_style/assertions)
come first. The problem is understood from its core before an
algorithm is chosen, so the algorithm is the one the problem calls
for. An hour or a day of design is worth weeks or months in
production. Go slow to go fast: the cost that counts is the total cost
of owning the software, paid by the many who read and run it rather
than the one who writes it once, so quality that compounds wins over a
deadline kept.

> “The simple and elegant systems tend to be easier and faster to design
> and get right, more efficient in execution, and much more reliable.”
>
> Edsger Dijkstra

### Closed by default

Every set starts from the smallest that does the job and widens only
for a reason given: a type, a permission, the surface of an API, the
tools a program may run, the words of a rule. An allow list states what
is; a deny list leaves open everything it forgot. This is
volt_style/a-limit-on-everything and the positive space of
volt_style/assertions, carried from code to design.

### Integrated, not glued

A problem is solved by one program that owns it from end to end, over
concrete types and the libraries made for that problem: a parser for a
language, a database for relational data, never a pattern over text.
Every seam between tools passes text that the next one must parse
again, and the combinations of their flags, formats and encodings
multiply with each tool added, until the system is mostly glue that no
one can trust. Inside one program the compiler checks that the parts
fit, and the style makes them right from the first line. A small
interface is a good one: each part shows the least it can, and the
parts are few. Code that is more than trivial use of the standard
library, and that will be needed again, becomes a module rather than
being written a second time (volt_style/modules-are-black-boxes).

### Modules are black boxes

Software is built from modules, and each module is a black box. What it
does is offered through an interface, an API or a protocol, that is
documented and clear; how it does it stays hidden. A user of the module
talks to the interface and to nothing else: they never read the code
behind it, never depend on what it does inside, and never reach around
it. Behind the interface the module is rock solid, so it just works.

A module is small enough for one person to write, and one person
implements it, though many may design its interface. An application is
written by composing modules, most of them written by others, so one
person can build all of a large system one module at a time. The hard
modules go to the most experienced hands and the easy ones to anyone.

The interface is the decision that lasts, so it is designed first and
kept as small as it can be. The implementation is the part that can be
thrown away: a module that breaks, ages or loses its author is
rewritten from scratch behind the same interface, with other
algorithms and other data, and everything that uses it keeps working
while the users move to the new one at their own pace. After Eskil
Steenberg, on architecting large software.

In Go a module is a package. Its exported identifiers, the types,
fields, functions and methods that start with a capital letter, are its
interface, what a header file is in C, and the compiler keeps every
other package away from the rest.

### Zero technical debt

What could go wrong? What is wrong? Which question would one rather
ask? The former, because code, like steel, is less expensive to change
while it is hot. A problem solved in production is many times more
expensive than a problem solved in implementation, or a problem solved
in design.

Since it is hard enough to discover showstoppers, when one is found, it
is solved. No latency spike from a large copy, no algorithm of
exponential cost slips through.

> “You shall not pass!”
>
> Gandalf

It is done right the first time, because the second time may not
transpire, and because doing good work that one can be proud of builds
momentum. What ships is solid. It may lack crucial features, but what
it has meets the design goals. This is the only way to make steady
incremental progress, knowing that the progress made is indeed
progress. Debt older than the style is inventoried and only shrinks.

### Always say why

Never forget to say why. A rationale given increases the reader's
understanding and makes them more likely to follow the decision, and it
shares the criteria by which to judge the decision and its importance.
A reason given is a reason that can be checked.

## The functional rules

Richard Feldman reduces functional programming to two habits: avoid
mutation, and avoid side effects. Volt style takes both as five rules:
data structures are values; nothing is mutated after its construction
except inventoried state; effects happen at the edge, as data; the
mutable state is inventoried; and a program that works in stages
writes each stage as a pure function. Code written this way can be
read one function at a time, tested without a world around it, and run
in parallel without locks.

### Data structures are values

A method on a value returns a new value and leaves its receiver as it
was.

### Owned state only

A function mutates only what it created and nobody else can see: a
local slice, map or struct that has not escaped. It never writes
through an argument or a receiver, nor package state. Leaf functions
are pure: the parent function keeps the relevant state in its locals,
a helper computes what should change and returns it, and the parent,
which owns the state, applies it. A function literal may write the
locals of the function that creates it only when it runs synchronously
inside that function, as the visitor of `ast.Inspect` or
`filepath.WalkDir` does; a function value that may run later or
concurrently (a callback stored for later, a goroutine) writes nothing
it captures. An `init` function builds package state, which is
construction; nothing writes that state after. The one exception is
inventoried state (volt_style/mutable-state-is-inventoried), whose own
methods may write it: a framework that owns the loop and calls back
into your types, as Guigui does with its widgets, makes those types
inventoried state.

### Effects at the edge, as data

Files, the network, the database, the clock, randomness, the
environment and logging happen in one place per lifetime: `main`, the
handler shell of a server, the loop of an app, the runner that performs
effects described as values. Goroutines that compute without I/O and
join before they return are CPU parallelism inside pure code, not
effects. In a test, the edge is the test function and the helpers it
calls to read and write; what they read is handed to pure functions.
Everything else computes values, descriptions of effects included.

### Mutable state is inventoried

Every piece of state that changes after its construction is on a
written list, with its owner: a connection pool, a cache, a broker's
log, a logger, the widgets of a Guigui app. Their own methods may
mutate them. Nothing else may.

### Pure phases

A program that works in stages (scan, parse, check, transform,
generate, lay out) writes each stage as a function from values to
values, so each one can be tested, memoized and run in parallel on its
own.

### Go idioms

One per need, so the code stays uniform:

| Need | Idiom |
|---|---|
| Sum types | A sealed interface: one method whose name starts with a lowercase letter, one small struct per kind in the same package, constructors that return the interface, a type switch in the single consumer. A test compares the list of kinds with the switch, since Go does not check exhaustiveness. |
| Immutable records | Structs with unexported fields, exported constructors, `With*` methods on value receivers. |
| Copy on write | `slices.Clone` before appending to a slice that came from outside; never `append` to an argument. |
| Optional values | A small generic struct of a value and a validity flag; never a pointer in a public API. |
| Effects | A small set of effect kinds, each implementing a sealed interface whose methods are unexported, performed by one runner; each kind describes itself, and a test holds the kinds to their descriptions. |

## Safety

> “The rules act like the seat-belt in your car: initially they are
> perhaps a little uncomfortable, but after a while their use becomes
> second-nature and not using them becomes unimaginable.”
>
> Gerard J. Holzmann

[NASA's Power of Ten: Rules for Developing Safety Critical
Code](https://spinroot.com/gerard/pdf/P10.pdf) will change the way you
code forever. The rules below expand it.

### Simple control flow

Only very simple, explicit control flow, for clarity. No recursion, so
that every execution that should be bounded is bounded: it becomes a
loop over an explicit stack, with a depth limit that is reported when
input crosses it and asserted where the program guarantees it
(volt_style/a-limit-on-everything, volt_style/assertions). No `goto`; a labeled
`break` or `continue` only leaves nested loops. A panic is an
assertion failure and nothing else: an operating error is returned,
never panicked. Nothing recovers a panic except a test that checks an
assertion trips, and a runner that must carry a panic from one
goroutine to another, which raises it again (assert/tests).

Only a minimum of excellent abstractions, and only where they make the
best sense of the domain. Abstractions are
[never zero cost](https://isaacfreund.com/blog/2022-05/), and every one
brings the risk of a leaky abstraction.

### A limit on everything

Put a limit on everything, because in reality this is what to expect:
everything has a limit. Every loop, queue, channel, buffer, cache, map
that grows with input or time, input size, nesting depth, retry and
timeout has a fixed upper bound, so that there are no infinite loops
and no tail latency spikes. This follows the
["fail-fast"](https://en.wikipedia.org/wiki/Fail-fast) principle, so
that violations are detected sooner rather than later. Crossing a bound
is a reported error when input crossed it and an assertion when the
program's own state did. A loop is bounded when:

- it is counted: its post statement steps an identifier with `++`,
  `--`, `+=` or `-=`, its condition, or one of the condition's `&&`
  terms, compares that identifier with `<`, `<=`, `>` or `>=` against a
  bound in the direction it moves, and the body writes neither the
  identifier nor the bound;
- it ranges over a finite value: a slice, array, string, map or
  integer, or a standard library iterator over one (the `All`, `Keys`
  and `Values` of `maps`; the `All`, `Backward` and `Values` of
  `slices`; the `Lines`, `SplitSeq`, `SplitAfterSeq`, `FieldsSeq` and
  `FieldsFuncSeq` of `strings` and of `bytes`); a range over a channel
  or over any other function is not bounded;
- or it provably never ends: a `for` without a condition that no
  `return`, `goto`, labeled branch to outside it, or unlabeled `break`
  of its own leaves. Such a loop lives as long as the process and
  asserts what keeps it alive, a channel that must stay open for one.

A loop that serves a connection or a process and ends with its input or
its context, an event loop, does bounded work per iteration.

### Bounded allocation

Memory is planned in the design, not discovered in production: nothing
grows without a limit, what can be allocated at startup is, and the hot
paths (per request, per row, per token, per frame) preallocate and
reuse instead of allocating per item. This avoids unpredictable pauses
and latency, and a design that has considered every way memory is used
up front is in practice simpler, faster and easier to reason about than
one that has not. Reuse happens inside one owning function, across the
rows or tokens of a single call; a buffer kept from one call to the
next is inventoried state (volt_style/mutable-state-is-inventoried).

### Fixed-size integers at boundaries

What is stored, sent over the wire or declared in a schema has a fixed
size (`int32`, `int64`, `uint32`); `int` is for in-memory lengths and
indexes, as Go's `len` returns it. The type is the first limit: a value
gets the narrowest type that holds its declared range, so a kind of
three states is a `uint8`, and the type states its range where an
assertion would otherwise have to. Signed is for what can go below
zero, unsigned for counts, sizes and kinds; a value that can be
negative is never unsigned, since a negative result would wrap to a
large positive one that passes every check after it.

### Arithmetic that cannot wrap

Go wraps on overflow, modulo two to the power of the type's width, and
truncates a narrowing conversion, without a sound; only constant
expressions are checked, by the compiler. A wrapped value looks valid:
it passes the very comparison that should have caught it, and only an
index into a slice ever traps it. So arithmetic that can overflow, and
a conversion to a narrower or differently signed type, is checked
before or with the operation, never after:

- first it is rearranged so that it cannot wrap;
- where it cannot be, it goes through the package
  `github.com/Piechutowski/volt/checked`: `Add`, `Subtract`,
  `Multiply` and `Convert` return the result and whether it is exact,
  as `(T, bool)`;
- a false is a reported error when input made it and an assertion when
  the program's own values did (assert/checked-arithmetic).

This check passes a huge `amount`, because the sum wraps below the
limit:

```go
type creditLine struct {
    debits uint64
    limit  uint64
}

func (line creditLine) debitAllowed(amount uint64) bool {
    return line.debits+amount <= line.limit
}
```

The same check, rearranged, cannot wrap once the line's own invariant
is asserted:

```go
import "github.com/Piechutowski/volt/assert"

type creditLine struct {
    debits uint64
    limit  uint64
}

func (line creditLine) debitAllowed(amount uint64) bool {
    assert.That(line.debits <= line.limit, "debits must not exceed the limit")
    return amount <= line.limit-line.debits
}
```

### Assertions

Assertions detect programmer errors. Unlike operating errors (bad
input, a full disk, a refused connection), which are expected and must
be handled, assertion failures are unexpected, and the only correct way
to handle corrupt code is to crash. Assertions downgrade catastrophic
correctness bugs into liveness bugs, and they are a force multiplier for
finding bugs by fuzzing. A function must not operate blindly on data it
has not checked: the purpose of a function is to increase the
probability that a program is correct, and its assertions are part of
how it serves that purpose. Assertions:

- use the package `github.com/Piechutowski/volt/assert`, which panics,
  stays on in production and depends on the standard library only;
  `assert.md` is how to use it;
- average at least two per function, over arguments, results,
  preconditions, postconditions and invariants;
- have no side effect: an assertion reads, calls nothing that writes,
  and the program means the same without it;
- are never padding: an assertion a tool can prove always true or
  always false does not count;
- check the positive space, what is expected, and the negative space,
  what is not, because where data crosses the boundary between the
  valid and the invalid is where the interesting bugs are found;
- come in pairs: for every property to enforce, at least two code paths
  where it can be asserted, such as the data right before it is written
  and right after it is read back, the call site and the definition, or
  the generator and the parser of its output
  ([It Takes Two to Contract](https://tigerbeetle.com/blog/2023-12-27-it-takes-two-to-contract));
- are split, two assertions rather than one over `a && b`, since the
  first is simpler to read and says more precisely what failed, and an
  implication is `if a { assert(b) }`;
- check relationships between constants, and type sizes through
  `unsafe.Sizeof`, at compile time, with a constant expression that
  fails to compile when the relationship breaks, which checks the
  design's integrity before the program even runs;
- may state a blatantly true condition instead of a comment, as
  stronger documentation, where the condition is critical and
  surprising;
- are a safety net, not a substitute for understanding. A fuzzer can
  prove only the presence of bugs, not their absence, so: build a
  precise model of the code first, encode it as assertions, write the
  code to explain and justify the model to its reviewer, and use
  fuzzing and simulation as the last line of defense, to find the bugs
  in the author's and the reviewer's understanding.

### Short functions

There is a sharp discontinuity between a function that fits on a screen
and one that needs scrolling to see how long it is, so a function has at
most 70 lines from `func` to the closing brace. Art is born of
constraints. There are many ways to cut a wall of code into pieces of
70 lines, but only a few splits feel right:

- A good function shape is often the inverse of an hourglass: a few
  parameters, a simple result, and a lot of meaty logic between the
  braces.
- Control flow is centralized: when a large function is split, its
  `switch` and `if` statements stay in the parent, and the fragments
  without branches move to helpers. One function handles the control
  flow; the rest do not care about it
  (["push ifs up and fors down"](https://matklad.github.io/2023/11/15/push-ifs-up-and-fors-down.html)).
- State changes are centralized the same way (volt_style/owned-state-only).

### Smallest scope

A variable is declared at the smallest possible scope, and few
variables are in scope at once, so that the wrong one is less likely to
be used. A variable is computed or checked close to where and when it
is used, not introduced before it is needed and not left around after
its purpose ends. That closes the gap between the place of check and the
place of use, a distant cousin of the
[time of check to time of use](https://en.wikipedia.org/wiki/Time-of-check_to_time-of-use):
most bugs come down to a semantic gap in time or in space, because code
that is not contained along those dimensions is harder to check.

### Every error is handled

No error result is dropped: not by a call written as a statement, in a
`defer` or a `go`, nor by a blank in a tuple. Discarding one is written
`_ = f()`, and only where failure cannot change the outcome. The result
of a call to a pure function is used, since as a statement it does
nothing. An
[analysis of production failures in distributed data-intensive systems](https://www.usenix.org/system/files/conference/osdi14/osdi14-paper-yuan.pdf)
found that most catastrophic failures could have been prevented by
simple testing of error handling code:

> “Specifically, we found that almost all (92%) of the catastrophic
> system failures are the result of incorrect handling of non-fatal
> errors explicitly signaled in software.”

### No warnings

Every warning is appreciated from day one, at the strictest setting:
`go vet` and the project's own checks report nothing. When an analyzer
is confused, the code is rewritten until it is plainly right, without
changing what it does, and the analyzer is not silenced.

### Run at your own pace

A program that deals with the outside world does not do things directly
in reaction to external events; it runs at its own pace. That keeps its
control flow under its own control, which is safer; it batches instead
of switching context on every event, which is faster; and it makes the
work done per period easy to bound. Background work (a worker, a
broker, a periodic loop) wakes on an interval, takes a bounded batch
and finishes it. Each request is served on its own, in parallel as
`net/http` delivers them, and is never batched with another.

### Control flow a reader can check

A compound condition over several booleans makes it hard for the reader
to verify that every case is handled, so it becomes nested branches; a
long `else if` chain becomes nested blocks or a `switch` whose cases
exclude each other. An `if` without an `else` is a decision that the
negative space needs nothing, made on purpose. Negations are not easy,
so an invariant is stated positively. This form is easy to get right
and to understand:

```go
func itemAt(items []string, index int) (string, bool) {
    if index < len(items) {
        return items[index], true
    } else {
        return "", false
    }
}
```

This one is harder, and goes against the grain of how `index` is
compared with a count in a loop condition:

```go
func itemAt(items []string, index int) (string, bool) {
    if index >= len(items) {
        return "", false
    }
    return items[index], true
}
```

### Explicit options

A call passes the options that matter instead of trusting a library
default, for readability but most of all to avoid a latent and possibly
catastrophic bug when the library changes its default: the timeouts and
header limits of `http.Server`, `http.MaxBytesReader`, pool sizes, file
modes, deadlines on contexts, and the SQLite pragmas, `synchronous`
included.

### Run to completion

A function does not wait (on a channel, a lock or I/O) between an
assertion and the code that relies on it, so its preconditions stay
true for its whole run; across a wait they would only mislead. After a
wait it asserts again, or holds the lock that keeps the assertion true.
A pure function never waits.

### Few configurations

Build constraints are few, each is justified, and every configuration
they select runs in the tests. No `//go:linkname`.

### Calls a reader can follow

A function value or an interface method is called only where its set of
callees is closed and known: the kinds of a sealed interface, a
callback passed down and called in place, the methods a framework
calls. No pointer to a pointer.

## Performance

> “The lack of back-of-the-envelope performance sketches is the root of
> all evil.”
>
> Rivacindela Hudsoni

### Sketch before building

Performance is thought about from the outset. The best time to solve
it, to get the thousandfold wins, is in the design, which is precisely
when there is nothing to measure or profile. A system is also harder to
fix after implementation and profiling, and the gains are smaller. So
it takes mechanical sympathy: like a carpenter, work with the grain. A
back-of-the-envelope sketch of the four resources (network, storage,
memory, compute), each by its two main characteristics, bandwidth and
latency, is cheap, and it is roughly right: it lands close to the best
design.

### The slowest resource first

Network, then storage, then memory, then compute, each weighted by how
often it is used, because a faster resource may be used many times
more: a cache miss repeated a million times can cost more than one
fsync.

### Control plane and data plane

A clear line between the control plane and the data plane, drawn by
batching, allows a high density of assertions without losing
performance: the control plane decides and asserts once per batch, and
the data plane runs through the batch without branching. Batching
amortizes the network, storage, memory and compute. Let the CPU be a
sprinter doing the hundred metres: predictable, with large enough
chunks of work, not forced to zigzag and change lanes. Batching happens
inside one request and inside one own-pace loop, never across requests
(volt_style/run-at-your-own-pace).

### Explicit over clever

Depend on the compiler as little as possible to do the right thing. A
hot loop goes into a function of its own that takes primitive
arguments, not a struct or a receiver, so that the compiler does not
have to prove it can keep the struct's fields in registers, and a human
reader spots a redundant computation more easily.

### No copies in the data plane

No copying, encoding or decoding that the data plane does not need. A
buffer reused for speed is owned (volt_style/bounded-allocation) and cleared
to the length it is used at (volt_style/no-buffer-bleeds).

## Experience

> “There are only two hard things in Computer Science: cache
> invalidation, naming things, and off-by-one errors.”
>
> Phil Karlton

### Names

#### Nouns and verbs just right

Great names are the essence of great code: they capture what a thing
is or does and give a crisp, intuitive model of it. They show that the
domain is understood. Time goes into finding the right name, nouns and
verbs that work together, so that the whole is greater than the sum of
its parts.

#### Go casing, big-endian order

MixedCaps as Go requires. Names are subject first and verb last
(`UserGet`, `UserChangeset`, `OrderStateKnown`), internals too, and the
rest of the words follow the same order: the most significant first,
the qualifiers and units last, so related names sort and line up
together: `queueLengthMax` and `queueLengthMin`, not `maxQueueLength`.
The exceptions are the methods an interface mandates (Go's, or a
framework's such as Guigui's `Build` and `Layout`), `New*`, `is*` and
`has*`, APIs that parallel the standard library's, and cursor
primitives.

#### No abbreviations

Except Go's idioms: `ctx`, `err` and `ok`; `i`, `j` and `k` for
indexes; `w` and `r` in a handler; `t`, `b` and `f` for the values of a
test, a benchmark and a fuzz target. A receiver is named with a word,
as any variable is.

#### One case per acronym

`ID`, `URL`, `SQL`, as Go has them.

#### Units in the name, or in the type

In memory a duration is a `time.Duration` and its name carries no unit
(`timeoutMax`); a size or a count names its unit (`bodyBytesMax`,
`rowCount`). On the wire and in storage, where
volt_style/fixed-size-integers-at-boundaries makes a value a fixed-size
integer, the unit goes into the name (`timeoutMs int64`).

#### Index, count and size

The usual suspects for off-by-one errors are casual interactions
between an index, a count and a size. They are all integers, but they
are distinct types with clear rules between them: an index is
zero-based and a count one-based, so an index becomes a count by adding
one, and a size is a count times a unit, which is why names carry their
units and qualifiers.

#### Names carry meaning

`pool *sql.DB` is a good, if boring, name; `writer *sql.DB` and
`readers *sql.DB` are excellent, because they tell the reader which
statements each may run.

#### Equal lengths for related names

Where it is easy: `source` and `target` line up, `src` and `dest` do
not, and so `sourceOffset` and `targetOffset` line up in every
calculation and slice after them. The code becomes symmetrical, in
clean blocks that are easier for the eye to parse and for the reader
to check.

#### A helper is named after its caller

Subject first, so the name shows the call history: `sectorRead` calls
`sectorReadDone`.

#### Arguments in order

A `context.Context` comes first, then the singletons such as a logger,
from the most general to the most specific, and callbacks last, since
they are also invoked last.

#### One meaning per name

A name does not carry two meanings that depend on context. A name is
also thought of as it is used outside the code, in documentation and
conversation: a noun is often better than an adjective or a participle,
because a noun can head a sentence or a section without being
rephrased, and composes into derived names: `pipeline` and
`pipelineMax`, not `preparing`.

#### No swappable arguments

Two parameters of one type that a caller could exchange go into an
options struct. The exception is the operands of an arithmetic function,
which keep the order of the operator they stand for:
`checked.Subtract(minuend, subtrahend)` is `minuend - subtrahend`. A
parameter that can be nil is named so that a `nil` at the call site
reads clearly.

#### File names

Lowercase snake case.

#### What matters comes first

A file is read top-down on the first read, so the important things go
near the top: the entry point first, then each type with its
constructors and its methods, then the types it uses. A complex nested
type becomes a top-level one. Where no order is right, big-endian names
sorted alphabetically.

#### Long flags

A command's flags have long names (`--force`, not `-f`); a one-letter
flag is for interactive use only.

#### A test says what it proves

Its name and the names of its subtests state the goal and the method,
so a reader can get up to speed or skip to what they need.

#### Comments say why

Code alone is not documentation. A comment says why the code is the way
it is and shows the workings. Comments are sentences, with a space
after the slashes, a capital letter and a full stop, or a colon when
they lead into what follows; a comment at the end of a line can be a
phrase.

#### Commit messages inform

A commit message informs and delights its reader, because it is read,
and a pull request description is not stored in the repository nor
seen by `git blame`.

#### Effective Go for the rest

For everything else, Effective Go and Go Code Review Comments.

### Shape

#### Fewer dimensions

A simpler signature removes branches from every caller, and that
dimensionality is viral, propagating up the call chain: no result
beats `bool`, `bool` beats an integer, an integer beats `(T, bool)`,
and `(T, bool)` beats `(T, error)`.

#### Values, not aliases

State is neither duplicated nor aliased, so it cannot fall out of sync.
A large argument may be passed by pointer, since
volt_style/owned-state-only makes every pointer parameter read-only,
and a value is built by a constructor that returns it, never through an
out pointer, so no one sees it half built.

#### Division says how it rounds

Go's `/` truncates toward zero, which for a negative operand is neither
floor nor ceiling, so a division goes through a function that names
its rounding (exact, floor or ceiling) and shows that every case where
it matters was thought through. Such a function also reports what Go's
`/` would panic on or wrap: a zero divisor, and the smallest signed
value divided by minus one.

#### No buffer bleeds

A [buffer bleed](https://en.wikipedia.org/wiki/Heartbleed) is a buffer
underflow, the opposite of an overflow: a buffer not fully used, its
padding not zeroed. It can leak sensitive data and break deterministic
output, so a reused buffer is cleared to the length it is used at, and
no stale byte is read or sent.

#### Acquire and release together

A resource is acquired and its `defer` release written next to each
other, with a blank line before and after them, so a leak shows.

### Numbers

#### `gofmt` with tabs

`gofmt`, with tabs; its output is not argued with.

#### Short lines

No line of Go or SQL that a person writes is wider than 120 columns,
counting a tab as four, so two files fit side by side and nothing hides
behind a horizontal scrollbar; an `.editorconfig` sets four-column tabs
for GitHub and editors, and a column ruler in the editor helps. To wrap
a signature, a call or a composite literal, break after an opening
parenthesis or brace, end each line with a comma and let `gofmt` do the
rest.

### Dependencies and tools

#### Zero dependencies

Beyond the Go toolchain, nothing is a dependency. Each one brings risk
to the supply chain, to safety and to performance, and its cost
multiplies through everything built on it. Code needed from elsewhere
is forked and trimmed to the one use it serves, and from then on it is
owned. A fork is not updated for the sake of it: an update needs the
commit that fixes a problem the users have, and the whole change,
through every transitive dependency, is read before it lands. A latent
bug can be tracked and watched; an update is the bigger risk. After
Mitchell Hashimoto. A project lists what each of its modules may
require, and a test holds every `go.mod` to the list.

#### One toolbox

Tools have costs too. A small standard toolbox is simpler to operate
than an array of specialized instruments, each with its own manual.
The primary tool is Go. It may not be the best for everything, but it
is good enough for most things.

> “The right tool for the job is often the tool you are already
> using; adding new tools has a higher cost than many people
> appreciate.”
>
> John Carmack

So tools, tests and scripts are Go, and the only program a test or a
tool runs is `go` or a binary it built: a script in Go is portable and
typed, and it runs the same for everyone instead of failing on one
shell or one system. The exceptions are what another tool dictates,
such as an editor's grammar and extension.

## Verification

The assertions are where verification starts; the tests make them
run.

### A bug is fixed with the test that shows it

It is not fixed until a test fails without the fix.

### The negative space is tested

Tests test exhaustively: invalid input, input at and past every limit
(volt_style/a-limit-on-everything), and valid data becoming invalid, not
only the inputs that work.

### Parsers and decoders are fuzzed

Every parser and every decoder of what comes from outside has a native
Go fuzz target. `go test` replays its corpus on every run; each target
runs under `go test -fuzz` for a fixed time before a release and after
a change to its package. A crasher becomes a seed and stays.

### Failures are injected

At each driver call of a write and of a migration (`Exec`, `Query`,
`Next`, `Commit`), stepped one call at a time until the operation
completes, a failure is injected once and then persistently, and again
during the recovery from an earlier one; a full database comes from
`PRAGMA max_page_count`, and a crash from a child process killed in the
middle of a write. Every run ends with `PRAGMA integrity_check`, and
the data is as it was before or after, never in between. The I/O inside
SQLite's own commit is left to SQLite's tests, since mattn exposes no
VFS.

### Randomness is reproducible

A randomized test takes a seed and prints it. Concurrency that the
program schedules itself is tested by deterministic simulation, after
TigerBeetle's VOPR: a simulated executor and a seeded order for the
steps it runs.

### Coverage does not fall

Statement coverage is measured per package and does not fall. Go has
no branch coverage, so a boundary condition carries a test-only record
of both of its outcomes, after SQLite's `testcase()`, and the suite
fails when one side never ran.

### Assertions multiply the tests

Every test, fuzz case and simulation also checks every assertion it
passes through (volt_style/assertions).

### Round trips are checked

Encode and decode, generate and parse, write and read back: each pair
is run as a pair, and the result matches the input.

### Fast paths agree with plain ones

A fast path and its plain path give the same answer on the same input,
and a test runs both.

### No races and no leaks

Every package that starts goroutines or holds inventoried state runs
under `-race`, and its tests check that no goroutine, connection or
statement outlives them.

### Examples compile

A Go example in a document is code, and it is checked like code: it
compiles, `gofmt` leaves it as it is, and the project's own checks
pass it, so a document cannot teach what the style forbids.

## Technologies

How the rules meet the libraries Volt style is used with most. A
technology gets a section here when a project starts using it.

### HTTP servers

- The handler is the edge (volt_style/effects-at-the-edge-as-data): it
  decodes the request, calls pure code, performs the effects that code
  describes and encodes the response.
- The `http.Server` sets `ReadHeaderTimeout`, `ReadTimeout`,
  `WriteTimeout`, `IdleTimeout` and `MaxHeaderBytes`, and every request
  body goes through `http.MaxBytesReader` (volt_style/explicit-options).
- The router is wrapped once with `assert.Crash`, since `net/http`
  recovers a handler's panic and keeps serving (assert/servers).
- Cross-origin requests are refused by `http.CrossOriginProtection`,
  not by tokens.
- A goroutine started by a handler ends before the handler returns,
  and a panic in it ends the process, with or without `Crash`.

### SQLite

- The driver is `github.com/mattn/go-sqlite3` behind `database/sql`,
  built with cgo; a C compiler on the path is part of the toolchain.
- A test opens a file in its temporary directory, never `:memory:`,
  which gives each pooled connection a database of its own.
- The pragmas are set on every connection, explicitly:
  `journal_mode`, `synchronous`, `foreign_keys` and `busy_timeout`.
- The database runs in WAL mode, where readers and the writer do not
  block each other. Writes go through one connection, so they queue in
  Go and never meet `database is locked`, and reads go through a bounded
  pool of `query_only` connections.
- A statement run per request or per row is prepared once and reused.
- `SQLITE_BUSY` and a constraint that input violated are operating
  errors; a constraint that the program's own data violated is a bug,
  and the code asserts it before the write.

### Guigui and Ebitengine

Guigui builds desktop apps from widgets on top of Ebitengine, which
owns the loop: it calls a widget's methods, and the widget sets fields
and registers handlers. The rules hold for everything a widget calls;
how they meet the widget itself is below. Assertions in widgets are
assert/guigui.

#### A widget is inventoried state

A widget is a struct that embeds `guigui.DefaultWidget` and that Guigui
calls back, so its methods write its own fields, and nothing else
writes them (volt_style/mutable-state-is-inventoried). The model the widgets
show is a value with its own invariants, owned by one widget, usually
the root, and replaced whole by a handler that computes the next one
(volt_style/data-structures-are-values).

#### A widget works from its zero value

Children are plain fields, referred to as `&widget.child`, never
allocated in a constructor, so `var widget Widget` is ready to use. A
variable number of children lives in a `guigui.WidgetSlice`, since
`append` to a slice of widgets moves them. A widget's own setters
follow Volt names (`PageSet`); Guigui's keep theirs.

#### Decisions are plain functions

What the app decides (which page shows, what a list holds, whether a
button is enabled) is a function of the model, or a plain method of the
widget, and its tests run without Guigui (assert/testing-widget-decisions).

#### Build reads, handlers write

`Build` adds the children it uses and then configures them from the
current state; it runs on every rebuild, so it is idempotent and
changes nothing outside the widget. A handler registered in `Build`
(an `On*` callback, an input method) is where state changes, and
Guigui rebuilds after it runs. A widget gets its parent only when a
`Build` adds it, so the model is read through the widget whose `Build`
is running, never through a child created since the last build.

#### Layout only places

`Layout` positions the children `Build` added, with the bounds known;
`Build` never reads bounds and `Layout` never configures content. A
child that `Build` did not add still takes the bounds `LayoutWidget`
gives it, with no error, so `Layout` asserts it lays out what was built
(assert/build-and-layout-agree).

#### State changed outside a handler goes in the state key

A field written in `Tick`, or from a result that arrives later, causes
no rebuild on its own. The field goes into `WriteStateKey`, or a cheap
generation counter does for a large model; `guigui.RequestRedraw` is
for a change that only repaints; `guigui.RequestRebuild` is for the
change the key cannot observe.

#### Env carries the model, fields carry the rest

An `EnvKey` is generated once, in a package variable's declaration. The
ancestor that owns the model returns it from `Env` after checking its
invariants; a descendant treats a missing value as normal and a value
of the wrong type as a bug. A parent hands its direct children their
data through their setters.

#### Tick steps, Draw paints

Ticks and frames are not one to one, so per-step logic goes in `Tick`
and `Draw` only paints, changing nothing. In an Ebitengine game without
Guigui the same holds for `Update` and `Draw`, and an operating error
returned from `Update` ends `RunGame` with that error. `Layout` and
`Draw` run often: they reuse the slices they need and keep their
assertion messages constant (volt_style/bounded-allocation).

#### A panic ends the app

On desktop neither Guigui nor Ebitengine recovers a panic, so a failed
assertion ends the app with its message, and nothing like
`assert.Crash` is needed. On mobile, Ebitengine recovers a panic in its
loop into an error that carries the message and the stack as text, and
the app treats every error from its run as the end.

## The last stage

At the end of the day, keep trying things out, have fun, and remember
that small is a feature: the less code there is, the less of it can be
wrong.

> “You don't really suppose, do you, that all your adventures and
> escapes were managed by mere luck, just for your sole benefit? You are
> a very fine person, Mr. Baggins, and I am very fond of you; but you
> are only quite a little fellow in a wide world after all!”
>
> “Thank goodness!” said Bilbo laughing, and handed him the
> tobacco-jar.
