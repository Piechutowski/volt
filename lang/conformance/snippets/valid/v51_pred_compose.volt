// spec: §V10 — predicates compose predicates; the closed operator set
package db

Table ms_revenue {
  id integer [pk]
  org text [not null]
  year integer [not null]
  amount real [not null]
  note text
}

Pred current { org = :org and year = :year }
Pred recent { year >= :since }
Pred fresh { current and (recent or year in (2024, 2025)) }
Pred noted { note is not null and org like 'acme-%' }
Pred pricey { amount > 100.5 and not (year < 2000) }

Select rows for ms_revenue where fresh and noted and pricey
