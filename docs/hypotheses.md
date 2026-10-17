# Hypotheses

> [!CAUTION]
> **This project is an alpha version, and everything may change in the future.**

Open design questions, numbered so they can be cited, held loosely.
One graduates into the spec when it is built, or dies when practice
says otherwise. A hypothesis binds nothing.

## H1 — How much DBML compatibility is worth keeping

Today the schema core is DBML minus file imports, so a core-only file
pastes into dbdiagram.io. Unsettled: whether that keeps paying for
itself. Schemas (`auth.users`, the implicit `public`) are the next
candidate cut, since SQLite has none and packages already namespace;
the presentation constructs (colors, TableGroup, DiagramView, sticky
notes) are the one after, once a Volt-owned diagram tool exists.

## H2 — nao's ergonomic surface

Today each scan is a fresh plain struct: no identity map, no `Save()`,
no association fields. Unsettled: which conveniences join it. A
convenience is welcome iff it can be generated as visible, compiled Go
(explicit association loaders, generated join structs). Lazy loading,
callbacks, dirty tracking and identity maps stay out (§non-goals).

## H3 — Member-scoped select overrides

Today everything a `Select` says applies to every member of its group.
Unsettled: whether one member may deviate. Decide together with the
dataset override ladder so selects and datasets share one idiom.

## H4 — Which field-property settings to mint

Today every per-field property is a column setting, and anything Volt
does not act on rides `[tag: 'key:"value"']` verbatim. Unsettled: which
first-class settings to add (`[sensitive]` is the candidate) and
whether a Go-field-name override is ever needed. Mint each from a real
need; every one costs spec, checker, generator and corpus.

## H5 — Enum columns in predicates

Today enum columns cannot be compared in the predicate language.
Unsettled only in timing: a parameter typed as the generated enum with
literals checked against the declared values is well defined in both
tiers and should land the first time an application filters by an
enum.

## H6 — Raw-SQL query blocks

Belief: structure is Volt's and expressions are SQLite's. A declared
raw-SQL `Select` or `View` block would have its select list, tables,
joins and parameters parsed by Volt's front end and its WHERE, HAVING
and other expression bodies left opaque and validated by preparing
them against the generated DDL. Parameters of an opaque body must be
declared, since nothing else can type them; joins follow a unique
declared ref with an explicit `on:` as the override. Unsettled: all of
it, including whether `volt gen` ships a SQLite to do the preparing.

## H7 — Migrations

Belief: the schema file is authored, the ledger is derived. A
migration is a generated entry pinning the canonical hash of the
checked schema before and after, listing its operations and their
SQLite realization, prepare-validated against the previous snapshot.
Rename provenance lives in the entry, captured at diff time by a
question, a flag, or the editor's rename refactoring, never in the
schema file. Unsettled: all of it.
