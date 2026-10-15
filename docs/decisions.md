# Decisions

The standing rulings of Volt, each under a name. A ruling is cited by
that name: `decision sqlite-first` in a comment, a diagnostic or a
doc, `decisions one-parse, pure-memos` for several. Names are unique
and hyphenated; `lang/spec_doc_test.go` fails the build on a citation
that names no heading here. A decision is changed by editing it,
removed when it has become a spec rule or is obsolete, and never
drifted from. An entry states the ruling and its reason. How it was
found, what it measured and what it replaced live in git history. What
is not yet decided lives in [`hypotheses.md`](hypotheses.md).

## Scope and identity

### sqlite-first

SQLite is the only database, all in. Every simplification bought by
rejecting other engines is a good trade, and SQLite is also tooling:
at generation time an in-memory SQLite is the SQL parser and type
checker, so everything emitted or accepted is prepare-validated.

### zero-dependencies

Generated code imports the standard library and Volt's own runtime
packages, nothing else; the runtime registers no driver, the
application imports one and passes its name. Proven shapes may be
borrowed (Queries/DBTX), code is not.

### library-first

Everything the CLI does is importable. User binaries embed the
subcommands and the migrations; one self-contained deployable.

### packages-as-in-go

`volt gen` with no argument generates the package in the working
directory, `volt gen ./app` that package, `volt gen ./...` every
package beneath, exactly `go build`'s rules. Output goes to the named
packages only; their imports are loaded for checking, never written.

### editor-tooling-in-repo

The tree-sitter grammar (`grammar/`), the Zed extension
(`zed-extension/`) and the language server (`lsp/`, served by
`volt lsp`) live in this repository. Every language change must move
the parser, checker, vet, corpus, grammar and editor features in
lockstep, and one repository makes that one commit.

### one-binary

One CLI, `volt`, with the language server as its `lsp` subcommand.
What the data layer emits is named after nao, like `protoc`'s
`.pb.go`: `nao_models.go`, `nao_queries.go`, `nao_schema.sql`.
Plain `.dbml` files stay fully supported as input.

### newest-go

The floor is the newest stable Go, declared in all three go.mod files
and go.work. A personal framework tracks the toolchain it is developed
on; `GOTOOLCHAIN=auto` makes the floor cheap for anyone else.

### no-example-apps

No example application in the repository: it would be a second
application to maintain through every change, and the maintainer
already builds a real one on Volt. The executable proof is the test
suites, `itest/` for routing and `nao/itest` for data, as languages
keep it; worked examples belong in the docs.

## Language and front end

### expressions-are-sqlites

Structure is Volt's, expressions are SQLite's. Declared raw-SQL
queries parse their select lists, tables, joins and declared
parameters with Volt's front end against the checked schema; WHERE,
HAVING and other expression bodies stay opaque and are validated by
gen-time prepare. Volt never builds an SQL expression grammar. Query
parameters are declared, never inferred (`Select foo (year int)`),
and joins are inferred from a unique declared ref between the joined
tables, with an explicit `on:` as the override.

### closed-predicates

`Pred` (spec §V10) is a closed, typed language: comparisons,
and/or/not, in, like, is null, type-checked at every use site, and it
never grows toward SQL. That carve-out is what keeps
expressions-are-sqlites intact. What it cannot say belongs in raw-SQL
Select/View blocks or the dynamic layer. Checks (§6.6) are its
parameterless ancestors.

### group-vs-tablegroup

`Group` (spec §V9) is a codegen set: overlapping freely, composed by
set algebra where re-adding a member or removing a non-member is an
error, because the algebra must say something true. `TableGroup` is a
diagram partition. The §V11.4 agreement rule makes group selects
honest: every referenced column exists in every member with one
agreed type, so one WHERE rendering and one Go signature serve all.

### set-difference

A TableGroup is a set wherever a group is named, resolving last after
tables and groups. Difference is spelled `\`, the set operator, with
`(a, b)` for a union of names; `-` is an error naming `\`. Star
projections use the same spelling, `(* \ a)`, so "without" is one
operator everywhere.

### go-module-is-the-project

A Volt project is a Go module: `go.mod` marks the root, there is no
`volt.mod`. With it comes Go's loading model (§V1.7): tools load the
named packages plus their transitive imports, and `testdata`, dot and
underscore directories, `node_modules` and nested modules are
excluded as Go excludes them. Root-relative imports need a root, and
the Go module is the honest one.

### go-references-by-spelling

A `.volt` file names Go in one place, the containing package's own
non-test Go files, and the checker reads them with the standard
parser and holds every reference to a declared top-level function
with the exact contract: `func(http.Handler) http.Handler` for
plugs, `func(<the columns' generated types>) error` for checks, the
runtime's ErrorHandler shape for `error_handler:`. Types are compared
by spelling against Appendix A's generated names, never resolved: no
build, no type checker. A typo is an error at the reference; an
unimplemented function is a to-do with its signature in the message.
A named type where a `string` column is passed is refused; spell the
generated type.

### named-sections

Spec sections and decisions are named, not numbered. A heading is
stable and a number moves whenever something is inserted. Existing
`§` citations resolve through the spec's Citation key until the
migration to heading names lands (roadmap, deferred chores).

### one-corpus

The conformance corpus is one tree, `lang/conformance/snippets/
{valid,invalid}`: `.dbml` entries exercise the single-file schema
pass, `.volt` entries and project directories the project pass. The
upstream `@dbml/parse` cross-check is retired at zero disagreements;
if upstream grows a feature worth adopting, it is spec'd and corpus'd
then.

### one-parse

An element boundary is a rule of the language (§3.2.5): an unquoted
identifier in the first column outside every brace, string and
comment begins an element and ends the one before it. The scanner
enforces it and nothing else does; a chunk's scan stops before the
next element start, and the scanner cannot see past a line break it
has not consumed, so a chunk's tokens, parse and diagnostics are a
function of its text alone. `ParseFile` is the chunked parse from
nothing; there is no second lexer and no whole-file path for it to
differ from. Braces alone count, not brackets or parentheses.

### carriage-return

The source is scanned as written: a carriage return keeps its byte
and has no column, a token never starts on one, a value never
contains one, so offsets, lines and columns describe the file the
editor shows. The source is never rewritten before scanning.

### invalid-input-goldens

What invalid input produces is a rule of the language (§3.2 rule 6):
a syntax error inside an element discards the rest of the element up
to the next element start, and inside a one-per-line body it discards
the rest of the item's line. Every diagnostic of every invalid
snippet is a golden, refreshed by `-update` after reading the diff,
and a scripted stdio session against the real server process is a
golden too (`cmd/volt/lsp_session_test.go`). Nothing a user sees
changes without a golden changing with it.

### executable-grammar

The spec's EBNF is data: a reader of ISO 14977 turns every block into
a grammar, the lexical facts the prose states beside it are cited
rules, and the test holds that every block reads with every name
defined once, the collected grammars repeat the sections exactly,
every derived sentence parses, and every text one token away from one
is decided alike by grammar and front end. A grammar is never edited
without the test, and the parser is never stricter or more lenient
than the grammar states.

## Generated Go

### subject-first-naming

Naming is subject-first, verb-last: `UserGet`, `UserList`,
`UserCreateParams`, `EOrderStatusPending`. Autocomplete and search
group by model, then filter by verb. Hand-written internals follow
the same rule, with carve-outs for Go-mandated interface methods,
`New*`/`is*`/`has*`, stdlib-parallel APIs and cursor primitives.

### singular-models

Models are singular (`User` from `users`) via a small deterministic
inflector; `[model: 'Person']` overrides; `volt vet` warns when the
inflector guessed. Go names use the initialisms convention (`UserID`).

### e-prefixed-enums

Enums are string-backed, stored as TEXT with a `CHECK (col IN (…))`
(Appendix B), and the type carries an `E` prefix (`type EOrderStatus
string`, `EOrderStatusPending`). The prefix keeps enum types out of
the flat column-handle namespace: `Table orders { status
order_status }` mints the handle `OrderStatus` and the type
`EOrderStatus` instead of colliding, and the tax lands on enum
mentions rather than query code. Int-backed `[repr: int]` is an
unbuilt opt-in whose int must never escape the process.

### null-t

A nullable column is `Null[T]`, the runtime's generic value type with
Scan/Value and JSON that marshals as the value or `null`. Value
semantics, no heap, no nil dereference.

### values-first

Each scan is a fresh plain struct: no identity map, no `Save()`, no
association fields on row structs. This states what is built; which
ActiveRecord-grade conveniences join it is open
([`hypotheses.md`](hypotheses.md) H2), inside the line non-goals
draws.

### named-placeholders

Static SQL uses SQLite's named parameters (`:email`) bound with
`sql.Named`: the placeholder is the column name, so argument order
cannot exist as a concept and the SQL is self-documenting. At the Go
boundary, identity arguments (the key columns of Get and Delete) are
positional and data arguments arrive in a generated params struct
whose fields map one to one to the placeholders.

### create-params

CreateParams exclude auto-increment keys and defaulted columns; the
INSERT omits them and `RETURNING` brings back the row the database
wrote. Overriding a default is a dynamic or declared query. Never
guess: a defaulted column is the database's value.

### keyless-tables

Tables without a primary key get only `List`, `Create` and `Count`.
Composite keys get one positional argument per key column in `Get`,
`Update` and `Delete`.

### sibling-files

Hand-written code extends generated types in sibling files of the same
package. Generated business logic is a non-goal. Raw SQL strings in
`.go` files are outside the safety net; the supported escape hatch is
a declared `Select`, which regeneration re-validates.

### schema-owns-fields

Every per-field property is a schema declaration, because generated
code cannot carry hand-authored metadata: models sit behind the
DO-NOT-EDIT marker and the next `volt gen` erases a hand-written
struct tag. A per-field degree of freedom exists as a column setting
in Volt or does not exist. A side config file is a second source of
truth and is rejected.

### tag-passthrough

A column setting earns a first-class name only when Volt itself acts
on it. Everything else rides the repeatable `tag: 'key:"value"'`
(spec §6.3 extension, Appendix A.5), one Go struct tag passed through
verbatim: `db` is reserved for the scan contract, a `json` pair
replaces the generated default, others append in declaration order,
and params structs carry the same tags.

### validation-rides-checks

No annotation zoo. `checks { }` lines are parameterless predicates and
validation reuses them (spec §V12): a typed check compiles to both a
SQL `CHECK` and a clause of the generated `Validate()`, rendered in one
walk so the tiers cannot disagree, while a check referencing a Go
function of the package lands in the validator only, because SQLite
cannot call Go. Identity of the tiers is bought with three
narrowings: not-null columns only, no date or time columns, fractional
literals only against floats. Nothing calls `Validate` implicitly.

### gofmt-canonical

Generated Go is gofmt-canonical by construction; gofmt is a test, not
a stage. `gen/align` lays out aligned blocks with tabwriter's own
rules, the emitters apply the rest, and gofmt must be the identity on
every golden and every valid conformance project. There is one output
shape and no `--no-fmt`.

### verify-type-checks

`gen --verify` lays the outputs over the target directory and
type-checks the package before writing, so a wrong signature, a
missing import or a collision with a hand-written name is this run's
error and nothing is written. An error in a generated file is a
generator bug; one beside the outputs leaves the check inconclusive
and says so.

## Query layer

### options-are-data

Options and predicates are plain values, a small expression tree,
never closures and never a mutable builder: one terminal call walks
the tree once and renders SQL and arguments in lockstep. Values
compose, store in variables, append conditionally, and can be
inspected, cached and tested; a closure can only be executed.

### flat-handles

The runtime defines `Column[M, T]` once (phantom model type `M`;
`NullColumn[M, T]` adds `IsNull`/`SetNull`, comparisons take plain
`T`), and the generator emits one-line vars with flat `Model+Field`
names in package scope. Operators live once in the runtime; the
phantom type makes a cross-model mixup a compile error. A
concatenation collision fails generation loudly and the vet `dynname`
rule reports every one with both origins; nothing is renamed silently.

### per-model-wrappers

Go cannot infer `M` for value-less options, so the generator emits
per-model wrappers: `UserLimit(n)`, `UserOffset(n)`, `UserOrderBy`,
`UserAfter`, `UserDistinct`, `UserSet`.

### statement-cache

Prepared statements are cached per rendered SQL text (LRU). The
interpreter is deterministic, so identical tree shapes render
identical SQL. Static SQL is prepare-validated at generation time;
dynamic shapes by construction plus first prepare. The cache binds to
the handle `WithCache` saw and `WithTx` drops it, because cached
statements belong to the outer connection.

### shape-rule

Anything that changes what a row is (joins, computed columns, GROUP
BY, HAVING) happens at generation time, where structs can be minted.
Anything that only filters, orders or limits an existing shape is
runtime material. Predicates are shared across the verbs: `Query`,
`Count`, `Exists`, `DeleteWhere`, `UpdateWhere` with typed `Assign`
setters.

### keyset-pagination

Keyset pagination is first-class in the runtime: `After` renders the
lexicographic expansion over the `OrderBy` terms and errors without
an order or with the wrong key arity. `OFFSET` degrades linearly;
the solo developer with a hundred million rows is the thesis.

### positional-placeholders

Dynamic SQL binds positional `?` placeholders: a dynamic predicate has
no stable column-derived name, the same column can appear twice, and
independently built fragments must not collide, so parameter hygiene
belongs to the interpreter. Its guarantees: `LIMIT` and `OFFSET`
values bind as parameters so page size never changes the text;
`In()` over the empty set renders constant false (`NOT IN` constant
true); the zero-value `Pred` vanishes from `And`, `Or`, `Not` and
`WHERE`; `DeleteWhere` and `UpdateWhere` whose predicates all vanish
are errors, and affecting every row is written out loud
(`rt.Raw("1 = 1")`).

### list-parameters

`col in :names` types the parameter as a slice of the column's Go
type and renders `col IN (SELECT value FROM json_each(:names))`, one
JSON array, so the statement text is static whatever the list length,
the prepared statement is the one that runs, and an empty list is
`[]`, no rows. Time-class columns refuse a list parameter, since JSON
has no date literal.

## Runtime and CRUD

### crud-miss-semantics

`Get` and `GetBy*` return `rt.ErrNotFound`, which is `sql.ErrNoRows`.
`Update` rewrites every non-key column and returns the row via
`RETURNING`, `ErrNotFound` on a miss; `Delete` checks `RowsAffected`.
`GetMany` treats missing keys as absence and exists only for
single-column keys. Every list-shaped result is an empty slice, never
nil, so an empty reply is `[]` on every wire.

### one-connection

`rt.Open` pins the pool to one connection and applies WAL,
`busy_timeout=5000` and `foreign_keys=ON`. Pragmas are per connection
in SQLite, so one connection makes them hold for every statement and
removes in-process busy contention; SQLite has a single writer anyway.

### temporal-types

The DDL declares `TIMESTAMP`, `DATETIME` and `DATE` (NUMERIC
affinity, ISO-8601 text) because SQLite drivers key their `time.Time`
parsing on the declared type. Time-of-day types map to Go `string`:
SQLite has no time type and drivers no scan rule for one.

## The framework

### servemux-matcher

Registrations go onto the stdlib `http.ServeMux`, a real router with
registration-time conflict detection since Go 1.22; Volt spends its
code on what surrounds the match: typed shims, pipelines, reverse URLs,
the route table. A generated static matcher stays possible behind the
same API and is built only if profiling justifies it.

### no-runtime-routes

The language is the only way a route exists; there is no
`router.GET(...)`. The escape hatch is `mount`, which forwards a
subtree to any `http.Handler`. A closed route table is the
precondition for typed reverse URLs, conflict checking and datasets.
Pipelines speak `func(http.Handler) http.Handler`; the `error` return
lives only on controller interfaces, where the generated shim owns the
error spine.

### query-routes

A handler reference whose first part is an import qualifier, or the
package's own name, binds the route to a generated query method, and
Volt writes the handler (spec §V4.8, §V4.9): path parameters by name
with the type spelled to match, a params struct from the body by
`Content-Type`, list parameters from a repeated query key, the rest
from the query string, then `RenderStatus` by `Accept`. It is the one
route shape whose request and response types Volt can prove end to
end. Hand-written functions in the handler slot stay controller
routes; formats never ride in the URL, headers carry them. The
`Controllers` manifest carries one `*pkg.Queries` per data package.

### generated-client

`volt gen` writes `client/volt_client.go` beside every routing
package (spec §V4.10): a `Client` embedding the runtime's, one typed
method per query route named as its helper is, one raw
`*http.Response` method per named controller route. It imports the
runtime and the data packages only, so a desktop application shares
the generated models without importing the server; the one exception
is a package that routes its own tables. A non-2xx reply is a
`volt.HTTPError` matched by code, so `errors.Is(err,
volt.ErrNotFound)` reads the same on both sides.

### dataset-expansion

`dataset db.browse [strip: 'da_']` is one scope item that yields one
GET query route per member of a group select, the segment being the
table name minus the prefix, every route obeying the conflict and
helper rules (spec §V13). `only:`/`except:` take a member out to
hand-write it. The route string and the query call come from the same
iteration and cannot disagree; every expanded route exists in the
generated source and in `volt routes`. The reserved word `Dataset` is
retired.

### resources-default

`resources db.t [default]` expands the §V5.2 action table into query
routes over the table's CRUD, regenerated forever, with `except:` to
take an action back by hand. `default` implies `api`, since a
generated handler has nothing to render for `new` and `edit`. It is
not a scaffold.

### server-sent-events

Live updates are server-sent events through one runtime broker (spec
§V4.11): `get /events volt.Events` is the one handler the runtime
provides, `Publish` fans out to every stream with a replay buffer for
`Last-Event-ID`, and the generated client's `Events(ctx)` reconnects
with backoff. One process, in memory; event names are the
application's contract, not the language's.

### checks-on-the-api-path

Params structs get their own `Validate` over the checks they can see,
the generated handler runs it before the query, `CheckError` and
`ValidationError` are 422 by interface, and every generated write
translates the driver's constraint failure into `rt.ConstraintError`,
409 for a duplicate key and 422 otherwise (spec §V12.6, §V12.7,
§V4.8.3). A check the struct cannot see (a defaulted column) stays
with the row and the DDL, and the method's doc comment says which.

### required-and-problems

`required` is the word for "not empty either": it lowers to a check
in both tiers, named `<column>_required`, and is refused where it
means nothing or contradicts (`null`, `default:`, `increment`).
Errors name their columns, the spine renders a structured `Problem`
body the client decodes, `Decode` rejects unknown JSON fields and
caps the body (spec §V12.7, §V12.8, §V4.9.2, §V4.9.5). Messages are
not localized; the check's name is the stable key.

### self-routing-package

A package may route its own tables: a query reference is spelled
bare, `PostList`, or self-qualified, `site.PostList`, and a bare
`resources` or `dataset` resolves in the package itself (spec §V2.4,
§V4.3, §V4.8.6). The package's own name is a qualifier an import
cannot take, the `Controllers` manifest is one namespace, the router's
constructor is `NewRouter`, and the client imports the routing
package for its row types. No client is generated for a `main`
package that routes its own tables.

### layout-independent-gen

`volt gen -o DIR -parts LIST FILE` writes one package's files into
any directory under the package clause that directory declares, with
`-parts` choosing among models, queries, router, client and sql, so a
`//go:generate` line beside a `main.go` is the layout (spec §V1.7,
§V4.10.6). The params structs are models, so a client needs nothing
but `nao_models.go`. The `.volt` package clause stays mandatory, and
there is no manifest describing layouts.

## Data layer doctrine

### integration-over-mocks

Generated code is exercised against a real SQLite with a test-only
driver; the language front end keeps its exhaustive unit corpus;
golden files surface unintended generator-output changes. Every
generated SQL artifact executes against a real engine in tests, and
the committed fixtures are drift-checked against the generator.

### non-goals

Never: identity map or unit of work, lazy loading, active-record
`Save()`, reflection in generated paths, callbacks, dirty tracking,
default scopes, single-table inheritance or delegated types, a
Turing-complete query DSL.

## Docs

### docs-doctrine

`docs/` holds exactly: `spec.md` (normative, every constraint in the
corpus), `lint.md` (every rule with executable examples, drift-checked
by build), `editor.md` (the tooling and its verification checklist),
`decisions.md` (this file), `roadmap.md` (features with IDs and
statuses, deferred chores included), `hypotheses.md` (open
questions), `nao.md`, `model-layer.md`, and `orm-matrix.md` (pinned
research, consulted not maintained). No other directory carries prose
docs; the root README.md and AGENT.md are the exceptions. Operational
detail may appear only in docs a test or the compiler keeps honest;
philosophy is written at an altitude where nothing can rot; no doc is
ever knowingly wrong, so a change whose blast radius includes a doc
updates the doc in the same commit, and a feature's status updates in
the commit that lands it.

### comments-are-contracts

A code comment states what a thing is or does, a contract a caller
relies on, or an invariant the code depends on, in one or two
sentences, and cites the spec section or decision that holds the
reason. Rationale, history, alternatives, measurements and
descriptions of callers do not live in comments. Test files carry no
comments beyond directives; the test's name and its got/want are its
documentation, with one carve-out for the proof instruments (the
purity gate, the grammar tests, the session golden), where each check
keeps the one-sentence rule it enforces.

## Performance architecture

### one-naming-plan

Right after the schema pass, `lang.Check` builds one immutable naming
plan per package: every Go and SQL name of every table and column,
the CRUD method table indexed by method name, the §V11.7 name scope.
Everything downstream reads that plan; nothing derives a name from the
AST again, and no helper plans on the caller's behalf inside a loop.

### worker-pool

One primitive, `internal/par.For(n, fn)`: a phase hands it
independent items, they run on up to GOMAXPROCS goroutines, every
result lands in its own index slot, and assembly happens in index
order, so no output depends on which CPU finished first. Phases are
barriers; no phase writes another package's fields. A test runs the
same project on one CPU and on all and compares diagnostics and
bytes under the race detector.

### indexed-lookups

A per-route or per-table question is answered from an index built
once, never by a scan: select methods by generated name, routes in a
trie of literal prefixes, validators memoized per table. The
Go-reference scan skips files that open with the generated marker.
`Check` is idempotent on a Project: a package's results are reset
before its phases run. The trie's equivalence to the pairwise
ambiguity rule is a rule of the spec (§V4.7 rule 3) and is proven by
enumeration.

### flat-front-end

A `token.File` holds the name and the source once; a position is that
pointer plus three 32-bit offsets; a token borrows its text from the
file by offset and length, 48 bytes; the parser hands out its hottest
node kinds from slabs. Consumers read text through `Token.Text()` and
the filename through `Position.Filename()`; a token the source does
not contain is `token.Synthetic`. A Position always carries its file.

### parse-by-declaration

`parser.ParseFileReuse` cuts a source into chunks, one per top-level
element at the language's boundary (one-parse), and parses each as a
`token.File` of its own with a base offset and line. The next parse of
the same file keeps every chunk whose text is unchanged, nodes and
diagnostics as they are, relocated by one store of its base, and
parses only what changed. Positions are relative to their chunk and
answer through accessors that add the base atomically, because a
reused declaration is shared with a result a server thread may still
be reading. A shared chunk is mutated nowhere but its base.

### check-by-declaration

The checker's per-package phases keep memos across a session's
checks, each keyed on the identity of its inputs: a table from its
node and its injected partials' nodes, a model from the checked table
and the enum types, the lowered checks from the table, its model and
the directory's Go files, a select from its declaration, members,
models, predicates and the recorded answers of the name scope, and a
route from its scope item, what its scopes inherit, the package's
facts and the recorded answers of a project oracle. A memo drops every
entry a check did not use. A table's CRUD methods, field signatures
and minted names are built with its model. What the check does with a
lowered route, judging it against the accepted routes, it does on its
own copy. There is no reverse dependency index: each memo verifies
its own inputs on lookup.

### index-by-table

The editor's navigation index is kept by table across analyses: a
table whose declaration node and checked table are the objects they
were contributes the occurrences it did, select hovers are kept by
the select they were rendered for, the package directory's Go scan is
kept while the sources are the bytes they were, and each build
copies references into maps of its own, so a request reading the
index during the next build shares nothing with it.

### vet-by-declaration

An analyzer is a Rule, a Fold, or both. A Rule judges one declaration
as a pure function of its syntax, its nodes in source order and the
facts the checker resolved for it, and `vet.DeclVet` is a target of
the purity gate. A Fold judges the file from the checker's model and a
summary per declaration. The session keeps a vet memo per package
keyed on the declaration node, the facts' identity and the rule set.
The generated-name rule reads the collisions the plan's name base
keeps rather than walking every name.

### editor-session

The language server keeps a `lang.Session` per project root: every
file's last parse, reused when the bytes are the same, and every
package's last results keyed by its inputs; a package on an import
cycle is never memoized. An edit is recorded cheaply on the handler
goroutine through the document's own memoized front end (parse by
declaration, check by table, index by table, owned by that
goroutine), and one analysis per project runs per quiet moment on a
goroutine that touches no Document, publishing from its own snapshot;
every request adopts the newest result at its start. The single-file
vet runs only where the document's own verdict is what the editor
shows. Sync is incremental and every publish of diagnostics carries
the version of the text they are positioned in. A run that lost the
race is superseded, never cancelled; identity is by counter, never by
address.

### generated-stress

The project the scaling tests and benchmarks run on
(`internal/corpus`: every feature, both layouts, any size) exists only
while a test runs; no repository carries a thousand-table copy.
`volt stress DIR` writes the one-file shape to disk, a thousand tables
of a hundred and fifty columns by default, as a module that builds, so
the limits of check, gen, the server and the Go compiler are felt on
the project the tests measure. It refuses a directory that holds
anything.

## Proof architecture

### pure-memos

What a memo answers is a pure function of explicit inputs: a top-level
function whose parameters are its whole input and whose results are
its whole effect. The build proves it: `cmd/volt`'s purity gate walks
each target and everything it calls, through every implementation of
an interface it calls, to a fixpoint, and refuses a read of mutable
package state, a write through an input (an append to a slice whose
array may be an input's included), a call through a function value the
walk did not see made, a call into a standard package that keeps
state, and any goroutine or channel. A new memo adds its function to
the gate's targets in the same commit.

### keys-are-inputs

A memo key is its inputs themselves, compared element by element,
never a hash, a digest, a serialized spelling, or a size and time
stamp. A disk file is read on every load and its parse reused only
when the bytes are the bytes of the last parse; a Go directory is read
on every check and its scan kept only when every source is what it
was. The read is the cost of knowing, and it is small.

### immutable-results

What a memo answers is written by nobody: the gate walks every
function of the three modules with everything from outside it tainted
and refuses a write through a tainted value into a memoized result
type, or into a node or token, unless the function is the result's
producer or the value's maker (the parser and `ast` for nodes, the
scanner, parser and `token` for tokens). A standard function that
writes its argument is a write. No cache is filled on first use inside
a memoized result.

### shared-surface

Freedom from data races has no static proof in Go, so the shared
surface is kept small enough to name, each object safe by
construction, and the race detector is the bar on every change. The
surface: the worker pool's claim counter; a token file's atomic base
offset and line; a check run's Go scan cache (mutex, once); the
server's document and analysis tables (one mutex); the session's
caches (one mutex), with the session running one operation at a time.
A shared object outside this list is a bug.

### trust-boundary

The purity argument trusts a named list of standard packages to be
functions of their arguments (`strings`, `strconv`, `unicode`, `fmt`,
`sort`, a compiled `regexp`) and `go/build`'s default context to stay
what it was. The gate keeps the list exact: it records every standard
package a memoized computation reaches and fails on an entry none
reaches. Trust is never granted in advance of use.

### no-unsafe

Nothing in the three modules imports `unsafe`; the gate refuses it.
The AST walker tests each optional field as the pointer it is, so a
field the parser left unset is skipped by type, and no typed nil ever
reaches a visitor.
