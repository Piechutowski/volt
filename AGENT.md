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

Seven files in `docs/` and the README, nothing else carries prose:

- `spec.md` — the language and the generated code, normative for the
  implementation as it stands. Spec and code must agree; the language
  changes by changing both in one commit. Every constraint has a
  conformance snippet in `lang/conformance/snippets/` tagged with the
  section it proves, the EBNF is executed against the parser, and
  every diagnostic cites the section it enforces as `spec/<anchor>`.
- `style.md`: how Volt's code is written, TigerStyle translated to Go
  joined with the functional rules, normative for every line of the
  repository and every line `volt gen` writes. The rules marked gate
  are checked by `cmd/volt/style_test.go`, whose ledger lists the debt
  older than the style and only shrinks; its examples are executable.
- `lint.md` — every vet rule with executable examples; a test fails
  the build if it drifts.
- `editor.md` — grammar, Zed extension, language server, and the
  verification checklist in its section 8.
- `runner.md`: how `nao/run` meets SQLite: the two pools, which
  connection an effect runs on, prepared statements, the bulk insert,
  what was measured, the questions still open, and the note on batching
  writes, which is not built; the rules themselves stay normative in
  `spec.md`.
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

Three Go modules under `go.work`: the library at the root (runtime,
`lang/`, `gen/`, `nao/` with the value runtime `nao/rt` and the runner
`nao/run`, `itest/`; stdlib only, sqlite3 test-only) and the tools
`lsp/` (stdlib only, its LSP layer `lsp/protocol` its own) and
`cmd/volt/`, which carries the third-party dependencies; it carries the `mattn/go-sqlite3` driver for the fold and
`volt migration up`, so the tool is built with cgo (a C compiler on the
path; without it the driver is a stub and `volt` refuses the fold and
the migration commands with exit 2), while the library and the runtime
register none. Generated code is committed only where it is a golden:
the generator testdata and the `itest/` and `nao/itest/` fixtures. Go
floor is the newest stable Go, declared in all three `go.mod` files.
`grammar/`, `zed-extension/`, `scripts/sync-grammar` and its wrapper
`scripts/sync-grammar.sh` are the editor stack.
`go test ./...` at the root does not cover the tool modules.

## Stack: Go, Volt and SQLite, nothing else

Code, tests, tools and scripts are Go. The database is SQLite through
`mattn/go-sqlite3`, in tests as in production: a test opens a file in
its temporary directory through `run.Open` (`internal/dbtest`), never
an in-memory database, which `run.Open` refuses; `nao/rt`'s own tests,
below `nao/run`, open the file through `database/sql`. The only
program a test or tool runs is `go`, or a `volt` it built. No Python, no
shell scripts, no other SQLite driver or engine, no other runtime, and
no check that skips itself when a tool is missing: a check that seems
to need another tool is written in Go against mattn.

mattn is the driver, chosen and settled. It wraps SQLite's C library
through cgo, so a C compiler is a required and accepted dependency of
building and testing Volt. A pure-Go driver is not wanted; do not
raise the cgo dependency as a cost or a risk.

The one exception is the editor stack, whose languages the editors
dictate. `grammar/` is a tree-sitter grammar: `grammar.js`, the C
parser generated from it, and the tree-sitter CLI that `npm ci` pins.
`zed-extension/` is a Zed extension: Rust, TOML and tree-sitter queries,
with its grammar loaded from a git mirror. `scripts/sync-grammar`,
written in Go, runs `git` and that CLI for them;
`scripts/sync-grammar.sh` is a one-line wrapper that runs it, kept for
convenience until the Zed extension is published. Nothing outside
those directories uses any of it.

One violation remains: `internal/perfsweep` runs `git`, `uname`,
`sysctl` and `sw_vers` to describe the machine; `plan.md` step 20
removes them.

## Verification bar, all of it, before claiming done

```sh
go build ./... ./lsp/... ./cmd/volt/...
go vet   ./... ./lsp/... ./cmd/volt/...
go test  ./... ./lsp/... ./cmd/volt/...
go test  -race ./lang/... ./lsp/...
gofmt -l . | grep -v zed-extension        # must be empty
```

Golden refresh is `go test ./gen/router -update` and `go test
./nao/gen/... -update`; read the diff, goldens
exist to surface unintended output changes. The style ledger is lowered
with `go test ./cmd/volt -run TestStyleLedger -update`, which never
raises it; debt the code adds is fixed, not recorded. Grammar changes also need
`tree-sitter generate`, `tree-sitter test`, the checklist in
`editor.md` section 8 and `go run ./scripts/sync-grammar`. Anything
interactive in the language server gets a scripted stdio session; one
is a golden (`cmd/volt/lsp_session_test.go`).

## Conventions

- Code follows `docs/style.md`: TigerStyle in Go and the functional
  rules, for hand-written and generated code alike. A rule marked gate
  is checked by the style gate; the others are held in review.
- Names are subject-first, verb-last: `UserGet`, `UserChangeset`,
  `OrderStateKnown`; internals too. Carve-outs: Go-mandated
  interface methods, `New*`/`is*`/`has*`, stdlib-parallel APIs, cursor
  primitives, LSP handlers named after protocol methods. Qualifiers and
  units go last (`delayMsMax`, `style.md` N1).
- Comments are the maintainer's: for now an agent writes none, doc
  comments included (`style.md` C1). The comments there are doc
  comments on exported identifiers, one or two sentences citing the
  spec section (`§columns`), and directives. Nothing else is commented;
  a generated file carries the header and the doc comments of its
  schema (spec §doc-comments-in-generated-code) and nothing else.
- Generated files start with the `Code generated … DO NOT EDIT.`
  marker; the CLI refuses to overwrite a file without it. Generated
  data files import the standard library, `nao/rt` and the packages of
  declared types only; route files add the root `volt` package, the
  runner `nao/run` and the data packages they route.
- SQLite is the only database. Values and effects at the edge (spec
  §values-and-effects, `docs/style.md` section 2).
- A memoized computation is a pure function of explicit inputs, keyed
  on those inputs themselves; `cmd/volt`'s purity gate and
  immutability walk enforce it, and a new memo is added to the gate's
  targets in the same commit. The front end and the generators touch
  no file and no database: a loader reads through `lang/source` and a
  generator returns a map `volt gen` writes (spec §values-and-effects).
- Test data is neutral (blog and metrics domains, never agriculture).
  A test that needs a project of some size writes one with
  `internal/corpus`.
- A commit message is `.`, nothing else.

## Working style

Design in the spec before building anything big. Integration tests
over mocks for the data layer, an exhaustive corpus for the language,
goldens for the generators. Be direct: state what failed, do not
oversell. When the maintainer asks a question, answer it and stop.
When they ask for work, finish it: build, vet, test and gofmt green,
committed, pushed.
