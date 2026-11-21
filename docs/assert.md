# Asserting in Go

> [!CAUTION]
> **This project is an alpha version, and everything may change in the future.**

I assert because I want to know the moment my code stops doing what I think it does. Not later, in a log nobody reads. Not as a wrong balance a customer finds a month from now. Right then, at the line where it broke.

This is how Volt style asserts (volt_style/assertions), with the package `github.com/Piechutowski/volt/assert`:

| Call | What it does |
|---|---|
| `assert.That(ok, "message")` | panics if `ok` is false |
| `assert.Fail("format", value)` | always panics, and formats only when it does |
| `assert.Maybe(cond)` | says both outcomes are fine here, and does nothing |
| `assert.IsFailure(recover())` | tells a failed assertion from any other panic |
| `assert.Crash(router)` | makes a failed assertion in a handler stop the server |

When an assertion fails, the panic says what and where:

```text
panic: assertion failed: balance must not be negative (/home/me/app/main.go:7)
```

## Check what you were given

A function does not touch data it has not checked. These are things only my own callers could get wrong, so they are assertions, not errors.

```go
import "github.com/Piechutowski/volt/assert"

const windowBytesMax = 4096

type window struct {
    offset int
    length int
}

func (window window) slice(data []byte) []byte {
    assert.That(window.offset >= 0, "offset must not be negative")
    assert.That(window.length > 0, "length must be positive")
    assert.That(window.length <= windowBytesMax, "length must fit in a window")
    assert.That(window.offset <= len(data), "window must start inside data")
    assert.That(window.length <= len(data)-window.offset, "window must end inside data")
    end := window.offset + window.length
    return data[window.offset:end:end]
}
```

One condition per assertion. `assert.That(a && b, ...)` tells me something broke. Two lines tell me what.

`offset` and `length` are both `int`, so they travel in a struct a caller cannot swap them in (volt_style/no-swappable-arguments).

The last check is `length <= len(data)-offset`, not `offset+length <= len(data)`: with the offset known to be inside the data, the subtraction cannot wrap, and the sum could (volt_style/arithmetic-that-cannot-wrap).

## Check what must not be true

Most bugs live in the cases I was sure could not happen, so I write those down too.

```go
import (
    "slices"

    "github.com/Piechutowski/volt/assert"
)

const queueLengthMax = 64

type connection struct {
    open     bool
    draining bool
    queue    []int64
}

func (connection connection) queued(value int64) connection {
    assert.That(connection.open, "connection must be open")
    assert.That(!connection.draining, "connection must not be draining")
    assert.That(len(connection.queue) < queueLengthMax, "queue must not be full")
    connection.queue = append(slices.Clone(connection.queue), value)
    return connection
}
```

The method takes a value and returns a new one (volt_style/data-structures-are-values), and it clones the queue before it appends, because the caller's connection still holds the old slice.

## If this, then that

```go
import (
    "time"

    "github.com/Piechutowski/volt/assert"
)

type transfer struct {
    pending bool
    timeout time.Duration
}

func (transfer transfer) invariants() {
    if transfer.pending {
        assert.That(transfer.timeout > 0, "pending transfer must have a timeout")
    }
    if !transfer.pending {
        assert.That(transfer.timeout == 0, "posted transfer must not have a timeout")
    }
}
```

## Check what you give back

With one exit, right before `return`:

```go
import "github.com/Piechutowski/volt/assert"

type span struct {
    low  int64
    high int64
}

func (span span) clamp(value int64) int64 {
    assert.That(span.low <= span.high, "low must not exceed high")
    clamped := min(max(value, span.low), span.high)
    assert.That(clamped >= span.low, "result must not be below low")
    assert.That(clamped <= span.high, "result must not be above high")
    return clamped
}
```

With a named result and a deferred closure, so the check sits at the top next to the precondition:

```go
import "github.com/Piechutowski/volt/assert"

const slotCount = 64

func operationSlot(operation uint64) (slot uint64) {
    assert.That(operation > 0, "operation must be positive")
    defer func() { assert.That(slot < slotCount, "slot must be in range") }()
    return operation % slotCount
}
```

It has to be a closure. `defer assert.That(slot < slotCount, ...)` evaluates `slot < slotCount` on the defer line, when `slot` is still zero, and checks that stale answer at exit.

## The point of no return

Errors first, for everything a caller can get wrong. Then one line that says: from here on this must succeed. The deferred check only covers that path.

```go
import (
    "errors"

    "github.com/Piechutowski/volt/assert"
)

var (
    errAmountNotPositive   = errors.New("amount must be positive")
    errBalanceInsufficient = errors.New("balance is insufficient")
)

type account struct {
    balance int64
}

func (account account) withdraw(amount int64) (after account, err error) {
    if amount <= 0 {
        return account, errAmountNotPositive
    }
    if amount > account.balance {
        return account, errBalanceInsufficient
    }

    defer func() {
        assert.That(err == nil, "withdraw must succeed after validation")
        assert.That(after.balance == account.balance-amount, "balance must drop by amount")
        assert.That(after.balance >= 0, "balance must not be negative")
    }()

    after = account
    after.balance -= amount
    return after, nil
}
```

The account is a value, so it cannot be nil and needs no check for it, and the receiver is still the account as it was before, so the postcondition compares against it without a copy saved for the purpose.

## What a type promises

One method holds everything that must always be true about the type. Every method that makes a new one calls it on the way in and on the way out.

```go
import (
    "math"

    "github.com/Piechutowski/volt/assert"
)

const ledgerCountMax = 1 << 20

type ledger struct {
    debits  uint64
    credits uint64
    count   uint32
}

func (ledger ledger) invariants() {
    assert.That(ledger.debits == ledger.credits, "debits must equal credits")
    assert.That(ledger.count <= ledgerCountMax, "count must not exceed ledgerCountMax")
    assert.That(ledger.debits <= uint64(ledger.count)*math.MaxUint32, "debits must fit the amounts posted")
}

func (ledger ledger) post(amount uint32) ledger {
    ledger.invariants()
    assert.That(ledger.count < ledgerCountMax, "ledger must not be full")
    next := ledger
    next.debits += uint64(amount)
    next.credits += uint64(amount)
    next.count++
    next.invariants()
    return next
}
```

The amounts are `uint32` and the count is bounded, so the sums cannot wrap, and the last invariant says so (volt_style/fixed-size-integers-at-boundaries).

The check on the way out is a call before `return`, not `defer next.invariants()`. A deferred method call evaluates its receiver on the defer line, and a value receiver is copied there, so the deferred call would check `next` as it was before the posting. A pointer receiver does not have that problem, which is why the mutable types of [Under the lock](#under-the-lock) can defer theirs.

## Which way a state machine may move

```go
import "github.com/Piechutowski/volt/assert"

type machineState uint8

const (
    machineIdle machineState = iota
    machineRunning
    machineStopped
)

func (state machineState) transition(next machineState) machineState {
    switch state {
    case machineIdle:
        assert.That(next == machineRunning, "idle can only go to running")
    case machineRunning:
        assert.That(next == machineStopped, "running can only go to stopped")
    case machineStopped:
        assert.Fail("stopped is final, got a transition to %d", next)
    default:
        assert.Fail("unknown state %d", state)
    }
    return next
}
```

## Code that cannot be reached

Go does not check that a switch covers every case, so I say it out loud.

```go
import "github.com/Piechutowski/volt/assert"

func (state machineState) String() string {
    switch state {
    case machineIdle:
        return "idle"
    case machineRunning:
        return "running"
    case machineStopped:
        return "stopped"
    }
    assert.Fail("unknown state %d", state)
    return ""
}
```

`%d` prints the number: `fmt` calls `String` only for the verbs that print a string, so the failure message cannot call back into the method that failed.

## Every loop ends

Everything has a limit, so loops do too (volt_style/a-limit-on-everything). The bound goes into the loop's condition, and the assertion after the loop says the bound was never the reason it stopped.

```go
import "github.com/Piechutowski/volt/assert"

const treeDepthMax = 64

func rootFind(parents []int32, node int32) int32 {
    assert.That(node >= 0, "node must not be negative")
    assert.That(int(node) < len(parents), "node must be in the tree")
    for step := 0; step < treeDepthMax && parents[node] != node; step++ {
        node = parents[node]
    }
    assert.That(parents[node] == node, "walk must reach the root within treeDepthMax steps")
    return node
}
```

And the one loop that must never end has no condition, and asserts what keeps it alive:

```go
import "github.com/Piechutowski/volt/assert"

type event struct {
    name string
}

func eventsServe(events chan event, handle func(event)) {
    for {
        next, open := <-events
        assert.That(open, "events channel must never close")
        handle(next)
    }
}
```

`for next := range events` would end quietly when the channel closes, which is why volt_style/a-limit-on-everything counts it as unbounded.

## Values in the message

`That` evaluates its arguments every time, even when the check passes. A `fmt.Sprintf` there cost me about 100 ns and 3 allocations per call on a hot path, for a message I almost never see. Constant messages are free. When I want the numbers, I put `Fail` behind an `if`, so the formatting only happens when it fails:

```go
import (
    "github.com/Piechutowski/volt/assert"
    "github.com/Piechutowski/volt/checked"
)

func balanceAfter(balance int64, amount int64) int64 {
    after, ok := checked.Subtract(balance, amount)
    if !ok {
        assert.Fail("balance %d - amount %d wrapped", balance, amount)
    }
    if after < 0 {
        assert.Fail("balance %d - amount %d went negative", balance, amount)
    }
    return after
}
```

The subtraction goes through `checked`, because `balance - amount` could wrap to a positive number and pass the second check ([Checked arithmetic](#checked-arithmetic)).

## Checked arithmetic

Go wraps without a sound (volt_style/arithmetic-that-cannot-wrap). Where rearranging the arithmetic cannot rule it out, the package `checked` says whether the result is exact, and who made the values decides what a false means. Input that wraps is an error:

```go
import (
    "errors"

    "github.com/Piechutowski/volt/checked"
)

const amountCountMax = 1024

var (
    errAmountsTooMany = errors.New("more amounts than amountCountMax")
    errTotalTooLarge  = errors.New("total does not fit in 64 bits")
)

func amountsTotal(amounts []uint64) (uint64, error) {
    if len(amounts) > amountCountMax {
        return 0, errAmountsTooMany
    }
    total := uint64(0)
    for _, amount := range amounts {
        next, ok := checked.Add(total, amount)
        if !ok {
            return 0, errTotalTooLarge
        }
        total = next
    }
    return total, nil
}
```

A length read from the wire is converted, not cast:

```go
import (
    "errors"

    "github.com/Piechutowski/volt/checked"
)

var (
    errLengthTooLarge = errors.New("length does not fit in an int")
    errLengthPastEnd  = errors.New("length runs past the end of the data")
)

func payloadLength(prefix uint64, available int) (int, error) {
    length, ok := checked.Convert[int](prefix)
    if !ok {
        return 0, errLengthTooLarge
    }
    if length > available {
        return 0, errLengthPastEnd
    }
    return length, nil
}
```

`int(prefix)` would turn a prefix of 2⁶³ into a negative length, and `length > available` would let it through.

The program's own values wrap only when the code is wrong, so there a false is an assertion, as in [Values in the message](#values-in-the-message). A division goes through a function that names how it rounds (volt_style/division-says-how-it-rounds).

## The world first, then me

Data from outside starts untrusted. The checks against it are errors. Once it passes them, it is mine, and from there I assert.

```go
import (
    "encoding/binary"
    "errors"
    "hash/crc32"

    "github.com/Piechutowski/volt/assert"
)

const headerSize = 16

var (
    errHeaderShort   = errors.New("header is shorter than headerSize")
    errHeaderCorrupt = errors.New("header checksum does not match")
)

type header struct {
    checksum  uint32
    size      uint32
    operation uint64
}

func (header header) checksumComputed() uint32 {
    var body [headerSize - 4]byte
    binary.LittleEndian.PutUint32(body[0:], header.size)
    binary.LittleEndian.PutUint64(body[4:], header.operation)
    return crc32.ChecksumIEEE(body[:])
}

func (header header) valid() bool {
    return header.checksum == header.checksumComputed()
}

func headerParse(data []byte) (header, error) {
    if len(data) < headerSize {
        return header{}, errHeaderShort
    }
    parsed := header{
        checksum:  binary.LittleEndian.Uint32(data[0:]),
        size:      binary.LittleEndian.Uint32(data[4:]),
        operation: binary.LittleEndian.Uint64(data[8:]),
    }
    if !parsed.valid() {
        return header{}, errHeaderCorrupt
    }
    assert.That(parsed.size >= headerSize, "valid header must cover itself")
    return parsed, nil
}
```

Disks flip bits, so a bad checksum is an error. A good checksum on a header that does not cover itself means the writer is wrong, so that is an assertion.

`headerParse` takes bytes, not a reader. The read is an effect and happens at the edge (volt_style/effects-at-the-edge-as-data); what it read is handed to this pure function.

## The same thing, from both sides

The writer asserts the header is right before it goes out. The reader, once the checksum passes, asserts the same thing. If the two ever disagree, one of them misunderstands the format, and I find out on the first run instead of the first corruption.

```go
import (
    "encoding/binary"

    "github.com/Piechutowski/volt/assert"
)

func headerNew(size uint32, operation uint64) header {
    made := header{size: size, operation: operation}
    made.checksum = made.checksumComputed()
    return made
}

func (header header) encoded() [headerSize]byte {
    assert.That(header.valid(), "header must be valid before writing")
    assert.That(header.size >= headerSize, "header must cover itself before writing")
    var buffer [headerSize]byte
    binary.LittleEndian.PutUint32(buffer[0:], header.checksum)
    binary.LittleEndian.PutUint32(buffer[4:], header.size)
    binary.LittleEndian.PutUint64(buffer[8:], header.operation)
    return buffer
}
```

## Under the lock

What a mutex protects is only true while you hold it, so that is where the assertion goes.

```go
import (
    "sync"

    "github.com/Piechutowski/volt/assert"
)

type pool struct {
    mutex    sync.Mutex
    capacity int
    free     int
    used     int
}

func (pool *pool) acquire() bool {
    pool.mutex.Lock()
    defer pool.mutex.Unlock()

    if pool.free == 0 {
        return false
    }
    pool.free--
    pool.used++
    assert.That(pool.free+pool.used == pool.capacity, "free + used must equal capacity")
    return true
}
```

A pool mutates in place, so it is inventoried state (volt_style/mutable-state-is-inventoried): its own methods may write it, and nothing else may.

## Instead of a comment

A comment can be wrong for years. This cannot.

```go
import "github.com/Piechutowski/volt/assert"

func slotPairs(count int) int {
    assert.That(count%2 == 0, "count is always even here: callers pass slotCount")
    return count / 2
}
```

## Both are fine

`Maybe` does nothing. It tells the next reader I thought about both cases and neither is a bug.

```go
import "github.com/Piechutowski/volt/assert"

func (connection connection) flushed() connection {
    assert.Maybe(len(connection.queue) == 0)
    connection.queue = nil
    return connection
}
```

## Before the program even runs

No package needed. A constant that breaks the rule breaks the build.

```go
import (
    "fmt"
    "unsafe"
)

const sectorSize = 4096

type wireHeader struct {
    checksum  uint32
    size      uint32
    operation uint64
    reserved  [48]byte
}

const _ uint = sectorSize - headerSize // sectorSize >= headerSize

var _ = [1]struct{}{}[sectorSize%headerSize]          // sectorSize is a multiple of headerSize
var _ = [1]struct{}{}[unsafe.Sizeof(wireHeader{})-64] // wireHeader is exactly 64 bytes

var _ fmt.Stringer = machineState(0) // machineState implements fmt.Stringer
```

When one breaks, the build stops at that line with `(overflows)`, `overflows uintptr` or `index 1 out of bounds [0:1]`.

## Servers

`net/http` catches a panic in a handler, logs it and keeps serving. That is exactly what I do not want: the code is wrong and it keeps taking requests. So the router gets wrapped once.

```go
import (
    "net/http"
    "time"

    "github.com/Piechutowski/volt/assert"
)

func serverRun(address string, router *http.ServeMux) error {
    server := &http.Server{
        Addr:              address,
        Handler:           assert.Crash(router),
        ReadHeaderTimeout: 5 * time.Second,
        ReadTimeout:       10 * time.Second,
        WriteTimeout:      10 * time.Second,
        IdleTimeout:       time.Minute,
        MaxHeaderBytes:    1 << 16,
    }
    return server.ListenAndServe()
}
```

A failed assertion now prints its message and stack and exits with status 2. Other panics go through untouched. `os.Exit` does not run deferred functions, so nothing cleans up on the way out. That is the trade: a stopped process instead of a wrong one. A panic in a goroutine I start from a handler already kills the process, with or without `Crash`.

The server spells out its timeouts and header limit instead of trusting the defaults, which are none (volt_style/explicit-options).

## Guigui

Guigui calls the widget methods. Assertions go where it hands over control. VOLT_STYLE.md's chapter on Guigui (volt_style/guigui-and-ebitengine) says how the rest of Volt style meets it.

### No `Crash` needed

On desktop, neither Guigui nor Ebitengine recovers panics, so a failed assertion stops the app. Run it from a terminal to see the message. On mobile, Ebitengine recovers a panic in its loop and turns it into an error that carries the message and the stack as text, so `IsFailure` cannot see it there; the app treats every error from its run as the end.

### Guigui's arguments

Don't assert `context`, `adder`, `layouter` or `widgetBounds`. They are Guigui's contract. Assert the widget's own fields.

### Build and Layout agree

`LayoutWidget` on a child that `Build` did not add only stores its bounds, with no error. Pick the child in one method, record it in `Build`, check it in `Layout`.

```go guigui
import (
    "github.com/guigui-gui/guigui"

    "github.com/Piechutowski/volt/assert"
)

type page uint8

const (
    pageList page = iota
    pageDetail

    pageCount
)

type Pages struct {
    guigui.DefaultWidget

    list      List
    detail    Detail
    page      page
    pageBuilt page
}

func (pages *Pages) current() guigui.Widget {
    switch pages.page {
    case pageList:
        return &pages.list
    case pageDetail:
        return &pages.detail
    }
    assert.Fail("unknown page %d", pages.page)
    return nil
}

func (pages *Pages) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
    adder.AddWidget(pages.current())
    pages.pageBuilt = pages.page
    return nil
}

func (pages *Pages) Layout(
    context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter,
) {
    assert.That(pages.page == pages.pageBuilt, "page must not change between Build and Layout")
    layouter.LayoutWidget(pages.current(), widgetBounds.Bounds())
}
```

Children build after their parent in the same pass, so one of them could change the page through a shared model. Volt style forbids that, since `Build` only reads (volt_style/build-reads-handlers-write), and this assertion is how I find out when it happens anyway.

### The setter checks the write

```go guigui
import "github.com/Piechutowski/volt/assert"

func (pages *Pages) PageSet(next page) {
    assert.That(next < pageCount, "page must be a known page")
    pages.page = next
}
```

`PageSet` checks the write, `current` checks the read. `page` is a `uint8`, so the type already says it is not negative, and an assertion of that would be padding (volt_style/fixed-size-integers-at-boundaries).

### State keys

A field changed outside an input or event handler, in `Tick` for example, triggers no rebuild. No assertion sees that, so put the field in the state key:

```go guigui
import "github.com/guigui-gui/guigui"

func (pages *Pages) WriteStateKey(context *guigui.Context, writer *guigui.StateKeyWriter) {
    writer.WriteInt(int(pages.page))
}
```

### Providing a value through Env

`Env` hands out a pointer that descendants can write through. Check the type's invariants there ([What a type promises](#what-a-type-promises)).

```go guigui
import (
    "github.com/guigui-gui/guigui"

    "github.com/Piechutowski/volt/assert"
)

var envKeyModel = guigui.GenerateEnvKey()

type Model struct {
    itemCount    int
    itemSelected int
}

func (model Model) invariants() {
    assert.That(model.itemCount >= 0, "itemCount must not be negative")
    assert.That(model.itemSelected >= 0, "itemSelected must not be negative")
    assert.That(model.itemSelected <= model.itemCount, "itemSelected must not pass itemCount")
}

type Root struct {
    guigui.DefaultWidget

    pages Pages
    model Model
}

func (root *Root) Env(context *guigui.Context, key guigui.EnvKey, source *guigui.EnvSource) (any, bool) {
    switch key {
    case envKeyModel:
        root.model.invariants()
        return &root.model, true
    }
    return nil, false // basicwidget's keys pass through here too
}
```

### Reading a value from Env

A missing value is normal. A wrong type is a bug.

```go guigui
import (
    "github.com/guigui-gui/guigui"

    "github.com/Piechutowski/volt/assert"
)

type Detail struct {
    guigui.DefaultWidget

    itemShown int
}

func (detail *Detail) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
    value, ok := context.Env(detail, envKeyModel)
    if !ok {
        return nil // not in the tree yet
    }
    model, ok := value.(*Model)
    assert.That(ok, "envKeyModel must hold a *Model")
    detail.itemShown = model.itemSelected
    return nil
}
```

A plain `value.(*Model)` also panics, but `IsFailure` does not recognize that panic.

### Not bugs

A layout item gets empty bounds when its siblings take all the space:

```go guigui
import (
    "github.com/guigui-gui/guigui"

    "github.com/Piechutowski/volt/assert"
)

type List struct {
    guigui.DefaultWidget
}

func (list *List) Layout(
    context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter,
) {
    assert.Maybe(widgetBounds.Bounds().Empty())
}
```

A shader the driver rejects is an error returned from `Build`.

### Hot paths

`Layout` runs on every rebuild and `Draw` on every repaint. Keep their messages constant ([Values in the message](#values-in-the-message)).

### Testing widget decisions

Keep decisions in plain methods, so tests cover every case without running Guigui.

```go guigui
import (
    "math"
    "testing"

    "github.com/Piechutowski/volt/assert"
)

func TestPagesCurrentCoversEveryPage(t *testing.T) {
    var pages Pages
    for next := range pageCount {
        pages.PageSet(next)
        if pages.current() == nil {
            t.Fatalf("page %d has no widget", next)
        }
    }
}

func TestPagesPageSetRejectsUnknownPages(t *testing.T) {
    for _, next := range []page{pageCount, math.MaxUint8} {
        t.Run("", func(t *testing.T) {
            defer func() {
                if !assert.IsFailure(recover()) {
                    t.Fatalf("page %d: want an assertion failure", next)
                }
            }()
            var pages Pages
            pages.PageSet(next)
        })
    }
}
```

## Tests

```go
import (
    "testing"

    "github.com/Piechutowski/volt/assert"
)

func TestLedgerPostRejectsAFullLedger(t *testing.T) {
    defer func() {
        if !assert.IsFailure(recover()) {
            t.Fatal("want an assertion failure")
        }
    }()
    full := ledger{count: ledgerCountMax}
    _ = full.post(1)
}
```

If I ever write a catch-all `recover`, it hands assertion failures back:

```go
import "github.com/Piechutowski/volt/assert"

func taskRun(task func()) (failed bool) {
    defer func() {
        value := recover()
        if value == nil {
            return
        }
        if assert.IsFailure(value) {
            panic(value)
        }
        failed = true
    }()
    task()
    return false
}
```

## Fuzzing

The fuzzer throws inputs at the code. The assertions are how it knows something went wrong. Without them, a broken invariant is just another run that finished.

```go
import "testing"

func FuzzLedgerPost(f *testing.F) {
    f.Add(uint32(1), uint32(2))
    f.Fuzz(func(t *testing.T, first uint32, second uint32) {
        posted := ledger{}.post(first).post(second)
        if posted.debits != uint64(first)+uint64(second) {
            t.Fatalf("debits %d, want %d", posted.debits, uint64(first)+uint64(second))
        }
    })
}
```

## In production

They stay on. Production is where the data is real. A passing `assert.That` with a constant message is inlined and costs about 0.1 ns on top of the work around it. A wrong balance costs more.

Benchmarking that took a few tries. The first one asserted `i >= 0` on the loop counter, the compiler proved it was always true and deleted the check, and I measured an empty loop. Then `b.Loop`, which is the right default for almost every benchmark but not this one. Before Go 1.26 it turns off inlining in its body, so I measured a function call my real code never makes. From Go 1.26 it inlines, but it keeps every value in the body alive, and that costs about 0.45 ns, more than the thing I was measuring. That is why the `bloop` modernizer is off by default in `go fix`. What works is a plain `b.N` loop over data the compiler cannot see through, next to a baseline doing the same work without the assert. To check that a benchmark still asserts anything:

```fish
go test -c -o /dev/null -gcflags='-d=ssa/prove/debug=1 -m' ./assert 2>&1 | grep -E 'Proved|inlining call to That'
```
