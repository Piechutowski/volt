# Entropy and purity report

> [!CAUTION]
> **This project is an alpha version, and everything may change in the future.**

A review of Volt made at commit b94d08d, in two parts: places where the
language or the generated API says one thing in two ways, and effects
that could be pure functions. Each item carries the reviewer's verdict
(remove, merge, decide, keep for the first part; do, consider, keep for
the second) and the evidence it rests on.

Items the maintainer has decided are removed from this file. The ones
to build are listed in roadmap.md under "One spelling per meaning";
item 17 (the Go type of a nullable column) was decided as no change, a
nullable column stays `rt.Null[T]`. The numbers are the review's
original ranking, so a gap in the numbering is a decided item. When an
item is decided, it leaves this file in the same commit.

File and line references point at the code as it was at b94d08d. Names
such as purity-bugs/null are the review's scratch demos, which were not
kept; the text of each item records what its demo showed.

## Open follow-ups from the enum removal

The enum removal is built (roadmap.md, §types). These questions it
raises are open:

- Whether a validator of T on a nullable column of T runs on the value
  when it is present and is skipped on NULL. Today a nullable column
  needs a second validator taking `rt.Null[T]`; the rule would help every
  declared type, not only the former enums.
- Query-string binding of a parameter whose type is a declared type
  with a basic underlying type (a select taking `OrderState` could then
  back a query route).
- A vet rule comparing default and record literals against the Go
  constants of a declared type, which needs a scan of constants.
- Go to definition from a `Type` declaration to its Go declaration, whose
  position the Go-file scan records.
- The `<Model><Field>` column handle names take the Go name a per-model
  enum type would naturally have (`OrderStatus` for orders.status), which is
  why the replacement type is named `OrderState`. Whether to change the
  handle naming scheme is open.

## Redundant duplications

Verdicts: remove, merge, decide (the maintainer's call), keep (pays for itself).

### 5. [merge] The two-word setting name and the multi-word values that remain once `primary key` and `not null` are gone, plus a value that equals the default

- [no transaction] (a migration setting name)
- delete: set null / set default / no action (accepted with any spacing and case, e.g. `NO   ACTION`)
- delete: no action ≡ omitting delete: (SQLite's default)

**Proposal.** Make every setting name and value a single identifier (`setting name = identifier`). Delete the name-joining loop, multiWordValue and MultiWord. This finishes the parser simplification the maintainer asked for. Remove `no action`, since leaving the setting out says the same. Spell the other two values as one word each, e.g. set_null and set_default (or drop set default). Rename `no transaction` to one word of the maintainer's choosing.

**Cost.** Two parser loops, refActions and its two diagnostics (decls.go:170, migrations.go:321), migrationSettings, the lowering read, the structure strings, 5 .volt files (12, v80 and the migration snippets), the i20/i207 messages, three spec sections, the grammar.js setting rule. Low.

**Evidence.** docs/spec.md:594 (the setting name production allows spaces), 602-603, 926-927, §migrations grammar. lang/parser/parser.go:803-812 (the loop that joins name words) and 858-894 (multiWordValue and the MultiWord interface); lang/check/settings.go:146-150; nao/gen/sqlite/gen.go:152-157, 419-422; lang/check/decls.go:155-158 (refActions); lang/check/migrations.go:30-31; nao/migrate/lower.go:51; nao/migrate/structure.go:639-649 already treats `no action` as absent. `delete: no action` emits ON DELETE NO ACTION, which has the same effect as no clause. The schema lens claimed restrict ≡ no action; that is wrong and is dropped here: SQLite's RESTRICT fires as soon as the parent key changes, while NO ACTION checks at the end of the statement, so `restrict` stays.

### 25. [merge] Generated names for the same CRUD route depend on how it was declared, controller resources use verb-first names, and singular: and [model:] fix the same name (with opposite precedence in spec and code)

- get /users db.UserList in Scope api → PathAPIUserList, client APIUserList; post /users db.UserCreate → APIUserCreate
- resources db.users [default] in Scope ref → PathRefUsers/PathRefUser, clients RefUsers, RefUser, RefCreateUser, RefUpdateUser, RefDeleteUser
- controller resources → PathUsers, PathUser, PathNewUser, PathEditUser
- resources series [singular: series_item] / Table series [model: 'SeriesItem']

**Proposal.** Use one rule for every derived name: scope prefix + <Model> + <Action>. Default resources then mint RefUserList/RefUserGet/RefUserCreate/RefUserUpdate/RefUserDelete, the same names explicit query routes mint. Controller resources mint UserIndex/UserShow/UserNew/UserEdit. Plural and singular never meet, so singular:, the plural-equals-singular error (§table-resolution clause 6) and the inflector's role in routing go away. [model:] stays as the one naming override.

**Cost.** The naming block of resources() in lang/routes_lower.go. Regenerate the gen/router goldens (paths, client, client_beside, routes) and itest/blog/app, and update about 7 test references in itest/resources_test.go and client_test.go. Snippets v23_singular_override and i44_singular_collision. Spec: §the-action-table, §settings, §table-resolution, §default-handlers, §settings-whitelists. Roughly 15-20 files. Behavior change: fixing a helper collision now also renames the Go model.

**Evidence.** docs/spec.md §the-action-table Helper column (~2177-2189), §settings singular (~2199-2209), §table-resolution clauses 4-6 (2230-2244; clause 5 says [model:] 'wins when both apply'), §default-handlers clause 6, §query-routes clause 4. lang/routes_lower.go:690-692 sets singular from golang.ModelName, then 750-761 overwrites it with singular:, so singular: wins, the opposite of clause 5. lang/routes_lower.go:846-880. gen/router/testdata/blog_app_client.go.golden:70-98 vs 152-213 (verified). AGENT.md requires subject-first, verb-last names; CreateUser, NewUser and EditUser break that rule.

### 26. [remove] Several indexes or validators blocks in one body, whose contents accumulate

- indexes { id } indexes { name } / indexes { id  name }
- validators { A(x) } ... validators { B(y) } / one validators block

**Proposal.** Allow at most one indexes block and one validators block per Table or TablePartial body. Partials still contribute theirs through injection.

**Cost.** One count in tableBodyCheck, snippet 38 turned into an invalid case, two spec sentences. Trivial.

**Evidence.** docs/spec.md:860-861 ('A body MAY contain more than one indexes block; their contents accumulate') and ~2703 (the same for validators). lang/check/check.go:412-419 appends the blocks. Records is limited to one per body (lang/check/decls.go:84-90), so bodies follow two different rules. Snippet valid/38_multiple_index_blocks.volt.

### 27. [merge] Parentheses around a single index key or ref column

- indexes { id } / indexes { (id) }
- (`lower(id)`) [name: ...] / `lower(id)` [name: ...]
- posts.user_id > users.id / posts.(user_id) > users.(id)

**Proposal.** Allow parentheses only around two or more elements. A parenthesized single element is an error that names the bare form.

**Cost.** Two parser checks, one invalid snippet, a rewrite of snippet 10. Trivial.

**Evidence.** docs/spec.md: §indexes `index key`, §relationships-ref `column group`. lang/parser/parser.go:463-484 and 571-585 accept both forms. ast.Index.Composite records which spelling was used, but only diagnostics read it (check/migrations.go:419).

### 28. [merge] Numeric literals have several spellings for one value, and the minus sign is specified three different ways

- 100 / 1e2 / 1E2 / 1e+2 / 100.0
- 7 / 007; -10 / -010
- -5 / - 5
- DDL keeps the spelling verbatim (verified here: DEFAULT 007, DEFAULT 1E2) while Go gets 7 and 100.0

**Proposal.** One canonical spelling per value: decimal digits with no leading zero (except 0 itself), an optional .digits, and an exponent with a lowercase e and no + (or no exponent at all; maintainer's call). The sign is part of the literal (-5, never - 5) and is admitted the same way in defaults, records and predicates.

**Cost.** A scanner change plus a leading-zero diagnostic, the EBNF number and predicate literal productions, 3 snippets, the predicate sentence in the spec. Low.

**Evidence.** docs/spec.md:499-511 (§numeric-literals); 2477-2482 (§predicates lists -010, 007 and 1E2 as binding -10, 7 and 100.0). The sign is `[ "-" ], number` in defaults (~815) and records (~1116), and absent from the predicate `literal` production (~2451), although lang/parser/volt.go:445-450 accepts MINUS NUMBER there. lang/scanner/scanner.go:476-528. Exponents appear in 3 files (32_scientific_numbers, nao/gen/testdata/changesets.volt, i191).

### 29. [merge] Three identifier rules: Part I allows Unicode letters, marks and a leading digit, while Part II requires ASCII Go identifiers

- Part I identifier: Unicode L + M + _, may start with a digit (2fa_codes, 漢字; café precomposed and cafe + U+0301 are two distinct names)
- Part II plain identifier: the same set, without quotes
- ASCII Go identifier (goIdentOK): route segments, params, pipeline names, plugs, handlers, import aliases and segments, resources names, singular:, param:

**Proposal.** Use one identifier rule for the whole language: Go's ASCII form [A-Za-z_][A-Za-z0-9_]* (goIdentOK). This drops categories L and M, the NFC/NFD double spelling, digit-leading names and the X-prefix rule. Migration names keep their own token form.

**Cost.** Scanner isLetter and numberOrIdentScan; delete §unicode-general-categories and rewrite §identifiers rules 1-2; drop the leading-digit rule in mapping-to-go and the prefixing in nao/gen/golang/naming.go; edit 29_unicode_idents, kitchen_sink and hostile.volt. Low.

**Evidence.** docs/spec.md:223-239 (§unicode-general-categories admits marks so that a decomposed é is valid), 377-398 (§identifiers), 1776-1801 (§route-paths rules 2 and 6), §imports rule 2, ~3513 (2fa_codes becomes X2faCode). lang/scanner/scanner.go:218-220; lang/semantics.go:946-960 (goIdentOK). Reviewers verified that columns café (NFC), café (NFD) and 漢字 check clean and generate the fields Cafe, Café and 漢字: two visually identical columns get different Go names, and 漢字 is unexported in Go, so encoding/json silently drops it.

### 30. [merge] Names that differ only in case are distinct in Volt, but SQLite folds them into one

- Table users and Table Users
- columns Name and name

**Proposal.** Lookup stays case-sensitive, but two names in one namespace that differ only in case are a check error. This promotes casecollision from vet into check.

**Cost.** Move the rule from vet to check; its testdata becomes an invalid snippet, and the lint.md section is removed. Low.

**Evidence.** docs/spec.md:246-247. docs/lint.md:283-292: casecollision is only a vet warning. Reviewers verified that Table users plus Table Users [model: 'Other'] passes check and gen, and the generated DDL then fails in SQLite (volt migration check: table "Users" already exists).

### 31. [merge] A setting value that names something is a string in some settings and an identifier in others, even for the same key

- [model: 'Person'] must be a string (model: Person is rejected)
- [singular: post], [param: plot_id], [name: root] must be identifiers (name: 'web' is rejected)
- index [name: 'idx_booking_date'] is a string, while index [type: hash] is an identifier
- [strip: 'da_'] is a string, although da_ is an identifier

**Proposal.** A value that names something (a Go name, a SQL name, a name prefix) is an identifier: model: Person, name: idx_booking_date, strip: da_. Strings remain for prose (note:), paths (from) and opaque text (tag:).

**Cost.** The setting kind maps, the modelname lint message, about 18 .volt files, the spec tables. Low.

**Evidence.** docs/spec.md:698-702, 858-877, 2065-2069, 2179-2186, 2323-2327. lang/check/settings.go:39 (model: strValue); lang/semantics.go:444 and lang/routes_lower.go:379 ('name: takes an identifier'); lang/routes_lower.go:211, 740-762. Reviewers verified both errors. model: is used in 7 files, index name: in 6, strip: in 5.

### 32. [merge] Raw SQL has two lexical forms: backtick expression literals, and escaped single-quoted strings in exec

- `now()`, [where: `country != ''`], [from: `CAST(hits AS INTEGER)`]: raw, no escapes
- exec 'INSERT INTO accounts (owner) VALUES (\'root\')' [down: 'DELETE FROM accounts WHERE owner = \'root\'']: an escaped string

**Proposal.** exec and its down: take an expression literal, so every piece of raw SQL in the language is in backticks, raw and never escaped.

**Cost.** The parser's exec operand becomes FUNCEXPR; the check kind of down: and the drift hint writer change; about 7 .volt sites and 5 Go test strings. Low.

**Evidence.** docs/spec.md:531-542 (§expression-literals), 2985-2986 and 3117-3124 (exec). valid/v74_migration_operations.volt:34 needs an escaped quote for every SQL string literal. Also v72, i177, i216, nao/migrate/testdata/blog.volt:71,120, nao/migrate/lower_test.go:180-181 (the drift hints print exec '...') and nao/gen/golang/migrations_test.go:34.

### 33. [merge] Irreversibility is marked three ways, and one of them discards a written down block

- Migration 0002__x [irreversible] { rebuild table t { ... } }
- Migration 0002__x { up { ... } }: an up block without a down block is irreversible
- Migration 0002__x [irreversible] { up { ... } down { ... } }: the down block is checked, then thrown away

**Proposal.** Keep one mark. Either [irreversible] is the only mark and an up block without a down block is an error asking for it, or a missing down block is the only mark and [irreversible] goes (a flat body with a non-invertible operation must then use up/down). In both cases, refuse [irreversible] together with a down block.

**Cost.** migrationCheck, the 5 snippets that use [irreversible], rule 12 and its worked example (0002__users_bio_and_int_hits). Low.

**Evidence.** docs/spec.md §migrations rule 12. lang/check/migrations.go:153-160 runs operationsCheck(d.Down) and then sets reversible = !irreversible && HasDown(). nao/migrate/lower.go:60-63 emits no Down when the migration is not Reversible.

### 34. [merge] Group declaration: a block form and an `=` form, a parenthesized set term that repeats the operator chain, and a different shape for Pred

- Group series { ms_revenue <newline> ms_usage } / Group series = ms_revenue + ms_usage
- Group g = commerce \ (audit_log, api_keys) / Group g = commerce \ audit_log \ api_keys
- Select s (* \ (bio, avatar)) for t / Select s (* \ bio \ avatar) for t
- Pred current { org = :org and year = :year } (braces) beside Group wide = series + ks_costs (=)

**Proposal.** Keep the operator chain, which is needed because + and \ mix, and remove the parenthesized set term in both group algebra and projections. Then give Group and Pred one declaration shape. The natural choice is braces around one expression (Group wide { series + ks_costs }, Pred current { ... }), because `=` reads badly for Pred, where `=` is also comparison. Allow a line break after an operator for long lists.

**Cost.** groupDecl, groupTerm and selectProjection; the grammar (spec, collected grammar, tree-sitter); the 19 block-form or 7 expression-form snippets, depending on the pick; 4 paren-term snippets including i67; the blog fixture's Group series; the README. Low to moderate.

**Evidence.** docs/spec.md:2398-2440 (§groups grammar; clause 3: 'The block form is the expression form with every member joined by +' and 'a parenthesized term is the union of its names, applied one name at a time'), the §selects-over-groups `column term` and clause 7, the §predicates `pred decl`. lang/parser/volt.go:275-314 (both forms build the same Terms), 315-332 (groupTerm), 333-346 (predDecl), 557-590 (selectProjection). Reviewers verified that (* \ (email, bio)) and (* \ email \ bio) generate the same SELECT. 19 snippets use the block form, 7 the expression form.

### 35. [merge] Overlapping resources action filters api, only: and except: (and the dataset only:/except: pair), with redundant combinations accepted

- resources db.users [api]
- [except: (new, edit)]
- [only: (index, create, show, update, delete)]
- [default] implies api; [default, api] and [api, except: (new, edit)] are accepted silently
- dataset [only: (ms_a)] vs [except: (ms_b)] over the same member set

**Proposal.** Keep one filter. Every use in the corpus hand-writes one action or member beside the macro, which except: covers, so drop api and only:. Better still, drop the form-page actions new and edit from the action table: Volt renders no forms, and a controller that wants one writes get /users/new Users.New. Then api is the default and disappears, and [default] implies nothing. Whether exclusion is spelled except: here and \ in groups is a separate decision (exclusion-spelling item).

**Cost.** The settings switch in resources(); 2 action-table rows if new/edit go; 5 only: snippets rewritten; the 1 api snippet removed; the blog fixture's [only: (index, show, create)]; the README, the spec tables and §settings-whitelists. Low.

**Evidence.** docs/spec.md: the §settings table and clause 1, §default-handlers 2, §dataset-settings. lang/routes_lower.go:718-724 (api and default both set api, with no redundancy check), 725-781, 218-250. Reviewers verified that RESOURCES db.users [DEFAULT, API] and [api, except: (new, edit)] pass check.

### 36. [merge] Records in three placements, one of which depends on partial injection order

- top level: Records posts(id, user_id) { 1, 1 }
- inside the table with columns: Records (id, user_id) { 1, 1 }
- inside the table without columns: Records { 1, 1, 'x' }, meaning all columns in order after partial injection

**Proposal.** Keep one form: top-level Records table(cols) { ... } with a required column list. It can live in a seed file and never depends on injection order. Remove the in-table block and the implicit list.

**Cost.** The records-block parser path, AST field and TableItem arm, the partial 'no records' rule, the cross-form check, the implicit branch in recordsResolve, grammar.js records_block. About 12 snippets (20, 21, 34, 35, v67, v68, i25, i38, i135, i136, i152, i153), mostly mechanical. Low to moderate.

**Evidence.** docs/spec.md §records-sample-data rules 1-2. lang/parser/parser.go:626-658 (separate recordsDecl and records paths); lang/check/resolve.go:62-87 needs a cross-form 'at most one per table' check; nao/gen/sqlite/gen.go:62-80 emits in-table rows first. The implicit list follows §partial-injection-order, so reordering a partial's columns silently reorders the seed values.

### 37. [merge] A foreign key is declared three ways: inline ref:, the Ref element, and the migration column with actions as column settings

- user_id int [ref: users.id]: no actions
- Ref: posts.user_id > users.id [delete: cascade]
- migration column: user_id int [ref: users.id, delete: cascade]

**Proposal.** One spelling for single-column FKs: inline ref: with delete:/update: as column settings, exactly as in migrations, so schema and migration share one production. Composite FKs need a home, and that is the open choice: keep a Ref element for composite keys only, or add a table-level form beside composite indexes. The Ref name, the long form, the visual settings and the reverse operators are gone already (roadmap, Step 1), so all three spellings write the foreign key first.

**Cost.** The Ref parser and AST (lang/parser/parser.go:510-541) and the resolve.go Ref pass; refSettingsCheck/endpointArityCheck move to columns; columnSettings gains delete/update with the 'requires ref:' rule already written for migrations (check/migrations.go:316-319); grammar.js ref_definition. About 14 snippets and about 21 .volt files repo-wide; §relationships-ref. Moderate.

**Evidence.** docs/spec.md §relationships-ref rule 2 and §columns rule 5: an inline ref carries no settings, so any action forces the Ref element. §migrations (~3019) gives a migration column delete:/update: 'since ... a migration has no Ref', so the fold has to match the two spellings (lang/check/migrations.go:315-323). RefInfo already unifies both schema forms (lang/check/check.go; resolve.go:17-48). The element is only needed for composite FKs (§relationships-ref rule 3).

### 38. [decide] Keyword casing mixes PascalCase and lowercase, even among siblings in one body and between the schema and migration spellings of the same word

- one table body: indexes {, validators { beside Records (...) {
- top level: Table, Type, Ref, Pipeline, Scope, Group, Pred, Select, Migration beside package, import
- resources, dataset (lowercase) inside Scope (PascalCase)
- schema Table users { ... } vs migration create table users { ... } with the identical column body; schema indexes vs migration create index

**Proposal.** Pick one casing rule and let case-sensitive matching (first item) enforce it. Option A, the lowest entropy: every keyword lowercase as in Go (table, ref, type, records, ...), so schema and migration bodies share spellings. Option B: PascalCase only for words that open a top-level declaration and lowercase for everything else, with no exceptions (in-body Records becomes lowercase).

**Cost.** Option A: a mechanical rewrite of every .volt file (49 outside the corpus, about 332 snippets), the spec examples, grammar.js, the LSP completion lists and editor.md section 8. Option B: only the in-body Records spelling changes.

**Evidence.** docs/spec.md:405-421 (§keywords), ~2148 (resources), ~2289 (dataset), 2968-2983 and 3007-3009 (migration words); the grammar/grammar.js kw() calls at lines 104-852, each displayed under a canonical spelling.

### 39. [remove] <Model>Distinct has no effect on any table with a key

- q.PostQuery(ctx, PostDistinct())
- q.PostQuery(ctx): the same rows whenever posts has a primary key or a unique column

**Proposal.** Remove <Model>Distinct, rt.Distinct, the distinct flag and the [DISTINCT ] of the rendering rule. A keyless table full of duplicate rows is already what vet missingpk warns about.

**Cost.** dyn.go, rt/query.go and its tests, about 23 goldens (one line per model), spec ~2556 and ~3595, the lint.md dynname suffix list. Low.

**Evidence.** The dynamic layer always selects the full model shape (the nao/rt/query.go header; SelectRender uses <model>ColumnsSQL), so rows with a key are already distinct. nao/gen/golang/dyn.go:185-186 emits Distinct for every model, including id int [pk] models (changesets_dyn.go.golden:21-22, verified). No Volt Select produces DISTINCT, so the `SELECT [DISTINCT ]` of §predicates rule 6 (docs/spec.md:~2556) is reachable only through this option.

### 40. [remove] Two ways to bind Queries to a transaction: WithTx(tx) and New(tx)

- q.WithTx(tx): body `return &Queries{db: tx}`
- New(tx): the same value
- q.Tx(ctx, fn), built on WithTx; rt.Tx(ctx, db, fn) plus New(tx)

**Proposal.** Remove WithTx, since New(tx) already says it. q.Tx stays the one user-facing transaction spelling and calls New(tx). rt.Tx stays exported because generated code calls it, and its doc should say so.

**Cost.** The WithTx emission and the Tx body, one method per *_queries golden, the WithTx call sites in nao/itest and itest, the spec line. About 19 files, mechanical.

**Evidence.** nao/gen/golang/queries.go:534-542 (verified); nao/gen/golang/testdata/changesets_queries.go.golden:18-40; nao/rt/rt.go:38; docs/spec.md:3565-3566, 3604.

### 41. [remove] Three ways to ask whether a changeset is valid

- cs.Valid()
- cs.Err() == nil (Err is implemented as `if c.Valid() { return nil }`)
- len(cs.Errors()) == 0

**Proposal.** Remove Valid. The Go idiom is `if err := cs.Validate().Err(); err != nil { return err }`.

**Cost.** One method in gen.go, one line per models golden and in itest/blog/db, the spec example. About 22 files, mechanical.

**Evidence.** docs/spec.md:2815-2816, the example at 2823-2827, rule 7 (2891-2893); nao/gen/golang/gen.go:250-253; nao/gen/golang/testdata/changesets.go.golden:565-575 (verified).

### 42. [remove] <Model>GetMany, a static name for a one-line dynamic query

- q.PostGetMany(ctx, ids)
- q.PostQuery(ctx, PostID.In(ids...)): GetMany's own body

**Proposal.** Remove GetMany. The dynamic call becomes the one way, and the nil-slice exception disappears from the spec.

**Cost.** The queries.go emission, every *_queries golden, the nao/itest tests, the spec. About 22 files, mechanical.

**Evidence.** docs/spec.md:3572-3588 (§generated-query-surface rules 1-3, including the carve-out 'GetMany on an empty input excepted'). nao/gen/golang/queries.go:618-620; itest/blog/db/nao_queries.go GetMany calls rt.SelectRender(..., UserID.In(ids...)). The only differences from Query: nil instead of [] on empty input, and no statement cache. It is not a query-route target (§query-routes 1).

### 43. [remove] <Model>OrderBy and <Model>Set wrappers whose type argument Go already infers

- PostOrderBy(PostID.Asc()) vs rt.OrderBy(PostID.Asc()): the wrapper's body is exactly that call
- PostSet(PostSlug.Set("x")) vs []rt.Assign[Post]{PostSlug.Set("x")}: the wrapper's body is `return assigns`

**Proposal.** Remove both. They carry values, so inference already works, and two names per model leave package scope and the dynname collision list. If bulk UpdateWhere goes under the assignment item, Set goes anyway. Limit/Offset/After stay (keep item).

**Cost.** Two funcLines and dynWrapperSuffixes in dyn.go, two functions per model in every *_dyn golden, the dynname docs and testdata, the nao/itest call sites. About 22 files, mechanical.

**Evidence.** nao/gen/golang/dyn.go:38, 187-194; changesets_dyn.go.golden (PostOrderBy, PostSet); nao/rt/query.go:302-323, whose Opt doc justifies wrappers only because 'value-less options cannot infer M'; docs/spec.md §generated-query-surface rule 4 (~3595-3596); docs/lint.md dynname (326-340).

### 44. [remove] The generated db.Migrate beside rt.MigrateUp

- db.Migrate(ctx, sqlDB): its body is rt.MigrateUp(ctx, db, Migrations) with the applied names thrown away
- rt.MigrateUp(ctx, sqlDB, db.Migrations)
- volt migration up DSN

**Proposal.** Remove the generated Migrate. Migrations plus rt.MigrateUp/Down/Status is the Go API, still one call. The CLI remains the path that needs no Go.

**Cost.** A few lines in migrations.go, one golden, spec rule 2, 2 non-test callers. Trivial.

**Evidence.** nao/gen/golang/migrations.go:45; nao/gen/golang/testdata/blog_migrations.go.golden:140-142; nao/rt/migrate.go:93; docs/spec.md §migration-tooling rules 2-3 (3249-3268).

### 45. [remove] Three flags select generated output; one is mislabelled and silently ignored

- volt gen -parts models,queries,router,client,sql
- --models-only / -m: behaves as -parts models,router,client, although its help says 'emit only nao_models.go'; ignored when -parts is also given
- --sql: adds sql to the selected parts
- the docs mix single-dash (-o, -parts, -package) and double-dash (--sql, --verify, --models-only) flags

**Proposal.** Make -parts the only selector, with sql either in the default set or named explicitly. Remove --models-only, -m and --sql. Spell every flag with a single dash in the docs, as Go tools do.

**Cost.** partsParse and the flag table, the README, spec ~3613, roadmap.md, the nao/itest header comment, any cmd/volt tests that exercise those flags. Low.

**Evidence.** cmd/volt/main.go:82-87 (flags) and 525-543 (partsParse, verified); docs/spec.md §projects-and-packages rule 8 (1669-1678), §migration-tooling rule 2 (3256-3258), and the §mapping-to-sqlite-ddl header (~3613, which cites volt gen --sql); README.md:49, 55, 119-122; nao/itest/schema.volt:5 ('volt gen ./nao/itest --sql').

### 46. [remove] Not-found and status error sentinels: one value under two names, two values under one name, and two sentinels nobody uses

- rt.ErrNotFound (= sql.ErrNoRows) / sql.ErrNoRows
- volt.ErrNotFound is a different value (an HTTP 404): errors.Is(err, volt.ErrNotFound) is false for a query miss on the server and true for a 404 on the client
- rt.ErrNoRow (Update on a changeset not made from a row), one letter away from ErrNoRows
- volt.ErrBadRequest ≡ volt.Error(400, "") and volt.ErrForbidden ≡ volt.Error(403, "") for errors.Is, with no callers
- client errors: §generated-client says an HTTPError, §formats says a ProblemError, and client.go returns either a ProblemError or a plain statusError

**Proposal.** Drop rt.ErrNotFound and use sql.ErrNoRows, Go's own name, on the server; keep volt.ErrNotFound as the HTTP 404. Rename rt.ErrNoRow to say what it means (e.g. rt.ErrNoKey). Remove ErrBadRequest and ErrForbidden. State one client error contract: always a ProblemError, with the body text as the message when the body is not a Problem.

**Cost.** About 20 files, mostly generated doc comments in goldens; the spec and itest assertions; two unused lines; the client.go fallback. Low.

**Evidence.** nao/rt/rt.go:27-29 (verified); nao/rt/changeset.go:143-145; errors.go:34-39 (verified: no non-test callers of ErrBadRequest or ErrForbidden); client.go:74-83; docs/spec.md §query-routes 3 (~1925), §changesets 8 (~2907), §generated-client 2 (2007-2009) vs §formats 5 (1988-1990).

### 47. [merge] Writing a response: volt.JSON, Render and RenderStatus, plus exported plumbing that nothing outside the package calls

- volt.JSON(w, v): always JSON, status 200, no negotiation
- volt.Render(w, r, v) ≡ RenderStatus(w, r, 200, v); generated code only ever calls RenderStatus, even for 200
- Encode, DecodeBody and FormatOf are exported beside Render and Decode

**Proposal.** Remove volt.JSON, keep one Render(w, r, status, v), and unexport Encode, DecodeBody and FormatOf.

**Cost.** json.go and its test, two generator lines, the router goldens and itest/blog/app/volt_router.go (RenderStatus becomes Render), formats_test, spec ~1982. Low.

**Evidence.** json.go:9; formats.go:69-90, 144-176; gen/router/gen.go:461-465 (verified: RenderStatus only). The only caller of volt.JSON is itest/router_test.go:29. docs/spec.md §formats rule 4 (1981-1984) names only Negotiate, Render, RenderStatus and Decode.

### 48. [merge] The (check, columns, message) triple under three names

- rt.FieldError{Check, Columns, Message}, a named struct with Error()
- rt.Detail, an alias of an unnamed struct with json tags
- volt.Detail, the identical alias; Invalid.Details() copies FieldError into Detail field by field

**Proposal.** Make rt.FieldError the alias itself, with the json tags, so Invalid.Fields already satisfies volt's Detailer and rt.Detail goes. Invalid.Error() joins the fields itself. That leaves one name for the triple on each side of the package boundary.

**Cost.** rt/changeset.go and the rt tests that call FieldError.Error(). The generated rt.FieldError{...} literals stay valid. Trivial.

**Evidence.** nao/rt/changeset.go:24-35, 71-83; errors.go:41-52 (verified: 'nao's runtime declares the identical alias').

### 49. [merge] A database refusal of a write is reported two ways, depending on which method wrote

- q.PostDelete, Create, Update: the refusal becomes *rt.Invalid (409 or 422 through the error spine)
- q.PostDeleteWhere, UpdateWhere: the raw driver error comes back (500 through the spine)

**Proposal.** Wrap the DeleteWhere and UpdateWhere errors in rt.Refused so every write refusal is *rt.Invalid. Correct the 'no other write path' sentence, or remove the bulk writes under the assignment item.

**Cost.** Two lines in dyn.go, every *_dyn golden, two spec sentences, one test. Trivial.

**Evidence.** docs/spec.md §generated-query-surface rule 3 (3590-3591). The §changesets intro says 'there is no other write path for a row' (2793-2794), which rule 11 (2932-2935) contradicts. changesets_queries.go.golden: PostDelete wraps with rt.Refused; changesets_dyn.go.golden:90-98: PostDeleteWhere returns err unwrapped (verified).

### 50. [merge] Row counts are int64, while every other count and key is int

- Count, DeleteWhere and UpdateWhere return int64
- Limit(n int), Offset(n int), int keys, int route parameters

**Proposal.** Return int everywhere the surface counts rows.

**Cost.** dyn.go, the *_dyn goldens, the nao/itest assertions. Trivial.

**Evidence.** changesets_dyn.go.golden:62, 90, 106 (verified); nao/gen/golang/dyn.go:221-260; docs/spec.md §go-types (3500-3502): Volt targets 64-bit platforms, where int carries the full range.

### 51. [merge] Constructor names that change with the output layout, or do not say what they build

- client: client.New(base) in the client subpackage, but NewClient(base) beside the models; the choice depends only on whether -o was passed
- data: New(db rt.DBTX) *Queries, beside NewRouter(c Controllers)
- PostNew() returns a PostChangeset, not a Post
- volt.NewBroker() ≡ &volt.Broker{}

**Proposal.** Give each thing one name regardless of layout: NewClient(base) in both client layouts, like NewRouter, and NewQueries(db) for the data handle. Name the changeset constructor after what it returns (NewPostChangeset() under the New* carve-out), or keep PostNew and state why. Drop NewBroker, since the zero value is ready.

**Cost.** One emitter line for the client, one golden, the itest/blog client and its callers in itest/client_test.go, the README and 2 spec clauses. New→NewQueries touches every *_queries golden and the itest callers. Two NewBroker sites. Low to moderate.

**Evidence.** docs/spec.md §generated-client 2 and 6 (2005-2006, 2035-2037), §query-routes 6 (1947-1950); gen/router/gen.go:282, 672-675; cmd/volt/main.go:305 (ClientBeside: outDir != ""); goldens blog_app_client.go.golden:25 vs blog_app_client_beside.go.golden:20 (verified); events.go:45 (verified: NewBroker returns &Broker{} and the zero Replay uses the default); changesets.go.golden:90.

### 52. [merge] Generated file names and headers use two prefixes and three header forms, all written by one command

- nao_models.go, nao_queries.go, nao_dyn.go, nao_selects.go, nao_validate.go, nao_schema.sql
- volt_migrations.go, volt_router.go, volt_handlers.go, volt_paths.go, volt_routes.go, volt_client.go
- headers: // Code generated by nao gen from X; DO NOT EDIT. / -- Code generated by nao gen sqlite from X; DO NOT EDIT. / // Code generated by volt gen from package app. DO NOT EDIT.

**Proposal.** Use one prefix, volt_, and one header form: // Code generated by volt gen from <source>. DO NOT EDIT. partOf and the stale list follow, and the next gen removes the old nao_ files once. Whether the name nao survives at all (the package path nao/rt, imported as rt, beside volt) is a separate, larger decision.

**Cost.** Renames across all golden families, itest/blog/db and the nao/itest generated files, partOf, the stale list, and the spec, README and editor mentions. About 19 files plus the renames. Moderate, mechanical.

**Evidence.** nao/gen/golang/gen.go:283, dyn.go:143, queries.go:499, selects.go:90, validate.go:49, migrations.go:31; nao/gen/sqlite/gen.go:48; gen/router/gen.go:111; gen/model/gen.go:50-66; cmd/volt/main.go:382 (the stale-file list) and 545-560 (partOf keys on both prefixes). Verified the nao headers in generated output.

### 53. [merge] The spec grammar defines the shape `name [. name]` under five names

- go ref = name, [ ".", name ]
- handler ref = name, [ ".", name ]
- table ref = name, [ ".", name ]
- dataset inline [ name, "." ], name
- Part I type name, and the dotted setting value the checker reads error_handler: app.Errors with (ast.QualIdent, lang/routes_lower.go selfFuncRefIn), which no production of the setting grammar names

**Proposal.** Define one production, `qualified name = name, [ ".", name ]`, use it everywhere, and have the parser call qualName for all of them.

**Cost.** Mostly the spec, plus a small parser consolidation; the EBNF test re-derives its sentences. Low.

**Evidence.** docs/spec.md §pipelines, §handler-references, §resources, §datasets, §collected-grammar-part-ii, §types, §names-and-qualified-names. lang/parser/parser.go:152-159 (qualName); lang/parser/volt.go:111-118 (goRef); dataset() and resources() at volt.go:166-208 each parse their own copy.

### 54. [merge] CLI subcommands take packages in two argument grammars

- check, vet, gen and routes take [packages] with Go patterns (./..., dir, or a .volt file)
- migration new LABEL [DIR]; migration check and sql [DIR]; migration up, down and status DSN [DIR], with the DSN positional and first: one directory, no patterns

**Proposal.** Every subcommand resolves packages through load(), and the DSN becomes a flag (-db DSN), so positional arguments are always packages.

**Cost.** Argument parsing in migration.go, migration_test.go, the spec rules, the README. Low.

**Evidence.** cmd/volt/main.go:57-97, 128-182 (load); cmd/volt/migration.go:39-71; docs/spec.md §projects-and-packages rule 7 (1659-1668), §migration-tooling rule 1 (3229-3238).

### 55. [merge] Runtime plumbing exported twice

- rt.RowScan(ctx, db, cache, q, args, dest...) ≡ rt.StmtQueryRow(...) followed by .Scan(dest...)
- two UPDATE renderers: rt.UpdateRender (assignments plus predicate) and rt.UpdateRowRender (marked columns plus key plus RETURNING)
- the Column.Name field and the ColumnName() method read the same string
- rt.PredCmp/PredIn/PredLike/PredNull: string-typed mirrors of the Column methods, exported for the generator

**Proposal.** Fold RowScan into StmtQueryRow, keep one UPDATE renderer with an optional RETURNING, and drop ColumnName() (generated literals need the field). Keep the Pred mirrors, documented as generator-only.

**Cost.** rt, the generator emit lines, the Count/Exists call sites in every *_dyn golden. About 20 files, mechanical.

**Evidence.** nao/rt/query.go:460, 580; nao/rt/changeset.go:160, 405, 433; nao/rt/gen.go; docs/spec.md §predicates rule 6 ('There is one SQL renderer', 2519-2525).

### 56. [merge] Query parameter names: CRUD keys use the Go name, select parameters use the name as written, and the wire uses the column name

- CRUD key: column user_id becomes Go userID, so the route must be /roles/:userID(int)/:roleID(int)
- select `where user_id = :user_id` becomes Go parameter user_id (PostByuser(ctx, user_id int)) and query key user_id
- JSON body: user_id

**Proposal.** One spelling per surface. On the wire (path segments, query keys, JSON), the column or parameter name as written (user_id). In Go, the lowerCamel name (userID), for CRUD keys and select parameters alike. The binder maps between the two, so :user_id binds userID.

**Cost.** Name matching in queryBind, select-parameter Go naming in nao/gen/golang, the blog fixture paths, the handlers/client goldens and the nao select goldens; §query-routes 2, §default-handlers 4 and §selects-over-groups 6. Moderate.

**Evidence.** docs/spec.md §query-routes clause 2 (binding by name), §default-handlers 4. gen/router/testdata/blog/app/routes.volt (/roles/:userID(int)/:roleID(int)); blog_app_client.go.golden:122 (verified: APIUserRoleGet(ctx, userID int, roleID int)); nao/gen/golang/plan.go:196; lang/routes_lower.go:804. Reviewers verified the select signature.

### 57. [decide] The Project element: a name and properties that nothing reads, on a SQLite-only implementation

- Project ecommerce [note: '...'] { database_type: 'PostgreSQL'  custom_key: '...' }
- the Go module, which Part II calls the project (§projects-and-packages)
- a Go package comment in doc.go, Go's own place for package docs

**Proposal.** Recommended: remove Project and let the package doc live where Go puts it (doc.go). Smaller step: keep only one package note, with no name and no properties.

**Cost.** Parser project(), ast.Project, the uniqueness check (check.go:262-267), the vet emptybody branch, both generators' projectNote and the header goldens, lsp/features.go:45, grammar.js project; about 4 snippets (03, 25, 35, i34) and nao/itest/schema.volt; §project and lint.md. Low.

**Evidence.** docs/spec.md:644-667 (§project; the suggested key is database_type: 'PostgreSQL'). Only the note is read, in nao/gen/golang/gen.go and nao/gen/sqlite/gen.go; since the removals a Project carries at most one, the `note:` setting. 'Project' means two different things in the spec. nao/itest/schema.volt declares an empty Project only to carry its note.

### 58. [decide] Two comment forms

- // line comment
- /* block comment */

**Proposal.** Recommended: keep only //. Volt has no doc comments (notes document things), a run of // lines does what a block does, and the /* hazard in route paths disappears. The argument against: Go has both, so a Go reader expects both.

**Cost.** One scanner function and its diagnostic, the EBNF block comment, a grammar.js rule, one Part II paragraph, 3 files. Low.

**Evidence.** docs/spec.md:347-367 (§comments: 'exactly two'); ~1607-1611: /* always opens a block comment, which forced the :name... wildcard spelling in route paths. lang/scanner/scanner.go:275-292; grammar/grammar.js:972-976. Used only by valid/02_comments.volt, invalid/i01_unterminated_block_comment.volt and grammar/examples/kitchen_sink.volt.

### 59. [decide] A literal and a backtick expression wrapping the same literal: the same database default, different Go behaviour

- default: 5 / default: `5`
- records: 1 / `1`

**Proposal.** Keep the backtick escape hatch, since real SQL expressions are needed (CURRENT_TIMESTAMP, lower(x)). Make a backtick body that is just a plain literal (a number, a quoted string, TRUE/FALSE, NULL) a check error naming the literal spelling, or a vet rule if a hard error is too strict.

**Cost.** One check in default and record value typing, one snippet. Low.

**Evidence.** docs/spec.md:838-840 (an expression literal is accepted on any column), 2821-2827 (§changesets rule 2: literal defaults prefill <Model>New(), expression defaults do not). Reviewers verified that b int [not null, default: 5] and c int [not null, default: `5`] give DEFAULT 5 and DEFAULT (5), but PostNew() starts as Post{B: 5}, so C reads 0 until the row is stored.

### 60. [decide] Negation in predicates: equivalent predicates spelled two ways render different SQL, and the runtime has negations the language cannot write

- where bio is not null → "bio" IS NOT NULL; where not bio is null → NOT ("bio" IS NULL)
- where id != 1 → "id" != ?; where not id = 1 → NOT ("id" = ?)
- runtime UserID.NotIn(1, 2) → "id" NOT IN (?, ?), vs rt.Not(UserID.In(1, 2)) → NOT ("id" IN (...)), which is what source `not id in (1, 2)` lowers to; NotLike vs rt.Not(Like)
- Ne, NeCol, IsNotNull vs rt.Not(Eq/EqCol/IsNull)

**Proposal.** Remove NotIn and NotLike and their NOT IN/NOT LIKE renderings; the language has no such operators. For the language, make `not` the one negation (drop `is not null`, and keep != only if SQL familiarity is wanted). Alternatively, keep both spellings and canonicalize in the renderer: NOT over IS NULL renders IS NOT NULL, and NOT over = renders !=. Each predicate then has one output, and Ne/IsNotNull follow the choice.

**Cost.** Two rt methods and their render branches, the rt tests, 3 files that reference them. Then either the null-test grammar/parser and 5 snippets that use `is not null`, or a two-case canonicalization that changes some golden SQL. Low.

**Evidence.** docs/spec.md §predicates grammar (pred unary with not; null test `is [not] null`; comp op includes !=) and rule 6 (one renderer; 'the same predicate spells the same bytes on both paths', 2523-2524; bullets 2532-2537). nao/rt/query.go:43-104, 185; lang/selects.go:917, 946-948, 1129-1145 (the generator never builds NotIn or NotLike).

### 61. [decide] `(int)` on a query route repeats the type that the generated method's signature already fixes

- get /users/:id(int) db.UserGet (required)
- get /users/:id db.UserGet is an error that prints the right annotation
- resources take the same type from the primary key, with no annotation

**Proposal.** On a query route, take the path parameter's type from the method signature, the same way resources take it from the primary key. `(int)` stays on controller routes, where nothing else knows the type. The trade-off: one written path then means int on a query route and string on a controller route.

**Cost.** queryBind adopts the type instead of comparing it. The 13 snippets and the blog fixture drop the annotation on query routes, and i99 goes. Low.

**Evidence.** docs/spec.md:~1893 (§query-routes clause 2: the declared type MUST spell the method's Go type), §settings clause 2, §table-resolution clause 3; lang/routes_lower.go:549-562 computes the right annotation and prints it; invalid/i99_query_path_type.

### 62. [decide] Two rules name a resource's key parameter, and param: overrides only one of them

- controller resources: :id unless [param: plot_id]
- resources db.t [default]: the primary key's Go name (slug, userID), and param: is an error

**Proposal.** Use one rule for both: the key parameter is the primary-key column's name, in its wire spelling (see the parameter-naming item). param: goes. A colliding scope parameter is renamed in the scope, as [default] already requires.

**Cost.** The resources function, 3 snippets, §settings 4. Tables whose pk is id keep :id, so little of the corpus moves. Low.

**Evidence.** docs/spec.md: the §settings table (param, default id) and clause 4, §default-handlers 4; lang/routes_lower.go:671, 762-773, 800-806.

### 63. [decide] A migrated package has two DDL sources

- volt gen -parts sql (today --sql) writes nao_schema.sql, the schema's DDL and the fold's target
- Migrations and volt migration up build the same database, while the spec says a migration is the only path to it

**Proposal.** For a package with migrations, either skip or refuse the sql part (the migrations are its DDL, and volt migration sql prints them for review), or keep nao_schema.sql and label it review-only.

**Cost.** One condition in genRun or the sqlite generator, and one spec sentence. Trivial.

**Evidence.** docs/spec.md §mapping-to-sqlite-ddl (3611-3644), §migration-tooling rule 4 (3275-3277), §the-fold rule 1 (3170-3174).

### 64. [decide] Pipelines use four words for one concept, and the `use` keyword carries no information

- element Pipeline api { ... }
- scope setting [pipe: api]
- line keyword use volt.RequestID
- the spec calls each line a plug, while validators lines and group members carry no keyword

**Proposal.** One word per concept: [pipeline: api], or rename the element Pipe. Drop `use`, since a Pipeline body holds only references: Pipeline api { volt.RequestID <newline> BearerAuth }, like a validators block.

**Cost.** plugLine, one settings key, the tree-sitter grammar, 6 Pipeline snippets, the blog fixture, the README; §pipelines and §scopes. Low.

**Evidence.** docs/spec.md §pipelines grammar (plug = "use", go ref, newline), the §scopes table (pipe); lang/parser/volt.go:90-110 (plugLine requires use). The decided removal of file imports leaves `use` with only this job.

### 65. [decide] A package path is written in two lexical forms

- project package: import ( d2 shared/dicts ): bare slash-joined identifiers, contiguity-checked
- Go package: Type money.Money int from 'example.com/app/money': a string

**Proposal.** Either write project import paths as strings, as Go does, which removes package-path contiguity (route paths keep it) and the plain-identifier-only rule for path segments, or keep the bare form.

**Cost.** Parser importDecl, the contiguity check for package paths, grammar.js, the import snippets. Low.

**Evidence.** docs/spec.md:1689-1736 (§imports), ~1601-1606 (the slash token), ~3405-3408 (contiguity for package paths), §types.

### 66. [decide] Removing names from a set is an operator in groups and projections, and a setting in resources and datasets

- Group g = Commerce \ (audit_log, api_keys); Select s (* \ (a, b)) for t
- resources db.tags [except: (delete)]; dataset db.browse [except: (da_r_r)]

**Proposal.** One spelling for exclusion across the language: the backslash everywhere (dataset db.browse \ da_r_r) or the word except everywhere. Or keep both, on the grounds that one operates on table and column sets and the other on generated actions.

**Cost.** Parsing of the resources and dataset settings, or of group expressions, plus their snippets. Moderate.

**Evidence.** docs/spec.md:2392-2422 (§groups), the §selects-over-groups projection, 2179-2189 (resources only/except), 2323-2334 (dataset only/except).

### 67. [decide] In a Select, filtering is a clause but ordering is a setting, and the setting's value has no grammar production

- Select rows for series where current
- [order: (year desc, id asc)]

**Proposal.** Either every part of a Select is a clause (... where current order year desc, id asc), or order: stays as its one setting and gains an `order list` production in the grammar.

**Cost.** The grammar, the parser, the selects check, and the 6+ snippets that use order:. Low.

**Evidence.** docs/spec.md §selects-over-groups grammar and clause 5; lang/selects.go:732-740 (order is the only select setting). The `setting value` grammar (§collected-grammar-part-ii) offers only `ident list`, which does not cover `year desc`; lang/parser/parser.go:840-860 parses the direction word as an unspecified modifier.

### 68. [decide] Select row types are named and shaped by two rules

- Select summary (site, day) for traffic → one shared struct Summary, with default db/json tags only
- Select public (* \ target) for link_clicks → a per-member struct LinkClickPublic, with tags and doc comments copied, the same identifier as the method

**Proposal.** One rule. Either both forms mint per-member derivatives (and the shared struct needs an explicit opt-in), or a uniform suffix (SummaryRow, LinkClickPublicRow) gives both forms one shape and keeps a bare select name from colliding with a model. Or keep both and state when each is meant.

**Cost.** selects.go naming, the select goldens, the collision checks, one spec paragraph, i65's semantics. Low to moderate.

**Evidence.** docs/spec.md §selects-over-groups rule 7 (2647-2684); nao/gen/golang/testdata/rollups_selects.go.golden:92, 145.

### 69. [decide] Dynamic membership: In versus InList

- UserID.In(ids...) → IN (?, ?, ...): the text changes with list length, so each length is a separate cached statement
- UserID.InList(ids) → IN (SELECT value FROM json_each(?)): one text for any length. The language uses In for a literal list and InList for a :ids parameter

**Proposal.** One method: In(vs ...T) renders json_each whenever it is given values, matching select list parameters and the cache, and InList goes. time.Time, which JSON cannot carry as a SQLite timestamp (§predicates rule 3), either keeps the placeholder form internally or is refused, as it already is for list parameters. Or keep both and document In as the literal form.

**Cost.** query.go, GetMany's body if GetMany stays, the rt tests, 7 files that mention InList, the spec bullets. Low.

**Evidence.** nao/rt/query.go:60-76; lang/selects.go:1114, 1129; docs/spec.md §predicates rule 6 (2532-2535), §selects-over-groups rule 6 (2638-2641).

### 70. [decide] Reversible migrations as a flat body with derived inverses or as up/down blocks, with inverse annotations accepted where nothing reads them

- flat exec 'CREATE VIEW v ...' [down: 'DROP VIEW v'] / up { exec '...' } down { exec 'DROP VIEW v' }
- flat drop table t { shape } / up { drop table t } down { create table t { shape } }
- flat alter table t { drop c string [...] } / up and down blocks with add

**Proposal.** Choose one model. (a) Flat only: each operation carries what its inverse needs, [irreversible] marks the rest, and up/down blocks go. (b) Up/down only: drop exec down:, drop-table shapes, typed drops and the extra drop-index settings. Either way, refuse inverse annotations where they cannot take effect.

**Cost.** (a) 3 up/down snippets, one grammar alternative and one lowering branch. (b) The exec settings production, the drop shapes, inverse() in nao/migrate/lower.go, 2 exec-down snippets and a rewrite of rule 12. Low to moderate.

**Evidence.** docs/spec.md §migrations grammar and rules 6, 8, 10 and 12. lang/check/migrations.go:210-245 still accepts and checks exec [down:], drop-table bodies, typed drops and drop-index settings inside up/down blocks. nao/migrate/lower.go reads them only to derive a flat body's inverse (inverse()), so inside blocks, and under [irreversible], they do nothing.

### 71. [decide] check, vet and lint: two commands and two nouns for the analyzer, and three meanings of 'check'

- volt check (errors) / volt vet (check plus warnings, -werror)
- 'vet rules' (lint.md title, vet/<rule> codes, lang/vet/) / 'lint' (the spec's 'lint annex', lang/lint.go, 'the dynname lint', and volt's own help text 'check, lint and generate')
- volt check (no fold) / volt migration check [DIR] (the fold) / volt gen (check plus the fold, failing on drift), so a clean volt check does not mean volt gen will succeed

**Proposal.** Use one noun, vet, for the rules: rename lint.md, lang/lint.go and the spec mentions (or the reverse). Decide whether volt check prints warnings (one command, with -werror) or the Go-style pair stays. Have volt check run the fold for packages that declare migrations (exit 2 without cgo) and drop migration check, or rename it.

**Cost.** Renaming the noun touches file names, AGENT.md, the lang tests that link lint.md and about 5 doc sites. Merging the commands and the fold touches main.go, migration.go, cmd/volt tests, README, editor.md and the spec rule. Low to moderate.

**Evidence.** cmd/volt/main.go:56-76, 183-243; cmd/volt/migration.go:45-47; docs/lint.md:1-38; docs/spec.md:33-35, ~2947, 3528-3529, §the-fold rule 3 (3186-3192).

### 72. [decide] volt gen --verify duplicates the proof chain as a user-facing flag

- volt gen --verify: gofmt-checks and type-checks the output before writing, described as 'a generator self-check'
- go test: byte-compares the goldens, checks they are gofmt-stable and compiles them

**Proposal.** Either remove it, or keep only the part that helps users ('the destination package does not type-check') and describe it that way. If the escape item lands, the known escape-to-invalid-Go path is gone.

**Cost.** About 50 lines in main.go, one roadmap line, any test of the flag. Low.

**Evidence.** cmd/volt/main.go:84, 330-350, 420-468; docs/spec.md §conformance-and-the-proof-chain rule 3 (3446-3449); docs/roadmap.md:88. Counterpoint found in this review: the NUL-escape case produces Go that does not compile, which --verify catches and the goldens do not.

### 73. [decide] Static read conveniences that repeat a dynamic query: List, a bare Select, and GetBy<Column>

- q.UserList(ctx) / q.UserQuery(ctx) / Select all for users → q.UserAll(ctx): identical SQL
- q.PostGetBySlug(ctx, s) (rt.ErrNotFound on a miss) / q.PostQuery(ctx, PostSlug.Eq(s)) (a slice)

**Proposal.** For 'every row', choose one spelling: either List stays as the route target and a Select over one table with no where, order or projection is refused as a duplicate, or List goes and routes and index bind <Model>Query() with no options. For GetBy: keep one name per unique column, or replace them all with one dynamic <Model>One(ctx, preds...) (M, error) that renders LIMIT 2 (ErrNotFound on no row, an error on two). Get stays, since it is the positional key and the route target.

**Cost.** Dropping List: queries.go, every *_queries golden, the qualifying-methods list and the default-handler map in lang, the routes.volt fixtures and snippets that name db.UserList, §query-routes and §default-handlers; about 25 files. GetBy → One: queries.go, about 12 files, the dynname name set, the spec rule.

**Evidence.** docs/spec.md §generated-query-surface rules 1-2 (3574-3581), §query-routes rule 1 (1882-1888), §default-handlers 1 (index maps to List). itest/blog/db/nao_queries.go:47-108; nao/gen/golang/queries.go:632-637; nao/gen/testdata/rollups.volt `Select all for link_clicks` → rollups_selects.go.golden:220 LinkClickAll. GetBy is not a route target.

### 74. [decide] Assigning a value to a column, and assigning NULL, in several spellings; 'Set' with three meanings

- changeset setter cs.BioSet(rt.Some("x")); NULL is cs.BioSet(rt.Null[string]{})
- column handle UserBio.Set("x") and UserBio.SetNull(), collected by UserSet(...) for UpdateWhere
- bulk: cs.From(u) (= every setter except the auto-increment key) and cs.Cast(p, UserBio)

**Proposal.** Setters take typed Go values and Cast takes wire params; the inputs differ, so both stay. Drop From: nothing generated uses it, and H2 says a convenience must earn its place. For column := value, Option A: remove UpdateWhere, rt.Assign, Column.Set, NullColumn.SetNull, <Model>Set and rt.UpdateRender, which leaves the setters as the only path and makes the spec sentence true. Option B: keep Assign as the one spelling and give the changeset a single cs.Set(UserBio.Set("x")), dropping the N minted <Field>Set methods. Either way NULL gets one spelling.

**Cost.** From: one emitter function, two tests, the spec. Option A: the UpdateWhere/Set emission in dyn.go, rt Assign/UpdateRender and their tests, every *_dyn golden, the nao/itest dynamic tests, the partial-update bullet of §predicates rule 6, §changesets rule 11, dynname. Option B: every models golden, the client example (spec ~2022), the itest call sites. About 20 goldens either way.

**Evidence.** docs/spec.md §changesets code block and rules 2, 3, 11 and 12 (2796-2820, 2849-2854, 2932-2935); the intro says 'there is no other write path for a row' (2793-2794). nao/rt/query.go:128, 131; nao/gen/golang/dyn.go:194; nao/gen/golang/gen.go:175 (From), which only tests use (nao/itest/changeset_test.go:155, crud_test.go:193); generated handlers use Cast.

### 75. [decide] Imports are package-scoped, and repeated imports are accepted

- each file imports what it uses, as in Go
- one file imports db, and the package's other files use db. without importing it
- the same import in several files, or two import ( db ) blocks in one file, accepted as idempotent

**Proposal.** Scope imports to the file, as Go does: at most one import declaration per file, each path once, and an import unused in that file is an error. Reading one file then shows where every qualifier comes from.

**Cost.** The checker (imports per file instead of per package), LSP import completion and navigation, and the multi-file snippets that rely on another file's import. Moderate.

**Evidence.** docs/spec.md §imports clause 4 (~1712: 'Within one package (across all its files): importing the same path twice is idempotent'). Reviewers verified that a file using db.PostByuser with no import, and a file with two import blocks, both pass check.

### 76. [decide] Two wire formats, JSON and GOB, for the same rows, params and problems

- Accept / Content-Type application/json
- application/x-gob for the same payloads; Client.Format picks one
- rt.Params has a JSON arm and a GOB arm, and its GOB encoding embeds the marks' JSON (GobEncode writes Fields)

**Proposal.** Decide whether GOB pays for itself. The generated client already shares the model types, and encoding/json round-trips them (rt.Null as the value or null, time.Time as RFC 3339, []byte as base64). JSON-only reduces Negotiate to 'JSON or 406', collapses rt.Params to one arm, and removes Client.Format, MIMEGOB and the arm-specific rules.

**Cost.** The runtime's formats, client and Params code, the format branches in router handlers, formats_test and the itest client tests, §formats and §changesets 5. Moderate.

**Evidence.** docs/spec.md §formats 1-5 (1953-1990), §changesets rule 5 (2868-2880); formats.go:17-90; client.go:18; nao/rt/changeset.go:160-292 (GobEncode ~240). The two arms treat unknown fields differently in hand-written controllers (§formats 2: JSON gives an itemized 400, GOB ignores them).

### 77. [decide] One word, Query, for five things

- Queries, the data handle
- <Model>Query, the dynamic select
- 'query route', a route bound to a generated method, and the volt.Route.Query field
- volt.Query(k, v), a URL option that adds a query-string parameter, plus QueryParam, QueryParams and FormatQuery
- -parts queries, which also carries Validate and volt_migrations.go

**Proposal.** Rename one or two so each word means one thing, for example <Model>Query → <Model>Where (it only filters the fixed shape), the volt.Query URL option → volt.URLParam, and the queries part → data.

**Cost.** Depends on the pick; renaming <Model>Query alone touches every *_dyn golden and nao/itest. Moderate.

**Evidence.** paths.go (Query URLOption); query.go (QueryParam); volt.go (Route.Query); nao/gen/golang/dyn.go (<Model>Query); cmd/volt/main.go:86, 545-560; docs/spec.md §query-routes, §generated-client rule 3.

### 78. [decide] Flat package-scope naming, and the machinery that repairs its collisions

- column handles <Model><Field> (PostSlug)
- option wrappers <Model>Limit/Offset/Distinct/OrderBy/After/Set; <Model>Changeset/Column/Params/New; select row types <Model><Select>
- the package's bare declared types, which share the scope; one that takes a generated name is a check error suggesting a free one (§types), so the type of orders.status is OrderState and not OrderStatus
- the dynname vet rule and generator errors catch whatever still collides

**Proposal.** Decide whether one rule can replace the collision machinery. For example, group column handles per model (var PostC = struct{ ID rt.Column[Post, int]; ... }, used as PostC.Slug.Eq(...)) so a field name can never meet a wrapper or changeset name. The enum removal took the E prefix out; the removals proposed elsewhere (OrderBy, Set, Distinct, Valid) shrink the set further.

**Cost.** Every column-handle use site, a rewrite of the dynname rule, all the model and dyn goldens, the spec's naming text. Large.

**Evidence.** docs/lint.md dynname (326-363); docs/spec.md §names (3515-3529), §changesets rule 12 (2945-2948), §selects-over-groups rule 7 (2678-2684); nao/gen/golang/dyn.go:38.

### 79. [decide] The spec states the same facts in several places

- settings: a generic production plus per-element EBNF in Part I, and whitelist tables in Part II
- §settings-whitelists repeats the Scope, route, resources and dataset tables, each of which already calls itself complete
- §collected-grammar-part-i/ii copy every production, minus an exception list
- required's semantics appear in §columns, §validators 8 and §changesets 9; PUT-as-PATCH in §query-routes 2 and §default-handlers 6; the Validate contract in §validators 6 and §changesets 6 and 12
- §generation-contract-informative restates §handler-references, §query-routes 5 and §generated-client

**Proposal.** State each fact once and cite it elsewhere. Keep the generic settings production in the grammar and give every element one whitelist table (key, value kind, meaning). Drop §settings-whitelists, or the per-section tables. Drop the collected grammars or generate them; the EBNF test already reads every block. Keep required's semantics in §columns and the Validate contract in §changesets.

**Cost.** A spec rewrite and the grammar test; no code and no language change. Moderate effort.

**Evidence.** docs/spec.md:591-598 vs 683-685, 722-730, 857-863, 921-925, 983-984, 1186, 1215-1218, 2954-2957; Part II 2063-2069, 2179-2186, 2323-2327, 2350-2364; 1370-1583 and 3327-3425. The real whitelists are the settingSpec maps in lang/check/settings.go and lang/check/migrations.go, and grammar.js parses settings generically. lang/spec_grammar_test.go:158-190 enforces the collected copies.

### 80. [keep] float64 as the floating-point column type (the maintainer asked what it means)

- total float64 → Go float64, SQLite REAL

**Proposal.** Keep it. float64 is Go's name for a 64-bit floating-point number, which is SQLite's only floating-point storage (8-byte IEEE REAL); a column of it holds fractional numbers such as prices or measurements. Adding float32 or a SQL spelling (real, double) would create the second spelling.

**Cost.** None.

**Evidence.** docs/spec.md §columns rule 1, §go-types, the §mapping-to-sqlite-ddl row; lang/check/types.go:85-92. The built-in types use exactly Go's spellings, with no float, real, double or float32 alias. Verified here: m float64 [not null, default: 1E2] gives `m REAL NOT NULL ...` in SQLite and the field M float64 in Go.

### 81. [keep] increment beside a plain integer pk

- id int [pk, increment] → INTEGER PRIMARY KEY, no setter
- id int [pk] → INTEGER PRIMARY KEY NOT NULL, with a setter

**Proposal.** Keep it: one word with one meaning, 'the application never chooses the key'. The spec should say that the DDL difference is cosmetic and the effect is on the Go surface.

**Cost.** None beyond one spec sentence.

**Evidence.** docs/spec.md §columns rule 4; nao/gen/sqlite/gen.go:188-195; nao/gen/golang/gen.go:174 (From skips the key). In SQLite, both tables assign keys automatically: any single-column INTEGER PRIMARY KEY is the rowid alias. The real difference is on the Go surface.

### 82. [keep] The `required` setting versus a hand-written validator that does the same check

- email string [not null, required]
- validators { EmailPresent(email) } with func EmailPresent(email string) error

**Proposal.** Keep required. A validator must be non-generic and take the column's exact type (§validators rule 2), so one Go function cannot cover every column; required is the one rule every type shares. Under the decided nullability default, 'MUST accompany not null' becomes 'not allowed on a null column'. State its semantics once, in §columns.

**Cost.** Only the wording of one rule.

**Evidence.** docs/spec.md §columns extension (required), §validators rule 8, §changesets rules 6 and 9; lang/check/settings.go:52, 295.

### 83. [keep] The verbs any, head and options

- any /ping Home.Ping (a pattern with no method)
- get /x, which ServeMux also serves for HEAD, beside a separate head /x
- options /x

**Proposal.** Keep all three on controller routes. any registers a pattern that no list of verbs reproduces, head lets a HEAD handler differ from the GET one, and options serves CORS preflights. None of them is a second spelling of another verb; only their use on query routes is redundant (verb item).

**Cost.** None.

**Evidence.** docs/spec.md §routes verb production and clause 1; lang/parser/volt.go:13-18; the blog fixture's any /ping.

### 84. [keep] resources and dataset as one-line macros over route lines

- resources db.users [default] / five hand-written query-route lines
- dataset db.browse / one get line per member

**Proposal.** Keep both. Each expands from schema facts that the hand-written lines would have to repeat (table name, key type, member list), so the routes and the queries cannot disagree. Their output must match the hand-written lines exactly (naming and verb items). Optionally make the keywords agree in number (resources is plural, dataset singular).

**Cost.** None beyond the naming and verb items.

**Evidence.** docs/spec.md §resources, §default-handlers 1, §datasets.

### 85. [keep] Value-less option wrappers and per-model type aliases over the generic runtime API

- PostLimit(10) / rt.Limit[Post](10); PostOffset and PostAfter likewise
- PostColumn / rt.ColumnOf[Post]; PostParams / rt.Params[Post] (routers write var volt_params db.UserParams)

**Proposal.** Keep. Go cannot infer M for value-less options, and the aliases shorten every changeset and handler signature. Document the generic rt forms as generator plumbing. The aliases cost two reserved select names and two dynname entries per model; revisit them only if the flat-naming item is redesigned.

**Cost.** None if kept.

**Evidence.** nao/gen/golang/dyn.go:38, 180-192; nao/rt/query.go:332-352 ('Value-less options cannot infer M, so the generator emits per-model wrappers'); docs/spec.md §generated-query-surface rule 4, §changesets rule 12, §selects-over-groups rule 7 (Column and Params are rejected as select names).

### 86. [keep] Server versus client update signature

- server: q.UserUpdate(ctx, cs), with the key taken from the row the changeset was made from
- client: c.APIUserUpdate(ctx, id, cs), with the key as a path parameter and cs built from db.UserNew()

**Proposal.** Keep both. Validators run over Row() and may read unmarked columns (Consistent(email, hits)), so the server needs the stored row, while the client only carries intent and the key.

**Cost.** None.

**Evidence.** docs/spec.md §changesets rule 8 (2903-2908), §generated-client rule 3 (2017-2023), §query-routes rule 2 (Update loads the row by the key, 1900-1903).

### 87. [keep] Dynamic-layer pairs that look alike but answer different questions: Offset vs After, Count vs Exists

- PostOffset(n) → LIMIT ? OFFSET ?; PostAfter(k...) → a keyset predicate over the OrderBy terms
- q.PostCount(ctx, preds...) > 0 / q.PostExists(ctx, preds...)

**Proposal.** Keep both pairs. Offset gives random-access page numbers, and After gives a position that stays stable under inserts; neither can express the other. EXISTS stops at the first row, while count(*) scans every match.

**Cost.** None.

**Evidence.** nao/rt/query.go:336-352; changesets_dyn.go.golden (PostCount, PostExists); docs/spec.md §predicates rule 6, keyset, count and exists bullets (2551-2562).


## Effects that could be pure

Verdicts: do, consider, keep.

### 1. [do] nao/rt/rt.go:68-85 rt.Open (SetMaxOpenConns(1), then the three PRAGMAs once through db.Exec); the same open in cmd/volt/migration.go (status path); spec §generated-query-surface rule 5

**Effect.** foreign_keys and busy_timeout belong to one connection, not to the database. Open sets them once, on whichever connection the pool holds at that moment. database/sql replaces connections without saying so, and the replacement has no pragmas. Three things trigger a replacement: (1) a Tx whose ctx is cancelled (mattn implements neither SessionResetter nor Validator, so awaitDone discards the connection); (2) migrationRun's driver.ErrBadConn after a failed ROLLBACK; (3) any ConnMaxLifetime or ConnMaxIdleTime.

**Bug.** Demo purity-datalayer/pragma/main.go: after rt.Open, foreign_keys=1. After one rt.Tx whose ctx is cancelled (what a client hanging up does to q.Tx(r.Context(), ...)), foreign_keys=0, and inserting a dangling foreign key returns err=nil. From then on no foreign key in the generated DDL is enforced for the life of the process, and rt.Refused never sees foreign_key. purity-bugs/pragma (ConnMaxLifetime) and purity-bugs/mig (a MigrateUp cancelled by a deadline at startup) reproduce it the same way. With ":memory:" the replacement connection is a new, empty database: "no such table: posts".

**Pure form.** Make connection setup a value applied on every connect, not state set once. Verified in purity-datalayer/pragmafix/main.go: after the same cancelled Tx, foreign_keys=1.
```go
var openPragmas = []string{"PRAGMA journal_mode = WAL", "PRAGMA busy_timeout = 5000", "PRAGMA foreign_keys = ON"}
type pragmaConnector struct{ base driver.Connector; pragmas []string }
func (c pragmaConnector) Connect(ctx context.Context) (driver.Conn, error) // base.Connect, run each pragma, Close on error
func Open(driverName, dsn string) (*sql.DB, error) {
	// base = DriverContext.OpenConnector(dsn), or a dsn connector over Driver.Open
	db := sql.OpenDB(pragmaConnector{base, openPragmas}); db.SetMaxOpenConns(1); return db, db.Ping()
}
```
The same connector is the single bind point the canonical-time item needs.

**Cost.** About 50 lines in nao/rt/rt.go and one test (cancel a Tx, assert foreign_keys=1). The migration-status open reuses the connector. Spec rule 5 becomes "on every connection". rt.Open should refuse plain ":memory:" or document that a reconnect loses it. No generated code changes and no API changes.

### 2. [do] Every bind of a time.Time: generated changeset args; static select methods (q.db.QueryContext(ctx, userSinceSQL, since)); rt.Column predicates (nao/rt/query.go cmp, Set, After keys at :352); rt.Null[time.Time].Value (nao/rt/null.go); rt.JSONList (nao/rt/list.go:13-33)

**Effect.** The text an instant becomes in SQLite depends on the value's Location, which is ambient (time.Local comes from TZ; a client chooses its own offset), and on who produced the text. mattn writes "2006-01-02 15:04:05.999999999-07:00" in whatever zone the value carries, CURRENT_TIMESTAMP writes UTC with no offset, and JSONList writes RFC 3339. SQLite compares these columns as text, so =, <, >, ORDER BY and After all compare bytes that depend on that ambient state.

**Bug.** Demo purity-datalayer/blog/main.go on the real generated itest package: UserSince(1h ago) returns 1 row with the cutoff in UTC and 0 rows for the same instant in +02:00, and a cutoff 3h in the future, written in -04:00, still matches a past row. purity-bugs/clock: under TZ=America/New_York, "created in the last hour" returns 0 (1 under UTC). A row stamped by the CURRENT_TIMESTAMP default is not found by Eq(u.CreatedAt). Keyset UserAfter(last.CreatedAt, last.ID) over 6 rows created in the same second returns ids [1 2] and never the rest, because "...:51" sorts before "...:51+00:00". purity-datalayer/inlist/main.go: InList([]time.Time{at}) matches 0 rows where Eq and In match 1. The test fixtures use whole UTC years (2000, 2999), so the suite cannot see any of this.

**Pure form.** Use one encoding that depends only on the instant: CURRENT_TIMESTAMP's own shape, in UTC, so database defaults and application values sort together.
```go
func TimeText(t time.Time) string { return t.UTC().Format("2006-01-02 15:04:05.999999999") }
```
Apply it at the single bind point: a driver.NamedValueChecker on the connection the rt.Open connector wraps, which also covers raw SQL. Without the connector, an rt.Arg at StmtQuery, StmtQueryRow, StmtExec, RowScan and JSONList's elements, with static methods routed through rt. Verified in purity-bugs/canon: Eq finds 6 of 6, keyset returns [1..6], the 11:00Z row no longer passes a 12:00Z cursor, and mattn scans values back as UTC.

**Cost.** About 20 lines on top of the connector (the wrapped connection must forward mattn's optional driver interfaces). The alternative is about 5 rt call sites plus generator routing and a full golden refresh. Spec §mapping-to-sqlite-ddl's time.Time row names the canonical text. Times always read back as UTC. Existing databases holding offset text need a one-off UPDATE, but there are no users.

### 3. [do] From emission, nao/gen/golang/gen.go:174-178; spec §changesets rule 2 ("From(u) marks every data column of u but the auto-increment key")

**Effect.** From marks columns the database is meant to fill (expression defaults such as CURRENT_TIMESTAMP), so Go's zero value replaces the database's clock. The zero time.Time cannot mean "let the database decide", because `required` on time.Time is refused (i113_required_time).

**Bug.** Demo purity-datalayer/blog/main.go: q.UserCreate(ctx, db.UserNew().From(db.User{Email: "eve@example.com"})) stores created_at = 0001-01-01 00:00:00 UTC. The default never runs and nothing refuses the write. Two consequences follow from the same rule: "duplicate this post" through From copies the original's timestamp, and a generated client that builds its body with From sends the zero time over the wire.

**Pure form.** Compute From's mark set from the schema at generation time: the data columns minus every column the database assigns (the auto-increment key and every column with an expression default). This extends the rule that already excludes the key. Setting a stamp stays explicit through its setter. Fuller form (separate decision): a stamp is an ordinary column set from one explicit clock read per request, `UserNew().CreatedAtSet(now)`, so Row, Validate, Params and the stored row agree and tests pass a fixed now. The DDL default stays for raw-SQL writers.

**Cost.** One condition in the From emission, goldens for tables with expression defaults (partials, junction_composite_pk, itest), and one clause in §changesets rule 2. An import that wants to keep its stamps calls the setter.

### 4. [do] Expression defaults in examples: docs/spec.md:541 and :1105 (`now()`), :842 (`now() - interval '5 days'`, which is PostgreSQL); lang/conformance/snippets/valid/07_column_settings, 19_tablepartial, 21_records_values, 35_kitchen_sink; grammar/examples/kitchen_sink.volt:33. Rule: §default-values rule 1 ("SQLite's to judge")

**Effect.** SQLite evaluates an expression default at the first INSERT that omits the column, not at CREATE TABLE. Neither the checker nor the fold evaluates it (the fold compares structure only). The spec and four "valid" snippets teach a function SQLite does not have.

**Bug.** Checked with sqlite3 just now: `CREATE TABLE t (id INTEGER PRIMARY KEY, a TEXT DEFAULT (now()))` succeeds, and the INSERT that follows fails with "unknown function: now()". A schema copied from the spec therefore passes check, gen and the fold, and then every create that leaves created_at unmarked fails at run time.

**Pure form.** Remove the wrong examples now and use CURRENT_TIMESTAMP; the maintainer asked for incorrect syntax to be removed. Then move the evaluation to gen: evaluate each expression default once as `SELECT (<expr>)` in the in-memory SQLite the fold already opens, and report SQLite's message at the default. This uses the engine as the oracle, like the fold lemma does. Lower-entropy alternative: a closed set (CURRENT_TIMESTAMP and literals), with anything else written as a migration exec.

**Cost.** Examples: spec (3 places), 4 snippets, grammar/examples/kitchen_sink.volt (plus sync-grammar). Probe: about 30 lines in cmd/volt next to the fold, inert without cgo just as the fold is.

### 5. [do] events.go:30-52, the exported fields Broker.Replay and Broker.Heartbeat; replay() read at :67 and :87; Heartbeat read at :130 without the lock; spec §event-routes rule 2

**Effect.** Configuration lives in exported fields of a live, mutex-guarded object and is re-read on every Publish, Subscribe and Serve; Serve reads it outside the mutex. Subscribe sizes its channel from the current Replay, but replays a buffer sized by earlier values, pushing into the channel synchronously while it holds b.mu.

**Bug.** Demo purity-http/rt TestBrokerReplayLowered and purity-bugs/broker: publish 500 events, then set Replay=10, as happens when config is applied late or a test tunes a shared broker. Subscribe(0) blocks forever holding the mutex, and every later Publish blocks behind it, which means every write handler that announces a change. Setting Heartbeat while serving is a data race.

**Pure form.** Fix the configuration at construction, and size the channel from what is actually replayed:
```go
func NewBroker(replay int, heartbeat time.Duration) *Broker // unexported, immutable; 0 means default
func (b *Broker) Subscribe(after uint64) (<-chan Event, func()) {
	b.mu.Lock(); defer b.mu.Unlock()
	pending := since(b.buffer, after) // pure slice computation
	ch := make(chan Event, len(pending)+64)
	...
}
```

**Cost.** events.go, events_test.go (3 struct literals), itest/router_test.go:126, the doc on the generated Events field, spec §event-routes rule 2. About 5 sites.

### 6. [do] Generated NewRouter and register (gen/router/gen.go:282, :332) reading the Controllers manifest (generated volt_handlers.go); spec §handler-references rule 4, §query-routes rule 5

**Effect.** The manifest is read lazily. Each route closure dereferences c.<Controller> or c.DB only when a request arrives, so whether the manifest is complete is found out one route at a time, by live traffic.

**Bug.** Demo purity-http/manifest against itest/blog/app: NewRouter(Controllers{Home: home{}}) succeeds and GET / returns 200. GET /admin/stats and GET /api/users panic on a nil interface or pointer; net/http recovers and drops the connection, and the client sees EOF. The error spine, the Problem body and volt.Logger's line are all skipped, so in production the first admin visit is the first sign.

**Pure form.** Check the manifest once, at construction, as a pure function of the value:
```go
func (c Controllers) Check() error // generated: errors.Join of "Controllers.Admin is nil (GET /admin/stats)" for each nil field a route uses
func NewRouter(c Controllers) (http.Handler, error) { if err := c.Check(); err != nil { return nil, err }; ... }
```
Events stays optional, since a nil broker serves 503 by §event-routes.

**Cost.** Router generator: one generated method plus the NewRouter signature, which changes the handlers golden. Also the itest call sites, the README examples and one clause in §handler-references. About 5 sites.

### 7. [do] Generated changeset setters (nao/gen/golang/gen.go:187), Changeset() (:172), Row() (:219), and Validate's validator calls over c.Row() (nao/gen/golang/validate.go:76-90); rt.ParamsOf; rt.FieldErrorsAppend (nao/rt/changeset.go:339) and (*Invalid).Details (:78), which share Columns; rt.After (nao/rt/query.go:352), OrderBy (:323) and Raw (:195), which keep the caller's variadic slice

**Effect.** §changesets rule 1 promises value semantics, and §predicates promises inert, immutable data. Neither holds for slices. A []byte column (and rt.Null[[]byte]) shares its bytes with the caller's buffer, the source row, every copy of the changeset, every Row() and Params, and every validator argument. Error Columns and option slices are shared the same way.

**Bug.** Demo purity-datalayer/alias (the generated golden, copied unchanged): a changeset validated with cover "cover-v1" holds "XXXXXXXX" once the caller reuses its buffer (bufio, sync.Pool, the next request). Writing into a Row() result changes the changeset, and a Params built earlier encodes the mutated bytes. purity-bugs/blob: changesets built from bufio.Scanner.Bytes() store "gamma" for a body validated as "alpha". Prefixing c.Errors()[i].Columns for a nested form rewrites another copy's errors, which -race reports as a DATA RACE (reconfirmed). purity-lang/demo/cmd/aliasing: a validator that zeroes its argument's CRC bytes in place passes; afterwards an earlier copy and the caller's buffer are changed, and Create's second Validate fails. purity-bugs/clock alias_test: the same After option renders [10], then [99].

**Pure form.** Copy at the boundary, so a changeset owns its bytes:
```go
func (c PostChangeset) CoverSet(v []byte) PostChangeset { c.vals.Cover, c.set[4] = bytes.Clone(v), true; return c }
func (r Post) Changeset() PostChangeset { r.Cover = bytes.Clone(r.Cover); return PostChangeset{data: r, row: true} }
func (c PostChangeset) Row() Post { r := c.row(); r.Cover = bytes.Clone(r.Cover); return r } // row() internal
```
Validators get bytes.Clone(v.X); rt.NullClone covers rt.Null[[]byte]. FieldErrorsAppend and Details apply slices.Clone to Columns; After, OrderBy and Raw apply slices.Clone to their variadics. bytes.Clone(nil) is nil, so "nil is NULL" still holds.

**Cost.** 3-4 emission sites, emitted only for []byte fields (bytes is stdlib, so the import rule allows it); 5 rt constructors and helpers; goldens of tables with []byte columns; one clause in §changesets rule 1. One copy per set and per public Row() call. A declared type stored as []byte is not covered unless the generator learns its underlying type, so the spec should say declared types must behave as values.

### 8. [do] rt.Blank (nao/rt/changeset.go:349) treats a Value error as "not blank"; requiredEmit (nao/gen/golang/validate.go:94); spec §types rule 3, §changesets rules 6 and 9

**Effect.** A declared type's Value method is user code that decides whether a value can be stored. Validate consults it only for `required`, and swallows its error there. The real verdict comes at bind time, inside database/sql, as a raw driver error.

**Bug.** Demo purity-lang/demo/cmd/valuer, with Type Code whose Value accepts only A-Z and 0-9 and the column code [not null, required]. CouponNew().CodeSet("half off").Validate() is valid. CouponCreate then returns `sql: converting argument $1 type: code "half off": only A-Z and 0-9`, which is not an *rt.Invalid. DefaultErrorHandler therefore answers 500 "internal server error", and the form cannot mark the field. The same happens on any declared-type column without `required`.

**Pure form.** Compute the bound value once, while validating. Validate calls Value for every declared-type column and records an error as a field error; `required` reads the value that call returned.
```go
func Blank(v any) (blank bool, err error)
// generated
if dv, err := v.Code.Value(); err != nil { errs = append(errs, rt.FieldError{Check: "value", Columns: []string{"code"}, Message: err.Error()}) } else if rt.BlankValue(dv) { /* required */ }
```
Valid() then implies the bind accepts every value.

**Cost.** About 15 lines in validate.go; rt.Blank and TestBlank; validate_test; itest nao_validate.go regenerated. Spec §types rule 3 and §changesets rules 6, 7 and 9 gain the check `value`. One extra Value call per declared column per Validate.

### 9. [do] nao/rt/stmtcache.go:16-84: StmtCache keyed by SQL text only; Prepare takes `db DBTX` on every call and ignores it on a hit (:49, :68). The cached statement is run as-is by nao/rt/query.go:554/567/580 and nao/rt/changeset.go:433. Generated WithCache and WithTx: nao/gen/golang/queries.go:523-542

**Effect.** The cache memoizes PrepareContext(handle, sql) but keys only on sql, so the handle is a hidden input: whichever handle first prepared that text. The rule "one cache per *sql.DB, never a transaction" exists only in a doc comment, and the types accept anything.

**Bug.** Demo purity-datalayer/cache and purity-bugs/cache. Two databases sharing one cache: a read through B returns A's row, and a UserCreate through B lands in A (a=2, b=0), silently. A cache fed a transaction once answers "sql: statement is closed" for that SQL text on the *sql.DB until LRU eviction. A cache warmed on the DB, then used through New(tx).WithCache(c) inside rt.Tx, deadlocks on rt.Open's single connection (context deadline exceeded; forever without a deadline). With two connections the insert escapes the transaction and survives the rollback (1 row afterwards).

**Pure form.** Bind the cache to its handle at construction, and derive the statement from (cache, handle, query) instead of trusting the caller:
```go
func NewStmtCache(db *sql.DB, max int) *StmtCache
func (c *StmtCache) stmt(ctx context.Context, h DBTX, q string) (s *sql.Stmt, ok bool, err error)
// cached when h == c.db; h.StmtContext(ctx, cached) when h is a *sql.Tx (database/sql refuses a tx of another DB); ok=false otherwise, which runs directly
```
Generated side: `func (q *Queries) WithCache(max int) *Queries` creates the cache for q's own handle, so users never hold a cache they could share. WithTx keeps the cache, so writes inside q.Tx get cached statements too.

**Cost.** stmtcache.go and the four Stmt* helpers; one template in queries.go plus about 15 *_queries goldens, itest/blog/db and nao/itest; about 4 test call sites. Spec §generated-query-surface rule 4 ("WithTx drops it") and §changesets rule 8 change wording. The signature change breaks no one, since there are no users.

### 10. [do] Spec §validators rule 5 ("Pure") is stated but never checked: lang/checks.go:116 validatorSpec checks only the spelled signature. Create and Update re-run Validate (nao/gen/golang/queries.go:671/684). The analyzer that could check it is test-only, in cmd/volt/purity_test.go

**Effect.** Nothing reads a validator's body. A validator can call time.Now, read a package variable that main sets, hold a *sql.DB, or memoize in a package map. Meanwhile Validate runs twice per write, inside transactions, and concurrently on copies (which rule 1 allows).

**Bug.** Demo purity-datalayer/blogv and purity-bugs/uniq. A Rails-style "email taken?" validator querying a package-level *sql.DB passes volt vet and works outside a transaction. Inside q.Tx it waits for the one connection the transaction holds. With a 1s deadline the Tx fails with "context deadline exceeded", and the discarded connection comes back with foreign_keys=0 (the rt.Open item). With context.Background it is still blocked after 3s, and the validator has no ctx through which it could be cancelled. It also runs twice per create. Two more follow from the rules: a clock-reading validator ("publish_at in the future") can pass in the controller and fail in Create's second Validate; a validator reading a package variable that main sets from the environment gives a different verdict in a client binary that shares the models.

**Pure form.** Check rule 5 where it can be checked. Move the analyzer from purity_test.go into a non-test file of cmd/volt and give it a second target set: each validator, plus the Value method of every declared type, which Validate reaches. `volt vet` then reports a warning citing spec/validators. Rules for that set: no package variable, except an unexported one never assigned after its declaration (this admits `var emailRe = regexp.MustCompile(...)`); no writes through an argument; no goroutines or channels; stdlib only through an allowlist that excludes time.Now/Since/Until/Sleep/Local, os, net/http, database/sql, context and math/rand. A call into a third-party package is reported as unverified. The message names the fix: pass the value as a column, or judge the rule in the controller with ErrorAdd, where the clock and the config are explicit. A later, separate step to consider: a typestate (`Validated() (UserValid, error)`, `UserCreate(ctx, UserValid)`), so the write runs no user code and validators run exactly once.

**Cost.** cmd/volt only; x/tools is already a direct dependency. Relocate about 1200 lines (the self-gate keeps using them) and add about 150 for target discovery and the user allowlist. lint.md gains an entry whose examples need Go sidecar files, so the docs test harness must support them. §validators rule 5 says vet checks it. It should be a warning, not an error: the analysis is conservative. The reviewer's probe (purity-lang/gate) flagged the package-variable read, the time.Now call and the in-place write in 0.6s.

### 11. [do] formats.go:99 `var MaxBodyBytes int64 = 4 << 20`, read at :114 and again at :118; formats_test.go:290 mutates it; every generated query route calls volt.Decode (gen/router/gen.go:444); spec §formats rule 2

**Effect.** A package global that the application writes, read twice on every Decode (once for the limit, once for the message), and shared by every router and every test in the process.

**Bug.** Rerun under -race: purity-http/rt TestMaxBodyBytesRace reports a DATA RACE at formats.go:114. purity-bugs/body: a body refused under a limit of 100 got a 413 saying "request body over 1048576 bytes", because the second read saw another value. One binary cannot give an upload route 50 MB and a JSON API 64 KB: raising the limit for uploads raises it for every changeset body. The save/restore pattern in formats_test.go cannot be made t.Parallel.

**Pure form.** Remove the global, and make the limit an explicit input fixed at construction, with no new syntax. A NewRouter or Controllers field (zero means 4 MiB) that volt.Handler stores on the Request; Decode reads it from the Request. `func DecodeLimit(r *Request, v any, max int64) error` serves a hand-written controller that needs its own limit. The 413 message is built from tooBig.Limit. A scope `max_body` setting would also work, but it adds language surface, against the goal of lowering entropy.

**Cost.** About 10 lines in formats.go; volt.Handler gains one parameter or Request field; one template line in gen/router/gen.go:444; router goldens; formats_test; §formats rule 2. About 6 sites.

### 12. [do] events.go:174 Client.Stream and :201 streamOnce; client.go:17 and :243 (a nil HTTP means http.DefaultClient); spec §event-routes rule 3, §generated-client rule 2

**Effect.** The reconnect schedule is hidden loop state driven by ambient time. "Success" means "received at least one event", not "connected". One http.Client, or the global DefaultClient, serves both Do, which wants a Timeout, and Stream, which must not have one.

**Bug.** Demo purity-http/client TestStreamInheritsTimeout: with the usual Client.HTTP = &http.Client{Timeout: 300ms}, every stream is cut at the timeout. Quiet reconnects then count as failures and the backoff grows, so an event published at 5.0s is delivered at 7.9s; after longer quiet periods the wait reaches 32s. If more than Replay events are published in that window they are lost (the replay-gap item). The cap is 32s, not the documented 30s, because 16 < 30 doubles to 32; a table test of a pure schedule would have caught it.

**Pure form.** ```go
func StreamBackoff(failures int) time.Duration { return min(time.Second<<failures, 30*time.Second) } // table-tested
func sseParse(r io.Reader, yield func(Event) bool) (last uint64, err error)                          // pure over the body
```
A connection succeeds when it gets a 200. Stream sends its request on a copy of the client with Timeout 0. The loop becomes `ok := connect(); if ok { failures = 0 } else { failures++ }; sleep(StreamBackoff(failures))`.

**Cost.** events.go (Stream, streamOnce), client.go, events_test, wording of §event-routes rule 3. About 4 sites. Making the http.Client an explicit New parameter is optional and would touch the client goldens.

### 13. [do] volt.go:97-129 statusWriter (WriteHeader, Flush) and Request.Committed (:26); Broker.Serve's w.(http.Flusher) at events.go:113

**Effect.** The runtime works out whether a response is committed by watching writes through a wrapper, and the wrapper guesses. Any WriteHeader call, a 1xx included, counts as a commit. Flush is advertised, but forwarded only when the writer directly inside is an http.Flusher.

**Bug.** Demo purity-http/rt. TestEarlyHintsThenError: 103 Early Hints followed by ErrNotFound gives status 200 with an empty body, and the 404 is only logged. TestEventsBehindUnwrapMiddleware: a pipeline plug that wraps the writer in the Go 1.20 style (embeds it, has Unwrap, no Flush) makes statusWriter.Flush a silent no-op while Serve's Flusher check still passes. No response headers arrived within 1.5s while 20 events were published. In a controller, http.NewResponseController(w).Flush() returns nil instead of ErrNotSupported.

**Pure form.** WriteHeader commits only for code >= 200. `func (w *statusWriter) FlushError() error { err := http.NewResponseController(w.ResponseWriter).Flush(); if err == nil && !w.wrote { w.wrote, w.status = true, 200 }; return err }`, plus Unwrap. Broker.Serve calls http.NewResponseController(w).Flush() and returns the error when flushing is unsupported, instead of streaming into a buffer.

**Cost.** volt.go (two methods), Serve's flush in events.go, new cases in volt_test and events_test. About 3 sites; the spec needs a wording fix for Committed.

### 14. [do] nao/rt/stmtcache.go:76-82 (eviction closes the oldest statement) while Prepare hands out the shared *sql.Stmt after unlocking (:50-54)

**Effect.** The cache hands out a shared resource whose lifetime only the cache controls. Between Prepare returning a statement and the caller running it, another goroutine's insert can evict that statement and Close it.

**Bug.** Demo purity-bugs/cache TestEvictionClosesStatementInUse: NewStmtCache(2), 8 goroutines cycling through five query shapes. In my run 4 of 4000 UserQuery calls failed with "sql: statement is closed"; the reviewer saw 40-46 on a 4-connection pool and 1-2 on rt.Open's single connection. Under load these are rare, flaky 500s; a single-goroutine test never shows them.

**Pure form.** Make the borrow explicit, so a statement closes only when its last borrower releases it:
```go
func (c *StmtCache) acquire(ctx context.Context, q string) (*sql.Stmt, func(), error)
```
Eviction removes the entry and marks it doomed; release closes a doomed statement when its refcount reaches 0. The Stmt* helpers `defer release()`; database/sql already keeps a statement alive while its Rows are open.

**Cost.** About 30 lines in stmtcache.go, four helper call sites and a concurrency test. No generated-code change. Best done in the same change as binding the cache to its handle.

### 15. [do] volt.go:36 `type ErrorHandler func(http.ResponseWriter, *Request, error)`, :40 Handler, :57 DefaultErrorHandler; the error_handler signature check at lang/semantics.go:453; spec §scopes clause 3, §formats rule 5

**Effect.** The error handler both decides (status, message, details) and writes. It must therefore honour Committed() itself, negotiate the format, and produce a volt.Problem the generated client can decode, yet writeProblem is unexported. The "must not write once committed" rule exists only in a doc comment, and the handler logs through the ambient slog.Default.

**Bug.** Demo purity-http/rt TestErrorHandlerAfterCommit: a handler that forgets Committed() appends a second document to a response that already went out as 200. The body is `{"total":3}\n{"status":500,...}`, and a JSON client decodes the first value and reports success. An obvious handler, `http.Error(w, err.Error(), 500)`, turns a 422 *rt.Invalid into a text/plain 500 the client cannot map to fields, and leaks driver text the default hides. A handler that writes nothing (as the fixture gen/router/testdata/blog/app/mw.go's Errors does) turns every error into an empty 200, by net/http semantics.

**Pure form.** Separate the decision from the write, and let the runtime own the commit state and the logging:
```go
func ProblemOf(err error) Problem                     // pure: HTTPError / Detailer / sql.ErrNoRows / 500
type ErrorHandler func(r *Request, err error) Problem  // decides; gets no writer
// in Handler, the only writer:
p := eh(req, err); if sw.wrote || p.Status >= 500 { log }; if !sw.wrote { writeProblem(sw, req, p) }
```
A zero Status counts as 500. ProblemOf can be table-tested without httptest.

**Cost.** volt.go, errors.go, volt_test; the signature check in lang/semantics.go and its snippets (v13_error_handler_qualified and the matching invalid ones); gen/router/testdata/blog/app/mw.go; itest/blog/app/user.go, whose header hook becomes a Message; §scopes clause 3 and §formats rule 5. About 8 small sites. It removes rendering an error as a non-Problem page (HTML); such a page belongs in a controller, or in a Reply if that lands.

### 16. [do] Records (§records-sample-data, docs/spec.md:1114): nao/gen/sqlite/gen.go:276 insertsEmit; lang/check/types.go:258 recordsTypeCheck; the fold's target DDL (cmd/volt/migration.go:391)

**Effect.** Records are a second write path next to changesets: verbatim INSERTs in nao_schema.sql that skip `required`, every validator (membership in a value set among them) and every declared type's Valuer. That contradicts §changesets ("there is no other write path for a row"). An expression literal in a record (snippet 21_records_values uses `now()`) also ties the seed to the clock.

**Bug.** Demo purity-lang/demo/cmd/defaults: with title [not null, required] and validator TitleShape, the records (1, '') and (2, 'lowercase title') load without a word. Afterwards even a no-op PostUpdate(ctx, p.Changeset()) on either row fails with 422, because Update validates before it checks for marks. Every later edit of any column on a seeded row is refused over a value nobody is touching.

**Pure form.** Remove Records, both forms. Seed data becomes Go that goes through the one write path: `func Seed(ctx context.Context, q *db.Queries) error { _, err := q.PostCreate(ctx, db.PostNew().TitleSet("Hello")); return err }`. Every seeded row then passes Validate and the Valuer, and nao_schema.sql and the fold's target become a function of the tables alone. It also takes two syntaxes out of the language, which is the entropy reduction the maintainer asked for.

**Cost.** Mostly deletions across about 20 files: parser; ast; check; sqlite insertsEmit; vet; lsp (completion, index, features); grammar.js plus tree-sitter generate, the editor.md checklist and sync-grammar; zed queries; grammar/examples/kitchen_sink.volt; snippet 21_records_values and the kitchen-sink snippets; the spec section, its EBNF and the DDL table row; roadmap.

### 17. [do] middleware.go:17-30 RequestID

**Effect.** It writes the generated ID into the caller's *http.Request with r.Header.Set, which net/http says handlers must not do, and it draws the ID from crypto/rand.

**Bug.** Demo purity-http/client TestRequestIDMutatesRequest: serving one *http.Request twice, as a test reusing httptest.NewRequest or a retrying wrapper does, gives the same ID ("1ca6e8420e415f4f") both times, and the caller's request now carries that header.

**Pure form.** Leave the input untouched and pass the ID down on a copy: `r2 := r.Clone(r.Context()); r2.Header.Set("X-Request-ID", id); next.ServeHTTP(w, r2)`. A context value read through volt.RequestIDOf(r) also works. crypto/rand stays until plugs can take inputs.

**Cost.** 3 lines in middleware.go and one test.

### 18. [do] nao/rt/null.go:50 `var jsonNull`, returned by MarshalJSON (:55) and compared against in UnmarshalJSON (:62) and rt.Field (nao/rt/changeset.go:312)

**Effect.** MarshalJSON returns the package's own slice, so any caller holding the result can change how NULL marshals and decodes for the whole process.

**Bug.** Demo purity-bugs/null: one b[0] = 'N' on the bytes MarshalJSON returned is enough. Afterwards every rt.Null fails to marshal ("invalid character 'N'"), a JSON null decodes as Valid:true, and Cast stops refusing null on non-nullable columns. The trigger is rare (a direct caller mutating the result); the fix is trivial.

**Pure form.** `func (n Null[T]) MarshalJSON() ([]byte, error) { if !n.Valid { return []byte("null"), nil }; return json.Marshal(n.V) }`, with both comparisons made against a constant string.

**Cost.** Three lines, and one 4-byte allocation per NULL marshalled.

### 19. [do] nao/rt/migrate.go:131-169 MigrateDown (ledgerRead and the choice of `last` happen before migrationRun's BEGIN IMMEDIATE); migrationRun :280/:307 re-checks only that the chosen row still exists (`has != rowBefore`)

**Effect.** The plan (which migration is "the greatest applied") is computed from a ledger read made outside the write lock. Under the lock, only part of it is checked again: that this one row is still there.

**Bug.** Demo purity-datalayer/downrace, which uses a driver wrapper to run runner B just before runner A's BEGIN IMMEDIATE. The ledger holds {0001, 0002}. Runner A (a rollback) picks 0002. Runner B (a deploy) applies 0003, an index on 0002's table. A then reverts 0002 and drops the table, which takes the index with it. Status now shows 0001 applied, 0002 pending, 0003 applied. Every later down fails with "no such index: idx_comments_body", and the next up recreates 0002 without 0003's index. The database has drifted silently and the ledger is stuck.

**Pure form.** Make the pick a pure function and recompute it from a ledger read made under the lock:
```go
func migrationDownPick(done map[string]string, ms []Migration) (Migration, error)
```
MigrateDown reads the ledger and picks m (needed early for ForeignKeysOff), runs BEGIN IMMEDIATE, then reads again and picks m2. If m2.Name != m.Name it refuses ("another runner changed the ledger; run down again"). migrationRun takes a `still func(ctx, *sql.Conn) (bool, error)` in place of the fixed ledgerHas check. A NoTransaction down takes no lock, so for it the second read only narrows the window; §migration-ledger rule 3 should say so.

**Cost.** About 30 lines in nao/rt/migrate.go; the demo, made deterministic, is the regression test; one sentence in §migration-ledger rule 3. No generated-code or CLI change.

### 20. [do] nao/gen/golang/gen.go defaultGo skips a declared type that is not kinded; spec §changesets rule 2 ("the default of a declared type that implements Scanner and Valuer ... the database's alone"), §default-values rule 1

**Effect.** For built-in columns and kinded declared types (§types), <Model>New seeds the literal default, so Validate sees what the database will write. For a declared type that implements Scanner and Valuer it does not: Validate judges the zero value while the database stores the default. Whether the default even scans into the Go type is found out only when a row is read back.

**Bug.** Demo purity-lang/demo/cmd/defaults and purity-bugs/money: with `amount Money [not null, default: 100]` and validator AmountPositive, PriceNew().Row().Amount is 0, and PriceCreate(ctx, PriceNew()) is refused with 422 "amount must be positive", although the database would have stored 100. The reverse case follows from the same rule: a Type Slug whose Scan refuses spaces, with default 'Draft Post', stores rows that every later Get fails to scan (500).

**Pure form.** Seed the default through the type's own Scan, from the storage type's canonical driver value:
```go
func PriceNew() PriceChangeset { var c PriceChangeset; if err := c.data.Amount.Scan(int64(100)); err != nil { c.errs = []rt.FieldError{{Check: "default", Columns: []string{"amount"}, Message: err.Error()}} }; return c }
```
Validate then judges what will be stored, and an unscannable default is an error on the first create. Lower-entropy alternative: refuse a literal default on a declared-type column (null or an expression only); itest's `tip Money [not null, default: 0]` would then set tip explicitly.

**Cost.** defaultGo and the New emit (about 20 lines), the itest models golden, §changesets rule 2 and §default-values rule 1. Scan becomes reachable from New, so it joins the validator gate's targets (allowed to write only its receiver). The refusal alternative is one check, one snippet and the itest edit.

### 21. [do] events.go:84 Subscribe(after uint64), with IDs from the process-local counter b.next (:64); Publish drops a slow subscriber (:68-74); the client cursor keeps only the maximum ID it has seen (streamOnce, `if ev.ID > *last`); spec §event-routes rules 2-3 ("resumes where it was")

**Effect.** What a reconnecting client missed is answered implicitly, inside the effectful Subscribe. It returns whatever part of the bounded buffer is newer than the cursor and never says whether that is everything. The cursor has no epoch, so a restart numbers from 1 again.

**Bug.** Demo purity-http/rt and purity-bugs/broker. With Replay 4 and ten events, Last-Event-ID 2 receives [7 8 9 10]. A subscriber dropped after 68 resumes with [97..100]; 69..96 are lost with no event, error or flag. After a deploy, Last-Event-ID 10 replays 0 of the 5 events the new process published, and the client cursor ignores the new IDs until they pass 10, so the next reconnect skips events again. The spec promises resumption; screens stay stale and nothing says so.

**Pure form.** Make resumption a pure function that reports whether the replay is complete, and make the epoch an explicit input:
```go
type Cursor struct{ Epoch string; Seq uint64 } // wire id "epoch-seq"
func since(buf []Event, epoch string, c Cursor) (evs []Event, complete bool) // false when epochs differ or c.Seq+1 < buf[0].Seq
func NewBroker(epoch string, replay int, heartbeat time.Duration) *Broker
```
When the replay is not complete, Serve first writes an `event: volt.reset` that the spec defines, and the generated client passes it on so the application refetches. since can be table-tested with no goroutines.

**Cost.** events.go (ID format, since, Serve, client parsing and cursor), events_test, the itest client test, and §event-routes rules 2-3, which must name the reset event. About 5 sites; best combined with the NewBroker change from the broker-config item.

### 22. [consider] Generated <Model>Update (nao/gen/golang/queries.go:682), which issues UPDATE ... SET <marked> WHERE <key> through rt.UpdateRowRender (nao/rt/changeset.go:405); the generated PATCH/PUT shim (gen/router/gen.go:447-454): Get, Cast, Update, no transaction; spec §changesets rule 8, §validators rule 5

**Effect.** Validate judges Row(), which is the snapshot plus the marks, but the statement carries only the marks. The snapshot values a validator read for unmarked columns are a precondition the write never states, and no CHECK backs them up (§validators rule 9).

**Bug.** Demo purity-http/skew (a generated package with Ordered(starts, ends)), purity-lang/demo/cmd/race and purity-bugs/span. The row is (9,12). PATCH {"ends":10} validates as (9,10); PATCH {"starts":11} validates as (11,12). Both shims answer 200, and the stored row is (11,10), which Ordered refuses. It happens only under concurrency, so tests never see it; it surfaces later as a row that should be impossible.

**Pure form.** Have the plan carry its precondition. For every validator that reads a marked column, each unmarked column it reads joins the WHERE as `col IS ?` with its snapshot value; the generator knows ValidatorSpec.Cols statically. `func UpdateRowRender(table string, set, key, guard []string, returning string) string`. Zero rows on a key that still exists is rt.ErrStale (409, "row changed since it was read"); otherwise rt.ErrNotFound. Tables with only single-column validators generate exactly what they do today. This depends on the canonical-time item: a guard on a CURRENT_TIMESTAMP-stamped column would never match while times bind with offsets.

**Cost.** UpdateRowRender plus an ErrStale sentinel and its status; the Update emission; queries goldens of tables with multi-column validators; one existence probe on zero rows; §changesets rules 8 and 10 and §validators rule 5 name the guard next to unique and FK; one itest proof. Clients must handle a new 409.

### 23. [consider] §validators rule 3 (a validator may list any column); lang/checks.go:116 validatorSpec; no rule in docs/lint.md covers this

**Effect.** On a create, a column whose value the database decides at the write (an expression default such as CURRENT_TIMESTAMP, and a declared type's default until that item lands) holds Go's zero value in Row(). A validator that lists it judges a value that will never be stored.

**Bug.** Demo purity-bugs/post: with `posted_at time.Time [not null, default: CURRENT_TIMESTAMP]` and validator PostedAtSane(posted_at) (year >= 2000), PostCreate(PostNew().TitleSet("hello")) is refused with "posted_at is before 2000", although the row the database would write passes. On update the same validator passes, so tests that only update never notice. volt vet exits 0.

**Pure form.** A vet warning citing spec/validators: "validator PostedAtSane reads posted_at, whose value on create is the database's (expression default): Validate sees the zero time.Time". The message points to the explicit form: the controller passes the clock with PostedAtSet(now), so Validate judges what is written.

**Cost.** One lang/vet rule over ValidatorSpec.Cols and the column settings, plus a lint.md entry with executable examples. No generated-code change.

### 24. [consider] Plugs named as top-level Go functions: §pipelines rule 2, §scopes clauses 2-3; resolution in lang/semantics.go:390-402 through lang/gofuncs.go; the generated pipeN closures applied once per route (gen/router/gen.go:341-353; pipe0 is applied 33 times in gen/router/testdata/blog_app_router.go.golden)

**Effect.** A plug takes no inputs, so whatever it depends on (a signing key, a session store, a limiter, a logger) can only reach it through package variables that main sets at some point. And pipe0(h) runs once per route inside NewRouter, so state built in a plug's body exists once per route, while §scopes rule 2 says the chain is "composed once at generation time".

**Bug.** Demo purity-lang/demo/cmd/plugs, which copies the generated register: a Quota plug meant to allow 3 requests across the API is instantiated 3 times for 3 routes and serves 9 of 10 requests. Every new route silently raises the limit; the blog fixture would get 33 limiters. From the same rule: a BearerAuth that reads `var AuthKey []byte` gets whatever the variable holds when NewRouter runs, so a router built in TestMain or in a package var before config loads verifies against an empty key, and parallel tests that set it race.

**Pure form.** Make plugs explicit inputs of NewRouter, as controllers already are: `type Plugs struct { BearerAuth func(http.Handler) http.Handler }`, or fields on Controllers. main builds the state once: `Plugs{BearerAuth: auth.New(key, store).Middleware}`. register reads p.BearerAuth, and a nil field is refused at construction (the manifest-check item). volt.<Name> plugs stay direct references. The spec states that a plug is applied once per route. The Go compiler checks the signatures at the NewRouter call, which removes the go/build scan for plugs and one class of checker diagnostic.

**Cost.** Router generator (register, the manifest), the blog golden and fixture, §pipelines rule 2 and §scopes clauses 2-3, plug resolution in lang/semantics.go, snippets v03_pipeline, i86_plug_undeclared and i87_plug_not_middleware, lsp completion and definition for plugs, and one line per plug in each main. About 12 sites. Once the error handler returns a Problem, it can follow the same path.

### 25. [consider] exec in migrations: §migrations rule 11, §the-fold rule 4; no vet rule covers it

**Effect.** An exec guarded by IF NOT EXISTS or IF EXISTS makes a migration's effect depend on what the target database already holds, which the fold's empty database cannot show.

**Bug.** Follows from SQLite semantics (reviewer setup in purity-lang/demo/mig). Migration 0001 is `exec 'CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, email TEXT NOT NULL)'`. `volt migration check` reports that the fold matches. `volt migration up` on a database that already has users(id) records 0001 as applied, yet users still lacks email, and every UserCreate fails. A structural `create table users` would have failed loudly.

**Pure form.** A vet warning on exec text containing IF NOT EXISTS or IF EXISTS (case-insensitive), citing spec/the-fold: "the fold proves nothing about an exec whose effect depends on the database's prior state". exec itself stays.

**Cost.** One lang/vet rule and one lint.md entry. No runtime change.

### 26. [consider] lang/gofuncs.go:109 (`ctxt := build.Default` plus MatchFile), memoized per directory in lang/session.go:198-222 (s.scans, keyed on dir and sources) and used by lsp/voltnav.go:256; spec §pipelines rule 2, §validators rule 1

**Effect.** Which Go files a validator, plug or error handler resolves in depends on the process's GOOS, GOARCH and build tags, which go/build.Default reads from the environment, while the generated call sites carry no build constraint. The s.scans memo is keyed on the sources, but its result also depends on that ambient context, and GoFuncsOf is not among cmd/volt/purity_test.go's pureTargets, contrary to AGENT.md's rule that every memo is gated.

**Bug.** Reviewer demo purity-lang/demo/host: validator BodyShape is defined in shape_linux.go. volt gen on linux succeeds; GOOS=windows go build fails with "undefined: BodyShape" in nao_validate.go, and GOOS=windows volt gen reports "no function BodyShape". CI on one OS and a laptop on another disagree about the same commit. The failure is a loud compile error, so the risk is moderate.

**Pure form.** Resolve against the sources alone: a name resolves only in files with no build constraint (no //go:build line, no _GOOS/_GOARCH suffix, no leading _ or .), because generated code calls it on every platform. A match found only in a constrained file is an error naming that file. GoFuncsOf(dir, srcs) then needs no build context, and its memo can join the gate's targets.

**Cost.** Delete the go/build matching (about 10 lines), add the constrained-file message and a snippet, update §pipelines rule 2 and §validators rule 1. It rules out per-OS variants, which can wrap an unconstrained function instead. Smaller still if plugs leave Go-name resolution.

### 27. [consider] Broker.Publish called from inside actions (events.go:57); spec §event-routes rules 2 and 4

**Effect.** Announcing a change is an immediate effect, performed in the middle of the action and mixed in with the database effects it announces. Nothing ties it to whether those effects committed or whether the action succeeded.

**Bug.** Not demonstrated, because it depends on timing. A controller publishes "posts.changed", then its commit fails (SQLITE_BUSY) or a later step returns an error, and every open screen refetches for a change that never happened. Or it publishes before tx.Commit: clients refetch, read the previous snapshot under WAL, and keep showing stale data until some unrelated event arrives.

**Pure form.** An inert per-request outbox: `func (r *Request) Emit(name string, data any)` only appends, and volt.Handler publishes the outbox only when the action returned nil. With a Reply type, events become part of the reply. Direct Publish stays for work done outside a request.

**Cost.** events.go; volt.go (Handler needs the broker, so generated shims pass c.Events); §event-routes rule 2; the itest event test. Small, and free once Reply exists.

### 28. [consider] Controller methods `Action(w http.ResponseWriter, r *volt.Request, params...) error` (spec §handler-references rule 2; gen/router/testdata/blog_app_handlers.go.golden); volt.Render and RenderStatus (formats.go:69-87); volt.JSON

**Effect.** A controller produces its response as a series of writes mixed in with its logic, and its error return arrives after those writes, so success or failure is decided at one moment and the response at another. Generated query routes already compute `out` first and render once; nothing holds hand-written controllers to the same order.

**Bug.** Demo purity-http/rt TestHeaderAfterRender: a header set after Render is dropped (201 with Location ""). An error returned after Render is only logged, and the client gets 200 with the body. A missing return after an early Render writes two JSON documents into one 200. A test that checks only the status catches none of these.

**Pure form.** Return the response as a value and let the runtime render it once, after the action returns: `type Reply struct{ Status int; Header http.Header; Body any }`, with OK(v), Created(loc, v), NoContent(), and Stream(func(http.ResponseWriter) error) as the escape hatch for SSE and file serving. A method becomes `Show(r *volt.Request, id int) (volt.Reply, error)`. An error then always wins over a half-built reply, and Committed() matters only inside Stream.

**Cost.** Every controller interface and every hand-written controller; the router shim and the handlers golden; §handler-references rule 2 and §formats rule 4; the itest fixtures (about 20 methods); the README; Broker.Serve becomes a Stream reply. Large but mechanical; best done together with the ErrorHandler split and the event outbox.

### 29. [keep] nao/rt/migrate.go:114 `at := time.Now().UTC().Format(time.RFC3339)` in MigrateUp (the ledger's applied_at)

**Effect.** Reads the wall clock inside the runner, once per migration, before BEGIN IMMEDIATE, which may wait up to busy_timeout.

**Bug.** No bug class found. Nothing decides anything from applied_at: down picks by name, status only displays it, and out-of-order detection uses names. The only costs are that MigrateStatus.AppliedAt cannot be compared against a golden, and that the stamp can be earlier than the commit by the lock wait.

**Pure form.** MigrateUp(ctx, db, ms, now func() time.Time), or computing the stamp with strftime(..., 'now') in the ledger INSERT, which only moves the ambient clock into the database.

**Cost.** An extra parameter on a public runtime function and on the generated Migrate, only for test determinism of a display field. Revisit if anything starts ordering or deciding by applied_at.
