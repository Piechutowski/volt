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
   indent, bracket matching.
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
  newline-sensitive: columns, records rows, routes and
  plugs are line-terminated. `extras` holds only `[ \t\r]` + comments,
  and every body uses one `newlineSep1` helper. Falls out for free:
  `Table t { id int name string }` is a syntax error (spec-correct),
  one-liners still parse, and a line comment doesn't consume its
  newline. A settings list takes line breaks inside its brackets
  (around a setting, around a comma, after a `:` and between the
  items of a parenthesized value), so it may span lines; a break
  before a `:` and the other continuations the spec allows are
  limitations (section 7).
- **Case-sensitive contextual keywords** (§keywords). Each keyword is a
  plain string in its one spelling, and tree-sitter's keyword
  extraction matches it against the identifier token exactly and only
  where the parse state admits it: that is the "not reserved" half, so
  `table` in column-name position lexes as an identifier, while `TABLE`
  at top level is an ERROR node, as the front end's error
  (§case-sensitivity-and-terminology). Known limitation: a column
  named exactly like a block keyword at the start of a body line lexes
  as the keyword (section 7).
- **Names are identifiers, strings are Go's** (§identifiers,
  §string-literals). An identifier is a Go identifier, the one token
  that begins with digits is a migration name, and no name is quoted;
  `"..."` and `"""..."""` are the strings, with the four escapes, and a
  single quote opens nothing.
- **Split string tokens for injection** (§string-literals). Delimiters and
  content are separate tokens, so `(string_content)` is a node with a
  range; `injections.scm` hands backtick expressions, an `exec`'s SQL
  among them, to SQL.
- **One endpoint shape** (§relationships-ref). A ref endpoint is
  `table.column` or `table.(columns)`, in a `ref:` and in a column's
  `ref:` alike, so the grammar declares no conflicts: `setting` gives
  `ref` its own alternative whose value is a `ref_endpoint`, and every
  other setting takes the generic value.
- **Generic settings, open vocabulary** (§settings-lists). `settings_list` parses
  any `[name: value]`; which keys are legal where is semantics, and the
  checker enforces it with positions the LSP surfaces. New settings
  parse without touching the grammar.
- **The Volt layer** (Part II of the spec): `package` clause, `import (…)` blocks,
  `pipeline`, `scope`, routes (the verbs of §routes, `put` not among
  them) with bare and `:name(int)` parameters, `int` being the one
  type written (§route-paths), and `:name...`
  wildcards, `resources` with an optional package qualifier, and the
  query layer — `group` algebra, the `pred` expression grammar
  (prec.left or/and, keyword operators at the same precedence as other
  keywords, `exists rel (...)` terms, and a query parameter's optional
  `?` as a token immediate after its name), `Select ... for ... where`.
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
regenerates `src/parser.c` if the pinned tree-sitter CLI is present
(section 5), mirrors
`grammar/` into `~/.cache/volt/tree-sitter-volt-git`, commits there,
and rewrites `[grammars.volt]` with the mirror's `file://` URL and
fresh SHA. The rewritten lines are machine-local by design; the
committed `extension.toml` carries a placeholder.

**Capture-name discipline**: Zed themes style a fixed capture
vocabulary (`@keyword`, `@type`, `@property`, `@attribute`, …); the
queries map *roles*, not node names — tables → `@type`, columns →
`@property`, setting names → `@attribute`.
Later patterns win in a query file, so generic rules go first.

## 4. The language server (`lsp/`)

Speaks LSP 3.16 over stdio through its own protocol layer,
`lsp/protocol` (standard library only, described below); the wiring
is one method table in `server.go`. Document sync is
incremental: the client sends each keystroke as a range and the
text that replaces it, and the server applies every item of a change
to the text as it then stands, positions in UTF-16 units; a client
that still sends the whole text is served the same way. Every publish
of diagnostics carries the version of the text they are positioned
in, so a client whose buffer has moved on drops it.

**The protocol layer (`lsp/protocol`).** Written from the LSP 3.16
specification and the JSON-RPC 2.0 specification, and holding only
what Volt uses: stdio, the methods in `server.go`'s table, and types
with the fields Volt reads or writes.

- *Framing* (Base Protocol): a header block of `Name: value` lines,
  names case-insensitive, ended by a blank line; `Content-Length` is
  required, `Content-Type` is read and ignored (UTF-8 is the only
  encoding, and a body in another one fails to parse). A missing or
  malformed `Content-Length` loses the message boundary, so the stream
  ends with an error. Messages are read through one buffered reader,
  so several in one read and one split across reads are the same case.
- *Messages*: a message with a method and an `id` is a request, one
  with a method and no `id` a notification, one with an `id` and a
  `result` or an `error` but no method a reply to the server's own
  request, which is read and discarded. An id is a number or a string
  and is echoed byte for byte; any other id gets `InvalidRequest`
  with a `null` id. A body that is not JSON gets `ParseError`, a JSON
  value that is not one message object (a batch included, which LSP
  does not use) gets `InvalidRequest`. A response carries `result`
  (`null` when the handler has none) or `error`, never both.
- *Dispatch*: one `Handler` per method; `protocol.Request` and
  `protocol.Notification` turn a typed function into one, decoding
  the params (unknown fields ignored, absent or `null` params give the
  zero value) and answering `InvalidParams` when they do not decode.
  An unknown method gets `MethodNotFound` as a request and is ignored
  as a notification, which is also the whole of the `$/` rule ($
  Notifications and Requests): `$/setTrace` and `$/cancelRequest` are
  dropped, and a `$/` request is answered `MethodNotFound`. A handler
  error is answered `InternalError` (-32603) with the error's text as
  the message: 3.16 defines no code for a request the server
  declines (3.17 adds `RequestFailed`), and a refused rename is the
  one such error Volt returns.
- *Lifecycle* (Lifecycle Messages): before a successful `initialize`
  every request gets `ServerNotInitialized` (-32002) and every
  notification but `exit` is dropped; a second `initialize` gets
  `InvalidRequest`. The layer answers `shutdown` itself, and after it
  every request gets `InvalidRequest` and every notification but
  `exit` is dropped. `exit` ends the serve loop with exit code 0 when
  `shutdown` came first and 1 otherwise, and the end of the input
  stream counts as `exit`; `RunStdio` exits the process with that
  code.
- *Order and concurrency*: messages are handled one at a time in
  arrival order, each to completion before the next is read, which
  the document memos rely on. Every write (a response, a notification
  from the analysis goroutine, a request) is one locked write of
  header and body, so concurrent writers never interleave.
- *Server to editor*: the handler `Context` carries `Notify` and
  `Request`. `Request` writes a request with the next numeric id and
  returns at once; the read loop drops the reply. So
  `client/registerCapability` is sent from the `initialized` handler
  directly.
- *Capabilities*: the `initialize` reply is written out field by
  field in `server.go`, not derived from the method table.
- *Required fields*: a field the spec requires has no `omitempty`;
  `PublishDiagnosticsParams` writes a nil list as `[]`, so clearing
  diagnostics never sends `null`.

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
   declared types, partials) and the project index (`voltIndex`:
   qualified references like `db.posts`, pipeline uses, resources).

**Features on top of the indexes** — definition, references, rename,
hover and completion all try the project layer first, then the schema
layer:

- Rename rewrites exactly the identifier, never the `db.` qualifier
  (the hit-test span and the edit span are tracked separately), only
  occurrences whose spelling matches the one under the cursor, each
  once, and validates the new name against the identifier grammar
  (§identifiers). It then checks what it would write: every error of
  the project with the edits applied (the file alone outside a
  project) must be one it had before, the same file and message with
  the renamed names spelled as they were and the positions it names
  set aside, or the rename is refused naming the first error it adds,
  so a name that makes a relation ambiguous, or that a `pred` would no
  longer find, never lands (§relationships-ref), even beside an error
  it fixes. A rename at a Go reference skips the check, since gopls
  renames the Go side (section 7). A rename rewrites `.volt` files
  alone: the generated Go names derived from what it renames change at
  the next `volt gen`, and hand-written Go that uses them does not
  (section 7).
- A relation is its own symbol (§relationships-ref): one a ref's `as:`
  or `reverse:` names is declared there, and renames, at the setting
  or at any use, with every use: a join, a column's qualifier, a
  `with`, an exists term, and those of the `pred` bodies a select over
  its table reads; one named after its foreign-key column refuses
  rename from any file and renames with the column instead, which
  rewrites its uses to the column's new name without `_id`; one named
  after a table is that table's and renames with it. Definition at a
  relation goes to its `as:` or `reverse:`, else to its column, else to
  the table; it hovers as the relationship it walks and the table it
  leads to, one named after that table too.
- Column identity is container-scoped; a column injected from a
  `table_partial` resolves into the partial, so renaming it updates the
  partial once plus every reference through every injecting table
  (§tablepartial). A column renames across every file of its package,
  from its declaration or from any use.
- Completion is textual-context + semantic-fill: where-am-I is
  answered by line-prefix regexes plus a `{`/`}`-balance scanner (the
  current line usually doesn't parse mid-keystroke); candidates come
  from the last coherent semantic model — real tables after `db.`
  (case-insensitively filtered), verbs and `resources` in `scope`
  bodies, tables and groups in a `group` body, operations in a
  `migration` body, the columns of the named table after `ref: table.`,
  `pred` names and `exists` where a predicate term starts, after
  `where`, `and`, `or`, `not`, a pred's `{`, with a space or without,
  or a predicate's `(` (not a projection's, a list's or a setting's),
  and at the start of a line of a pred body, `exists` left out inside
  an exists term, which does not nest, and the `pred`s there those
  whose every column is a column of the relation's table, and the relations of the
  select's table after `exists` (§predicates),
  settings per construct and none on a construct that takes none
  (§settings-lists, §settings-whitelists), and `true`/`false` after
  `default:` only on a column stored as `bool`. Every keyword
  completion offers is in its one spelling (§keywords): the
  referential actions are the parser's own list, and a test holds the
  element words, the verbs and the setting names to the parser's
  lists. Resource actions and order directions are not completed.
- Every hover of a declaration ends with its doc text (§doc-comments):
  a table, a column, a partial, a declared type, a group, a pred, a
  select, a pipeline and a migration. On the first line of any other
  documented construct, a route, an index or a load among them, where
  no other hover answers, the hover is its doc text alone.
- A group hovers as its declaration as written, set terms and all,
  and the tables it resolves to (§groups).
- A migration's name hovers as its position among the package's
  migrations in name order ("migration 2 of 3") and whether it runs
  backward (§migrations); the project index answers it, so a
  migration alone in its own file counts its siblings.
- A select hovers as its row types, the nested row type of a `with`
  select, its rendered `WHERE`, `ORDER BY` and `LIMIT` with what each
  binds, and its method signatures, each with the effect it yields:
  `rt.Query`, `rt.Lookup` for one row of aggregates, `rt.With` for a
  `with` select (§selects-over-groups). A parameter the select names
  in its `where` or as its `limit:`, `:at?` and `:limit` among them,
  hovers as the Go parameter its functions take, saying whether it is
  optional or the limit; one a `pred` names is typed at each use and
  hovers as none, and no parameter renames, since a `pred` may name it
  too. A select kept unresolved by a column
  of unknown type (§columns) hovers as its header, its row types with
  that field, nullable or not, spelled in its declared type and marked
  unknown, and one line naming the unknown columns; never an empty Go
  type, and never a signature or SQL that its dropped terms would make
  wrong.
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

The CLI is pinned: `grammar/package-lock.json` holds tree-sitter-cli
0.25.10, the version the first line of `src/parser.c` names, and
`npm ci` in `grammar/` installs it as
`grammar/node_modules/.bin/tree-sitter`. Run that one: another version
rewrites the committed `src/` from an unchanged `grammar.js` (0.27.1
rewrites `src/parser.c` and `src/tree_sitter/array.h`).
`sync-grammar.sh` regenerates with the pinned version only, from
`grammar/node_modules/.bin` or from the `PATH`, and otherwise leaves
`src/` as committed and says so. Moving the pin is a change of its
own: update the lockfile, regenerate, and commit both.

## 6. Tooling choices

The server is Go and wraps the reference front end, so diagnostics come
from one implementation. The protocol layer is Volt's own
`lsp/protocol`, LSP 3.16 over stdio on the standard library alone
(section 4), so the `lsp` module has no third-party dependency.

## 7. Known limitations (documented trade-offs, not bugs)

- **Grammar: columns named exactly like block keywords.** A line in a
  `table` or `table_partial` body that starts with a column named exactly
  `indexes` or `validators` (`records` too in a `table` body), or a line
  in a create, rebuild or drop table shape body that starts with a
  column named exactly `indexes`, lexes as the block keyword (§keywords
  note above); the front end, spec-exact, reads them as columns.
  Another case of the word (`indexes`) is an identifier to both.
- **Grammar: a multi-line string that opens with `//`.** When the
  first characters after an opening `"""`, spaces allowed, are `//`,
  the grammar lexes the rest of that line as a comment and draws an
  ERROR node (`[default: """// x"""]`); the front end reads the string.
  A `//` later in the body, and a single-line string or an expression
  literal that begins with `//`, parse in both.
- **Grammar: continuations outside a settings list, and a break
  before a setting's `:`.** An element continued on an indented line
  (`ref:` with its relationship on the next line,
  §line-structure-and-whitespace), a parenthesized list spanning
  lines outside a settings list (a composite index key, a records
  column list, a composite ref endpoint) and a line break between a
  setting's name and its `:` (`[default` then `: 1` on the next line)
  draw an ERROR node, since a line break there is not whitespace to
  the grammar; the front end accepts them.
- **LSP: a rename stops at the `.volt` files.** Renaming a table, a
  column, a relation or a select renames the Go names `volt gen`
  derives from it, a model, a field, a function, a with row's type
  (`PostWithReplies`), a joined column's field (`PostTitle`) and its
  JSON key among them, at the next generation, and the server does not
  rewrite the hand-written Go that uses the old ones, which stops
  compiling then; renaming each Go name with gopls first, while the
  generated code still declares it, keeps the two in step. A rename at
  a Go reference is the other way round: the Go side is gopls's, the
  `.volt` spellings the server's.
- **LSP: Go-side navigation is partial.** Handler references (`Home.Index`)
  and `error_handler:` point into Go, which this server does not
  index (roadmap). A query reference (`db.UserGet`, §query-routes) is
  not navigated either; a dataset's select (`dataset db.browse`) is.
  A declared type's hover stays on its `type` line; go-to-definition
  on a column's type lands there, and from the name of the `type`
  line itself on the Go type (Go references, below).

### Go references

A `.volt` file names Go by rule only in the containing package's own
Go files (§projects-and-packages): pipeline plugs (§pipelines),
validators (§validators), scope-load functions (§scope-loads), error
handlers (§scopes) and bare declared types (§types). The server
resolves plugs, validators and scope-load functions with the
standard library's Go parser
over the package directory's non-test `*.go` files (no gopls, no
build): go-to-definition lands on the `func`, hover shows its signature
and doc comment, and an undeclared name hovers as undeclared while the
checker's diagnostic on the reference names the exact signature to
write. `volt.*` plugs live in the runtime and
make no claim. Go-to-definition on the name of a `type` declaration
(§types), either part of a qualified one, lands on the Go type spec: a
bare name in the package's own Go files, a qualified one in the
package its `from` path imports, whose directory the go tool finds
(`go list -find`, offline), so a package of the project, of a
workspace module, of the module cache or of the standard library all
resolve; with no go tool on the path it finds nothing. Error handlers
are not navigated (section 7), but their existence, and a handler's
signature, are checked like the rest. Existence and the spelled signature are the
checker's, so a typo or a wrong parameter type is a diagnostic at the
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
the table's validators (their arguments), and a foreign-key column into
the relation it names (section 4); a column named through a group, in
a `pred` body or a group select, is left alone, and the rename's check
refuses the rename when the agreement error (§selects-over-groups)
would then name what is missing. A relation or a table a `pred` body
names renames with it.

## 8. The verification checklist

How to audit that the three components implement
[`spec.md`](spec.md) — each row is a runnable check, not a promise:

| Claim | Check | Where |
|---|---|---|
| Front end accepts/rejects exactly what the spec says | conformance corpus: `valid/` MUST pass, `invalid/` MUST fail, each snippet tagged `// spec: §…` (files without a `package` clause = the schema pass, files with one and project dirs = the project pass); every diagnostic of every invalid snippet, recovery included, is a golden | `lang/conformance/snippets/` and `lang/conformance/invalid_*.golden` via `go test ./lang/...`; refresh with `-update` after reading the diff |
| Grammar parses everything the spec allows | corpus cases (input → expected tree, incl. `:error` cases) | `grammar/test/corpus/`, `tree-sitter test` with the pinned CLI, tree-sitter-cli 0.25.10 (section 5) |
| Spec grammar and front end agree | the spec's EBNF read as data: every derived sentence parses, every one-token neighbour is decided alike by the grammar and the front end | `lang/spec_grammar_test.go` via `go test ./lang/...` |
| Grammar and front end agree | every valid conformance snippet (the files its loader reads, so not `testdata/` or a nested module) and `grammar/examples/*` must produce zero front-end diagnostics, and every derived sentence of the spec's grammar (`go test ./lang -run TestSpecGrammarSentencesParse -sentences DIR`) zero parser diagnostics; all of them must produce zero tree-sitter ERROR nodes, except where a limitation of section 7 applies: today `v55_element_continuation` (a `ref:` continued on an indented line) and `34_no_reserved_words` (a column named `indexes`) | differential run after grammar changes |
| Highlight queries cover the grammar | every keyword `grammar.js` matches (`kw(...)`) is captured in `highlights.scm`, by its spelling or through the node it is aliased to (a verb, a boolean, `null`, an aggregate name) | `lsp/highlights_test.go` via `go test ./lsp/...`; `tree-sitter query zed-extension/languages/volt/highlights.scm FILE` from `grammar/` shows the captures |
| Spec examples run | every `volt` block of `spec.md` (§conformance-and-the-proof-chain): a plain block is a whole file the front end accepts, a block fenced `volt invalid` one it rejects, a block fenced `volt fragment` a piece of a file that parses alone; any other fence marker fails | `lang/spec_examples_test.go` via `go test ./lang/...` |
| Lint rules match their doc | doc ↔ registry ↔ testdata consistency test; `//WANT` markers both directions | `lang/vet/docs_test.go` against [`lint.md`](lint.md) |
| Generated routers implement Part II of the spec | goldens byte-compared, gofmt-stable, **compiled**; itest exercises match/404/405, typed-param 404s, pipeline order, the error spine, reverse-URL round-trip totality | `gen/router`, `itest/` |
| Generated models implement §mapping-to-go/B | goldens compiled; every CRUD statement prepared against the generated DDL; SQL goldens executed on real SQLite | `nao/gen/...`, `nao/itest` |
| LSP behaves per spec | in-process unit suites: index, rename spelling rules, hover content, completion contexts, UTF-16 | `lsp/*_test.go` via `go test ./lsp/...` |
| LSP behaves per spec, interactively | a scripted stdio JSON-RPC session against the real server process, replaying keystrokes that break and fix a project file, one of them as a ranged change, and asking for hover, definition, completion and symbols; every reply and published diagnostic pinned as a golden | `cmd/volt/lsp_session_test.go` and `testdata/lsp_session.golden` via `go test ./cmd/volt/...`; refresh with `-update` after reading the diff |
| A keystroke costs what the budget says | the whole in-process path of one keystroke inside one table of a one-file project, didChange to publish, and the document's own front end alone; compare against the last report of `scripts/perf.go`. The bytes the keystroke's three parts allocate (the document's pass, the project analysis, the publish) are pinned exactly, within ten percent, hardware-neutral | `go test ./lsp -run '^$' -bench 'BenchmarkKeystroke|BenchmarkDocumentUpdateLocal' -benchmem`; `go test ./lsp -run TestKeystrokeAllocationBudget -v` |

Nothing checks the corpus against the DBML implementation Part I
descends from (spec §license-and-provenance); the conformance corpus
alone is the record.

When adding a construct, every row above must gain its case — §program-structure's
recipes say where. A construct that exists in code but has no row here
is unverified, and that is a bug in the process, not a smaller task.
