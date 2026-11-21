# Working on Volt

> [!CAUTION]
> **This project is an alpha version, and everything may change in the future.**

Volt is a codegen-first web framework for Go. Schema, queries, routes
and migrations are declared in `.volt` files; `volt gen` writes models,
queries, DDL, a router, controller interfaces, reverse-URL helpers and
a client as plain Go on the standard library's `net/http`. One
maintainer, no users, no backward compatibility to protect.

Its essence: say each thing once, in the place that knows it best, and
let an integrated system prove that everything else agrees with it.
The language, its checker, the generators, the runtime and SQLite are
designed as one system, so the interfaces between them are small and
checked at compile time, and the code a person writes is only the
decisions that are truly the program's own. The least code for the
most behaviour, and most of that code generated so it cannot drift.

Nothing here is set in stone. The docs describe what is built today, so
that spec, code and tests agree; they do not say what Volt must become.
The design is not inferred from what exists. When a better shape
appears, the agent says so and proposes it, rather than working around
the current one.

## Read first

Before talking with the maintainer or writing code:

- this file;
- `docs/VOLT_STYLE.md`: Volt style, how every line of Go, Volt and SQL
  is written, here and in any project;
- `docs/style_in_volt.md`: how this repository applies it: each rule
  as it meets Volt's code, the debt Volt still carries, Volt's
  exceptions, its dependency guarantee, and the style gate,
  `cmd/volt/style_test.go`, whose ledger holds the debt older than the
  style and only shrinks.

When the work touches them:

- `docs/spec.md`, before changing the language or the generated code:
  the sections the change touches. It is normative for what is built.
  The language changes by changing spec and code in one commit. Every
  constraint has a conformance snippet in `lang/conformance/snippets/`
  tagged with the section it proves, the EBNF runs against the parser,
  and every diagnostic cites the section it enforces.
- `docs/assert.md`, before writing an assertion.
- `docs/runner.md`, before changing `nao/run`: how it meets SQLite, what
  was measured, what is open.
- `docs/lint.md`, before changing a vet rule.
- `docs/editor.md`, before changing the grammar, the Zed extension or
  the language server, with editor/the-verification-checklist.
- `docs/plan.md` and `docs/roadmap.md`, before picking, landing or
  deferring a step. A step deletes its section in the commit that lands
  it, and a deferred step keeps one line; a roadmap status changes in
  the commit that lands it.

## The method

Go slow to go fast: a day of design saves weeks in production, and code
is written once and read and run many times. The agent's effort goes to
understanding the problem, to the design, to assertions and to the
questions that make the result hold, and it writes the least code that
does the job. Every task, in this order:

1. The problem: what is wrong, how it is known, and what done means;
   asked of the maintainer when it is unclear.
2. The design: the smallest change, the narrowest types, the limits,
   the assertions, and a sketch of what it costs; for anything big, in
   the spec first.
3. The decisions: the architecture is the maintainer's, the
   implementation inside its boundaries the agent's: modules are black
   boxes (volt_style/modules-are-black-boxes), their interfaces are the
   maintainer's, and the agent builds what lies behind one. How the parts
   compose, where a boundary lies and what crosses it go to the
   maintainer before any code, with options and a recommendation, and
   so do a limit's value, a change of behaviour, a new dependency or
   tool, a gate widened and work beyond the task. Inside an agreed
   boundary the agent decides, and the report names what it decided.
4. The build: the minimum code, the verification bar, committed and
   pushed.
5. The report: what changed and why, what was decided and by whom,
   what remains, and the concerns. Short.

A question from the maintainer gets its answer, and the answer ends
there. The agent states what failed, plainly.

## The tool belt

Chosen from the bottom up, each tool for what it does best:

- Go and `gofmt`: every program the agent writes to change, analyse or
  check the code.
- `git`: history, commits and pushes.
- `grep`, `find`, `ls`, `cat`, `head`, `diff`, `cmp`, `wc`, `file` and
  `iconv`: one look at the code.
- SQLite, through a Go program and mattn, when the data outgrows plain
  files (When SQLite, below).
- For the editor stack, the tree-sitter CLI that `npm ci` pins, through
  `go run ./scripts/sync-grammar`.

The shell starts one program at a time. A task that takes more than one
look is one integrated Go program, built and run from the shell, that
reads, decides and reports by itself, over concrete types and the
libraries made for the problem: `go/parser` and `go/types` for Go,
Volt's own front end for Volt, `database/sql` and mattn for SQLite,
never a pattern over text. Glue written this way is checked by the
compiler, which tells whether it fits the libraries at all, and Volt
style makes it right from its first line; a chain of tools passes text
it must reparse at every seam, and the combinations of their flags,
formats and quoting multiply. A small interface is a good one.

A file changes by exact text replacement, or by such a program, shown
to the maintainer before it runs. A program the agent keeps lives in
the repository: a command under `cmd/`, built from modules under
`internal/`, so every session and every agent reuses it. Code the agent writes that is more
than trivial use of the standard library, and that a later task would
write again, is proposed as a module: a tiny black box behind the
smallest interface that does the job, tested once and reused by every
session (volt_style/modules-are-black-boxes). A tool beyond the belt is proposed with the problem it
solves, what staying with the belt would cost in lines, hours and run
time, and what the tool gains; the maintainer decides, and the belt
grows by that one tool.

### When SQLite

Plain files hold what is read once, in order, and found with `grep`.
SQLite holds data that:

- is relational: records that refer to each other and are joined, such
  as every citation and the heading it names, or every function and
  the findings against it;
- is asked many questions: grouped, counted, ranked, compared across
  runs, with aggregates and window functions;
- is searched as text: FTS5 ranks matches with bm25, built into mattn
  by the tag `sqlite_fts5`;
- is JSON to take apart or build, with the JSON functions;
- outlives one program: a database file in the scratchpad, kept between
  the agent's runs.

It is used as Volt uses it: a file on disk, `STRICT` tables, the pragmas
set explicitly (`journal_mode`, `synchronous`, `foreign_keys`), one
writer, and a bulk load in one transaction.

## The repository

Three Go modules under `go.work`:

- the root: the network runtime, `lang/`, `gen/`, `nao/` with the value
  runtime `nao/rt` and the runner `nao/run`, `assert/`, `checked/`, `tint/` and
  `itest/`; standard library only, with mattn for its tests;
- `lsp/`: the language server, standard library only, with its own
  protocol layer `lsp/protocol`;
- `cmd/volt/`: the one binary, which carries the third-party
  dependencies, mattn among them for the fold and `volt migration up`;
  the library and the runtime register no driver.

`go test ./...` at the root does not cover the other two modules. The Go
floor is the newest stable Go, declared in all three `go.mod` files.
Generated code is committed only where it is a golden: the generators'
testdata and the `itest/` and `nao/itest/` fixtures.

The stack is Go, Volt and SQLite. Code, tests and tools are Go; the only
program a test or a tool runs is `go`, or a `volt` it built; no check
skips itself when a tool is missing, and a check that seems to need
another tool is written in Go against mattn. The database is SQLite through
`mattn/go-sqlite3`, in tests as in production: a test opens a file in
its temporary directory through `run.Open` (`internal/dbtest`), and
`nao/rt`'s own tests, below `nao/run`, open it through `database/sql`;
an in-memory database is refused. mattn is chosen and settled: it wraps
SQLite's C library through cgo, so a C compiler on the path is a
required and accepted part of building Volt, never raised as a cost or
a risk, and a pure-Go driver is not wanted. Without a C compiler the
driver is a stub and `volt` refuses the fold and the migration commands
with exit 2.

The editor stack is the one place other languages live, because the
editors dictate them: `grammar/` is a tree-sitter grammar (`grammar.js`,
the C parser generated from it, the CLI `npm ci` pins); `zed-extension/`
is Rust, TOML and tree-sitter queries, loading its grammar from a git
mirror; `scripts/sync-grammar`, in Go, runs `git` and that CLI, and
`scripts/sync-grammar.sh` is a one-line wrapper kept until the
extension is published. One violation remains: `internal/perfsweep`
runs `git`, `uname`, `sysctl` and `sw_vers`, which
plan/no-external-tools-in-the-performance-report removes.

## Conventions

- Code follows `docs/VOLT_STYLE.md`, hand-written and generated alike.
- Names are subject first, verb last (`UserGet`, `UserChangeset`,
  `OrderStateKnown`), qualifiers and units last (`queueLengthMax`),
  internals too (volt_style/go-casing-big-endian-order).
- Comments are the maintainer's. An agent writes none in Go or Volt
  source, doc comments included, apart from directives, the generated
  header, the `// spec/…` tag of a conformance snippet, and the doc
  comments a fixture or snippet needs to test them. The maintainer's
  are doc comments on exported identifiers, one or two sentences citing
  the spec. A generated file carries the header and its schema's doc
  comments (spec/doc-comments-in-generated-code). An agent's reasons go
  into the spec, `runner.md` or `plan.md` section its change touches.
- A generated file starts with the `Code generated … DO NOT EDIT.`
  marker, and the CLI refuses to overwrite a file without it. Generated
  data files import the standard library, `nao/rt` and the packages of
  declared types; route files add the root `volt` package, `nao/run`
  and the data packages they route.
- Values, and effects at the edge (spec/values-and-effects,
  volt_style/the-functional-rules). The front end and the generators
  touch no file and no database: a loader reads through `lang/source`,
  and a generator returns a map that `volt gen` writes.
- A memoized computation is a pure function of explicit inputs, keyed
  on those inputs; `cmd/volt`'s purity gate and immutability walk hold
  it, and a new memo joins the gate's targets in the same commit.
- Test data is neutral: blog and metrics domains. A test that needs a
  project of some size writes one with `internal/corpus`. Integration
  tests serve the data layer, an exhaustive corpus the language, and
  goldens the generators.
- A commit message is `.`.

## Docs

No doc is ever knowingly wrong: a change whose reach includes a doc
updates the doc in the same commit. A test is never weakened to get
green.

Every section, rule and step is a heading with a name, never a number.
A citation is the doc's name, a slash and the heading's anchor (the
heading lowercased, its words joined by hyphens): `spec/columns`,
`volt_style/owned-state-only`, `plan/the-assert-package`,
`runner/two-pools`, `editor/the-verification-checklist`,
`lint/unusedpartial`, `assert/checked-arithmetic`. It is written the same
way in docs, comments, conformance tags, diagnostics and messages. A
new heading gets a name no other heading of its doc has, and renaming
one renames every citation of it in the same commit.
`lang/spec_doc_test.go` fails the build on a citation that names no
heading, on a numbered heading or citation, on the section sign, and on
a tab in a doc.

## The verification bar

All of it, before a task is done:

```sh
go build ./... ./lsp/... ./cmd/volt/...
go vet   ./... ./lsp/... ./cmd/volt/...
go test  ./... ./lsp/... ./cmd/volt/...
go test  -race ./lang/... ./lsp/... ./nao/... .
gofmt -l . | grep -v zed-extension        # must be empty
```

Goldens refresh with `go test ./gen/router -update` and
`go test ./nao/gen/... -update`, and their diff is read: goldens exist
to surface output that changed by accident. The style ledger is lowered
with `go test ./cmd/volt -run TestStyleLedger -update`, which never
raises it; debt the code adds is fixed, not recorded. A grammar change
also needs `tree-sitter generate`, `tree-sitter test`,
editor/the-verification-checklist and `go run ./scripts/sync-grammar`.
Anything interactive in the language server gets a scripted stdio
session; one is a golden (`cmd/volt/lsp_session_test.go`).
