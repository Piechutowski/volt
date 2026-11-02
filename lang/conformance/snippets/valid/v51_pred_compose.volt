// spec: §predicates — predicates compose predicates; the closed operator set
package db

table ms_revenue {
  id int [pk]
  org string
  year int
  amount float64
  note string [null]
}

pred current { org = :org and year = :year }
pred recent { year >= :since }
pred fresh { current and (recent or year in (2024, 2025)) }
pred noted { note is not null and org like "acme-%" }
pred pricey { amount > 100.5 and not (year < 2000) }

select rows for ms_revenue where fresh and noted and pricey
