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

## H4 — Column properties belong in the schema, not in Go tags

**Current working position:** Go code is derived, so a property that
today would be a hand-written Go tag or a convention (this column is
sensitive; this column is heavy; this column is internal) belongs on
the **column declaration** in Volt, where every generator and check
can see it.

**What's actually unsettled:** the surface. Candidates, roughly in
order of appeal: a `[sensitive]` column setting that (a) makes vet
warn when a projection or full-row select exposes the column to a
route or dataset, and maybe (b) excludes it from `*` unless named
explicitly; a `[heavy]`-style hint for list-view defaults; whether
these are separate flags or one extensible property mechanism.
Decide from a real leak-shaped moment in the app, not in advance.
