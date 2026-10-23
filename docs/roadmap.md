# Roadmap

> [!CAUTION]
> **This project is an alpha version, and everything may change in the future.**

What Volt is made of and whether each piece is built. `[x]` built and
tested, `[ ]` not built, `[~]` partly built. *hand* means a documented
pattern, deliberately hand-written, never generated; *no* means a
non-goal. What an item is lives in the spec; a
status changes in the commit that lands it. Build order: raw-SQL
selects, then associations, then the migration generator, then the
report engine when a consumer exists.

## Language and front end

- [x] Spec: EBNF, numbered constraints, conformance corpus
  - [x] The grammar is executable and proven against the front end
  - [x] Invalid input is a rule with goldens; the editor session is a golden
  - [x] Executable spec examples: every `volt` block of the spec runs through the front end in `go test`, plain, `volt invalid` or `volt fragment`
- [x] Scanner, parser, checker, vet; project loading from `go.mod`
  - [x] Parse and check by declaration; routes lowered per scope item
  - [x] Named placeholders, Go references checked by spelling
- [x] Schema core: Project, Table, TablePartial, Records, Type, Ref, notes
  - [x] One spelling per meaning: `pk` is the key, a column is NOT NULL unless it carries `null`, settings follow the type in brackets only, the files of a package share a namespace without imports
- [x] One spelling per meaning, decided in review and built in steps; docs/entropy-and-purity-report.md keeps what is still open
  - [x] Step 1, removals
    - [x] Enums go: `Type OrderState string` with Go constants and a membership validator replaces `Enum`. In the same commit, a declared type whose Go declaration spells its storage type (`bool`, `int`, `float64`, `string`) needs no Scanner or Valuer, `<Model>New` seeds its literal defaults, `required` compares it directly, its name is exported and a bare one joins the generated name scope (the router's and the migration list's names included) so a collision is a check error, and `-o DIR` with a bare declared type is refused unless DIR is the package's own directory
    - [x] Diagram-only constructs go: `headercolor`, Ref `color` and `inactive`, `TableGroup` (`Group` is the one set of tables, in the tables' namespace), sticky notes, `DiagramView`, the color literal
    - [x] A note has one spelling, the `note:` setting; the body `Note: '...'` and `Note { '...' }` forms go
    - [x] Relationships have one direction, the foreign key on the left: `<`, `-` and `<>` go; an inline ref is `ref: users.id` with no operator; the Ref element keeps only `Ref: posts.author_id > users.id`, without the long form or a name; a one-to-one is `unique` plus a ref
    - [x] Table aliases (`as`) and schema qualification go; a table is named by its name only
    - [x] Settings nothing reads go: index `type:` and `note:`, `note:` on migrations, migration columns and migration indexes; a TablePartial takes no settings and no note
    - [x] NULL has one spelling: `default: null` goes, a record row writes `null`, and an empty field is an error
    - [x] The spec says what the code does: punctuation table, the doubled exclusion backslash, the multi-line string and `import ( db )` examples, §table-resolution, the empty Reserved headings, version relics
  - [x] Setting names and values are one word: `set_null`, `set_default`, `no_transaction`; `no action` goes
  - [x] A nullable []byte is rt.Null[[]byte] like every other nullable type; a plain []byte is bytes, and nil writes empty bytes
  - [x] Step 2, case and names
    - [x] Volt is case-sensitive. Words that open an element or a block are capitalized (`Table`, `TablePartial`, `Type`, `Ref`, `Group`, `Pred`, `Select`, `Records`, `Indexes`, `Validators`, `Migration`, `Scope`, `Pipeline`, `Project`); every other word is lowercase (settings and values, `true`, `false`, `null`, clause words, route words, migration operations, `package`, `import`); a wrong case is a syntax error naming the right spelling
    - [x] Identifiers are Go identifiers: quoted identifiers go
    - [x] Strings are Go strings: `"..."` is single-line with only the escapes `\\`, `\"`, `\n` and `\t`, every other backslash sequence and line continuation an error; a multi-line string is `"""..."""` with its common indentation trimmed, replacing `'''...'''`; backticks stay SQL expressions (`default:`, `where:`, `from:`, `exec`, index expressions)
    - [x] A package names its own declarations bare; `pkg.Name` always names an import (or `volt.`), and the rule that an import may not take the package's own name goes
  - [x] Step 3, keys and indexes
    - [x] `pk` is a column setting only: several `pk` columns form one composite key in declaration order, the `[pk]` index form goes, `increment` only on a single `int` pk
    - [x] A single-column index is the column setting `unique` or `index`, emitted as `CREATE [UNIQUE] INDEX` with a derived name; the `Indexes` block holds composite, expression and partial indexes only, and a plain single-column index there is an error; a partial or expression unique index never makes a column unique (GetBy, foreign-key targets); the fold compares indexes by table, key, uniqueness and condition, not by name, and a primary key by its columns and their positions; two indexes of the schema with one name, or an index named as a table, compared as SQLite compares names, are a check error, and so are an empty `name:` and one beginning with `sqlite_`; a typed `drop` of a `unique` or `index` column drops its index first
  - [x] Step 4, routes
    - [x] `:slug(string)` goes, a bare path parameter is a string and `int` the one type written, `(string)` an error naming the bare form; PUT goes, PATCH is the partial update: a `put` line is one error naming the verb the line takes (`patch` on a controller route or an Update, the fixed verb on another query route, `get` on the event route), and resources' `update` is one PATCH route; a query route's verb is fixed by the generated method (List, Get and selects `get`, which ServeMux also serves for HEAD; Create `post`, Update `patch`, Delete `delete`), any other verb an error naming the right one, while a controller route keeps every verb; a `get` and a `head` route overlap in the conflict check, as they do in ServeMux
- [x] `Migration`: the element, its contextual keywords and its twelve checks; what runs it is under the data layer
  - [x] Column types are a closed Go set (`bool`, `int`, `float64`, `string`, `[]byte`, `time.Time`) and `Type` declarations; defaults are typed; a bare `Type` is held to its Go declaration, which decides whether it needs a Scanner and a Valuer
- [~] Schema extensions
  - [x] `[model:]`, `[tag:]`, `[required]`, `Validators`
  - [x] `Group`, `Pred`, `Select … for <group>` with projection, list parameters
  - [ ] Raw-SQL `Select` / `View` blocks
    - [ ] `Select name (params) { … }`: declared params, select list, from and joins, opaque where/having/group/order bodies
    - [ ] One result struct per block, typed function, single-row variants
    - [ ] Joins inferred from a unique declared ref, `on:` override
    - [ ] Every block prepare-validated against the generated DDL
    - [ ] `View`: `CREATE VIEW`, read-only struct, declared columns verified against the SQL
    - [ ] `Trigger`: deferred, drop-and-recreate
    - *hand* raw SQL strings in `.go` files; promote them to a block
  - *hand* integer value sets: `Type Priority int` with Go constants and a membership validator beside it, a kinded declared type (§types); the planned `[repr: int]` enums are retired
  - [ ] `[lock_version]` optimistic locking
- [x] Citations by heading name: every `§…` is the heading's anchor, diagnostic codes are `spec/<anchor>`, the test resolves them

## Data layer (nao)

- [x] Models: structs, `Null[T]`, singular names, notes as doc comments
- [x] DDL and seed `INSERT`s from `Records`
- [x] CRUD: `Get`, `GetMany`, `GetBy*`, `List`, `Count`, `Create`, `Update`, `Delete`; keyless tables get `List`/`Create`/`Count`; `Create` and `Update` take a changeset and return the stored row
- [x] Dynamic queries: predicates as values, flat typed handles, per-model wrappers, shared verbs, keyset pagination, statement cache, `dynname` vet rule
- [x] Changesets: the single write path (§changesets); `<Model>New()` and `row.Changeset()`, one setter per column, `Cast` of a `rt.Params[M]` wire body behind an allow list, `Validate`, `Create` and `Update` rendered from the marked columns; every refused write, a failed validator or the database's own refusal alike, is one `*rt.Invalid` itemizing its field errors, 409 for a `unique` violation, 422 otherwise
- [x] Validation: validators are exported Go functions of the package named in the schema, `required` is the built-in one, nothing reaches the DDL; `Validate()` on the changeset runs `required` and the validators, every failure collected; the API path answers with the changeset's `*rt.Invalid`
  - [ ] `[on: create]` / `[on: update]` as triggers in SQL and, in Go, by whether the changeset was made from a row
  - *hand* membership in a value set: a validator written beside the declared type (§types)
  - [ ] `[message: "…"]` with a runtime hook keyed by table and check
  - *no* named validation sets
- [x] Transactions: one `Queries` over `DBTX`, `Tx` with rollback on error or panic, single-writer `Open`
  - [ ] Savepoints
- [ ] Associations
  - [ ] Single loaders: `PostAuthorGet(ctx, post)`, `UserPostsList(ctx, user)`
  - [ ] Batched loaders: `PostCommentsLoad(ctx, posts) map[int][]Comment`, one `IN`, grouped in memory; the atomicity answer to loops of reads
  - [ ] Many-to-many through junction tables
  - *no* association fields on row structs; *hand* polymorphic and STI patterns
- [~] Migrations
  - [x] The `Migration` element: hand-written structural operations in a flat body or `up { } down { }`, checked by rules 1 to 12 of §migrations without the schema, in the grammar, the outline, completion and hover
  - [x] Index names derive from the table and the key columns, an expression index carries `name:`, `where:` makes an index partial (§indexes)
  - [x] The fold: every migration applied to an empty SQLite, compared with the schema's DDL table by table; `volt gen` fails on drift (§the-fold)
  - [x] The ledger `volt_migrations` and `rt.MigrateUp`, `rt.MigrateDown`, `rt.MigrateStatus` (§migration-ledger)
  - [x] `volt migration new | check | up | down | status | sql`, and `volt_migrations.go` with `Migrate` for the binary (§migration-tooling)
  - [x] The pragma bracket around `rebuild table` and `drop table`, `foreign_key_check` before commit; `no_transaction` for what needs it
  - [x] No fake `down`: an irreversible migration refuses `down` with its name
  - *hand* data migrations, as `exec`
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

- [x] One binary: `check`, `vet`, `gen`, `routes`, `stress`, `migration`, `lsp`, `version`; `gen --verify` type-checks the output first
- [x] Generated Go gofmt-canonical by construction; goldens compiled by the real toolchain; every statement prepared against the generated DDL
- [x] Parallel schedule on a worker pool; memoized sessions; the purity gate, the immutability walk, the race detector as the bar
- [x] Language server: diagnostics, completion, hover, navigation, rename; incremental sync; keystroke under a third of a second on a thousand-table file
  - [ ] Cross-file column occurrences: a select or a `Type` reference in a sibling file is neither navigable nor renamed
  - [ ] Hover and definition for select parameters, columns inside a Pred body, projection and order columns of a group select; references list a table once; hover ranges on qualified references exclude the qualifier
  - [ ] Navigation into Go symbols: handler references, `error_handler:` and bare declared types need a Go-side index
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
