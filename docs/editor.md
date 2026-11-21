# Editor tooling: the grammar, the Zed extension, the language server

> [!CAUTION]
> **This project is an alpha version, and everything may change in the future.**

How Volt editor support is put together: three independently testable
components, the decisions that shape each, and, in
editor/the-verification-checklist, the checks that hold all of it to
[`spec.md`](spec.md). What each component does in detail is pinned by
its tests, not described here.

## The big picture

```
                        ┌────────────────────── Zed process ──────────────────────┐
                        │                                                         │
   grammar/             │   ┌───────────────┐        ┌───────────────────────┐    │
   grammar.js ──generate──▶ │ parser.c→WASM │        │ extension WASM (glue) │    │
                        │   │  incremental  │        │  zed-extension/src/   │    │
                        │   │  syntax tree  │        │  "how do I launch     │    │
                        │   └──────┬────────┘        │   volt lsp?"          │    │
                        │          │ tree            └────────┬──────────────┘    │
                        │          ▼                          │ spawns            │
                        │   .scm queries                      │                   │
                        │   highlights / outline /            │                   │
                        │   indents / brackets / injections   │                   │
                        └─────────────────────────────────────┼───────────────────┘
                                                              ▼
                                    ┌──────────────────── volt lsp (Go) ────────┐
                                    │  JSON-RPC over stdin/stdout (LSP 3.16)     │
                                    │                                            │
                                    │  scanner → parser → check → vet            │
                                    │  + lang.LoadOverlay (whole project,        │
                                    │    open buffers overlaid over disk)        │
                                    │  + the occurrence/symbol indexes powering  │
                                    │    def / refs / rename / hover / complete  │
                                    └────────────────────────────────────────────┘
```

1. **The grammar** (`grammar/`): a *syntactic*, error-tolerant,
   incremental parser compiled to WebAssembly and run inside Zed on
   every keystroke. It powers everything visual: colors, outline,
   indent, bracket matching.
2. **The extension** (`zed-extension/`): declarative files (manifest,
   language config, queries) plus about 70 lines of Rust whose only job
   is telling Zed how to launch `volt lsp`.
3. **The language server** (`lsp/`, served by `volt lsp`, its own Go
   module on the standard library alone): a *semantic*, spec-exact
   analyzer speaking LSP over stdio.

### Why two parsers?

The grammar and the server both parse Volt, deliberately, because their
requirements are opposite:

| | tree-sitter grammar | Go front end |
|---|---|---|
| Runs | inside the editor, every keystroke | separate process, per change |
| Must be | fast, incremental, **error-tolerant** | **correct**, the reference implementation of spec.md |
| Output | concrete syntax tree | AST + semantic model + diagnostics |
| Knows | shape only | meaning: resolution, partial injection, route conflicts |

The differential checks of editor/the-verification-checklist catch the
two disagreeing. When they do, **the front end is right by definition**.

The front end is layered like the Go toolchain it mirrors, and the
language server reuses it whole, so an error in Zed is by construction
the error `volt check` reports:

```
lang/token     lexical vocabulary (Kind, Token, Position with byte Offset)
lang/scanner   text → tokens                  (lexical errors, spec/lexical-structure)
lang/parser    tokens → ast.File              (syntax errors, Part I and Part II of the spec)
lang/check     ast.File → check.Info          (schema semantics, spec/static-semantics)
lang           project Load/Check             (packages, imports, routes, Part II of the spec)
lang/vet       lints                          (lint.md; legal but suspicious)
lsp            all of the above → LSP
```

## The grammar

In `grammar/`, tree-sitter compiles `grammar.js` into a table-driven
parser in C (`src/parser.c`, committed, because Zed builds the WASM
from it). The patterns it rests on:

- **Explicit newlines, not `extras`** (spec/line-structure-and-whitespace).
  Volt is newline-sensitive, so `extras` holds only `[ \t\r]` and
  comments, and every body uses one `newlineSep1` helper. A settings
  list takes line breaks inside its brackets; the other continuations
  the spec allows are limitations (editor/known-limitations).
- **Case-sensitive contextual keywords** (spec/keywords). Each keyword
  is a plain string in its one spelling, and tree-sitter's keyword
  extraction matches it only where the parse state admits it, so
  keywords are not reserved: `table` in a column's place is an
  identifier, `TABLE` at top level an ERROR node
  (spec/case-sensitivity-and-terminology). The words of a migration
  body (spec/migrations) appear only inside `migration_definition`, so
  `table t { up int }` is a column named `up`; the corpus case
  `migrations.txt` proves it.
- **Names are identifiers, strings are Go's** (spec/identifiers,
  spec/string-literals). A name is a Go identifier, never quoted, and
  a migration name is the one name that begins with digits; `"..."` and
  `"""..."""` are the strings, with the four escapes.
- **Split tokens for injection.** Delimiters and contents are separate
  tokens, so `injections.scm` hands the contents of a backtick
  expression, an `exec`'s SQL among them, to SQL.
- **One endpoint shape** (spec/relationships-ref). A ref endpoint is
  `table.column` or `table.(columns)` wherever it appears, so the
  grammar declares no conflicts.
- **Generic settings, open vocabulary** (spec/settings-lists). A
  settings list parses any `[name: value]`; which keys are legal where
  is the checker's business, so a new setting needs no grammar change.

## The Zed extension

In `zed-extension/`:

```
extension.toml               manifest: ids, [grammars.volt], [language_servers.volt]
Cargo.toml, src/lib.rs       Rust glue, compiled BY ZED to WASM at install time
languages/volt/
  config.toml                registration: name, suffix (volt), comments, brackets
  highlights.scm  outline.scm  indents.scm  brackets.scm  injections.scm
```

The glue implements `language_server_command`: the user setting
`lsp.volt.binary.path`, then `worktree.which("volt")`, and passes
`initialization_options` and settings through. There is no download
step: the extension is local-first, and `go install ./cmd/volt` is the
installer, with cgo (a C compiler on the path), since the tool carries
its own SQLite.

**The local grammar mirror** (`go run ./scripts/sync-grammar`): Zed
loads grammars from a *git repository*, even for dev extensions. The
tool regenerates `src/parser.c` if the pinned tree-sitter CLI is present
(editor/recipes-for-extending-the-system), mirrors `grammar/` into
`~/.cache/volt/tree-sitter-volt-git`, commits there, and rewrites
`[grammars.volt]` with the mirror's `file://` URL and commit. The
rewritten lines are machine-local; the committed `extension.toml`
carries a placeholder.

**Capture names** follow the fixed vocabulary Zed themes style
(`@keyword`, `@type`, `@property`, `@attribute`, …) by role, not by
node name: tables are `@type`, columns `@property`, setting names
`@attribute`. Later patterns win in a query file, so generic rules go
first.

## The language server

The decisions that shape `lsp/`:

- **Its own protocol layer.** `lsp/protocol` is written from the LSP
  3.16 and JSON-RPC 2.0 specifications and holds only what Volt uses,
  so the `lsp` module has no third-party dependency. Messages are
  handled one at a time in arrival order, which the document memos rely
  on, and every write is one locked write of header and body. A request
  the server declines, a refused rename, is answered `InternalError`,
  since 3.16 has no code for it. `lsp/protocol/jsonrpc_test.go` pins
  the framing, the message rules and the lifecycle.
- **Two passes per edit.** An edit updates the document's own front end
  at once: parse, single-file check and schema index, through the
  document's memos. It also schedules one background analysis of its
  project, the nearest `go.mod` (spec/projects-and-packages), once the
  edits settle for 75 ms: every open buffer overlaid over the disk,
  loaded and checked through the root's `lang.Session`, which caches
  parses by content and package results by their inputs, then the
  project index (`voltIndex`) built from the resolved model. Every open
  document of the project is published from that one run, with the
  version of the text it analyzed. A file outside any project gets the
  single-file pass alone.
- **Project layer first.** Definition, references, rename, hover and
  completion ask the project index, then the file's schema index.
- **Positions convert at the boundary.** The front end counts runes,
  LSP counts UTF-16 units; the conversion happens where a position
  enters or leaves the server, from the line's actual text.
- **A rename is checked.** It rewrites exactly the identifier, never a
  qualifier, validates the new name (spec/identifiers), and is refused,
  naming the error, when the project with the edits applied (the file
  alone outside a project) has an error it did not have; a rename at a
  Go reference skips the check.
- **vet speaks only on clean files.** Style advice stacked on errors is
  noise while typing: vet's warnings are published for a file only
  when it has no error, from the project analysis in a project and from
  the single-file pass outside one or in a directory the loader does not
  read. The single-file pass leaves out
  `modelname`.

The features themselves, the completion contexts, the hovers and the
rename rules, are pinned by `lsp/*_test.go` and by the scripted session
of editor/the-verification-checklist.

### Go references

A `.volt` file names Go functions and types of its package
(spec/projects-and-packages): plugs, validators, scope loads, error
handlers, controllers and declared types. The server reads them with
the standard library's Go parser over the package's non-test `.go`
files, without gopls or a build, and finds a qualified declared type's
package through `go list -find`, offline. A route's `Controller.Action`
resolves to the one type whose methods name every action the routes
give the controller, else to the generated interface in
`volt_handlers.go` (spec/handler-references). The Go side changes
without any `.volt` buffer changing, so the server watches `**/*.go`
when the client allows it, and every hover, definition, references,
rename and completion request first compares the Go sources it read
with the disk and re-analyzes when they differ. The Go
buffers belong to gopls: only saved files count.

## Recipes for extending the system

**New setting on an existing construct**: grammar, nothing. Checker,
one row in the settings whitelist. LSP, one entry in
`settingsByContext`. Spec, one row in the spec/settings-whitelists and
spec/settings-lists tables. Lint, only if misuse is *legal but
suspicious*.

**New element type** (a new top-level `thing name { … }`):

1. spec.md: grammar production, enumerated constraints and an
   example, in the right part.
2. Grammar: a rule following an existing element's shape, added to the
   element choice; regenerate; corpus cases.
3. Queries: a `@keyword` line in `highlights.scm`, a block in
   `outline.scm`.
4. Front end: AST node and walk case, parser production, checker rules
   citing `spec/<anchor>`, vet analyzers if any.
5. LSP: if it declares names, index cases, a hover, an outline entry
   and completion keywords.
6. Conformance: `valid/` and `invalid/` snippets tagged `// spec/…`.

**New lint**: one analyzer file and its `register()`, a `### name`
section in [`lint.md`](lint.md), a `testdata/*.volt` with `//WANT`
markers; the docs consistency test fails the build until all three
agree. The LSP picks it up automatically.

**Grammar change**: edit `grammar.js`, `tree-sitter generate`,
`tree-sitter test`, the differential checks of
editor/the-verification-checklist, `go run ./scripts/sync-grammar`,
Rebuild in Zed.

The CLI is pinned: `grammar/package-lock.json` holds tree-sitter-cli
0.25.10, the version the first line of `src/parser.c` names, and
`npm ci` in `grammar/` installs it as
`grammar/node_modules/.bin/tree-sitter`. Run that one: another version
rewrites the committed `src/` from an unchanged `grammar.js`.
`sync-grammar` regenerates with the pinned version only, from
`grammar/node_modules/.bin` or from the `PATH`, and otherwise leaves
`src/` as committed and says so. Moving the pin is a change of its own:
update the lockfile, regenerate, and commit both.

## Known limitations

Documented trade-offs, not bugs.

- **Grammar: columns named exactly like block keywords.** A line in a
  `table` or `table_partial` body that starts with a column named
  exactly `indexes` or `validators` (`records` too in a `table` body),
  or a line of a `table_create`, `table_rebuild` or `table_drop` shape
  body that starts with a column named `indexes`, lexes as the block
  keyword; the front end reads a column. Another case of the word
  (`Indexes`) is an identifier to both.
- **Grammar: a multi-line string that opens with `//`.** When the first
  characters after an opening `"""`, spaces allowed, are `//`, the
  grammar lexes the rest of that line as a comment and draws an ERROR
  node (`[default: """// x"""]`); the front end reads the string.
- **Grammar: continuations outside a settings list, and a break before
  a setting's `:`.** An element continued on an indented line,
  a parenthesized list spanning lines outside a settings list (a
  composite index key, a records column list, a composite ref
  endpoint) and a line break between a setting's name and its `:` draw
  an ERROR node; the front end accepts them
  (spec/line-structure-and-whitespace).
- **LSP: a rename stops at the `.volt` files.** The Go names `volt gen`
  derives from what it renames change at the next generation, and the
  hand-written Go that uses them is not rewritten; renaming the Go name
  with gopls first keeps the two in step. A rename at a Go reference
  rewrites the `.volt` spellings only and leaves the Go side to gopls.
  A column named through a group, in a `pred` body or a group select,
  is left alone, and the rename's check refuses the rename when that
  leaves an error.
- **LSP: Go-side navigation is partial.** A controller implemented by a
  type in another package or in a test file is not found, and the
  reference lands on the generated interface, where gopls's
  go-to-implementation takes over. A query route's reference
  (`db.UserGet`, spec/query-routes) is not navigated.
- **LSP: a relation needs the project.** A relation's hover and
  definition come from the project index, so a file outside any
  project has none.

## The verification checklist

How to audit that the three components implement
[`spec.md`](spec.md); each row is a runnable check, not a promise:

| Claim | Check | Where |
|---|---|---|
| Front end accepts/rejects exactly what the spec says | conformance corpus: `valid/` MUST pass, `invalid/` MUST fail, each snippet tagged `// spec/…` (files without a `package` clause = the schema pass, files with one and project dirs = the project pass); every diagnostic of every invalid snippet, recovery included, is a golden | `lang/conformance/snippets/` and `lang/conformance/invalid_*.golden` via `go test ./lang/...`; refresh with `-update` after reading the diff |
| Grammar parses everything the spec allows | corpus cases (input → expected tree, incl. `:error` cases) | `grammar/test/corpus/`, `tree-sitter test` with the pinned CLI, tree-sitter-cli 0.25.10 (editor/recipes-for-extending-the-system) |
| Spec grammar and front end agree | the spec's EBNF read as data: every derived sentence parses, every one-token neighbour is decided alike by the grammar and the front end | `lang/spec_grammar_test.go` via `go test ./lang/...` |
| Grammar and front end agree | every valid conformance snippet (the files its loader reads, so not `testdata/` or a nested module) and `grammar/examples/*` must produce zero front-end diagnostics, and every derived sentence of the spec's grammar (`go test ./lang -run TestSpecGrammarSentencesParse -sentences DIR`) zero parser diagnostics; all of them must produce zero tree-sitter ERROR nodes, except where a limitation of editor/known-limitations applies: today `v55_element_continuation` (a `ref:` continued on an indented line) and `34_no_reserved_words` (a column named `indexes`) | differential run after grammar changes |
| Highlight queries cover the grammar | every keyword `grammar.js` matches (`kw(...)`) is captured in `highlights.scm`, by its spelling or through the node it is aliased to (a verb, a boolean, `null`, an aggregate name) | `lsp/highlights_test.go` via `go test ./lsp/...`; `tree-sitter query zed-extension/languages/volt/highlights.scm FILE` from `grammar/` shows the captures |
| Spec examples run | every `volt` block of `spec.md` (spec/conformance-and-the-proof-chain): a plain block is a whole file the front end accepts, a block fenced `volt invalid` one it rejects, a block fenced `volt fragment` a piece of a file that parses alone; any other fence marker fails | `lang/spec_examples_test.go` via `go test ./lang/...` |
| Lint rules match their doc | doc ↔ registry ↔ testdata consistency test; `//WANT` markers both directions | `lang/vet/docs_test.go` against [`lint.md`](lint.md) |
| Generated routers implement Part II of the spec | goldens byte-compared, gofmt-stable, **compiled**; itest exercises match/404/405, typed-param 404s, pipeline order, the error spine, reverse-URL round-trip totality | `gen/router`, `itest/` |
| Generated models implement spec/mapping-to-go | goldens compiled; every CRUD statement prepared against the generated DDL; SQL goldens executed on real SQLite | `nao/gen/...`, `nao/itest` |
| LSP behaves per spec | in-process unit suites: protocol, index, rename, hover, completion, UTF-16 | `lsp/*_test.go` and `lsp/protocol/*_test.go` via `go test ./lsp/...` |
| LSP behaves per spec, interactively | a scripted stdio JSON-RPC session against the real server process, replaying keystrokes that break and fix a project file, one of them as a ranged change, and asking for hover, definition, completion and symbols; every reply and published diagnostic pinned as a golden | `cmd/volt/lsp_session_test.go` and `testdata/lsp_session.golden` via `go test ./cmd/volt/...`; refresh with `-update` after reading the diff |
| A keystroke costs what the budget says | the whole in-process path of one keystroke inside one table of a one-file project, didChange to publish, and the document's own front end alone; compare against the last report of `scripts/perf.go`. The bytes the keystroke's three parts allocate (the document's pass, the project analysis, the publish) may not exceed their budgets by more than ten percent, hardware-neutral | `go test ./lsp -run '^$' -bench 'BenchmarkKeystroke|BenchmarkDocumentUpdateLocal' -benchmem`; `go test ./lsp -run TestKeystrokeAllocationBudget -v` |

Nothing checks the corpus against the DBML implementation Part I
descends from (spec/license-and-provenance); the conformance corpus
alone is the record.

When adding a construct, every row above must gain its case;
editor/recipes-for-extending-the-system says where. A construct that
exists in code but has no row here is unverified, and that is a bug in
the process, not a smaller task.
