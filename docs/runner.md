# The runner

> [!CAUTION]
> **This project is an alpha version, and everything may change in the future.**

How `nao/run` meets SQLite: its connections, its prepared statements,
how it picks a connection for an effect, and how it inserts many rows.
The rules are normative in `spec.md` (§generated-query-surface, the
runner, and §changesets, `Create`, `CreateMany` and `Update`); this file says why they are what they are and
what was measured on the way. Every number here comes from one 4-core
Linux container with mattn/go-sqlite3 and a database file on disk; read
them as proportions, not as promises.

## 1. Two pools

SQLite in WAL mode lets one writer and any number of readers work at the
same time. A reader never blocks the writer and the writer never blocks
a reader. Writes, however, run one at a time however many connections
are open. Measured with four goroutines:

| Connections | 40,000 key lookups | 400 one-row write transactions |
|---|---|---|
| 1 | 590 ms | 12 ms |
| 2 | 284 ms | |
| 4 | 135 ms | 10.5 ms |

Reads scale with connections up to the number of cores; writes do not
scale at all. One connection shared by four goroutines is worse than
serial: each lookup cost about 15 µs against 4.6 µs from one goroutine,
the difference spent handing the connection over.

So `run.Open` opens two pools on the file:

- **The writer**: one connection, with `journal_mode = WAL`,
  `busy_timeout = 5000` and `foreign_keys = ON`. Every effect that can
  write queues on it in Go, so Volt's own writers never meet `database
  is locked`.
- **The readers**: as many connections as `GOMAXPROCS`, with
  `busy_timeout = 5000`, `foreign_keys = ON` and `query_only = ON`. A
  reader handed a write fails loudly instead of writing outside the
  writer.

The readers' pool keeps as many idle connections as it allows open.
database/sql keeps only two idle connections by default and closes the
rest as soon as they are returned, which under load would open and close
a connection for almost every read. Nothing closes idle readers on a
timer: a reader stays open and warm until the pool shrinks or the
database closes.

**Following the cores.** Since Go 1.25 the runtime updates `GOMAXPROCS`
by itself when the CPU count, the affinity mask or the cgroup CPU quota
changes, which covers systemd's `CPUQuota=` and `AllowedCPUs=` set at run
time with `systemctl set-property`. Each time the runner hands out a
reader it compares `GOMAXPROCS` with the size it last gave the pool and
resizes the pool when they differ; a smaller size closes the readers
left idle, reads in flight finish. There is no goroutine watching.

**One core** gives one writer and one reader: a read still never waits
behind an open write transaction such as an autosave. **Many cores**
give many readers; section 6 lists what is not yet known there.

**No in-memory database.** An in-memory database belongs to the one
connection that opened it: a second connection gets a separate, empty
database, and the pool closing an idle connection loses the data. Two
pools cannot share one, and WAL does not apply to one anyway. `run.Open`
refuses `:memory:`, `file::memory:` and `mode=memory`; tests open a file
in their temporary directory through `internal/dbtest`, as production
opens one. The suites that use a database went from 1.5 s to 1.9 s.

**Read only.** `run.OpenReadOnly` opens readers only, with no change of
journal mode, so a question such as `volt migration status` leaves the
file exactly as it found it.

## 2. Which connection an effect runs on

Every effect knows whether it can write (`writes()`), the same flag that
already chose `BEGIN IMMEDIATE` over `BEGIN`. The runner decides where an
effect meets an executor: at `run.Run`, at each step of a `Parallel`
outside a transaction, and at each half of a `Then`.

| Effect | Runs on |
|---|---|
| Cannot write, outside a transaction | a reader |
| Can write | the writer |
| A `Transaction` or an `Atomic` of reads alone | one reader, `BEGIN` |
| A transaction that can write | the writer, `BEGIN IMMEDIATE` |
| Anything inside a transaction | that transaction's connection |

A SQLite transaction lives on one connection, so inside a writing
transaction its reads run on the writer too: they must see the
transaction's own uncommitted writes, and a check followed by a write
must happen under the same lock.

A read-only transaction gets one snapshot. Two reads such as a user and
their orders could otherwise straddle a commit; inside `BEGIN` on one
reader, SQLite fixes the snapshot at the first read and every later read
in that transaction sees the same one, while it takes no lock and never
waits for the writer.

A read that starts after a write committed sees it, because every read
transaction under WAL starts from the last commit, so `Then(write, read)`
outside a transaction is correct with the read on another connection. A
read never sees a write transaction before it commits.

`rt.All` of reads now really runs in parallel; with one connection its
steps queued.

**Readers under write load.** Each connection keeps its own page cache,
up to about 2 MB of RAM, so that the pages every lookup walks, the
B-tree's root and interior pages, need no system call. A reader throws
that cache away whenever a read transaction starts after another
connection committed (`pager_reset` in `pagerBeginReadTransaction`), so
behind an active writer the readers fetch those pages again from the
operating system's copy. Measured with four readers doing 80,000 random
key lookups in 200,000 rows while a writer commits one-row updates:

| Writer | Two pools | One connection, the old design |
|---|---|---|
| none | 260 to 290 ms | 1,120 to 1,210 ms |
| a commit every 1 ms | 365 to 395 ms | 1,110 to 1,140 ms |
| commits back to back | 540 to 560 ms | 1,910 to 1,970 ms |

At a commit a millisecond the writer uses almost no CPU, so the 40%
slowdown is the readers refilling their caches. The two pools still read
3.5 to 4.3 times faster under every load, since on one connection every
read queues behind every other statement. Memory-mapped reads
(`mmap_size`), which read the operating system's copy directly with
nothing to throw away, measured the same with no writer or a commit a
millisecond and about 20% slower with commits back to back, so the
readers do not set it. The memory is not a concern either: 128 full
caches are about 256 MB.

**Migrations.** A migration that changes the journal mode needs the file
to itself, and the readers' idle connection made it fail with `database
is locked`. `MigrateUp` and `MigrateDown` close the idle readers before
they run; the readers reopen on the next read.

## 3. Prepared statements

SQLite compiles a statement's text into bytecode (`sqlite3_prepare_v2`)
before it runs. Reusing the text in Go is cheap; reusing the compiled
statement is what saves time.

- **Outside a transaction** each pool has its statement cache, keyed by
  the text, least recently used out past 256. A `*sql.Stmt` of
  database/sql is prepared lazily on each connection it runs on and
  kept there, so on the writer's one connection every text compiles
  once.
- **Inside a transaction** the statement runs on the transaction's own
  connection. The pool's cached statements cannot be used there: the
  pool has one writer connection, the transaction holds it, and a pool
  statement would wait for a free connection forever. Until this change
  every statement in a transaction was therefore compiled and thrown
  away, which a profile showed as 48% of a bulk insert's time in
  `sqlite3_prepare_v2`.
- **Now** each transaction keeps a map from text to a statement it
  prepared on its connection, up to 64 texts, runs a text past those
  unprepared, and closes every one before it commits or rolls back,
  whatever ends it. Ten thousand `UserCreate` steps in one `rt.Tx` went
  from 202 ms to 111 ms.

**The design that was dropped.** Holding the writer's connection for
the database's whole life, with one statement cache on it, would have
reused statements across transactions too. It breaks `db.Pool()`: with
its only connection held, every caller of the pool waits forever, and
98 call sites use it (95 of them tests applying DDL). The gain is small:
writes outside a transaction already reuse their statements on the one
connection, so holding it only saves one compile per distinct statement
per transaction, about 6 µs, some 20 µs for an autosave, less than the
commit itself.

## 4. Bulk insert

`<Model>CreateMany(cs)` is an `rt.Batch`: every changeset validated as
`Create` validates one, then multi-row inserts in one transaction, all
or nothing, yielding how many rows it inserted.

- **A count, not rows or ids.** SQLite returns the rows of a multi-row
  `RETURNING` in no promised order, which may change between releases or
  runs, so ids could not be matched to the changesets given. A caller
  that needs the rows reads them back by a key it knows.
- **Every invalid row is reported.** One `*rt.InvalidRows` lists each by
  its index with its field errors, before any statement runs; the error
  spine answers 422 and each detail carries `row`, so an import can mark
  them all at once.
- **A database refusal names no row.** A unique, NOT NULL or foreign key
  violation rolls the batch back and maps onto `*rt.Invalid` as
  `Create`'s does, but SQLite does not say which row of a multi-row
  insert broke it.
- **Grouping.** `VALUES` takes no `DEFAULT`, so rows that mark different
  columns cannot share a statement. Consecutive rows with the same marks
  share statements, which keeps the insertion order; rows built with
  `From` are one group.
- **Chunking.** SQLite binds at most 32,766 parameters in one statement
  as mattn builds it, so a statement holds 32,766 divided by the marked
  columns rows: 8,191 for four columns. Every full statement has the
  same text, so the transaction compiles it once. A test pins the limit:
  32,766 parameters work and 32,767 fail.

Measured for ten thousand rows of `User` built with `From`:

| Path | Time |
|---|---|
| One `UserCreate` per row in one transaction | 112 ms |
| `UserCreateMany`, statements of 8,191 rows | 44 ms |
| `UserCreateMany`, statements of 1,000 rows | 37 ms |

The smaller statements were faster here; section 6 keeps the choice
open.

## 5. Where the time goes

Comparing a bulk insert against a raw prepared `INSERT` makes Volt look
four times slower, but the raw insert reads nothing back. Measured for
ten thousand rows on one connection:

| Work | Time |
|---|---|
| Prepared `INSERT`, nothing read back | 24 ms |
| Prepared `INSERT ... RETURNING` the row, scanned | 71 ms |
| Volt with statements reused (rt changesets, validation, effects) | about 85 ms |

Reading the row back is the expensive part: several cgo calls per
column, a `sql.Rows` per insert, a reset per close and a time parse.
Volt's own layer, building and validating the effects, costs about
12 ms of it.

Against other Go data layers, all on the same pool, ten thousand rows:

| Workload | Volt | ent | bob | GORM | raw |
|---|---|---|---|---|---|
| Insert one by one in a transaction | 144 ms, 81 ms with statements reused | 149 ms | 177 ms | 190 ms | 23 ms (no read back) |
| Insert in 1,000-row statements | 20 ms (hand-written then) | 34 ms | 23 ms | 29 ms | 15 ms |
| Key lookups, no transaction | 44 ms | 129 ms | 112 ms | 126 ms | 38 ms |
| List every row | 12 ms | 12 ms | 14 ms | 18 ms | 11 ms |

GORM and ent read back only the key; Volt and bob read back the whole
row. Volt's pool statement cache is why its lookups are three times
faster than the others', which prepare every statement.

Among drivers behind database/sql, mattn is the fastest, tied with
tailscale's, and the pure Go ones are 20 to 50% slower. A driver that
skips database/sql (zombiezen, on the pure Go engine) was a third faster
on reads, so most of the remaining cost is database/sql itself. mattn is
the settled choice (AGENT.md); this is recorded, not proposed.

## 6. Open questions

Kept in `plan.md` step 21 until they are measured and settled:

1. **Readers on many cores.** Scaling was measured to four cores only.
   mattn's SQLite keeps memory statistics behind one global mutex
   (`SQLITE_DEFAULT_MEMSTATUS` is on), which may cap read scaling at
   high core counts; if it does, a cap on the readers is the remedy.
2. **Chunk size.** Statements of 1,000 rows were 16% faster than
   statements cut to the parameter limit; whether a fixed chunk should
   replace the limit is undecided.
3. **Double-quoted identifiers.** mattn builds SQLite with
   `SQLITE_DQS=3`, so a double-quoted identifier that names no column
   compiles as a string literal. Volt quotes every identifier, so a
   misspelled column in generated SQL would compile and match nothing;
   the generator tests cannot see it either.
4. **Cancellable contexts.** With a context that can be cancelled, mattn
   starts a goroutine per statement and per row to watch it; the numbers
   here used `context.Background()`, so request traffic pays more.
