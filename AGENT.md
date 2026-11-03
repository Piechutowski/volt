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

Five files in `docs/` and the README, nothing else carries prose:

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
- `roadmap.md` — what is built and what is not; a status changes in
  the commit that lands it.
- `plan.md` — the one forward-looking document: the decisions made
  and the steps that remain, in order; a step deletes its section in
  the commit that lands it, and a deferred step keeps one line saying
  so.

No doc is ever knowingly wrong: a change whose blast radius reaches a
doc updates the doc in the same commit. Never weaken a test to get
green.

## Repository

Four Go modules under `go.work`: the library at the root (runtime,
`lang/`, `gen/`, `nao/` with the value runtime `nao/rt` and the runner
`nao/run`, `itest/`; stdlib only, sqlite3 test-only), the tools `lsp/`
and `cmd/volt/`, and the example application `examples/voltmeter/`,
which carry the third-party dependencies; `cmd/volt` carries the
`mattn/go-sqlite3` driver for the fold and `volt migration up`, so the
tool is built with cgo (a C compiler on the path; without it the
driver is a stub and `volt` refuses the fold and the migration
commands with exit 2), while the library and the runtime register
none. `examples/voltmeter` is Voltmeter, the example application and
end-to-end test, built on the same driver with cgo; its generated
files are committed and drift-checked by its own tests, its CLI leg
builds `volt` from the checkout and runs it over the example, and its
coverage map (`examples/voltmeter/e2e/coverage_test.go`) maps every
heading of the spec's Parts I and II and its appendices to the
Voltmeter tests that prove it, with the rules of it they leave
unexercised, or a one-line reason it is excluded, so a new heading
needs an entry; the map is a reviewed claim, since the test checks
that the named tests exist, not what they exercise. Its desktop
client `examples/voltmeter/gui` is guigui on Ebitengine, so building
that module also needs the X11 and OpenGL development headers
Ebitengine needs on Linux, and the window tests in
`examples/voltmeter/e2e` run the client on a virtual X server, the
smoke test under `xvfb-run` and the handler tests under `Xvfb` with
clicks and key presses sent through `libXtst`, each skipping with a
message when what it needs is missing. Go floor is the
newest stable Go, declared in all four `go.mod` files. `grammar/`,
`zed-extension/` and `scripts/sync-grammar.sh` are the editor stack.
`go test ./...` at the root does not cover the tool modules or the
example.

## Verification bar, all of it, before claiming done

```sh
go build ./... ./lsp/... ./cmd/volt/... ./examples/voltmeter/...
go vet   ./... ./lsp/... ./cmd/volt/... ./examples/voltmeter/...
go test  ./... ./lsp/... ./cmd/volt/... ./examples/voltmeter/...
go test  -race ./lang/... ./lsp/...
gofmt -l . | grep -v zed-extension        # must be empty
```

Golden refresh is `go test ./gen/router -update`, `go test
./nao/gen/... -update` and, for Voltmeter's route table, `go test
./examples/voltmeter/e2e -run TestCLI -update`; read the diff, goldens
exist to surface unintended output changes. Grammar changes also need
`tree-sitter generate`, `tree-sitter test`, the checklist in
`editor.md` section 8 and `./scripts/sync-grammar.sh`. Anything
interactive in the language server gets a scripted stdio session; one
is a golden (`cmd/volt/lsp_session_test.go`).

## Conventions

- Names are subject-first, verb-last: `UserGet`, `UserChangeset`,
  `OrderStateKnown`; internals too. Carve-outs: Go-mandated
  interface methods, `New*`/`is*`/`has*`, stdlib-parallel APIs, cursor
  primitives, LSP handlers named after protocol methods.
- The only comments are doc comments on exported identifiers, one or
  two sentences citing the spec section (`§columns`), and directives.
  Nothing else is commented; a generated file carries the header and
  the doc comments its notes become (spec §notes-become-doc-comments)
  and nothing else.
- Generated files start with the `Code generated … DO NOT EDIT.`
  marker; the CLI refuses to overwrite a file without it. Generated
  data files import the standard library, `nao/rt` and the packages of
  declared types only; route files add the root `volt` package, the
  runner `nao/run` and the data packages they route.
- SQLite is the only database. Values and effects at the edge (spec
  §values-and-effects, `docs/plan.md` part 0).
- A memoized computation is a pure function of explicit inputs, keyed
  on those inputs themselves; `cmd/volt`'s purity gate and
  immutability walk enforce it, and a new memo is added to the gate's
  targets in the same commit. The front end and the generators touch
  no file and no database: a loader reads through `lang/source` and a
  generator returns a map `volt gen` writes (spec §values-and-effects).
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
