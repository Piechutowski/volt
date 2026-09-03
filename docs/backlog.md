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
