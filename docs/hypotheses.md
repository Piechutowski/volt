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
