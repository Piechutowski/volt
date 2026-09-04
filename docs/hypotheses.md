# Hypotheses

Working beliefs that have **not** earned decision status: recorded so
they are not re-argued from scratch, numbered so they can be cited, and
held loosely. Each one either graduates — into `decisions.md`, the
spec, or the roadmap — or dies, when practice supplies the evidence.
Unlike decisions, a hypothesis binds nothing; unlike a roadmap entry,
it is a question, not a plan. Numbers retire like decision numbers.

## H1 — How much DBML compatibility is worth keeping

**Current working position** (and what the spec implements today): the
schema core is DBML minus the file-import system — a core-only file
stays upstream-valid and diagrams on dbdiagram.io as-is (§V0.2,
Appendix C), and everything beyond the core is Volt's own, additive,
owing upstream nothing.

**What's actually unsettled:** whether that compatibility keeps paying
for itself as Volt grows. The file-import break was the first cut;
§8.1 schemas (`auth.users`, the implicit `public`) are the next
candidate — Rails AR treats schemas as a connection/database concern,
not a model concern; SQLite has no schemas at all (Appendix B flattens
them); and Volt packages already namespace better. Each further cut
trades diagrammability against language coherence, and is made
deliberately, one decision at a time, when it bites — never as a side
effect.

## H2 — nao's ergonomic surface

**Current working position:** values first — each scan is a fresh
plain struct; no identity map, no `Save()`, no association fields on
row structs (D14 records what is built).

**What's actually unsettled:** which Active-Record-grade conveniences
belong in nao at all. The maintainer is building a real application on
Volt; the app is the experiment. When a convenience is missed in
practice, it gets a verdict against `reference/orm-matrix.md`, and it
is welcome **iff it can be generated** — visible, compile-checked,
plain Go (candidates: association ergonomics via explicit loaders,
generated join structs, validations compiling to `CHECK`s and a
validator surface). The immovable line is D27's: no lazy loading,
callbacks, dirty tracking, identity map — those are runtime magic by
nature, not by implementation choice.

## H3 — Member-scoped select overrides

**Current working position:** everything a `Select` says applies to
every member of its group uniformly — projection exclusions included.

**What's actually unsettled:** whether one member should be able to
deviate (exclude a column only for itself, override an operation only
for one table). The shape this wants is the Dataset override ladder
(roadmap FW-2): defaults generated, overrides declared per member,
escape by omission. Decide when FW-2's ladder lands, so selects and
datasets end up with one override idiom, not two.

## H4 — Which field-property settings to mint, and when

**Settled part → D59:** every per-field property lives on the column
declaration; generated code cannot carry hand-authored tags.

**Graduated part → D60** (2026-09-02, built): the namespace resolution
— first-class settings only for what Volt itself understands, plus the
repeatable `[tag: 'key:"value"']` verbatim passthrough (spec §6.3
extension, Appendix A.5).

**What's actually unsettled:** which first-class settings to mint and
when (`[sensitive]` is the candidate), and whether a Go-field-name
override (`[go:]`-style, sibling of `[model:]`) is ever needed — the
gob rename case, since gob ignores tags.

**Graduated part → D61** (2026-09-02, built): validation rides checks
— spec §V12, `nao_validate.go`, `rt.CheckError`/`rt.Like`.

Mint each surface from a real moment in the app, not in advance; every
one added costs spec + checker + generator + corpus (the D57 lesson:
closed sets, deliberately grown).

## H5 — Enum columns in predicates

Enum-typed columns cannot be compared in the predicate language (spec
§V10.4): `where plan = :plan` is refused. The refusal is a v1
narrowing, not a belief: a typed enum parameter (`:plan` typed as the
generated `EPlan`, literals checked against the enum's values) is
well-defined in both tiers. **Belief:** it should be lifted the first
time a real app filters by an enum, with parameters typed as the enum
and `in (a, b)` literals validated against the declared values —
never as loose strings.

## H6 — Custom route segment types

**Current working position** (spec §V4.1.3): the parameter type set is
closed, `int`, `int32`, `int64`, `string`, chosen to coincide with the
Go types the model generator emits for routable primary keys. Types
shape handler signatures, never matching; a segment that fails to
parse is that route's 404, never a fallthrough.

**What's actually unsettled:** whether a package should be able to
name its own segment type, `:code(PostalCode)`, for shapes the closed
set cannot spell (a Polish postal code, a slug, a ULID, a date). Two
shapes were considered:

1. A regex declared in Volt. It would have to be a string literal
   (`/` is a token and `/*` opens a comment, §V0.2), it grows a regex
   sublanguage that grammar, LSP, vet and spec all have to learn, and
   it yields no Go type to carry the guarantee into the action.
2. A Go reference held the D63 way: the type name resolves to a named
   type and two functions in the package's own Go files, checked by
   spelling like plugs and Go checks: `func PostalCodeParse(string)
   (PostalCode, bool)` for the shim and `func PostalCodeSegment(
   PostalCode) string` for the `Path*` helper (§V4.6). The action
   receives `code PostalCode`; the helper takes the same type.

**Belief:** the second shape, if either. Invariants that must survive
it: matching stays ServeMux's (D50), so a type never affects dispatch
and two routes differing only in segment type stay ambiguous (§V4.7);
a parse failure stays the route's 404 (Django's converter chaining,
reference/django.md ROUTE-6, would need the static matcher D50 keeps in
reserve); the round-trip proof (`itest` `TestRoundTripTotality`) keeps
holding, so the segment renderer must produce what the parser accepts;
`resources` stays untouched, its key type comes from the primary key
(§V5.4.3); the local-package rule of §V3.2 applies, so an imported
package's type needs a local alias. Not now: this is an FW-6-class
convenience, added the first time a real app needs a shape the closed
set cannot spell.

## H7 — Route guards

**Current working position:** authorization is middleware. It attaches
to a Scope through a Pipeline (§V3, §V4.4) and sees the request before
the shim parses parameters; object-level checks ("does this user own
this post") live in the action body. Authorization is never a segment
type (H6): a type is a pure function of the segment string, with no
request and no I/O, and a `Path*` helper cannot render a permission.

**What's actually unsettled:** whether Volt needs a declared per-route
guard, the gap Phoenix fills with controller plugs under action guards
(reference/phoenix.md CTRL-7) and Laravel with policies over bound
models (reference/laravel.md ROUTE-9). Candidate shape: a `guard:`
setting on a route or Scope naming a function of the package, held by
D63 with the route's typed parameters appended,
`func PostAccessible(r *volt.Request, id int32) error`. The shim calls
it after parsing and before the action; an error goes down the route's
error spine (`volt.ErrForbidden` is a 403); the guard appears in
`volt routes`, so the table stays the whole truth. Tension to resolve
first: D27 forbids callbacks. A guard is a declared, visible step of
the route line, not a lifecycle hook, but that reading is the
maintainer's to confirm before anything is built.

**Belief:** wait. Scope pipelines already cover role-level checks, and
the alternative that makes Laravel's policies pay for themselves, model
binding, is not planned. Decide the first time a real app on Volt
needs object-level authorization that neither a Scope pipeline nor two
lines at the top of the action express well.
