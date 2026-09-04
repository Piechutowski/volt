# Editor tooling: the grammar, the Zed extension, the language server

How Volt editor support is put together — three independently testable
components (D40), why the system is shaped this way, the load-bearing
patterns in each, and (§8) the checklist for verifying all of it
against [`spec.md`](spec.md).

## 1. The big picture

```
                        ┌────────────────────── Zed process ──────────────────────┐
                        │                                                          │
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

Every serious language setup (Go, Rust, TypeScript in any modern
editor) works this way; §8's differential checks handle the risk of the
two disagreeing. When they do, **the front end is right by
definition**.

The front end is layered like the Go toolchain it mirrors, and the LSP
reuses it wholesale:

```
lang/token     lexical vocabulary (Kind, Token, Position with byte Offset)
lang/scanner   text → tokens                  (lexical errors, spec §3)
lang/parser    tokens → ast.File              (syntax errors, spec §4–§6, §V)
lang/check     ast.File → check.Info          (schema semantics, spec §8)
lang           project Load/Check             (packages, imports, routes, §V)
lang/vet       lints                          (lint.md; legal but suspicious)
lsp            all of the above → LSP
```

One source of truth: an error squiggle in Zed is *by construction* the
same error `volt check` or codegen would report.

## 2. The grammar (`grammar/`)

Tree-sitter compiles `grammar.js` into a table-driven GLR parser in C
(`src/parser.c`, committed, because Zed builds the WASM from it). The
load-bearing patterns, each citing the spec section it implements:

- **Explicit newlines, not `extras`** (§3.2). Volt is
  newline-sensitive — columns, enum values, records rows, routes and
  plugs are line-terminated. `extras` holds only `[ \t\r]` + comments,
  and every body uses one `newlineSep1` helper. Falls out for free:
  `Table t { id int name varchar }` is a syntax error (spec-correct),
  one-liners still parse, settings lists are same-line only, and a line
  comment doesn't consume its newline.
- **Case-insensitive contextual keywords** (§3.5). Keywords are
  generated regexes aliased to a canonical spelling; tree-sitter's
  context-aware lexing provides the "not reserved" half — `table` in
  column-name position lexes as an identifier. Known limitation: at the
  start of a table-body line, an unquoted column literally named
  `indexes`/`checks`/`note`/`records` lexes as the keyword; quoting
  opts out (§1.4).
- **Split string tokens for injection** (§3.6–3.7). Delimiters and
  content are separate tokens, so `(string_content)` is a node with a
  range — exactly what `injections.scm` needs to hand note bodies to
  Markdown and backtick expressions to SQL.
- **GLR conflict for reference chains** (§6.7). `a.b.c` in a ref
  endpoint is ambiguous at the first dot; `ref_target` is declared in
  `conflicts` and the token after the chain kills the wrong stack.
- **Generic settings, open vocabulary** (§4.2). `settings_list` parses
  any `[name: value]`; which keys are legal where is semantics, and the
  checker enforces it with positions the LSP surfaces. New settings
  parse without touching the grammar.
- **The Volt layer** (§V): `package` clause, `import (…)` blocks,
  `Pipeline`, `Scope`, routes with typed parameters and `:name...`
  wildcards, `resources` with an optional package qualifier, and the
  query layer — `Group` algebra, the `Pred` expression grammar
  (prec.left or/and, keyword operators at the same precedence as other
  keywords so `not` never outlexes `Note`), `Select ... for ... where`.
  `/` is a token per §V0.2.

## 3. The Zed extension (`zed-extension/`)

```
extension.toml               manifest: ids, [grammars.volt], [language_servers.volt]
Cargo.toml, src/lib.rs       Rust glue, compiled BY ZED to WASM at install time
languages/volt/
  config.toml                registration: name, suffixes (volt, dbml), comments, brackets
  highlights.scm  outline.scm  indents.scm  brackets.scm  injections.scm
```

The glue implements one trait method,
`language_server_command`, with a resolution chain — user setting
`lsp.volt.binary.path`, then `worktree.which("volt")` — plus optional
passthrough of `initialization_options`/settings. No
download-from-releases step: this extension is local-first, and
`go install ./cmd/volt` is the installer.

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
`protocol.Handler` literal in `server.go`. Document sync is full-text:
files are small, and full sync makes the server stateless per edit.

**The per-edit pipeline.** Every open/change/close re-runs the whole
front end — it is fast enough that there is no cache to invalidate:

1. Find the project root (the nearest `go.mod`, §V1.1). A file under a root is
   checked as part of its **whole project**: every open buffer is
   overlaid over the disk (`lang.LoadOverlay`) so unsaved edits in one
   file are visible to checks in another. A file outside any project
   gets the single-file schema pass.
2. `lang.Check` resolves packages, imports, tables, routes, conflicts.
3. Diagnostics are filtered per open file and published (push);
   `go.mod` problems are remapped to line 1 of the file. After each
   change the server re-checks and republishes **every other open
   document** (`refreshOthers`), so cross-file diagnostics never go
   stale.
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
  validates the new name against the identifier grammar (§3.4).
- Column identity is container-scoped; a column injected from a
  TablePartial resolves into the partial, so renaming it updates the
  partial once plus every reference through every injecting table
  (§6.9.4).
- Completion is textual-context + semantic-fill: where-am-I is
  answered by line-prefix regexes plus a `{`/`}`-balance scanner (the
  current line usually doesn't parse mid-keystroke); candidates come
  from the last coherent semantic model — real tables after `db.`
  (case-insensitively filtered), verbs and `resources` in Scope
  bodies, settings per construct (§4.2, §V6).
- Position encoding is converted at the boundary only (1-based rune
  columns ↔ 0-based UTF-16), re-derived from the line's actual text;
  nothing else does position arithmetic.

**vet runs only on error-free files** — style advice stacked on hard
errors is noise while typing; it reappears when the file checks clean.

## 5. Extending the system — the recipes

**New setting on an existing construct**: grammar — nothing (generic
settings). Checker — one row in the settings whitelist. LSP — one entry
in `settingsByContext`. Spec — one row in the §V6 / §4.2 tables. Lint —
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
[`lint.md`](lint.md), a `testdata/*.dbml` (or `.volt`) with `//WANT`
markers — the docs consistency test fails the build until all three
agree. The LSP picks it up automatically.

**Grammar change**: edit `grammar.js` → `tree-sitter generate` →
`tree-sitter test` → §8's differential checks →
`./scripts/sync-grammar.sh` → Rebuild in Zed.

## 6. Decision log

- **Go for the server** — the reference front end is Go; wrapping it
  gives spec-exact diagnostics for free, one implementation to
  maintain.
- **`tliron/glsp` over `go.lsp.dev`** — maintained, complete 3.16
  types, no codegen. Its dependency tail is real (websocket, jsonrpc2,
  an LSP-3.17 rewrite would be ~300 lines over stdio) — contained by
  the tool-module split; replacing it stays an open option.
- **Full-text sync over incremental** — statelessness beats patch
  bookkeeping at these file sizes.
- **Whole-project re-check per edit** — no dirty tracking to get
  wrong; measured fast at realistic project sizes.

## 7. Known limitations (documented trade-offs, not bugs)

- **Grammar: unquoted columns named like block keywords** — `indexes`,
  `checks`, `note`, `records` need quoting in column position (§3.5
  note above); the front end, spec-exact, accepts both.
- **Grammar: exotic string edges** — a triple-quoted string whose
  content ends in a quote immediately before the closing `'''` may
  tokenize differently from the spec's maximal-munch rule; the front
  end handles it correctly.
- **LSP: no Go-side navigation** — handler references (`Home.Index`)
  and `error_handler:` point into Go, which this server does not
  index (roadmap FW-6). A query reference (`db.UserGet`, §V4.8) is
  not navigated either; a dataset's select (`dataset db.browse`) is.

### Go references

A `.volt` file names Go by rule only in the containing package's own
Go files — pipeline plugs (§V3.2) and Go-reference checks (§V12.5).
The server resolves those names with the standard library's Go parser
over the package directory's non-test `*.go` files (no gopls, no
build): go-to-definition lands on the `func`, hover shows its signature
and doc comment, and an undeclared name hovers as undeclared while the
checker's diagnostic on the reference names the exact signature to
write. `volt.*` plugs live in the runtime and
make no claim. Existence and the spelled signature are the checker's
(D63), so a typo or a wrong parameter type is a diagnostic at the
reference. The Go side can move without any `.volt` buffer changing —
a gopls rename, a newly written function — so the server registers a
`**/*.go` file watcher with the client and re-checks every open
document when one is saved; independently, every hover, definition,
references, rename and completion request first compares the scanned
Go files' fingerprint (names, sizes, mtimes) with the disk and re-runs
the analysis when it moved, republishing diagnostics. Only saved files
count: the Go buffers themselves belong to gopls, not to this server. Rename on a Go reference rewrites its Volt spellings only —
the Go declaration is gopls' job, and the existence error then points
at whichever side is still behind. Rename on a column follows it into
the table's checks (typed operands and Go-reference arguments); a
column named through a group — in a `Pred` body or a group select — is
left alone, and the agreement error (§V11.4 for a select, §V12.2 for a
Pred used by a table's checks) names what is now missing.

## 8. The verification checklist

How to audit that the three components implement
[`spec.md`](spec.md) — each row is a runnable check, not a promise:

| Claim | Check | Where |
|---|---|---|
| Front end accepts/rejects exactly what the spec says | conformance corpus: `valid/` MUST pass, `invalid/` MUST fail, each snippet tagged `// spec: §…` (`.dbml` entries = the schema pass, `.volt` entries and project dirs = the project pass) | `lang/conformance/snippets/` via `go test ./lang/...` |
| Grammar parses everything the spec allows | corpus cases (input → expected tree, incl. `:error` cases) | `grammar/test/corpus/`, `tree-sitter test` |
| Grammar and front end agree | every valid conformance snippet and `grammar/examples/*` must produce zero tree-sitter ERROR nodes **and** zero front-end diagnostics | differential run after grammar changes |
| Lint rules match their doc | doc ↔ registry ↔ testdata consistency test; `//WANT` markers both directions | `lang/vet/docs_test.go` against [`lint.md`](lint.md) |
| Generated routers implement §V | goldens byte-compared, gofmt-stable, **compiled**; itest exercises match/404/405, typed-param 404s, pipeline order, the error spine, reverse-URL round-trip totality | `gen/router`, `itest/` |
| Generated models implement Appendix A/B | goldens compiled; every CRUD statement prepared against the generated DDL; SQL goldens executed on real SQLite | `nao/gen/...`, `nao/itest` |
| LSP behaves per spec | in-process unit suites: index, rename spelling rules, hover content, completion contexts, UTF-16 | `lsp/*_test.go` via `go test ./lsp/...` |
| LSP behaves per spec, interactively | scripted stdio JSON-RPC sessions against the built binary, replaying real keystrokes — a development practice, not yet an automated test | manual; automating it is an open roadmap item |

The upstream `@dbml/parse` cross-check that established Part I's
fidelity was retired at zero disagreements (D54); the corpus verdicts
it pinned are the surviving record, and a divergence-worthy upstream
feature gets re-checked by hand when adopted.

When adding a construct, every row above must gain its case — §5's
recipes say where. A construct that exists in code but has no row here
is unverified, and that is a bug in the process, not a smaller task.
