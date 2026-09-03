# AGENT.md — how to work on Volt

You are working on **Volt**: a codegen-first web framework for Go.
Schema and routes are declared in `.volt` files (one language, grown
outward from DBML); `volt gen` writes the models, queries, DDL, router,
controller interfaces and reverse-URL helpers as plain Go on stdlib
`net/http`. Personal project of one maintainer, alpha: no users, no
backward compatibility to protect — when a better shape appears,
propose it and restructure boldly. Locked decisions still bind until
edited.

## The doctrine (read this even if you read nothing else)

**The spec is the software; the code is its proof.** Volt is built the
way Go stdlib developers work against an RFC: `docs/spec.md` is
normative — EBNF plus numbered constraints plus examples — and the
implementation MUST be 100% compliant. Every constraint is executable
(conformance corpus, `// spec: §…` tags; valid MUST pass, invalid MUST
fail); every diagnostic cites the section it enforces; the grammar,
the LSP, vet and the generators are all audited against the same
document (`docs/editor.md` §8 is the checklist). When spec and code
disagree, one of them has a bug — the fixing commit says which. Never
change behavior without changing the spec in the same commit, and
never weaken a test to get green.

**No doc is ever knowingly wrong (D49).** Ten maintained docs live in
`docs/` — spec, lint, editor, decisions, hypotheses, roadmap, backlog,
nao, model-layer, reference/ — and no other directory carries prose
docs (root README.md/AGENT.md are the named exceptions, D49).
Operational detail (paths, commands, code snippets) may appear only in
docs a test or the compiler keeps honest; philosophy is written at an
altitude where nothing can rot. The accepted cost, chosen deliberately:
**every change's blast radius includes the docs** — an API rename
sweeps `nao.md` and the spec's examples in the same commit. That is
the correct price for a project whose thesis is "one source of truth,
everything else derived"; the normative docs already pay it via tests
(`lang/vet/docs_test.go` fails the build if `lint.md` drifts).

**Vocabulary.** Two names: **DBML** is the schema core
(spec Part I, kept diagrammable on dbdiagram.io); **Volt** is the
language — everything, extensions included. The word "EDBML" is
retired; do not reintroduce it. There is no `.edbml` file type;
extension-exercising fixtures are `.volt`, plain `.dbml` stays
supported as input.

**nao is a sub-library, not a product.** `nao/` is Volt's ORM the
way ActiveRecord is Rails'. It has no separate go.mod, no separate CLI
(one binary, `volt`), no separate identity in docs beyond
`docs/nao.md`. Don't "helpfully" rename it to `orm/` (tried once,
reverted: nao is its *name*).

## Read in this order

1. `docs/decisions.md` — **the law** (numbered with gaps — removed
   decisions retire their numbers). Never contradict one
   silently; changing a decision means editing that file in the same
   commit. New session-level rulings get appended there. What is
   *not yet* decided lives in `docs/hypotheses.md` — working beliefs,
   numbered, graduated or killed by practice; never treat one as law,
   and put new open questions there, not in decisions.
2. `docs/roadmap.md` — **the build plan**: stable IDs, statuses
   (updated in the landing commit). Current priority: FW-2 (Datasets;
   unblocked — FW-1's structured Group/Pred/Select layer landed
   2026-09-01, raw-SQL Select/View blocks remain).
3. `docs/spec.md` — the language: Part I the schema core, Part II
   the project, routing and query layers, then the appendices (Go and
   SQLite mappings, DBML compatibility). Sections are named, not
   numbered (D64); existing `§…` citations resolve through the spec's
   Citation key until the migration in `docs/backlog.md` lands.
   `docs/backlog.md` — scheduled chores with the reason each waits.
4. `docs/editor.md` — grammar/extension/LSP architecture, extension
   recipes, and the §8 verification checklist.
5. `docs/nao.md` + `docs/model-layer.md` — the data layer and the
   philosophy behind it. `docs/reference/` — pinned research
   (framework inventories, ORM matrix): consult, don't maintain.

## Repository shape

Three Go modules, stitched by `go.work`: the **library** at the
root — runtime (`*.go`), `lang/` (token/scanner/parser/ast/check/vet +
project semantics), `gen/router`, `gen/model`, `nao/` (rt, gen,
inflect, itest), `itest/` — stdlib-only (sqlite3 is
test-only); and the **tools** `lsp/` and `cmd/volt/`, which carry the
third-party deps (glsp, urfave/cli). Go floor: **1.27** (D45),
declared in all three go.mod files and go.work. `grammar/` +
`zed-extension/` + `scripts/sync-grammar.sh` are the editor stack.

Consequences that bite: `go test ./...` at the root does NOT cover the
tool modules — the full sweep is
`go test ./... ./lsp/... ./cmd/volt/...`. Tools that lag the toolchain
(govulncheck) need `GOTOOLCHAIN=go1.27.0 go run …`.

## Verification bar (all of it, before claiming done)

```sh
go build ./... ./lsp/... ./cmd/volt/...
go vet   ./... ./lsp/... ./cmd/volt/...
go test  ./... ./lsp/... ./cmd/volt/...   # conformance, goldens, itests, LSP
gofmt -l . | grep -v zed-extension        # must be empty
```

Golden refresh is `go test ./gen/router -update` / `go test ./nao/gen/...
-update` — inspect the diff; goldens exist to surface unintended output
changes. Grammar changes additionally need `tree-sitter generate`,
`tree-sitter test`, the differential checks of `docs/editor.md` §8, and
`./scripts/sync-grammar.sh`. For LSP behavior, scripted stdio JSON-RPC
sessions replaying the user's exact keystrokes have repeatedly caught
what unit tests missed — do that for anything interactive.

## Conventions

- Naming is subject-first, verb-last everywhere, hand-written code
  included: `UserGet`, `UserCreateParams`, `EOrderStatusPending` (D09);
  internals too (`columnCheck`, `tableEmit`). Carve-outs: Go-mandated
  interface methods, `New*`/`is*`/`has*`, stdlib-parallel APIs
  (`parser.ParseFile`, `check.File`), cursor primitives (`next`,
  `peek`), LSP handlers named after protocol methods.
- Error messages and code comments cite spec sections (`§6.7`, `§V4.6`)
  or decision numbers. Generated files start with the
  `Code generated … DO NOT EDIT.` marker; the CLI refuses to overwrite
  files lacking it. Zero runtime dependencies in generated code (D03).
- SQLite-first, all-in (D02); SQLite is the gen-time SQL checker via
  in-memory prepare (D06). Never: callbacks, lazy loading, dirty
  tracking, reflection in generated paths (D27).
- Commit messages explain *why*; a session that produces new decisions
  appends them to `docs/decisions.md` before committing.

## Working style the maintainer expects

Plan before building (design in the roadmap/spec first for big
features). Integration tests over mocks for the data layer; exhaustive
unit corpus for language code; golden files for generators. Be direct
and honest in reports — state what failed plainly, don't oversell.
The maintainer reads code and specs fluently; explain reasoning, not
basics. Don't write for an imagined audience: README and docs are for
the maintainer (it's alpha; honesty over marketing). Test data must be
neutral (blog/metrics domains — never agriculture/FADN; that fixture
history is why). When the maintainer asks a question, answer it and
stop — don't implement until asked; when they ask for work, finish it:
build + vet + test + gofmt green, committed, pushed.
