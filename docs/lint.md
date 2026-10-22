# Volt Vet Rules

> [!CAUTION]
> **This project is an alpha version, and everything may change in the future.**

This document specifies every `vet` analyzer: what it reports, why the
pattern is a concern even though it is **valid Volt**, and its known
limitations. It is the lint annex to the [language specification](spec.md);
errors (invalid Volt) are the [check package's](../lang/check/) business and are
not listed here.

**The examples are executable, not prose.** Each rule links to its test
file under [`testdata/`](../lang/vet/testdata/). In those files, a line ending in
`//WANT <rule>` is a **bad** example — the analyzer MUST warn on exactly
that line — and every unmarked line is a **good** example — the analyzer
MUST stay silent. `go test ./lang/vet/` verifies both directions, and
`TestRulesDocumentation` fails the build if any analyzer is missing from
this document, links to a nonexistent file, or has no bad example in its
linked tests. The docs cannot silently drift from the code. The
complete verdict of one project, every rule speaking at least once
across the files of a package, is a golden too
([`lang/testdata/vet.golden`](../lang/testdata/vet.golden), refreshed
with `go test ./lang -run TestVetGolden -update` after reading the
diff): it pins position, message and how many times a rule speaks for
one cause, so a change to how vet is computed shows its every effect.

Run every rule over the named packages with:

```sh
volt vet ./...
```

`volt vet` is `volt check` plus these rules: each package's
declarations are analyzed as one file, the way the checker sees them
(§projects-and-packages), alongside the one project-level rule, `vet/pipeline`, which
reports a pipeline no scope pipes through. The language server runs
the same rules on a file that checks clean. There is no per-rule
switch: the rules are cheap, and a suppressed warning would be a rule
the document lists but the tool does not run.

A rule is judged declaration by declaration: a pure function of one
declaration's syntax and the facts the checker resolved for it (its
checked table, its partial, the relationships starting at its
columns), so an editor session re-judges only the declarations an
edit changed. The rules that need the whole file (a foreign-key
cycle, names that differ only in case across tables, the generated
names that collide) fold over the checker's model and its naming plan.
Two warnings at one position stand in the order the rules
are listed below, which is the order they are registered in;
`TestRulesDocumentation` asserts the two agree.

---

## Dead declarations

The analogue of Go's "imported and not used": a declaration nothing
references is usually a refactoring leftover, and it keeps readers
searching for uses that do not exist.

### unusedpartial

**What it does.** Reports a `TablePartial` that no table injects.

**Why.** A partial's only effect is through the tables that inject it;
an unused partial is dead weight that still occupies a name in its
package.

**Examples.** [testdata/unused.volt](../lang/vet/testdata/unused.volt):
`orphan_partial` is bad; `used_partial` is good.

**Limitations.** Analysis is per-package: the files of a package are
analyzed as one (§projects-and-packages). Treat the warning as "unused
*here*".

---

## Redundancy

Two ways of saying the same thing eventually disagree. Each of these
patterns states something the language (or another line) already
guarantees.

### redundantindex

**What it does.** Reports four index patterns:
1. `unique` or `index` on the column of a single-column primary key,
   which is unique and indexed already;
2. a single-column `[unique]` index under `where:` on a column that
   carries `unique` or is the single-column primary key, either of
   which holds over the whole table already;
3. an index without `where:` whose key is the columns of a composite
   primary key in its order, which the key's own index already is;
4. two indexes with an identical key (column order significant) and the
   same `where:` condition, or none on either, under two names.

**Why.** The duplicate creates a second physical index (cost on every
write) or a second spelling of the same constraint that can drift from the
first.

**Examples.** [testdata/redundantindex.volt](../lang/vet/testdata/redundantindex.volt):
`(email, country)` and `(country, id)` coexisting is good, since
composite indexes with different column order are different indexes,
and so are a composite key `(user_id, group_id)` and an index on
`(group_id, user_id)` or on `group_id` alone. So are a column's
`index` on `slug`, `slug` under `` where: `deleted_at IS NULL` `` and
`slug` under `` where: `deleted_at IS NOT NULL` ``: a partial index
covers only the rows its condition admits (spec §indexes). Two
indexes of one name are not this rule's: the checker refuses them
(spec §indexes).

**Limitations.** Expression atoms and `where:` conditions are compared
textually: `` `a+b` `` and `` `a + b` `` count as different keys.

### emptybody

**What it does.** Reports empty `Indexes {}` and `Validators {}` blocks,
and a `Project` with neither properties nor a `note:` (spec §project).

**Why.** An empty block does nothing but implies work in progress.

**Examples.** [testdata/emptybody.volt](../lang/vet/testdata/emptybody.volt):
every empty block warns; the table's column is the good line.
[testdata/emptybody_note.volt](../lang/vet/testdata/emptybody_note.volt):
a `Project` that carries only a note is not empty, and draws nothing.

### shadowedcolumn

**What it does.** Reports a column definition that overrides another via
partial-injection conflict resolution (spec §tablepartial) — table-over-partial or
later-partial-over-earlier-partial, and the columns of a partial a table
injects more than once, which come in twice.

**Why.** The override is **silent**: no error, last writer wins. A type or
constraint change in the losing partial then has no effect on this table,
which is invisible at the partial's definition site.

**Examples.** [testdata/shadowedcolumn.volt](../lang/vet/testdata/shadowedcolumn.volt):
both conflict directions and a partial injected twice are bad;
non-overlapping columns are good.

---

## Modeling traps

Valid Volt that describes a database likely to misbehave. This is the
family where a warning most often prevents a production incident.

### missingpk

**What it does.** Reports a table with no primary key: no column
carrying `pk`, injected partials included (spec §columns).

**Why.** Tables without primary keys defeat replication, upserts, ORMs and
row identity in general. Almost always an oversight.

**Examples.** [testdata/missingpk.volt](../lang/vet/testdata/missingpk.volt): a
single `pk` column and several, a composite key, are good.

### reftype

**What it does.** Reports a relationship whose two columns have different
declared types, compared pairwise for composite endpoints.

**Why.** Nothing in the schema requires type agreement, but the database
either rejects the FK or performs implicit casts (`string` vs `int`
breaks joins).

**Examples.** [testdata/reftype.volt](../lang/vet/testdata/reftype.volt).

**Limitations.** Types are compared as written: a declared type and its
storage type are reported as different. A pair with a column of unknown
type is not compared; the checker reports that type once, at the
column (spec §columns).

### reftarget

**What it does.** Reports a foreign key whose referenced ("one"-side)
column set is not guaranteed unique (spec §relationships-ref, targets):
not exactly the columns of the primary key, not one column that
carries `unique`, and not exactly the key of a `[unique]` index whose
key is columns only and which has no `where:`. A unique index with
`where:` constrains only the rows its condition admits, and one with an
expression constrains no column (spec §indexes), so SQLite refuses
either as a foreign-key target; one member of a composite key alone is
not unique either. The referenced columns are those right of `>` in a
`Ref`, or the endpoint of a column's `ref:` (spec §relationships-ref).
The generator asks the same question to give `<Model>GetBy<Column>`
(spec §generated-query-surface).

**Why.** SQL requires FK targets to be unique; a schema that violates this
fails at export or, worse, models a "to-one" relationship that the data
cannot uphold.

**Examples.** [testdata/reftarget.volt](../lang/vet/testdata/reftarget.volt): the
composite case shows that only an index with **the same column set** makes
a composite target unique, and a composite key is a target as a whole,
in any order, while one of its members alone is not; `accounts.email`,
unique only where `deleted_at IS NULL`, and `accounts.login`, unique
only together with an expression, are no targets, while the unique
column `accounts.handle` is; a one-to-one names its target like any
ref, so `bios.user_id [null, unique, ref: users.id]` is good.

### selfref

**What it does.** Reports a relationship with the same column on both
sides (`Ref: employees.id > employees.id`).

**Why.** A column trivially equals itself; the relationship is either a
typo for a hierarchy edge (`manager_id > id`) or meaningless.

**Examples.** [testdata/selfref.volt](../lang/vet/testdata/selfref.volt) —
`manager_id > id` (a real hierarchy) is good.

### refcycle

**What it does.** Reports cycles in the foreign-key graph between distinct
tables (`a → b → a`), each cycle once.

**Why.** Cyclic FKs mean no valid insertion order exists without deferred
constraints or nullable columns, and bulk loads/dumps need special
handling. Sometimes intentional — then the warning documents the decision.

**Examples.** [testdata/refcycle.volt](../lang/vet/testdata/refcycle.volt) — the
acyclic `c → a` edge is good; self-references are excluded (that hierarchy
pattern is legitimate and covered by `selfref` only when degenerate).

### casecollision

**What it does.** Reports two names in one namespace differing only in
letter case: tables, and columns per table.

**Why.** Volt names are case-sensitive (spec §case-sensitivity-and-terminology) but many databases and
most humans are not; `users` next to `Users` is a bug generator.

**Examples.** [testdata/casecollision.volt](../lang/vet/testdata/casecollision.volt).

---

## Code generation

Rules that guard the schema → Go mapping (spec §mapping-to-go) rather than the schema itself.

### modelname

**What it does.** Reports a table whose singular Go model name is a
*guess*: the [inflector](../nao/inflect/) could not confidently singularize the
table name (endings like `-us`, `-is`, `-ss` where a trailing `s` is often
not a plural marker at all) and no `[model: "..."]` setting pins the name.

**Why.** Models are singular by the singular-model rule (`users` → `User`), but
English pluralization is not decidable by rules: `menus` wants `Menu`,
`status` wants `Status`, and a deterministic inflector cannot know which is
which. Generation proceeds with the guess either way; this rule marks
exactly the places where the guess deserves a human eye. Writing
`[model: "Menu"]` both fixes the name and silences the warning; the
setting is the documentation.

**Examples.** [testdata/modelname.volt](../lang/vet/testdata/modelname.volt) —
regular plurals (`users`, `order_items`), irregulars the inflector knows
(`people`, `account_statuses`), and a pinned `menus [model: "Menu"]` are
good; `order_status`, `bonus` and `axis` warn.

**Limitations.** Only the final underscore-separated segment is inflected,
and the confidence heuristic is suffix-based: a wrongly-guessed regular
plural (schemas full of domain jargon) stays silent. `[model:]` always
wins; use it whenever the generated name matters.

### dynname

**What it does.** Reports declarations whose *generated dynamic-query
names* collide in Go package scope. Generation mints flat
package-scope names by concatenation from the model name: the model
struct itself, the changeset `<Model>Changeset`, the aliases
`<Model>Column` and `<Model>Params`, the constructor `<Model>New`
(§changesets), the column handles `<Model><Field>`, and the option
wrappers `<Model>Limit`, `<Model>Offset`, `<Model>Distinct`,
`<Model>OrderBy`, `<Model>After`, `<Model>Set` of the dynamic layer,
so two distinct declarations can flatten to one Go identifier: `Table
user_limits` mints model `UserLimit`, exactly the option wrapper of
`Table users`; `users.foo_bar` and `user_foos.bar` both mint the
handle `UserFooBar`. A bare declared type shares that scope, since the
package's own Go files declare it (§types); a declared type that takes
a generated name is the checker's error, not this rule's warning.

**Why.** `volt gen` refuses such schemas outright (loud failure, never a
silent rename), but the generator can only name the first collision; this
rule reports every collision, at the later of the two declarations, with
both origins spelled out. The name derivation is the generator's own — the
rule calls into `nao/gen/golang` — so rule and generator cannot drift apart.

**Examples.** [testdata/dynname.volt](../lang/vet/testdata/dynname.volt): `users`,
`orders` and the declared type `OrderState`, named apart from the
handle `OrderStatus` of orders.status, coexist fine;
`user_limits` (model = users' `UserLimit` wrapper), `user_foos.bar`
and `user_foo_bars` (handle/model = users' `UserFooBar` handle),
`users.new` (handle = users' own `UserNew` constructor), `user_columns`
(model = users' `UserColumn` alias) and the pinned models `UserParams`
and `UserChangeset` (users' write body and changeset) all warn.

**Limitations.** Per-file, like every rule here. A schema that fails
generation planning altogether (unusable names, field collisions) reports
nothing from this rule — generation raises those errors itself.

---

## Candidate rules — considered, not implemented

Reviewed and deliberately deferred; listed so a reader can see what was
*not* overlooked but judged out of scope for now. Promoting any of these is
one file in `vet/` plus one testdata file.

| Candidate | What it would report | Why deferred |
|---|---|---|
| `orphantable` | tables with no relationships at all | too noisy: lookup/config tables are legitimately standalone |
| `nullablefk` | nullable FK columns, i.e. zero-or-many cardinality (spec §nullability-and-cardinality) | that cardinality is a valid modeling choice, not a smell |
| `recordsdupkey` | duplicate primary-key values across record rows | belongs in `check` beside record value typing (§records-sample-data) |
| `sqlkeyword` | names that are reserved words in common SQL dialects (`order`, `user`) | needs per-dialect wordlists and a target-dialect setting |
| `namingstyle` | mixed naming conventions (snake_case vs camelCase) | pure style; needs configuration machinery first |

---

## Adding a rule

1. Create the analyzer in a `vet/*.go` file, a named type embedding
   `meta` (its name and doc), and `register` it. A check on one
   declaration implements `Rule`: `Decl(d, nodes, facts)` returns its
   warnings from the declaration, its nodes in source order and the
   facts the checker resolved for it, and nothing else — the purity
   gate walks it: no package state, no function values, no
   writes through an input, no sort of anything the inputs reach. A
   check on the whole file implements `Fold`: `File(pass)` reads the
   checker's model and its naming plan (`FilePass.Info`,
   `FilePass.Plan()`). One type may be both.
2. Add `testdata/<rule>.volt` with `// analyzers: <rule>` on line 1, bad
   lines marked `//WANT <rule>`, and good lines unmarked.
3. Add a `### <rule>` section here linking that file, and make the
   rule fire in `lang/testdata/vet` so the project golden pins its
   every warning.

`TestRulesDocumentation` and `TestAnalyzers` enforce that the first
three exist and agree; `TestVetGolden` pins the fourth.
