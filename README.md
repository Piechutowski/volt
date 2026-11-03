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
| `examples/` | one tiny program per feature (`tables`, `migrations`, `scopes`, `pipelines`, `validation`): its `.volt` files, a server in process and a guigui window that calls it through the generated client; `go generate` writes the generated code, which is not committed |
| `docs/` | `spec.md`, `lint.md`, `editor.md`, `roadmap.md`, `plan.md` |

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
go generate ./examples/...
go test ./... ./lsp/... ./cmd/volt/... ./examples/...
```

The examples' generated code is not in the repository, so
`go generate ./examples/...` comes before building or testing them.
