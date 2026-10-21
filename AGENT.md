# AGENT.md — how to work on Volt

> [!CAUTION]
> **This project is an alpha version, and everything may change in the future.**

Volt is a codegen-first web framework for Go. Schema and routes are
declared in `.volt` files; `volt gen` writes models, queries, DDL, a
router, controller interfaces, reverse-URL helpers and a client as
plain Go on stdlib `net/http`. One maintainer, no users, no backward
compatibility to protect.

**Nothing here is set in stone.** The design is still being searched
for. The docs describe what is implemented today so that spec, code
and tests stay consistent; they do not describe how the project must
be. Do not infer the design from what exists. When a better shape
appears, say so plainly and propose it; never work around the current
one silently.

## Docs

Five files in `docs/`, nothing else carries prose:

- `spec.md` — the language and the generated code, normative for the
  implementation as it stands. Spec and code must agree; the language
  changes by changing both in one commit. Every constraint has a
  conformance snippet in `lang/conformance/snippets/` tagged with the
  section it proves, the EBNF is executed against the parser, and
  every diagnostic cites the section it enforces as `spec/<anchor>`.
- `lint.md` — every vet rule with executable examples; a test fails
  the build if it drifts.
- `editor.md` — grammar, Zed extension, language server, and the
  verification checklist in its section 8.
- `hypotheses.md` — open design questions, numbered, held loosely.
- `roadmap.md` — what is built and what is not; a status changes in
  the commit that lands it.

No doc is ever knowingly wrong: a change whose blast radius reaches a
doc updates the doc in the same commit. Never weaken a test to get
green.

## Repository

Three Go modules under `go.work`: the library at the root (runtime,
`lang/`, `gen/`, `nao/`, `itest/`; stdlib only, sqlite3 test-only) and
the tools `lsp/` and `cmd/volt/`, which carry the third-party
dependencies; `cmd/volt` carries the `mattn/go-sqlite3` driver for the
fold and `volt migration up`, so the tool is built with cgo (a C
compiler on the path; without it the driver is a stub and `volt`
refuses with exit 2), while the library and the runtime register
none. Go floor is the newest stable Go, declared in all three
`go.mod` files. `grammar/`, `zed-extension/` and
`scripts/sync-grammar.sh` are the editor stack. `go test ./...` at the
root does not cover the tool modules.

## Verification bar, all of it, before claiming done

```sh
go build ./... ./lsp/... ./cmd/volt/...
go vet   ./... ./lsp/... ./cmd/volt/...
go test  ./... ./lsp/... ./cmd/volt/...
go test  -race ./lang/... ./lsp/...
gofmt -l . | grep -v zed-extension        # must be empty
```

Golden refresh is `go test ./gen/router -update` and `go test
./nao/gen/... -update`; read the diff, goldens exist to surface
unintended output changes. Grammar changes also need `tree-sitter
generate`, `tree-sitter test`, the checklist in `editor.md` section 8
and `./scripts/sync-grammar.sh`. Anything interactive in the language
server gets a scripted stdio session; one is a golden
(`cmd/volt/lsp_session_test.go`).

## Conventions

- Names are subject-first, verb-last: `UserGet`, `UserChangeset`,
  `EOrderStatusPending`; internals too. Carve-outs: Go-mandated
  interface methods, `New*`/`is*`/`has*`, stdlib-parallel APIs, cursor
  primitives, LSP handlers named after protocol methods.
- The only comments are doc comments on exported identifiers, one or
  two sentences citing the spec section (`§columns`), and directives.
  Nothing else is commented, generated code included.
- Generated files start with the `Code generated … DO NOT EDIT.`
  marker; the CLI refuses to overwrite a file without it. Generated
  code imports the standard library, Volt's runtime and the packages of declared types only.
- SQLite is the only database. Never: callbacks, lazy loading, dirty
  tracking, reflection in generated code (spec §non-goals).
- A memoized computation is a pure function of explicit inputs, keyed
  on those inputs themselves; `cmd/volt`'s purity gate and
  immutability walk enforce it, and a new memo is added to the gate's
  targets in the same commit.
- Test data is neutral (blog and metrics domains, never agriculture).
  A test that needs a project of some size writes one with
  `internal/corpus`.
- Commit messages explain why.

## Working style

Design in the spec before building anything big. Integration tests
over mocks for the data layer, an exhaustive corpus for the language,
goldens for the generators. Be direct: state what failed, do not
oversell. When the maintainer asks a question, answer it and stop.
When they ask for work, finish it: build, vet, test and gofmt green,
committed, pushed.
