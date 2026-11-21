# Volt

> [!CAUTION]
> **This project is an alpha version, and everything may change in the future.**

Volt is a web framework for Go built around a small language. Tables,
queries, routes and migrations live in `.volt` files, and `volt gen`
turns them into ordinary Go: models, query functions, a router, handler
interfaces, reverse-URL helpers and an HTTP client.

## The idea

Say each thing once, in the place that knows it best, and let an
integrated system prove that everything else agrees with it.

In a typical web stack one fact is written many times: the schema in
SQL, again in the models, the validators, the router, the client and
the migrations, with glue keeping the copies in step until one of them
drifts. Volt states the fact once, in a language made for it, and
derives the rest. The language, its checker, the generators, the
runtime and SQLite are designed as one system, so the interfaces
between them are small and a disagreement is a compile error, caught by
Volt's checker and then by Go's. The code a person writes is only the
decisions that are truly the program's own: the least code for the
most behaviour, and most of it generated so it cannot drift.

The trade is that Volt asks for a language and tools that work
differently from the usual stack. The thesis is that the leverage
repays it.

## What that gives

- **One source of truth.** The `.volt` file is the contract for SQLite,
  Go and the generated client.
- **Nothing hidden.** What runs is in a file you can open: no
  reflection, no registries, no base class to trace, nothing resolved
  at run time.
- **Values and effects at the edge.** A handler is a function from its
  inputs to a response value. Queries, writes and the reply are
  effects described as data and performed by generated code you can
  read, so an application is predictable, testable without a database
  and safe to run concurrently. Go enforces neither immutability nor
  purity, so Volt does: the generator produces values and `volt vet`
  keeps handlers pure.
- **Migrations you write.** Each one is a `migration` element written by
  hand. The checker folds the migrations against the declared schema
  and refuses the project when the two disagree, and
  `volt migration up` applies them in order. A tool that guesses a
  migration from a diff gets a rename wrong sooner or later, so Volt
  has none.
- **Fewer tokens.** A schema line stands for many lines of Go, for a
  person and for a model alike, and the generated Go is the same every
  time.
- **One binary, one database.** A Volt application is a Go binary with
  SQLite inside, and complexity joins the stack only when a real limit
  is reached.

## How the code is written

Volt's own code and the code it generates follow Volt style,
[`docs/VOLT_STYLE.md`](docs/VOLT_STYLE.md): TigerStyle, TigerBeetle's
engineering style, carried into Go and joined with the functional
rules, written to hold for any Go program. It puts safety first:
narrow types, a limit on everything, assertions that stay on in
production, no recursion, every error handled.
[`docs/assert.md`](docs/assert.md) shows how it asserts, in examples
that compile and pass the style gate.
[`docs/style_in_volt.md`](docs/style_in_volt.md) is how Volt applies the
style: its exceptions, its dependency guarantee, and the gate, a test in
the suite that checks the mechanical rules. The gate's ledger lists the
debt older than the style, which only shrinks, and
[`docs/plan.md`](docs/plan.md) orders the work that pays it off.

## Layout

| Directory | What it is |
|---|---|
| `*.go` (root) | the network runtime generated code links against |
| `cmd/volt` | the one binary: `check`, `vet`, `gen`, `routes`, `stress`, `migration`, `lsp`, `version` |
| `lang/` | the language: scanner, parser, checker, vet, conformance corpus |
| `gen/` | the generators |
| `nao/` | the data layer: naming, generation, the value runtime `rt`, the runner `run` |
| `assert/` | assertions that panic and stay on in production (`docs/assert.md`) |
| `checked/` | integer arithmetic and conversions that report when they would wrap (volt_style/arithmetic-that-cannot-wrap) |
| `tint/` | a `slog.Handler` that writes colored lines, forked from lmittmann/tint under its MIT license (spec/license-and-provenance) |
| `lsp/` | the language server, with its own LSP protocol layer `lsp/protocol` on the standard library |
| `grammar/`, `zed-extension/` | the editor stack |
| `itest/` | the fixture application served over `httptest`, drift-checked against the generator |
| `docs/` | `spec.md`, `VOLT_STYLE.md`, `style_in_volt.md`, `assert.md`, `lint.md`, `editor.md`, `runner.md`, `roadmap.md`, `plan.md` |
