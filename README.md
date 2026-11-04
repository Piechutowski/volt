# Volt

> [!CAUTION]
> **This project is an alpha version, and everything may change in the future.**

Volt is a web framework for Go built around a small schema language.
Tables, queries and routes live in `.volt` files, and `volt gen` turns
them into ordinary Go: models, query functions, a router, handler
interfaces, reverse-URL helpers and an HTTP client. The output is meant
to be read, and nothing in it is resolved at run time: no reflection,
no registries, no base class to trace.

Migrations are not generated. Each one is written by hand as a
`migration` element; the checker folds the migrations against the
declared schema and refuses the project when the two disagree, and
`volt migration up` applies them in order. A tool that guesses a
migration from a diff gets a rename wrong sooner or later, so there is
none.

The generated code and the runtime follow the functional style: values
that are never mutated, and effects described as data and performed in
one place. A handler is a function from its inputs to a response value;
the generated code runs the queries, performs the writes and sends the
reply. That is what makes a Volt application predictable, testable
without a database, and safe to run concurrently.

## Why code generation

- **One source of truth.** The `.volt` file is the contract for SQLite,
  Go and the generated client. A disagreement is a compile error.
- **Nothing hidden.** What runs is in a file you can open. There is no
  run-time dispatch and no framework base class to understand.
- **Fewer tokens.** A schema line stands for many lines of Go, for a
  person and for a model alike, and the generated Go is the same every
  time.

## Why the functional style

Avoiding mutation and avoiding side effects is what makes code easy to
reason about and to test. Go does not enforce either, so Volt does: the
generator produces values, `volt vet` keeps handlers pure, and every
effect is performed by generated code you can read.

## Layout

| Directory | What it is |
|---|---|
| `*.go` (root) | the network runtime generated code links against |
| `cmd/volt` | the one binary: `check`, `vet`, `gen`, `routes`, `stress`, `migration`, `lsp`, `version` |
| `lang/` | the language: scanner, parser, checker, vet, conformance corpus |
| `gen/` | the generators |
| `nao/` | the data layer: naming, generation, the value runtime `rt`, the runner `run` |
| `lsp/` | the language server |
| `grammar/`, `zed-extension/` | the editor stack |
| `itest/` | the fixture application served over `httptest`, drift-checked against the generator |
| `examples/` | one tiny program per feature (`tables`, `migrations`, `scopes`, `pipelines`, `validation`): its `.volt` files, a server in process and a guigui window that calls it through the generated client; `go generate` writes the generated code, which is not committed; `examples/byhand` is the `tables` server and client written by hand, held to the same replies by a test |
| `compare/` | one blog with real rules built five ways, Volt and four popular Go stacks, each held to the same contract suite and counted by `go run ./compare/measure` |
| `docs/` | `spec.md`, `lint.md`, `editor.md`, `roadmap.md`, `plan.md` |

## Compared with other Go stacks

`compare/` builds one small blog five ways: members, drafts only their
author sees or edits, publishing with an audit entry in one
transaction, comments only on published posts and at most three an
hour, a keyset-paged feed with a tag filter and comment counts, and
per-member stats. Every implementation passes the same black-box
suite (`compare/suite`) unchanged and ships a typed Go client. Lines
are neither blank nor comments, tests left out; `go run
./compare/measure` prints this table after `go generate ./compare/...`.

| Stack | Server Go | Client Go | Contract and schema | Hand-written total | Generated | Third-party modules |
|---|---:|---:|---:|---:|---:|---:|
| Volt | 294 | 66 | 117 | 477 | 1352 | 2 |
| Gin + GORM | 707 | 124 | 0 | 831 | 0 | 24 |
| chi + sqlc + goose | 636 | 120 | 99 | 855 | 440 | 7 |
| Echo + sqlx + goose | 671 | 168 | 43 | 882 | 0 | 23 |
| oapi-codegen + Ent | 575 | 0 | 587 | 1162 | 16496 | 25 |

Volt's 477 hand-written lines are 57 percent of the 831 of Gin + GORM
and 54 percent of the 882 of Echo + sqlx + goose, chi + sqlc + goose's
855 between them, so 43 to 46 percent fewer than those three, and 41
percent of the 1162 of oapi-codegen + Ent, 59 percent fewer; not the 97
percent fewer of a pure CRUD table (`examples/byhand`): the rules are
handlers either way. Writing the Volt version found five gaps, which
cost it lines and, in three of its rules, correctness under
concurrency; four are closed now, the blog rewritten onto them. A
scope load could not answer its own miss, so every handler checked the
member; `Load.Rescue` answers it with 401 in the load. An effect could
not use what an earlier one read or assigned inside one transaction,
so a post's tags were written in a second one and a refused tag left
the post, and publishing, editing and the comment limit checked what
they read before the transaction that wrote, so concurrent requests
could publish a draft twice, write a short body into a post a publish
had just published, or store a fourth comment; `rt.Atomic` over
`rt.Then` writes the post and its tags in one and makes the three
checks inside the transaction that writes. A select had no limit, so the feed
read every post after the cursor and cut the page in Go; `limit:` cuts
it in SQL. A select had no optional predicate, so the tag filter was a
second select and the first page passed a sentinel cursor; optional
parameters and `exists` make the feed one select. The fifth is half
closed: the generated client now carries default headers, so X-User
rides on its `Header` where a hand-written transport carried it, but
it stays untyped for controller routes, since Volt does not know what
a hand-written action writes, and that is unchanged for now, so the
typed client is still written by hand, one method per route over the
generated one. The rewrite took the blog from 523 hand-written lines
to 477: 46 fewer of server Go and 2 fewer of `.volt`, while the typed
client gained 2 by building on the generated client instead of the
runtime's.

The counts are of each stack as written, and `compare/suite` sends one
request at a time, so it does not hold the rules under concurrency;
Volt's own test does (`compare/volt`), and the other stacks have none.
`go run ./compare/race | grep '^|'` sends all five servers thirty
rounds of twenty publishes of one draft at once, ten comments by one
member at once and sixteen edits to a short body racing four publishes
of a draft; GORM logs each write SQLite refuses, which the `grep`
drops. The counts are races and move from run to run; over seven runs
every stack published a draft once, Volt and oapi-codegen + Ent broke
no rule, and the other three broke these, in rounds of thirty:

| Stack | More than three comments | Published with a short body | Replies 500 |
|---|---:|---:|---:|
| Gin + GORM | 0 | 12 to 21 | 84 to 161 |
| chi + sqlc + goose | 0 | 0 to 4 | 0 |
| Echo + sqlx + goose | 12 to 25 | 4 to 11 | 0 |

Gin + GORM's 500s are `database is locked`, mostly on the comments, and
in up to five rounds a run they left a member fewer than the three
comments the rules allow. Those three stacks make the checks Volt makes,
but before writes that do not hold them, where Volt's 477 lines and
Ent's 1162 make them inside the transaction that writes, so the
percentages above compare programs that differ in that much.

## Crib sheet

Requires Go 1.27+ and a C compiler; the examples also need the X11 and
OpenGL development headers their guigui windows build against.

From the repository root. An example regenerates its code from its
`.volt` files and opens its window, so the generated files can be read
beside the running program:

```sh
go run ./examples/run tables
```

The tool and the tests, with `go install`'s directory on the path;
`volt check` is silent when the packages are clean:

```sh
go install ./cmd/volt
volt check ./examples/... && volt routes ./examples/scopes/app
go generate ./examples/... ./compare/...
go test ./... ./lsp/... ./cmd/volt/... ./examples/... ./compare/...
```

The examples' generated code is not in the repository, so
`go generate ./examples/...` comes before building or testing them.
