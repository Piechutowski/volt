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

**Validation — the maintainer's design, replacing the settings
sketch:** no `[min:]`-style settings at all. `checks { }` blocks ARE
parameterless predicates (D57), so validation reuses them in two
tiers:
- a check written in the typed predicate language compiles to **both**
  a SQL `CHECK` in the DDL and a generated Go validation (early,
  friendly errors; the DB stays the last line of defense);
- a check referencing a **hand-written Go function**
  (`app.EmailValid(email)` — the same Go-ref pattern pipelines use for
  middleware) lands in the generated validator only, never in DDL
  (SQLite cannot call Go); the asymmetry is documented, not hidden.
One construct, zero new settings names, composable with everything
Pred already is.

Mint each surface from a real moment in the app, not in advance; every
one added costs spec + checker + generator + corpus (the D57 lesson:
closed sets, deliberately grown).
