# Volt

> [!CAUTION]
> **This project is an alpha version, and everything may change in the future.**

Volt is a code-generation-first web framework for Go. You declare the
schema and the routes in `.volt` files; `volt gen` writes the models, the
queries, the migrations, the router and the client as plain Go that you
read line by line. Nothing is resolved at run time: no reflection, no
registries, no inheritance to trace.

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
| `nao/` | the data layer: naming, generation, the value runtime `rt` |
| `lsp/` | the language server |
| `grammar/`, `zed-extension/` | the editor stack |
| `itest/` | the fixture application served over `httptest`, drift-checked against the generator |
| `docs/` | `spec.md`, `lint.md`, `editor.md`, `roadmap.md`, `plan.md` |

## Crib sheet

Requires Go 1.27+ and a C compiler.

```sh
go install ./cmd/volt
volt check ./app && volt gen ./app && volt routes ./app
go test ./... ./lsp/... ./cmd/volt/...
```
