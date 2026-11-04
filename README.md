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
| Volt | 340 | 64 | 119 | 523 | 1377 | 2 |
| Gin + GORM | 707 | 124 | 0 | 831 | 0 | 24 |
| chi + sqlc + goose | 636 | 120 | 99 | 855 | 440 | 7 |
| Echo + sqlx + goose | 671 | 168 | 43 | 882 | 0 | 23 |
| oapi-codegen + Ent | 575 | 0 | 587 | 1162 | 16496 | 25 |

Volt writes about 40 percent less than the hand-written stacks here,
not the 97 percent of a pure CRUD table (`examples/byhand`): the rules
are handlers either way. Writing the Volt version found five gaps,
paid for in its own lines: a scope load could not answer its own miss,
so every handler checks the member, which `Load.Rescue` now spares
though the blog is not yet rewritten onto it; a transaction cannot
use an id an earlier step assigned, so a post's tags are not written
atomically; a select has no limit, so the feed reads every post after
the cursor; a select has no optional predicate, so the tag filter is a
second select; and the generated client is untyped for controller
routes, so the typed client is written by hand.

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
