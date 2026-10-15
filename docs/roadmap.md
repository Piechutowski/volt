# Roadmap

What Volt is made of and whether each piece is built. `[x]` built and
tested, `[ ]` not built, `[~]` partly built. *hand* means a documented
pattern, deliberately hand-written, never generated; *no* means a
non-goal (decision non-goals). The why behind an item lives in
[`decisions.md`](decisions.md) or the spec; a status changes in the
commit that lands it (decision docs-doctrine). Build order: raw-SQL
selects, then associations, then migrations, then the report engine
when a consumer exists.

## Language and front end

- [x] Spec: EBNF, numbered constraints, conformance corpus
  - [x] The grammar is executable and proven against the front end
  - [x] Invalid input is a rule with goldens; the editor session is a golden
  - [ ] Executable spec examples: run the spec's code blocks through the front end in CI
- [x] Scanner, parser, checker, vet; project loading from `go.mod`
  - [x] Parse and check by declaration; routes lowered per scope item
  - [x] Named placeholders, Go references checked by spelling
- [x] Schema core (DBML): Project, Table, TablePartial, Records, Enum, Ref, TableGroup, Notes, DiagramView
- [~] Schema extensions
  - [x] `[model:]`, `[tag:]`, `[required]`, typed `checks`
  - [x] `Group`, `Pred`, `Select … for <group>` with projection, list parameters
  - [ ] Raw-SQL `Select` / `View` blocks
    - [ ] `Select name (params) { … }`: declared params, select list, from and joins, opaque where/having/group/order bodies
    - [ ] One result struct per block, typed function, single-row variants
    - [ ] Joins inferred from a unique declared ref, `on:` override
    - [ ] Every block prepare-validated against the generated DDL
    - [ ] `View`: `CREATE VIEW`, read-only struct, declared columns verified against the SQL
    - [ ] `Trigger`: deferred, drop-and-recreate
    - *hand* raw SQL strings in `.go` files; promote them to a block
  - [ ] `[was:]` rename memory (with migrations)
  - [ ] `[repr: int]` enums: full String/Valuer/Scanner/JSON suite, the int never escapes the process
  - [ ] `[lock_version]` optimistic locking
- [ ] Citations by heading name: every `§…` becomes the heading's anchor, diagnostic codes `spec/<anchor>`, the spec's Citation key deleted, the test resolves names

## Data layer (nao)

- [x] Models: structs, enums, `Null[T]`, singular names, notes as doc comments
- [x] DDL and seed `INSERT`s from `records`
- [x] CRUD: `Get`, `GetMany`, `GetBy*`, `List`, `Count`, `Create`, `Update`, `Delete`; keyless tables get `List`/`Create`/`Count`
- [x] Dynamic queries: predicates as values, flat typed handles, per-model wrappers, shared verbs, keyset pagination, statement cache, `dynname` vet rule
- [x] Validation: typed checks compile to `CHECK` and `Validate()`, Go-reference checks to the validator only, `required`, 422/409 with per-column details on the API path
  - [ ] `[on: create]` / `[on: update]` as triggers in SQL and per-params in Go
  - [ ] Enum membership in the Go tier (hypotheses H5)
  - [ ] `length()` in typed checks, rune-counted on text so the tiers agree
  - [ ] `[check:]` on select parameters, Go tier only
  - [ ] `[message: '…']` with a runtime hook keyed by table and check
  - *no* nullable columns in typed checks; named validation sets
- [x] Transactions: one `Queries` over `DBTX`, `Tx` with rollback on error or panic, single-writer `Open`
  - [ ] Savepoints
- [ ] Associations
  - [ ] Single loaders: `PostAuthorGet(ctx, post)`, `UserPostsList(ctx, user)`
  - [ ] Batched loaders: `PostCommentsLoad(ctx, posts) map[int64][]Comment`, one `IN`, grouped in memory; the atomicity answer to loops of reads
  - [ ] Many-to-many through junction tables
  - *no* association fields on row structs; *hand* polymorphic and STI patterns
- [ ] Migrations
  - [ ] Semantic diff of the schema against the last-migrated snapshot, never SQL parsing; identity is a canonical hash of the checked AST
  - [ ] Renames via `[was: 'old']`
  - [ ] Generated ledger plus `schema_migrations`, each migration pinned to the schema hash
  - [ ] Owned twelve-step rebuild in the pragma bracket, `foreign_key_check` after
  - [ ] Applied by CLI or an embedded runner, never on app start; user binaries embed subcommands and migrations
  - [ ] No fake `down` for destructive changes
  - *hand* data migrations
- [ ] Report engine, when a consumer exists
  - [ ] Schema catalog exported as runtime data
  - [ ] Quarantined interpreter: validated requests, joins only along declared refs, values only behind placeholders, `[]map[string]any` out

## Framework

- [x] Routing: `package`/`import`, pipelines, scopes, routes, `resources`, plugs, per-scope error handlers, reverse URLs, `ServeMux` as the matcher
- [x] Query routes bound to generated queries; `dataset`; `resources … [default]`; event routes over the SSE broker
- [x] Generated client beside every routing package; JSON and GOB by `Accept`
- [x] Layouts: packages, one directory routing its own tables, `go:generate` workspaces via `-o DIR -parts LIST`
- [ ] HTML rendering and typed compiled templates: variables type-checked against the models they render
- [ ] Jobs: declared job types to typed `Enqueue`
- [ ] Mailer: a declaration generating typed send functions over SMTP, once templates exist
  - *no* storage, websockets, inbound mail, rich text: point at existing Go libraries
- [ ] Router conveniences: plugs from imported packages, `volt.WithQueries` / `WithTemplates` / `BuildDeps`
- [ ] Datasets' override ladder: one operation on one table receives the generated default as `next`; `volt routes` shows generated against overridden; column metadata (`Title`, `Unit`, `Dict`) from notes, later first-class settings

## Tooling

- [x] One binary: `check`, `vet`, `gen`, `routes`, `stress`, `lsp`, `version`; `gen --verify` type-checks the output first
- [x] Generated Go gofmt-canonical by construction; goldens compiled by the real toolchain; every statement prepared against the generated DDL
- [x] Parallel schedule on a worker pool; memoized sessions; the purity gate, the immutability walk, the race detector as the bar
- [x] Language server: diagnostics, completion, hover, navigation, rename; incremental sync; keystroke under a third of a second on a thousand-table file
  - [ ] Cross-file column occurrences: a select, enum reference or default in a sibling file is neither navigable nor renamed
  - [ ] Hover and definition for select parameters, columns inside a Pred body, projection and order columns of a group select; references list a table once; hover ranges on qualified references exclude the qualifier
  - [ ] Navigation into Go symbols: handler references and `error_handler:` need a Go-side index
  - [ ] The navigation index's hit path: walk occurrences by table without copying, keep name maps across builds, memoize a select's hover on its members
  - [ ] Cold open of the one-file layout: parse the chunks on the worker pool
  - [ ] Retained chunk texts: a chunk keeps its own bytes rather than pinning the file it was cut from
- [x] Tree-sitter grammar and Zed extension
- [x] Scaling tests, allocation budgets, `volt stress`, the profiling sweep and the performance report of any machine
- [ ] Diagnostics
  - [ ] A Pred that fails typing at a use site is reported at the use, not once per failing select inside the declaration
  - [ ] Two same-named projected selects on disjoint tables name both selects
  - [ ] A projection over an unmapped column type gets a user-facing message
  - [ ] Generated doc comments quote a select's schema spelling
  - [ ] `rt.Null[T]` for non-JSON encoders reached through `tag:`
  - [ ] CLI: a nonexistent argument says "no such directory"; relative paths in diagnostics for relative arguments; cascading import errors collapsed; `Project.Module` either read or dropped
