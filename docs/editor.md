# Editor tooling: the grammar, the Zed extension, the language server

> [!CAUTION]
> **This project is an alpha version, and everything may change in the future.**

How Volt editor support is put together — three independently testable
components, the load-bearing patterns in each, and (section 8) the
checklist for verifying all of it against [`spec.md`](spec.md).

## 1. The big picture

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

1. **The grammar** (`grammar/`) — a *syntactic*, error-tolerant,
   incremental parser compiled to WebAssembly and run inside Zed on
   every keystroke. It powers everything visual: colors, outline,
   indent, bracket matching, Markdown-in-notes.
2. **The extension** (`zed-extension/`) — a thin package: declarative
   files (manifest, language config, queries) plus ~70 lines of Rust
   whose only job is telling Zed how to launch `volt lsp`.
3. **The language server** (`lsp/`, served by `volt lsp`; its own Go
   module, so its dependencies stay out of the library) — a *semantic*, spec-exact analyzer speaking LSP over
   stdio: diagnostics, completion, hover, navigation, rename.

### Why two parsers?

The grammar and the server both parse Volt, deliberately — opposite
requirements:

| | tree-sitter grammar | Go front end |
|---|---|---|
| Runs | inside the editor, every keystroke | separate process, per change |
| Must be | fast, incremental, **error-tolerant** | **correct** — the reference implementation of spec.md |
| Output | concrete syntax tree | AST + semantic model + diagnostics |
| Knows | shape only | meaning: resolution, partial injection, route conflicts |

Section 8's differential checks catch the two disagreeing. When they
do, **the front end is right by definition**.

The front end is layered like the Go toolchain it mirrors, and the LSP
reuses it wholesale:

```
lang/token     lexical vocabulary (Kind, Token, Position with byte Offset)
lang/scanner   text → tokens                  (lexical errors, spec §lexical-structure)
lang/parser    tokens → ast.File              (syntax errors, spec §common-syntactic-forms–§element-definitions, Part II of the spec)
lang/check     ast.File → check.Info          (schema semantics, spec §static-semantics)
lang           project Load/Check             (packages, imports, routes, Part II of the spec)
lang/vet       lints                          (lint.md; legal but suspicious)
lsp            all of the above → LSP
```

One source of truth: an error squiggle in Zed is *by construction* the
same error `volt check` or codegen would report.

## 2. The grammar (`grammar/`)

Tree-sitter compiles `grammar.js` into a table-driven GLR parser in C
(`src/parser.c`, committed, because Zed builds the WASM from it). The
load-bearing patterns, each citing the spec section it implements:

- **Explicit newlines, not `extras`** (§line-structure-and-whitespace). Volt is
  newline-sensitive — columns, enum values, records rows, routes and
  plugs are line-terminated. `extras` holds only `[ \t\r]` + comments,
  and every body uses one `newlineSep1` helper. Falls out for free:
  `Table t { id int name string }` is a syntax error (spec-correct),
  one-liners still parse, settings lists are same-line only, and a line
  comment doesn't consume its newline.
- **Case-insensitive contextual keywords** (§keywords). Keywords are
  generated regexes aliased to a canonical spelling; tree-sitter's
  context-aware lexing provides the "not reserved" half — `table` in
  column-name position lexes as an identifier. Known limitation: at the
  start of a table-body line, an unquoted column literally named
  `indexes`/`validators`/`note`/`records` lexes as the keyword; quoting
  opts out (§case-sensitivity-and-terminology).
- **Split string tokens for injection** (§string-literals–3.7). Delimiters and
  content are separate tokens, so `(string_content)` is a node with a
  range — exactly what `injections.scm` needs to hand note bodies to
  Markdown and backtick expressions to SQL.
- **GLR conflict for reference chains** (§relationships-ref). `a.b.c` in a ref
  endpoint is ambiguous at the first dot; `ref_target` is declared in
  `conflicts` and the token after the chain kills the wrong stack.
- **Generic settings, open vocabulary** (§settings-lists). `settings_list` parses
  any `[name: value]`; which keys are legal where is semantics, and the
  checker enforces it with positions the LSP surfaces. New settings
  parse without touching the grammar.
- **The Volt layer** (Part II of the spec): `package` clause, `import (…)` blocks,
  `Pipeline`, `Scope`, routes with typed parameters and `:name...`
  wildcards, `resources` with an optional package qualifier, and the
  query layer — `Group` algebra, the `Pred` expression grammar
  (prec.left or/and, keyword operators at the same precedence as other
  keywords so `not` never outlexes `Note`), `Select ... for ... where`.
  `/` is a token per §layers-files-and-the-superset-rule.
- **Keywords of a migration body are tokens of the migration rules
  only** (§migrations, §keywords). `up`, `down`, `create`, `drop`,
  `rename`, `alter`, `rebuild`, `exec`, `table`, `index`, `add` and
  `to` appear as `kw()` tokens inside `migration_definition` and its
  operations and nowhere else, so the context-aware lexer never
  considers them where a column name is expected: `Table t { up int }`
  parses as a column named `up`, and `rename to to down` inside an
  `alter table` renames the column `to`. The corpus case
  `migrations.txt` proves both.

## 3. The Zed extension (`zed-extension/`)

```
extension.toml               manifest: ids, [grammars.volt], [language_servers.volt]
Cargo.toml, src/lib.rs       Rust glue, compiled BY ZED to WASM at install time
languages/volt/
  config.toml                registration: name, suffix (volt), comments, brackets
  highlights.scm  outline.scm  indents.scm  brackets.scm  injections.scm
```

The glue implements one trait method,
`language_server_command`, with a resolution chain — user setting
`lsp.volt.binary.path`, then `worktree.which("volt")` — plus optional
passthrough of `initialization_options`/settings. No
download-from-releases step: this extension is local-first, and
`go install ./cmd/volt` is the installer, with cgo (a C compiler on
the path), since the tool carries its own SQLite.

**The local grammar mirror** (`scripts/sync-grammar.sh`): Zed loads
grammars from a *git repository* even for dev extensions. The script
regenerates `src/parser.c` if a tree-sitter CLI is present, mirrors
`grammar/` into `~/.cache/volt/tree-sitter-volt-git`, commits there,
and rewrites `[grammars.volt]` with the mirror's `file://` URL and
fresh SHA. The rewritten lines are machine-local by design; the
committed `extension.toml` carries a placeholder.

**Capture-name discipline**: Zed themes style a fixed capture
vocabulary (`@keyword`, `@type`, `@property`, `@attribute`, …); the
queries map *roles*, not node names — tables → `@type`, columns →
`@property`, enum values → `@variant`, setting names → `@attribute`.
Later patterns win in a query file, so generic rules go first.

## 4. The language server (`lsp/`)

Speaks LSP 3.16 over stdio via `tliron/glsp`; the wiring is one
`protocol.Handler` literal in `server.go`. Document sync is
incremental: the client sends each keystroke as a range and the
text that replaces it, and the server applies every item of a change
to the text as it then stands, positions in UTF-16 units; a client
that still sends the whole text is served the same way. Every publish
of diagnostics carries the version of the text they are positioned
in, so a client whose buffer has moved on drops it.

**The per-edit pipeline.** An edit updates the document's own front
end at once, through the document's own memos (the parse by
declaration, the check by table and the occurrence index by table are
the session's memo shapes, owned by the handler goroutine),
and schedules the project's analysis:

1. Find the project root (the nearest `go.mod`, §projects-and-packages). A file under a root is
   checked as part of its **whole project**: every open buffer is
   overlaid over the disk (`lang.LoadOverlay`) so unsaved edits in one
   file are visible to checks in another. A file outside any project
   gets the single-file schema pass.
2. `lang.Check` resolves packages, imports, tables, routes, conflicts.
3. Diagnostics are filtered per open file and published (push);
   `go.mod` problems are remapped to line 1 of the file. An edit
   updates the document's own front end at once and schedules **one
   background analysis of its project**: after the edits settle
   (75 ms), the project is loaded and checked through the root's
   `lang.Session` — parses cached by content, per-package results
   memoized by input identity — and every open document of the
   project is published from that one run, so cross-file diagnostics
   never go stale and one edit costs one parse plus the packages that
   could see it. A request adopts the newest result at its start.
4. Two indexes are (re)built *after* Check, from the resolved model:
   the schema occurrence index (single-file symbols: tables, columns,
   enums, partials, aliases) and the project index (`voltIndex`:
   qualified references like `db.posts`, pipeline uses, resources).

**Features on top of the indexes** — definition, references, rename,
hover and completion all try the project layer first, then the schema
layer:

- Rename rewrites exactly the identifier, never the `db.` qualifier
  (the hit-test span and the edit span are tracked separately), only
  occurrences whose spelling matches the one under the cursor, and
  validates the new name against the identifier grammar (§identifiers).
- Column identity is container-scoped; a column injected from a
  TablePartial resolves into the partial, so renaming it updates the
  partial once plus every reference through every injecting table
  (§tablepartial).
- Completion is textual-context + semantic-fill: where-am-I is
  answered by line-prefix regexes plus a `{`/`}`-balance scanner (the
  current line usually doesn't parse mid-keystroke); candidates come
  from the last coherent semantic model — real tables after `db.`
  (case-insensitively filtered), verbs and `resources` in Scope
  bodies, operations in a Migration body, settings per construct
  (§settings-lists, §settings-whitelists).
- A migration's name hovers as its position among the package's
  migrations in name order ("migration 2 of 3") and whether it runs
  backward (§migrations); the project index answers it, so a
  migration alone in its own file counts its siblings.
- Position encoding is converted at the boundary only (1-based rune
  columns ↔ 0-based UTF-16), re-derived from the line's actual text;
  nothing else does position arithmetic.

**vet runs only on error-free files** — style advice stacked on hard
errors is noise while typing; it reappears when the file checks clean.
And it runs only where its verdict is what the editor shows: a
file in a project package is vetted by the project analysis, never by
the document's own pass; a file outside any project, or one under a
project the loader never read (§projects-and-packages), gets the single-file vet added
to its own verdict once per text.

## 5. Extending the system — the recipes

**New setting on an existing construct**: grammar — nothing (generic
settings). Checker — one row in the settings whitelist. LSP — one entry
in `settingsByContext`. Spec — one row in the §settings-whitelists / §settings-lists tables. Lint —
only if misuse is *legal but suspicious*.

**New element type** (a new top-level `Thing name { … }`):
1. spec.md: grammar production + numbered constraints + example, in
   the right part.
2. Grammar: a rule following an existing element's shape, added to the
   element choice; regenerate; corpus cases.
3. Queries: a `@keyword` line in `highlights.scm`, a block in
   `outline.scm`.
4. Front end: AST node (+ walk case), parser production, checker rules
   with a `spec/§` code, optional vet analyzers.
5. LSP: if it declares names — index cases, hover renderer,
   `DocumentSymbols` case, completion keywords. Additive switch cases;
   nothing central changes.
6. Conformance: `valid/` and `invalid/` snippets tagged `// spec: §…`.

**New lint**: one analyzer file + `register()`, a `### name` section in
[`lint.md`](lint.md), a `testdata/*.volt` with `//WANT`
markers — the docs consistency test fails the build until all three
agree. The LSP picks it up automatically.

**Grammar change**: edit `grammar.js` → `tree-sitter generate` →
`tree-sitter test` → section 8's differential checks →
`./scripts/sync-grammar.sh` → Rebuild in Zed.

## 6. Tooling choices

The server is Go and wraps the reference front end, so diagnostics come
from one implementation. The protocol library is `tliron/glsp` (LSP
3.16), kept out of the library module by the tool-module split.

## 7. Known limitations (documented trade-offs, not bugs)

- **Grammar: unquoted columns named like block keywords** — `indexes`,
  `validators`, `note`, `records` need quoting in column position (§keywords
  note above); the front end, spec-exact, accepts both.
- **Grammar: exotic string edges** — a triple-quoted string whose
  content ends in a quote immediately before the closing `'''` may
  tokenize differently from the spec's maximal-munch rule; the front
  end handles it correctly.
- **LSP: no Go-side navigation** — handler references (`Home.Index`)
  and `error_handler:` point into Go, which this server does not
  index (roadmap). A query reference (`db.UserGet`, §query-routes) is
  not navigated either; a dataset's select (`dataset db.browse`) is.

### Go references

A `.volt` file names Go by rule only in the containing package's own
Go files — pipeline plugs (§pipelines) and validators (§validators).
The server resolves those names with the standard library's Go parser
over the package directory's non-test `*.go` files (no gopls, no
build): go-to-definition lands on the `func`, hover shows its signature
and doc comment, and an undeclared name hovers as undeclared while the
checker's diagnostic on the reference names the exact signature to
write. `volt.*` plugs live in the runtime and
make no claim. Existence and the spelled signature are the checker's
, so a typo or a wrong parameter type is a diagnostic at the
reference. The Go side can move without any `.volt` buffer changing —
a gopls rename, a newly written function — so the server registers a
`**/*.go` file watcher with the client and re-analyzes every project
with an open document when one is saved (the Go sources are part of
a package's memo key, so only the packages beside the change
re-check); independently, every hover, definition,
references, rename and completion request first compares the Go
sources it scanned with the disk, byte for byte, and when they
differ kicks that same background analysis, which republishes; the
request in hand answers from the analysis it adopted, the next adopts
the fresh one. Only saved files
count: the Go buffers themselves belong to gopls, not to this server. Rename on a Go reference rewrites its Volt spellings only —
the Go declaration is gopls' job, and the existence error then points
at whichever side is still behind. Rename on a column follows it into
the table's validators (their arguments); a column named through a
group, in a `Pred` body or a group select, is left alone, and the
agreement error (§selects-over-groups) names what is now missing.

## 8. The verification checklist

How to audit that the three components implement
[`spec.md`](spec.md) — each row is a runnable check, not a promise:

| Claim | Check | Where |
|---|---|---|
| Front end accepts/rejects exactly what the spec says | conformance corpus: `valid/` MUST pass, `invalid/` MUST fail, each snippet tagged `// spec: §…` (files without a `package` clause = the schema pass, files with one and project dirs = the project pass); every diagnostic of every invalid snippet, recovery included, is a golden | `lang/conformance/snippets/` and `lang/conformance/invalid_*.golden` via `go test ./lang/...`; refresh with `-update` after reading the diff |
| Grammar parses everything the spec allows | corpus cases (input → expected tree, incl. `:error` cases) | `grammar/test/corpus/`, `tree-sitter test` |
| Spec grammar and front end agree | the spec's EBNF read as data: every derived sentence parses, every one-token neighbour is decided alike by the grammar and the front end | `lang/spec_grammar_test.go` via `go test ./lang/...` |
| Grammar and front end agree | every valid conformance snippet, `grammar/examples/*` and the spec's derived sentences (`go test ./lang -run TestSpecGrammarSentencesParse -sentences DIR`) must produce zero tree-sitter ERROR nodes **and** zero front-end diagnostics | differential run after grammar changes |
| Lint rules match their doc | doc ↔ registry ↔ testdata consistency test; `//WANT` markers both directions | `lang/vet/docs_test.go` against [`lint.md`](lint.md) |
| Generated routers implement Part II of the spec | goldens byte-compared, gofmt-stable, **compiled**; itest exercises match/404/405, typed-param 404s, pipeline order, the error spine, reverse-URL round-trip totality | `gen/router`, `itest/` |
| Generated models implement §mapping-to-go/B | goldens compiled; every CRUD statement prepared against the generated DDL; SQL goldens executed on real SQLite | `nao/gen/...`, `nao/itest` |
| LSP behaves per spec | in-process unit suites: index, rename spelling rules, hover content, completion contexts, UTF-16 | `lsp/*_test.go` via `go test ./lsp/...` |
| LSP behaves per spec, interactively | a scripted stdio JSON-RPC session against the real server process, replaying keystrokes that break and fix a project file, one of them as a ranged change, and asking for hover, definition, completion and symbols; every reply and published diagnostic pinned as a golden | `cmd/volt/lsp_session_test.go` and `testdata/lsp_session.golden` via `go test ./cmd/volt/...`; refresh with `-update` after reading the diff |
| A keystroke costs what the budget says | the whole in-process path of one keystroke inside one table of a one-file project, didChange to publish, and the document's own front end alone; compare against the last report of `scripts/perf.go`. The bytes the keystroke's three parts allocate (the document's pass, the project analysis, the publish) are pinned exactly, within ten percent, hardware-neutral | `go test ./lsp -run '^$' -bench 'BenchmarkKeystroke|BenchmarkDocumentUpdateLocal' -benchmem`; `go test ./lsp -run TestKeystrokeAllocationBudget -v` |

The cross-check against the implementation Part I descends from
(spec §license-and-provenance) was retired at zero disagreements;
the corpus verdicts it pinned are the surviving record.

When adding a construct, every row above must gain its case — §program-structure's
recipes say where. A construct that exists in code but has no row here
is unverified, and that is a bug in the process, not a smaller task.
