# Volt — Roadmap

Everything left to build, organized **by problem solved**. Each feature
has a stable ID (`CRUD-3`, `SEL-2`, `FW-1`), a slice, and a status;
implementation sessions work off this list, and landing a feature
updates its status in the same commit (D26).

Part one is the **data layer** (nao); part two is the **framework**.
How this document relates to the others:

- [`model-layer.md`](model-layer.md) names the data-layer problems;
- [`reference/orm-matrix.md`](reference/orm-matrix.md) proves nothing
  Rails AR does is unaccounted for; [`reference/`](reference/) holds
  the framework inventories the FW-series draws on;
- [`decisions.md`](decisions.md) records *why* each answer is what it is;
- [`spec.md`](spec.md) specifies what already exists;
- **this file** says *what to build*, per problem.

Statuses: `DONE` (built and tested) · `vN` (scheduled slice) · `HAND`
(documented pattern, deliberately hand-written) · `LATER` (compatible,
unscheduled) · `NO` (non-goal, see [D27](decisions.md)).

---

## The ladder: where this tool sits

The Go data-access ecosystem is a chain of supersets. Each layer adds one
capability over the previous — and each maps to one piece of Not an ORM:

| Layer | What it adds | Ecosystem | Not an ORM's version |
|---|---|---|---|
| driver | connections, `Exec`/`Query`, `Rows` | `database/sql` + a SQLite driver | the same base; `rt.Open`, `rt.DBTX` (driver stays test-only, D03/D25) |
| scan ergonomics | rows → structs, named params | `sqlx` | generated per-query scan code — zero reflection, checked at compile time |
| SQL building | composing SQL in Go at runtime | `squirrel` | v2 typed option values — **only** for filter/order/limit (D28–D32) |
| typed queries from SQL | write SQL, generate typed Go | `sqlc` | v1 `Select`/`View` blocks in the schema file, gen-time prepare validation (D06) |
| full ORM | CRUD, associations, migrations, callbacks | Rails AR, GORM, Ent | v0 CRUD + v3 loaders + v4 migrations; callbacks/dirty tracking/lazy loading: never (D27) |

I am not inventing a new problem — I am answering each rung with
codegen instead of runtime machinery, from one authored DBML file.

### Why there is no "any query" builder

An ORM must let you build any query at runtime because runtime is its
only chance to construct SQL. I have a build step, so I split "any
query" by **what actually varies when the program runs** — and the split
is exhaustive (D32, the shape rule):

| What varies at runtime | Answer | Tier | Slice |
|---|---|---|---|
| nothing — fixed shapes | generated CRUD (`UserGet`, …) | [CRUD](#p2--default-crud) | v0 `DONE` |
| values only | real SQL in a declared `Select` block, `:named` params, typed function generated, prepare-validated | [SEL](#p3--custom-queries-static-shapes-select--view) | v1 |
| filter / order / limit | typed predicate **values** (`UserEmail.Eq(x)`) over generated column handles | [DYN](#p4--dynamic-queries-runtime-composition) | v2 `DONE` |
| the shape itself | quarantined report engine over the exported schema catalog | [RPT](#p6--ad-hoc-user-defined-queries) | LATER |

The long tail an ORM covers with its query DSL lands in tier two, where
the query language is SQL itself — already the best language for it —
and SQLite proves every statement at generation time (D02). The only
builder-shaped surface that survives is tier three, and it is inert data,
not string assembly.

---

## P1 — One schema, everything derived

**Problem.** The same truth lives in four places (diagram, DDL, code,
live DB) and drifts ([model-layer.md](model-layer.md)). **Answer.**
DBML is canonical; everything else is generated from `check.Info`.

| ID | Feature | Slice | Status |
|---|---|---|---|
| SRC-1 | Normative DBML spec (EBNF) + conformance corpus | — | `DONE` ([spec.md](spec.md); cross-check retired, D54) |
| SRC-2 | Full front end: scanner, parser, `check.Info`, multi-error recovery | — | `DONE` |
| SRC-3 | `vet`: lint for legal-but-suspicious DBML, docs-enforced rules | — | `DONE` ([lint.md](lint.md), D26) |
| SRC-4 | `volt gen` → models (`nao_models.go`): structs, enums, notes → doc comments, `Null[T]`, singular names + `[model:]` | v0 | `DONE` (D09–D13) |
| SRC-5 | `volt gen --sql` → DDL + seed `INSERT`s from `records` | v0 | `DONE` (D12, D38) |
| SRC-6 | Extensions stay a projectable superset (core diagrams still work) | all | ongoing ([hypotheses H1](hypotheses.md)) |
| SRC-7 | Every field property lives in the schema: `[tag:]` verbatim struct-tag passthrough | v0 | `DONE` (D59, D60; spec §6.3 + A.5) |

## P2 — Default CRUD

**Problem.** 90% of data access is the same eight statements per table;
hand-writing them is boilerplate, an ORM hides them behind reflection.
**Answer.** Generate them as plain, readable Go (v0, shipped).

| ID | Feature | Slice | Status |
|---|---|---|---|
| CRUD-1 | `UserGet` by pk; composite pks → one positional arg per key column | v0 | `DONE` (D15, D17) |
| CRUD-2 | `UserGetMany(ids)` — one `IN` query; missing keys are absence, not error | v0 | `DONE` (D36) |
| CRUD-3 | `UserGetByEmail` per unique column | v0 | `DONE` |
| CRUD-4 | `UserList`, `UserCount` | v0 | `DONE` |
| CRUD-5 | `UserCreate(params)` — excludes auto-increment + defaulted columns, `RETURNING` (D16) | v0 | `DONE` |
| CRUD-6 | `UserUpdate(id, params)` full-row, `RETURNING`; misses → `rt.ErrNotFound` | v0 | `DONE` (D36) |
| CRUD-7 | `UserDelete(id)` — `RowsAffected`-checked miss | v0 | `DONE` (D36) |
| CRUD-8 | No-pk tables: `List`/`Create`/`Count` only | v0 | `DONE` (D17) |
| CRUD-9 | Named `:param` placeholders end to end; identity positional, data in params structs | v0 | `DONE` (D15) |

## P3 — Custom queries: static shapes (`Select` / `View`)

**Problem.** Real applications need joins, aggregates, projections, CTEs,
window functions — "almost any query". ORMs answer with a runtime DSL;
sqlc answers with `.sql` files divorced from the schema. **Answer.**
Declared query blocks *inside the schema file*: structure (select list,
tables, joins, declared params) parsed by my front end and resolved
against `check.Info`; expression bodies opaque and proven by gen-time
prepare against the generated DDL (D06). Each block mints its own result
struct (the shape rule, D32) and a typed function.

Requirements (syntax to be specified in spec.md when built; sketches
are illustrative):

| ID | Feature | Slice | Status |
|---|---|---|---|
| SEL-1 | `Select name (param type, …) { … }` block: declared params (D07), select list, from/joins, opaque `where`/`having`/`group by`/`order by` bodies | v1 | planned |
| SEL-2 | Result struct minted per block (`OrderReport`), fields typed from `check.Info` + SQLite's column metadata | v1 | planned |
| SEL-3 | Generated function `OrderReportQuery(ctx, params) ([]OrderReport, error)`; single-row variants | v1 | planned |
| SEL-4 | Joins inferred from declared refs where unique; explicit `on:` override (D08) | v1 | planned |
| SEL-5 | Gen-time prepare validation of every block against the generated DDL; column-list agreement checked (D02, D24) | v1 | planned |
| SEL-6 | `View` blocks: DDL `CREATE VIEW` + read-only struct + accessors; declared columns verified against the SQL (D24) | v1 | planned |
| SEL-7 | Narrow projections (list views without full hydration) — a `Select` block per shape is *the* mechanism | v1 | planned |
| SEL-8 | Raw SQL strings in `.go` files: outside the safety net, escape hatch documented as "promote it to a Select block" | — | `HAND` (D18) |

## P4 — Dynamic queries: runtime composition

**Problem.** Filters chosen by user input at runtime (search screens,
admin lists) can't be enumerated at build time. **Answer.** Functional
options as inert data (D28): predicates are values over generated typed
column handles, one interpreter renders SQL + positional args in
lockstep (D42), statements cached by text (D31).

| ID | Feature | Slice | Status |
|---|---|---|---|
| DYN-1 | Runtime core: `Pred[M]`, `And/Or/Not`, `Raw` escape hatch, deterministic interpreter, positional `?` binding | v2 | `DONE` (D28, D42) |
| DYN-2 | Generated flat handles: `UserEmail = rt.Column[User, string]{…}` (nullable columns `rt.NullColumn` with `IsNull`/`SetNull`; enum types E-prefixed to stay out of the handle namespace, D11); operators `Eq/Ne/In/Gt/Lt/Like/IsNull/Desc/EqCol…` once in the runtime | v2 | `DONE` (D29) |
| DYN-3 | Generated wrappers for value-less options: `UserLimit(n)`, `UserOffset(n)`, `UserOrderBy(…)`, `UserAfter(…)`, `UserDistinct()`, `UserSet(…)` | v2 | `DONE` (D30) |
| DYN-4 | Verbs sharing predicates: `UserQuery`, `UserCount`, `UserExists`, `UserDeleteWhere`, `UserUpdateWhere` + typed `Assign[M]` setters (partial updates land here) | v2 | `DONE` (D32) |
| DYN-5 | Keyset pagination first-class (`UserAfter(key)`, lexicographic expansion, mixed directions); OFFSET degrades linearly | v2 | `DONE` (D34) |
| DYN-6 | Prepared-statement cache keyed by rendered SQL (LRU), wired via `Queries.WithCache` | v2 | `DONE` (D31) |
| DYN-7 | vet rule `dynname`: generated package-scope name collisions (handle sets, option wrappers vs models/enums) | v2 | `DONE` (D29) |

## P5 — Associations

**Problem.** A `Post` needs its `Comments`; lazy loading and struct
fields lie about what's loaded. **Answer.** Explicit per-ref loaders,
generated from declared refs (D19); loops of queries are officially fine
in embedded SQLite, batching is for atomicity and large N (D35).

| ID | Feature | Slice | Status |
|---|---|---|---|
| ASSOC-1 | Single loaders: `PostAuthorGet(ctx, post)` (FK side), `UserPostsList(ctx, user)` (one-to-many) | v3 | planned |
| ASSOC-2 | Batched loaders: `PostCommentsLoad(ctx, posts) (map[int64][]Comment, error)` — one `IN`, grouped in memory | v3 | planned (D19, D35) |
| ASSOC-3 | Many-to-many through junction tables: `UserRolesLoad` walks both refs | v3 | planned |
| ASSOC-4 | No association fields on row structs, ever — not-loaded vs empty must stay distinguishable | — | `NO` by construction (D14, D19) |
| ASSOC-5 | Polymorphic / STI patterns | — | `HAND`/`NO` (D27; matrix §3) |

## P6 — Ad-hoc (user-defined) queries

**Problem.** End users composing their own reports at runtime — the one
case where the shape is unknowable at build time. **Answer.** Quarantine
it (D33): one package, validated against a generated catalog, values only
ever behind placeholders, output `[]map[string]any` whose sole consumer
is serialization.

| ID | Feature | Slice | Status |
|---|---|---|---|
| RPT-1 | Schema catalog export: tables, columns, types, refs as runtime data | LATER | planned (D33) |
| RPT-2 | Report engine: request validation, joins only along declared refs, placeholder-only values | LATER | planned (D33) |

## P7 — Schema evolution (migrations)

**Problem.** "Good luck" — the hardest sub-layer
([model-layer.md](model-layer.md)). **Answer.** Declarative:
diff current DBML against the last-migrated snapshot; the ledger is
generated, ordered, hash-pinned; I own SQLite's twelve-step rebuild.

| ID | Feature | Slice | Status |
|---|---|---|---|
| MIG-1 | Semantic diff: DBML vs snapshot (never SQL parsing); adds/drops/type changes | v4 | planned (D20) |
| MIG-2 | Renames via `[was: 'old']` provenance hints (extension) | v4 | planned (D20) |
| MIG-3 | Generated migration ledger + `schema_migrations` table; each migration embeds the schema content hash → drift is machine-detectable | v4 | planned (D21) |
| MIG-4 | Owned twelve-step rebuild for changes ALTER TABLE can't do (pragma bracket, `foreign_key_check` after) | v4 | planned (D22) |
| MIG-5 | Apply via CLI or embedded runner (`app migrate up`, `embed.FS`); **never** on app start | v4 | planned (D04, D23) |
| MIG-6 | Irreversibility honesty: no fake `down` for destructive changes | v4 | planned |
| MIG-7 | Data migrations (backfills) | — | `HAND`, documented pattern; kept out of schema migrations |

## P8 — Data integrity (constraints & validation)

**Problem.** Where validation lives (struct vs controller vs DB) and how
much the database enforces. **Answer.** Structure is the database's job
(generated DDL), business invariants are typed Go on the models (D18).

| ID | Feature | Slice | Status |
|---|---|---|---|
| INT-1 | `NOT NULL`, `UNIQUE`, FKs with `ON DELETE/UPDATE`, composite PKs, 1:1 uniqueness, `CHECK` expressions | v0 | `DONE` (DDL) |
| INT-2 | Enums: string-backed types + constants; DB `CHECK` opt-in only (twelve-step-dance avoidance) | v0 | `DONE` (D11, D12) |
| INT-3 | Int-backed enums via `[repr: int]` (extension) — full String/Valuer/Scanner/JSON suite, int never escapes the process | LATER | planned (D11) |
| INT-4 | Business validation: hand-written methods in sibling files of the generated package | — | `HAND` (D18); Go-reference checks (INT-6) formalize the entry point |
| INT-5 | Optimistic locking via `[lock_version]` (extension): `Update` gains version check + bump | LATER | planned (matrix §2) |
| INT-6 | Validation rides `checks`: typed predicates → SQL `CHECK` + generated `Validate()`; Go-reference checks → validator only | v0 | `DONE` (spec §V12, D61) |

## P9 — Transactions & concurrency

| ID | Feature | Slice | Status |
|---|---|---|---|
| TX-1 | `Queries` over `DBTX`: same generated methods inside and outside a tx; `WithTx`, `q.Tx(ctx, fn)` | v0 | `DONE` |
| TX-2 | `rt.Tx`: begin/commit, rollback on error or panic | v0 | `DONE` |
| TX-3 | Single-writer discipline: one-connection pool, WAL, `busy_timeout` via `rt.Open` | v0 | `DONE` (D37) |
| TX-4 | Batched loaders as the *atomicity* answer to loops of reads | v3 | planned (D35) |
| TX-5 | Nested transactions / savepoints | — | `LATER`; error today, honest about it |

## P10 — Runtime & deployment

| ID | Feature | Slice | Status |
|---|---|---|---|
| RUN-1 | `rt` package: `Null[T]` (value-or-null JSON), `DBTX`, `ErrNotFound` (= `sql.ErrNoRows`), `Open`, `StmtCache` — stdlib-only, registers no driver | v0 | `DONE` (D03, D13, D36–D37) |
| RUN-2 | Everything library-callable; CLI is a thin wrapper | v0 | `DONE` (D04) |
| RUN-3 | User binaries embed subcommands + migrations: one self-contained deployable | v4 | planned (D04) |
| RUN-4 | Generated code header + clobber protection in the CLI | v0 | `DONE` |

## P11 — Guarantees (the correctness harness)

The features above are only as good as their proofs. Standing invariants,
all enforced by `go test ./...`:

| ID | Guarantee | Status |
|---|---|---|
| PROOF-1 | Spec ⇄ conformance corpus; verdicts pinned against upstream `@dbml/parse` before the cross-check retired at 0 disagreements (D54) | `DONE` |
| PROOF-2 | Generated Go: gofmt-stable goldens + compiled by the real toolchain | `DONE` |
| PROOF-3 | Generated SQL: executes on real SQLite (`foreign_key_check` clean) | `DONE` |
| PROOF-4 | Every generated statement prepares against the generated DDL (cross-generator coherence) | `DONE` |
| PROOF-5 | `itest`: full CRUD round trips through a real driver; generated files drift-tested | `DONE` (D25) |
| PROOF-6 | vet rules ⇄ lint.md ⇄ testdata consistency test | `DONE` (D26) |
| PROOF-7 | v1 onward: every `Select`/`View` block prepare-validated at gen time; declared columns verified | with v1 |

## Non-goals

Permanent, with reasons — see **D27**: identity map / unit of work, lazy
loading, active-record `Save()`, callbacks, dirty tracking, default
scopes, single-table inheritance, reflection in generated paths, a
Turing-complete query DSL in Go. Every one is either a lie waiting to
happen (lazy loading, callbacks) or a worse language for a job SQL
already does.

---

## Build order

v1 `SEL-*` → v2 `DYN-*` (+ INT-3/INT-5 opportunistically) → v3 `ASSOC-*`
→ v4 `MIG-*` + RUN-3 → RPT-* when a consumer exists. Each feature lands
with its PROOF-row test before it's called done.

---

# The framework

The routing layer shipped v0 (spec Part II; the proof chain runs on
every `go test ./...`). What follows is the framework's build list, in
priority order. The first two entries are what stand between the
language and running a real application on it.

## FW-1 — The `Select` query layer — **structured half `DONE`** (2026-09-01)

Landed: `Group` (overlapping table sets with +/- algebra, §V9), `Pred`
(the closed, statically typed predicate language, §V10) and
`Select ... for <group> where <expr> [order: ...]` (§V11) — one typed
method per group member, one signature for all (the §V11.4 agreement
rule), WHERE rendered once, driver-proven in `nao/itest`. Design law:
D57, D58.

Still open from the original slice, in the maintainer's build order:

- **Column projection — `DONE` 2026-09-02** (spec §V11.7). Clauses
  read in SQL order — name, columns, source, filter:
  `Select <name> (<columns>) for <target> where …`. Three cases:
  1. **Explicit list = intersection = one shared type.**
     `Select summary (site, day) for metrics where at` — every listed
     column passes field-type agreement (nullability included), so one
     shared struct named `<SelectName>` (`Summary`) serves every
     member: one wire type, N sources.
  2. **No list = full row = per-member models**:
     `[]PageView`, `[]LinkClick` — heterogeneous rests welcome.
  3. **Star with exclusion = per-member struct derivatives.**
     `Select public (* - password_hash) for accounts` — each member
     projects its own columns minus the exclusions (the
     `SELECT * EXCEPT` BigQuery has and standard SQL lacks), minting
     `<Model><SelectName>` (`UserPublic`) with every kept field copied
     verbatim — Go type, assembled tag (`tag:` passthroughs included),
     doc comment. A column that structurally cannot exist beats a
     serialization tag that must be remembered — the sensitive-data
     case. Excluded columns must exist in every member (§V9.3's "the
     algebra must say something true"); per-single-member exclusion
     stays hypotheses H3.
  Where/order columns need not be projected; minted row-type names are
  collision-checked against the generated package scope; the LSP
  hovers a select with its signatures, rendered SQL and output row
  structs (the shared type once, derivatives per member). Proof:
  corpus v55/i64-i70, generator output compiled and driver-proven in
  `nao/itest`, grammar corpus, hover tests.
- **Raw-SQL `Select`/`View` blocks** — real SQL declared in the schema
  for shapes the closed language refuses (joins, aggregates), typed
  functions out, prepare-validated (D06, D24).

FW-2 needs the structured half only, so Datasets are unblocked.

## Known gaps (adversarial audit, 2026-09-02)

Confirmed by reproduction, not yet fixed — listed so the docs never
imply otherwise (D49). Each is small; none blocks FW-2.

- **Editor, cross-file.** Column occurrences are indexed per file: a
  select, enum reference or `default: status.x` in a *sibling* `.volt`
  file of the same package is neither navigable nor renamed from the
  declaration. Pred hover from a sibling file shows only the name.
- **Editor, uncovered spellings.** No hover/definition/references for
  select parameters (`:site`), for columns inside a Pred body, or for
  projection/order columns of a group-targeted select (by design the
  latter are not renamed; they should still navigate). References on a
  table list its declaration twice; hover ranges on qualified
  references span the qualifier.
- **Diagnostics anchoring.** A Pred that fails typing at a use site is
  reported inside the Pred declaration, once per failing select or
  check, without naming the use. Two same-named projected selects on
  disjoint tables are refused with a message naming one select twice.
  A projection over a member with an unmapped column type leaks the
  generator's internal message.
- **Selects.** `TableGroup` as a select target gets the generic "no
  such table or group" without saying that TableGroup is a diagram
  construct (D58). Generated doc comments lower-case the first letter
  of a select name instead of quoting the schema spelling.
- **Validation.** `Validate()` does not enforce enum membership (the
  DDL `CHECK … IN` does); `rt.Null[T]` implements JSON only, so a
  non-JSON encoder reached through `tag:` sees the wrapper's fields;
  Pred-reference checks render with redundant parentheses.
- **CLI.** `Project.Module` is required but unused; a nonexistent
  argument gets a loader message rather than "no such directory";
  diagnostics print absolute paths even for relative arguments;
  cascading import errors are not collapsed.

## FW-2 — Datasets: schema-driven group expansion

`Dataset` (reserved word, spec §V8) turns a `TableGroup` into a full
CRUD surface: `resources` where the loop variable is bound by the
schema. Condensed design (argued in full in working sessions of
2026-07; supersedes the old router draft §12):

- **Declaration.** `Dataset ms [from: db.group(MS), pipe: api,
  formats: (html, json, gob)] { path: strip('ms_')  key: id
  ops: (list, create, update, delete)  … }` — gen checks every group
  member has the declared key (gen error naming the table), then emits
  per table: the routes, a typed instantiation, a `[]volt.Column`
  spec, and `paths` helpers. A `GET /<dataset>/_meta` index route
  replaces hand-maintained meta-tables.
- **Matching by instantiation.** The expansion loop emits the route
  string and the handler call from the same iteration — they cannot
  disagree. The runtime ships one generic implementation per operation
  (`dataset.List[T]`); generation picks `T`, so `/ms/revenue` returns
  `[]models.MsRevenue`, compiler-held end to end. Zero reflection.
- **The one runtime-dynamic surface** is `GridQuery`
  (`?sort=-year&f.year=2024`), validated against the generated column
  whitelist: unknown column → 400, values parsed to the column's type,
  predicates mapped onto nao's typed predicate values (needs FW-1/DYN).
  Injection impossible by construction.
- **Renderer seam.** Generic handlers end with
  `return render(w, r, page)`; the default renderer negotiates HTML
  (per-table template if present, shared grid template otherwise —
  missing generic template is a gen error, not a 500), JSON, and GOB —
  the Go-native arm: a desktop client imports the same generated
  models package and decodes `dataset.Page[models.MsRevenue]` directly.
- **Override ladder** (R14's shape, now the model for all generated
  surfaces): nothing → one operation on one table
  (`ms_revenue [list: App.MsRevenueList]`, override receives the
  generated default as `next` — wrap or replace) → presentation only
  (drop a template file) → `except:` a table and hand-write it → no
  `Dataset` at all. Every override visible in source; `volt routes`
  shows generated vs overridden per row. Datasets regenerate forever
  *around* what you have claimed.
- **Column metadata.** Lift `'Title, unit=EUR, dict=YesNo'` note
  conventions into `Column{Title, Unit, Dict}` now; promote to
  first-class settings (`[unit:]`, `[dict:]`) later; vet warns on
  notes that do not parse.
- **Honest costs.** Column whitelist contains (not eliminates) the
  dynamism; generated routes grow linearly with group size; the note
  convention is load-bearing until promoted.

## FW-3 — Schema extensions from the old design

`[was:]` (rename memory for migrations, with MIG-\*) and `[repr:]`
(storage representation) — D-series law exists (D18–D24 for
migrations); blocked behind FW-1 and the migration slice v4.

## FW-4 — Action View: typed compiled templates

The biggest win available after datasets, and the gateway to FW-5:
templates compiled at gen time, variables typechecked against the
models they render (what `html/template` cannot give). Declared in the
language, generated as plain Go. Draws on: reference/comparison.md P6,
gostd.md's `NO` rows for components/layouts.

## FW-5 — Sub-libraries worth building (Rails family, curated)

Verdicts from the 2026-08 session (full argument in the session
record): **build** Active Job analog (declared job types → typed
`Enqueue`, the stringly-typed payload is the real pain) and Action
Mailer analog (small once FW-4 exists — a Mailer declaration generates
typed send functions over an SMTP runtime). **Do not build** —
integration products, point at existing Go libraries: storage, cable
(websockets), mailbox (inbound email), rich text. **Already have the
part that pays** of Active Support: `nao/inflect`. Each new
sub-library is a declaration form + a generator + a small runtime, and
each costs a spec section, parser, checker, grammar rule, LSP
awareness and corpus cases — the language makes sub-libraries better,
not cheaper.

## FW-6 — Router conveniences (parked)

Plugs from imported packages (a v0 check error on purpose);
`volt.WithQueries` / `WithTemplates` / `BuildDeps` runtime options;
LSP navigation into Go symbols (handler refs `Home.Index`,
`error_handler:`) — needs a Go-side index the server deliberately does
not have yet.

## FW-7 — Executable spec examples

Extract the code blocks from spec.md and run them through the front
end in CI, the way lint.md's examples already run — closing the last
gap between "the spec is tested" (constraints, via the corpus) and
"the spec's own prose is tested" (examples). Cheap, high-trust.

