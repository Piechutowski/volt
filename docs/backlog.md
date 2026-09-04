# Backlog — scheduled chores

Work the maintainer has decided to do and deliberately deferred: each
entry says why it is not now, so nothing is forgotten and nothing is
started by accident. When an entry lands, delete it here in the same
commit (D49: no doc knowingly wrong).

## Citations by heading name

**Decided:** 2026-09-03 (D64). **Deferred because:** the sweep touches
roughly nine hundred sites — error messages, `spec/…` diagnostic codes,
`// spec:` corpus tags, tests asserting message text, every doc — and
more important work is queued.

The spec's sections are named, not numbered (Go style). Citations still
use the former numbers (`§V11.4`, `Appendix A.5`) and resolve through
the spec's *Citation key*. The chore:

1. Replace every citation with the heading-name form, e.g.
   `spec "Selects over groups" (4)` for rule 4 of that section; the
   `spec/V11` diagnostic code family becomes a short stable slug per
   section (`spec/selects`), listed once in the spec.
2. Extend `lang/spec_doc_test.go` so the citation-integrity test
   resolves names instead of numbers.
3. Delete the Citation key section.

One scripted sweep with a mapping table; verification is the test in
step 2, not a review.

## Validation, round 3

**Decided:** 2026-09-04 (after D72). **Deferred because:** none of these
blocks the browse-the-tables round in the battlefield app; each is taken
the day a real rule in that app needs it, so the syntax is chosen against
a case and not a guess.

What stands after D71/D72: typed checks (comparisons, `and`/`or`/`not`,
`in`, `like`, `[required]`) in both tiers; Go-reference checks of any
arity over one row, signature verified (D63); `rt.Constraint` typing the
database's refusals; strict, bounded bodies; per-column `Details()`.
The gaps, in the order to close them:

1. **Contextual checks, `[on: create]` / `[on: update]`.** A rule that
   holds only for one verb (a password set on create, a status that may
   only move forward on update). The Go tier is easy: the check joins
   the matching params struct's `Validate` and stays out of the other.
   The SQL tier is NOT "impossible" — a `CHECK` has no verb, but a
   trigger does:

   ```sql
   CREATE TRIGGER "users_password_set" BEFORE INSERT ON "users"
   WHEN NOT (NEW."password_hash" <> '')
   BEGIN SELECT RAISE(ABORT, 'volt check failed: users.password_set'); END;
   ```

   `BEFORE UPDATE OF <cols>` for the update side, `NEW.` prefixed on the
   rendering. So the two-tier guarantee survives. Costs that decide the
   design: (a) triggers enter the DDL diff (D20) as drop-and-recreate,
   which D24 already reserved for them; (b) `rt.Constraint` learns the
   `RAISE` message shape (we own the text, so it is one more prefix,
   `volt check failed: <table>.<check>` → `ConstraintError{Kind:
   "check"}` with the check name); (c) `NEW.` rendering is a third
   walker output next to the SQL and Go forms. Without a trigger the
   setting would be Go-tier only, and the spec would have to say so as
   it does for Go-reference checks; with it, `[on:]` is as strong as any
   other typed check. Take the trigger route.

2. **Enum membership in the Go tier (H5).** The DDL has `CHECK (col IN
   (…))` for every enum column; the Go tier trusts the enum type and a
   caller can still pass `EStatus("bogus")`. Emit one clause per enum
   column into `Validate()` — a `switch` over the enum's constants, or a
   generated `Valid()` method on the enum type called from the validator.
   No syntax, no spec change beyond the generation contract; closes H5.

3. **`length()` in typed checks.** Today a string bound is an opaque SQL
   check or a Go function. Add `length(col)` as a §V10 term of numeric
   type over text, enum and bytes columns, rendering `length(col)` in
   SQL and `len(v.Col)` in Go. One subtlety pins the rule: SQLite's
   `length()` counts characters on text and bytes on blobs, Go's `len`
   counts bytes on both — render `utf8.RuneCountInString` for text
   columns so the tiers agree, and add the pair to the runtime's LIKE-
   style parity corpus.

4. **`[check:]` on select parameters.** Query parameters are typed but
   unbounded: `:limit` accepts a million. A parameter setting carrying a
   predicate over the parameter itself (`:limit (int32) [check: 'limit
   between 1 and 500']`) renders into the query route's binding step
   before the query runs, 422 with the parameter name in `Columns`.
   Go tier only by nature; the spec says so.

5. **Localized messages.** `Detail.Message` is the check's name or its
   rendered form. A `[message: '…']` check setting, plus a runtime hook
   that maps `(table, check)` to text so an app can ship translations
   without touching the schema. The name stays the stable key.

6. **Unknown-field attribution.** `volt.Decode` answers 400 with Go's
   own text (`json: unknown field "emial"`). Surface it as
   `Detail{Check: "unknown_field", Columns: ["emial"]}` so a form marks
   the field like any other failure. Only the first unknown field is
   reported (the decoder stops); and GOB bodies get no such check at all
   because `encoding/gob` drops undeclared fields silently — say so in
   §V4.9.

Not planned: nullable columns inside typed checks (rule 3 stands; the
Go tier does not mirror three-valued NULL logic, and pretending it does
is where Rails and Ecto quietly diverge from their databases), and
composable named validation sets in the schema — Ecto's changesets are a
code-level feature, and in Go the equivalent is `Validate()` followed by
your own function.
